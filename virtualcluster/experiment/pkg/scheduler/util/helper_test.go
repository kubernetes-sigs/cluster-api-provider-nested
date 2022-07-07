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

package util

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Equals(a corev1.ResourceList, b corev1.ResourceList) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value1 := range a {
		value2, found := b[key]
		if !found {
			return false
		}
		if value1.Cmp(value2) != 0 {
			return false
		}
	}
	return true
}

func TestGetTotalNodeCapacity(t *testing.T) {
	testcases := map[string]struct {
		nodelist *corev1.NodeList
		expect   corev1.ResourceList
	}{
		"one node": {
			nodelist: &corev1.NodeList{
				Items: []corev1.Node{
					{
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								"cpu":    resource.MustParse("0.5"),
								"memory": resource.MustParse("10485760Ki"),
							},
							Conditions: []corev1.NodeCondition{
								{
									Status: corev1.ConditionTrue,
									Type:   corev1.NodeReady,
								},
							},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0.5"),
				"memory": resource.MustParse("10Gi"),
			},
		},
		"two nodes": {
			nodelist: &corev1.NodeList{
				Items: []corev1.Node{
					{
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								"cpu":    resource.MustParse("1.8"),
								"memory": resource.MustParse("2048Mi"),
							},
							Conditions: []corev1.NodeCondition{
								{
									Status: corev1.ConditionTrue,
									Type:   corev1.NodeReady,
								},
							},
						},
					},
					{
						Status: corev1.NodeStatus{
							Capacity: corev1.ResourceList{
								"cpu":    resource.MustParse("0.5"),
								"memory": resource.MustParse("10485760Ki"),
							},
							Conditions: []corev1.NodeCondition{
								{
									Status: corev1.ConditionTrue,
									Type:   corev1.NodeReady,
								},
							},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("2.3"),
				"memory": resource.MustParse("12Gi"),
			},
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {

			total := getTotalNodeCapacity(tc.nodelist)

			if !Equals(tc.expect, total) {
				t.Errorf("the total capacity is not expected. Exp: %v, Got %v", tc.expect, total)
			}
		})
	}
}

func TestGetMaxQuota(t *testing.T) {
	testcases := map[string]struct {
		quotalist *corev1.ResourceQuotaList
		expect    corev1.ResourceList
	}{
		"case 1": {
			quotalist: &corev1.ResourceQuotaList{
				Items: []corev1.ResourceQuota{
					{
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("0.5"),
								"memory": resource.MustParse("10485760Ki"),
							},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0.5"),
				"memory": resource.MustParse("10Gi"),
			},
		},
		"case 2": {
			quotalist: &corev1.ResourceQuotaList{
				Items: []corev1.ResourceQuota{
					{
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("0.5"),
								"memory": resource.MustParse("10485760Ki"),
							},
						},
					},

					{
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("0.7"),
								"memory": resource.MustParse("3Gi"),
							},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0.7"),
				"memory": resource.MustParse("10Gi"),
			},
		},
		"case 3": {
			quotalist: &corev1.ResourceQuotaList{
				Items: []corev1.ResourceQuota{
					{
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("0.5"),
								"memory": resource.MustParse("10485760Ki"),
							},
						},
					},

					{
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0.5"),
				"memory": resource.MustParse("10Gi"),
			},
		},
		"case 4": {
			quotalist: &corev1.ResourceQuotaList{
				Items: []corev1.ResourceQuota{
					{
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{},
						},
					},

					{
						Spec: corev1.ResourceQuotaSpec{
							Hard: corev1.ResourceList{},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0"),
				"memory": resource.MustParse("0"),
			},
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			max := GetMaxQuota(tc.quotalist)
			if !Equals(tc.expect, max) {
				t.Errorf("the max capacity is not expected. Exp: %v, Got %v", tc.expect, max)
			}
		})
	}
}

func TestGetPodRequirements(t *testing.T) {
	testcases := map[string]struct {
		pod    *corev1.Pod
		expect corev1.ResourceList
	}{
		"case 1": {
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("0.5"),
									"memory": resource.MustParse("10485760Ki"),
								},
							},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0.5"),
				"memory": resource.MustParse("10Gi"),
			},
		},
		"case 2": {
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("0.5"),
									"memory": resource.MustParse("10485760Ki"),
								},
							},
						},
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("2.5"),
									"memory": resource.MustParse("6Gi"),
								},
							},
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("3"),
				"memory": resource.MustParse("16Gi"),
			},
		},
		"case 3": {
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("0.5"),
									"memory": resource.MustParse("10485760Ki"),
								},
							},
						},
						{
							Name: "empty",
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0.5"),
				"memory": resource.MustParse("10Gi"),
			},
		},
		"case 4": {
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{},
						},
						{
							Name: "empty",
						},
					},
				},
			},
			expect: corev1.ResourceList{
				"cpu":    resource.MustParse("0"),
				"memory": resource.MustParse("0"),
			},
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			total := GetPodRequirements(tc.pod)

			if !Equals(tc.expect, total) {
				t.Errorf("the total pod requests is not expected. Exp: %v, Got %v", tc.expect, total)
			}
		})
	}
}
