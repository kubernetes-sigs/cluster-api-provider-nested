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
	"k8s.io/client-go/kubernetes"
	"math/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	uplugin "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/plugin"
)

const (
	// DefaultServiceAccountName is the name of the default service account to set on pods which do not specify a service account
	DefaultServiceAccountName = "default"

	// EnforceMountableSecretsAnnotation is a default annotation that indicates that a service account should enforce mountable secrets.
	// The value must be true to have this annotation take effect
	EnforceMountableSecretsAnnotation = "kubernetes.io/enforce-mountable-secrets"

	// ServiceAccountVolumeName is the prefix name that will be added to volumes that mount ServiceAccount secrets
	ServiceAccountVolumeName = "kube-api-access"

	// DefaultAPITokenMountPath is the path that ServiceAccountToken secrets are automounted to.
	// The token file would then be accessible at /var/run/secrets/kubernetes.io/serviceaccount
	DefaultAPITokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

	defaultAttemptTimes = 10
)

func init() {
	MutatorRegister.Register(&uplugin.Registration{
		ID: "00_PodRootCACertMutator",
		InitFn: func(ctx *uplugin.InitContext) (interface{}, error) {
			return NewMutatorPlugin(ctx)
		},
	})
}

type PodRootCACertMutatorPlugin struct {
	client               kubernetes.Interface
	secretLister         listersv1.SecretLister
	serviceAccountLister listersv1.ServiceAccountLister

	generateName func(string) string
}

func NewMutatorPlugin(ctx *uplugin.InitContext) (*PodRootCACertMutatorPlugin, error) {
	secretInformer := ctx.Informer.Core().V1().Secrets()
	serviceAccountInformer := ctx.Informer.Core().V1().ServiceAccounts()
	plugin := &PodRootCACertMutatorPlugin{
		client:               ctx.Client,
		secretLister:         secretInformer.Lister(),
		serviceAccountLister: serviceAccountInformer.Lister(),
	}
	return plugin, nil
}

// Mutator will automatically reassign configmap references for configmaps named
// kube-root-ca.crt in the pod spec, these places are
// * volumes
// * env
// * envFrom
func (pl *PodRootCACertMutatorPlugin) Mutator() conversion.PodMutator {
	return func(p *conversion.PodMutateCtx) error {
		if !featuregate.DefaultFeatureGate.Enabled(featuregate.RootCACertConfigMapSupport) {
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

		serviceAccount, err := pl.getServiceAccount(p.PPod.Namespace, p.PPod.Spec.ServiceAccountName)
		if err != nil {
			return fmt.Errorf("error looking up serviceAccount %s/%s: %v", p.PPod.Namespace, p.PPod.Spec.ServiceAccountName, err)
		}

		secret, err := pl.getSecret(p.ClusterName, p.PPod.Namespace, p.PPod.Spec.ServiceAccountName)
		if err != nil || secret == nil {
			klog.Errorf("not found serviceAccount %s token secret, err: %v", p.PPod.Spec.ServiceAccountName, err)
			return fmt.Errorf("error looking up secret of serviceAccount %s/%s: %v", p.PPod.Namespace, p.PPod.Spec.ServiceAccountName, err)
		}
		klog.V(6).Infof("mutate pod: %s/%s, found service account: %s, secret: %s", p.PPod.Namespace, p.PPod.Name, serviceAccount.Name, secret.Name)

		if shouldAutomount(serviceAccount, p.PPod) {
			pl.mountServiceAccountToken(secret, p.PPod)
			pl.mountKubeRootCAConfigMap(p.PPod)
		}
		return nil
	}
}

// getServiceAccount returns the ServiceAccount for the given namespace and name if it exists
func (p *PodRootCACertMutatorPlugin) getServiceAccount(namespace string, name string) (*corev1.ServiceAccount, error) {
	serviceAccount, err := p.serviceAccountLister.ServiceAccounts(namespace).Get(name)
	if err == nil {
		return serviceAccount, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Could not find in cache, attempt to look up directly
	numAttempts := 1
	if name == DefaultServiceAccountName {
		// If this is the default serviceaccount, attempt more times, since it should be auto-created by the controller
		numAttempts = defaultAttemptTimes
	}
	retryInterval := time.Duration(rand.Int63n(100)+int64(100)) * time.Millisecond
	for i := 0; i < numAttempts; i++ {
		if i != 0 {
			time.Sleep(retryInterval)
		}
		serviceAccount, err := p.client.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return serviceAccount, nil
		}
		if !errors.IsNotFound(err) {
			return nil, err
		}
	}

	return nil, errors.NewNotFound(corev1.Resource("serviceaccount"), name)
}

func (p *PodRootCACertMutatorPlugin) getSecret(cluster, namespace, serviceAccount string) (*corev1.Secret, error) {
	secrets, err := p.secretLister.Secrets(namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("error listing secret from super control plane informer cache: %v", err)
		return nil, err
	}
	for _, secret := range secrets {
		if secret.Type != corev1.SecretTypeOpaque {
			continue
		}
		annotations := secret.GetAnnotations()
		if annotations[constants.LabelCluster] != cluster || annotations[corev1.ServiceAccountNameKey] != serviceAccount {
			continue
		}
		return secret, nil
	}
	return nil, nil
}

func (p *PodRootCACertMutatorPlugin) mountServiceAccountToken(secret *corev1.Secret, pod *corev1.Pod) {
	// Determine a volume name for the ServiceAccountTokenSecret in case we need it
	tokenVolumeName := p.generateName(ServiceAccountVolumeName + "-")
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
					p.MutateAutoKubeApiAccessVolumeMounts(volume.Name, tokenVolumeName, pod)
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

// Mutator will automatically reassign configmap references for configmaps named
// kube-root-ca.crt in the pod spec, these places are
// * volumes
// * env
// * envFrom
func (p *PodRootCACertMutatorPlugin) mountKubeRootCAConfigMap(pod *corev1.Pod) {
	pod.Spec.Containers = p.containersMutator(pod.Spec.Containers)
	pod.Spec.InitContainers = p.containersMutator(pod.Spec.InitContainers)
	pod.Spec.Volumes = p.volumesMutator(pod.Spec.Volumes)
}

func (p *PodRootCACertMutatorPlugin) MutateAutoKubeApiAccessVolumeMounts(old, new string, pod *corev1.Pod) {
	for i, container := range pod.Spec.InitContainers {
		for j := 0; j < len(container.VolumeMounts); j++ {
			if container.VolumeMounts[j].Name == old {
				klog.V(6).Infof("mutate initContainer %s volumeMount.name from %s to %s", container.Name, old, new)
				pod.Spec.InitContainers[i].VolumeMounts[j].Name = new
				continue
			}
		}
	}

	for i, container := range pod.Spec.Containers {
		for j := 0; j < len(container.VolumeMounts); j++ {
			if container.VolumeMounts[j].Name == old {
				klog.V(6).Infof("mutate containers %s volumeMount.name from %s to %s", container.Name, old, new)
				pod.Spec.Containers[i].VolumeMounts[j].Name = new
				continue
			}
		}
	}
}

func (pl *PodRootCACertMutatorPlugin) containersMutator(containers []corev1.Container) []corev1.Container {
	for i, container := range containers {
		for j, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil &&
				env.ValueFrom.ConfigMapKeyRef.Name == constants.RootCACertConfigMapName {
				containers[i].Env[j].ValueFrom.ConfigMapKeyRef.Name = constants.TenantRootCACertConfigMapName
			}
		}

		for j, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == constants.RootCACertConfigMapName {
				containers[i].EnvFrom[j].ConfigMapRef.Name = constants.TenantRootCACertConfigMapName
			}
		}
	}
	return containers
}

func (pl *PodRootCACertMutatorPlugin) volumesMutator(volumes []corev1.Volume) []corev1.Volume {
	for i, volume := range volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name == constants.RootCACertConfigMapName {
			volumes[i].ConfigMap.Name = constants.TenantRootCACertConfigMapName
		}

		if volume.Projected != nil {
			for j, source := range volume.Projected.Sources {
				if source.ConfigMap != nil && source.ConfigMap.Name == constants.RootCACertConfigMapName {
					volumes[i].Projected.Sources[j].ConfigMap.Name = constants.TenantRootCACertConfigMapName
				}
			}
		}
	}
	return volumes
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
