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

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha4"
)

// NestedEtcdReconciler reconciles a NestedEtcd object
type NestedEtcdReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedetcds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedetcds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=statefulset,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulset/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=,resources=service,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=,resources=service/status,verbs=get;update;patch

func (r *NestedEtcdReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("nestedetcd", req.NamespacedName)
	log.Info("Reconciling NestedEtcd...")
	var netcd clusterv1.NestedEtcd
	if err := r.Get(ctx, req.NamespacedName, &netcd); err != nil {
		return ctrl.Result{}, ctrlcli.IgnoreNotFound(err)
	}
	log.Info("creating NestedEtcd",
		"namespace", netcd.GetNamespace(),
		"name", netcd.GetName())

	// check if the ownerreference has been set by the NestedControlPlane controller.
	owner := getOwner(netcd.ObjectMeta)
	if owner == (metav1.OwnerReference{}) {
		// requeue the request if the owner NestedControlPlane has
		// not been set yet.
		log.Info("the owner has not been set yet, will retry later",
			"namespace", netcd.GetNamespace(),
			"name", netcd.GetName())
		return ctrl.Result{Requeue: true}, nil
	}

	var netcdSts appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      netcd.GetName(),
	}, &netcdSts); err != nil {
		if apierrors.IsNotFound(err) {
			// as the statefulset is not found, mark the NestedEtcd as unready
			if IsComponentReady(netcd.Status.CommonStatus) {
				netcd.Status.Phase =
					string(clusterv1.Unready)
				log.V(5).Info("The corresponding statefulset is not found, " +
					"will mark the NestedEtcd as unready")
				if err := r.Status().Update(ctx, &netcd); err != nil {
					log.Error(err, "fail to update the status of the NestedEtcd Object")
					return ctrl.Result{}, err
				}
			}
			// the statefulset is not found, create one
			if err := createNestedComponentSts(ctx,
				r.Client, netcd.ObjectMeta,
				netcd.Spec.NestedComponentSpec,
				clusterv1.Etcd, owner.Name, log); err != nil {
				log.Error(err, "fail to create NestedEtcd StatefulSet")
				return ctrl.Result{}, err
			}
			log.Info("successfully create the NestedEtcd StatefulSet")
			return ctrl.Result{}, nil
		}
		log.Error(err, "fail to get NestedEtcd StatefulSet")
		return ctrl.Result{}, err
	}

	if netcdSts.Status.ReadyReplicas == netcdSts.Status.Replicas {
		log.Info("The NestedEtcd StatefulSet is ready")
		if IsComponentReady(netcd.Status.CommonStatus) {
			// As the NestedEtcd StatefulSet is ready, update NestedEtcd status
			ip, err := getNestedEtcdSvcClusterIP(ctx, r.Client, netcd)
			if err != nil {
				log.Error(err, "fail to get NestedEtcd Service ClusterIP")
				return ctrl.Result{}, err
			}
			netcd.Status.Phase = string(clusterv1.Ready)
			netcd.Status.Addresses = []clusterv1.NestedEtcdAddress{
				{
					IP:   ip,
					Port: 2379,
				},
			}
			log.V(5).Info("The corresponding statefulset is ready, " +
				"will mark the NestedEtcd as ready")
			if err := r.Status().Update(ctx, &netcd); err != nil {
				log.Error(err, "fail to update NestedEtcd Object")
				return ctrl.Result{}, err
			}
			log.Info("Successfully set the NestedEtcd object to ready",
				"address", netcd.Status.Addresses)
		}
		return ctrl.Result{}, nil
	}

	// As the NestedEtcd StatefulSet is unready, mark the NestedEtcd as unready
	// if its current status is ready
	if IsComponentReady(netcd.Status.CommonStatus) {
		netcd.Status.Phase = string(clusterv1.Unready)
		if err := r.Status().Update(ctx, &netcd); err != nil {
			log.Error(err, "fail to update NestedEtcd Object")
			return ctrl.Result{}, err
		}
		log.Info("Successfully set the NestedEtcd object to unready")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NestedEtcdReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(),
		&appsv1.StatefulSet{},
		statefulsetOwnerKey,
		func(rawObj ctrlcli.Object) []string {
			// grab the statefulset object, extract the owner
			sts := rawObj.(*appsv1.StatefulSet)
			owner := metav1.GetControllerOf(sts)
			if owner == nil {
				return nil
			}
			// make sure it's a NestedEtcd
			if owner.APIVersion != clusterv1.GroupVersion.String() ||
				owner.Kind != "NestedEtcd" {
				return nil
			}

			// and if so, return it
			return []string{owner.Name}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.NestedEtcd{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

func getNestedEtcdSvcClusterIP(ctx context.Context, cli ctrlcli.Client,
	netcd clusterv1.NestedEtcd) (string, error) {
	var svc corev1.Service
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      netcd.GetName(),
	}, &svc); err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

// genInitialClusterArgs generates the values for `--inital-cluster` option of
// etcd based on the number of replicas specified in etcd StatefulSet
func genInitialClusterArgs(replicas int32,
	stsName, svcName string) (argsVal string) {
	for i := int32(0); i < replicas; i++ {
		// use 2380 as the default port for etcd peer communication
		peerAddr := fmt.Sprintf("%s-%d=https://%s-%d.%s:%d",
			stsName, i, stsName, i, svcName, 2380)
		if i == replicas-1 {
			argsVal = argsVal + peerAddr
			break
		}
		argsVal = argsVal + peerAddr + ","
	}

	return argsVal
}
