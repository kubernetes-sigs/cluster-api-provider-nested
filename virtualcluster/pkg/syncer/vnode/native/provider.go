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

package native

import (
	corev1 "k8s.io/api/core/v1"

	vnodeprovider "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/provider"
)

type provider struct {
	vnAgentPort  int32
	labelsToSync map[string]struct{}
}

var _ vnodeprovider.VirtualNodeProvider = &provider{}

func NewNativeVirtualNodeProvider(vnAgentPort int32, labelsToSync map[string]struct{}) vnodeprovider.VirtualNodeProvider {
	return &provider{vnAgentPort: vnAgentPort, labelsToSync: labelsToSync}
}

func (p *provider) GetNodeDaemonEndpoints(node *corev1.Node) (corev1.NodeDaemonEndpoints, error) {
	return corev1.NodeDaemonEndpoints{
		KubeletEndpoint: corev1.DaemonEndpoint{
			Port: p.vnAgentPort,
		},
	}, nil
}

func (p *provider) GetNodeAddress(node *corev1.Node) ([]corev1.NodeAddress, error) {
	var addresses []corev1.NodeAddress
	for _, a := range node.Status.Addresses {
		// notes: drop host name address because tenant apiserver using cluster dns.
		// It could not find the node by hostname through this dns.
		if a.Type != corev1.NodeHostName {
			addresses = append(addresses, a)
		}
	}

	return addresses, nil
}

func (p *provider) GetLabelsToSync() map[string]struct{} {
	return p.labelsToSync
}
