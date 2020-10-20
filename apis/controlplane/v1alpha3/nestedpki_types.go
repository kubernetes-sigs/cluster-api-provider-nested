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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO(christopherhein) bikeshed over the naming of this resource it probably
// shouldn't be "PKI" but something more apt to the data it holds

// NestedPKISpec defines the desired state of NestedPKI
type NestedPKISpec struct {
	ControlPlaneRef *corev1.ObjectReference `json:"controlPlaneRef,omitempty"`
}

// NestedPKIStatus defines the observed state of NestedPKI
type NestedPKIStatus struct {
	// Ready is set if all resources have been created
	// +kubebuilder:default=false
	Ready bool `json:"ready,omitempty"`

	// RootCASecretName defines the
	// +optional
	RootCASecretName string `json:"rootCASecretName,omitempty"`

	// EtcdSecretName defines the name of the secret after it's been created
	// +optional
	EtcdSecretName string `json:"etcdSecretName,omitempty"`

	// APIServerCASecretName defines the name of the secret after it's been
	// created
	// +optional
	APIServerSecretName string `json:"apiServerSecretName,omitempty"`

	// AdminKubeconfigSecretName defines the name of the secret after it's been
	// created
	// +optional
	AdminKubeconfigSecretName string `json:"adminKubeconfigSecretName,omitempty"`

	// ControllerManagerKubeconfigSecretName defines the name of the secret after it's been
	// created
	// +optional
	ControllerManagerKubeconfigSecretName string `json:"controllerManagerKubeconfigSecretName,omitempty"`
}

// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,categories=cluster-api;capn;capi,shortName=np

// NestedPKI is the Schema for the nestedpkis API
type NestedPKI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NestedPKISpec   `json:"spec,omitempty"`
	Status NestedPKIStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NestedPKIList contains a list of NestedPKI
type NestedPKIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NestedPKI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NestedPKI{}, &NestedPKIList{})
}
