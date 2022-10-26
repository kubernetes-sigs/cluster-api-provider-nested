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

package persistentvolumeclaim

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"

	vcclient "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/client/clientset/versioned"
	vcinformers "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/client/informers/externalversions/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/apis/config"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/manager"
	pa "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/patrol"
	uw "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/uwcontroller"
	mc "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/mccontroller"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/plugin"
)

func init() {
	plugin.SyncerResourceRegister.Register(&plugin.Registration{
		ID: "persistentvolumeclaim",
		InitFn: func(ctx *plugin.InitContext) (interface{}, error) {
			return NewPVCController(ctx.Config.(*config.SyncerConfiguration), ctx.Client, ctx.Informer, ctx.VCClient, ctx.VCInformer, manager.ResourceSyncerOptions{})
		},
	})
}

type controller struct {
	manager.BaseResourceSyncer
	// super control plane pvc client
	pvcClient v1core.PersistentVolumeClaimsGetter
	// super control plane pvc lister
	pvcLister listersv1.PersistentVolumeClaimLister
	pvcSynced cache.InformerSynced
	informer  coreinformers.Interface
}

func NewPVCController(config *config.SyncerConfiguration,
	client clientset.Interface,
	informer informers.SharedInformerFactory,
	vcClient vcclient.Interface,
	vcInformer vcinformers.VirtualClusterInformer,
	options manager.ResourceSyncerOptions) (manager.ResourceSyncer, error) {
	c := &controller{
		BaseResourceSyncer: manager.BaseResourceSyncer{
			Config: config,
		},
		pvcClient: client.CoreV1(),
		informer:  informer.Core().V1(),
	}

	var err error
	c.MultiClusterController, err = mc.NewMCController(&corev1.PersistentVolumeClaim{}, &corev1.PersistentVolumeClaimList{}, c, mc.WithOptions(options.MCOptions))
	if err != nil {
		return nil, err
	}

	c.pvcLister = informer.Core().V1().PersistentVolumeClaims().Lister()
	if options.IsFake {
		c.pvcSynced = func() bool { return true }
	} else {
		c.pvcSynced = informer.Core().V1().PersistentVolumeClaims().Informer().HasSynced
	}

	c.UpwardController, err = uw.NewUWController(&corev1.PersistentVolumeClaim{}, c, uw.WithOptions(options.UWOptions))
	if err != nil {
		return nil, err
	}

	c.Patroller, err = pa.NewPatroller(&corev1.PersistentVolumeClaim{}, c, pa.WithOptions(options.PatrolOptions))
	if err != nil {
		return nil, err
	}

	c.informer.PersistentVolumeClaims().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				pvc := newObj.(*corev1.PersistentVolumeClaim)
				c.enqueuePersistentVolumeClaim(pvc)
			},
		},
	)
	return c, nil
}

func (c *controller) enqueuePersistentVolumeClaim(obj interface{}) {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return
	}

	clusterName, _ := conversion.GetVirtualOwner(pvc)
	if clusterName == "" {
		return
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %v: %v", obj, err))
		return
	}

	klog.V(4).Infof("enqueue PersistentVolumeClaim %s", key)
	c.UpwardController.AddToQueue(key)
}
