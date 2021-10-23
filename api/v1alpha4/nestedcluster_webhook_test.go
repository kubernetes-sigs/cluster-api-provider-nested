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

package v1alpha4

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestNestedCluster_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		old     *NestedCluster
		new     *NestedCluster
		wantErr bool
	}{
		{
			name: "NestedCluster with immutable spec",
			old: &NestedCluster{
				Spec: NestedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "foo",
						Port: 6443,
					},
				},
			},
			new: &NestedCluster{
				Spec: NestedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "bar",
						Port: 6443,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NestedCluster with mutable metadata",
			old: &NestedCluster{
				Spec: NestedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "foo",
						Port: 6443,
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
			new: &NestedCluster{
				Spec: NestedClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "foo",
						Port: 6443,
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "fooNew",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.new.ValidateUpdate(tt.old)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
