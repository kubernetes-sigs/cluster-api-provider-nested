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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	kcpv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	addonv1alpha1 "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/apis/v1alpha1"

	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
)

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrollermanagers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// NestedControlPlaneReconciler reconciles a NestedControlPlane object
type NestedControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// SetupWithManager will configure the controller with the manager
func (r *NestedControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1.NestedControlPlane{}).
		Owns(&controlplanev1.NestedEtcd{}).
		Owns(&controlplanev1.NestedAPIServer{}).
		Owns(&controlplanev1.NestedControllerManager{}).
		Complete(r)
}

// Reconcile is ths main process which will handle updating the NCP
func (r *NestedControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nestedcontrolplane", req.NamespacedName)
	log.Info("Reconciling NestedControlPlane...")
	// Fetch the NestedControlPlane
	ncp := &controlplanev1.NestedControlPlane{}
	if err := r.Get(ctx, req.NamespacedName, ncp); err != nil {
		// check for not found and don't requeue
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// If there are errors we should retry
		return ctrl.Result{Requeue: true}, nil
	}

	// Fetch the cluster object
	cluster, err := ncp.GetOwnerCluster(ctx, r.Client)
	if err != nil || cluster == nil {
		log.Error(err, "Failed to retrieve owner Cluster from the API Server")
		return ctrl.Result{Requeue: true}, err
	}
	log = log.WithValues("cluster", cluster.Name)

	if annotations.IsPaused(cluster, ncp) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(ncp, r.Client)
	if err != nil {
		log.Error(err, "Failed to configure the patch helper")
		return ctrl.Result{Requeue: true}, nil
	}

	if !controllerutil.ContainsFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer) {
		controllerutil.AddFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer)

		// patch and return right away instead of reusing the main defer,
		// because the main defer may take too much time to get cluster status
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{patch.WithStatusObservedGeneration{}}
		if err := patchHelper.Patch(ctx, ncp, patchOpts...); err != nil {
			log.Error(err, "Failed to patch NestedControlPlane to add finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	// TODO(christopherhein) handle deletion
	if !ncp.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion reconciliation loop.
		return r.reconcileDelete(ctx, log, ncp)
	}

	defer func() {
		if err := patchControlPlane(ctx, patchHelper, ncp); err != nil {
			log.Error(err, "Failed to patch KubeadmControlPlane")
		}
	}()

	// Handle normal reconciliation loop.
	return r.reconcile(ctx, log, cluster, ncp)
}

// reconcileDelete will delete the control plane and all it's nestedcomponents
func (r *NestedControlPlaneReconciler) reconcileDelete(ctx context.Context, log logr.Logger, ncp *controlplanev1.NestedControlPlane) (ctrl.Result, error) {
	patchHelper, err := patch.NewHelper(ncp, r.Client)
	if err != nil {
		log.Error(err, "Failed to configure the patch helper")
		return ctrl.Result{Requeue: true}, nil
	}

	if controllerutil.ContainsFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer) {
		controllerutil.RemoveFinalizer(ncp, controlplanev1.NestedControlPlaneFinalizer)

		// patch and return right away instead of reusing the main defer,
		// because the main defer may take too much time to get cluster status
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{patch.WithStatusObservedGeneration{}}
		if err := patchHelper.Patch(ctx, ncp, patchOpts...); err != nil {
			log.Error(err, "Failed to patch NestedControlPlane to remove finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func patchControlPlane(ctx context.Context, patchHelper *patch.Helper, ncp *controlplanev1.NestedControlPlane) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	conditions.SetSummary(ncp,
		conditions.WithConditions(
			kcpv1.AvailableCondition,
			kcpv1.CertificatesAvailableCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		ncp,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			kcpv1.AvailableCondition,
			kcpv1.CertificatesAvailableCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
}

// reconcile will handle all "normal" NCP reconciles this means create/update actions
func (r *NestedControlPlaneReconciler) reconcile(ctx context.Context, log logr.Logger, cluster *clusterv1.Cluster, ncp *controlplanev1.NestedControlPlane) (res ctrl.Result, reterr error) {
	log.Info("Reconcile NestedControlPlane")

	certificates := secret.NewCertificatesForInitialControlPlane(nil)
	controllerRef := metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	if err := certificates.LookupOrGenerate(ctx, r.Client, util.ObjectKey(cluster), *controllerRef); err != nil {
		log.Error(err, "unable to lookup or create cluster certificates")
		conditions.MarkFalse(ncp, kcpv1.CertificatesAvailableCondition, kcpv1.CertificatesGenerationFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
		return ctrl.Result{}, err
	}
	// TODO(christopherhein) use conditions to mark when ready
	conditions.MarkTrue(ncp, kcpv1.CertificatesAvailableCondition)

	// If ControlPlaneEndpoint is not set, return early
	if !cluster.Spec.ControlPlaneEndpoint.IsValid() {
		log.Info("Cluster does not yet have a ControlPlaneEndpoint defined")
		return ctrl.Result{}, nil
	}

	if result, err := r.reconcileKubeconfig(ctx, cluster, ncp); !result.IsZero() || err != nil {
		if err != nil {
			log.Error(err, "failed to reconcile Kubeconfig")
		}
		return result, err
	}

	addOwners := []client.Object{}
	isReady := []int{}
	nestedComponents := map[client.Object]*corev1.ObjectReference{
		&controlplanev1.NestedEtcd{}:              ncp.Spec.EtcdRef,
		&controlplanev1.NestedAPIServer{}:         ncp.Spec.APIServerRef,
		&controlplanev1.NestedControllerManager{}: ncp.Spec.ControllerManagerRef,
	}

	// Adopt NestedComponents in the same Namespace
	for component, nestedComponent := range nestedComponents {
		if nestedComponent != nil {
			objectKey := types.NamespacedName{Namespace: ncp.GetNamespace(), Name: nestedComponent.Name}
			if err := r.Get(ctx, objectKey, component); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{Requeue: true}, err
				}
			}

			if !util.HasOwner(component.GetOwnerReferences(), controlplanev1.GroupVersion.String(), []string{"NestedControlPlane"}) {
				log.Info("Component Missing Owner", "component", nestedComponent)
				addOwners = append(addOwners, component)
			}

			if commonObject, ok := component.(addonv1alpha1.CommonObject); ok {
				if IsComponentReady(commonObject.GetCommonStatus()) {
					isReady = append(isReady, 1)
				} else {
					log.Info("Component is not ready", "component", nestedComponent)
				}
			}
		}
	}

	// Add Controller Reference
	if err := r.reconcileControllerOwners(ctx, ncp, addOwners); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// Set Initialized
	if !ncp.Status.Initialized {
		conditions.MarkTrue(ncp, kcpv1.AvailableCondition)
		ncp.Status.Initialized = true
		if err := r.Status().Update(ctx, ncp); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set Ready
	if !ncp.Status.Ready && len(isReady) == 3 {
		conditions.MarkTrue(ncp, clusterv1.ReadyCondition)
		ncp.Status.Ready = true
		if err := r.Status().Update(ctx, ncp); err != nil {
			return ctrl.Result{}, err
		}
	} else if !ncp.Status.Ready && len(isReady) < 3 {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileKubeconfig will check if the control plane endpoint has been set
// and if so it will generate the KUBECONFIG or regenerate if it's expired.
func (r *NestedControlPlaneReconciler) reconcileKubeconfig(ctx context.Context, cluster *clusterv1.Cluster, ncp *controlplanev1.NestedControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	endpoint := cluster.Spec.ControlPlaneEndpoint
	if endpoint.IsZero() {
		return ctrl.Result{}, nil
	}

	controllerOwnerRef := *metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	clusterName := util.ObjectKey(cluster)
	configSecret, err := secret.GetFromNamespacedName(ctx, r.Client, clusterName, secret.Kubeconfig)
	switch {
	case apierrors.IsNotFound(err):
		createErr := kubeconfig.CreateSecretWithOwner(
			ctx,
			r.Client,
			clusterName,
			endpoint.String(),
			controllerOwnerRef,
		)
		if errors.Is(createErr, kubeconfig.ErrDependentCertificateNotFound) {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		// always return if we have just created in order to skip rotation checks
		return ctrl.Result{}, createErr
	case err != nil:
		return ctrl.Result{}, errors.Wrap(err, "failed to retrieve kubeconfig Secret")
	}

	// only do rotation on owned secrets
	if !util.IsControlledBy(configSecret, ncp) {
		return ctrl.Result{}, nil
	}

	needsRotation, err := kubeconfig.NeedsClientCertRotation(configSecret, certs.ClientCertificateRenewalDuration)
	if err != nil {
		return ctrl.Result{}, err
	}

	if needsRotation {
		log.Info("rotating kubeconfig secret")
		if err := kubeconfig.RegenerateSecret(ctx, r.Client, configSecret); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to regenerate kubeconfig")
		}
	}

	return ctrl.Result{}, nil
}

// reconcileControllerOwners will loop through any known nested components that
// aren't owned by a control plane yet and associate them
func (r *NestedControlPlaneReconciler) reconcileControllerOwners(ctx context.Context, ncp *controlplanev1.NestedControlPlane, addOwners []client.Object) error {
	for _, component := range addOwners {
		if err := ctrl.SetControllerReference(ncp, component, r.Scheme); err != nil {
			if _, ok := err.(*controllerutil.AlreadyOwnedError); !ok {
				continue
			}
			return err
		}

		if err := r.Update(ctx, component); err != nil {
			return err
		}
	}
	return nil
}
