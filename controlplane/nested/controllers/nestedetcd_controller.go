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
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controlplanev1alpha4 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/certs"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controlplanev1 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/certificate"
	"sigs.k8s.io/cluster-api/util"
)

// NestedEtcdReconciler reconciles a NestedEtcd object.
type NestedEtcdReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	TemplatePath string
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedetcds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedetcds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedetcds/finalizers,verbs=update

func (r *NestedEtcdReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling NestedEtcd...")
	var netcd controlplanev1.NestedEtcd
	if err := r.Get(ctx, req.NamespacedName, &netcd); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
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

	var ncp controlplanev1.NestedControlPlane
	if err := r.Get(ctx, types.NamespacedName{Namespace: netcd.GetNamespace(), Name: owner.Name}, &ncp); err != nil {
		log.Info("the owner could not be found, will retry later",
			"namespace", netcd.GetNamespace(),
			"name", owner.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cluster, err := ncp.GetOwnerCluster(ctx, r.Client)
	if err != nil || cluster == nil {
		log.Error(err, "Failed to retrieve owner Cluster from the control plane")
		return ctrl.Result{}, err
	}

	etcdName := fmt.Sprintf("%s-etcd", cluster.GetName())
	var netcdSts appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      etcdName,
	}, &netcdSts); err != nil {
		if apierrors.IsNotFound(err) {
			// as the statefulset is not found, mark the NestedEtcd as unready
			if IsComponentReady(netcd.Status.CommonStatus) {
				netcd.Status.Phase =
					string(controlplanev1.Unready)
				log.V(5).Info("The corresponding statefulset is not found, " +
					"will mark the NestedEtcd as unready")
				if err := r.Status().Update(ctx, &netcd); err != nil {
					log.Error(err, "fail to update the status of the NestedEtcd Object")
					return ctrl.Result{}, err
				}
			}

			if err := r.createEtcdClientCrts(ctx, cluster, &ncp, &netcd); err != nil {
				log.Error(err, "fail to create NestedEtcd Client Certs")
				return ctrl.Result{}, err
			}

			// the statefulset is not found, create one
			if err := createNestedComponentSts(ctx,
				r.Client, netcd.ObjectMeta,
				netcd.Spec.NestedComponentSpec,
				controlplanev1.Etcd, owner.Name, cluster.GetName(), r.TemplatePath, log); err != nil {
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
		if !IsComponentReady(netcd.Status.CommonStatus) {
			// As the NestedEtcd StatefulSet is ready, update NestedEtcd status
			ip, err := getNestedEtcdSvcClusterIP(ctx, r.Client, cluster.GetName(), &netcd)
			if err != nil {
				log.Error(err, "fail to get NestedEtcd Service ClusterIP")
				return ctrl.Result{}, err
			}
			netcd.Status.Phase = string(controlplanev1.Ready)
			netcd.Status.Addresses = []controlplanev1.NestedEtcdAddress{
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
		netcd.Status.Phase = string(controlplanev1.Unready)
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
		statefulsetOwnerKeyNEtcd,
		func(rawObj client.Object) []string {
			// grab the statefulset object, extract the owner
			sts := rawObj.(*appsv1.StatefulSet)
			owner := metav1.GetControllerOf(sts)
			if owner == nil {
				return nil
			}
			// make sure it's a NestedEtcd
			if owner.APIVersion != controlplanev1.GroupVersion.String() ||
				owner.Kind != "NestedEtcd" {
				return nil
			}

			// and if so, return it
			return []string{owner.Name}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1.NestedEtcd{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

func getNestedEtcdSvcClusterIP(ctx context.Context, cli client.Client,
	clusterName string, netcd *controlplanev1.NestedEtcd) (string, error) {
	var svc corev1.Service
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: netcd.GetNamespace(),
		Name:      fmt.Sprintf("%s-etcd", clusterName),
	}, &svc); err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

// genInitialClusterArgs generates the values for `--initial-cluster` option of
// etcd based on the number of replicas specified in etcd StatefulSet.
func genInitialClusterArgs(replicas int32,
	stsName, svcName, svcNamespace string) (argsVal string) {
	for i := int32(0); i < replicas; i++ {
		// use 2380 as the default port for etcd peer communication
		peerAddr := fmt.Sprintf("%s-etcd-%d=https://%s-etcd-%d.%s-etcd.%s.svc:%d",
			stsName, i, stsName, i, svcName, svcNamespace, 2380)
		if i == replicas-1 {
			argsVal += peerAddr
			break
		}
		argsVal += peerAddr + ","
	}

	return argsVal
}

func getEtcdServers(name, namespace string, replicas int32) (etcdServers []string) {
	var i int32
	for ; i < replicas; i++ {
		etcdServers = append(etcdServers, fmt.Sprintf("%s-etcd-%d.%s-etcd.%s", name, i, name, namespace))
	}
	etcdServers = append(etcdServers, name)
	return etcdServers
}

// createEtcdClientCrts will find of create client certs for the etcd cluster.
func (r *NestedEtcdReconciler) createEtcdClientCrts(ctx context.Context, cluster *controlplanev1alpha4.Cluster, ncp *controlplanev1.NestedControlPlane, netcd *controlplanev1.NestedEtcd) error {
	certificates := secret.NewCertificatesForInitialControlPlane(nil)
	if err := certificates.Lookup(ctx, r.Client, util.ObjectKey(cluster)); err != nil {
		return err
	}
	cert := certificates.GetByPurpose(secret.EtcdCA)
	if cert == nil {
		return fmt.Errorf("could not fetch EtcdCA")
	}

	crt, err := certs.DecodeCertPEM(cert.KeyPair.Cert)
	if err != nil {
		return err
	}

	key, err := certs.DecodePrivateKeyPEM(cert.KeyPair.Key)
	if err != nil {
		return err
	}

	etcdKeyPair, err := certificate.NewEtcdServerCertAndKey(&certificate.KeyPair{Cert: crt, Key: key}, getEtcdServers(cluster.GetName(), cluster.GetNamespace(), netcd.Spec.Replicas))
	if err != nil {
		return err
	}

	etcdHealthKeyPair, err := certificate.NewEtcdHealthcheckClientCertAndKey(&certificate.KeyPair{Cert: crt, Key: key})
	if err != nil {
		return err
	}

	certs := &certificate.KeyPairs{
		etcdKeyPair,
		etcdHealthKeyPair,
	}

	controllerRef := metav1.NewControllerRef(ncp, controlplanev1.GroupVersion.WithKind("NestedControlPlane"))
	return certs.LookupOrSave(ctx, r.Client, util.ObjectKey(cluster), *controllerRef)
}
