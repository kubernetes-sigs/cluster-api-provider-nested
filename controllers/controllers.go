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

package controllers

import (
	"fmt"

	controlplanecontroller "sigs.k8s.io/cluster-api-provider-nested/controllers/controlplane"
	ctrl "sigs.k8s.io/controller-runtime"
)

// TODO(christopherhein) setup each of these reconcilers with `injection` so
// that we can loop over types and configure them with the proper fields
// Ref: https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/runtime/inject/inject.go#L90-L92

// SetupWithManager will configure the controllers for managing the CAPI
// implementation.
func SetupWithManager(mgr ctrl.Manager) (err error) {
	if err = (&controlplanecontroller.NestedControlPlaneTemplateReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NestedControlPlaneTemplate"),
		Scheme: mgr.GetScheme(),
		Event:  mgr.GetEventRecorderFor("NestedControlPlaneTemplate"),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller controller NestedControlPlaneTemplate err=%s", err)
	}

	if err = (&controlplanecontroller.NestedControlPlaneReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NestedControlPlane"),
		Scheme: mgr.GetScheme(),
		Event:  mgr.GetEventRecorderFor("NestedControlPlane"),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller controller NestedControlPlane err=%s", err)
	}

	if err = (&controlplanecontroller.NestedPKIReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NestedPKI"),
		Scheme: mgr.GetScheme(),
		Event:  mgr.GetEventRecorderFor("NestedPKI"),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller controller NestedPKI err=%s", err)
	}

	if err = (&controlplanecontroller.NestedEtcdReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NestedEtcd"),
		Scheme: mgr.GetScheme(),
		Event:  mgr.GetEventRecorderFor("NestedEtcd"),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller controller NestedEtcd err=%s", err)
	}

	return
}
