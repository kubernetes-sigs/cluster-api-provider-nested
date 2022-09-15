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

package native

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vnodeprovider "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/provider"
)

func newNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"a": "test-0",
				"b": "test-1",
			},
		},
	}
}

func Test_provider_GetNodeLabels(t *testing.T) {
	type fields struct {
		labelsToSync map[string]struct{}
	}
	type args struct {
		node *corev1.Node
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]string
	}{
		{
			name:   "TestWithNoLabels",
			fields: fields{},
			args:   args{newNode()},
			want: map[string]string{
				"tenancy.x-k8s.io/virtualnode": "true",
			},
		},
		{
			name: "TestWithOneLabel",
			fields: fields{
				labelsToSync: map[string]struct{}{
					"a": {},
				},
			},
			args: args{newNode()},
			want: map[string]string{
				"tenancy.x-k8s.io/virtualnode": "true",
				"a":                            "test-0",
			},
		},
		{
			name: "TestWithManyLabels",
			fields: fields{
				labelsToSync: map[string]struct{}{
					"a": {},
					"b": {},
					"c": {},
				},
			},
			args: args{newNode()},
			want: map[string]string{
				"tenancy.x-k8s.io/virtualnode": "true",
				"a":                            "test-0",
				"b":                            "test-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewNativeVirtualNodeProvider(8080, tt.fields.labelsToSync)
			got := vnodeprovider.GetNodeLabels(p, tt.args.node)
			if len(tt.want) != 0 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("vnodeprovider.GetNodeLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
