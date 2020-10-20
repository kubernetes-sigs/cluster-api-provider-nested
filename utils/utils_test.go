/*
Copyright 2020 The Kubernetes Authors.

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

package utils

import (
	"reflect"
	"testing"

	"sigs.k8s.io/cluster-api-provider-nested/constants"
	"sigs.k8s.io/cluster-api-provider-nested/testutils"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetClusterLabels(t *testing.T) {
	controlplane := testutils.MakeNestedControlPlane()
	namespace := &corev1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			GenerateName: "namespace",
		},
	}

	type args struct {
		controlplane metav1.Object
		object       metav1.Object
		root         bool
	}
	tests := []struct {
		name           string
		args           args
		expectedLabels map[string]string
	}{
		{
			"TestAddingLabelsWithRoot",
			args{controlplane, namespace, true},
			map[string]string{
				constants.LabelNestedControlPlaneName:          controlplane.GetName(),
				constants.LabelNestedControlPlaneNamespace:     controlplane.GetNamespace(),
				constants.LabelNestedControlPlaneUID:           string(controlplane.GetUID()),
				constants.LabelNestedControlPlaneRootNamespace: "true",
			},
		},
		{
			"TestAddingLabelsWithoutRoot",
			args{controlplane, namespace, false},
			map[string]string{
				constants.LabelNestedControlPlaneName:      controlplane.GetName(),
				constants.LabelNestedControlPlaneNamespace: controlplane.GetNamespace(),
				constants.LabelNestedControlPlaneUID:       string(controlplane.GetUID()),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetClusterLabels(tt.args.controlplane, tt.args.object, tt.args.root)
			labels := tt.args.object.GetLabels()
			if !reflect.DeepEqual(labels, tt.expectedLabels) {
				t.Errorf("SetClusterLabels labels %+v did not match %+v", labels, tt.expectedLabels)
			}
		})
	}
}
