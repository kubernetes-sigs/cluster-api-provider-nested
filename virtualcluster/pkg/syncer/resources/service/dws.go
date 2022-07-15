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

package service

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
	if !cache.WaitForCacheSync(stopCh, c.serviceSynced) {
		return fmt.Errorf("failed to wait for caches to sync before starting Service dws")
	}
	return c.MultiClusterController.Start(stopCh)
}

func (c *controller) Reconcile(request reconciler.Request) (reconciler.Result, error) {
	klog.V(4).Infof("reconcile service %s/%s for cluster %s", request.Namespace, request.Name, request.ClusterName)
	targetNamespace := conversion.ToSuperClusterNamespace(request.ClusterName, request.Namespace)
	pService, err := c.serviceLister.Services(targetNamespace).Get(request.Name)
	pExists := true
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		pExists = false
	}
	vExists := true
	vService := &corev1.Service{}
	if err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vService); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		vExists = false
	}
	switch {
	case vExists && !pExists:
		err := c.reconcileServiceCreate(request.ClusterName, targetNamespace, request.UID, vService)
		if err != nil {
			klog.Errorf("failed reconcile service %s/%s CREATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case !vExists && pExists:
		err := c.reconcileServiceRemove(targetNamespace, request.UID, request.Name, pService)
		if err != nil {
			klog.Errorf("failed reconcile service %s/%s DELETE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case vExists && pExists:
		err := c.reconcileServiceUpdate(request.ClusterName, targetNamespace, request.UID, pService, vService)
		if err != nil {
			klog.Errorf("failed reconcile service %s/%s UPDATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	default:
		// object is gone.
	}
	return reconciler.Result{}, nil
}

func (c *controller) reconcileServiceCreate(clusterName, targetNamespace, requestUID string, service *corev1.Service) error {
	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, service)
	if err != nil {
		return err
	}

	pService := newObj.(*corev1.Service)
	conversion.VC(nil, "").Service(pService).Mutate(service)

	pService, err = c.serviceClient.Services(targetNamespace).Create(context.TODO(), pService, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		if pService.Annotations[constants.LabelUID] == requestUID {
			klog.Infof("service %s/%s of cluster %s already exist in super control plane", targetNamespace, pService.Name, clusterName)
			return nil
		}
		return fmt.Errorf("pService %s/%s exists but its delegated object UID is different", targetNamespace, pService.Name)
	}
	return err
}

func (c *controller) reconcileServiceUpdate(clusterName, targetNamespace, requestUID string, pService, vService *corev1.Service) error {
	if pService.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("pService %s/%s delegated UID is different from updated object", targetNamespace, pService.Name)
	}

	vc, err := util.GetVirtualClusterObject(c.MultiClusterController, clusterName)
	if err != nil {
		return err
	}
	updated := conversion.Equality(c.Config, vc).CheckServiceEquality(pService, vService)
	if updated != nil {
		_, err = c.serviceClient.Services(targetNamespace).Update(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) reconcileServiceRemove(targetNamespace, requestUID, name string, pService *corev1.Service) error {
	if pService.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("to be deleted pService %s/%s delegated UID is different from deleted object", targetNamespace, name)
	}

	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
		Preconditions:     metav1.NewUIDPreconditions(string(pService.UID)),
	}
	err := c.serviceClient.Services(targetNamespace).Delete(context.TODO(), name, *opts)
	if apierrors.IsNotFound(err) {
		klog.Warningf("To be deleted service %s/%s not found in super control plane", targetNamespace, name)
		return nil
	}
	return err
}
