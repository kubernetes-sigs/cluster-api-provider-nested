/*
Copyright 2022 The Kubernetes Authors.

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

package featuregate

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

var DefaultFeatureGate, _ = NewFeatureGate(nil)

// FeatureGate indicates whether a given feature is enabled or not
type FeatureGate interface {
	// Enabled returns true if the key is enabled.
	Enabled(key Feature) bool
	// KnownFeatures returns a slice of strings describing the FeatureGate's known features.
	KnownFeatures() []string
	// Set feature gate for known features.
	Set(key Feature, value bool) error
}

const (
	// SuperClusterServiceNetwork is an experimental feature that allows the
	// services to share the same ClusterIPs as the super cluster.
	SuperClusterServiceNetwork = "SuperClusterServiceNetwork"

	// SuperClusterPooling is an experimental feature that allows the syncer to
	// pool multiple super clusters for use with the experimental scheduler
	SuperClusterPooling = "SuperClusterPooling"

	// SuperClusterLabelling is an experimental feature that allows the syncer to
	// label managed resources in super cluster for easier filtering.
	SuperClusterLabelling = "SuperClusterLabelling"

	// SuperClusterLabelFilter is an experimental feature that allows the syncer to
	// use labels to filter managed resources in super cluster.
	// The feature requires SuperClusterLabelling=true.
	SuperClusterLabelFilter = "SuperClusterLabelFilter"

	// VNodeProviderService is an experimental feature that allows the
	// vn-agent to run as a load balanced deployment proxy to the super
	// cluster API Server
	VNodeProviderService = "VNodeProviderService"

	// TenantAllowDNSPolicy is an experimental feature that allows the
	// tenant pods to label themselves and use dnsPolicy's like `ClusterFirst`
	// from the super cluster
	TenantAllowDNSPolicy = "TenantAllowDNSPolicy"

	// VNodeProviderPodIP is an experimental feature that allows the
	// vn-agent to run as a daemonset but run without hostNetworking and
	// accessed by the PodIP on each pod on the node
	VNodeProviderPodIP = "VNodeProviderPodIP"

	// ClusterVersionPartialUpgrade is an experimental feature that allows the cluster provisioner
	// to apply ClusterVersion updates if VirtualCluster object is requested it
	ClusterVersionPartialUpgrade = "ClusterVersionPartialUpgrade"

	// TenantAllowResourceNoSync is an experimental feature that gives tenant the capability
	// of not syncing certain resources to super cluster.
	TenantAllowResourceNoSync = "TenantAllowResourceNoSync"

	// DisableCRDPreserveUnknownFields helps control syncing deprecated(k8s <= 1.20) field on CRD.
	// Enabling this will set spec.preserveUnknownField to false regardless the value in source CRD spec.
	DisableCRDPreserveUnknownFields = "DisableCRDPreserveUnknownFields"

	// RootCACertConfigMapSupport is an experimental feature that allows clusters +1.21 to support
	// the kube-root-ca.crt dropped into each Namespace
	RootCACertConfigMapSupport = "RootCACertConfigMapSupport"

	// VServiceExternalIP is an experimental feature that allows the syncer to
	// add clusterIP of pService to vService's externalIPs.
	// So that vService can be resolved by using the k8s_external plugin in coredns.
	VServiceExternalIP = "VServiceExternalIP"

	// KubeAPIAccessSupport is an experimental feature that allows clusters +1.21 to support
	// kube-api-access volume mount
	KubeAPIAccessSupport = "KubeAPIAccessSupport"
)

var defaultFeatures = FeatureList{
	SuperClusterPooling:             {Default: false},
	SuperClusterServiceNetwork:      {Default: false},
	SuperClusterLabelling:           {Default: false},
	SuperClusterLabelFilter:         {Default: false},
	VNodeProviderService:            {Default: false},
	TenantAllowDNSPolicy:            {Default: false},
	VNodeProviderPodIP:              {Default: false},
	ClusterVersionPartialUpgrade:    {Default: false},
	TenantAllowResourceNoSync:       {Default: false},
	DisableCRDPreserveUnknownFields: {Default: false},
	RootCACertConfigMapSupport:      {Default: false},
	VServiceExternalIP:              {Default: false},
	KubeAPIAccessSupport:            {Default: false},
}

type Feature string

// FeatureSpec represents a feature being gated
type FeatureSpec struct {
	Default bool
}

// FeatureList represents a list of feature gates
type FeatureList map[Feature]FeatureSpec

// Supports indicates whether a feature name is supported on the given
// feature set
func Supports(featureList FeatureList, featureName string) bool {
	for k := range featureList {
		if featureName == string(k) {
			return true
		}
	}
	return false
}

// featureGate implements FeatureGate
type featureGate struct {
	mu sync.Mutex
	// enabled holds a map[Feature]bool
	enabled *atomic.Value
}

// NewFeatureGate stores flag gates for known features from a map[string]bool or returns an error
func NewFeatureGate(m map[string]bool) (FeatureGate, error) {
	known := make(map[Feature]FeatureSpec)
	for k, v := range defaultFeatures {
		known[k] = v
	}

	for k, v := range m {
		if !Supports(defaultFeatures, k) {
			return nil, fmt.Errorf("unrecognized feature-gate key: %s", k)
		}
		known[Feature(k)] = FeatureSpec{Default: v}
	}

	enabledValue := &atomic.Value{}
	enabledValue.Store(known)

	return &featureGate{
		enabled: enabledValue,
	}, nil
}

// Enabled indicates whether a feature name has been enabled
func (f *featureGate) Enabled(key Feature) bool {
	if v, ok := f.enabled.Load().(map[Feature]FeatureSpec)[key]; ok {
		return v.Default
	}

	panic(fmt.Errorf("feature %q is not registered in FeatureGate", key))
}

// KnownFeatures returns a slice of strings describing the FeatureGate's known features.
// Deprecated and GA features are hidden from the list.
func (f *featureGate) KnownFeatures() []string {
	known := make([]string, 0)
	for k, v := range f.enabled.Load().(map[Feature]FeatureSpec) {
		known = append(known, fmt.Sprintf("%s=true|false (default=%t)", k, v.Default))
	}
	sort.Strings(known)
	return known
}

// Set feature gate for known features.
func (f *featureGate) Set(key Feature, value bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, known := defaultFeatures[key]; !known {
		return fmt.Errorf("unrecognized feature gate: %s", key)
	}

	enabled := map[Feature]FeatureSpec{}
	for k, v := range f.enabled.Load().(map[Feature]FeatureSpec) {
		enabled[k] = v
	}
	enabled[key] = FeatureSpec{Default: value}

	f.enabled.Store(enabled)

	return nil
}
