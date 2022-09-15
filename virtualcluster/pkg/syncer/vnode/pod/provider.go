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

// Package pod allows you to configure the syncer with vnodes backed by a vn-agent running without hostNetworking
//
// WARNING: For anyone implementing this you should have a node level check for example node-exporter or an
// initContainer on the vn-agent which updates a condition on the Node or other part of the node object. By doing
// this the syncer will update all your VirtualClusters VNodes anytime the vn-agent is rescheduled, this can be
// done without rebinding.
package pod

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	vnodeprovider "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/provider"
)

type provider struct {
	vnAgentPort          int32
	vnAgentNamespaceName string
	vnAgentLabelSelector string
	labelsToSync         map[string]struct{}
	taintsToSync         map[string]struct{}
	client               clientset.Interface
}

var _ vnodeprovider.VirtualNodeProvider = &provider{}

func NewPodVirtualNodeProvider(vnAgentPort int32, vnAgentNamespaceName, vnAgentLabelSelector string, client clientset.Interface, labelsToSync, taintsToSync map[string]struct{}) vnodeprovider.VirtualNodeProvider {
	return &provider{
		vnAgentPort:          vnAgentPort,
		vnAgentNamespaceName: vnAgentNamespaceName,
		vnAgentLabelSelector: vnAgentLabelSelector,
		client:               client,
		labelsToSync:         labelsToSync,
		taintsToSync:         taintsToSync,
	}
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
	namespaceName := strings.Split(p.vnAgentNamespaceName, "/")
	// TODO(christopherhein) Use NodeName informer index to make this more efficient.
	pods, err := p.client.CoreV1().Pods(namespaceName[0]).List(context.TODO(), metav1.ListOptions{LabelSelector: p.vnAgentLabelSelector})
	if err != nil || len(pods.Items) == 0 {
		return addresses, fmt.Errorf("vn-agent pods could not be found %s", err)
	}

	var pod *corev1.Pod
	for podIndex := range pods.Items {
		if pods.Items[podIndex].Spec.NodeName == node.Name {
			pod = &pods.Items[podIndex]
		}
	}
	if pod == nil {
		return addresses, fmt.Errorf("vn-agent pods could not be found, %s", p.vnAgentLabelSelector)
	}

	addresses = append(addresses, corev1.NodeAddress{
		Type:    corev1.NodeInternalIP,
		Address: pod.Status.PodIP,
	})

	return addresses, nil
}

func (p *provider) GetLabelsToSync() map[string]struct{} {
	return p.labelsToSync
}

func (p *provider) GetTaintsToSync() map[string]struct{} {
	return p.taintsToSync
}
