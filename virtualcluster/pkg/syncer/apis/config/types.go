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

package config

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	componentbaseconfig "k8s.io/component-base/config"
)

// SyncerConfiguration configures a syncer. It is read only during syncer life cycle.
type SyncerConfiguration struct {
	metav1.TypeMeta

	// LeaderElection defines the configuration of leader election client.
	LeaderElection SyncerLeaderElectionConfiguration

	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection componentbaseconfig.ClientConnectionConfiguration

	// DefaultOpaqueMetaDomains is the default configuration for each Virtual Cluster.
	// The key prefix of labels or annotations match this domain would be invisible to Virtual Cluster but
	// are kept in super cluster.
	// take tenant labels(annotations) ["foo=bar", "foo.kubernetes.io/foo=bar"] for example,
	// different configurations and possible final states are as follows:
	// DefaultOpaqueMetaDomains | labels(annotations) in super cluster
	// []                       | ["foo=bar", "foo.kubernetes.io/foo=bar"]
	// ["foo.kubernetes.io"]    | ["foo=bar", "foo.kubernetes.io/foo=foo", "foo.kubernetes.io/a=b"]
	// ["kubernetes.io"]        | ["foo=bar", "foo.kubernetes.io/foo=foo", "foo.kubernetes.io/a=b", "a.kubernetes.io/b=c"]
	// ["aaa"]                  | ["foo=bar", "foo.kubernetes.io/foo=bar", "aaa/b=c"]
	DefaultOpaqueMetaDomains []string

	//ExtraSyncingResources defines additional resources that need to be synced for each Virtual Cluster
	ExtraSyncingResources []string

	// DisableServiceAccountToken indicates whether to disable super cluster service account tokens being auto generated
	// and mounted in vc pods.
	DisableServiceAccountToken bool

	// DisablePodServiceLinks indicates whether to disable the `EnableServiceLinks` field in pPod spec.
	// Defaults to false, it won‘t mutate the EnableServiceLinks field in pPod spec.
	// If set to true, it will disable service links for all of the pPods to avoid massive env injections
	// from syncer which replace the kubelet generated envs.
	DisablePodServiceLinks bool

	// VNAgentPort defines the port that the VN Agent is running on per host
	VNAgentPort int32

	// VNAgentNamespacedName defines the namespace/name of the VN Agent Kubernetes
	// service, this is used for feature VNodeProviderService.
	VNAgentNamespacedName string

	// FeatureGates enabled by the user.
	FeatureGates map[string]bool

	// Super cluster rest config
	RestConfig *rest.Config

	// The maximum length of time to wait before giving up on a server request. A value of "" means use default.
	Timeout string

	// The DnsOptions are the DNS options in resolv.conf that is attached to pod
	DnsOptions []v1.PodDNSConfigOption
}

// SyncerLeaderElectionConfiguration expands LeaderElectionConfiguration
// to include syncer specific configuration.
type SyncerLeaderElectionConfiguration struct {
	componentbaseconfig.LeaderElectionConfiguration
	// LockObjectNamespace defines the namespace of the lock object
	LockObjectNamespace string
	// LockObjectName defines the lock object name
	LockObjectName string
}
