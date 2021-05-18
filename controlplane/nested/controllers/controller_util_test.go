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

package controllers

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
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
		netcd  controlplanev1.NestedEtcd
		expect metav1.OwnerReference
	}{
		{
			"no owner",
			controlplanev1.NestedEtcd{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-netcd",
					OwnerReferences: []metav1.OwnerReference{},
				},
			},
			metav1.OwnerReference{},
		},
		{
			"owner APIVersion not matched",
			controlplanev1.NestedEtcd{
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
			controlplanev1.NestedEtcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-netcd",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
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
			controlplanev1.NestedEtcd{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-netcd",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: controlplanev1.GroupVersion.String(),
							Kind:       "NestedControlPlane",
							Name:       "test-name",
							UID:        "xxxxx-xxxxx-xxxxx-xxxxx",
						},
					},
				},
			},
			metav1.OwnerReference{
				APIVersion: controlplanev1.GroupVersion.String(),
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
				get := getOwner(st.netcd.ObjectMeta)
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
		name         string
		replicas     int32
		stsName      string
		svcName      string
		svcNamespace string
		expect       string
	}{
		{
			"1 replicas",
			1,
			"netcdSts",
			"netcdSvc",
			"default",
			"netcdSts-etcd-0=https://netcdSts-etcd-0.netcdSvc-etcd.default.svc:2380",
		},
		{
			"2 replicas",
			2,
			"netcdSts",
			"netcdSvc",
			"default",
			"netcdSts-etcd-0=https://netcdSts-etcd-0.netcdSvc-etcd.default.svc:2380,netcdSts-etcd-1=https://netcdSts-etcd-1.netcdSvc-etcd.default.svc:2380",
		},
	}
	for _, tt := range tests {
		st := tt
		tf := func(t *testing.T) {
			t.Parallel()
			t.Logf("\tTestCase: %s", st.name)
			{
				get := genInitialClusterArgs(st.replicas, st.stsName, st.svcName, st.svcNamespace)
				if !reflect.DeepEqual(get, st.expect) {
					t.Fatalf("\t%s\texpect %v, but get %v", failed, st.expect, get)
				}
				t.Logf("\t%s\texpect %v, get %v", succeed, st.expect, get)

			}
		}
		t.Run(st.name, tf)
	}
}
