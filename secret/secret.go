/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secret

import (
	"crypto/rsa"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
	"sigs.k8s.io/cluster-api-provider-nested/pki"
)

const (
	CAPostfix                   = "ca"
	ProxyPostfix                = "proxy"
	EtcdPostfix                 = "etcd"
	ControllerManagerSecretName = "controller-manager-kubeconfig"
	AdminSecretName             = "admin-kubeconfig"
	ServiceAccountSecretName    = "serviceaccount-rsa"
)

// GenerateSecretName will create the CA name from clusterName and a postfix
// this supports the cert naming from https://cluster-api.sigs.k8s.io/tasks/certs/using-custom-certificates.html
func GenerateSecretName(clusterName, postfix string) string {
	return strings.Join([]string{clusterName, postfix}, "-")
}

// RsaKeyToSecret encapsulates rsaKey into a secret object
func RsaKeyToSecret(name, namespace string, rsaKey *rsa.PrivateKey) (*v1.Secret, error) {
	encodedPubKey, err := pkiutil.EncodePublicKeyPEM(&rsaKey.PublicKey)
	if err != nil {
		return nil, err
	}
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			v1.TLSCertKey:       encodedPubKey,
			v1.TLSPrivateKeyKey: pki.EncodePrivateKeyPEM(rsaKey),
		},
	}, nil
}

// CrtKeyPairToSecret encapsulates ca/key pair ckp into a secret object
func CrtKeyPairToSecret(name, namespace string, ckp *pki.CrtKeyPair) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			v1.TLSCertKey:       pkiutil.EncodeCertPEM(ckp.Crt),
			v1.TLSPrivateKeyKey: pki.EncodePrivateKeyPEM(ckp.Key),
		},
	}
}

// KubeconfigToSecret encapsulates kubeconfig cfgContent into a secret object
func KubeconfigToSecret(name, namespace string, cfgContent string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			name: []byte(cfgContent),
		},
	}
}
