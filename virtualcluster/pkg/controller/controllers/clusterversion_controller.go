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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	tenancyv1alpha1 "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	strutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/strings"
)

var log = logf.Log.WithName("clusterversion-controller")

var _ reconcile.Reconciler = &ReconcileClusterVersion{}

// ReconcileClusterVersion reconciles a ClusterVersion object
type ReconcileClusterVersion struct {
	client.Client
	Log logr.Logger
}

// SetupWithManager will configure the VirtualCluster reconciler
func (r *ReconcileClusterVersion) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&tenancyv1alpha1.ClusterVersion{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=clusterversions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=clusterversions/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a ClusterVersion object and makes changes based on the state read
// and what is in the ClusterVersion.Spec
func (r *ReconcileClusterVersion) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ClusterVersion instance
	r.Log.Info("reconciling ClusterVersion...")
	cv := &tenancyv1alpha1.ClusterVersion{}
	err := r.Get(context.TODO(), request.NamespacedName, cv)
	if err != nil {
		// Error reading the object - requeue the request.
		if apierrors.IsNotFound(err) {
			err = nil
		}
		return reconcile.Result{}, err
	}
	r.Log.Info("new ClusterVersion event", "ClusterVersionName", cv.Name)

	// Register finalizers
	cvf := "clusterVersion.finalizers"

	if cv.ObjectMeta.DeletionTimestamp.IsZero() {
		// the object has not been deleted yet, registers the finalizers
		if strutil.ContainString(cv.ObjectMeta.Finalizers, cvf) == false {
			cv.ObjectMeta.Finalizers = append(cv.ObjectMeta.Finalizers, cvf)
			r.Log.Info("register finalizer for ClusterVersion", "finalizer", cvf)
			if err := r.Update(context.Background(), cv); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		// the object is being deleted, star the finalizer
		if strutil.ContainString(cv.ObjectMeta.Finalizers, cvf) == true {
			// the finalizer logic
			r.Log.Info("a ClusterVersion object is deleted", "ClusterVersion", cv.Name)

			// remove the finalizer after done
			cv.ObjectMeta.Finalizers = strutil.RemoveString(cv.ObjectMeta.Finalizers, cvf)
			if err := r.Update(context.Background(), cv); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}
