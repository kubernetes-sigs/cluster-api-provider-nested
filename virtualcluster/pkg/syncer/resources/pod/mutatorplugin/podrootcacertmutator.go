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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	uplugin "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/plugin"
)

func init() {
	MutatorRegister.Register(&uplugin.Registration{
		ID: "00_PodRootCACertMutator",
		InitFn: func(ctx *uplugin.InitContext) (interface{}, error) {
			return &PodRootCACertMutatorPlugin{}, nil
		},
	})
}

type PodRootCACertMutatorPlugin struct{}

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

		p.PPod.Spec.Containers = pl.containersMutator(p.PPod.Spec.Containers)
		p.PPod.Spec.InitContainers = pl.containersMutator(p.PPod.Spec.InitContainers)
		p.PPod.Spec.Volumes = pl.volumesMutator(p.PPod.Spec.Volumes)
		return nil
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
