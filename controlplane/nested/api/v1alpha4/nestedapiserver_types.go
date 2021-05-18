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
//+kubebuilder:resource:scope=Namespaced,shortName=nkas,categories=capi;capn
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

var _ addonv1alpha1.CommonObject = &NestedAPIServer{}
var _ addonv1alpha1.Patchable = &NestedAPIServer{}

// ComponentName returns the name of the component for use with
// addonv1alpha1.CommonObject
func (c *NestedAPIServer) ComponentName() string {
	return string(APIServer)
}

// CommonSpec returns the addons spec of the object allowing common funcs like
// Channel & Version to be usable
func (c *NestedAPIServer) CommonSpec() addonv1alpha1.CommonSpec {
	return c.Spec.CommonSpec
}

// GetCommonStatus will return the common status for checking is a component
// was successfully deployed
func (c *NestedAPIServer) GetCommonStatus() addonv1alpha1.CommonStatus {
	return c.Status.CommonStatus
}

// SetCommonStatus will set the status so that abstract representations can set
// Ready and Phases
func (c *NestedAPIServer) SetCommonStatus(s addonv1alpha1.CommonStatus) {
	c.Status.CommonStatus = s
}

// PatchSpec returns the patches to be applied
func (c *NestedAPIServer) PatchSpec() addonv1alpha1.PatchSpec {
	return c.Spec.PatchSpec
}
