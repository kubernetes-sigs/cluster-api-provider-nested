/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package net

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	GetLBIPTimeoutSec = 30
	GetLBIPPeriodSec  = 2
)

// GetSvcNodePort returns the nodePort of service with given name
// in given namespace
func GetSvcNodePort(name, namespace string, cli client.Client) (int32, error) {
	svc := &corev1.Service{}
	err := cli.Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name}, svc)
	if err != nil {
		return 0, err
	}
	return svc.Spec.Ports[0].NodePort, nil
}

// GetNodeIP returns a node IP address
func GetNodeIP(cli client.Client) (string, error) {
	nodeLst := &corev1.NodeList{}
	if err := cli.List(context.TODO(), nodeLst); err != nil {
		return "", err
	}
	if len(nodeLst.Items) == 0 {
		return "", errors.New("there is no available nodes")
	}

	for _, node := range nodeLst.Items {
		// Check if the node has the ready status condition
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				// Look for internal IP
				for _, addr := range node.Status.Addresses {
					if addr.Type == corev1.NodeInternalIP {
						return addr.Address, nil
					}
				}
			}
		}
	}

	return "", errors.New("there is no 'NodeInternalIP' type address with Ready status")
}

// GetLBIP returns the external ip address assigned to Loadbalancer by
// cloud provider
func GetLBIP(name, namespace string, cli client.Client) (string, error) {
	timeout := time.After(GetLBIPTimeoutSec * time.Second)
	for {
		period := time.After(GetLBIPPeriodSec * time.Second)
		select {
		case <-timeout:
			return "", fmt.Errorf("get LoadBalancer IP timeout for svc %s:%s",
				namespace, name)
		case <-period:
			// if external IP is not assigned to LB yet, we will
			// retry to get in period second
			svc := &corev1.Service{}
			err := cli.Get(context.TODO(), types.NamespacedName{
				Namespace: namespace,
				Name:      name}, svc)
			if err != nil {
				return "", err
			}
			if len(svc.Status.LoadBalancer.Ingress) != 0 {
				if svc.Status.LoadBalancer.Ingress[0].IP == "" {
					return svc.Status.LoadBalancer.Ingress[0].Hostname, nil
				}
				return svc.Status.LoadBalancer.Ingress[0].IP, nil
			}
		}
	}
}
