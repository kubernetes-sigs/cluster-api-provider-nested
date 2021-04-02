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
	statefulsetOwnerKey       = ".metadata.controller"
	defaultEtcdStatefulSetURL = "https://raw.githubusercontent.com/kubernetes-sigs/" +
		"cluster-api-provider-nested/master/config/component-templates/" +
		"nestedetcd/nested-etcd-statefulset-template.yaml"
	defaultEtcdServiceURL = "https://raw.githubusercontent.com/kubernetes-sigs/" +
		"cluster-api-provider-nested/master/config/component-templates/" +
		"nestedetcd/nested-etcd-service-template.yaml"
	defaultKASStatefulSetURL = "https://raw.githubusercontent.com/kubernetes-sigs/" +
		"cluster-api-provider-nested/master/config/component-templates/" +
		"nestedetcd/nested-apiserver-statefulset-template.yaml"
	defaultKASServiceURL = "https://raw.githubusercontent.com/kubernetes-sigs/" +
		"cluster-api-provider-nested/master/config/component-templates/" +
		"nestedetcd/nested-apiserver-service-template.yaml"
)
