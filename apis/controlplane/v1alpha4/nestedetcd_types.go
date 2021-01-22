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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

// NestedEtcdSpec defines the desired state of NestedEtcd
type NestedEtcdSpec struct {
	// NestedComponentSpec contains the common and user-specified information that are
	// required for creating the component
	// +optional
	NestedComponentSpec `json:",inline"`
}

// NestedEtcdStatus defines the observed state of NestedEtcd
type NestedEtcdStatus struct {
	// Ready is set if all resources have been created
	Ready bool `json:"ready,omitempty"`

	// EtcdDomain defines how to address the etcd instance
	Addresses []NestedEtcdAddress `json:"addresses,omitempty"`

	// CommonStatus allows addons status monitoring
	addonv1alpha1.CommonStatus `json:",inline"`
}

// EtcdAddress defines the observed addresses for etcd
type NestedEtcdAddress struct {
	// IP Address of the etcd instance.
	// +optional
	IP string `json:"ip,omitempty"`

	// Hostname of the etcd instance
	Hostname string `json:"hostname,omitempty"`

	// Port of the etcd instance
	// +optional
	Port int32 `json:"port"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NestedEtcd is the Schema for the nestedetcds API
type NestedEtcd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NestedEtcdSpec   `json:"spec,omitempty"`
	Status NestedEtcdStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NestedEtcdList contains a list of NestedEtcd
type NestedEtcdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NestedEtcd `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NestedEtcd{}, &NestedEtcdList{})
}
