/*
Copyright 2019 The Kubernetes Authors.
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

package endpoints

import (
	"fmt"
	"sync/atomic"

	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/metrics"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/patrol/differ"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util"
)

var numMissingEndPoints uint64
var numMissMatchedEndPoints uint64

func (c *controller) StartPatrol(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	if !cache.WaitForCacheSync(stopCh, c.endpointsSynced) {
		return fmt.Errorf("failed to wait for caches to sync before starting Endpoint checker")
	}
	c.Patroller.Start(stopCh)
	return nil
}

// PatrollerDo checks to see if Endpoints in super control plane informer cache and tenant control plane
// keep consistency.
// Note that eps are managed by tenant/super ep controller separately. The checker will not do GC but only report diff.
func (c *controller) PatrollerDo() {
	clusterNames := c.MultiClusterController.GetClusterNames()
	if len(clusterNames) == 0 {
		klog.V(5).Infof("super cluster has no tenant control planes, giving up periodic checker: %s", "endpoint")
		return
	}

	numMissingEndPoints = 0
	numMissMatchedEndPoints = 0

	pList, err := c.endpointsLister.List(util.GetSuperClusterListerLabelsSelector())
	if err != nil {
		klog.Errorf("error listing endpoints from super control plane informer cache: %v", err)
		return
	}
	pSet := differ.NewDiffSet()
	for _, p := range pList {
		pSet.Insert(differ.ClusterObject{Object: p, Key: differ.DefaultClusterObjectKey(p, "")})
	}

	knownClusterSet := sets.NewString(clusterNames...)
	vSet := differ.NewDiffSet()
	for _, cluster := range clusterNames {
		vList := &v1.EndpointsList{}
		if err := c.MultiClusterController.List(cluster, vList); err != nil {
			klog.Errorf("error listing endpoints from cluster %s informer cache: %v", cluster, err)
			knownClusterSet.Delete(cluster)
			continue
		}

		for i := range vList.Items {
			vSet.Insert(differ.ClusterObject{
				Object:       &vList.Items[i],
				OwnerCluster: cluster,
				Key:          differ.DefaultClusterObjectKey(&vList.Items[i], cluster),
			})
		}
	}

	d := differ.HandlerFuncs{}
	d.AddFunc = func(vObj differ.ClusterObject) {
		atomic.AddUint64(&numMissingEndPoints, 1)
		if err := c.MultiClusterController.RequeueObject(vObj.OwnerCluster, vObj); err != nil {
			klog.Errorf("error requeue vEndpoints %s: %v", vObj.Key, err)
		} else {
			metrics.CheckerRemedyStats.WithLabelValues("RequeuedTenantEndpoints").Inc()
		}
	}
	d.UpdateFunc = func(vObj, pObj differ.ClusterObject) {
		v := vObj.Object.(*v1.Endpoints)
		p := pObj.Object.(*v1.Endpoints)
		updated := conversion.Equality(c.Config, nil).CheckEndpointsEquality(p, v)
		if updated != nil {
			atomic.AddUint64(&numMissMatchedEndPoints, 1)
			if err := c.MultiClusterController.RequeueObject(vObj.OwnerCluster, vObj); err != nil {
				klog.Errorf("error requeue vEndpoints %s: %v", vObj.Key, err)
			} else {
				metrics.CheckerRemedyStats.WithLabelValues("RequeuedTenantEndpoints").Inc()
			}
		}
	}

	vSet.Difference(pSet, differ.FilteringHandler{
		Handler:    d,
		FilterFunc: differ.DefaultDifferFilter(knownClusterSet),
	})

	metrics.CheckerMissMatchStats.WithLabelValues("MissingEndPoints").Set(float64(numMissingEndPoints))
	metrics.CheckerMissMatchStats.WithLabelValues("MissMatchedEndPoints").Set(float64(numMissMatchedEndPoints))
}
