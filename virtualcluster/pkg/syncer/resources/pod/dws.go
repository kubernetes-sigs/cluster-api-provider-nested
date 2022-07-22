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

package pod

import (
	"context"
	"fmt"
	"reflect"
	"time"

	pkgerr "github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/metrics"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	utilconstants "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
)

func (c *controller) StartDWS(stopCh <-chan struct{}) error {
	if !cache.WaitForCacheSync(stopCh, c.podSynced, c.serviceSynced, c.secretSynced) {
		return fmt.Errorf("failed to wait for caches to sync before starting Pod dws")
	}
	return c.MultiClusterController.Start(stopCh)
}

func (c *controller) Reconcile(request reconciler.Request) (res reconciler.Result, retErr error) {
	klog.V(4).Infof("reconcile pod %s/%s for cluster %s", request.Namespace, request.Name, request.ClusterName)
	reconcilestart := time.Now()
	targetNamespace := conversion.ToSuperClusterNamespace(request.ClusterName, request.Namespace)

	pPod, err := c.podLister.Pods(targetNamespace).Get(request.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconciler.Result{Requeue: true}, err
	}

	vPod := &corev1.Pod{}
	if err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vPod); err != nil && !apierrors.IsNotFound(err) {
		return reconciler.Result{Requeue: true}, err
	}

	var operation string
	defer func() {
		recordOperationDuration(operation, reconcilestart)
		recordOperationStatus(operation, retErr)
	}()

	switch {
	case !reflect.DeepEqual(vPod, &corev1.Pod{}) && pPod == nil:
		operation = "pod_add"
		err := c.reconcilePodCreate(request.ClusterName, targetNamespace, request.UID, vPod)
		if err != nil {
			klog.Errorf("failed reconcile Pod %s/%s CREATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)

			if parentRef := getParentRefFromPod(vPod); parentRef != nil {
				c.MultiClusterController.Eventf(request.ClusterName, parentRef, corev1.EventTypeWarning, "FailedCreate", "Error creating: %v", err)
			}
			c.MultiClusterController.Eventf(request.ClusterName, &corev1.ObjectReference{
				Kind:      "Pod",
				Name:      vPod.Name,
				Namespace: vPod.Namespace,
				UID:       vPod.UID,
			}, corev1.EventTypeWarning, "FailedCreate", "Error creating: %v", err)

			return reconciler.Result{Requeue: true}, err
		}
	case reflect.DeepEqual(vPod, &corev1.Pod{}) && pPod != nil:
		operation = "pod_delete"
		err := c.reconcilePodRemove(request.ClusterName, targetNamespace, request.UID, request.Name, pPod)
		if err != nil {
			klog.Errorf("failed reconcile Pod %s/%s DELETE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
		if pPod.Spec.NodeName != "" {
			c.updateClusterVNodePodMap(request.ClusterName, pPod.Spec.NodeName, request.UID, reconciler.DeleteEvent)
		}
	case vPod != nil && pPod != nil:
		operation = "pod_update"
		err := c.reconcilePodUpdate(request.ClusterName, targetNamespace, request.UID, pPod, vPod)
		if err != nil {
			klog.Errorf("failed reconcile Pod %s/%s UPDATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
		if vPod.Spec.NodeName != "" {
			c.updateClusterVNodePodMap(request.ClusterName, vPod.Spec.NodeName, request.UID, reconciler.UpdateEvent)
		}
	default:
		// object is gone.
	}
	return reconciler.Result{}, nil
}

func isPodScheduled(pod *corev1.Pod) bool {
	_, cond := getPodCondition(&pod.Status, corev1.PodScheduled)
	return cond != nil && cond.Status == corev1.ConditionTrue
}

// getPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func getPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return getPodConditionFromList(status.Conditions, conditionType)
}

// getPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func getPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

func getParentRefFromPod(vPod *corev1.Pod) *corev1.ObjectReference {
	if len(vPod.OwnerReferences) == 0 {
		return nil
	}

	owner := vPod.OwnerReferences[0]
	return &corev1.ObjectReference{
		Kind:      owner.Kind,
		Namespace: vPod.Namespace,
		Name:      owner.Name,
		UID:       owner.UID,
	}
}

func (c *controller) reconcilePodCreate(clusterName, targetNamespace, requestUID string, vPod *corev1.Pod) error {
	// load deleting pod, don't create any pod on super control plane.
	if vPod.DeletionTimestamp != nil {
		return nil
	}

	if vPod.Spec.NodeName != "" {
		// For now, we skip vPod that has NodeName set to prevent tenant from deploying DaemonSet or DaemonSet alike CRDs.
		err := c.MultiClusterController.Eventf(clusterName, &corev1.ObjectReference{
			Kind:      "Pod",
			Name:      vPod.Name,
			Namespace: vPod.Namespace,
			UID:       vPod.UID,
		}, corev1.EventTypeWarning, "NotSupported", "The Pod has nodeName set in the spec which is not supported for now")
		return err
	}

	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, vPod)
	if err != nil {
		return err
	}

	pPod := newObj.(*corev1.Pod)

	pSecretMap, err := c.findPodServiceAccountSecret(clusterName, pPod, vPod)
	if err != nil {
		return fmt.Errorf("failed to get service account secret from cluster %s cache: %v", clusterName, err)
	}

	services, err := c.getPodRelatedServices(clusterName, pPod)
	if err != nil {
		return fmt.Errorf("failed to list services from cluster %s cache: %v", clusterName, err)
	}

	nameServer, err := c.getClusterNameServer(clusterName)
	if err != nil {
		return fmt.Errorf("failed to find nameserver: %v", err)
	}

	var ms = []conversion.PodMutator{
		conversion.PodMutateServiceLink(c.Config.DisablePodServiceLinks),
		conversion.PodMutateDefault(vPod, pSecretMap, services, nameServer, c.Config.DNSOptions),
		conversion.PodMutateAutoMountServiceAccountToken(c.Config.DisableServiceAccountToken),
		// TODO: make extension configurable
		// conversion.PodAddExtensionMeta(vPod),
	}

	err = conversion.VC(c.MultiClusterController, clusterName).Pod(pPod).Mutate(ms...)
	if err != nil {
		return fmt.Errorf("failed to mutate pod: %v", err)
	}

	// Validation plugin processing
	if c.plugin != nil {
		pluginstart := time.Now()
		if c.plugin.Enabled() {
			// Serialize pod creation for each tenant
			t := c.plugin.GetTenantLocker(clusterName)
			if t == nil {
				return apierrors.NewBadRequest("cannot get tenant")
			}
			t.Cond.Lock()
			defer t.Cond.Unlock()
			if !c.plugin.Validation(newObj, clusterName) {
				// put pod aside, not to try to create it again.
				klog.Errorf("validation failed for virtual cluster namespace %v, no pod sync", targetNamespace)
				recordOperationDuration("validation_plugin", pluginstart)
				return nil
				// do not requeue return apierrors.NewBadRequest("validation failed for virtual cluster")
			}
		}
		recordOperationDuration("validation_plugin", pluginstart)
	}

	pPod, err = c.client.Pods(targetNamespace).Create(context.TODO(), pPod, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		if pPod.Annotations[constants.LabelUID] == requestUID {
			klog.Infof("pod %s/%s of cluster %s already exist in super control plane", targetNamespace, pPod.Name, clusterName)
		} else {
			klog.Errorf("pPod %s/%s exists but the UID is different from tenant control plane", targetNamespace, pPod.Name)
		}
		return nil
	}

	return err
}

func (c *controller) findPodServiceAccountSecret(clusterName string, pPod, vPod *corev1.Pod) (map[string]string, error) {
	mountSecretSet := sets.NewString()
	for _, volume := range vPod.Spec.Volumes {
		if volume.Secret != nil && !pointer.BoolDeref(volume.Secret.Optional, false) {
			mountSecretSet.Insert(volume.Secret.SecretName)
		}
	}

	// vSecretName -> pSecretName
	mutateNameMap := make(map[string]string)

	for secretName := range mountSecretSet {
		vSecret := &corev1.Secret{}
		if err := c.MultiClusterController.Get(clusterName, vPod.Namespace, secretName, vSecret); err != nil {
			return nil, pkgerr.Wrapf(err, "failed to get vSecret %s/%s", vPod.Namespace, secretName)
		}

		// normal secret. pSecret name is the same as the vSecret.
		if vSecret.Type != corev1.SecretTypeServiceAccountToken {
			continue
		}

		secretList, err := c.secretLister.Secrets(pPod.Namespace).List(labels.SelectorFromSet(map[string]string{
			constants.LabelSecretUID: string(vSecret.UID),
		}))
		if err != nil || len(secretList) == 0 {
			return nil, fmt.Errorf("failed to find sa secret from super control plane %s/%s: %v", pPod.Namespace, vSecret.UID, err)
		}

		mutateNameMap[secretName] = secretList[0].Name
	}

	return mutateNameMap, nil
}

func (c *controller) getClusterNameServer(cluster string) (string, error) {
	svc, err := c.serviceLister.Services(conversion.ToSuperClusterNamespace(cluster, constants.TenantDNSServerNS)).Get(constants.TenantDNSServerServiceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

func (c *controller) getPodRelatedServices(cluster string, pPod *corev1.Pod) ([]*corev1.Service, error) {
	var services []*corev1.Service
	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterServiceNetwork) {
		apiserver, err := c.serviceLister.Services(cluster).Get("apiserver-svc")
		if err != nil {
			return nil, err
		}
		services = append(services, apiserver)
	}

	var list []*corev1.Service
	var err error
	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterPooling) {
		// In case of super cluster pooling, it is possible that the tenant default namespace is not synced to current super cluster.
		// We need to query the tenant apiserver to get the services in tenant default namespace.
		// Note that the cluster ip in the service from the tenant default namespace can be a bogus value.
		// We expect an external loadbalancer is used for each tenant service.
		serviceList := &corev1.ServiceList{}
		if err = c.MultiClusterController.List(cluster, serviceList, client.InNamespace(metav1.NamespaceDefault)); err != nil {
			return nil, err
		}

		for i := range serviceList.Items {
			list = append(list, &serviceList.Items[i])
		}
	} else {
		list, err = c.serviceLister.Services(conversion.ToSuperClusterNamespace(cluster, metav1.NamespaceDefault)).List(labels.Everything())
		if err != nil {
			return nil, err
		}
	}
	services = append(services, list...)

	list, err = c.serviceLister.Services(pPod.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	services = append(services, list...)
	if len(services) == 0 {
		return nil, fmt.Errorf("service is not ready")
	}
	return services, nil
}

func (c *controller) reconcilePodUpdate(clusterName, targetNamespace, requestUID string, pPod, vPod *corev1.Pod) error {
	if pPod.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("pPod %s/%s delegated UID is different from updated object", targetNamespace, pPod.Name)
	}

	if vPod.DeletionTimestamp != nil {
		if pPod.DeletionTimestamp != nil {
			// pPod is under deletion, waiting for UWS bock populate the pod status.
			return nil
		}
		deleteOptions := metav1.NewDeleteOptions(*vPod.DeletionGracePeriodSeconds)
		deleteOptions.Preconditions = metav1.NewUIDPreconditions(string(pPod.UID))
		err := c.client.Pods(targetNamespace).Delete(context.TODO(), pPod.Name, *deleteOptions)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	vc, err := util.GetVirtualClusterObject(c.MultiClusterController, clusterName)
	if err != nil {
		return err
	}
	updatedPod := conversion.Equality(c.Config, vc).CheckPodEquality(pPod, vPod)
	if updatedPod != nil {
		pPod, err = c.client.Pods(targetNamespace).Update(context.TODO(), updatedPod, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	updatedPodStatus := conversion.CheckDWPodConditionEquality(pPod, vPod)
	if updatedPodStatus != nil {
		updatedPod = pPod.DeepCopy()
		updatedPod.Status = *updatedPodStatus
		_, err = c.client.Pods(targetNamespace).UpdateStatus(context.TODO(), updatedPod, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) reconcilePodRemove(clusterName, targetNamespace, requestUID, name string, pPod *corev1.Pod) error {
	if pPod.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("to be deleted pPod %s/%s delegated UID is different from deleted object", targetNamespace, name)
	}

	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
		Preconditions:     metav1.NewUIDPreconditions(string(pPod.UID)),
	}
	err := c.client.Pods(targetNamespace).Delete(context.TODO(), name, *opts)
	if apierrors.IsNotFound(err) {
		klog.Warningf("To be deleted pod %s/%s of cluster (%s) is not found in super control plane", targetNamespace, name, clusterName)
		return nil
	}
	return err
}

func recordOperationDuration(operation string, start time.Time) {
	metrics.PodOperationsDuration.WithLabelValues(operation).Observe(metrics.SinceInSeconds(start))
}

func recordOperationStatus(operation string, err error) {
	if err != nil {
		metrics.PodOperations.With(prometheus.Labels{"operation_type": operation, "code": utilconstants.StatusCodeError}).Inc()
		return
	}
	metrics.PodOperations.With(prometheus.Labels{"operation_type": operation, "code": utilconstants.StatusCodeOK}).Inc()
}
