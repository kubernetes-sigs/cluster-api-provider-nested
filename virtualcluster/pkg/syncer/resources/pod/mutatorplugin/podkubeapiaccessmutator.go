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
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	uplugin "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/plugin"
)

const (
	// DefaultServiceAccountName is the name of the default service account to set on pods which do not specify a service account
	DefaultServiceAccountName = "default" // #nosec G101

	// ServiceAccountVolumeName is the prefix name that will be added to volumes that mount ServiceAccount secrets
	ServiceAccountVolumeName = "kube-api-access" // #nosec G101

	// DefaultAPITokenMountPath is the path that ServiceAccountToken secrets are automounted to.
	// The token file would then be accessible at /var/run/secrets/kubernetes.io/serviceaccount
	DefaultAPITokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount" // #nosec G101

	defaultAttemptTimes = 10
)

func init() {
	MutatorRegister.Register(&uplugin.Registration{
		ID: "00_PodKubeAPIAccessMutator",
		InitFn: func(ctx *uplugin.InitContext) (interface{}, error) {
			return NewPodKubeAPIAccessMutatorPlugin(ctx)
		},
	})
}

type PodKubeAPIAccessMutatorPlugin struct {
	client       kubernetes.Interface
	generateName func(string) string
}

func NewPodKubeAPIAccessMutatorPlugin(ctx *uplugin.InitContext) (*PodKubeAPIAccessMutatorPlugin, error) {
	plugin := &PodKubeAPIAccessMutatorPlugin{
		client:       ctx.Client,
		generateName: names.SimpleNameGenerator.GenerateName,
	}
	return plugin, nil
}

func (pl *PodKubeAPIAccessMutatorPlugin) Mutator() conversion.PodMutator {
	return func(p *conversion.PodMutateCtx) error {
		if !featuregate.DefaultFeatureGate.Enabled(featuregate.KubeAPIAccessSupport) {
			return nil
		}

		// Don't modify the spec of mirror pods.
		// That makes the kubelet very angry and confused, and it immediately deletes the pod (because the spec doesn't match)
		// That said, don't allow mirror pods to reference ServiceAccounts or SecretVolumeSources either
		if _, isMirrorPod := p.PPod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
			return nil
		}

		// Set the default service account if needed
		if len(p.PPod.Spec.ServiceAccountName) == 0 {
			p.PPod.Spec.ServiceAccountName = DefaultServiceAccountName
		}

		targetNamespace := conversion.ToSuperClusterNamespace(p.ClusterName, p.PPod.Namespace)
		serviceAccount, err := pl.getServiceAccount(targetNamespace, p.PPod.Spec.ServiceAccountName)
		if err != nil {
			return fmt.Errorf("error looking up serviceAccount %s/%s: %v", targetNamespace, p.PPod.Spec.ServiceAccountName, err)
		}

		secret, err := pl.getSecret(p.ClusterName, targetNamespace, serviceAccount)
		if err != nil || secret == nil {
			klog.Errorf("not found serviceAccount %s/%s token secret, err: %v", targetNamespace, p.PPod.Spec.ServiceAccountName, err)
			return fmt.Errorf("error looking up secret of serviceAccount %s/%s: %v", targetNamespace, p.PPod.Spec.ServiceAccountName, err)
		}

		if shouldAutomount(serviceAccount, p.PPod) {
			pl.mountServiceAccountToken(secret, p.PPod)
		}
		return nil
	}
}

// getServiceAccount returns the ServiceAccount for the given namespace and name if it exists
func (pl *PodKubeAPIAccessMutatorPlugin) getServiceAccount(namespace string, name string) (*corev1.ServiceAccount, error) {
	serviceAccount, err := pl.client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return serviceAccount, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Could not find in cache, attempt to look up directly
	numAttempts := 1
	if name == DefaultServiceAccountName {
		// If this is the default serviceaccount, attempt more times, since it should be auto-created by the controller
		numAttempts = defaultAttemptTimes
	}
	retryInterval := time.Duration(rand.Int63n(100)+int64(100)) * time.Millisecond // #nosec G404
	for i := 0; i < numAttempts; i++ {
		serviceAccount, err := pl.client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return serviceAccount, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		time.Sleep(retryInterval)
	}

	return nil, apierrors.NewNotFound(corev1.Resource("serviceaccount"), name)
}

func (pl *PodKubeAPIAccessMutatorPlugin) getSecret(cluster, namespace string, sa *corev1.ServiceAccount) (*corev1.Secret, error) {
	secrets, err := pl.client.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("error listing secret from super control plane informer cache: %v", err)
		return nil, err
	}
	for _, secret := range secrets.Items {
		if secret.Type != corev1.SecretTypeOpaque {
			continue
		}
		annotations := secret.GetAnnotations()
		if annotations[constants.LabelCluster] != cluster || annotations[corev1.ServiceAccountNameKey] != sa.Name {
			continue
		}
		return &secret, nil
	}
	return nil, nil
}

func (pl *PodKubeAPIAccessMutatorPlugin) mountServiceAccountToken(secret *corev1.Secret, pod *corev1.Pod) {
	// Determine a volume name for the ServiceAccountTokenSecret in case we need it
	tokenVolumeName := pl.generateName(ServiceAccountVolumeName + "-")
	klog.V(4).Infof("generate a new VolumeMount.name: %s", tokenVolumeName)

	// Create the prototypical VolumeMount
	tokenVolumeMount := corev1.VolumeMount{
		Name:      tokenVolumeName,
		ReadOnly:  true,
		MountPath: DefaultAPITokenMountPath,
	}

	// Find the volume and volume name for the ServiceAccountTokenSecret if it already exists
	serviceAccountVolumeExist := false
	allVolumeNames := sets.NewString()
	for i, volume := range pod.Spec.Volumes {
		allVolumeNames.Insert(volume.Name)
		if strings.HasPrefix(volume.Name, ServiceAccountVolumeName+"-") {
			for _, source := range volume.Projected.Sources {
				if source.ServiceAccountToken != nil {
					klog.V(4).Infof("pod: %s/%s volume: %s mount service account token, mutate it!", pod.Namespace, pod.Name, volume.Name)
					pod.Spec.Volumes[i] = corev1.Volume{
						Name: tokenVolumeName,
						VolumeSource: corev1.VolumeSource{
							Projected: TokenVolumeSource(secret),
						},
					}
					pl.MutateAutoKubeAPIAccessVolumeMounts(volume.Name, tokenVolumeName, pod)
					serviceAccountVolumeExist = true
					break
				}
			}
			break
		}
	}

	// Ensure every container mounts the APISecret volume
	for i, container := range pod.Spec.InitContainers {
		existingContainerMount := false
		for _, volumeMount := range container.VolumeMounts {
			// Existing mounts at the default mount path prevent mounting of the API token
			if volumeMount.MountPath == DefaultAPITokenMountPath {
				existingContainerMount = true
				break
			}
		}
		if serviceAccountVolumeExist && !existingContainerMount {
			pod.Spec.InitContainers[i].VolumeMounts = append(pod.Spec.InitContainers[i].VolumeMounts, tokenVolumeMount)
		}
	}
	for i, container := range pod.Spec.Containers {
		existingContainerMount := false
		for _, volumeMount := range container.VolumeMounts {
			// Existing mounts at the default mount path prevent mounting of the API token
			if volumeMount.MountPath == DefaultAPITokenMountPath {
				existingContainerMount = true
				break
			}
		}
		if serviceAccountVolumeExist && !existingContainerMount {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, tokenVolumeMount)
		}
	}
}

func (pl *PodKubeAPIAccessMutatorPlugin) MutateAutoKubeAPIAccessVolumeMounts(old, new string, pod *corev1.Pod) {
	for i, container := range pod.Spec.InitContainers {
		for j := 0; j < len(container.VolumeMounts); j++ {
			if container.VolumeMounts[j].Name == old {
				klog.V(4).Infof("mutate initContainer %s volumeMount.name from %s to %s", container.Name, old, new)
				pod.Spec.InitContainers[i].VolumeMounts[j].Name = new
				continue
			}
		}
	}

	for i, container := range pod.Spec.Containers {
		for j := 0; j < len(container.VolumeMounts); j++ {
			if container.VolumeMounts[j].Name == old {
				klog.V(4).Infof("mutate containers %s volumeMount.name from %s to %s", container.Name, old, new)
				pod.Spec.Containers[i].VolumeMounts[j].Name = new
				continue
			}
		}
	}
}

func shouldAutomount(sa *corev1.ServiceAccount, pod *corev1.Pod) bool {
	// Fixme: Now we will always return true.
	// Perhaps in the future, some distinction between pods needs to be made here.
	return true
}

// TokenVolumeSource returns the projected volume source for service account token.
func TokenVolumeSource(secret *corev1.Secret) *corev1.ProjectedVolumeSource {
	return &corev1.ProjectedVolumeSource{
		Sources: []corev1.VolumeProjection{
			{
				Secret: &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
				},
			},
		},
	}
}
