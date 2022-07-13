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

package controller

import (
	"time"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/controllers"
)

// Controllers defines all the shared information between all
type Controllers struct {
	client.Client
	Log                     logr.Logger
	MaxConcurrentReconciles int
	ProvisionerName         string
	ProvisionerTimeout      time.Duration
}

// SetupWithManager adds all Controllers to the Manager
func (c *Controllers) SetupWithManager(mgr ctrl.Manager) error {
	opts := controller.Options{
		MaxConcurrentReconciles: c.MaxConcurrentReconciles,
	}

	// If Provisioner is CAPI exit fast and only implement CAPI
	// VC reconciler.
	if c.ProvisionerName == "capi" {
		if err := (&controllers.ReconcileCAPIVirtualCluster{
			Client: mgr.GetClient(),
			Log:    c.Log.WithName("virtualcluster"),
		}).SetupWithManager(mgr, opts); err != nil {
			return err
		}
		return nil
	}

	// add controller based the type of the controlPlaneProvisioner
	if c.ProvisionerName == "native" {
		if err := (&controllers.ReconcileClusterVersion{
			Client: mgr.GetClient(),
			Log:    c.Log.WithName("virtualcluster"),
		}).SetupWithManager(mgr, opts); err != nil {
			return err
		}
	}

	if err := (&controllers.ReconcileVirtualCluster{
		Client:             mgr.GetClient(),
		Log:                c.Log.WithName("virtualcluster"),
		ProvisionerName:    c.ProvisionerName,
		ProvisionerTimeout: c.ProvisionerTimeout,
	}).SetupWithManager(mgr, opts); err != nil {
		return err
	}
	return nil
}
