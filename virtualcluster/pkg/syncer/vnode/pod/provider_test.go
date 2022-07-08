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

package pod

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func newNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.0.2",
				},
			},
		},
	}
}

func newClient() clientset.Interface {
	return fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vn-agent-12345",
			Namespace: "vc-manager",
			Labels: map[string]string{
				"app": "vn-agent",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node1",
		},
		Status: corev1.PodStatus{
			PodIP: "192.168.0.5",
		},
	})
}

func Test_provider_GetNodeAddress(t *testing.T) {
	type fields struct {
		vnAgentPort          int32
		vnAgentNamespaceName string
		vnAgentLabelSelector string
		client               clientset.Interface
	}
	type args struct {
		node *corev1.Node
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []corev1.NodeAddress
		wantErr bool
	}{
		{
			name: "TestWithNoPods",
			fields: fields{
				vnAgentNamespaceName: "default/vn-agent",
				vnAgentLabelSelector: "no=pods",
				client:               newClient(),
			},
			args:    args{newNode("node1")},
			want:    []corev1.NodeAddress{},
			wantErr: true,
		},
		{
			name: "TestWithPodsButWrongNodeName",
			fields: fields{
				vnAgentNamespaceName: "vc-manager/vn-agent",
				vnAgentLabelSelector: "app=vn-agent",
				client:               newClient(),
			},
			args:    args{newNode("node2")},
			want:    []corev1.NodeAddress{},
			wantErr: true,
		},
		{
			name: "TestWithPods",
			fields: fields{
				vnAgentNamespaceName: "vc-manager/vn-agent",
				vnAgentLabelSelector: "app=vn-agent",
				client:               newClient(),
			},
			args: args{newNode("node1")},
			want: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "192.168.0.5",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				vnAgentPort:          tt.fields.vnAgentPort,
				vnAgentNamespaceName: tt.fields.vnAgentNamespaceName,
				vnAgentLabelSelector: tt.fields.vnAgentLabelSelector,
				client:               tt.fields.client,
			}
			got, err := p.GetNodeAddress(tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("provider.GetNodeAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(tt.want) != 0 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("provider.GetNodeAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
