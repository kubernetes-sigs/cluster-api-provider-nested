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

package persistentvolumeclaim

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	core "k8s.io/client-go/testing"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	util "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/test"
)

func updatePVCStatus(pvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	if pvc.Status.Capacity == nil {
		pvc.Status.Capacity = make(map[corev1.ResourceName]resource.Quantity)
	}
	pvc.Status.Capacity["storage"] = *resource.NewQuantity(10*1024*1024*1024, resource.BinarySI)
	return pvc
}

func TestUWPVCUpdate(t *testing.T) {
	testTenant := &v1alpha1.VirtualCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "tenant-1",
			UID:       "7374a172-c35d-45b1-9c8e-bf5c5b614937",
		},
		Status: v1alpha1.VirtualClusterStatus{
			Phase: v1alpha1.ClusterRunning,
		},
	}

	defer util.SetFeatureGateDuringTest(t, featuregate.DefaultFeatureGate, featuregate.SyncTenantPVCStatusPhase, true)()
	defaultClusterKey := conversion.ToClusterKey(testTenant)
	superDefaultNSName := conversion.ToSuperClusterNamespace(defaultClusterKey, "default")

	testcases := map[string]struct {
		ExistingObjectInSuper  []runtime.Object
		ExistingObjectInTenant []runtime.Object
		EnqueuedKey            string
		ExpectedUpdatedObject  []string
		ExpectedError          string
	}{
		"pPVC exists, vPVC exists with no diff": {
			ExistingObjectInSuper: []runtime.Object{
				tenantPVC("pvc-1", "default", "12345"),
			},
			ExistingObjectInTenant: []runtime.Object{
				tenantPVC("pvc-1", "default", "12345"),
			},
			EnqueuedKey:   superDefaultNSName + "/pvc-1",
			ExpectedError: "",
		},
		"pPVC exists, but vPVC does not exist": {
			ExistingObjectInSuper: []runtime.Object{
				tenantPVC("pvc-1", "default", "12345"),
			},
			ExistingObjectInTenant: []runtime.Object{},
			EnqueuedKey:            superDefaultNSName + "/pvc-1",
			ExpectedError:          "",
		},
		"vPVC exists, but pPVC does not exist": {
			ExistingObjectInSuper: []runtime.Object{},
			ExistingObjectInTenant: []runtime.Object{
				tenantPVC("pvc-1", "default", "12345"),
			},
			EnqueuedKey:   superDefaultNSName + "/pvc-1",
			ExpectedError: "",
		},
		"pPVC exists, vPVC exists with different status": {
			ExistingObjectInSuper: []runtime.Object{
				updatePVCStatus(superPVC("pvc-1", superDefaultNSName, "12345", defaultClusterKey)),
			},
			ExistingObjectInTenant: []runtime.Object{
				tenantPVC("pvc-1", "default", "12345"),
			},
			EnqueuedKey: superDefaultNSName + "/pvc-1",
			ExpectedUpdatedObject: []string{
				"default/pvc-1",
			},
		},
		"pPVC is lost, vPVC is bound": {
			ExistingObjectInSuper: []runtime.Object{
				applyStatusToPVC(superPVC("pvc-1", superDefaultNSName, "12345", defaultClusterKey), statusLost),
			},
			ExistingObjectInTenant: []runtime.Object{
				applyStatusToPVC(tenantPVC("pvc-1", "default", "12345"), statusBound),
			},
			EnqueuedKey:   superDefaultNSName + "/pvc-1",
			ExpectedError: "",
			ExpectedUpdatedObject: []string{
				"default/pvc-1",
			},
		},
		"pPVC is bound, vPVC is pending": {
			ExistingObjectInSuper: []runtime.Object{
				applyStatusToPVC(superPVC("pvc-1", superDefaultNSName, "12345", defaultClusterKey), statusBound),
			},
			ExistingObjectInTenant: []runtime.Object{
				applyStatusToPVC(tenantPVC("pvc-1", "default", "12345"), statusPending),
			},
			EnqueuedKey:           superDefaultNSName + "/pvc-1",
			ExpectedError:         "",
			ExpectedUpdatedObject: []string{},
		},
		"pPVC is pending, vPVC is pending": {
			ExistingObjectInSuper: []runtime.Object{
				applyStatusToPVC(superPVC("pvc-1", superDefaultNSName, "12345", defaultClusterKey), statusPending),
			},
			ExistingObjectInTenant: []runtime.Object{
				applyStatusToPVC(tenantPVC("pvc-1", "default", "12345"), statusPending),
			},
			EnqueuedKey:           superDefaultNSName + "/pvc-1",
			ExpectedError:         "",
			ExpectedUpdatedObject: []string{},
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			actions, reconcileErr, err := util.RunUpwardSync(NewPVCController, testTenant, tc.ExistingObjectInSuper, tc.ExistingObjectInTenant, tc.EnqueuedKey, nil)
			if err != nil {
				t.Errorf("%s: error running upward sync: %v", k, err)
				return
			}

			if reconcileErr != nil {
				if tc.ExpectedError == "" {
					t.Errorf("expected no error, but got \"%v\"", reconcileErr)
				} else if !strings.Contains(reconcileErr.Error(), tc.ExpectedError) {
					t.Errorf("expected error msg \"%s\", but got \"%v\"", tc.ExpectedError, reconcileErr)
				}
			} else {
				if tc.ExpectedError != "" {
					t.Errorf("expected error msg \"%s\", but got empty", tc.ExpectedError)
				}
			}

			for _, expectedName := range tc.ExpectedUpdatedObject {
				matched := false
				for _, action := range actions {
					if !action.Matches("update", "persistentvolumeclaims") {
						continue
					}
					actionObj := action.(core.UpdateAction).GetObject()
					accessor, _ := meta.Accessor(actionObj)
					fullName := accessor.GetNamespace() + "/" + accessor.GetName()
					if fullName != expectedName {
						t.Errorf("%s: Expected pvc %s to be updated, got %s", k, expectedName, fullName)
					}
					matched = true
					break
				}
				if !matched {
					t.Errorf("%s: Expect updated pvc %s but not found", k, expectedName)
				}
			}
		})
	}
}
