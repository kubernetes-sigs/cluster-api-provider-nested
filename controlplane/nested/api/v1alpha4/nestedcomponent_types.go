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
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

// NestedComponentSpec defines the common fields for nested components.
type NestedComponentSpec struct {
	// NestedComponentSpec defines the common information for creating the
	// component.
	// +optional
	addonv1alpha1.CommonSpec `json:",inline"`

	// PatchSpecs includes the user specified settings.
	// +optional
	addonv1alpha1.PatchSpec `json:",inline"`

	// Resources defines the amount of computing resources that will be used
	// by this component.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Replicas defines the number of replicas in the component's workload.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

// ComponentPhase defines the state of the component.
type ComponentPhase string

const (
	// Ready defines that the component is available and control plane can
	// continue provisioning.
	Ready ComponentPhase = "Ready"

	// Unready defines that the component is unavailable.
	Unready ComponentPhase = "Unready"
)

// ComponentKind defines the available kinds.
type ComponentKind string

const (
	// APIServer defines the Kind name for the apiserver.
	APIServer ComponentKind = "NestedAPIServer"
	// Etcd defines the Kind name for the etcd.
	Etcd ComponentKind = "NestedEtcd"
	// ControllerManager defines the kind name for the controller-manager.
	ControllerManager ComponentKind = "NestedControllerManager"
)
