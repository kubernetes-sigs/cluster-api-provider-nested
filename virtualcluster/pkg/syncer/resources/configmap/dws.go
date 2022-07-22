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

package configmap

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
)

func (c *controller) StartDWS(stopCh <-chan struct{}) error {
	if !cache.WaitForCacheSync(stopCh, c.configMapSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	return c.MultiClusterController.Start(stopCh)
}

// The reconcile logic for tenant control plane configMap informer
func (c *controller) Reconcile(request reconciler.Request) (reconciler.Result, error) {
	klog.V(4).Infof("reconcile configmap %s/%s event for cluster %s", request.Namespace, request.Name, request.ClusterName)

	targetNamespace := conversion.ToSuperClusterNamespace(request.ClusterName, request.Namespace)
	pConfigMap, err := c.configMapLister.ConfigMaps(targetNamespace).Get(request.Name)
	pExists := true
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		pExists = false
	}
	vExists := true
	vConfigMap := &corev1.ConfigMap{}
	if err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		vExists = false
	}

	switch {
	case vExists && !pExists:
		err := c.reconcileConfigMapCreate(request.ClusterName, targetNamespace, request.UID, vConfigMap)
		if err != nil {
			klog.Errorf("failed reconcile configmap %s/%s CREATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case !vExists && pExists:
		err := c.reconcileConfigMapRemove(request.ClusterName, targetNamespace, request.UID, request.Name, pConfigMap)
		if err != nil {
			klog.Errorf("failed reconcile configmap %s/%s DELETE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case vExists && pExists:
		err := c.reconcileConfigMapUpdate(request.ClusterName, targetNamespace, request.UID, pConfigMap, vConfigMap)
		if err != nil {
			klog.Errorf("failed reconcile configmap %s/%s UPDATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	default:
		// object is gone.
	}
	return reconciler.Result{}, nil
}

func (c *controller) reconcileConfigMapCreate(clusterName, targetNamespace, requestUID string, configMap *corev1.ConfigMap) error {
	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, configMap)
	if err != nil {
		return err
	}

	pConfigMap, err := c.configMapClient.ConfigMaps(targetNamespace).Create(context.TODO(), newObj.(*corev1.ConfigMap), metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		if pConfigMap.Annotations[constants.LabelUID] == requestUID {
			klog.Infof("configmap %s/%s of cluster %s already exist in super control plane", targetNamespace, configMap.Name, clusterName)
		} else {
			klog.Errorf("pConfigMap %s/%s exists but its delegated object UID is different", targetNamespace, pConfigMap.Name)
		}
		return nil
	}
	return err
}

func (c *controller) reconcileConfigMapUpdate(clusterName, targetNamespace, requestUID string, pConfigMap, vConfigMap *corev1.ConfigMap) error {
	if pConfigMap.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("pConfigMap %s/%s delegated UID is different from updated object", targetNamespace, pConfigMap.Name)
	}
	vc, err := util.GetVirtualClusterObject(c.MultiClusterController, clusterName)
	if err != nil {
		return err
	}
	updatedConfigMap := conversion.Equality(c.Config, vc).CheckConfigMapEquality(pConfigMap, vConfigMap)
	if updatedConfigMap != nil {
		_, err = c.configMapClient.ConfigMaps(targetNamespace).Update(context.TODO(), updatedConfigMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) reconcileConfigMapRemove(clusterName, targetNamespace, requestUID, name string, pConfigMap *corev1.ConfigMap) error {
	if pConfigMap.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("to be deleted pConfigMap %s/%s delegated UID is different from deleted object", targetNamespace, name)
	}
	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
	}
	err := c.configMapClient.ConfigMaps(targetNamespace).Delete(context.TODO(), name, *opts)
	if apierrors.IsNotFound(err) {
		klog.Warningf("configmap %s/%s of cluster %s not found in super control plane", targetNamespace, name, clusterName)
		return nil
	}
	return err
}
