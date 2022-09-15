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
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "a", Effect: corev1.TaintEffectNoSchedule},
				{Key: "b", Effect: corev1.TaintEffectNoExecute},
				{Key: "secret", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}
}

func Test_provider_GetNodeLabels(t *testing.T) {
	type fields struct {
		labelsToSync map[string]struct{}
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]string
	}{
		{
			name:   "TestWithNoLabels",
			fields: fields{},
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
			want: map[string]string{
				"tenancy.x-k8s.io/virtualnode": "true",
				"a":                            "test-0",
				"b":                            "test-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewNativeVirtualNodeProvider(8080, tt.fields.labelsToSync, map[string]struct{}{})
			got := vnodeprovider.GetNodeLabels(p, newNode())
			if len(tt.want) != 0 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("vnodeprovider.GetNodeLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_provider_GetNodeTaints(t *testing.T) {
	type fields struct {
		taintsToSync map[string]struct{}
	}
	now := metav1.Now()
	tests := []struct {
		name   string
		fields fields
		want   []corev1.Taint
	}{
		{
			name:   "TestWithNoTaints",
			fields: fields{},
			want: []corev1.Taint{
				{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule, TimeAdded: &now},
			},
		},
		{
			name: "TestWithOneTaint",
			fields: fields{
				taintsToSync: map[string]struct{}{
					"a": {},
				},
			},
			want: []corev1.Taint{
				{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule, TimeAdded: &now},
				{Key: "a", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		{
			name: "TestWithManyTaints",
			fields: fields{
				taintsToSync: map[string]struct{}{
					"a": {},
					"b": {},
					"c": {},
				},
			},
			want: []corev1.Taint{
				{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule, TimeAdded: &now},
				{Key: "a", Effect: corev1.TaintEffectNoSchedule},
				{Key: "b", Effect: corev1.TaintEffectNoExecute},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewNativeVirtualNodeProvider(8080, map[string]struct{}{}, tt.fields.taintsToSync)
			got := vnodeprovider.GetNodeTaints(p, newNode(), now)
			if taintsDiffer(got, tt.want) {
				t.Errorf("vnodeprovider.GetNodeTaints() = %v, want %v", got, tt.want)
			}
		})
	}
}

// This method is like https://github.com/kubernetes/kubernetes/blob/v1.21.9/pkg/util/taints/taints.go#L324
// but only returns bool
func taintsDiffer(t1, t2 []corev1.Taint) bool {
	for i := range t1 {
		if !taintExists(t2, &t1[i]) {
			return true
		}
	}

	for i := range t2 {
		if !taintExists(t1, &t2[i]) {
			return true
		}
	}

	return false
}

// This is the copy-paste of https://github.com/kubernetes/kubernetes/blob/v1.21.9/pkg/util/taints/taints.go#L315
// as it would be too heavy to import kubernetes lib here
func taintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}
