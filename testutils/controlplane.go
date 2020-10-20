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

package testutils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "sigs.k8s.io/cluster-addons/installer/pkg/apis/config/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
)

// MakeNestedControlPlaneTemplate will generate a dummy type which can be customized by functions
func MakeNestedControlPlaneTemplate(updaters ...func(*v1alpha3.NestedControlPlaneTemplate)) *v1alpha3.NestedControlPlaneTemplate {
	obj := &v1alpha3.NestedControlPlaneTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NestedControlPlaneTemplate",
			APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "template",
		},
		Spec: v1alpha3.NestedControlPlaneTemplateSpec{
			Default: true,
			KubernetesVersion: v1alpha3.KubernetesVersion{
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

// MakeNestedControlPlane will generate a dummy type which can be customized by functions
func MakeNestedControlPlane(updaters ...func(*v1alpha3.NestedControlPlane)) *v1alpha3.NestedControlPlane {
	obj := &v1alpha3.NestedControlPlane{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NestedControlPlane",
			APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "controlplane",
			Namespace:    "default",
		},
		Spec: v1alpha3.NestedControlPlaneSpec{},
	}

	for _, f := range updaters {
		f(obj)
	}

	obj.Default()

	return obj
}

// MakeNestedPKI will generate a dummy type which can be customized by functions
func MakeNestedPKI(updaters ...func(*v1alpha3.NestedPKI)) *v1alpha3.NestedPKI {
	obj := &v1alpha3.NestedPKI{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NestedPKI",
			APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pki",
			Namespace:    "default",
		},
		Spec: v1alpha3.NestedPKISpec{},
	}

	for _, f := range updaters {
		f(obj)
	}

	return obj
}

// MakeNestedEtcd will generate a dummy type which can be customized by functions
func MakeNestedEtcd(updaters ...func(*v1alpha3.NestedEtcd)) *v1alpha3.NestedEtcd {
	obj := &v1alpha3.NestedEtcd{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NestedEtcd",
			APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "etcd",
			Namespace:    "default",
		},
		Spec: v1alpha3.NestedEtcdSpec{},
	}

	for _, f := range updaters {
		f(obj)
	}

	return obj
}

func MakeNamespace(updaters ...func(*corev1.Namespace)) *corev1.Namespace {
	obj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "namespace",
		},
	}

	for _, f := range updaters {
		f(obj)
	}

	return obj
}
