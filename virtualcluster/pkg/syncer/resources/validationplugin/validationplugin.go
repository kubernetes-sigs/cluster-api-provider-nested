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

package validationplugin

import (
	"sync"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klog "k8s.io/klog/v2"
	mc "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/mccontroller"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Tenant struct {
	ClusterName string
	Cond        *sync.Mutex
}

type ValidationPlugin struct {
	name string
	client.Client
	alltenants map[string]Tenant
	*sync.RWMutex
	mc *mc.MultiClusterController
}

type ValidationPluginInterface interface {
	GetName() string
	Validation(client.Object, string) bool
	GetTenantLocker(string) *Tenant
	GetIdleTenantDuration() v1.Duration
	EnableValidationPlugin() bool
}

func New(name string, mccontroller *mc.MultiClusterController) *ValidationPlugin {
	return &ValidationPlugin{
		name:       name,
		alltenants: make(map[string]Tenant),
		RWMutex:    &sync.RWMutex{},
		mc:         mccontroller,
	}
}

func (q *ValidationPlugin) Validation(client.Object, string) bool {
	return true
}

func (q *ValidationPlugin) CleanIdleTenantInValidationPlugin(tenantCheckDuration v1.Duration) {
	//	tenantCheckDuration := q.GetIdleTenantDuration()
	m := make(map[string]bool)
	var diff []string
	for {
		time.Sleep(tenantCheckDuration.Duration)
		cnames := q.GetClusterNames()
		if cnames == nil {
			continue
		}
		for _, cn := range cnames {
			m[cn] = true
		}
		q.Lock()
		for k := range q.alltenants {
			if _, ok := m[k]; !ok {
				diff = append(diff, k)
			}
		}
		klog.Infof("delete %v tenant", len(diff))
		for _, k := range diff {
			delete(q.alltenants, k)
			klog.Infof("tenant %v removed from map", k)
		}
		q.Unlock()
	}
}

func (q *ValidationPlugin) GetTenantLocker(clusterName string) *Tenant {
	q.RLock()
	t, ok := q.alltenants[clusterName]
	if ok {
		q.RUnlock()
		return &t
	}
	q.RUnlock()

	q.Lock()
	defer q.Unlock()
	q.alltenants[clusterName] = Tenant{
		ClusterName: clusterName,
		Cond:        &sync.Mutex{},
	}
	if t, ok := q.alltenants[clusterName]; ok {
		klog.V(0).Infof("init lock for tenant %v and then locked", clusterName)
		return &t
	} else {
		klog.Errorf("cannot initialize lock for tenant %v", clusterName)
		return nil
	}
}

func (q *ValidationPlugin) GetClusterNames() []string {
	if q.mc == nil {
		klog.Errorf("mccontroller is nil.")
		return nil
	}
	return q.mc.GetClusterNames()
}

func (q *ValidationPlugin) GetCluster() *mc.MultiClusterController {
	return q.mc
}

func (q *ValidationPlugin) GetName() string {
	return q.name
}

func (q *ValidationPlugin) GetIdleTenantDuration() v1.Duration {
	return v1.Duration{Duration: 120 * time.Minute}
}

func (q *ValidationPlugin) EnableValidationPlugin() bool {
	return false
}
