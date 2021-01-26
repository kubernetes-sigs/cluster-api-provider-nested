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

package controlplane

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controlplanev1alpha4 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha4"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"
)

const (
	failed  = "\u2717"
	succeed = "\u2713"
)

func TestSubstituteTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		context  interface{}
		expect   string
	}{
		{
			"empty template",
			"",
			map[string]interface{}{},
			"",
		},
		{
			"normal template",
			"name: {{.name}},\nage: {{.age}}",
			map[string]interface{}{
				"name": "test",
				"age":  "1",
			},
			"name: test,\nage: 1",
		},
	}
	for _, tt := range tests {
		st := tt
		tf := func(t *testing.T) {
			t.Parallel()
			t.Logf("\tTestCase: %s", st.name)
			{
				get, err := substituteTemplate(st.context, st.template)
				if err != nil {
					t.Fatalf("case %s failed: %v", st.name, err)
				}
				if get != st.expect {
					t.Fatalf("\t%s\texpect %v, but get %v", failed, st.expect, get)
				}
				t.Logf("\t%s\texpect %v, get %v", succeed, st.expect, get)

			}
		}
		t.Run(st.name, tf)
	}
}

func TestGetOwner(t *testing.T) {
	tests := []struct {
		name   string
		netcd  controlplanev1alpha4.NestedEtcd
		expect metav1.OwnerReference
	}{
		{
			"no owner",
			controlplanev1alpha4.NestedEtcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-netcd",
					OwnerReferences: []metav1.OwnerReference{},
				},
			},
			metav1.OwnerReference{},
		},
		{
			"owner APIVersion not matched",
			controlplanev1alpha4.NestedEtcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-netcd",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "controlplane.cluster.x-k8s.io/v1",
							Kind:       "test-kind",
							Name:       "test-name",
							UID:        "xxxxx-xxxxx-xxxxx-xxxxx",
						},
					},
				},
			},
			metav1.OwnerReference{},
		},
		{
			"owner kind not matched",
			controlplanev1alpha4.NestedEtcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-netcd",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1alpha4.GroupVersion.String(),
							Kind:       "test-kind",
							Name:       "test-name",
							UID:        "xxxxx-xxxxx-xxxxx-xxxxx",
						},
					},
				},
			},
			metav1.OwnerReference{},
		},
		{
			"owner found",
			controlplanev1alpha4.NestedEtcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-netcd",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1alpha4.GroupVersion.String(),
							Kind:       "NestedControlPlane",
							Name:       "test-name",
							UID:        "xxxxx-xxxxx-xxxxx-xxxxx",
						},
					},
				},
			},
			metav1.OwnerReference{
				APIVersion: controlplanev1alpha4.GroupVersion.String(),
				Kind:       "NestedControlPlane",
				Name:       "test-name",
				UID:        "xxxxx-xxxxx-xxxxx-xxxxx",
			},
		},
	}

	for _, tt := range tests {
		st := tt
		tf := func(t *testing.T) {
			t.Parallel()
			t.Logf("\tTestCase: %s", st.name)
			{
				get := getOwner(st.netcd)
				if !reflect.DeepEqual(get, st.expect) {
					t.Fatalf("\t%s\texpect %v, but get %v", failed, st.expect, get)
				}
				t.Logf("\t%s\texpect %v, get %v", succeed, st.expect, get)

			}
		}
		t.Run(st.name, tf)
	}
}

func TestGenInitialClusterArgs(t *testing.T) {
	tests := []struct {
		name     string
		replicas int32
		stsName  string
		svcName  string
		expect   string
	}{
		{
			"1 replicas",
			1,
			"netcdSts",
			"netcdSvc",
			"netcdSts-0=https://netcdSts-0.netcdSvc:2380",
		},
		{
			"2 replicas",
			2,
			"netcdSts",
			"netcdSvc",
			"netcdSts-0=https://netcdSts-0.netcdSvc:2380,netcdSts-1=https://netcdSts-1.netcdSvc:2380",
		},
	}
	for _, tt := range tests {
		st := tt
		tf := func(t *testing.T) {
			t.Parallel()
			t.Logf("\tTestCase: %s", st.name)
			{
				get := genInitialClusterArgs(st.replicas, st.stsName, st.svcName)
				if !reflect.DeepEqual(get, st.expect) {
					t.Fatalf("\t%s\texpect %v, but get %v", failed, st.expect, get)
				}
				t.Logf("\t%s\texpect %v, get %v", succeed, st.expect, get)

			}
		}
		t.Run(st.name, tf)
	}
}

func TestIsNetcdReady(t *testing.T) {
	tests := []struct {
		name   string
		status controlplanev1alpha4.NestedEtcdStatus
		expect bool
	}{
		{
			"ready",
			controlplanev1alpha4.NestedEtcdStatus{
				CommonStatus: addonv1alpha1.CommonStatus{
					Phase: string(controlplanev1alpha4.NestedEtcdReady),
				},
			},
			true,
		},
		{
			"unready",
			controlplanev1alpha4.NestedEtcdStatus{
				CommonStatus: addonv1alpha1.CommonStatus{
					Phase: string(controlplanev1alpha4.NestedEtcdUnready),
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		st := tt
		tf := func(t *testing.T) {
			t.Parallel()
			t.Logf("\tTestCase: %s", st.name)
			{
				get := IsNetcdReady(st.status)
				if get != st.expect {
					t.Fatalf("\t%s\texpect %v, but get %v", failed, st.expect, get)
				}
				t.Logf("\t%s\texpect %v, get %v", succeed, st.expect, get)

			}
		}
		t.Run(st.name, tf)
	}
}

func TestYamlToObject(t *testing.T) {
	tests := []struct {
		name   string
		yaml   string
		expect runtime.Object
	}{
		{
			"pod",
			`
apiVersion: v1
kind: Pod
metadata:
   name: busybox
spec:
   containers:
   - name: busybox
     image: busybox
     command: 
     - top`,
			&corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "busybox",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "busybox",
							Image:   "busybox",
							Command: []string{"top"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		st := tt
		tf := func(t *testing.T) {
			t.Parallel()
			t.Logf("\tTestCase: %s", st.name)
			{
				get, err := yamlToObject([]byte(st.yaml))
				if err != nil {
					t.Fatalf("case %s failed: %v", st.name, err)
				}
				if !reflect.DeepEqual(get, st.expect) {
					t.Fatalf("\t%s\texpect %v, but get %v\n", failed, get, st.expect)
				}
				t.Logf("\tTestcase: %s %s\t", st.name, succeed)

			}
		}
		t.Run(st.name, tf)
	}
}
