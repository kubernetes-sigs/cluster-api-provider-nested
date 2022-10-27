/*
Copyright 2022 The Kubernetes Authors.

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

package persistentvolumeclaim

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
)

// StartUWS starts the upward syncer
// and blocks until an empty struct is sent to the stop channel.
func (c *controller) StartUWS(stopCh <-chan struct{}) error {
	if !cache.WaitForCacheSync(stopCh, c.pvcSynced, c.pvcSynced) {
		return fmt.Errorf("failed to wait for caches to sync persistentvolumeclaim")
	}
	return c.UpwardController.Start(stopCh)
}

func (c *controller) BackPopulate(key string) error {
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key %v: %v", key, err))
		return nil
	}

	pPVC, err := c.pvcLister.PersistentVolumeClaims(pNamespace).Get(pName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	clusterName, vNamespace := conversion.GetVirtualOwner(pPVC)
	if clusterName == "" {
		// Bound PVC does not belong to any tenant.
		return nil
	}

	tenantClient, err := c.MultiClusterController.GetClusterClient(clusterName)
	if err != nil {
		return fmt.Errorf("failed to create client from cluster %s config: %w", clusterName, err)
	}

	vPVC := &corev1.PersistentVolumeClaim{}
	if err := c.MultiClusterController.Get(clusterName, vNamespace, pName, vPVC); err != nil {
		klog.Errorf("failed to get tenant cluster %s pvc %s/%s", clusterName, vNamespace, pName)
		return err
	}

	updatedPVC := conversion.Equality(c.Config, nil).CheckUWPVCStatusEquality(pPVC, vPVC)
	if updatedPVC != nil {
		_, err = tenantClient.CoreV1().PersistentVolumeClaims(vNamespace).UpdateStatus(context.TODO(), updatedPVC, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to update tenant cluster %s pvc %s/%s, %v", clusterName, vNamespace, pName, err)
			return err
		}
	}
	return nil
}
