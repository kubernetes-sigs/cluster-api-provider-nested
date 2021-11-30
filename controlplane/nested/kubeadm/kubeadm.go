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

// Package kubeadm contains functions that used to generate pod manifests
// of the nested control-plane using the kubeadm.
package kubeadm

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// DefaultKubeadmConfig denotes the content of the default kubeadm config.
const DefaultKubeadmConfig = `
apiVersion: kubeadm.k8s.io/v1beta3
certificatesDir: /etc/kubernetes/pki
clusterName: kubernetes
imageRepository: k8s.gcr.io
kind: ClusterConfiguration
kubernetesVersion: 1.21.1
apiServer:
  timeoutForControlPlane: 4m0s
  extraArgs:
    advertise-address:                0.0.0.0
    client-ca-file:                   /etc/kubernetes/pki/apiserver/ca/tls.crt
    tls-cert-file:                    /etc/kubernetes/pki/apiserver/tls.crt
    tls-private-key-file:             /etc/kubernetes/pki/apiserver/tls.key
    kubelet-certificate-authority:    /etc/kubernetes/pki/apiserver/ca/tls.crt
    kubelet-client-certificate:       /etc/kubernetes/pki/kubelet/tls.crt
    kubelet-client-key:               /etc/kubernetes/pki/kubelet/tls.key
    etcd-cafile:                      /etc/kubernetes/pki/etcd/ca/tls.crt
    etcd-certfile:                    /etc/kubernetes/pki/etcd/tls.crt
    etcd-keyfile:                     /etc/kubernetes/pki/etcd/tls.key
    service-account-key-file:         /etc/kubernetes/pki/service-account/tls.key
    service-account-signing-key-file: /etc/kubernetes/pki/service-account/tls.key
    proxy-client-cert-file:           /etc/kubernetes/pki/proxy/tls.crt
    proxy-client-key-file:            /etc/kubernetes/pki/proxy/tls.key
    requestheader-client-ca-file:     /etc/kubernetes/pki/proxy/ca/tls.crt

controllerManager:
  extraArgs:
    bind-address:                     0.0.0.0
    cluster-signing-cert-file:        /etc/kubernetes/pki/root/tls.crt
    cluster-signing-key-file:         /etc/kubernetes/pki/root/tls.key
    kubeconfig:                       /etc/kubernetes/kubeconfig/controller-manager-kubeconfig
    authorization-kubeconfig:         /etc/kubernetes/kubeconfig/controller-manager-kubeconfig
    authentication-kubeconfig:        /etc/kubernetes/kubeconfig/controller-manager-kubeconfig
    leader-elect:                     "false"
    requestheader-client-ca-file:     /etc/kubernetes/pki/proxy/ca/tls.crt
    client-ca-file:                   ""
    root-ca-file:                     /etc/kubernetes/pki/root/ca/tls.crt
    service-account-private-key-file: /etc/kubernetes/pki/service-account/tls.key
    controllers:                      "*,-nodelifecycle,bootstrapsigner,tokencleaner"
       
etcd:
  local:
    dataDir: /var/lib/etcd
    extraArgs: 
      trusted-ca-file:             /etc/kubernetes/pki/ca/tls.crt
      client-cert-auth:            "true"
      cert-file:                   /etc/kubernetes/pki/etcd/tls.crt
      key-file:                    /etc/kubernetes/pki/etcd/tls.key
      peer-client-cert-auth:       "true"
      peer-trusted-ca-file:        /etc/kubernetes/pki/ca/tls.crt
      peer-cert-file:              /etc/kubernetes/pki/etcd/tls.crt
      peer-key-file:               /etc/kubernetes/pki/etcd/tls.key
      listen-peer-urls:            https://0.0.0.0:2380
      listen-client-urls:          https://0.0.0.0:2379
      name:                        $(HOSTNAME)
      data-dir:                    /var/lib/etcd/data`

func execCommand(log logr.Logger, command string, subcommand ...string) error {
	cmd := exec.Command(command, subcommand...)
	var (
		out    bytes.Buffer
		stderr bytes.Buffer
	)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Error(err, "fail to run kubeadm", "stderr", stderr.String())
		return err
	}
	log.Info("successfully execute the kubeadm command", "stdout", out.String())
	return nil
}

// GenerateTemplates generates the manifests for the nested apiserver,
// controller-manager and etcd by calling the `kubeadm init phase control-plane/etcd`.
func GenerateTemplates(log logr.Logger, clusterName string) (map[string]string, error) {
	// create the cluster manifests directory if not exist
	if err := os.MkdirAll("/"+clusterName, 0755); err != nil {
		return nil, errors.Wrap(err, "fail to create the cluster manifests directory")
	}
	// defer os.RemoveAll("/" + clusterName)
	log.Info("cluster manifests directory is created")
	if err := generateKubeadmConfig(clusterName); err != nil {
		return nil, err
	}
	log.Info("kubeadmconfig is generated")

	// store manifests of different nested cluster in different directory
	KASSubcommand = append(KASSubcommand, "--rootfs", "/"+clusterName)
	KCMSubcommand = append(KCMSubcommand, "--rootfs", "/"+clusterName)
	EtcdSubcommand = append(EtcdSubcommand, "--rootfs", "/"+clusterName)
	// generate the manifests
	if err := execCommand(log, KubeadmExecPath, KASSubcommand...); err != nil {
		return nil, errors.Wrap(err, "fail to generate the apiserver manifests")
	}
	if err := execCommand(log, KubeadmExecPath, KCMSubcommand...); err != nil {
		return nil, errors.Wrap(err, "fail to generate the controller-manager manifests")
	}
	if err := execCommand(log, KubeadmExecPath, EtcdSubcommand...); err != nil {
		return nil, errors.Wrap(err, "fail to generate the etcd manifests")
	}
	log.Info("static pod manifests generated")

	var loadErr error
	kasPath := filepath.Join("/", clusterName, KASManifestsPath)
	KASManifests, loadErr := ioutil.ReadFile(filepath.Clean(kasPath))
	if loadErr != nil {
		return nil, errors.Wrap(loadErr, "fail to load the apiserver manifests")
	}
	kcmPath := filepath.Join("/", clusterName, KCMManifestsPath)
	KCMManifests, loadErr := ioutil.ReadFile(filepath.Clean(kcmPath))
	if loadErr != nil {
		return nil, errors.Wrap(loadErr, "fail to load the controller-manager manifests")
	}
	etcdPath := filepath.Join("/", clusterName, EtcdManifestsPath)
	EtcdManifests, loadErr := ioutil.ReadFile(filepath.Clean(etcdPath))
	if loadErr != nil {
		return nil, errors.Wrap(loadErr, "fail to load the etcd manifests")
	}

	return map[string]string{
		APIServer:         string(KASManifests),
		ControllerManager: string(KCMManifests),
		Etcd:              string(EtcdManifests),
	}, nil
}

// generateKubeadmConfig writes the DefaultKubeadmConfig to the DefaultKubeadmConfigPath,
// which will be read by the `kubeadm init` command.
func generateKubeadmConfig(clusterName string) error {
	completedKubeadmConfig, err := completeDefaultKubeadmConfig(clusterName)
	if err != nil {
		return err
	}
	return os.WriteFile("/"+clusterName+DefaultKubeadmConfigPath, []byte(completedKubeadmConfig), 0600)
}

func completeDefaultKubeadmConfig(clusterName string) (string, error) {
	config := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(DefaultKubeadmConfig), &config); err != nil {
		return "", err
	}
	kasConfig := config["apiServer"].(map[interface{}]interface{})
	kasExtraConfig, ok := kasConfig["extraArgs"].(map[interface{}]interface{})
	if !ok {
		return "", errors.Errorf("fail to assert apiServer.extraArgs to map")
	}
	kasExtraConfig["etcd-servers"] = "https://" + clusterName + "-etcd-0." + clusterName + "-etcd.$(NAMESPACE):2379"

	etcdConfig := config["etcd"].(map[interface{}]interface{})
	etcdLocalConfig, ok := etcdConfig["local"].(map[interface{}]interface{})
	if !ok {
		return "", errors.Errorf("fail to assert etcd.local to map")
	}
	etcdExtraConfig, ok := etcdLocalConfig["extraArgs"].(map[interface{}]interface{})
	if !ok {
		return "", errors.Errorf("fail to assert etcd.local.extraArgs to map")
	}
	etcdExtraConfig["initial-advertise-peer-urls"] = "https://$(HOSTNAME)." + clusterName + "-etcd.$(NAMESPACE).svc:2380"
	etcdExtraConfig["advertise-client-urls"] = "https://$(HOSTNAME)." + clusterName + "-etcd.$(NAMESPACE).svc:2379"
	completedConfigYaml, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(completedConfigYaml), nil
}
