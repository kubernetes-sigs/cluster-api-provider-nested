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

package certificate

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/pkg/errors"

	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/certificate/util"
)

// NewAPIServerCrtAndKey creates crt and key for apiserver using ca.
func NewAPIServerCrtAndKey(ca *KeyPair, clusterName, clusterDomainArg, apiserverDomain string, apiserverIPs ...string) (*KeyPair, error) {
	clusterDomain := defaultClusterDomain
	if clusterDomainArg != "" {
		clusterDomain = clusterDomainArg
	}
	apiserverIPs = append(apiserverIPs, "127.0.0.1", "0.0.0.0")

	// create AltNames with defaults DNSNames/IPs
	altNames := &cert.AltNames{
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			fmt.Sprintf("kubernetes.default.svc.%s", clusterDomain),
			apiserverDomain,
			// add virtual cluster name (i.e. namespace) for vn-agent.
			clusterName,
		},
	}

	for _, ip := range apiserverIPs {
		if ip != "" {
			altNames.IPs = append(altNames.IPs, net.ParseIP(ip))
		}
	}

	config := &util.CertConfig{
		Config: cert.Config{
			CommonName: clusterName,
			AltNames:   *altNames,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
	}

	apiCert, apiKey, err := util.NewCertAndKey(ca.Cert, ca.Key, config)
	if err != nil {
		return nil, fmt.Errorf("fail to create apiserver crt and key: %v", err)
	}

	rsaKey, ok := apiKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("fail to assert rsa private key")
	}

	return &KeyPair{APIServerClient, apiCert, rsaKey, true, true}, nil
}

// NewAPIServerKubeletClientCertAndKey creates certificate for the apiservers to connect to the
// kubelets securely, signed by the ca.
func NewAPIServerKubeletClientCertAndKey(ca *KeyPair) (*KeyPair, error) {
	config := &util.CertConfig{
		Config: cert.Config{
			CommonName:   "kube-apiserver-kubelet-client",
			Organization: []string{"system:masters"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}
	apiClientCert, apiClientKey, err := util.NewCertAndKey(ca.Cert, ca.Key, config)
	if err != nil {
		return &KeyPair{}, fmt.Errorf("failure while creating API server kubelet client key and certificate: %v", err)
	}

	rsaKey, ok := apiClientKey.(*rsa.PrivateKey)
	if !ok {
		return &KeyPair{}, errors.New("fail to assert rsa private key")
	}

	return &KeyPair{KubeletClient, apiClientCert, rsaKey, true, true}, nil
}

// NewEtcdServerCertAndKey creates new crt-key pair using ca for etcd.
func NewEtcdServerCertAndKey(ca *KeyPair, etcdDomains []string) (*KeyPair, error) {
	// create AltNames with defaults DNSNames/IPs
	altNames := &cert.AltNames{
		DNSNames: etcdDomains,
		IPs:      []net.IP{net.ParseIP("127.0.0.1")},
	}

	config := &util.CertConfig{
		Config: cert.Config{
			CommonName: "kube-etcd",
			AltNames:   *altNames,
			// all peers will use this crt-key pair as well.
			Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
	}
	etcdServerCert, etcdServerKey, err := util.NewCertAndKey(ca.Cert, ca.Key, config)
	if err != nil {
		return nil, fmt.Errorf("fail to create etcd crt and key: %v", err)
	}

	rsaKey, ok := etcdServerKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("fail to assert rsa private key")
	}

	return &KeyPair{EtcdClient, etcdServerCert, rsaKey, true, true}, nil
}

// NewEtcdHealthcheckClientCertAndKey creates certificate for liveness probes to healthcheck etcd,
// signed by the given ca.
func NewEtcdHealthcheckClientCertAndKey(ca *KeyPair) (*KeyPair, error) {
	config := &util.CertConfig{
		Config: cert.Config{
			CommonName:   "kube-etcd-healthcheck-client",
			Organization: []string{"system:masters"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}
	etcdHealcheckClientCert, etcdHealcheckClientKey, err := util.NewCertAndKey(ca.Cert, ca.Key, config)
	if err != nil {
		return &KeyPair{}, fmt.Errorf("failure while creating etcd healthcheck client key and certificate: %v", err)
	}

	rsaKey, ok := etcdHealcheckClientKey.(*rsa.PrivateKey)
	if !ok {
		return &KeyPair{}, errors.New("fail to assert rsa private key")
	}

	return &KeyPair{EtcdHealthClient, etcdHealcheckClientCert, rsaKey, true, true}, nil
}

// NewFrontProxyClientCertAndKey creates crt-key pair for proxy client using ca.
func NewFrontProxyClientCertAndKey(ca *KeyPair) (*KeyPair, error) {
	config := &util.CertConfig{
		Config: cert.Config{
			CommonName: "front-proxy-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}
	frontProxyClientCert, frontProxyClientKey, err := util.NewCertAndKey(ca.Cert, ca.Key, config)
	if err != nil {
		return nil, fmt.Errorf("fail to create crt and key for front-proxy: %v", err)
	}
	rsaKey, ok := frontProxyClientKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("fail to assert rsa private key")
	}
	return &KeyPair{ProxyClient, frontProxyClientCert, rsaKey, true, true}, nil
}
