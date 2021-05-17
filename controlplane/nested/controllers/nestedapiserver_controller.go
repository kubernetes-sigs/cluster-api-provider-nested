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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/certificate"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
)

// NestedAPIServerReconciler reconciles a NestedAPIServer object
type NestedAPIServerReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	TemplatePath string
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedapiservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedapiservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedapiservers/finalizers,verbs=update

func (r *NestedAPIServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nestedapiserver", req.NamespacedName)
	log.Info("Reconciling NestedAPIServer...")
	var nkas controlplanev1.NestedAPIServer
	if err := r.Get(ctx, req.NamespacedName, &nkas); err != nil {
		return ctrl.Result{}, ctrlcli.IgnoreNotFound(err)
	}
	log.Info("creating NestedAPIServer",
		"namespace", nkas.GetNamespace(),
		"name", nkas.GetName())

	// 1. check if the ownerreference has been set by the
	// NestedControlPlane controller.
	owner := getOwner(nkas.ObjectMeta)
	if owner == (metav1.OwnerReference{}) {
		// requeue the request if the owner NestedControlPlane has
		// not been set yet.
		log.Info("the owner has not been set yet, will retry later",
			"namespace", nkas.GetNamespace(),
			"name", nkas.GetName())
		return ctrl.Result{Requeue: true}, nil
	}

	var ncp controlplanev1.NestedControlPlane
	if err := r.Get(ctx, types.NamespacedName{Namespace: nkas.GetNamespace(), Name: owner.Name}, &ncp); err != nil {
		log.Info("the owner could not be found, will retry later",
			"namespace", nkas.GetNamespace(),
			"name", owner.Name)
		return ctrl.Result{}, ctrlcli.IgnoreNotFound(err)
	}

	cluster, err := ncp.GetOwnerCluster(ctx, r.Client)
	if err != nil || cluster == nil {
		log.Error(err, "Failed to retrieve owner Cluster from the control plane")
		return ctrl.Result{}, err
	}

	// 2. create the NestedAPIServer StatefulSet if not found
	nkasName := fmt.Sprintf("%s-apiserver", cluster.GetName())
	var nkasSts appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: nkas.GetNamespace(),
		Name:      nkasName,
	}, &nkasSts); err != nil {
		if apierrors.IsNotFound(err) {
			// as the statefulset is not found, mark the NestedAPIServer as unready
			if IsComponentReady(nkas.Status.CommonStatus) {
				nkas.Status.Phase =
					string(controlplanev1.Unready)
				log.V(5).Info("The corresponding statefulset is not found, " +
					"will mark the NestedAPIServer as unready")
				if err := r.Status().Update(ctx, &nkas); err != nil {
					log.Error(err, "fail to update the status of the NestedAPIServer Object")
					return ctrl.Result{}, err
				}
			}
			if err := r.createAPIServerClientCrts(ctx, cluster, &ncp, &nkas); err != nil {
				log.Error(err, "fail to create NestedAPIServer Client Certs")
				return ctrl.Result{}, err
			}

			// the statefulset is not found, create one
			if err := createNestedComponentSts(ctx,
				r.Client, nkas.ObjectMeta, nkas.Spec.NestedComponentSpec,
				controlplanev1.APIServer, owner.Name, cluster.GetName(), r.TemplatePath, log); err != nil {
				log.Error(err, "fail to create NestedAPIServer StatefulSet")
				return ctrl.Result{}, err
			}
			log.Info("successfully create the NestedAPIServer StatefulSet")
			return ctrl.Result{}, nil
		}
		log.Error(err, "fail to get NestedAPIServer StatefulSet")
		return ctrl.Result{}, err
	}

	// 3. reconcile the NestedAPIServer based on the status of the StatefulSet.
	// Mark the NestedAPIServer as Ready if the StatefulSet is ready
	if nkasSts.Status.ReadyReplicas == nkasSts.Status.Replicas {
		log.Info("The NestedAPIServer StatefulSet is ready")
		if !IsComponentReady(nkas.Status.CommonStatus) {
			// As the NestedAPIServer StatefulSet is ready, update
			// NestedAPIServer status
			nkas.Status.Phase = string(controlplanev1.Ready)
			objRef, err := genAPIServerSvcRef(r.Client, nkas, cluster.GetName())
			if err != nil {
				log.Error(err, "fail to generate NestedAPIServer Service Reference")
				return ctrl.Result{}, err
			}
			nkas.Status.APIServerService = &objRef

			log.V(5).Info("The corresponding statefulset is ready, " +
				"will mark the NestedAPIServer as ready")
			if err := r.Status().Update(ctx, &nkas); err != nil {
				log.Error(err, "fail to update NestedAPIServer Object")
				return ctrl.Result{}, err
			}
			log.Info("Successfully set the NestedAPIServer object to ready")
		}
		return ctrl.Result{}, nil
	}

	// mark the NestedAPIServer as unready, if the NestedAPIServer
	// StatefulSet is unready,
	if IsComponentReady(nkas.Status.CommonStatus) {
		nkas.Status.Phase = string(controlplanev1.Unready)
		if err := r.Status().Update(ctx, &nkas); err != nil {
			log.Error(err, "fail to update NestedAPIServer Object")
			return ctrl.Result{}, err
		}
		log.Info("Successfully set the NestedAPIServer object to unready")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NestedAPIServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(),
		&appsv1.StatefulSet{},
		statefulsetOwnerKeyNKas,
		func(rawObj ctrlcli.Object) []string {
			// grab the statefulset object, extract the owner
			sts := rawObj.(*appsv1.StatefulSet)
			owner := metav1.GetControllerOf(sts)
			if owner == nil {
				return nil
			}
			// make sure it's a NestedAPIServer
			if owner.APIVersion != controlplanev1.GroupVersion.String() ||
				owner.Kind != string(controlplanev1.APIServer) {
				return nil
			}

			// and if so, return it
			return []string{owner.Name}
		}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1.NestedAPIServer{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

// createAPIServerClientCrts will find of create client certs for the etcd cluster
func (r *NestedAPIServerReconciler) createAPIServerClientCrts(ctx context.Context, cluster *clusterv1.Cluster, ncp *controlplanev1.NestedControlPlane, nkas *controlplanev1.NestedAPIServer) error {
	certificates := secret.NewCertificatesForInitialControlPlane(nil)
	if err := certificates.Lookup(ctx, r.Client, util.ObjectKey(cluster)); err != nil {
		return err
	}
	cacert := certificates.GetByPurpose(secret.ClusterCA)
	if cacert == nil {
		return fmt.Errorf("could not fetch ClusterCA")
	}

	cacrt, err := certs.DecodeCertPEM(cacert.KeyPair.Cert)
	if err != nil {
		return err
	}

	cakey, err := certs.DecodePrivateKeyPEM(cacert.KeyPair.Key)
	if err != nil {
		return err
	}

	// TODO(christopherhein) figure out how to get service clusterIPs
	apiKeyPair, err := certificate.NewAPIServerCrtAndKey(&certificate.KeyPair{Cert: cacrt, Key: cakey}, nkas.GetName(), "", cluster.Spec.ControlPlaneEndpoint.Host)
	if err != nil {
		return err
	}

	kubeletKeyPair, err := certificate.NewAPIServerKubeletClientCertAndKey(&certificate.KeyPair{Cert: cacrt, Key: cakey})
	if err != nil {
		return err
	}

	fpcert := certificates.GetByPurpose(secret.FrontProxyCA)
	if cacert == nil {
		return fmt.Errorf("could not fetch FrontProxyCA")
	}

	fpcrt, err := certs.DecodeCertPEM(fpcert.KeyPair.Cert)
	if err != nil {
		return err
	}

	fpkey, err := certs.DecodePrivateKeyPEM(fpcert.KeyPair.Key)
	if err != nil {
		return err
	}

	frontProxyKeyPair, err := certificate.NewFrontProxyClientCertAndKey(&certificate.KeyPair{Cert: fpcrt, Key: fpkey})
	if err != nil {
		return err
	}

	certs := &certificate.KeyPairs{
		apiKeyPair,
		kubeletKeyPair,
		frontProxyKeyPair,
	}

	controllerRef := metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	if err := certs.LookupOrSave(ctx, r.Client, util.ObjectKey(cluster), *controllerRef); err != nil {
		return err
	}

	return nil
}
