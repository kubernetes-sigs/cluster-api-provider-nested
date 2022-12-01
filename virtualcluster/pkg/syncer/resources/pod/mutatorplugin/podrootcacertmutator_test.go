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

package mutatorplugin

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	util "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/test"
)

func tenantPod(name, namespace, uid string, fns ...func(*corev1.Pod)) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default",
			InitContainers: []corev1.Container{
				{
					Image: "busybox",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "root-ca-cert",
							MountPath: "/path",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name: "root-ca-crt-env",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: constants.RootCACertConfigMapName},
									Key:                  "ca.crt",
								},
							},
						},
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: constants.RootCACertConfigMapName},
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Image: "busybox",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "root-ca-cert",
							MountPath: "/path",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name: "root-ca-crt-env",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: constants.RootCACertConfigMapName},
									Key:                  "ca.crt",
								},
							},
						},
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: constants.RootCACertConfigMapName},
							},
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "root-ca-cert",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: constants.RootCACertConfigMapName},
						},
					},
				},
				{
					Name: "projected-volume-source",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									ConfigMap: &corev1.ConfigMapProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: constants.RootCACertConfigMapName},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, fn := range fns {
		fn(p)
	}

	return p
}

func TestPodRootCACertMutatorPlugin_Mutator(t *testing.T) {
	defer util.SetFeatureGateDuringTest(t, featuregate.DefaultFeatureGate, featuregate.RootCACertConfigMapSupport, true)()

	tests := []struct {
		name string
		pPod *corev1.Pod
	}{
		{
			"Test RootCACert Mutator",
			tenantPod("test", "default", "123-456-789"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := &PodRootCACertMutatorPlugin{}
			mutator := pl.Mutator()

			if err := mutator(&conversion.PodMutateCtx{PPod: tt.pPod}); err != nil {
				t.Errorf("mutator failed processing the pod")
			}

			for _, volume := range tt.pPod.Spec.Volumes {
				if volume.ConfigMap != nil && volume.ConfigMap.Name != constants.TenantRootCACertConfigMapName {
					t.Errorf("tt.pPod.Spec.Volumes[*].ConfigMap.Name = %v, want %v", volume.ConfigMap.Name, constants.TenantRootCACertConfigMapName)
				}

				if volume.Projected != nil {
					for _, source := range volume.Projected.Sources {
						if source.ConfigMap != nil && source.ConfigMap.Name != constants.TenantRootCACertConfigMapName {
							t.Errorf("tt.pPod.Spec.Volumes[*].Projected.Sources[*].ConfigMap.Name = %v, want %v", source.ConfigMap.Name, constants.TenantRootCACertConfigMapName)
						}
					}
				}
			}

			for c := range tt.pPod.Spec.Containers {
				for e := range tt.pPod.Spec.Containers[c].Env {
					if tt.pPod.Spec.Containers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name != constants.TenantRootCACertConfigMapName {
						t.Errorf("tt.pPod.Spec.Containers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name = %v, want %v", tt.pPod.Spec.Containers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name, constants.TenantRootCACertConfigMapName)
					}
				}
				for e := range tt.pPod.Spec.Containers[c].EnvFrom {
					if tt.pPod.Spec.Containers[c].EnvFrom[e].ConfigMapRef.Name != constants.TenantRootCACertConfigMapName {
						t.Errorf("tt.pPod.Spec.Containers[c].EnvFrom[e].ConfigMapRef.Name = %v, want %v", tt.pPod.Spec.Containers[c].EnvFrom[e].ConfigMapRef.Name, constants.TenantRootCACertConfigMapName)
					}
				}
			}

			for c := range tt.pPod.Spec.InitContainers {
				for e := range tt.pPod.Spec.InitContainers[c].Env {
					if tt.pPod.Spec.InitContainers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name != constants.TenantRootCACertConfigMapName {
						t.Errorf("tt.pPod.Spec.Containers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name = %v, want %v", tt.pPod.Spec.InitContainers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name, constants.TenantRootCACertConfigMapName)
					}
				}
				for e := range tt.pPod.Spec.InitContainers[c].EnvFrom {
					if tt.pPod.Spec.InitContainers[c].EnvFrom[e].ConfigMapRef.Name != constants.TenantRootCACertConfigMapName {
						t.Errorf("tt.pPod.Spec.Containers[c].EnvFrom[e].ConfigMapRef.Name = %v, want %v", tt.pPod.Spec.InitContainers[c].EnvFrom[e].ConfigMapRef.Name, constants.TenantRootCACertConfigMapName)
					}
				}
			}
		})
	}
}
