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

package kubeadm

const (
	// KubeadmExecPath denotes the path to the kubeadm executable.
	KubeadmExecPath = "/kubeadm"
	// KASManifestsPath denotes the Path to the apiserver static pod manifest.
	KASManifestsPath = "/etc/kubernetes/manifests/kube-apiserver.yaml"
	// KCMManifestsPath denotes the Path to the controller-manager static pod manifest.
	KCMManifestsPath = "/etc/kubernetes/manifests/kube-controller-manager.yaml"
	// EtcdManifestsPath denotes the Path to the etcd static pod manifest.
	EtcdManifestsPath = "/etc/kubernetes/manifests/etcd.yaml"
	// DefaultKubeadmConfigPath denotes the Path to the default kubeadm config.
	DefaultKubeadmConfigPath = "/kubeadm.config"
	// ManifestsConfigmapSuffix is the name of the configmap that will store the
	// manifests of the nested components' manifests.
	ManifestsConfigmapSuffix = "ncp-manifests"
	// APIServer denotes the name of the apiserver.
	APIServer = "apiserver"
	// ControllerManager denotes the name of the controller-manager.
	ControllerManager = "controller-manager"
	// Etcd denotes the name of the etcd.
	Etcd = "etcd"
)

var (
	// KASSubcommand is the command that generates the apiserver manifest.
	KASSubcommand = []string{"init", "phase", "control-plane", "apiserver", "--config", DefaultKubeadmConfigPath}
	// KCMSubcommand is the command that generates the controller-manager manifest.
	KCMSubcommand = []string{"init", "phase", "control-plane", "controller-manager", "--config", DefaultKubeadmConfigPath}
	// EtcdSubcommand is the command that generates the etcd manifest.
	EtcdSubcommand = []string{"init", "phase", "etcd", "local", "--config", DefaultKubeadmConfigPath}
)
