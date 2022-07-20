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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	tenancyv1alpha1 "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/controllers/provisioner"
	kubeutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/kube"
	strutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/strings"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
)

// GetProvisioner returns a new provisioner.Provisioner by ProvisionerName
func (r *ReconcileVirtualCluster) GetProvisioner(mgr ctrl.Manager, log logr.Logger, provisionerTimeout time.Duration) (provisioner.Provisioner, error) {
	switch r.ProvisionerName {
	case "aliyun":
		return provisioner.NewProvisionerAliyun(mgr, log, provisionerTimeout)
	case "native":
		return provisioner.NewProvisionerNative(mgr, log, provisionerTimeout)
	}
	return nil, fmt.Errorf("virtualcluster provisioner missing")
}

var _ reconcile.Reconciler = &ReconcileVirtualCluster{}

// ReconcileVirtualCluster reconciles a VirtualCluster object
type ReconcileVirtualCluster struct {
	client.Client
	Log                logr.Logger
	ProvisionerName    string
	ProvisionerTimeout time.Duration
	Provisioner        provisioner.Provisioner
}

// SetupWithManager will configure the VirtualCluster reconciler
func (r *ReconcileVirtualCluster) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	provisioner, err := r.GetProvisioner(mgr, r.Log, r.ProvisionerTimeout)
	if err != nil {
		return err
	}
	r.Provisioner = provisioner

	// Expose featuregate.ClusterVersionPartialUpgrade metrics only if it enabled
	if featuregate.DefaultFeatureGate.Enabled(featuregate.ClusterVersionPartialUpgrade) {
		metrics.Registry.MustRegister(
			clustersUpdatedCounter,
			clustersUpdateSeconds,
		)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(opts).
		For(&tenancyv1alpha1.VirtualCluster{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=virtualclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=virtualclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=clusterversions,verbs=get;list;watch
// +kubebuilder:rbac:groups=tenancy.x-k8s.io,resources=clusterversions/status,verbs=get

// Reconcile reads that state of the cluster for a VirtualCluster object and makes changes based on the state read
// and what is in the VirtualCluster.Spec
func (r *ReconcileVirtualCluster) Reconcile(ctx context.Context, request reconcile.Request) (rncilRslt reconcile.Result, err error) {
	r.Log.Info("reconciling VirtualCluster...")
	vc := &tenancyv1alpha1.VirtualCluster{}
	err = r.Get(context.TODO(), request.NamespacedName, vc)
	if err != nil {
		// set NotFound error as nil
		if apierrors.IsNotFound(err) {
			err = nil
		}
		return
	}

	vcFinalizerName := fmt.Sprintf("virtualcluster.finalizer.%s", r.Provisioner.GetProvisioner())

	if vc.ObjectMeta.DeletionTimestamp.IsZero() {
		if !strutil.ContainString(vc.ObjectMeta.Finalizers, vcFinalizerName) {
			vc.ObjectMeta.Finalizers = append(vc.ObjectMeta.Finalizers, vcFinalizerName)
			if err = kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log); err != nil {
				return
			}
			r.Log.Info("a finalizer has been registered for the VirtualCluster CRD", "finalizer", vcFinalizerName)
		}
	} else {
		// The VirtualCluster is being deleted
		if strutil.ContainString(vc.ObjectMeta.Finalizers, vcFinalizerName) {
			// delete the control plane
			r.Log.Info("VirtualCluster is being deleted, finalizer will be activated", "vc-name", vc.Name, "finalizer", vcFinalizerName)
			// block if fail to delete VC
			if err = r.Provisioner.DeleteVirtualCluster(ctx, vc); err != nil {
				r.Log.Error(err, "fail to delete virtualcluster", "vc-name", vc.Name)
				return
			}
			// remove finalizer from the list and update it.
			vc.ObjectMeta.Finalizers = strutil.RemoveString(vc.ObjectMeta.Finalizers, vcFinalizerName)
			err = kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log)
		}
		return
	}

	// reconcile VirtualCluster (vc) based on vc status
	// NOTE: vc status is required by other components (e.g. syncer need to
	// know the vc status in order to setup connection to the tenant control plane)
	switch vc.Status.Phase {
	case "":
		// set vc status as ClusterPending if no status is set
		r.Log.Info("will create a VirtualCluster", "vc", vc.Name)
		// will retry three times
		kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterPending,
			"retry: 3", "ClusterCreating")
		err = kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log)
		return
	case tenancyv1alpha1.ClusterPending:
		// create new virtualcluster when vc is pending
		r.Log.Info("VirtualCluster is pending", "vc", vc.Name)
		retryTimes, _ := strconv.Atoi(strings.TrimSpace(strings.Split(vc.Status.Message, ":")[1]))
		if retryTimes > 0 {
			err = r.Provisioner.CreateVirtualCluster(ctx, vc)
			if err != nil {
				r.Log.Error(err, "fail to create virtualcluster", "vc", vc.GetName(), "retrytimes", retryTimes)
				errReason := fmt.Sprintf("fail to create virtualcluster(%s): %s", vc.GetName(), err)
				errMsg := fmt.Sprintf("retry: %d", retryTimes-1)
				kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterPending, errMsg, errReason)
			} else {
				kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterRunning,
					"tenant control plane is running", "TenantControlPlaneRunning")
			}
		} else {
			kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterError,
				"fail to create virtualcluster", "TenantControlPlaneError")
		}

		err = kubeutil.RetryUpdateVCStatusOnConflict(ctx, r, vc, r.Log)
		return
	case tenancyv1alpha1.ClusterRunning:
		r.Log.Info("VirtualCluster is running", "vc", vc.GetName())
		if !featuregate.DefaultFeatureGate.Enabled(featuregate.ClusterVersionPartialUpgrade) {
			return
		}
		if isReady, ok := vc.Labels[constants.LabelVCReadyForUpgrade]; !ok || isReady != "true" {
			return
		}
		r.Log.Info("VirtualCluster is ready for upgrade", "vc", vc.GetName())
		upgradeStartTimestamp := time.Now()
		err = r.Provisioner.UpgradeVirtualCluster(ctx, vc)
		clustersUpdateSeconds.WithLabelValues(vc.Spec.ClusterVersionName, vc.Labels[constants.LabelClusterVersionApplied]).Observe(time.Since(upgradeStartTimestamp).Seconds())
		if err != nil {
			r.Log.Error(err, "fail to upgrade virtualcluster", "vc", vc.GetName())
			kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterRunning, fmt.Sprintf("fail to upgrade: %s", err), "TenantControlPlaneUpgradeFailed")
		} else {
			r.Log.Info("upgrade finished", "vc", vc.GetName())
			kubeutil.SetVCStatus(vc, tenancyv1alpha1.ClusterRunning, "tenant control plane is upgraded", "TenantControlPlaneUpgradeCompleted")
			clustersUpdatedCounter.WithLabelValues(vc.Spec.ClusterVersionName, vc.Labels[constants.LabelClusterVersionApplied]).Inc()
		}

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			vcStatus := vc.Status
			delete(vc.Labels, constants.LabelVCReadyForUpgrade)
			vcLabels := vc.Labels
			updateErr := r.Update(ctx, vc)
			if updateErr != nil {
				if err := r.Get(ctx, types.NamespacedName{
					Namespace: vc.GetNamespace(),
					Name:      vc.GetName(),
				}, vc); err != nil {
					r.Log.Info("fail to get obj on update failure", "object", vc.GetName(), "error", err.Error())
				}
				vc.Status = vcStatus
				vc.Labels = vcLabels
			}
			return updateErr
		})
		return
	case tenancyv1alpha1.ClusterError:
		r.Log.Info("fail to create virtualcluster", "vc", vc.GetName())
		return
	default:
		err = fmt.Errorf("unknown vc phase: %s", vc.Status.Phase)
		return
	}
}
