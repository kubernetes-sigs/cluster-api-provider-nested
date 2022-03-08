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

package plugin

import (
	"sync"

	mc "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/mccontroller"

	validationplugin "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/resources/validationplugin"
)

// Registration contains information for registering a plugin
type Registration struct {
	// ID of the plugin
	ID string
	// InitFn is called when initializing a plugin. The registration and
	// context are passed in.
	InitFn func(*mc.MultiClusterController, bool) (validationplugin.ValidationPluginInterface, error)
	// Disable the plugin from loading
	Disable bool
}

// Init the registered plugin
func (r *Registration) Init(mccontroller *mc.MultiClusterController, isFake bool) *Plugin {
	p, err := r.InitFn(mccontroller, isFake)
	return &Plugin{
		Registration: r,
		instance:     p,
		err:          err,
	}
}

// Plugin represents an initialized plugin, used with an init context.
type Plugin struct {
	Registration *Registration // registration, as initialized
	instance     validationplugin.ValidationPluginInterface
	err          error // will be set if there was an error initializing the plugin
}

// Instance returns the instance and any initialization error of the plugin
func (p *Plugin) Instance() (validationplugin.ValidationPluginInterface, error) {
	return p.instance, p.err
}

type FeatureRegister struct {
	sync.RWMutex
	feature *Registration
}

// Register allows plugins to register (put)
func (reg *FeatureRegister) Register(r *Registration) {
	reg.Lock()
	defer reg.Unlock()
	if reg.feature == nil {
		reg.feature = r
	}
}

// Get returns the list of registered plugins for initialization. (get)
func (reg *FeatureRegister) Get() *Registration {
	reg.RLock()
	defer reg.RUnlock()
	return reg.feature
}

var DefaultFeatureRegister FeatureRegister
