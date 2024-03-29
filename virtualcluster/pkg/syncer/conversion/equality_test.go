/*
Copyright 2019 The Kubernetes Authors.

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

package conversion

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/apis/config"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
)

func TestCheckDWKVEquality(t *testing.T) {
	syncerConfig := &config.SyncerConfiguration{
		DefaultOpaqueMetaDomains: []string{"kubernetes.io"},
	}
	vc := v1alpha1.VirtualCluster{
		Spec: v1alpha1.VirtualClusterSpec{
			TransparentMetaPrefixes: []string{"tp.x-k8s.io"},
			OpaqueMetaPrefixes:      []string{"tenancy.x-k8s.io"},
		},
	}
	for _, tt := range []struct {
		name     string
		super    map[string]string
		virtual  map[string]string
		isEqual  bool
		expected map[string]string
	}{
		{
			name:     "both empty",
			super:    nil,
			virtual:  nil,
			isEqual:  true,
			expected: nil,
		},
		{
			name:  "empty super",
			super: nil,
			virtual: map[string]string{
				"a": "b",
			},
			isEqual: false,
			expected: map[string]string{
				"a": "b",
			},
		},
		{
			name: "equal",
			super: map[string]string{
				"a": "b",
			},
			virtual: map[string]string{
				"a": "b",
			},
			isEqual:  true,
			expected: nil,
		},
		{
			name: "not equal",
			super: map[string]string{
				"a": "b",
			},
			virtual: map[string]string{
				"b": "c",
				"a": "c",
			},
			isEqual: false,
			expected: map[string]string{
				"a": "c",
				"b": "c",
			},
		},
		{
			name: "less key",
			super: map[string]string{
				"a": "b",
				"b": "c",
			},
			virtual: map[string]string{
				"a": "c",
			},
			isEqual: false,
			expected: map[string]string{
				"a": "c",
			},
		},
		{
			name: "empty key",
			super: map[string]string{
				"a": "b",
				"b": "c",
			},
			virtual:  nil,
			isEqual:  false,
			expected: nil,
		},
		{
			name: "limiting key",
			super: map[string]string{
				"a": "b",
			},
			virtual: map[string]string{
				"a":                     "b",
				"tenancy.x-k8s.io/name": "name",
			},
			isEqual:  true,
			expected: nil,
		},
		{
			name: "limiting key and less key",
			super: map[string]string{
				"a":                     "b",
				"tenancy.x-k8s.io/name": "name",
			},
			virtual: nil,
			isEqual: false,
			expected: map[string]string{
				"tenancy.x-k8s.io/name": "name",
			},
		},
		{
			name: "ignore transparent key",
			super: map[string]string{
				"tenancy.x-k8s.io/name": "name",
				"tp.x-k8s.io/foo":       "val",
			},
			virtual:  nil,
			isEqual:  true,
			expected: nil,
		},
		{
			name: "exactly matches defaultOpaqueMetaDomain",
			super: map[string]string{
				"a":               "b",
				"kubernetes.io/a": "b",
			},
			virtual: map[string]string{
				"a":               "b",
				"kubernetes.io/b": "c",
			},
			isEqual: true,
		},
		{
			name: "subdomain matches defaultOpaqueMetaDomain",
			super: map[string]string{
				"a":                   "b",
				"foo.kubernetes.io/a": "b",
			},
			virtual: map[string]string{
				"a":                   "b",
				"kubernetes.io/b":     "c",
				"foo.kubernetes.io":   "a",
				"bar.kubernetes.io/b": "c",
			},
			isEqual: true,
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			got, equal := Equality(syncerConfig, &vc).checkDWKVEquality(tt.super, tt.virtual)
			if equal != tt.isEqual {
				tc.Errorf("expected equal %v, got %v %v", tt.isEqual, equal, got)
			} else {
				if !equality.Semantic.DeepEqual(got, tt.expected) {
					tc.Errorf("expected result %+v, got %+v", tt.expected, got)
				}
			}
		})
	}
}

func TestCheckUWKVEquality(t *testing.T) {
	vc := v1alpha1.VirtualCluster{
		Spec: v1alpha1.VirtualClusterSpec{
			TransparentMetaPrefixes: []string{"tp.x-k8s.io", "tp1.x-k8s.io"},
			OpaqueMetaPrefixes:      []string{"tenancy.x-k8s.io"},
		},
	}
	for _, tt := range []struct {
		name     string
		super    map[string]string
		virtual  map[string]string
		isEqual  bool
		expected map[string]string
	}{
		{
			name: "equal - no transparent keys",
			super: map[string]string{
				"a": "b",
			},
			virtual: map[string]string{
				"a": "b",
			},
			isEqual:  true,
			expected: nil,
		},
		{
			name: "not equal - no transparent keys",
			super: map[string]string{
				"a":                "b",
				"tenancy.x-k8s.io": "c",
			},
			virtual: map[string]string{
				"a": "c",
			},
			isEqual:  true,
			expected: nil,
		},
		{
			name: "miss one transparent key",
			super: map[string]string{
				"a":           "b",
				"b":           "c",
				"tp.x-k8s.io": "a",
			},
			virtual: nil,
			isEqual: false,
			expected: map[string]string{
				"tp.x-k8s.io": "a",
			},
		},
		{
			name: "miss two transparent keys",
			super: map[string]string{
				"a":            "b",
				"tp.x-k8s.io":  "a",
				"tp1.x-k8s.io": "a",
			},
			virtual: map[string]string{
				"a": "b",
			},
			isEqual: false,
			expected: map[string]string{
				"a":            "b",
				"tp.x-k8s.io":  "a",
				"tp1.x-k8s.io": "a",
			},
		},
		{
			name: "has all transparent keys",
			super: map[string]string{
				"a":            "b",
				"tp.x-k8s.io":  "a",
				"tp1.x-k8s.io": "a",
			},
			virtual: map[string]string{
				"tp.x-k8s.io":  "a",
				"tp1.x-k8s.io": "a",
			},
			isEqual:  true,
			expected: nil,
		},
		{
			name: "wrong transparent key-val",
			super: map[string]string{
				"tenancy.x-k8s.io/name": "name",
				"tp.x-k8s.io/foo":       "a",
			},
			virtual: map[string]string{
				"tp.x-k8s.io/foo": "b",
			},
			isEqual: false,
			expected: map[string]string{
				"tp.x-k8s.io/foo": "a",
			},
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			got, equal := Equality(nil, &vc).checkUWKVEquality(tt.super, tt.virtual)
			if equal != tt.isEqual {
				tc.Errorf("expected equal %v, got %v", tt.isEqual, equal)
			} else {
				if !equality.Semantic.DeepEqual(got, tt.expected) {
					tc.Errorf("expected result %+v, got %+v", tt.expected, got)
				}
			}
		})
	}
}

func TestCheckContainersImageEquality(t *testing.T) {
	for _, tt := range []struct {
		name     string
		pObj     []v1.Container
		vObj     []v1.Container
		expected []v1.Container
	}{
		{
			name: "equal",
			pObj: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image2",
				},
			},
			vObj: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image2",
				},
			},
			expected: nil,
		},
		{
			name: "equal, container added by webhook",
			pObj: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image2",
				},
			},
			vObj: []v1.Container{
				{
					Name:  "c2",
					Image: "image2",
				},
			},
			expected: nil,
		},
		{
			name: "not equal",
			pObj: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image2",
				},
			},
			vObj: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image3",
				},
			},
			expected: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image3",
				},
			},
		},
		{
			name: "not equal, container added by webhook",
			pObj: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image2",
				},
			},
			vObj: []v1.Container{
				{
					Name:  "c2",
					Image: "image3",
				},
			},
			expected: []v1.Container{
				{
					Name:  "c1",
					Image: "image1",
				},
				{
					Name:  "c2",
					Image: "image3",
				},
			},
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			got := Equality(nil, nil).checkContainersImageEquality(tt.pObj, tt.vObj)
			if !equality.Semantic.DeepEqual(got, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, got)
			}
		})
	}
}

func TestCheckActiveDeadlineSecondsEquality(t *testing.T) {
	for _, tt := range []struct {
		name       string
		pObj       *int64
		vObj       *int64
		isEqual    bool
		updatedVal *int64
	}{
		{
			name:       "both nil",
			pObj:       nil,
			vObj:       nil,
			isEqual:    true,
			updatedVal: nil,
		},
		{
			name:       "both not nil and equal",
			pObj:       pointer.Int64Ptr(1),
			vObj:       pointer.Int64Ptr(1),
			isEqual:    true,
			updatedVal: nil,
		},
		{
			name:       "both not nil but not equal",
			pObj:       pointer.Int64Ptr(1),
			vObj:       pointer.Int64Ptr(2),
			isEqual:    false,
			updatedVal: pointer.Int64Ptr(2),
		},
		{
			name:       "updated to nil",
			pObj:       pointer.Int64Ptr(1),
			vObj:       nil,
			isEqual:    false,
			updatedVal: nil,
		},
		{
			name:       "updated to value",
			pObj:       nil,
			vObj:       pointer.Int64Ptr(1),
			isEqual:    false,
			updatedVal: pointer.Int64Ptr(1),
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			val, equal := Equality(nil, nil).checkInt64Equality(tt.pObj, tt.vObj)
			if equal != tt.isEqual {
				tc.Errorf("expected equal %v, got %v", tt.isEqual, equal)
			}
			if !equal {
				if !equality.Semantic.DeepEqual(val, tt.updatedVal) {
					tc.Errorf("expected val %v, got %v", tt.updatedVal, val)
				}
			}
		})
	}
}

func TestCheckUWPodStatusEquality(t *testing.T) {
	for _, tt := range []struct {
		name       string
		pObj       *v1.Pod
		vObj       *v1.Pod
		updatedVal *v1.PodStatus
	}{
		{
			name: "equal",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
					},
				},
			},
			updatedVal: nil,
		},
		{
			name: "not equal",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
					},
				},
			},
			updatedVal: &v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:    "a",
						Message: "bbb",
						Reason:  "bbb",
					},
				},
			},
		},
		{
			name: "no readiness condition in super",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: "test-gate",
						},
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
						{
							Type:    "test-gate",
							Message: "test",
							Reason:  "test",
						},
					},
				},
			},
			updatedVal: &v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:    "a",
						Message: "bbb",
						Reason:  "bbb",
					},
					{
						Type:    "test-gate",
						Message: "test",
						Reason:  "test",
					},
				},
			},
		},
		{
			name: "not equal in readiness",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
						{
							Type:    "test-gate",
							Message: "test2",
							Reason:  "test2",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: "test-gate",
						},
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
						{
							Type:    "test-gate",
							Message: "test",
							Reason:  "test",
						},
					},
				},
			},
			updatedVal: &v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:    "a",
						Message: "bbb",
						Reason:  "bbb",
					},
					{
						Type:    "test-gate",
						Message: "test",
						Reason:  "test",
					},
				},
			},
		},
		{
			name: "not equal in readiness and more conditions in super",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "c",
							Message: "ccc",
							Reason:  "ccc",
						},
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
						{
							Type:    "test-gate",
							Message: "test2",
							Reason:  "test2",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: "test-gate",
						},
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
						{
							Type:    "test-gate",
							Message: "test",
							Reason:  "test",
						},
					},
				},
			},
			updatedVal: &v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:    "c",
						Message: "ccc",
						Reason:  "ccc",
					},
					{
						Type:    "a",
						Message: "bbb",
						Reason:  "bbb",
					},
					{
						Type:    "test-gate",
						Message: "test",
						Reason:  "test",
					},
				},
			},
		},
		{
			name: "readiness gate exists in super and doesn't exist in tenant",
			pObj: &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: "super-gate",
						},
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "c",
							Message: "ccc",
							Reason:  "ccc",
						},
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
						{
							Type:    "test-gate",
							Message: "test2",
							Reason:  "test2",
						},
						{
							Type:    "super-gate",
							Message: "super",
							Reason:  "super",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: "test-gate",
						},
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
						{
							Type:    "test-gate",
							Message: "test",
							Reason:  "test",
						},
					},
				},
			},
			updatedVal: &v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:    "c",
						Message: "ccc",
						Reason:  "ccc",
					},
					{
						Type:    "a",
						Message: "bbb",
						Reason:  "bbb",
					},
					{
						Type:    "test-gate",
						Message: "test",
						Reason:  "test",
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			val := Equality(nil, nil).CheckUWPodStatusEquality(tt.pObj, tt.vObj)
			if !equality.Semantic.DeepEqual(val, tt.updatedVal) {
				tc.Errorf("expected val %v, got %v", tt.updatedVal, val)
			}
		})
	}
}

func TestCheckDWPodConditionEquality(t *testing.T) {
	for _, tt := range []struct {
		name       string
		pObj       *v1.Pod
		vObj       *v1.Pod
		updatedVal *v1.PodStatus
	}{
		{
			name: "equal",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
					},
				},
			},
			updatedVal: nil,
		},
		{
			name: "not diff in readiness gate condition",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
					},
				},
			},
			updatedVal: nil,
		},
		{
			name: "diff in readiness gate condition",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
						{
							Type:    "test-gate",
							Message: "test2",
							Reason:  "test2",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: "test-gate",
						},
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
						{
							Type:    "test-gate",
							Message: "test",
							Reason:  "test",
						},
					},
				},
			},
			updatedVal: &v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:    "a",
						Message: "bbb",
						Reason:  "bbb",
					},
					{
						Type:    "test-gate",
						Message: "test",
						Reason:  "test",
					},
				},
			},
		},
		{
			name: "missing readiness gate condition in super",
			pObj: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "bbb",
							Reason:  "bbb",
						},
					},
				},
			},
			vObj: &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: "test-gate",
						},
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:    "a",
							Message: "aaa",
							Reason:  "aaa",
						},
						{
							Type:    "test-gate",
							Message: "test",
							Reason:  "test",
						},
					},
				},
			},
			updatedVal: &v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:    "a",
						Message: "bbb",
						Reason:  "bbb",
					},
					{
						Type:    "test-gate",
						Message: "test",
						Reason:  "test",
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			val := CheckDWPodConditionEquality(tt.pObj, tt.vObj)
			if !equality.Semantic.DeepEqual(val, tt.updatedVal) {
				tc.Errorf("expected val %v, got %v", tt.updatedVal, val)
			}
		})
	}
}

func TestCheckUWPVCStatusEquality(t *testing.T) {
	featuregate.DefaultFeatureGate.Set(featuregate.SyncTenantPVCStatusPhase, true)
	for _, tt := range []struct {
		name       string
		pObj       *v1.PersistentVolumeClaim
		vObj       *v1.PersistentVolumeClaim
		updatedObj *v1.PersistentVolumeClaim
	}{
		{
			name: "pPVC is pending and vPVC is pending",
			pObj: &v1.PersistentVolumeClaim{
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimPending,
				},
			},
			vObj: &v1.PersistentVolumeClaim{
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimPending,
				},
			},
			updatedObj: nil,
		},
		{
			name: "pPVC is bound and vPVC is bound",
			pObj: &v1.PersistentVolumeClaim{
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimBound,
				},
			},
			vObj: &v1.PersistentVolumeClaim{
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimBound,
				},
			},
			updatedObj: nil,
		},
		{
			name: "pPVC is lost and vPVC is lost",
			pObj: &v1.PersistentVolumeClaim{
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimLost,
				},
			},
			vObj: &v1.PersistentVolumeClaim{
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimLost,
				},
			},
			updatedObj: nil,
		},
		{
			name: "pPVC is lost and vPVC is bound",
			pObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimLost,
				},
			},
			vObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimBound,
				},
			},
			updatedObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimLost,
				},
			},
		},
		{
			name: "pPVC is pending and vPVC is bound",
			pObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimPending,
				},
			},
			vObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimBound,
				},
			},
			updatedObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimPending,
				},
			},
		},
		{
			name: "pPVC is bound and vPVC is pending",
			pObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimBound,
				},
			},
			vObj: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vPVC",
				},
				Status: v1.PersistentVolumeClaimStatus{
					Phase: v1.ClaimPending,
				},
			},
			updatedObj: nil,
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			obj := Equality(nil, nil).CheckUWPVCStatusEquality(tt.pObj, tt.vObj)
			if obj != nil && obj.Status.Phase != tt.updatedObj.Status.Phase {
				tc.Errorf("expected vPVC's Status.Phase: %v, got: %v", tt.updatedObj.Status.Phase, obj.Status.Phase)
			}
		})
	}
}
