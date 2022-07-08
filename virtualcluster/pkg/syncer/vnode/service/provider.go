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

package service

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	vnodeprovider "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/provider"
)

type provider struct {
	vnAgentPort          int32
	vnAgentNamespaceName string
	client               clientset.Interface
}

var _ vnodeprovider.VirtualNodeProvider = &provider{}

func NewServiceVirtualNodeProvider(vnAgentPort int32, vnAgentNamespaceName string, client clientset.Interface) vnodeprovider.VirtualNodeProvider {
	return &provider{
		vnAgentPort:          vnAgentPort,
		vnAgentNamespaceName: vnAgentNamespaceName,
		client:               client,
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
	svc, err := p.client.CoreV1().Services(namespaceName[0]).Get(context.TODO(), namespaceName[1], metav1.GetOptions{})
	if err != nil {
		return addresses, err
	}

	addresses = append(addresses, corev1.NodeAddress{
		Type:    corev1.NodeInternalIP,
		Address: svc.Spec.ClusterIP,
	})
	return addresses, nil
}
