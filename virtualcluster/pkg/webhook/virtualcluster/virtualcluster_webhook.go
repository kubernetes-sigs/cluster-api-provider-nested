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

package virtualcluster

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	admv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	tenancyv1alpha1 "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/constants"
	kubeutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/kube"
)

const (
	VCWebhookCertCommonName   = "virtualcluster-webhook"
	VCWebhookCertOrg          = "virtualcluster"
	VCWebhookCertFileName     = "tls.crt"
	VCWebhookKeyFileName      = "tls.key"
	VCWebhookServiceName      = "virtualcluster-webhook-service"
	DefaultVCWebhookServiceNs = "vc-manager"
	VCWebhookCfgName          = "virtualcluster-validating-webhook-configuration"
	VCWebhookCSRName          = "virtualcluster-webhook-csr"
)

var (
	VCWebhookServiceNs string
	log                = logf.Log.WithName("virtualcluster-webhook")
)

func init() {
	VCWebhookServiceNs, _ = kubeutil.GetPodNsFromInside()
	if VCWebhookServiceNs == "" {
		log.Info("setup virtualcluster webhook in default namespace",
			"default-ns", DefaultVCWebhookServiceNs)
		VCWebhookServiceNs = DefaultVCWebhookServiceNs
	}
}

// Add adds the webhook server to the manager as a runnable
func Add(mgr manager.Manager, certDir string) error {
	// 1. create the webhook service
	if err := createVirtualClusterWebhookService(mgr.GetClient()); err != nil {
		return fmt.Errorf("fail to create virtualcluster webhook service: %s", err)
	}

	// 2. generate the serving certificate for the webhook server
	caPEM, genCrtErr := genCertificate(mgr, certDir)
	if genCrtErr != nil {
		return fmt.Errorf("fail to generate certificates for webhook server: %s", genCrtErr)
	}

	// 3. create the ValidatingWebhookConfiguration
	log.Info(fmt.Sprintf("will create validatingwebhookconfiguration/%s", VCWebhookCfgName))
	if err := createValidatingWebhookConfiguration(mgr.GetClient(), caPEM); err != nil {
		return fmt.Errorf("fail to create validating webhook configuration: %s", err)
	}
	log.Info(fmt.Sprintf("successfully created validatingwebhookconfiguration/%s", VCWebhookCfgName))

	// 4. register the validating webhook
	return (&tenancyv1alpha1.VirtualCluster{}).SetupWebhookWithManager(mgr)
}

// createVirtualClusterWebhookService creates the service for exposing the webhook server
func createVirtualClusterWebhookService(client client.Client) error {
	whSvc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VCWebhookServiceName,
			Namespace: VCWebhookServiceNs,
			Labels: map[string]string{
				"virtualcluster-webhook": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       constants.VirtualClusterWebhookPort,
					TargetPort: intstr.FromInt(constants.VirtualClusterWebhookPort),
				},
			},
			Selector: map[string]string{
				"virtualcluster-webhook": "true",
			},
		},
	}
	if err := client.Create(context.TODO(), &whSvc); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		log.Info(fmt.Sprintf("service/%s already exist", VCWebhookServiceName))
		return nil
	}
	log.Info(fmt.Sprintf("successfully created service/%s", VCWebhookServiceName))
	return nil
}

// createValidatingWebhookConfiguration creates the validatingwebhookconfiguration for the webhook
func createValidatingWebhookConfiguration(client client.Client, caPEM []byte) error {
	validatePath := "/validate-tenancy-x-k8s-io-v1alpha1-virtualcluster"
	svcPort := int32(constants.VirtualClusterWebhookPort)
	// reject request if the webhook doesn't work
	failPolicy := admv1beta1.Fail
	vwhCfg := admv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: VCWebhookCfgName,
			Labels: map[string]string{
				"virtualcluster-webhook": "true",
			},
		},
		Webhooks: []admv1beta1.ValidatingWebhook{
			{
				Name: "virtualcluster.validating.webhook",
				ClientConfig: admv1beta1.WebhookClientConfig{
					Service: &admv1beta1.ServiceReference{
						Name:      VCWebhookServiceName,
						Namespace: VCWebhookServiceNs,
						Path:      &validatePath,
						Port:      &svcPort,
					},
					CABundle: caPEM,
				},
				FailurePolicy: &failPolicy,
				Rules: []admv1beta1.RuleWithOperations{
					{
						Operations: []admv1beta1.OperationType{
							admv1beta1.OperationAll,
						},
						Rule: admv1beta1.Rule{
							APIGroups:   []string{"tenancy.x-k8s.io"},
							APIVersions: []string{"v1alpha1"},
							Resources:   []string{"virtualclusters"},
						},
					},
				},
			},
		},
	}

	if err := client.Create(context.TODO(), &vwhCfg); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		log.Info(fmt.Sprintf("validatingwebhookconfiguration/%s already exist", VCWebhookCfgName))
		return nil
	}
	log.Info(fmt.Sprintf("successfully created validatingwebhookconfiguration/%s", VCWebhookCfgName))
	return nil
}

// genCertificate generates the serving cerficiate for the webhook server
func genCertificate(mgr manager.Manager, certDir string) ([]byte, error) {
	caPEM, certPEM, keyPEM, err := genSelfSignedCert()
	if err != nil {
		log.Error(err, "fail to generate self-signed certificate")
		return nil, err
	}

	// 5. generate certificate files (i.e., tls.crt and tls.key)
	if err := genCertAndKeyFile(certPEM, keyPEM, certDir); err != nil {
		return nil, fmt.Errorf("fail to generate certificate and key: %s", err)
	}

	return caPEM, nil
}

// genSelfSignedCert generates the self signed Certificate/Key pair
func genSelfSignedCert() (caPEMByte, certPEMByte, keyPEMByte []byte, err error) {
	// CA config
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2021),
		Subject: pkix.Name{
			Organization: []string{"tenancy.x-k8s.io"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// CA private key
	caPrvKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	// Self signed CA certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrvKey.PublicKey, caPrvKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// PEM encode CA cert
	caPEM := new(bytes.Buffer)
	_ = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	caPEMByte = caPEM.Bytes()

	dnsNames := []string{
		VCWebhookServiceName,
		VCWebhookServiceName + "." + VCWebhookServiceNs,
		VCWebhookServiceName + "." + VCWebhookServiceNs + ".svc",
	}
	commonName := VCWebhookServiceName + "." + VCWebhookServiceNs + ".svc"

	// server cert config
	cert := &x509.Certificate{
		DNSNames:     dnsNames,
		SerialNumber: big.NewInt(2021),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"tenancy.x-k8s.io"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	// server private key
	certPrvKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	// sign the server cert
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrvKey.PublicKey, caPrvKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// PEM encode the  server cert and key
	certPEM := new(bytes.Buffer)
	_ = pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	certPEMByte = certPEM.Bytes()

	certPrvKeyPEM := new(bytes.Buffer)
	_ = pem.Encode(certPrvKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrvKey),
	})
	keyPEMByte = certPrvKeyPEM.Bytes()

	return
}

// genCertAndKeyFile creates the serving certificate/key files for the webhook server
func genCertAndKeyFile(certData, keyData []byte, certDir string) error {
	// always remove first
	if err := os.RemoveAll(certDir); err != nil {
		return fmt.Errorf("fail to remove certificates: %s", err)
	}
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("could not create directory %q to store certificates: %v", certDir, err)
	}
	certPath := filepath.Join(certDir, VCWebhookCertFileName)
	f, err := os.OpenFile(certPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("could not open %q: %v", certPath, err)
	}
	defer f.Close()
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return fmt.Errorf("invalid certificate data")
	}
	pem.Encode(f, certBlock)

	keyPath := filepath.Join(certDir, VCWebhookKeyFileName)
	kf, err := os.OpenFile(keyPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("could not open %q: %v", keyPath, err)
	}

	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return fmt.Errorf("invalid key data")
	}
	pem.Encode(kf, keyBlock)
	log.Info("successfully generate certificate and key file")
	return nil
}
