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

package node

import (
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/provider"
)

// StartUWS starts the upward syncer
// and blocks until an empty struct is sent to the stop channel.
func (c *controller) StartUWS(stopCh <-chan struct{}) error {
	if !cache.WaitForCacheSync(stopCh, c.nodeSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	return c.UpwardController.Start(stopCh)
}

func (c *controller) enqueueNode(obj interface{}) {
	node := obj.(*corev1.Node)
	c.UpwardController.AddToQueue(node.Name)
}

func (c *controller) BackPopulate(nodeName string) error {
	node, err := c.nodeLister.Get(nodeName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// TODO: notify every tenant.
			return nil
		}
		return err
	}
	klog.V(4).Infof("back populate node %s/%s", node.Namespace, node.Name)
	c.Lock()
	clusterList := make([]string, 0, len(c.nodeNameToCluster[node.Name]))
	for clusterName := range c.nodeNameToCluster[node.Name] {
		clusterList = append(clusterList, clusterName)
	}
	c.Unlock()

	if len(clusterList) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(len(clusterList))
	for _, clusterName := range clusterList {
		go c.updateClusterNode(clusterName, node, &wg)
	}
	wg.Wait()

	return nil
}

func (c *controller) updateClusterNode(clusterName string, node *corev1.Node, wg *sync.WaitGroup) {
	defer wg.Done()

	tenantClient, err := c.MultiClusterController.GetClusterClient(clusterName)
	if err != nil {
		klog.Errorf("failed to create client from cluster %s config: %v", clusterName, err)
		// Cluster is removed. We should remove the entry from nodeNameToCluster map.
		c.Lock()
		delete(c.nodeNameToCluster[node.Name], clusterName)
		c.Unlock()
		return
	}

	vNode := &corev1.Node{}
	if err := c.MultiClusterController.Get(clusterName, "", node.Name, vNode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("could not find node %s/%s: %v", clusterName, node.Name, err)
			c.Lock()
			if _, exists := c.nodeNameToCluster[node.Name]; exists {
				delete(c.nodeNameToCluster[node.Name], clusterName)
			}
			c.Unlock()
		}
		return
	}

	newVNode := vNode.DeepCopy()
	newVNode.Status.Conditions = node.Status.Conditions
	vNodeAddress, err := c.vnodeProvider.GetNodeAddress(node)
	if err != nil {
		klog.Errorf("unable get node address from provider: %v", err)
		return
	}
	newVNode.Status.Addresses = vNodeAddress
	nodeDaemonEndpoints, err := c.vnodeProvider.GetNodeDaemonEndpoints(node)
	if err != nil {
		klog.Errorf("unable get node daemon endpoints from provider: %v", err)
		return
	}
	newVNode.Status.DaemonEndpoints = nodeDaemonEndpoints

	newVNode.Spec.Taints = provider.GetNodeTaints(c.vnodeProvider, node, metav1.Now())
	newVNode.ObjectMeta.SetLabels(provider.GetNodeLabels(c.vnodeProvider, node))

	if err := vnode.UpdateNode(tenantClient.CoreV1().Nodes(), vNode, newVNode); err != nil {
		klog.Errorf("failed to update node %s/%s's heartbeats: %v", clusterName, node.Name, err)
	}
}
