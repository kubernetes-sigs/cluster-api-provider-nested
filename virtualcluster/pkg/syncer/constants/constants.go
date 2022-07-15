/*
Copyright 2019 The Kubernetes Authors.

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

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// LabelControlled records the object is controlled by virtualcluster controllers.
	LabelControlled = "tenancy.x-k8s.io/controlled"
	// LabelCluster records which cluster this resource belongs to.
	LabelCluster = "tenancy.x-k8s.io/cluster"
	// LabelUID is the uid in the tenant namespace.
	LabelUID = "tenancy.x-k8s.io/uid"
	// LabelNamespace records which cluster namespace this resource belongs to.
	LabelNamespace = "tenancy.x-k8s.io/namespace"
	// LabelOwnerReferences is the ownerReferences of the object in tenant context.
	LabelOwnerReferences = "tenancy.x-k8s.io/ownerReferences"
	// LabelClusterIP is the cluster ip of the corresponding service in tenant namespace.
	LabelClusterIP = "tenancy.x-k8s.io/clusterIP"
	// LabelSecretName is the service account token secret name in tenant namespace.
	LabelSecretName = "tenancy.x-k8s.io/secret.name" // #nosec G101 -- This is a label key
	// LabelAdminKubeConfig is the kubeconfig in base64 format for tenant control plane.
	LabelAdminKubeConfig = "tenancy.x-k8s.io/admin-kubeconfig"
	// LabelSecretAdminKubeConfig is the kubeconfig secret name for the tenant control plane.
	LabelSecretAdminKubeConfig = "tenancy.x-k8s.io/secret.admin-kubeconfig" // #nosec G101 -- This is a label key
	// LabelVCName is the name of the VC CR that owns the object.
	LabelVCName = "tenancy.x-k8s.io/vcname"
	// LabelVCNamespace is the namespace of the VC CR that owns the object.
	LabelVCNamespace = "tenancy.x-k8s.io/vcnamespace"
	// LabelVCUID is the uid of the VC CR that owns the object.
	LabelVCUID = "tenancy.x-k8s.io/vcuid"
	// LabelVCRootNS means the namespace is the rootns created by vc-manager.
	LabelVCRootNS = "tenancy.x-k8s.io/vcrootns"

	// LabelExternalApiserverDomain is the domain name for apiserver url from outside the cluster
	LabelExternalApiserverDomain = "tenancy.x-k8s.io/external-apiserver-domain"

	// LabelSecretUID is the service account token secret UID in tenant namespace.
	LabelSecretUID = "tenancy.x-k8s.io/secret.UID" // #nosec G101 -- This is a label key

	// UwsControllerWorkersHigh is the quantity of the worker routine for a resource that generates high number of uws requests.
	UwsControllerWorkerHigh = 10
	// UwsControllerWorkersLow is the quantity of the worker routine for a resource that generates low number of uws requests.
	UwsControllerWorkerLow = 3

	// DwsControllerWorkersHigh is the quantity of the worker routine for a resource that generates high number of dws requests.
	DwsControllerWorkerHigh = 10
	// DwsControllerWorkersLow is the quantity of the worker routine for a resource that generates low number of dws requests.
	DwsControllerWorkerLow = 3

	TenantDNSServerNS          = "kube-system"
	TenantDNSServerServiceName = "kube-dns"

	// TenantDisableDNSPolicyMutation is a label that allows pods to stop the syncer from mutating the dnsPolicy
	TenantDisableDNSPolicyMutation = "tenancy.x-k8s.io/disable.dnsPolicyMutation"

	// PublicObjectKey is a label key which marks the super control plane object that should be populated to every tenant control plane.
	PublicObjectKey = "tenancy.x-k8s.io/super.public"

	LabelVirtualNode = "tenancy.x-k8s.io/virtualnode"
	// LabelSuperClusterID is a label key added to the vNode object in tenant when SuperClusterPooling feature is enabled.
	LabelSuperClusterID = "tenancy.x-k8s.io/superclusterid"

	// DefaultvNodeGCGracePeriod is the grace period of time before deleting an orphan vNode in tenant control plane.
	DefaultvNodeGCGracePeriod = time.Second * 120

	DefaultOpaqueMetaPrefix      = "tenancy.x-k8s.io"
	DefaultTransparentMetaPrefix = "transparency.tenancy.x-k8s.io"

	// LabelSuperClusterIP is used to inform the tenant service about the cluster IP used in super control plane.
	LabelSuperClusterIP = "transparency.tenancy.x-k8s.io/clusterIP"

	KubeconfigAdminSecretName = "admin-kubeconfig" // #nosec G101 -- This is a secret name
)

const (
	// TODO(zhuangqh): make extend info plugable
	// LabelExtendDeploymentName is the parent deployment name of pod. only take effect on pods.
	LabelExtendDeploymentName = "tenancy.x-k8s.io/extend.deployment.name"
	// LabelExtendDeploymentUID is the parent deployment uid of pod. only take effect on pods.
	LabelExtendDeploymentUID = "tenancy.x-k8s.io/extend.deployment.uid"
)

var DefaultDeletionPolicy = metav1.DeletePropagationBackground
