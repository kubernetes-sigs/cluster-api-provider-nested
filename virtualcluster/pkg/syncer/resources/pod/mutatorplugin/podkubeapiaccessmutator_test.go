/*
Copyright 2023 The Kubernetes Authors.

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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	util "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/test"
	uplugin "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/plugin"
)

func tenantKubeAPIAccessPod(name, namespace, uid string) *corev1.Pod {
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
							Name:      "kube-api-access-l945t",
							MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
							ReadOnly:  true,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Image: "busybox",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "kube-api-access-l945t",
							MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "kube-api-access-l945t",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
										Path: "token",
									},
									ConfigMap: &corev1.ConfigMapProjection{
										Items: []corev1.KeyToPath{
											{
												Key:  "ca.crt",
												Path: "ca.crt",
											},
										},
										LocalObjectReference: corev1.LocalObjectReference{Name: constants.RootCACertConfigMapName},
									},
									DownwardAPI: &corev1.DownwardAPIProjection{
										Items: []corev1.DownwardAPIVolumeFile{
											{
												Path: "namespace",
												FieldRef: &corev1.ObjectFieldSelector{
													APIVersion: "v1",
													FieldPath:  "metadata.namespace",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return p
}

func TestPodKubeAPIAccessMutatorPlugin_Mutator(t *testing.T) {
	defer util.SetFeatureGateDuringTest(t, featuregate.DefaultFeatureGate, featuregate.KubeAPIAccessSupport, true)()

	tests := []struct {
		name                  string
		pPod                  *corev1.Pod
		existingObjectInSuper []runtime.Object
	}{
		{
			"Test RootCACert Mutator",
			tenantKubeAPIAccessPod("test", "default", "123-456-789"),
			[]runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default-token-pkm85",
						Namespace: "cluster1-default",
						Annotations: map[string]string{
							constants.LabelCluster:       "cluster1",
							corev1.ServiceAccountNameKey: "default",
							corev1.ServiceAccountUIDKey:  "7374a172-c35d-45b1-9c8e-bf5c5b614937",
						},
					},
					Type: corev1.SecretTypeOpaque,
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: "cluster1-default",
						UID:       "7374a172-c35d-45b1-9c8e-bf5c5b614937",
					},
					Secrets: []corev1.ObjectReference{
						{
							Name: "default-token-pkm85",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(tt.existingObjectInSuper...)
			informer := informers.NewSharedInformerFactory(client, 0)
			ctx := &uplugin.InitContext{
				Client:   client,
				Informer: informer,
			}
			pl, err := NewPodKubeAPIAccessMutatorPlugin(ctx)
			if err != nil {
				t.Errorf("mutator failed processing the pod")
			}
			mutator := pl.Mutator()

			if err := mutator(&conversion.PodMutateCtx{ClusterName: "cluster1", PPod: tt.pPod}); err != nil {
				t.Errorf("mutator failed processing the pod, %v", err)
			}

			kubeAPIAccessVolume := ""
			for _, volume := range tt.pPod.Spec.Volumes {
				if strings.HasPrefix(volume.Name, ServiceAccountVolumeName) && volume.Projected != nil {
					kubeAPIAccessVolume = volume.Name
					for _, source := range volume.Projected.Sources {
						if source.Secret == nil {
							t.Errorf("tt.pPod.Spec.Volumes[*].Name = %v, want to mount secret, but nil", volume.Name)
						}
					}
				}
			}
			for c := range tt.pPod.Spec.Containers {
				found := false
				for e := range tt.pPod.Spec.Containers[c].VolumeMounts {
					if tt.pPod.Spec.Containers[c].VolumeMounts[e].Name == kubeAPIAccessVolume {
						found = true
					}
				}
				if !found {
					t.Errorf("tt.pPod.Spec.Containers[c].VolumeMounts[e] want to mount %s, but not found", kubeAPIAccessVolume)
				}
			}

			for c := range tt.pPod.Spec.InitContainers {
				found := false
				for e := range tt.pPod.Spec.InitContainers[c].VolumeMounts {
					if tt.pPod.Spec.InitContainers[c].VolumeMounts[e].Name == kubeAPIAccessVolume {
						found = true
					}
				}
				if !found {
					t.Errorf("tt.pPod.Spec.InitContainers[c].VolumeMounts[e] want to mount %s, but not found", kubeAPIAccessVolume)
				}
			}
		})
	}
}
