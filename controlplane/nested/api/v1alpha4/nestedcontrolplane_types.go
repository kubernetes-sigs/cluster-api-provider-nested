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

package v1alpha4

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// NestedControlPlaneFinalizer is added to the NestedControlPlane to allow
	// nested deletions to happen before the object is cleaned up
	NestedControlPlaneFinalizer = "nested.controlplane.cluster.x-k8s.io"
)

// NestedControlPlaneSpec defines the desired state of NestedControlPlane
type NestedControlPlaneSpec struct {
	// EtcdRef is the reference to the NestedEtcd
	EtcdRef *corev1.ObjectReference `json:"etcd,omitempty"`

	// APIServerRef is the reference to the NestedAPIServer
	// +optional
	APIServerRef *corev1.ObjectReference `json:"apiserver,omitempty"`

	// ContollerManagerRef is the reference to the NestedControllerManager
	// +optional
	ControllerManagerRef *corev1.ObjectReference `json:"controllerManager,omitempty"`
}

// NestedControlPlaneStatus defines the observed state of NestedControlPlane
type NestedControlPlaneStatus struct {
	// Etcd stores the connection information from the downstream etcd
	// implementation if the NestedEtcd type isn't used this
	// allows other component controllers to fetch the endpoints.
	// +optional
	Etcd *NestedControlPlaneStatusEtcd `json:"etcd,omitempty"`

	// APIServer stores the connection information from the control plane
	// this should contain anything shared between control plane components
	// +optional
	APIServer *NestedControlPlaneStatusAPIServer `json:"apiserver,omitempty"`

	// Initialized denotes whether or not the control plane finished initializing.
	// +optional
	Initialized bool `json:"initialized"`

	// Ready denotes that the NestedControlPlane API Server is ready to
	// receive requests
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// ErrorMessage indicates that there is a terminal problem reconciling the
	// state, and will be set to a descriptive error message.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions specifies the conditions for the managed control plane
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// NestedControlPlaneStatusEtcd defines the status of the etcd component to
// allow other component controllers to take over the deployment
type NestedControlPlaneStatusEtcd struct {
	// Addresses defines how to address the etcd instance
	Addresses []NestedEtcdAddress `json:"addresses,omitempty"`
}

// NestedControlPlaneStatusAPIServer defines the status of the APIServer
// component, this allows the next set of component controllers to take over
// the deployment
type NestedControlPlaneStatusAPIServer struct {
	// ServiceCIDRs which is provided to kube-apiserver and kube-controller-manager
	// +optional
	ServiceCIDR string `json:"serviceCidr,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced,shortName=ncp,categories=capi;capn
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:subresource:status

// NestedControlPlane is the Schema for the nestedcontrolplanes API
type NestedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NestedControlPlaneSpec   `json:"spec,omitempty"`
	Status NestedControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NestedControlPlaneList contains a list of NestedControlPlane
type NestedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NestedControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NestedControlPlane{}, &NestedControlPlaneList{})
}

// GetOwnerCluster is a utility to return the owning clusterv1.Cluster
func (r *NestedControlPlane) GetOwnerCluster(ctx context.Context, cli client.Client) (cluster *clusterv1.Cluster, err error) {
	return util.GetOwnerCluster(ctx, cli, r.ObjectMeta)
}

func (r *NestedControlPlane) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

func (r *NestedControlPlane) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}
