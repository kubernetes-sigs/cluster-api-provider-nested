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

const (
	statefulsetOwnerKeyNEtcd  = ".metadata.netcd.controller"
	statefulsetOwnerKeyNKas   = ".metadata.nkas.controller"
	statefulsetOwnerKeyNKcm   = ".metadata.nkcm.controller"
	defaultEtcdStatefulSetURL = "./config/component-templates/" +
		"nested-etcd/nested-etcd-statefulset-template.yaml"
	defaultEtcdServiceURL = "./config/component-templates/" +
		"nested-etcd/nested-etcd-service-template.yaml"
	defaultKASStatefulSetURL = "./config/component-templates/" +
		"nested-apiserver/nested-apiserver-statefulset-template.yaml"
	defaultKASServiceURL = "./config/component-templates/" +
		"nested-apiserver/nested-apiserver-service-template.yaml"
	defaultKCMStatefulSetURL = "./config/component-templates/" +
		"nested-controllermanager/nested-controllermanager-statefulset-template.yaml"
)
