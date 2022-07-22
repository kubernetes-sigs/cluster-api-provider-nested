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

package ingress

import (
	"context"
	"fmt"

	"k8s.io/api/extensions/v1beta1"
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
	if !cache.WaitForCacheSync(stopCh, c.ingressSynced) {
		return fmt.Errorf("failed to wait for caches to sync before starting Ingress dws")
	}
	return c.MultiClusterController.Start(stopCh)
}

func (c *controller) Reconcile(request reconciler.Request) (reconciler.Result, error) {
	klog.V(4).Infof("reconcile ingress %s/%s for cluster %s", request.Namespace, request.Name, request.ClusterName)
	targetNamespace := conversion.ToSuperClusterNamespace(request.ClusterName, request.Namespace)
	pIngress, err := c.ingressLister.Ingresses(targetNamespace).Get(request.Name)
	pExists := true
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		pExists = false
	}
	vExists := true
	vIngress := &v1beta1.Ingress{}
	if err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vIngress); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		vExists = false
	}

	switch {
	case vExists && !pExists:
		err := c.reconcileIngressCreate(request.ClusterName, targetNamespace, request.UID, vIngress)
		if err != nil {
			klog.Errorf("failed reconcile ingress %s/%s CREATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case !vExists && pExists:
		err := c.reconcileIngressRemove(targetNamespace, request.UID, request.Name, pIngress)
		if err != nil {
			klog.Errorf("failed reconcile ingress %s/%s DELETE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case vExists && pExists:
		err := c.reconcileIngressUpdate(request.ClusterName, targetNamespace, request.UID, pIngress, vIngress)
		if err != nil {
			klog.Errorf("failed reconcile ingress %s/%s UPDATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	default:
		// object is gone.
	}
	return reconciler.Result{}, nil
}

func (c *controller) reconcileIngressCreate(clusterName, targetNamespace, requestUID string, ingress *v1beta1.Ingress) error {
	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, ingress)
	if err != nil {
		return err
	}

	pIngress := newObj.(*v1beta1.Ingress)

	pIngress, err = c.ingressClient.Ingresses(targetNamespace).Create(context.TODO(), pIngress, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		if pIngress.Annotations[constants.LabelUID] == requestUID {
			klog.Infof("ingress %s/%s of cluster %s already exist in super control plane", targetNamespace, pIngress.Name, clusterName)
		} else {
			klog.Errorf("pIngress %s/%s exists but its delegated object UID is different", targetNamespace, pIngress.Name)
		}
		return nil
	}
	return err
}

func (c *controller) reconcileIngressUpdate(clusterName, targetNamespace, requestUID string, pIngress, vIngress *v1beta1.Ingress) error {
	if pIngress.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("pIngress %s/%s delegated UID is different from updated object", targetNamespace, pIngress.Name)
	}

	vc, err := util.GetVirtualClusterObject(c.MultiClusterController, clusterName)
	if err != nil {
		return err
	}
	updated := conversion.Equality(c.Config, vc).CheckIngressEquality(pIngress, vIngress)
	if updated != nil {
		_, err = c.ingressClient.Ingresses(targetNamespace).Update(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) reconcileIngressRemove(targetNamespace, requestUID, name string, pIngress *v1beta1.Ingress) error {
	if pIngress.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("to be deleted pIngress %s/%s delegated UID is different from deleted object", targetNamespace, name)
	}

	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
		Preconditions:     metav1.NewUIDPreconditions(string(pIngress.UID)),
	}
	err := c.ingressClient.Ingresses(targetNamespace).Delete(context.TODO(), name, *opts)
	if apierrors.IsNotFound(err) {
		klog.Warningf("To be deleted ingress %s/%s not found in super control plane", targetNamespace, name)
		return nil
	}
	return err
}
