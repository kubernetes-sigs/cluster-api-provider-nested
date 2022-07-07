/*
Copyright 2020 The Kubernetes Authors.

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

package priorityclass

import (
	"context"
	"fmt"

	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
)

// StartUWS starts the upward syncer
// and blocks until an empty struct is sent to the stop channel.
func (c *controller) StartUWS(stopCh <-chan struct{}) error {
	if !cache.WaitForCacheSync(stopCh, c.priorityclassSynced) {
		return fmt.Errorf("failed to wait for caches to sync priorityclass")
	}
	return c.UpwardController.Start(stopCh)
}

func (c *controller) BackPopulate(key string) error {
	// The key format is clustername/pcName.
	clusterName, scName, _ := cache.SplitMetaNamespaceKey(key)

	op := reconciler.AddEvent
	pPriorityClass, err := c.priorityclassLister.Get(scName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		op = reconciler.DeleteEvent
	}

	tenantClient, err := c.MultiClusterController.GetClusterClient(clusterName)
	if err != nil {
		return fmt.Errorf("failed to create client from cluster %s config: %v", clusterName, err)
	}

	vPriorityClass := &schedulingv1.PriorityClass{}
	if err := c.MultiClusterController.Get(clusterName, "", scName, vPriorityClass); err != nil {
		if apierrors.IsNotFound(err) {
			if op == reconciler.AddEvent {
				// Available in super, hence create a new in tenant control plane
				vPriorityClass := conversion.BuildVirtualPriorityClass(clusterName, pPriorityClass)
				_, err := tenantClient.SchedulingV1().PriorityClasses().Create(context.TODO(), vPriorityClass, metav1.CreateOptions{})
				if err != nil {
					return err
				}
			}
			return nil
		}
		return err
	}

	if op == reconciler.DeleteEvent {
		opts := &metav1.DeleteOptions{
			PropagationPolicy: &constants.DefaultDeletionPolicy,
		}
		err := tenantClient.SchedulingV1().PriorityClasses().Delete(context.TODO(), scName, *opts)
		if err != nil {
			return err
		}
	} else {
		updatedPriorityClass := conversion.Equality(c.Config, nil).CheckPriorityClassEquality(pPriorityClass, vPriorityClass)
		if updatedPriorityClass != nil {
			_, err := tenantClient.SchedulingV1().PriorityClasses().Update(context.TODO(), updatedPriorityClass, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
