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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	configv1alpha1 "sigs.k8s.io/cluster-addons/installer/pkg/apis/config/v1alpha1"
)

func makeNestedControlPlaneTemplate(updaters ...func(old *NestedControlPlaneTemplate)) *NestedControlPlaneTemplate {
	obj := &NestedControlPlaneTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NestedControlPlaneTemplate",
			APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "template",
		},
		Spec: NestedControlPlaneTemplateSpec{
			KubernetesVersion: KubernetesVersion{
				Major: "1",
				Minor: "18",
				Patch: "1",
			},
			Components: []configv1alpha1.Addon{},
		},
	}

	for _, f := range updaters {
		f(obj)
	}

	return obj
}

func TestNestedControlPlaneTemplate_ValidateUpdate(t *testing.T) {
	obj := makeNestedControlPlaneTemplate()

	type fields struct {
		TypeMeta   metav1.TypeMeta
		ObjectMeta metav1.ObjectMeta
		Spec       NestedControlPlaneTemplateSpec
	}
	type args struct {
		old runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"TestValidUpdate",
			fields{ObjectMeta: obj.ObjectMeta, Spec: obj.Spec},
			args{obj},
			false,
		},
		{
			"TestInvalidUpdateWithMajorVersion",
			fields{ObjectMeta: obj.ObjectMeta, Spec: obj.Spec},
			args{makeNestedControlPlaneTemplate(func(obj *NestedControlPlaneTemplate) {
				obj.Spec.KubernetesVersion.Major = "2"
			})},
			true,
		},
		{
			"TestInvalidUpdateWithMinorVersion",
			fields{ObjectMeta: obj.ObjectMeta, Spec: obj.Spec},
			args{makeNestedControlPlaneTemplate(func(obj *NestedControlPlaneTemplate) {
				obj.Spec.KubernetesVersion.Minor = "2"
			})},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &NestedControlPlaneTemplate{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			if err := r.ValidateUpdate(tt.args.old); (err != nil) != tt.wantErr {
				t.Errorf("NestedControlPlaneTemplate.ValidateUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
