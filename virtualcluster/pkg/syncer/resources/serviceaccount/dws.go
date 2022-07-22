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

package serviceaccount

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
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
)

func (c *controller) StartDWS(stopCh <-chan struct{}) error {
	if !cache.WaitForCacheSync(stopCh, c.saSynced) {
		return fmt.Errorf("failed to wait for sa caches to sync")
	}
	return c.MultiClusterController.Start(stopCh)
}

// The reconcile logic for tenant control plane service account informer
func (c *controller) Reconcile(request reconciler.Request) (reconciler.Result, error) {
	klog.V(4).Infof("reconcile service account %s/%s for cluster %s", request.Namespace, request.Name, request.ClusterName)
	targetNamespace := conversion.ToSuperClusterNamespace(request.ClusterName, request.Namespace)
	pSa, err := c.saLister.ServiceAccounts(targetNamespace).Get(request.Name)
	pExists := true
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		pExists = false
	}
	vExists := true

	vSa := &corev1.ServiceAccount{}
	if err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vSa); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
		vExists = false
	}

	switch {
	case vExists && !pExists:
		err := c.reconcileServiceAccountCreate(request.ClusterName, targetNamespace, request.UID, vSa)
		if err != nil {
			klog.Errorf("failed reconcile serviceaccount %s/%s CREATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case !vExists && pExists:
		err := c.reconcileServiceAccountRemove(request.ClusterName, targetNamespace, request.UID, request.Name, pSa)
		if err != nil {
			klog.Errorf("failed reconcile serviceaccount %s/%s DELETE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case vExists && pExists:
		err := c.reconcileServiceAccountUpdate(request.ClusterName, targetNamespace, request.UID, pSa, vSa)
		if err != nil {
			klog.Errorf("failed reconcile serviceaccount %s/%s UPDATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	default:
		// object is gone.
	}
	return reconciler.Result{}, nil
}

func (c *controller) reconcileServiceAccountCreate(clusterName, targetNamespace, requestUID string, vSa *corev1.ServiceAccount) error {
	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, vSa)
	if err != nil {
		return err
	}
	pServiceAccount := newObj.(*corev1.ServiceAccount)
	// set to empty and token controller will regenerate one.
	pServiceAccount.Secrets = nil

	pServiceAccount, err = c.saClient.ServiceAccounts(targetNamespace).Create(context.TODO(), pServiceAccount, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		if pServiceAccount.Annotations[constants.LabelUID] == requestUID {
			klog.Infof("service account %s/%s of cluster %s already exist in super control plane", targetNamespace, pServiceAccount.Name, clusterName)
		} else {
			klog.Errorf("pServiceAccount %s/%s exists but its delegated UID is different", targetNamespace, pServiceAccount.Name)
		}
		return nil
	}
	return err
}

func (c *controller) reconcileServiceAccountUpdate(clusterName, targetNamespace, requestUID string, pSa, vSa *corev1.ServiceAccount) error {
	// Just mark the default service account of super control plane namespace, created by super control plane service account controller, as a tenant related resource.
	if vSa.Name == "default" {
		if len(pSa.Annotations) == 0 {
			pSa.Annotations = make(map[string]string)
		}
		var err error
		if pSa.Annotations[constants.LabelCluster] != clusterName || pSa.Annotations[constants.LabelUID] != string(vSa.UID) || pSa.Annotations[constants.LabelNamespace] != vSa.Namespace {
			pSa.Annotations[constants.LabelCluster] = clusterName
			pSa.Annotations[constants.LabelUID] = string(vSa.UID)
			pSa.Annotations[constants.LabelNamespace] = vSa.Namespace
			_, err = c.saClient.ServiceAccounts(targetNamespace).Update(context.TODO(), pSa, metav1.UpdateOptions{})
		}
		return err
	}

	if pSa.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("pServiceAccount %s/%s delegated UID is different from updated object", targetNamespace, pSa.Name)
	}

	// do nothing.
	return nil
}

func (c *controller) reconcileServiceAccountRemove(clusterName, targetNamespace, requestUID, name string, pSa *corev1.ServiceAccount) error {
	if pSa.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("to be deleted pServiceAccount %s/%s delegated UID is different from deleted object", targetNamespace, pSa.Name)
	}
	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
	}
	err := c.saClient.ServiceAccounts(targetNamespace).Delete(context.TODO(), name, *opts)
	if apierrors.IsNotFound(err) {
		klog.Warningf("service account %s/%s of cluster %s not found in super control plane", targetNamespace, name, clusterName)
		return nil
	}
	return err
}
