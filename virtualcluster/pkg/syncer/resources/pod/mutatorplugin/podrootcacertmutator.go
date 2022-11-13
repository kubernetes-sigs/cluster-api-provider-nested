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

		for i := range p.PPod.Spec.Volumes {
			if p.PPod.Spec.Volumes[i].ConfigMap != nil && p.PPod.Spec.Volumes[i].ConfigMap.Name == constants.RootCACertConfigMapName {
				p.PPod.Spec.Volumes[i].ConfigMap.Name = constants.TenantRootCACertConfigMapName
			}
		}

		for c := range p.PPod.Spec.InitContainers {
			for e := range p.PPod.Spec.InitContainers[c].Env {
				if p.PPod.Spec.InitContainers[c].Env[e].ValueFrom != nil &&
					p.PPod.Spec.InitContainers[c].Env[e].ValueFrom.ConfigMapKeyRef != nil &&
					p.PPod.Spec.InitContainers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name == constants.RootCACertConfigMapName {
					p.PPod.Spec.InitContainers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name = constants.TenantRootCACertConfigMapName
				}
			}

			for e := range p.PPod.Spec.InitContainers[c].EnvFrom {
				if p.PPod.Spec.InitContainers[c].EnvFrom[e].ConfigMapRef != nil &&
					p.PPod.Spec.InitContainers[c].EnvFrom[e].ConfigMapRef.Name == constants.RootCACertConfigMapName {
					p.PPod.Spec.InitContainers[c].EnvFrom[e].ConfigMapRef.Name = constants.TenantRootCACertConfigMapName
				}
			}
		}

		for c := range p.PPod.Spec.Containers {
			for e := range p.PPod.Spec.Containers[c].Env {
				if p.PPod.Spec.Containers[c].Env[e].ValueFrom != nil &&
					p.PPod.Spec.Containers[c].Env[e].ValueFrom.ConfigMapKeyRef != nil &&
					p.PPod.Spec.Containers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name == constants.RootCACertConfigMapName {
					p.PPod.Spec.Containers[c].Env[e].ValueFrom.ConfigMapKeyRef.Name = constants.TenantRootCACertConfigMapName
				}
			}

			for e := range p.PPod.Spec.Containers[c].EnvFrom {
				if p.PPod.Spec.Containers[c].EnvFrom[e].ConfigMapRef != nil &&
					p.PPod.Spec.Containers[c].EnvFrom[e].ConfigMapRef.Name == constants.RootCACertConfigMapName {
					p.PPod.Spec.Containers[c].EnvFrom[e].ConfigMapRef.Name = constants.TenantRootCACertConfigMapName
				}
			}
		}
		return nil
	}
}
