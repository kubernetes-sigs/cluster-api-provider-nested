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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

// NestedAPIServerSpec defines the desired state of NestedAPIServer
type NestedAPIServerSpec struct {
	// NestedComponentSpec contains the common and user-specified information that are
	// required for creating the component
	// +optional
	NestedComponentSpec `json:",inline"`
}

// NestedAPIServerStatus defines the observed state of NestedAPIServer
type NestedAPIServerStatus struct {
	// APIServerService is the reference to the service that expose the APIServer
	// +optional
	APIServerService *corev1.ObjectReference `json:"apiserverService,omitempty"`

	// CommonStatus allows addons status monitoring
	addonv1alpha1.CommonStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced,path=nestedapiservers,shortName=nkas
//+kubebuilder:categories=capi,capn
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:subresource:status

// NestedAPIServer is the Schema for the nestedapiservers API
type NestedAPIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NestedAPIServerSpec   `json:"spec,omitempty"`
	Status NestedAPIServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NestedAPIServerList contains a list of NestedAPIServer
type NestedAPIServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NestedAPIServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NestedAPIServer{}, &NestedAPIServerList{})
}
