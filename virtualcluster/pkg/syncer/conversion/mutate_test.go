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

package conversion

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
)

func Test_mutateDownwardAPIField(t *testing.T) {
	aPod := newPod()

	for _, tt := range []struct {
		name        string
		pod         *v1.Pod
		env         *v1.EnvVar
		expectedEnv *v1.EnvVar
	}{
		{
			name: "env without fieldRef",
			pod:  aPod,
			env: &v1.EnvVar{
				Name:      "env_name",
				Value:     "env_value",
				ValueFrom: nil,
			},
			expectedEnv: &v1.EnvVar{
				Name:      "env_name",
				Value:     "env_value",
				ValueFrom: nil,
			},
		},
		{
			name: "env with other fieldRef",
			pod:  aPod,
			env: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "spec.nodeName",
					},
				},
			},
			expectedEnv: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "spec.nodeName",
					},
				},
			},
		},
		{
			name: "env with metadata.name",
			pod:  aPod,
			env: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
			expectedEnv: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
		},
		{
			name: "env with metadata.namespace",
			pod:  aPod,
			env: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.namespace",
					},
				},
			},
			expectedEnv: &v1.EnvVar{
				Name:  "env_name",
				Value: aPod.Namespace,
			},
		},
		{
			name: "env with metadata.uid",
			pod:  aPod,
			env: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.uid",
					},
				},
			},
			expectedEnv: &v1.EnvVar{
				Name:  "env_name",
				Value: string(aPod.UID),
			},
		},
		{
			name: "env with metadata.annotations",
			pod:  aPod,
			env: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.annotations['anno1']",
					},
				},
			},
			expectedEnv: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.annotations['anno1']",
					},
				},
			},
		},
		{
			name: "env with metadata.labels",
			pod:  aPod,
			env: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.labels['label1']",
					},
				},
			},
			expectedEnv: &v1.EnvVar{
				Name: "env_name",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.labels['label1']",
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			mutateDownwardAPIField(tt.env, tt.pod)
			if !equality.Semantic.DeepEqual(tt.env, tt.expectedEnv) {
				tc.Errorf("expected env %+v, got %+v", tt.expectedEnv, tt.env)
			}
		})
	}
}

func Test_mutateContainerSecret(t *testing.T) {
	for _, tt := range []struct {
		name              string
		container         *v1.Container
		saSecretMap       map[string]string
		vPod              *v1.Pod
		expectedContainer *v1.Container
	}{
		{
			name: "normal case",
			container: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "some-mount",
						MountPath: "/path/to/mount",
						ReadOnly:  true,
					},
					{
						Name:      "service-token-secret-tenant",
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
						ReadOnly:  true,
					},
				},
			},
			saSecretMap: map[string]string{
				"service-token-secret-tenant": "service-token-secret",
			},
			vPod: &v1.Pod{
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "service-token-secret-tenant",
							VolumeSource: v1.VolumeSource{
								Secret: &v1.SecretVolumeSource{
									SecretName: "service-token-secret-tenant",
								},
							},
						},
					},
				},
			},
			expectedContainer: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "some-mount",
						MountPath: "/path/to/mount",
						ReadOnly:  true,
					},
					{
						Name:      "service-token-secret",
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
						ReadOnly:  true,
					},
				},
			},
		},
		{
			name: "customized secret, no change",
			container: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "local-token",
						MountPath: "/path/to/mount",
						ReadOnly:  true,
					},
					{
						Name:      "service-token-secret-tenant",
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
						ReadOnly:  true,
					},
				},
			},
			saSecretMap: map[string]string{
				"local-token": "service-token-secret",
			},
			vPod: &v1.Pod{
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "service-token-secret-tenant",
							VolumeSource: v1.VolumeSource{
								Secret: &v1.SecretVolumeSource{
									SecretName: "local-token",
								},
							},
						},
						{
							Name: "local-token",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/path/to/mount",
								},
							},
						},
					},
				},
			},
			expectedContainer: &v1.Container{
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "local-token",
						MountPath: "/path/to/mount",
						ReadOnly:  true,
					},
					{
						Name:      "service-token-secret-tenant",
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
						ReadOnly:  true,
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			mutateContainerSecret(tt.container, tt.saSecretMap, tt.vPod)
			if !equality.Semantic.DeepEqual(tt.container, tt.expectedContainer) {
				tc.Errorf("expected container %+v, got %+v", tt.expectedContainer, tt.container)
			}
		})
	}
}

func Test_mutateDNSConfig(t *testing.T) {
	podMutateCtxFunc := func(policy v1.DNSPolicy, config *v1.PodDNSConfig, hostNetwork bool) *PodMutateCtx {
		pPod := newPod(func(p *v1.Pod) {
			spec := v1.PodSpec{}
			spec.DNSPolicy = policy
			spec.HostNetwork = hostNetwork
			if config != nil {
				spec.DNSConfig = config
			}
			p.Spec = spec
		})
		return &PodMutateCtx{
			ClusterName: "sample",
			PPod:        pPod,
		}
	}
	defaultOptions := []v1.PodDNSConfigOption{
		{
			Name: "use-vc",
		},
		{
			Name:  "ndots",
			Value: pointer.StringPtr("5"),
		},
	}

	dnsDefault := v1.DNSDefault
	dnsClusterFirst := v1.DNSClusterFirst
	dnsNone := v1.DNSNone

	type args struct {
		p             *PodMutateCtx
		vPod          *v1.Pod
		clusterDomain string
		nameServer    string
		dnsoptions    []v1.PodDNSConfigOption
	}
	tests := []struct {
		name              string
		args              args
		allowDNSPolicy    bool
		expectedDNSPolicy *v1.DNSPolicy
		expectedDNSConfig *v1.PodDNSConfig
	}{
		{
			name: "dns policy set to none",
			args: args{
				p:          podMutateCtxFunc(v1.DNSNone, nil, false),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsNone,
		},
		{
			name: "dns policy set to none",
			args: args{
				p: podMutateCtxFunc(v1.DNSNone, &v1.PodDNSConfig{
					Nameservers: []string{"0.0.0.0"},
				}, false),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsNone,
			expectedDNSConfig: &v1.PodDNSConfig{
				Nameservers: []string{"0.0.0.0"},
			},
		},
		{
			name: "dns policy set to cluster first with hostnetwork",
			args: args{
				p:          podMutateCtxFunc(v1.DNSClusterFirstWithHostNet, nil, false),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsNone,
			expectedDNSConfig: &v1.PodDNSConfig{
				Nameservers: []string{"0.0.0.0"},
				Options:     defaultOptions,
			},
		},
		{
			name: "dns policy set to cluster first",
			args: args{
				p:          podMutateCtxFunc(v1.DNSClusterFirst, nil, false),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsNone,
			expectedDNSConfig: &v1.PodDNSConfig{
				Nameservers: []string{"0.0.0.0"},
				Options:     defaultOptions,
			},
		},
		{
			name: "dns policy set to cluster first with config",
			args: args{
				p: podMutateCtxFunc(v1.DNSClusterFirst, &v1.PodDNSConfig{
					Nameservers: []string{"127.0.0.1"},
					Options:     defaultOptions,
				}, false),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsNone,
			expectedDNSConfig: &v1.PodDNSConfig{
				Nameservers: []string{"0.0.0.0", "127.0.0.1"},
				Options:     defaultOptions,
			},
		},
		{
			name: "dns policy set to cluster first host network",
			args: args{
				p:          podMutateCtxFunc(v1.DNSClusterFirst, nil, true),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsClusterFirst,
			expectedDNSConfig: nil,
		},
		{
			name: "dns policy set to default",
			args: args{
				p:          podMutateCtxFunc(v1.DNSDefault, nil, true),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsDefault,
			expectedDNSConfig: nil,
		},
		{
			name: "dns policy set to cluster first with disable mutation label",
			args: args{
				p: podMutateCtxFunc(v1.DNSClusterFirst, nil, false),
				vPod: newPod(func(p *v1.Pod) {
					p.ObjectMeta.Labels[constants.TenantDisableDNSPolicyMutation] = "true"
				}),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			expectedDNSPolicy: &dnsNone,
			expectedDNSConfig: &v1.PodDNSConfig{
				Nameservers: []string{"0.0.0.0"},
				Options:     defaultOptions,
			},
		},
		{
			name: "dns policy set to cluster first with tenant dns policy mutation not active",
			args: args{
				p:          podMutateCtxFunc(v1.DNSClusterFirst, nil, false),
				vPod:       newPod(),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			allowDNSPolicy:    true,
			expectedDNSPolicy: &dnsNone,
		},
		{
			name: "dns policy set to cluster first with tenant dns policy mutation and disable mutation label",
			args: args{
				p: podMutateCtxFunc(v1.DNSClusterFirst, &v1.PodDNSConfig{Nameservers: []string{"127.0.0.1"}}, false),
				vPod: newPod(func(p *v1.Pod) {
					p.ObjectMeta.Labels[constants.TenantDisableDNSPolicyMutation] = "true"
				}),
				nameServer: "0.0.0.0",
				dnsoptions: []v1.PodDNSConfigOption{
					{
						Name: "use-vc",
					},
					{
						Name:  "ndots",
						Value: pointer.StringPtr("5"),
					},
				},
			},
			allowDNSPolicy:    true,
			expectedDNSPolicy: &dnsClusterFirst,
			expectedDNSConfig: &v1.PodDNSConfig{Nameservers: []string{"127.0.0.1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.allowDNSPolicy {
				featuregate.DefaultFeatureGate.Set(featuregate.TenantAllowDNSPolicy, true)
			}
			mutateDNSConfig(tt.args.p, tt.args.vPod, tt.args.clusterDomain, tt.args.nameServer, tt.args.dnsoptions)
			if tt.expectedDNSPolicy != nil {
				if tt.args.p.PPod.Spec.DNSPolicy != *tt.expectedDNSPolicy {
					t.Errorf("expected DNSPolicy %+v, got %+v", *tt.expectedDNSPolicy, tt.args.p.PPod.Spec.DNSPolicy)
				}
			}

			if tt.expectedDNSConfig != nil {
				if !equality.Semantic.DeepEqual(tt.args.p.PPod.Spec.DNSConfig, tt.expectedDNSConfig) {
					t.Errorf("expected DNSConfig %+v, got %+v", *tt.expectedDNSConfig, tt.args.p.PPod.Spec.DNSConfig)
				}
			}
		})
	}
}

func newPod(fns ...func(*v1.Pod)) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "ns",
			UID:       types.UID("5033b5b7-104f-11ea-b309-525400c042d5"),
			Labels:    map[string]string{},
		},
	}

	for _, fn := range fns {
		fn(pod)
	}

	return pod
}
