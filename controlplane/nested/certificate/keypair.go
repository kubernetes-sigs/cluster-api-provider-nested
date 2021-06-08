/*
Copyright 2021 The Kubernetes Authors.
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

package certificate

import (
	"crypto/rsa"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	"sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/certificate/util"

	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AsSecret will take a KeyPair and convert it into a corev1.Secret.
func (k *KeyPair) AsSecret(clusterName client.ObjectKey, owner metav1.OwnerReference) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterName.Namespace,
			Name:      secret.Name(clusterName.Name, k.Purpose),
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName.Name,
			},
		},
		Data: map[string][]byte{
			secret.TLSKeyDataName: util.EncodePrivateKeyPEM(k.Key.(*rsa.PrivateKey)),
			secret.TLSCrtDataName: util.EncodeCertPEM(k.Cert),
		},
		Type: clusterv1.ClusterSecretType,
	}

	if k.Generated {
		s.OwnerReferences = []metav1.OwnerReference{owner}
	}
	return s
}
