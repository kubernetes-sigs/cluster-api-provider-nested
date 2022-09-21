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

package vnode

import (
	"context"
	"encoding/json"
	"fmt"

	pkgerr "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/apis/config"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/native"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/pod"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/provider"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/service"
)

func GetNodeProvider(config *config.SyncerConfiguration, client clientset.Interface) provider.VirtualNodeProvider {
	for _, labelKey := range config.ExtraNodeLabels {
		defaultLabelsToSync[labelKey] = struct{}{}
	}
	taintsToSync := make(map[string]struct{})
	for _, taintKey := range config.OpaqueTaintKeys {
		taintsToSync[taintKey] = struct{}{}
	}
	if featuregate.DefaultFeatureGate.Enabled(featuregate.VNodeProviderService) {
		return service.NewServiceVirtualNodeProvider(config.VNAgentPort, config.VNAgentNamespacedName, client, defaultLabelsToSync, taintsToSync)
	}
	if featuregate.DefaultFeatureGate.Enabled(featuregate.VNodeProviderPodIP) {
		return pod.NewPodVirtualNodeProvider(config.VNAgentPort, config.VNAgentNamespacedName, config.VNAgentLabelSelector, client, defaultLabelsToSync, taintsToSync)
	}
	return native.NewNativeVirtualNodeProvider(config.VNAgentPort, defaultLabelsToSync, taintsToSync)
}

func NewVirtualNode(vNodeProvider provider.VirtualNodeProvider, node *corev1.Node) (vnode *corev1.Node, err error) {
	now := metav1.Now()
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   node.Name,
			Labels: provider.GetNodeLabels(vNodeProvider, node),
		},
		Spec: corev1.NodeSpec{
			Unschedulable: true,
			Taints:        provider.GetNodeTaints(vNodeProvider, node, now),
		},
	}

	// fill in status
	n.Status.Conditions = nodeConditions()
	de, err := vNodeProvider.GetNodeDaemonEndpoints(node)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "get node daemon endpoints from provider")
	}
	n.Status.DaemonEndpoints = de

	na, err := vNodeProvider.GetNodeAddress(node)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "get node address from provider")
	}

	n.Status.Addresses = na
	n.Status.NodeInfo = node.Status.NodeInfo
	n.Status.Capacity = node.Status.Capacity
	n.Status.Allocatable = node.Status.Allocatable

	return n, nil
}

var defaultLabelsToSync = map[string]struct{}{
	corev1.LabelOSStable:   {},
	corev1.LabelArchStable: {},
	corev1.LabelHostname:   {},
}

func nodeConditions() []corev1.NodeCondition {
	return []corev1.NodeCondition{
		{
			Type:               "Ready",
			Status:             corev1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "kubelet is ready.",
		},
		{
			Type:               "OutOfDisk",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientDisk",
			Message:            "kubelet has sufficient disk space available",
		},
		{
			Type:               "MemoryPressure",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               "DiskPressure",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               "NetworkUnavailable",
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "RouteCreated",
			Message:            "RouteController created a route",
		},
	}
}

func UpdateNode(client v1core.NodeInterface, node, newNode *corev1.Node) error {
	_, _, err := patchNodeStatus(client, types.NodeName(node.Name), node, newNode)
	return err
	// if err != nil {
	// 	return err
	// }
	// _, err = patchNode(client, types.NodeName(updatedNode.Name), updatedNode, newNode)
	// return err
}

func patchNode(nodes v1core.NodeInterface, nodeName types.NodeName, oldNode *corev1.Node, newNode *corev1.Node) (*corev1.Node, error) {
	oldData, err := json.Marshal(oldNode)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old node %#v for node %q: %v", oldNode, nodeName, err)
	}

	newNodeClone := oldNode.DeepCopy()
	newNodeClone.Spec = newNode.Spec
	newNodeClone.ObjectMeta = newNode.ObjectMeta

	newData, err := json.Marshal(newNodeClone)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new node %#v for node %q: %v", newNodeClone, nodeName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return nil, fmt.Errorf("failed to create patch for node %q: %v", nodeName, err)
	}

	return nodes.Patch(context.TODO(), string(nodeName), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
}

// patchNodeStatus patches node status.
// Copied from github.com/kubernetes/kubernetes/pkg/util/node
func patchNodeStatus(nodes v1core.NodeInterface, nodeName types.NodeName, oldNode *corev1.Node, newNode *corev1.Node) (*corev1.Node, []byte, error) {
	patchBytes, err := preparePatchBytesforNodeStatus(nodeName, oldNode, newNode)
	if err != nil {
		return nil, nil, err
	}

	updatedNode, err := nodes.Patch(context.TODO(), string(nodeName), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to patch status %q for node %q: %v", patchBytes, nodeName, err)
	}
	return updatedNode, patchBytes, nil
}

func preparePatchBytesforNodeStatus(nodeName types.NodeName, oldNode *corev1.Node, newNode *corev1.Node) ([]byte, error) {
	oldData, err := json.Marshal(oldNode)
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal oldData for node %q: %v", nodeName, err)
	}

	// NodeStatus.Addresses is incorrectly annotated as patchStrategy=merge, which
	// will cause strategicpatch.CreateTwoWayMergePatch to create an incorrect patch
	// if it changed.
	manuallyPatchAddresses := (len(oldNode.Status.Addresses) > 0) && !equality.Semantic.DeepEqual(oldNode.Status.Addresses, newNode.Status.Addresses)

	var newAddresses []corev1.NodeAddress
	if manuallyPatchAddresses {
		newAddresses = newNode.Status.Addresses
		newNode.Status.Addresses = oldNode.Status.Addresses
	}
	newData, err := json.Marshal(newNode)
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal newData for node %q: %v", nodeName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return nil, fmt.Errorf("failed to CreateTwoWayMergePatch for node %q: %v", nodeName, err)
	}

	if manuallyPatchAddresses {
		patchBytes, err = fixupPatchForNodeStatusAddresses(patchBytes, newAddresses)
		if err != nil {
			return nil, fmt.Errorf("failed to fix up NodeAddresses in patch for node %q: %v", nodeName, err)
		}
	}
	return patchBytes, nil
}

// fixupPatchForNodeStatusAddresses adds a replace-strategy patch for Status.Addresses to
// the existing patch
func fixupPatchForNodeStatusAddresses(patchBytes []byte, addresses []corev1.NodeAddress) ([]byte, error) {
	// Given patchBytes='{"status": {"conditions": [ ... ], "phase": ...}}' and
	// addresses=[{"type": "InternalIP", "address": "10.0.0.1"}], we need to generate:
	//
	//   {
	//     "status": {
	//       "conditions": [ ... ],
	//       "phase": ...,
	//       "addresses": [
	//         {
	//           "type": "InternalIP",
	//           "address": "10.0.0.1"
	//         },
	//         {
	//           "$patch": "replace"
	//         }
	//       ]
	//     }
	//   }

	var patchMap map[string]interface{}
	if err := json.Unmarshal(patchBytes, &patchMap); err != nil {
		return nil, err
	}

	addrBytes, err := json.Marshal(addresses)
	if err != nil {
		return nil, err
	}
	var addrArray []interface{}
	if err := json.Unmarshal(addrBytes, &addrArray); err != nil {
		return nil, err
	}
	addrArray = append(addrArray, map[string]interface{}{"$patch": "replace"})

	status := patchMap["status"]
	if status == nil {
		status = map[string]interface{}{}
		patchMap["status"] = status
	}
	statusMap, ok := status.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data in patch")
	}
	statusMap["addresses"] = addrArray

	return json.Marshal(patchMap)
}
