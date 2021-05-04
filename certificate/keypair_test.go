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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha4"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newCA() *KeyPair {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	return &KeyPair{
		Purpose:   secret.EtcdCA,
		Cert:      ca,
		Key:       caPrivKey,
		Generated: true,
		New:       false,
	}
}

func TestKeyPair_AsSecret(t *testing.T) {
	clusterName := client.ObjectKey{Name: "test-cluster", Namespace: "default"}
	ncp := &controlplanev1.NestedControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "ncp", Namespace: "default"}}
	controllerRef := metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	type args struct {
		clusterName client.ObjectKey
		owner       metav1.OwnerReference
	}
	tests := []struct {
		name         string
		keyGen       func() *KeyPair
		args         args
		wantOwnerRef bool
	}{
		{
			"TestCreateGeneratedSecret",
			func() *KeyPair {
				kp, _ := NewFrontProxyClientCertAndKey(newCA())
				return kp
			},
			args{
				clusterName,
				*controllerRef,
			},
			true,
		},
		{
			"TestCreateExistingSecret",
			func() *KeyPair {
				kp, _ := NewFrontProxyClientCertAndKey(newCA())
				kp.Generated = false
				return kp
			},
			args{
				clusterName,
				*controllerRef,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := tt.keyGen()
			got := k.AsSecret(tt.args.clusterName, tt.args.owner)
			if tt.wantOwnerRef && len(got.OwnerReferences) == 0 {
				t.Errorf("KeyPair.AsSecret().OwnerReferences = %v", got.OwnerReferences)
			}
		})
	}
}
