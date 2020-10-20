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
	"crypto/rsa"
	errs "errors"
	"fmt"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
	controlplanev1alpha3 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-nested/constants"
	"sigs.k8s.io/cluster-api-provider-nested/kubeconfig"
	"sigs.k8s.io/cluster-api-provider-nested/pki"
	"sigs.k8s.io/cluster-api-provider-nested/secret"
)

// NestedPKIReconciler reconciles a NestedPKI object
type NestedPKIReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Event  record.EventRecorder
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedpkis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedpkis/status,verbs=get;update;patch

func (r *NestedPKIReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("nestedpki", req.NamespacedName)

	instance := &controlplanev1alpha3.NestedPKI{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch NestedPKI")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(instance, constants.NestedPKIFinalizer) {
			controllerutil.AddFinalizer(instance, constants.NestedPKIFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(instance, constants.NestedPKIFinalizer) {
			controllerutil.RemoveFinalizer(instance, constants.NestedPKIFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	owner := instance.Spec.ControlPlaneRef
	if owner == nil {
		return ctrl.Result{}, fmt.Errorf("pki not owned by control plane, cannot create certificates")
	}

	ncp := &v1alpha3.NestedControlPlane{}
	if err := r.Get(ctx, types.NamespacedName{Name: owner.Name, Namespace: owner.Namespace}, ncp); err != nil {
		return ctrl.Result{}, err
	}
	ns := ncp.Status.ClusterNamespace
	name := instance.Spec.ControlPlaneRef.Name

	caGroup := &pki.ClusterCAGroup{}
	// create root ca, all components will share a single root ca
	rootCACrt, rootKey, err := pkiutil.NewCertificateAuthority(
		&pkiutil.CertConfig{
			Config: cert.Config{
				CommonName:   "Kubernetes API",
				Organization: []string{"controlplane.cluster.sigs.k8s.io/nested"},
			},
		})
	if err != nil {
		return ctrl.Result{}, err
	}

	rootRsaKey, ok := rootKey.(*rsa.PrivateKey)
	if !ok {
		return ctrl.Result{}, errs.New("fail to assert rsa PrivateKey")
	}

	rootCAPair := &pki.CrtKeyPair{
		Crt: rootCACrt,
		Key: rootRsaKey,
	}
	caGroup.RootCA = rootCAPair

	// TODO(christopherhein) figure out how to get these from whatever
	// provisions the cluster because this needs to precreated
	etcdDomains := append([]string{"etcd-0.etcd"}, "etcd")
	// create crt, key for etcd
	etcdCAPair, err := pki.NewEtcdServerCrtAndKey(rootCAPair, etcdDomains)
	if err != nil {
		return ctrl.Result{}, err
	}
	caGroup.ETCD = etcdCAPair

	// TODO(christopherhein) figure out how to get these from the apiserver upfront
	// clusterIP := ""
	// if isClusterIP {
	// 	var err error
	// 	clusterIP, err = kubeutil.GetSvcClusterIP(mpn, ns, cv.Spec.APIServer.Service.GetName())
	// 	if err != nil {
	// 		log.Info("Warning: failed to get API Service", "service", cv.Spec.APIServer.Service.GetName(), "err", err)
	// 	}
	// }

	// TODO(christopherhein) figure out how to get these from whatever
	// provisions the cluster because this needs to precreated
	apiserverDomain := "apiserver" + ns
	apiserverCAPair, err := pki.NewAPIServerCrtAndKey(rootCAPair, ncp, apiserverDomain)
	if err != nil {
		return ctrl.Result{}, err
	}
	caGroup.APIServer = apiserverCAPair

	finalAPIAddress := apiserverDomain
	// TODO(christopherhein) figure out how to get these from the apiserver upfront
	// if clusterIP != "" {
	// 	finalAPIAddress = clusterIP
	// }

	// create kubeconfig for controller-manager
	ctrlmgrKbCfg, err := kubeconfig.GenerateKubeconfig(
		"system:kube-controller-manager",
		instance.Name, finalAPIAddress, []string{}, rootCAPair)
	if err != nil {
		return ctrl.Result{}, err
	}
	caGroup.CtrlMgrKbCfg = ctrlmgrKbCfg

	// create kubeconfig for admin user
	adminKbCfg, err := kubeconfig.GenerateKubeconfig(
		"admin", instance.Name, finalAPIAddress,
		[]string{"system:masters"}, rootCAPair)
	if err != nil {
		return ctrl.Result{}, err
	}
	caGroup.AdminKbCfg = adminKbCfg

	// create rsa key for service-account
	svcAcctCAPair, err := pki.NewServiceAccountSigningKey()
	if err != nil {
		return ctrl.Result{}, err
	}
	caGroup.ServiceAccountPrivateKey = svcAcctCAPair

	// store ca and kubeconfig into secrets
	err = r.createPKISecrets(caGroup, name, ns)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO(christopherhein) update status with proper all secret names
	instance.Status.Ready = true
	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NestedPKIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1alpha3.NestedPKI{}).
		Complete(r)
}

func (r *NestedPKIReconciler) createPKISecrets(caGroup *pki.ClusterCAGroup, name, namespace string) error {
	// create secret for root crt/key pair
	rootCrt := secret.CrtKeyPairToSecret(secret.GenerateSecretName(name, secret.CAPostfix),
		namespace, caGroup.RootCA)
	// create secret for apiserver crt/key pair
	apiserverCrt := secret.CrtKeyPairToSecret(secret.GenerateSecretName(name, secret.ProxyPostfix),
		namespace, caGroup.APIServer)
	// create secret for etcd crt/key pair
	etcdCrt := secret.CrtKeyPairToSecret(secret.GenerateSecretName(name, secret.EtcdPostfix),
		namespace, caGroup.ETCD)
	// create secret for controller manager kubeconfig
	ctrlMgrCrt := secret.KubeconfigToSecret(secret.ControllerManagerSecretName,
		namespace, caGroup.CtrlMgrKbCfg)
	// create secret for admin kubeconfig
	adminCrt := secret.KubeconfigToSecret(secret.AdminSecretName,
		namespace, caGroup.AdminKbCfg)
	// create secret for service account rsa key
	svcActCrt, err := secret.RsaKeyToSecret(secret.ServiceAccountSecretName,
		namespace, caGroup.ServiceAccountPrivateKey)
	if err != nil {
		return err
	}
	secrets := []*v1.Secret{rootCrt, apiserverCrt, etcdCrt,
		ctrlMgrCrt, adminCrt, svcActCrt}

	// create all secrets on metacluster
	for _, Crt := range secrets {
		r.Log.Info("creating secret", "name",
			Crt.Name, "namespace", Crt.Namespace)
		err := r.Create(context.TODO(), Crt)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			r.Log.Info("Secret already exists",
				"secret", Crt.Name,
				"namespace", Crt.Namespace)
		}
	}

	return nil
}
