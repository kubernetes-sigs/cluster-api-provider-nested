---
title: Creating Control Plane Components
authors:
  - "@charleszheng44"
reviewers:
  - "@christopherhein"
  - "@Fei-Guo"
  - "@vincepri"
  - "@brightzheng100"
creation-date: 2020-10-26
last-updated: 2020-11-09
status: provisional
---

# Creating Control Plane Components

## Table of Contents

* [Creating Control Plane Components](#creating-control-plane-components)
   * [Table of Contents](#table-of-contents)
   * [Glossary](#glossary)
   * [Summary](#summary)
   * [Motivation](#motivation)
      * [Goals](#goals)
      * [Non-Goals](#non-goals)
   * [Proposal](#proposal)
      * [Portability and Customizability](#portability-and-customizability)
      * [Bootstrap](#bootstrap)
         * [Create prerequisites](#create-prerequisites)
      * [Creation](#creation)
         * [In-tree](#in-tree)
         * [Using out-of-tree provisioners](#using-out-of-tree-provisioners)
      * [Control Plane Custom Resources](#control-plane-custom-resources)
      * [NestedEtcd CRD](#nestedetcd-crd)
      * [NestedAPIServer CRD](#nestedapiserver-crd)
      * [NestedControllerManager CRD](#nestedcontrollermanager-crd)
      * [Security Model](#security-model)
   * [Implementation History](#implementation-history)

## Glossary

Refer to the [CAPN Glossary](https://github.com/kubernetes-sigs/cluster-api-provider-nested/blob/master/proposals/00_capn-glossary.md).

## Summary

The goal of this proposal is to define CRDs of the three major components (kube-apiserver~(KAS), Etcd, kube-controller-manager~(KCM)) of the NCP, and a standard process of creating them.

## Motivation

CAPN aims at providing control plane level isolation while sharing physical resources among control planes. There exist various approaches to creating isolated control planes. For example, one can run components of the nested control plane as pods on the underlying clusters, create NCPs through cloud providers' Kubernetes services or use out-of-tree component controllers to create each component. In this proposal, we try to define CRDs of the NCP's three major components and a standard process of creating the three components regardless of which underlying approach is used. As examples, we introduce two setups that 1) creating each component natively, 2) creating KAM and KCM natively while using the [Etcd-cluster-operator](https://github.com/improbable-eng/etcd-cluster-operator) to create the Etcd.

### Goals

- Define the CRD that represents each control plane component. The CRD needs to meet two requirements:
    * Portable - the CRD should hold information that is required by different component controllers, e.g., [etcdadm](https://github.com/kubernetes-sigs/etcdadm), [etcd-operator](https://github.com/coreos/etcd-operator), and [etcd-cluster-operator](https://github.com/improbable-eng/etcd-cluster-operator/blob/f84abc6561735814debd67d45bb62d2d2ed8cf4a/api/v1alpha1/etcdcluster_types.go#L31-L47)
    * Customizable - the CRD should allow end-users to customize each component, i.e., specify the image, component version, and command-line options.

- Define a standard process of creating control plane components for NCP.

- Support independently creating/updating each component

### Non-Goals

- Define how NCP controller works.

- Discuss the implementation details of the out-of-tree component controllers.

## Proposal

### Portability and Customizability

Generally, creating the three major components requires similar high-level information, like the components' version, the number of replicas, and the amount of computing resources. Meanwhile, end-users should be able to customize NCP components, i.e., specifying the component image, version, and command-line options. Therefore, we define a new struct `NestedComponentSpec` that contains common information required by different providers as well as customized information specified by the end-users. The `NestedComponentSpec` will look like the following

```go
type NestedComponentSpec struct {
    // NestedComponentSpec defines the common information for creating the component
    // +optional
    addonv1alpha1.CommonSpec `json:",inline"`

    // PatchSpecs includes the user specifed settings
    // +optional
    addonv1alpha1.PatchSpec `json:",inline"`

    // Resources defines the amount of computing resources that will be used by this component
    // +optional
    Resources corev1.ResourceRequirements `json:"resources",omitempty`
    
    // Replicas defines the number of replicas in the component's workload 
    // +optional
    Replicas int32 `json:"replicas",omitempty`
}
```

The `CommonSpecs` and the `PatchSpec` are defined in [kubebuilder-declarative-pattern](https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern/blob/1cbf859290cab81ae8e73fc5caebe792280175d1/pkg/patterns/addon/pkg/apis/v1alpha1/common_types.go):


```go
// CommonSpec defines the set of configuration attributes that must be exposed on all addons.
type CommonSpec struct {
    // Version specifies the exact addon version to be deployed, eg 1.2.3
    // It should not be specified if Channel is specified
    Version string `json:"version,omitempty"`
    // Channel specifies a channel that can be used to resolve a specific addon, eg: stable
    // It will be ignored if Version is specified
    Channel string `json:"channel,omitempty"`
}

// +k8s:deepcopy-gen=true
type PatchSpec struct {
    Patches []*runtime.RawExtension `json:"patches,omitempty"`
}
```

### Bootstrap

#### Create prerequisites 

We assume that the APIServer, ContollerManager, Etcd, and the NCP CR are located in the same namespace. To create an NCP, we need first to create NestedAPIserver CR, NestedControllerManager CR, NestedEtcd CR, NCP CR, and a namespace that holds all the CRs, then the component controller can cooperate to create components for the NCP. 

As there exist dependencies between components, i.e., KAS cannot run without Etcd, KCM cannot work without KAS, when creating NCP components, component controllers will need to get information and status of other CRs. To achieve this, we will add three `ObjectReference` to the `NestedControlPlaneSpec` with each `ObjectReference` points to a component.

```go
type NestedControlPlaneSpec struct {
    // other fields ...
    
    // EtcdRef is the eference to the NestedEtcd 
    EtcdRef *corev1.ObjectReference `json:"etcd,omitempty"` 
    
    // APIServerRef is the reference to the NestedAPIServer 
    APIServerRef *corev1.ObjectReference `json:"apiserver,omitempty"` 
    
    // ContollerManagerRef is the reference to the NestedControllerManager
    ControllerManagerRef *corev1.ObjectReference `json:"controllerManager,omitempty"`
}
```

After applying the NCP CR, the NCP controller will find the three associated components and set their `metav1.OwnerReference` as the NCP CR. Then, the component controller can find other CRs through the owner NCP, when creating the corresponding component workload.

### Creation

End-users can create component CRs manually and apply them to the cluster with an NCP to create the resources. In the future, we might introduce the `Template` CR, which will handle the creation of the component CRs in it's controller. We assume that there will be only one component controller for each component at any given time, and it is the cluster administrator's responsibility to set up the proper component controllers.

#### In-tree

The component controller will create the component under the in-tree mode, which will create the component using the default manifests. The readiness and liveness probe will be used, and we will mark each component as ready only when the corresponding workload is ready. As the KAS cannot work without available Etcd and the KCM cannot run without KAS, the three components need to be created by their respective controllers in the order of Etcd, KAS, and KCM. Creation order is maintained using cross resource status, which checks and wait until the dependencies are provisioned. We will host sets of default templates in this repository. Users can specify which set of templates they intend to use by specifying the corresponding `version` or `channel` in the embedded `CommonSpec` in the component's CR.

Each component's controller will generate necessary certificates for the component and store them to the [secret resources](https://cluster-api.sigs.k8s.io/tasks/certs/using-custom-certificates.html) defined by CAPI. Also, The KAS controller will store the content of the kubeconfig file in a secret named `[clustername]-kubeconfig`.

![Control Plane Creating Process](in-tree.png)

The creating process will include six steps:

1. The user generates all CRs, i.e., NCP, Etcd, APIServer, ControllerManager, with the same namespace, and apply them.

2. The Etcd controller generates the certificates (including a root CA and TLS serving certificates), creates the Etcd workload, and stores the certificates into `secret/[cluster-name]-etcd`

3. The KAS controller creates a KAS service (for exposing the NCP), generates certificates (including a root CA, TLS serving certificates), creates the KAS workload, stores the certificates into `secret/[cluster-name]-ca`, creates a kubeconfig and stores it into `secret/[cluster-name]-kubeconfig`.

4. The KCM controller generates the KCM kubeconfig and creates the KCM workload.

5. After all the three components are ready, the NCP controller marks the NCP CR as ready.

#### Using out-of-tree provisioners

If users intend to use an external controller to create the NCP component, they may need to implement a new component controller that can interact with the component CR and the external controller to create the component. For example, if the user wanted to use the [etcd-cluster-operator](https://github.com/improbable-eng/etcd-cluster-operator) that requires the [EtcdCluster](https://github.com/improbable-eng/etcd-cluster-operator/blob/master/api/v1alpha1/etcdcluster_types.go) CR. They need to implement a custom controller that watches the `NestedEtcd` resource, creates the necessary CRs for that implementation, and updates the required status fields on `NestedEtcd` to allow dependent services to be provisioned. This can be done using the [kubebuilder-declarative-pattern](https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern) like is done for in-tree component controllers.

![Creating a Control Plane using out-of-tree provisioners](out-of-tree.png)

In the following example, we assume that the user intends to use Etcd-cluster-operator(ECO) as the Etcd controller. The creating process will include seven steps:

1. The cluster administrator deletes the in-tree Etcd controller and deploys the custom Etcd controller (ECO controller).

2. The user generates all CRs and apply them.  

3. The ECO controller creates the EtcdCluster CR.

4. The ECO creates the Etcd workload.

5. At the meantime, the ECO controller keeps watching the EtcdCluster CR, stores the Etcd CA into the `secret/[cluster-name]-etcd`, and updates the `Etcd` CR accordingly.

6. The KAS controller creates the KAS service, generates certificates, creates the KAS workload, stores certificates into `secret/[cluster-name]-ca`, creates the kubeconfig and stores it into the `secret/[cluster-name]-kubeconfig`

7. The KCM controller generates the KCM kubeconfig and creates the KCM workload.

8. Once all the three components are ready, the NCP controller marks the NCP CR as ready.

### Control Plane Custom Resources 

The followings are CRDs of the three components.

### NestedEtcd CRD
```go
// NestedEtcdSpec defines the desired state of Etcd
type NestedEtcdSpec struct {
    // NestedComponentSpec contains the common and user-specified information that are 
    // required for creating the component
    // +optional
    NestedComponentSpec  `json:",inline"`
}

// NestedEtcdStatus defines the observed state of Etcd
type NestedEtcdStatus struct {
    // Ready is set if all resources have been created
    Ready bool `json:"ready,omitempty"`

    // EtcdDomain defines how to address the etcd instance
    Addresses []NestedEtcdAddress `json:"addresses,omitempty"`

    // CommonStatus allows addons status monitoring 
    addonv1alpha1. CommonStatus `json:",inline"`
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

// NestedEtcd is the Schema for the Etcd API
type NestedEtcd struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   EtcdSpec   `json:"spec,omitempty"`
    Status EtcdStatus `json:"status,omitempty"`
}
```

### NestedAPIServer CRD

```go
type NestedAPIServerSpec struct {
    // NestedComponentSpec contains the common and user-specified information that are 
    // required for creating the component
    // +optional
    NestedComponentSpec  `json:",inline"`
}

// NestedAPIServerStatus defines the observed state of APIServer 
type NestedAPIServerStatus struct {
    // Ready is set if all resources have been created
    // +kubebuilder:default=false
    Ready bool `json:"ready,omitempty"`

    // APIServerService is the reference to the service that expose the APIServer 
    // +optional
    APIServerService *corev1.ObjectReference `json:"apiserverService,omitempty"`

    // CommonStatus allows addons status monitoring 
    addonv1alpha1. CommonStatus `json:",inline"`
}

// NestedAPIServer is the Schema for the APIServers API
type NestedAPIServer struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   NestedAPIServerSpec   `json:"spec,omitempty"`
    Status NestedAPIServerStatus `json:"status,omitempty"`
}
```

### NestedControllerManager CRD

```go
// NestedControllerManagerSpec defines the desired state of ControllerManager
type NestedControllerManagerSec struct {
    // NestedComponentSpec contains the common and user-specified information that are 
    // required for creating the component
    // +optional
    NestedComponentSpec  `json:",inline"`
}

// NestedControllerManagerStatus defines the observed state of ControllerManager
type NestedControllerManagerStatus struct {
    // Ready is set if all resources have been created
    Ready bool `json:"ready,omitempty"`

    // CommonStatus allows addons status monitoring 
    addonv1alpha1. CommonStatus `json:",inline"`
}

// NestedControllerManager is the Schema for the ControllerManagers API
type NestedControllerManager struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   NestedControllerManagerSpec   `json:"spec,omitempty"`
    Status NestedControllerManagerStatus `json:"status,omitempty"`
}
```

### Security Model

Creating an NCP requires the end-user to submit a creation request, and the 
cluster administrator will be responsible for creating the NCP CRs and applying 
them. Once the NCP is ready, the cluster administrator will return a kubeconfig 
to the end-user, as each end-user can only access the apiserver assigned to them. 
There is no need to worry about malicious users to manipulate other users' resources. 
A malicious user can still skew the system by creating a massive amount of resources. 
To avoid this, we need to enhance the syncer component; however, this topic is 
beyond this proposal's scope. The proposed mechanism will not lead to any 
severe security issues.

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]
- [ ] MM/DD/YYYY: Compile a Google Doc following the CAEP template (link here)
- [ ] MM/DD/YYYY: First round of feedback from community
- [ ] MM/DD/YYYY: Present proposal at a [community meeting]
- [ ] MM/DD/YYYY: Open proposal PR
