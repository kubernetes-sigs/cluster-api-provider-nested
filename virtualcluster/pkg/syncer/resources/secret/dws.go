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

package secret

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
)

func (c *controller) StartDWS(stopCh <-chan struct{}) error {
	if !cache.WaitForCacheSync(stopCh, c.secretSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	return c.MultiClusterController.Start(stopCh)
}

// The reconcile logic for tenant control plane secret informer
func (c *controller) Reconcile(request reconciler.Request) (reconciler.Result, error) {
	klog.V(4).Infof("reconcile secret %s/%s for cluster %s", request.Namespace, request.Name, request.ClusterName)
	targetNamespace := conversion.ToSuperClusterNamespace(request.ClusterName, request.Namespace)
	vSecret := &corev1.Secret{}
	err := c.MultiClusterController.Get(request.ClusterName, request.Namespace, request.Name, vSecret)
	if err == nil {

	} else if !apierrors.IsNotFound(err) {
		return reconciler.Result{Requeue: true}, err
	}

	var pSecret *corev1.Secret
	secretList, err := c.secretLister.Secrets(targetNamespace).List(labels.SelectorFromSet(map[string]string{
		constants.LabelSecretUID: request.UID,
	}))
	if err != nil && !apierrors.IsNotFound(err) {
		return reconciler.Result{Requeue: true}, err
	}
	if len(secretList) != 0 {
		// This is service account vSecret, it is unlikely we have a dup name in super but
		for i, each := range secretList {
			if each.Annotations[constants.LabelUID] == request.UID {
				pSecret = secretList[i]
				break
			}
		}
		if pSecret == nil {
			return reconciler.Result{Requeue: true}, fmt.Errorf("there are pSecrets that represent vSerect %s/%s but the UID is unmatched", request.Namespace, request.Name)
		}
	} else {
		// We need to use name to search again for normal vSecret
		pSecret, err = c.secretLister.Secrets(targetNamespace).Get(request.Name)
		if err == nil {
			if pSecret.Type == corev1.SecretTypeServiceAccountToken {
				pSecret = nil
			}
		} else if !apierrors.IsNotFound(err) {
			return reconciler.Result{Requeue: true}, err
		}
	}

	switch {
	case !reflect.DeepEqual(vSecret, &corev1.Secret{}) && pSecret == nil:
		err := c.reconcileSecretCreate(request.ClusterName, targetNamespace, request.UID, vSecret)
		if err != nil {
			klog.Errorf("failed reconcile secret %s/%s CREATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case reflect.DeepEqual(vSecret, &corev1.Secret{}) && pSecret != nil:
		err := c.reconcileSecretRemove(targetNamespace, request.UID, request.Name, pSecret)
		if err != nil {
			klog.Errorf("failed reconcile secret %s/%s DELETE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	case vSecret != nil && pSecret != nil:
		err := c.reconcileSecretUpdate(request.ClusterName, targetNamespace, request.UID, pSecret, vSecret)
		if err != nil {
			klog.Errorf("failed reconcile secret %s/%s UPDATE of cluster %s %v", request.Namespace, request.Name, request.ClusterName, err)
			return reconciler.Result{Requeue: true}, err
		}
	default:
		// object is gone.
	}
	return reconciler.Result{}, nil
}

func (c *controller) reconcileSecretCreate(clusterName, targetNamespace, requestUID string, secret *corev1.Secret) error {
	switch secret.Type {
	case corev1.SecretTypeServiceAccountToken:
		return c.reconcileServiceAccountSecretCreate(clusterName, targetNamespace, secret)
	default:
		return c.reconcileNormalSecretCreate(clusterName, targetNamespace, requestUID, secret)
	}
}

func (c *controller) reconcileServiceAccountSecretCreate(clusterName, targetNamespace string, vSecret *corev1.Secret) error {
	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, vSecret)
	if err != nil {
		return err
	}

	pSecret := newObj.(*corev1.Secret)
	conversion.VC(c.MultiClusterController, "").ServiceAccountTokenSecret(pSecret).Mutate(vSecret, clusterName)

	_, err = c.secretClient.Secrets(targetNamespace).Create(context.TODO(), pSecret, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		klog.Infof("secret %s/%s of cluster %s already exist in super control plane", targetNamespace, pSecret.Name, clusterName)
		return nil
	}

	return err
}

func (c *controller) reconcileServiceAccountSecretUpdate(targetNamespace string, pSecret, vSecret *corev1.Secret) error {
	updatedBinaryData, equal := conversion.Equality(c.Config, nil).CheckBinaryDataEquality(pSecret.Data, vSecret.Data)
	if equal {
		return nil
	}

	updatedSecret := pSecret.DeepCopy()
	updatedSecret.Data = updatedBinaryData
	_, err := c.secretClient.Secrets(targetNamespace).Update(context.TODO(), updatedSecret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) reconcileNormalSecretCreate(clusterName, targetNamespace, requestUID string, secret *corev1.Secret) error {
	newObj, err := c.Conversion().BuildSuperClusterObject(clusterName, secret)
	if err != nil {
		return err
	}

	pSecret, err := c.secretClient.Secrets(targetNamespace).Create(context.TODO(), newObj.(*corev1.Secret), metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		if pSecret.Annotations[constants.LabelUID] == requestUID {
			klog.Infof("secret %s/%s of cluster %s already exist in super control plane", targetNamespace, secret.Name, clusterName)
		} else {
			klog.Errorf("pSecret %s/%s exists but its delegated object UID is different", targetNamespace, pSecret.Name)
		}
		return nil
	}

	return err
}

func (c *controller) reconcileSecretUpdate(clusterName, targetNamespace, requestUID string, pSecret, vSecret *corev1.Secret) error {
	switch vSecret.Type {
	case corev1.SecretTypeServiceAccountToken:
		return c.reconcileServiceAccountSecretUpdate(targetNamespace, pSecret, vSecret)
	default:
		return c.reconcileNormalSecretUpdate(clusterName, targetNamespace, requestUID, pSecret, vSecret)
	}
}

func (c *controller) reconcileNormalSecretUpdate(clusterName, targetNamespace, requestUID string, pSecret, vSecret *corev1.Secret) error {
	if pSecret.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("pEndpoints %s/%s delegated UID is different from updated object", targetNamespace, pSecret.Name)
	}
	vc, err := util.GetVirtualClusterObject(c.MultiClusterController, clusterName)
	if err != nil {
		return err
	}
	updatedSecret := conversion.Equality(c.Config, vc).CheckSecretEquality(pSecret, vSecret)
	if updatedSecret != nil {
		_, err = c.secretClient.Secrets(targetNamespace).Update(context.TODO(), updatedSecret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) reconcileSecretRemove(targetNamespace, requestUID, name string, secret *corev1.Secret) error {
	if _, isSaSecret := secret.Labels[constants.LabelSecretUID]; isSaSecret {
		return c.reconcileServiceAccountTokenSecretRemove(targetNamespace, requestUID, name)
	}
	return c.reconcileNormalSecretRemove(targetNamespace, requestUID, name, secret)
}

func (c *controller) reconcileNormalSecretRemove(targetNamespace, requestUID, name string, pSecret *corev1.Secret) error {
	if pSecret.Annotations[constants.LabelUID] != requestUID {
		return fmt.Errorf("to be deleted pSecret %s/%s delegated UID is different from deleted object", targetNamespace, pSecret.Name)
	}
	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
	}
	err := c.secretClient.Secrets(targetNamespace).Delete(context.TODO(), name, *opts)
	if apierrors.IsNotFound(err) {
		klog.Warningf("secret %s/%s of cluster is not found in super control plane", targetNamespace, name)
		return nil
	}
	return err
}

func (c *controller) reconcileServiceAccountTokenSecretRemove(targetNamespace, requestUID, name string) error {
	opts := &metav1.DeleteOptions{
		PropagationPolicy: &constants.DefaultDeletionPolicy,
	}
	err := c.secretClient.Secrets(targetNamespace).DeleteCollection(context.TODO(), *opts, metav1.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			constants.LabelSecretUID: requestUID,
		}).String(),
	})
	if apierrors.IsNotFound(err) {
		klog.Warningf("secret %s/%s of cluster is not found in super control plane", targetNamespace, name)
		return nil
	}
	return err
}
