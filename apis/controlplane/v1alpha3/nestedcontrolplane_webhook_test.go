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

package v1alpha3

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeNestedControlPlane(updaters ...func(old *NestedControlPlane)) *NestedControlPlane {
	obj := &NestedControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NestedControlPlane",
			APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "controlplane",
		},
		Spec: NestedControlPlaneSpec{},
	}

	for _, f := range updaters {
		f(obj)
	}
	return obj
}

func TestNestedControlPlane_Default(t *testing.T) {
	obj := makeNestedControlPlane()

	tests := []struct {
		name                   string
		controlPlane           *NestedControlPlane
		expectedNamespaceMatch bool
	}{
		{
			"Test Defaulting Name and Namespace",
			obj,
			false,
		},
		{
			"Test Defaulting Name Only",
			makeNestedControlPlane(func(n *NestedControlPlane) {
				n.Spec.ClusterNamespace = "cn"
			}),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.controlPlane.DeepCopy()
			r.Default()

			if tt.expectedNamespaceMatch && !reflect.DeepEqual(tt.controlPlane.Spec.ClusterNamespace, r.Spec.ClusterNamespace) {
				t.Errorf("cluster namespace %s does not match %s", tt.controlPlane.Spec.ClusterNamespace, r.Spec.ClusterNamespace)
			}
		})
	}
}
