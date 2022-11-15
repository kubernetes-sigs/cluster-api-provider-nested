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

package provisioner

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	tenancyv1alpha1 "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/kubeconfig"
	vcpki "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/pki"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/secret"
	kubeutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/kube"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	pkiutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/pki"
)

const (
	DefaultETCDPeerPort    = 2380
	ComponentPollPeriodSec = 2
)

var (
	definitelyTrue = true
	patchOptions   = &client.PatchOptions{Force: &definitelyTrue, FieldManager: "virtualcluster/provisioner/native"}
)

type Native struct {
	client.Client
	scheme             *runtime.Scheme
	Log                logr.Logger
	ProvisionerTimeout time.Duration
}

func NewProvisionerNative(mgr manager.Manager, log logr.Logger, provisionerTimeout time.Duration) (*Native, error) {
	return &Native{
		Client:             mgr.GetClient(),
		scheme:             mgr.GetScheme(),
		Log:                log.WithName("Native"),
		ProvisionerTimeout: provisionerTimeout,
	}, nil
}

func updateLabelClusterVersionApplied(vc *tenancyv1alpha1.VirtualCluster, cv *tenancyv1alpha1.ClusterVersion) {
	if featuregate.DefaultFeatureGate.Enabled(featuregate.ClusterVersionPartialUpgrade) {
		if vc.Labels == nil {
			vc.Labels = map[string]string{}
		}
		vc.Labels[constants.LabelClusterVersionApplied] = cv.ObjectMeta.ResourceVersion
	}
}

// CreateVirtualCluster sets up the control plane for vc on meta k8s
func (mpn *Native) CreateVirtualCluster(ctx context.Context, vc *tenancyv1alpha1.VirtualCluster) error {
	cv, err := mpn.fetchClusterVersion(vc)
	if err != nil {
		return err
	}

	updateLabelClusterVersionApplied(vc, cv)

	// 1. create the root ns
	_, err = kubeutil.CreateRootNS(mpn, vc)
	if err != nil {
		return err
	}
	return mpn.applyVirtualCluster(ctx, cv, vc, true)
}

func (mpn *Native) fetchClusterVersion(vc *tenancyv1alpha1.VirtualCluster) (*tenancyv1alpha1.ClusterVersion, error) {
	cvObjectKey := client.ObjectKey{Name: vc.Spec.ClusterVersionName}
	cv := &tenancyv1alpha1.ClusterVersion{}
	if err := mpn.Get(context.Background(), cvObjectKey, cv); err != nil {
		err = fmt.Errorf("desired ClusterVersion %s not found",
			vc.Spec.ClusterVersionName)
		return nil, err
	}
	return cv, nil
}

func (mpn *Native) UpgradeVirtualCluster(ctx context.Context, vc *tenancyv1alpha1.VirtualCluster) error {
	cv, err := mpn.fetchClusterVersion(vc)
	if err != nil {
		return err
	}
	if cvVersion, ok := vc.Labels[constants.LabelClusterVersionApplied]; ok && cvVersion == cv.ObjectMeta.ResourceVersion {
		mpn.Log.Info("cluster is already in desired version")
		return nil
	}
	updateLabelClusterVersionApplied(vc, cv)

	// We currently do not support ETCD upgrades because of amount of manual actions required
	// The easiest way to achieve it - pass empty ETCD definition to the ClusterVersion
	return mpn.applyVirtualCluster(ctx, cv, vc, false)
}

func (mpn *Native) applyVirtualCluster(ctx context.Context, cv *tenancyv1alpha1.ClusterVersion, vc *tenancyv1alpha1.VirtualCluster, applyETCD bool) error {
	var err error
	isClusterIP := cv.Spec.APIServer.Service != nil && cv.Spec.APIServer.Service.Spec.Type == corev1.ServiceTypeClusterIP
	// if ClusterIP, have to update API Server ahead of time to lay it down in the PKI
	if isClusterIP {
		mpn.Log.Info("applying ClusterIP Service for API component", "component", cv.Spec.APIServer.Name)
		complementAPIServerTemplate(conversion.ToClusterKey(vc), cv.Spec.APIServer, nil)
		err := mpn.Patch(ctx, cv.Spec.APIServer.Service, client.Apply, patchOptions)
		if err != nil {
			mpn.Log.Error(err, "failed to update service", "service", cv.Spec.APIServer.Service.GetName())
			return err
		}
	}

	// 2. apply PKI
	clusterCAGroup, err := mpn.createAndApplyPKI(ctx, vc, cv, isClusterIP)
	if err != nil {
		return err
	}

	// 3. deploy etcd if defined
	if applyETCD {
		err = mpn.deployComponent(ctx, vc, cv.Spec.ETCD, clusterCAGroup)
		if err != nil {
			return err
		}
	}

	// 4. deploy apiserver (must be defined always)
	err = mpn.deployComponent(ctx, vc, cv.Spec.APIServer, clusterCAGroup)
	if err != nil {
		return err
	}

	// 5. deploy controller-manager if defined
	if cv.Spec.ControllerManager != nil {
		err = mpn.deployComponent(ctx, vc, cv.Spec.ControllerManager, clusterCAGroup)
		if err != nil {
			return err
		}
	}
	return nil
}

// genInitialClusterArgs generates the values for `--initial-cluster` option of etcd based on the number of
// replicas specified in etcd StatefulSet
func genInitialClusterArgs(replicas int32, stsName, svcName string) (argsVal string) {
	for i := int32(0); i < replicas; i++ {
		// use 2380 as the default port for etcd peer communication
		peerAddr := fmt.Sprintf("%s-%d=https://%s-%d.%s:%d",
			stsName, i, stsName, i, svcName, DefaultETCDPeerPort)
		if i == replicas-1 {
			argsVal += peerAddr
			break
		}
		argsVal = argsVal + peerAddr + ","
	}

	return argsVal
}

// complementETCDTemplate complements the ETCD template of the specified clusterversion
// based on the virtual cluster setting
func complementETCDTemplate(vcns string, etcdBdl *tenancyv1alpha1.StatefulSetSvcBundle) {
	etcdBdl.StatefulSet.ObjectMeta.Namespace = vcns
	etcdBdl.Service.ObjectMeta.Namespace = vcns
	args := etcdBdl.StatefulSet.Spec.Template.Spec.Containers[0].Args
	icaVal := genInitialClusterArgs(*etcdBdl.StatefulSet.Spec.Replicas,
		etcdBdl.StatefulSet.Name, etcdBdl.Service.Name)
	args = append(args, "--initial-cluster", icaVal)
	etcdBdl.StatefulSet.Spec.Template.Spec.Containers[0].Args = args

	labels := etcdBdl.StatefulSet.Spec.Template.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[constants.LabelCluster] = vcns
	etcdBdl.StatefulSet.Spec.Template.SetLabels(labels)
}

// complementAPIServerTemplate complements the apiserver template of the specified clusterversion
// based on the virtual cluster setting
func complementAPIServerTemplate(vcns string, apiserverBdl *tenancyv1alpha1.StatefulSetSvcBundle, clusterCAGroup *vcpki.ClusterCAGroup) {
	apiserverBdl.StatefulSet.ObjectMeta.Namespace = vcns
	apiserverBdl.Service.ObjectMeta.Namespace = vcns

	// we use complementAPIServerTemplate for service creation before creating certs if the service isClusterIP
	if clusterCAGroup == nil {
		return
	}

	annotations := apiserverBdl.StatefulSet.Spec.Template.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[secret.RootCASecretName+"-hash"] = secret.GetHash(clusterCAGroup.RootCA)
	annotations[secret.APIServerCASecretName+"-hash"] = secret.GetHash(clusterCAGroup.APIServer)
	annotations[secret.FrontProxyCASecretName+"-hash"] = secret.GetHash(clusterCAGroup.FrontProxy)
	annotations[secret.ServiceAccountSecretName+"-hash"] = secret.GetHash(clusterCAGroup.ServiceAccountPrivateKey)
	apiserverBdl.StatefulSet.Spec.Template.SetAnnotations(annotations)

	labels := apiserverBdl.StatefulSet.Spec.Template.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[constants.LabelCluster] = vcns
	apiserverBdl.StatefulSet.Spec.Template.SetLabels(labels)
}

// complementCtrlMgrTemplate complements the controller manager template of the specified clusterversion
// based on the virtual cluster setting
func complementCtrlMgrTemplate(vcns string, ctrlMgrBdl *tenancyv1alpha1.StatefulSetSvcBundle, clusterCAGroup *vcpki.ClusterCAGroup) {
	ctrlMgrBdl.StatefulSet.ObjectMeta.Namespace = vcns
	annotations := ctrlMgrBdl.StatefulSet.Spec.Template.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[secret.RootCASecretName+"-hash"] = secret.GetHash(clusterCAGroup.RootCA)
	annotations[secret.ServiceAccountSecretName+"-hash"] = secret.GetHash(clusterCAGroup.ServiceAccountPrivateKey)
	annotations[secret.ControllerManagerSecretName+"-hash"] = secret.GetHash(clusterCAGroup.CtrlMgrKbCfg)
	ctrlMgrBdl.StatefulSet.Spec.Template.SetAnnotations(annotations)

	labels := ctrlMgrBdl.StatefulSet.Spec.Template.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[constants.LabelCluster] = vcns
	ctrlMgrBdl.StatefulSet.Spec.Template.SetLabels(labels)
}

// deployComponent deploys control plane component in namespace vcName based on the given StatefulSet
// and Service Bundle ssBdl
// the method also adds annotations with certificates hashes to trigger pod recreation if certificates were changed
func (mpn *Native) deployComponent(ctx context.Context, vc *tenancyv1alpha1.VirtualCluster, ssBdl *tenancyv1alpha1.StatefulSetSvcBundle, clusterCAGroup *vcpki.ClusterCAGroup) error {
	mpn.Log.Info("deploying StatefulSet for control plane component", "component", ssBdl.Name)

	ns := conversion.ToClusterKey(vc)

	switch ssBdl.Name {
	case "etcd":
		complementETCDTemplate(ns, ssBdl)
	case "apiserver":
		complementAPIServerTemplate(ns, ssBdl, clusterCAGroup)
	case "controller-manager":
		complementCtrlMgrTemplate(ns, ssBdl, clusterCAGroup)
	default:
		return fmt.Errorf("try to deploy unknown component: %s", ssBdl.Name)
	}

	err := mpn.Patch(ctx, ssBdl.StatefulSet, client.Apply, patchOptions)
	if err != nil {
		return err
	}

	// skip apiserver clusterIP service creation as it is already created in CreateVirtualCluster()
	if ssBdl.Service != nil && !(ssBdl.Name == "apiserver" && ssBdl.Service.Spec.Type == corev1.ServiceTypeClusterIP) {
		mpn.Log.Info("deploying Service for control plane component", "component", ssBdl.Name)
		err := mpn.Patch(ctx, ssBdl.Service, client.Apply, patchOptions)
		if err != nil {
			return err
		}
	}

	// wait for the statefuleset to be ready
	err = kubeutil.WaitStatefulSetReady(mpn, ns, ssBdl.Name, int64(mpn.ProvisionerTimeout/time.Second), ComponentPollPeriodSec)
	if err != nil {
		return err
	}
	return nil
}

// createOrUpdatePKISecrets creates secrets to store crt/key pairs and kubeconfigs
// for control plane components of the virtual cluster
func (mpn *Native) createOrUpdatePKISecrets(ctx context.Context, caGroup *vcpki.ClusterCAGroup, namespace string) error {
	// create secret for root crt/key pair
	rootSrt := secret.CrtKeyPairToSecret(secret.RootCASecretName, namespace, caGroup.RootCA)
	// create secret for apiserver crt/key pair
	apiserverSrt := secret.CrtKeyPairToSecret(secret.APIServerCASecretName,
		namespace, caGroup.APIServer)
	// create secret for etcd crt/key pair
	etcdSrt := secret.CrtKeyPairToSecret(secret.ETCDCASecretName,
		namespace, caGroup.ETCD)
	// create secret for front proxy crt/key pair
	frontProxySrt := secret.CrtKeyPairToSecret(secret.FrontProxyCASecretName,
		namespace, caGroup.FrontProxy)
	// create secret for controller manager kubeconfig
	ctrlMgrSrt := secret.KubeconfigToSecret(secret.ControllerManagerSecretName,
		namespace, caGroup.CtrlMgrKbCfg)
	// create secret for admin kubeconfig
	adminSrt := secret.KubeconfigToSecret(secret.AdminSecretName,
		namespace, caGroup.AdminKbCfg)
	// create secret for service account rsa key
	svcActSrt, err := secret.RsaKeyToSecret(secret.ServiceAccountSecretName,
		namespace, caGroup.ServiceAccountPrivateKey)
	if err != nil {
		return err
	}
	secrets := []*corev1.Secret{rootSrt, apiserverSrt, etcdSrt, frontProxySrt,
		ctrlMgrSrt, adminSrt, svcActSrt}

	// create all secrets on metacluster
	for _, srt := range secrets {
		mpn.Log.Info("applying secret", "name",
			srt.Name, "namespace", srt.Namespace)

		err := mpn.Patch(ctx, srt, client.Apply, patchOptions)
		if err != nil {
			return err
		}
	}

	return nil
}

// createAndApplyPKI constructs the PKI (all crt/key pair and kubeconfig) for the
// virtual clusters, and store them as secrets in the meta cluster
// The method returns the current ClusterCAGroup to use it as annotations for control-plane pods for restart
func (mpn *Native) createAndApplyPKI(ctx context.Context, vc *tenancyv1alpha1.VirtualCluster, cv *tenancyv1alpha1.ClusterVersion, isClusterIP bool) (*vcpki.ClusterCAGroup, error) {
	ns := conversion.ToClusterKey(vc)
	caGroup := &vcpki.ClusterCAGroup{}

	var rootCAPair *vcpki.CrtKeyPair

	// reuse rootCa if it is present
	rootCaSecret := &corev1.Secret{}
	err := mpn.Get(ctx, client.ObjectKey{Name: secret.RootCASecretName, Namespace: vc.Status.ClusterNamespace}, rootCaSecret)
	switch {
	case err == nil:
		rootCACrt, rootCAErr := pkiutil.DecodeCertPEM(rootCaSecret.Data[corev1.TLSCertKey])
		if rootCAErr != nil {
			return nil, rootCAErr
		}
		rootCAKey, rootCAErr := vcpki.DecodePrivateKeyPEM(rootCaSecret.Data[corev1.TLSPrivateKeyKey])
		if rootCAErr != nil {
			return nil, rootCAErr
		}
		rootCAPair = &vcpki.CrtKeyPair{
			Crt: rootCACrt,
			Key: rootCAKey,
		}
		mpn.Log.Info("rootCA pair is reused from the secret")
	case apierrors.IsNotFound(err):
		mpn.Log.Info("rootCA secret is not found. Creating")
		rootCACrt, rootKey, rootCAErr := pkiutil.NewCertificateAuthority(
			&pkiutil.CertConfig{
				Config: cert.Config{
					CommonName:   "kubernetes",
					Organization: []string{"kubernetes-sig.kubernetes-sigs/multi-tenancy.virtualcluster"},
				},
			})
		if rootCAErr != nil {
			return nil, rootCAErr
		}
		rootRsaKey, ok := rootKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("fail to assert rsa PrivateKey")
		}
		rootCAPair = &vcpki.CrtKeyPair{
			Crt: rootCACrt,
			Key: rootRsaKey,
		}
		mpn.Log.Info("rootCA pair generated")
	default:
		mpn.Log.Error(err, "failed to check rootCA secret existence")
		return nil, err
	}
	caGroup.RootCA = rootCAPair

	etcdDomains := append(cv.GetEtcdServers(), cv.GetEtcdDomain())
	// We may want to connect to etcd from controllers namespace
	// So we duplicate etcdDomains here with the namespace
	for _, etcdDomain := range etcdDomains {
		etcdDomains = append(etcdDomains, etcdDomain+"."+ns)
	}
	// create crt, key for etcd
	etcdCAPair, etcdCrtErr := vcpki.NewEtcdServerCertAndKey(rootCAPair, etcdDomains)
	if etcdCrtErr != nil {
		return nil, etcdCrtErr
	}
	caGroup.ETCD = etcdCAPair

	// create crt, key for frontendproxy
	frontProxyCAPair, frontProxyCrtErr := vcpki.NewFrontProxyClientCertAndKey(rootCAPair)
	if frontProxyCrtErr != nil {
		return nil, frontProxyCrtErr
	}
	caGroup.FrontProxy = frontProxyCAPair

	clusterIP := ""
	if isClusterIP {
		var err error
		clusterIP, err = kubeutil.GetSvcClusterIP(mpn, conversion.ToClusterKey(vc), cv.Spec.APIServer.Service.GetName())
		if err != nil {
			mpn.Log.Info("Warning: failed to get API Service", "service", cv.Spec.APIServer.Service.GetName(), "err", err)
		}
	}

	apiserverDomain := cv.GetAPIServerDomain(ns)
	apiserverCAPair, err := vcpki.NewAPIServerCrtAndKey(rootCAPair, vc, apiserverDomain, clusterIP)
	if err != nil {
		return nil, err
	}
	caGroup.APIServer = apiserverCAPair

	finalAPIAddress := apiserverDomain
	if clusterIP != "" {
		finalAPIAddress = clusterIP
	}

	// create kubeconfig for controller-manager
	ctrlmgrKbCfg, err := kubeconfig.GenerateKubeconfig(
		"system:kube-controller-manager",
		vc.Name, finalAPIAddress, []string{}, rootCAPair)
	if err != nil {
		return nil, err
	}
	caGroup.CtrlMgrKbCfg = ctrlmgrKbCfg

	// create kubeconfig for admin user
	adminKbCfg, err := kubeconfig.GenerateKubeconfig(
		"admin", vc.Name, finalAPIAddress,
		[]string{"system:masters"}, rootCAPair)
	if err != nil {
		return nil, err
	}
	caGroup.AdminKbCfg = adminKbCfg

	// create rsa key for service-account
	svcAcctCAPair, err := vcpki.NewServiceAccountSigningKey()
	if err != nil {
		return nil, err
	}
	caGroup.ServiceAccountPrivateKey = svcAcctCAPair

	// store ca and kubeconfig into secrets
	genSrtsErr := mpn.createOrUpdatePKISecrets(ctx, caGroup, ns)
	if genSrtsErr != nil {
		return nil, genSrtsErr
	}

	return caGroup, nil
}

func (mpn *Native) DeleteVirtualCluster(_ context.Context, _ *tenancyv1alpha1.VirtualCluster) error {
	return nil
}

func (mpn *Native) GetProvisioner() string {
	return "native"
}
