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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "sigs.k8s.io/cluster-addons/installer/pkg/apis/config/v1alpha1"
)

// NestedControlPlaneTemplateSpec defines the desired state of NestedControlPlaneTemplate
type NestedControlPlaneTemplateSpec struct {
	// KubernetesVersion allows you to specify the Kubernetes version this
	// template will provision
	KubernetesVersion KubernetesVersion `json:"version"`

	// Components allows you to specify the multiple components necessary to
	// provision a Virtual Control Plane
	// +optional
	Components []configv1alpha1.Addon `json:"components,omitempty"`

	// Default allows you to set this NestedTemplate as a a default template,
	// by setting this to true any new NestedClusters will be created without
	// a TemplateRef will be deployed with this template.
	Default bool `json:"default,omitempty"`
}

// ComponentName sets the string type for the component map
type ComponentName string

const (
	EtcdComponent              ComponentName = "etcd"
	APIServerComponent         ComponentName = "apiserver"
	ControllerManagerComponent ComponentName = "controller-manager"
)

// KubernetesVersion defines the desired state of the Kubernetes version
type KubernetesVersion struct {
	// Major allows you to specify the major Kubernetes version
	// +kubebuilder:default="1"
	Major string `json:"major"`

	// Minor allows you to specify the minor Kubernetes version
	Minor string `json:"minor"`

	// Patch allows you to specify the patch Kubernetes version
	Patch string `json:"patch"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=cluster-api;capn;capi,shortName=ncpt

// NestedControlPlaneTemplate is the Schema for the nestedcontrolplanetemplates API
type NestedControlPlaneTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NestedControlPlaneTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// NestedControlPlaneTemplateList contains a list of NestedControlPlaneTemplate
type NestedControlPlaneTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NestedControlPlaneTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NestedControlPlaneTemplate{}, &NestedControlPlaneTemplateList{})
}
