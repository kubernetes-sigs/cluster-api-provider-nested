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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newSecret(fns ...func(*corev1.Secret)) *corev1.Secret {
	secret := &corev1.Secret{}
	for _, fn := range fns {
		fn(secret)
	}
	return secret
}

func TestKeyPairs_Lookup(t *testing.T) {
	ctx := context.TODO()
	clusterName := client.ObjectKey{Name: "test-cluster", Namespace: "default"}
	type args struct {
		ctx         context.Context
		cli         client.Client
		clusterName client.ObjectKey
	}
	tests := []struct {
		name    string
		kp      KeyPairs
		args    args
		wantErr bool
		wantNew bool
	}{
		{
			"TestNotFoundCertificateTrue",
			KeyPairs{&KeyPair{Purpose: EtcdClient, New: false}},
			args{ctx, fake.NewClientBuilder().WithRuntimeObjects(newSecret()).Build(), clusterName},
			false,
			true,
		},
		{
			"TestFoundCertificateFalse",
			KeyPairs{&KeyPair{Purpose: EtcdClient, New: true}},
			args{
				ctx,
				fake.NewClientBuilder().WithRuntimeObjects(newSecret(func(s *corev1.Secret) {
					s.Name = "test-cluster-etcd-client"
					s.Namespace = "default"
				})).Build(),
				clusterName,
			},
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.kp.Lookup(tt.args.ctx, tt.args.cli, tt.args.clusterName); (err != nil) != tt.wantErr {
				t.Errorf("KeyPairs.Lookup() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantNew != tt.kp[0].New {
				t.Errorf("KeyPairs.Lookup() new = %v, wantNew %v", tt.kp[0].New, tt.wantNew)
			}
		})
	}
}

func TestKeyPairs_SaveGenerated(t *testing.T) {
	ctx := context.TODO()
	clusterName := client.ObjectKey{Name: "test-cluster", Namespace: "default"}
	ncp := &controlplanev1.NestedControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "ncp", Namespace: "default"}}
	controllerRef := metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	type args struct {
		ctx         context.Context
		ctrlclient  client.Client
		clusterName client.ObjectKey
		owner       metav1.OwnerReference
	}
	tests := []struct {
		name         string
		keypairsFunc func() KeyPairs
		args         args
		wantErr      bool
		wantCount    int
	}{
		{
			"TestCreatingNewSecret",
			func() KeyPairs {
				kp, _ := NewFrontProxyClientCertAndKey(newCA())
				return KeyPairs{kp}
			},
			args{
				ctx,
				fake.NewClientBuilder().WithRuntimeObjects().Build(),
				clusterName,
				*controllerRef,
			},
			false,
			1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keypairs := tt.keypairsFunc()
			if err := keypairs.SaveGenerated(tt.args.ctx, tt.args.ctrlclient, tt.args.clusterName, tt.args.owner); (err != nil) != tt.wantErr {
				t.Errorf("KeyPairs.SaveGenerated() error = %v, wantErr %v", err, tt.wantErr)
			}

			secrets := &corev1.SecretList{}
			if err := tt.args.ctrlclient.List(tt.args.ctx, secrets); err != nil {
				t.Errorf("List().Err expected = got %v", err)
			}
			if len(secrets.Items) != tt.wantCount {
				t.Errorf("KeyPairs.SaveGenerated().Count expected = %v, got %v", len(secrets.Items), tt.wantCount)
			}
		})
	}
}
