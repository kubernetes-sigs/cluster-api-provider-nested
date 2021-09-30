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

package manager

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/apis/config"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/cluster"
	mc "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/mccontroller"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
)

type fakeReconciler struct{}

func (f fakeReconciler) Reconcile(reconciler.Request) (reconciler.Result, error) {
	return reconciler.Result{}, nil
}

func MakeMultiClusterControllerWithFakeCluster(tb testing.TB, obj client.Object, objList client.ObjectList, vcList ...*v1alpha1.VirtualCluster) mc.MultiClusterInterface {
	mcc, err := mc.NewMCController(obj, objList, &fakeReconciler{})
	if err != nil {
		tb.Errorf("create mc controller: %v", err)
		return nil
	}

	for _, vc := range vcList {
		fc := cluster.NewFakeTenantCluster(vc, nil, nil)
		err = mcc.RegisterClusterResource(fc, mc.WatchOptions{})
		if err != nil {
			tb.Errorf("register cluster %v", err)
			return nil
		}
		err = mcc.WatchClusterResource(fc, mc.WatchOptions{})
		if err != nil {
			tb.Errorf("add cluster %v", err)
			return nil
		}
	}
	return mcc
}

func TestBuildSuperClusterObject(t *testing.T) {
	syncerConfig := &config.SyncerConfiguration{
		DefaultOpaqueMetaDomains: []string{"kubernetes.io"},
	}

	vc := &v1alpha1.VirtualCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "v1",
			Namespace: "t1",
			UID:       "d64ea0c0-91f8-46f5-8643-c0cab32ab0cd",
		},
		Spec: v1alpha1.VirtualClusterSpec{
			OpaqueMetaPrefixes: []string{"opaque.io"},
		},
	}
	mcc := MakeMultiClusterControllerWithFakeCluster(t, &v1.Pod{}, &v1.PodList{}, vc)
	ct := conversion.Convertor(syncerConfig, mcc)
	time := metav1.Now()

	clusterKey := conversion.ToClusterKey(vc)

	attachVCMeta := func(obj client.Object) error {
		var tenantScopeMetaInAnnotation = map[string]string{
			constants.LabelCluster:     clusterKey,
			constants.LabelNamespace:   "n1",
			constants.LabelVCName:      "v1",
			constants.LabelVCNamespace: "t1",
		}
		anno := obj.GetAnnotations()
		if anno == nil {
			anno = make(map[string]string)
		}
		for k, v := range tenantScopeMetaInAnnotation {
			anno[k] = v
		}
		obj.SetAnnotations(anno)
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		var tenantScopeMetaInLabel = map[string]string{
			constants.LabelVCName:      "v1",
			constants.LabelVCNamespace: "t1",
		}
		for k, v := range tenantScopeMetaInLabel {
			labels[k] = v
		}
		obj.SetLabels(labels)
		return nil
	}

	tests := []struct {
		name        string
		cluster     string
		obj         client.Object
		expectedObj client.Object
		errMsg      string
	}{
		{
			name:    "cluster not found",
			cluster: "not_found",
			obj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "v1",
				},
			},
			errMsg: "get virtualcluster",
		},
		{
			name:    "normal case",
			cluster: clusterKey,
			obj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "v1",
					Labels: map[string]string{
						"k": "v",
					},
					Annotations: map[string]string{
						"k": "v",
					},
				},
			},
			expectedObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: conversion.ToSuperClusterNamespace(clusterKey, "n1"),
					Name:      "v1",
					Labels: map[string]string{
						"k": "v",
					},
					Annotations: map[string]string{
						"k":                            "v",
						constants.LabelUID:             "",
						constants.LabelOwnerReferences: "null",
					},
				},
			},
		},
		{
			name:    "non empty meta",
			cluster: clusterKey,
			obj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "v1",
					UID:       "d64ea111-91f8-46f5-8643-c0cab32ab0cd",
					Labels: map[string]string{
						constants.LabelVCName:      "i want to overwrite",
						constants.LabelVCNamespace: "i want to overwrite",
					},
					Annotations: map[string]string{
						constants.LabelUID:             "i want to overwrite",
						constants.LabelOwnerReferences: "i want to overwrite",
						constants.LabelCluster:         "i want to overwrite",
					},
					ResourceVersion:            "1",
					Generation:                 10001,
					DeletionTimestamp:          &time,
					DeletionGracePeriodSeconds: pointer.Int64(1),
					OwnerReferences: []metav1.OwnerReference{
						{Name: "1"},
					},
					Finalizers: []string{"f"},
				},
			},
			expectedObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: conversion.ToSuperClusterNamespace(clusterKey, "n1"),
					Name:      "v1",
					Annotations: map[string]string{
						constants.LabelUID:             "d64ea111-91f8-46f5-8643-c0cab32ab0cd",
						constants.LabelOwnerReferences: `[{"apiVersion":"","kind":"","name":"1","uid":""}]`,
					},
				},
			},
		},
		{
			name:    "have opaqued keys",
			cluster: clusterKey,
			obj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "v1",
					Labels: map[string]string{
						"k":                 "v",
						"m.kubernetes.io":   "v",
						"m.kubernetes.io/a": "v",
						"kubernetes.io/b":   "v",
						"kubernetes.io":     "v",
						"opaque.io/kk":      "v",
						"m.opaque.io":       "v",
					},
					Annotations: map[string]string{
						"k":               "v",
						"m.kubernetes.io": "v",
						"opaque.io/kk":    "v",
						"m.opaque.io":     "v",
					},
				},
			},
			expectedObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: conversion.ToSuperClusterNamespace(clusterKey, "n1"),
					Name:      "v1",
					Labels: map[string]string{
						"k":           "v",
						"m.opaque.io": "v",
					},
					Annotations: map[string]string{
						"k":                            "v",
						"m.opaque.io":                  "v",
						constants.LabelUID:             "",
						constants.LabelOwnerReferences: "null",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ct.BuildSuperClusterObject(tt.cluster, tt.obj)
			if tt.errMsg != "" {
				if err == nil {
					t.Errorf("expected error %v, got empty", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error %v, got %v", tt.errMsg, err.Error())
					return
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error %v", err)
				return
			}

			err = attachVCMeta(tt.expectedObj)
			if err != nil {
				t.Errorf("unexpected error when attach vc meta: %v", err)
				return
			}
			if !equality.Semantic.DeepEqual(got, tt.expectedObj) {
				g, _ := json.MarshalIndent(got, "", "\t")
				e, _ := json.MarshalIndent(tt.expectedObj, "", "\t")
				t.Errorf("expected %v, got %v", string(e), string(g))
			}
		})
	}
}

func TestBuildSuperClusterNamespace(t *testing.T) {
	syncerConfig := &config.SyncerConfiguration{
		DefaultOpaqueMetaDomains: []string{"kubernetes.io"},
	}

	vc := &v1alpha1.VirtualCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "v1",
			Namespace: "t1",
			UID:       "d64ea0c0-91f8-46f5-8643-c0cab32ab0cd",
		},
		Spec: v1alpha1.VirtualClusterSpec{
			OpaqueMetaPrefixes: []string{"opaque.io"},
		},
	}
	mcc := MakeMultiClusterControllerWithFakeCluster(t, &v1.Namespace{}, &v1.NamespaceList{}, vc)
	ct := conversion.Convertor(syncerConfig, mcc)
	time := metav1.Now()

	clusterKey := conversion.ToClusterKey(vc)

	attachVCMeta := func(obj client.Object) error {
		var tenantScopeMetaInAnnotation = map[string]string{
			constants.LabelCluster:     clusterKey,
			constants.LabelNamespace:   "n1",
			constants.LabelVCName:      "v1",
			constants.LabelVCNamespace: "t1",
			constants.LabelVCUID:       "d64ea0c0-91f8-46f5-8643-c0cab32ab0cd",
		}
		anno := obj.GetAnnotations()
		if anno == nil {
			anno = make(map[string]string)
		}
		for k, v := range tenantScopeMetaInAnnotation {
			anno[k] = v
		}
		obj.SetAnnotations(anno)
		return nil
	}

	tests := []struct {
		name        string
		cluster     string
		obj         client.Object
		expectedObj client.Object
		errMsg      string
	}{
		{
			name:    "cluster not found",
			cluster: "not_found",
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "n1",
				},
			},
			errMsg: "get virtualcluster",
		},
		{
			name:    "normal case",
			cluster: clusterKey,
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "n1",
					Labels: map[string]string{
						"k": "v",
					},
					Annotations: map[string]string{
						"k": "v",
					},
				},
			},
			expectedObj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: conversion.ToSuperClusterNamespace(clusterKey, "n1"),
					Labels: map[string]string{
						"k": "v",
					},
					Annotations: map[string]string{
						"k":                "v",
						constants.LabelUID: "",
					},
				},
			},
		},
		{
			name:    "non empty meta",
			cluster: clusterKey,
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "n1",
					UID:  "d64ea111-91f8-46f5-8643-c0cab32ab0cd",
					Labels: map[string]string{
						constants.LabelVCName:      "i want to overwrite",
						constants.LabelVCNamespace: "i want to overwrite",
					},
					Annotations: map[string]string{
						constants.LabelUID:             "i want to overwrite",
						constants.LabelOwnerReferences: "i want to overwrite",
						constants.LabelCluster:         "i want to overwrite",
					},
					ResourceVersion:            "1",
					Generation:                 10001,
					DeletionTimestamp:          &time,
					DeletionGracePeriodSeconds: pointer.Int64(1),
					OwnerReferences: []metav1.OwnerReference{
						{Name: "1"},
					},
					Finalizers: []string{"f"},
				},
			},
			expectedObj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: conversion.ToSuperClusterNamespace(clusterKey, "n1"),
					Annotations: map[string]string{
						constants.LabelUID: "d64ea111-91f8-46f5-8643-c0cab32ab0cd",
					},
				},
			},
		},
		{
			name:    "have opaqued keys",
			cluster: clusterKey,
			obj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "n1",
					Labels: map[string]string{
						"k":                 "v",
						"m.kubernetes.io":   "v",
						"m.kubernetes.io/a": "v",
						"kubernetes.io/b":   "v",
						"kubernetes.io":     "v",
						"opaque.io/kk":      "v",
						"m.opaque.io":       "v",
					},
					Annotations: map[string]string{
						"k":               "v",
						"m.kubernetes.io": "v",
						"opaque.io/kk":    "v",
						"m.opaque.io":     "v",
					},
				},
			},
			expectedObj: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: conversion.ToSuperClusterNamespace(clusterKey, "n1"),
					Labels: map[string]string{
						"k":           "v",
						"m.opaque.io": "v",
					},
					Annotations: map[string]string{
						"k":                "v",
						"m.opaque.io":      "v",
						constants.LabelUID: "",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ct.BuildSuperClusterNamespace(tt.cluster, tt.obj)
			if tt.errMsg != "" {
				if err == nil {
					t.Errorf("expected error %v, got empty", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error %v, got %v", tt.errMsg, err.Error())
					return
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error %v", err)
				return
			}

			err = attachVCMeta(tt.expectedObj)
			if err != nil {
				t.Errorf("unexpected error when attach vc meta: %v", err)
				return
			}
			if !equality.Semantic.DeepEqual(got, tt.expectedObj) {
				g, _ := json.MarshalIndent(got, "", "\t")
				e, _ := json.MarshalIndent(tt.expectedObj, "", "\t")
				t.Errorf("expected %v, got %v", string(e), string(g))
			}
		})
	}
}
