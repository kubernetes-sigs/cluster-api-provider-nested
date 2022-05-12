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

package conversion

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	v1scheduling "k8s.io/api/scheduling/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/apis/config"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	mc "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/mccontroller"
)

// ToClusterKey makes a unique key which is used to create the root namespace in super master for a virtual cluster.
// To avoid name conflict, the key uses the format <namespace>-<hash>-<name>
func ToClusterKey(vc *v1alpha1.VirtualCluster) string {
	// If the ClusterNamespace is set then this will automatically return that prefix allowing us to override
	// any other hooks for the ClusterNamespace.
	if vc.Status.ClusterNamespace != "" {
		return vc.Status.ClusterNamespace
	}
	digest := sha256.Sum256([]byte(vc.GetUID()))
	return vc.GetNamespace() + "-" + hex.EncodeToString(digest[0:])[0:6] + "-" + vc.GetName()
}

func ToSuperClusterNamespace(cluster, ns string) string {
	targetNamespace := strings.Join([]string{cluster, ns}, "-")
	if len(targetNamespace) > validation.DNS1123SubdomainMaxLength {
		digest := sha256.Sum256([]byte(targetNamespace))
		return targetNamespace[0:57] + "-" + hex.EncodeToString(digest[0:])[0:5]
	}
	return targetNamespace
}

// GetVirtualNamespace is used to find the corresponding namespace in tenant master for objects created in super master originally, e.g., events.
func GetVirtualNamespace(nsLister listersv1.NamespaceLister, pNamespace string) (cluster, namespace string, err error) {
	vcInfo, err := nsLister.Get(pNamespace)
	if err != nil {
		return
	}

	if v, ok := vcInfo.GetAnnotations()[constants.LabelCluster]; ok {
		cluster = v
	}
	if v, ok := vcInfo.GetAnnotations()[constants.LabelNamespace]; ok {
		namespace = v
	}
	return
}

func GetVirtualOwner(meta client.Object) (cluster, namespace string) {
	cluster = meta.GetAnnotations()[constants.LabelCluster]
	namespace = meta.GetAnnotations()[constants.LabelNamespace]
	return cluster, namespace
}

func GetKubeConfigOfVC(c v1core.CoreV1Interface, vc *v1alpha1.VirtualCluster) ([]byte, error) {
	if adminKubeConfig, exists := vc.GetAnnotations()[constants.LabelAdminKubeConfig]; exists {
		decoded, err := base64.StdEncoding.DecodeString(adminKubeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to decode kubeconfig from annotations %s: %v", constants.LabelAdminKubeConfig, err)
		}
		return decoded, nil
	}

	// If VC has the Kubeconfig Secret Name Annotation, load the kubeconfig from there.
	secretName := constants.KubeconfigAdminSecretName
	secretFieldName := constants.KubeconfigAdminSecretName
	if adminKubeConfigName, exists := vc.GetAnnotations()[constants.LabelSecretAdminKubeConfig]; exists {
		secretName = adminKubeConfigName
		secretFieldName = "value"
	}

	clusterName := ToClusterKey(vc)
	adminKubeConfigSecret, err := c.Secrets(clusterName).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret (%s) for virtual cluster in root namespace %s: %v", constants.KubeconfigAdminSecretName, clusterName, err)
	}
	return adminKubeConfigSecret.Data[secretFieldName], nil
}

type objectConversion struct {
	config *config.SyncerConfiguration
	mcc    mc.MultiClusterInterface
}

// Convertor implement the Conversion interface.
func Convertor(syncerConfig *config.SyncerConfiguration, mcc mc.MultiClusterInterface) Conversion {
	return &objectConversion{config: syncerConfig, mcc: mcc}
}

type Conversion interface {
	BuildSuperClusterObject(cluster string, obj client.Object) (client.Object, error)
	BuildSuperClusterNamespace(cluster string, obj client.Object) (client.Object, error)
}

func (c *objectConversion) BuildSuperClusterObject(cluster string, obj client.Object) (client.Object, error) {
	m, err := c.buildCleanSuperClusterObject(cluster, obj)
	if err != nil {
		return nil, err
	}

	vcName, vcNS, _, err := c.mcc.GetOwnerInfo(cluster)
	if err != nil {
		return nil, errors.Wrapf(err, "get cluster owner info")
	}

	ownerReferencesStr, err := json.Marshal(obj.GetOwnerReferences())
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal owner references")
	}

	var tenantScopeMetaInAnnotation = map[string]string{
		constants.LabelCluster:         cluster,
		constants.LabelUID:             string(obj.GetUID()),
		constants.LabelOwnerReferences: string(ownerReferencesStr),
		constants.LabelNamespace:       obj.GetNamespace(),
		constants.LabelVCName:          vcName,
		constants.LabelVCNamespace:     vcNS,
	}

	anno := m.GetAnnotations()
	if anno == nil {
		anno = make(map[string]string)
	}
	for k, v := range tenantScopeMetaInAnnotation {
		anno[k] = v
	}
	m.SetAnnotations(anno)

	labels := m.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	var tenantScopeMetaInLabel = map[string]string{
		constants.LabelVCName:      vcName,
		constants.LabelVCNamespace: vcNS,
	}
	for k, v := range tenantScopeMetaInLabel {
		labels[k] = v
	}
	m.SetLabels(labels)

	m.SetNamespace(ToSuperClusterNamespace(cluster, obj.GetNamespace()))

	return m, nil
}

func (c *objectConversion) CleanOpaqueKeys(vc *v1alpha1.VirtualCluster, keyMap map[string]string) {
	var exceptionsList []string
	if vc != nil {
		exceptions := sets.NewString()
		exceptions.Insert(vc.Spec.OpaqueMetaPrefixes...)
		exceptions.Insert(constants.DefaultOpaqueMetaPrefix, constants.DefaultTransparentMetaPrefix)
		exceptionsList = exceptions.UnsortedList()
	}

	for k := range keyMap {
		if hasPrefixInArray(k, exceptionsList) || isOpaquedKey(c.config, k) {
			delete(keyMap, k)
		}
	}
}

func WithSuperClusterLabels(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[constants.LabelControlled] = "true"
	return labels
}

func (c *objectConversion) BuildSuperClusterNamespace(cluster string, obj client.Object) (client.Object, error) {
	m, err := c.buildCleanSuperClusterObject(cluster, obj)
	if err != nil {
		return nil, err
	}

	vcName, vcNamespace, vcUID, err := c.mcc.GetOwnerInfo(cluster)
	if err != nil {
		return nil, errors.Wrapf(err, "get cluster owner info")
	}

	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterLabelling) {
		m.SetLabels(WithSuperClusterLabels(m.GetLabels()))
	}

	anno := m.GetAnnotations()
	if anno == nil {
		anno = make(map[string]string)
	}
	anno[constants.LabelCluster] = cluster
	anno[constants.LabelUID] = string(obj.GetUID())
	anno[constants.LabelNamespace] = obj.GetName()
	// We put owner information in annotation instead of  metav1.OwnerReference because vc is a namespace scope resource
	// and metav1.OwnerReference does not provide namespace field. The owner information is needed for super master ns gc.
	anno[constants.LabelVCName] = vcName
	anno[constants.LabelVCNamespace] = vcNamespace
	anno[constants.LabelVCUID] = vcUID
	m.SetAnnotations(anno)

	m.SetName(ToSuperClusterNamespace(cluster, obj.GetName()))

	return m, nil
}

func (c *objectConversion) buildCleanSuperClusterObject(cluster string, obj client.Object) (client.Object, error) {
	target := obj.DeepCopyObject()
	accessor, err := meta.Accessor(target)
	if err != nil {
		return nil, err
	}
	m := accessor.(client.Object)

	vc, err := util.GetVirtualClusterObject(c.mcc, cluster)
	if err != nil {
		return nil, errors.Wrapf(err, "get virtualcluster")
	}

	c.CleanOpaqueKeys(vc, m.GetLabels())
	c.CleanOpaqueKeys(vc, m.GetAnnotations())

	ResetMetadata(m)

	return target.(client.Object), nil
}

func ResetMetadata(obj metav1.Object) {
	obj.SetSelfLink("")
	obj.SetUID("")
	obj.SetResourceVersion("")
	obj.SetGeneration(0)
	obj.SetDeletionTimestamp(nil)
	obj.SetDeletionGracePeriodSeconds(nil)
	obj.SetOwnerReferences(nil)
	obj.SetFinalizers(nil)
	obj.SetClusterName("")
}

func BuildVirtualEvent(cluster string, pEvent *v1.Event, vObj client.Object) *v1.Event {
	vEvent := pEvent.DeepCopy()
	ResetMetadata(vEvent)
	vEvent.SetNamespace(vObj.GetNamespace())
	vEvent.InvolvedObject.Namespace = vObj.GetNamespace()
	vEvent.InvolvedObject.UID = vObj.GetUID()
	vEvent.InvolvedObject.ResourceVersion = ""

	vEvent.Message = strings.ReplaceAll(vEvent.Message, cluster+"-", "")
	vEvent.Message = strings.ReplaceAll(vEvent.Message, cluster, "")

	return vEvent
}

func BuildVirtualStorageClass(cluster string, pStorageClass *storagev1.StorageClass) *storagev1.StorageClass {
	vStorageClass := pStorageClass.DeepCopy()
	ResetMetadata(vStorageClass)
	return vStorageClass
}

func BuildVirtualPriorityClass(cluster string, pPriorityClass *v1scheduling.PriorityClass) *v1scheduling.PriorityClass {
	vPriorityClass := pPriorityClass.DeepCopy()
	ResetMetadata(vPriorityClass)
	return vPriorityClass
}

func BuildVirtualCRD(cluster string, pCRD *v1beta1.CustomResourceDefinition) *v1beta1.CustomResourceDefinition {
	vCRD := pCRD.DeepCopy()
	ResetMetadata(vCRD)
	return vCRD
}

func BuildVirtualPersistentVolume(pPV *v1.PersistentVolume, vPVC *v1.PersistentVolumeClaim) *v1.PersistentVolume {
	vPV := pPV.DeepCopy()
	ResetMetadata(vPV)
	// The pv needs to bind with the vPVC
	vPV.Spec.ClaimRef.Namespace = vPVC.Namespace
	vPV.Spec.ClaimRef.UID = vPVC.UID
	return vPV
}

// IsControlPlaneService will return if the namespacedName matches the proper
// NamespacedName in the tenant control plane
func IsControlPlaneService(service *v1.Service, cluster string) bool {
	kubernetesNamespace := ToSuperClusterNamespace(cluster, metav1.NamespaceDefault)
	kubernetesService := "kubernetes"

	// If the super cluster service networking is enabled this supports allowing
	// the "real" apiserver-svc to propagate to the tenant default/kubernetes service
	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterServiceNetwork) {
		kubernetesNamespace = cluster
		kubernetesService = "apiserver-svc"
	}

	// If the super cluster pooling is enabled, the service in tenant default namepsace
	// is used.
	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterPooling) {
		kubernetesNamespace = metav1.NamespaceDefault
		kubernetesService = "kubernetes"
	}
	return service.Namespace == kubernetesNamespace && service.Name == kubernetesService
}
