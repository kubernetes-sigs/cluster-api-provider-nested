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

package constants

var (
	// ControlPlanePrefix defines the top level domain for all labels and
	// finalizers
	ControlPlanePrefix = "controlplane.cluster.x-k8s.io"

	// LabelNestedControlPlaneName defines the name of the control plane
	// resource
	LabelNestedControlPlaneName = ControlPlanePrefix + "/name"

	// LabelNestedControlPlaneNamespace defines the namespace of the control
	// plane resource
	LabelNestedControlPlaneNamespace = ControlPlanePrefix + "/namespace"

	// LabelNestedControlPlaneUID defines the uid of the control plane resource
	LabelNestedControlPlaneUID = ControlPlanePrefix + "/uid"

	// LabelNestedControlPlaneRootNamespace defines if this namespace is the
	// root for the cluster, meaning the control plane is run from here.
	LabelNestedControlPlaneRootNamespace = ControlPlanePrefix + "/namespace.root"

	// NestedControlPlaneTemplateFinalizer defines the finalizer that is used
	// to manage garbage collection and ensure no ControlPlanes still exist.
	NestedControlPlaneTemplateFinalizer = ControlPlanePrefix + "/nestedcontrolplanetemplates.finalizer"

	// NestedControlPlaneFinalizer defines the finalizer that is used to manage
	// garbage collection and ensure resources are torn down before control
	// planes are deleted.
	NestedControlPlaneFinalizer = ControlPlanePrefix + "/nestedcontrolplanes.finalizer"

	// NestedPKIFinalizer defines the finalizer that is used to manage garbage
	// collection and ensures resources are torn down before control plane are
	// deleted
	NestedPKIFinalizer = ControlPlanePrefix + "/nestedpkis.finalizer"

	// NestedEtcdFinalizer defines the finalizer that is used to manage garbage
	// collection and ensures resources are torn down before control plane are
	// deleted
	NestedEtcdFinalizer = ControlPlanePrefix + "/nestedetcds.finalizer"

	// NestedAPIServerFinalizer defines the finalizer that is used to manage
	// garbage collection and ensures resources are torn down before control
	// plane are deleted
	NestedAPIServerFinalizer = ControlPlanePrefix + "/nestedapiservers.finalizer"

	// NestedControllerManagerFinalizer defines the finalizer that is used to
	// manage garbage collection and ensures resources are torn down before
	// control plane are deleted
	NestedControllerManagerFinalizer = ControlPlanePrefix + "/nestedcontrollermanagers.finalizer"
)
