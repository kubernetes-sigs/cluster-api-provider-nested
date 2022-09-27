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
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/apis/config"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	uplugin "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/plugin"
)

func init() {
	MutatorRegister.Register(&uplugin.Registration{
		ID: "PodMountServiceAccountTokenMutator",
		InitFn: func(ctx *uplugin.InitContext) (interface{}, error) {
			return &PodMountServiceAccountTokenMutatorPlugin{disable: ctx.Config.(*config.SyncerConfiguration).DisableServiceAccountToken}, nil
		},
	})
}

type PodMountServiceAccountTokenMutatorPlugin struct {
	disable bool
}

func (pl *PodMountServiceAccountTokenMutatorPlugin) Mutator() conversion.PodMutator {
	return func(p *conversion.PodMutateCtx) error {
		if pl.disable {
			p.PPod.Spec.AutomountServiceAccountToken = pointer.BoolPtr(false)
		}
		return nil
	}
}
