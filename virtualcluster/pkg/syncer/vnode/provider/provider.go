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

package provider

import (
	corev1 "k8s.io/api/core/v1"
)

// VirtualNodeProvider is the interface used for registering the node address.
type VirtualNodeProvider interface {
	GetNodeDaemonEndpoints(node *corev1.Node) (corev1.NodeDaemonEndpoints, error)
	GetNodeAddress(node *corev1.Node) ([]corev1.NodeAddress, error)
	GetLabelsToSync() map[string]struct{}
}

// GetNodeLabels is used to sync allowed node labels to vNode
func GetNodeLabels(p VirtualNodeProvider, node *corev1.Node, labels map[string]string) map[string]string {
	labelsToSync := p.GetLabelsToSync()
	for k, v := range node.GetLabels() {
		if _, found := labelsToSync[k]; found {
			labels[k] = v
		}
	}
	return labels
}
