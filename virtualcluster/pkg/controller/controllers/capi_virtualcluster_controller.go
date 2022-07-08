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
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"

	tenancyv1alpha1 "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/controllers/provisioner"
	kubeutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/kube"
	strutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/strings"
)

var (
	_ reconcile.Reconciler = &ReconcileCAPIVirtualCluster{}
)

// ReconcileCAPIVirtualCluster reconciles a VirtualCluster object
type ReconcileCAPIVirtualCluster struct {
	client.Client
	Log             logr.Logger
	ProvisionerName string
	Provisioner     provisioner.Provisioner
}

// SetupWithManager will configure the VirtualCluster reconciler
func (r *ReconcileCAPIVirtualCluster) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&tenancyv1alpha1.VirtualCluster{}).
		Owns(&clusterv1.Cluster{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=virtualclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=virtualclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a VirtualCluster object and makes changes based on the state read
// and what is in the VirtualCluster.Spec
func (r *ReconcileCAPIVirtualCluster) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("reconciling VirtualCluster...")
	vc := &tenancyv1alpha1.VirtualCluster{}
	if err := r.Get(ctx, request.NamespacedName, vc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var clusterName string
	var ok bool
	annotations := vc.GetAnnotations()
	// Fast fail is Cluster label is missing
	if clusterName, ok = annotations[constants.VirtualClusterCAPIName]; !ok {
		r.Log.Info("VirtualCluster is missing Cluster Name", "vc-name", vc.Name, constants.VirtualClusterCAPIName, "")
		return ctrl.Result{}, nil
	}

	clusterObjectKey := client.ObjectKey{Namespace: vc.GetNamespace(), Name: clusterName}

	cluster := &clusterv1.Cluster{}
	if err := r.Get(ctx, clusterObjectKey, cluster); err != nil {
		errs := client.IgnoreNotFound(err)
		// Requeue the processing if the Cluster object wasn't found otherwise handle through normal means
		return ctrl.Result{Requeue: (errs == nil)}, errs
	}

	// Manually set ClusterNamespace to VC namespace to override clusterKey.
	vc.Status.ClusterNamespace = vc.GetNamespace()

	vcFinalizerName := "virtualcluster.finalizer.capi"

	if vc.ObjectMeta.DeletionTimestamp.IsZero() {
		if !strutil.ContainString(vc.ObjectMeta.Finalizers, vcFinalizerName) {
			vc.ObjectMeta.Finalizers = append(vc.ObjectMeta.Finalizers, vcFinalizerName)
			if err := kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log); err != nil {
				return ctrl.Result{}, err
			}
			r.Log.Info("a finalizer has been registered for the VirtualCluster CRD", "finalizer", vcFinalizerName)
		}
	} else {
		// The VirtualCluster is being deleted
		if strutil.ContainString(vc.ObjectMeta.Finalizers, vcFinalizerName) {
			// delete the control plane
			r.Log.Info("VirtualCluster is being deleted, finalizer will be activated", "vc-name", vc.Name, "finalizer", vcFinalizerName)
			// block if fail to delete VC

			// Delete CAPI
			// This is the current behavior of VC, we should decide if this controller should have
			// capabilities to delete or if it should purely be a middleware for managing VC status
			deletionPolicy := metav1.DeletePropagationForeground
			if err := r.Delete(ctx, cluster, &client.DeleteOptions{PropagationPolicy: &deletionPolicy}); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			// remove finalizer from the list and update it.
			vc.ObjectMeta.Finalizers = strutil.RemoveString(vc.ObjectMeta.Finalizers, vcFinalizerName)
			if err := kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log); err != nil {
				return ctrl.Result{}, err
			}

		}
		return ctrl.Result{}, nil
	}

	// reconcile VirtualCluster (vc) based on vc status
	// NOTE: vc status is required by other components (e.g. syncer need to
	// know the vc status in order to setup connection to the tenant )
	switch vc.Status.Phase {
	case "":
		// set vc status as ClusterPending if no status is set
		r.Log.Info("will create a VirtualCluster", "vc", vc.Name)
		// will retry three times
		kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterPending,
			"retry: 3", "ClusterCreating")
		if err := kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	case tenancyv1alpha1.ClusterPending:
		r.Log.Info("VirtualCluster is pending", "vc", vc.Name)
		// If VC isn't running and cluster.Status is running
		if clusterv1.ClusterPhase(cluster.Status.Phase) == clusterv1.ClusterPhaseProvisioned {
			kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterRunning,
				"tenant cluster provisioned", "ClusterRunning")
			if err := kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: (1 * time.Second)}, nil
	case tenancyv1alpha1.ClusterRunning:
		r.Log.Info("VirtualCluster is running", "vc", vc.GetName())
		return ctrl.Result{}, nil
	case tenancyv1alpha1.ClusterError:
		r.Log.Info("fail to create virtualcluster", "vc", vc.GetName())
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, fmt.Errorf("unknown vc phase: %s", vc.Status.Phase)
	}
}
