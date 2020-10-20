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
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	controlplanev1alpha3 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-nested/constants"
)

// NestedControlPlaneTemplateReconciler reconciles a NestedControlPlaneTemplate object
type NestedControlPlaneTemplateReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Event  record.EventRecorder
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanetemplates/status,verbs=get;update;patch

func (r *NestedControlPlaneTemplateReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("nestedcontrolplanetemplate", req.NamespacedName)

	instance := &controlplanev1alpha3.NestedControlPlaneTemplate{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch NestedControlPlaneTemplate")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(instance, constants.NestedControlPlaneTemplateFinalizer) {
			controllerutil.AddFinalizer(instance, constants.NestedControlPlaneTemplateFinalizer)
			if err := r.Update(context.Background(), instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(instance, constants.NestedControlPlaneTemplateFinalizer) {
			controllerutil.RemoveFinalizer(instance, constants.NestedControlPlaneTemplateFinalizer)
			if err := r.Update(context.Background(), instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// check for shadow etcd

	// check for shadow apiserver

	return ctrl.Result{}, nil
}

func (r *NestedControlPlaneTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1alpha3.NestedControlPlaneTemplate{}).
		Complete(r)
}
