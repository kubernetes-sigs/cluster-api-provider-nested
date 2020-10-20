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

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	controlplanev1alpha3 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
	// +kubebuilder:scaffold:imports
)

// AddToScheme will register all schemes needed to run CAPN
func AddToScheme(scheme *runtime.Scheme) (err error) {
	if err = controlplanev1alpha3.AddToScheme(scheme); err != nil {
		return err
	}
	// +kubebuilder:scaffold:scheme
	return
}
