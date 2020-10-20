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

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/cluster-api-provider-nested/constants"
	"sigs.k8s.io/cluster-api-provider-nested/kubeconfig"
	"sigs.k8s.io/cluster-api-provider-nested/pki"
	"sigs.k8s.io/cluster-api-provider-nested/secret"
	"sigs.k8s.io/cluster-api-provider-nested/utils"

	"sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
	controlplanev1alpha3 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
)

// NestedControlPlaneReconciler reconciles a NestedControlPlane object
type NestedControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Event  record.EventRecorder
}

// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *NestedControlPlaneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("nestedcontrolplane", req.NamespacedName)

	instance := &controlplanev1alpha3.NestedControlPlane{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to fetch NestedControlPlane")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(instance, constants.NestedControlPlaneFinalizer) {
			controllerutil.AddFinalizer(instance, constants.NestedControlPlaneFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(instance, constants.NestedControlPlaneFinalizer) {
			controllerutil.RemoveFinalizer(instance, constants.NestedControlPlaneFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	var template *v1alpha3.NestedControlPlaneTemplate
	if instance.Spec.TemplateRef == nil {
		templateList := v1alpha3.NestedControlPlaneTemplateList{}
		if err := r.List(ctx, &templateList); err != nil {
			return ctrl.Result{}, err
		}

		for _, temp := range templateList.Items {
			if temp.Spec.Default == true {
				template = &temp
			}
		}

		if template == nil {
			r.Event.Event(instance, corev1.EventTypeWarning, "NotFound", "'default' control plane template not found")
			return ctrl.Result{}, nil
		}

		instance.Spec.TemplateRef = &corev1.ObjectReference{
			Kind:       template.Kind,
			APIVersion: template.APIVersion,
			Name:       template.GetName(),
		}

		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check for Namespace created
	if created, err := r.checkOrCreateNamespace(ctx, instance); err != nil || created {
		return ctrl.Result{Requeue: created}, err
	}

	// // Check for PKI cr, if not create
	// if created, err := r.createPKI(ctx, instance); err != nil || created {
	// 	return ctrl.Result{Requeue: created}, err
	// }

	// Check if etcd cr exists, if not create
	// if created, err := r.checkOrCreateEtcd(ctx, instance); err != nil || created {
	// 	return ctrl.Result{Requeue: created}, err
	// }

	// Check if apiserver cr exists, if not create

	// Check if controller-manager cr exists, if not create

	return ctrl.Result{}, nil
}

func (r *NestedControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO(christopherhein) update this with indexes for Namespaces,
	// NestedPKI, NestedEtcd, NestedAPIServer, NestedControllerManager

	return ctrl.NewControllerManagedBy(mgr).
		For(&controlplanev1alpha3.NestedControlPlane{}).
		Complete(r)
}

func (r *NestedControlPlaneReconciler) checkOrCreateNamespace(ctx context.Context, instance *v1alpha3.NestedControlPlane) (created bool, err error) {
	namespace := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.ClusterNamespace}, namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return
	} else if apierrors.IsNotFound(err) {
		namespace = makeNamespace(instance)
		if err = r.Create(ctx, namespace); err != nil {
			return
		}

		instance.Status.ClusterNamespace = instance.Spec.ClusterNamespace
		if err = r.Status().Update(ctx, instance); err != nil {
			return
		}

		created = true
	}
	return
}

// TODO(christopherhein) Figure out how to make this dynamically provisioned
// should we have a separate secret controller and store references on the
// control plane object so that we know these are created?
func (r *NestedControlPlaneReconciler) createPKI(ctx context.Context, instance *v1alpha3.NestedControlPlane) (created bool, err error) {
	ns := instance.Status.ClusterNamespace
	name := instance.GetName()
	caGroup := &pki.ClusterCAGroup{}
	// create root ca, all components will share a single root ca
	rootCACrt, rootKey, rootCAErr := pkiutil.NewCertificateAuthority(
		&pkiutil.CertConfig{
			Config: cert.Config{
				CommonName:   "Kubernetes API",
				Organization: []string{"controlplane.cluster.sigs.k8s.io/nested"},
			},
		})
	if rootCAErr != nil {
		return false, rootCAErr
	}

	rootRsaKey, ok := rootKey.(*rsa.PrivateKey)
	if !ok {
		return false, errs.New("fail to assert rsa PrivateKey")
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
	etcdCAPair, etcdCrtErr := pki.NewEtcdServerCrtAndKey(rootCAPair, etcdDomains)
	if etcdCrtErr != nil {
		return false, etcdCrtErr
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
	apiserverCAPair, err := pki.NewAPIServerCrtAndKey(rootCAPair, instance, apiserverDomain)
	if err != nil {
		return false, err
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
		return false, err
	}
	caGroup.CtrlMgrKbCfg = ctrlmgrKbCfg

	// create kubeconfig for admin user
	adminKbCfg, err := kubeconfig.GenerateKubeconfig(
		"admin", instance.Name, finalAPIAddress,
		[]string{"system:masters"}, rootCAPair)
	if err != nil {
		return false, err
	}
	caGroup.AdminKbCfg = adminKbCfg

	// create rsa key for service-account
	svcAcctCAPair, err := pki.NewServiceAccountSigningKey()
	if err != nil {
		return false, err
	}
	caGroup.ServiceAccountPrivateKey = svcAcctCAPair

	// store ca and kubeconfig into secrets
	genCrtsErr := r.createPKISecrets(caGroup, name, ns)
	if genCrtsErr != nil {
		return false, genCrtsErr
	}
	return true, nil
}

func (r *NestedControlPlaneReconciler) createPKISecrets(caGroup *pki.ClusterCAGroup, name, namespace string) error {
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

func makeNamespace(instance *v1alpha3.NestedControlPlane) *corev1.Namespace {
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Spec.ClusterNamespace,
		},
	}

	utils.SetClusterLabels(instance, namespace, true)

	return namespace
}
