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

package endpoints

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
	if !cache.WaitForCacheSync(stopCh, c.endpointsSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	return c.MultiClusterController.Start(stopCh)
}

// The reconcile logic for tenant control plane endpoints informer
func (c *controller) Reconcile(request reconciler.Request) (reconciler.Result, error) {
	vService := &corev1.Service{}
	err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vService)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconciler.Result{Requeue: true}, fmt.Errorf("fail to query service from tenant control plane %s", request.ClusterName)
	}
	if err == nil {
		if vService.Spec.Selector != nil {
			// Supercontrol plane ep controller handles the service ep lifecycle, quit.
			return reconciler.Result{}, nil
		}
	}
	klog.V(4).Infof("reconcile endpoints %s/%s for cluster %s", request.Namespace, request.Name, request.ClusterName)
	targetNamespace := conversion.ToSuperClusterNamespace(request.ClusterName, request.Namespace)
	pEndpoints, err := c.endpointsLister.Endpoints(targetNamespace).Get(request.Name)
	pExists := true
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		pExists = false
	}
	vExists := true
	vEndpoints := &corev1.Endpoints{}
	if err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vEndpoints); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		vExists = false
	}

	switch {
	case vExists && !pExists:
		err := c.reconcileEndpointsCreate(request.ClusterName, targetNamespace, request.UID, vEndpoints)
		if err != nil {
			klog.Errorf("failed reconcile endpoints %s/%s CREATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case !vExists && pExists:
		err := c.reconcileEndpointsRemove(request.ClusterName, targetNamespace, request.UID, request.Name, pEndpoints)
		if err != nil {
			klog.Errorf("failed reconcile endpoints %s/%s DELETE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case vExists && pExists:
		err := c.reconcileEndpointsUpdate(request.ClusterName, targetNamespace, request.UID, pEndpoints, vEndpoints)
		if err != nil {
			klog.Errorf("failed reconcile endpoints %s/%s UPDATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	default:
		// object is gone.
	}
	return reconciler.Result{}, nil
}

func (c *controller) reconcileEndpointsCreate(clusterName, targetNamespace, requestUID string, ep *corev1.Endpoints) error {
	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, ep)
	if err != nil {
		return err
	}

	pEndpoints := newObj.(*corev1.Endpoints)

	pEndpoints, err = c.endpointClient.Endpoints(targetNamespace).Create(context.TODO(), pEndpoints, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		if pEndpoints.Annotations[constants.LabelUID] == requestUID {
			klog.Infof("endpoints %s/%s of cluster %s already exist in super control plane", targetNamespace, pEndpoints.Name, clusterName)
		} else {
			klog.Errorf("pEndpoints %s/%s exists but its delegated object UID is different", targetNamespace, pEndpoints.Name)
		}
		return nil
	}
	return err
}

func (c *controller) reconcileEndpointsUpdate(clusterName, targetNamespace, requestUID string, pEP, vEP *corev1.Endpoints) error {
	if pEP.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("pEndpoints %s/%s delegated UID is different from updated object", targetNamespace, pEP.Name)
	}
	vc, err := util.GetVirtualClusterObject(c.MultiClusterController, clusterName)
	if err != nil {
		return err
	}
	updatedEndpoints := conversion.Equality(c.Config, vc).CheckEndpointsEquality(pEP, vEP)
	if updatedEndpoints != nil {
		_, err = c.endpointClient.Endpoints(targetNamespace).Update(context.TODO(), updatedEndpoints, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) reconcileEndpointsRemove(clusterName, targetNamespace, requestUID, name string, pEP *corev1.Endpoints) error {
	if pEP.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("to be deleted pEndpoints %s/%s delegated UID is different from deleted object", targetNamespace, pEP.Name)
	}
	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
	}
	err := c.endpointClient.Endpoints(targetNamespace).Delete(context.TODO(), name, *opts)
	if apierrors.IsNotFound(err) {
		klog.Warningf("endpoints %s/%s of %s cluster not found in super control plane", targetNamespace, name, clusterName)
		return nil
	}
	return err
}
