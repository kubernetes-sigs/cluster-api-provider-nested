/*
Copyright 2021 The Kubernetes Authors.

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

package crd

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/conversion"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/errors"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
)

func (c *controller) StartUWS(stopCh <-chan struct{}) error {
	if c.crdcache != nil {
		go func() {
			_ = c.crdcache.Start(context.Background())
		}()
	} else {
		klog.Errorf("crd cache is nil")
	}

	if !cache.WaitForCacheSync(stopCh, c.crdSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	return c.UpwardController.Start(stopCh)
}

func (c *controller) BackPopulate(key string) error {
	// The key format is clustername/pcName.
	clusterName, crdName, _ := cache.SplitMetaNamespaceKey(key)
	op := reconciler.AddEvent
	pCRD := &apiextensionsv1.CustomResourceDefinition{}
	err := c.superClient.Get(context.TODO(), client.ObjectKey{
		Name: crdName,
	}, pCRD)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		op = reconciler.DeleteEvent
	}

	cluster := c.MultiClusterController.GetCluster(clusterName)
	if cluster == nil {
		return errors.NewClusterNotFound(clusterName)
	}

	vcrestconfig := cluster.GetRestConfig()
	var vcapiextensionsClient apiextensionclientset.CustomResourceDefinitionsGetter

	if vcrestconfig == nil {
		return fmt.Errorf("cannot get virtual cluster restful config")
	}
	vcc, err := apiextensionsclientset.NewForConfig(vcrestconfig)
	if err != nil {
		return err
	}
	vcapiextensionsClient = vcc.ApiextensionsV1()

	vCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := c.MultiClusterController.Get(clusterName, "", crdName, vCRD); err != nil {
		if apierrors.IsNotFound(err) {
			if op == reconciler.AddEvent {
				// Available in super, hence create a new in tenant control plane
				vCRD := conversion.BuildVirtualCRD(clusterName, pCRD)
				_, err = vcapiextensionsClient.CustomResourceDefinitions().Create(context.TODO(), vCRD, metav1.CreateOptions{})
				if err != nil {
					return err
				}
			}
			return nil
		}
		klog.Errorf("cannot obtain virtual cluster object")
		return err
	}

	if op == reconciler.DeleteEvent {
		opts := &metav1.DeleteOptions{
			PropagationPolicy: &constants.DefaultDeletionPolicy,
		}
		err = vcapiextensionsClient.CustomResourceDefinitions().Delete(context.TODO(), crdName, *opts)
		if err != nil {
			klog.Errorf("cannot delete with err=%v", err)
			return err
		}
	} else {
		updatedCRD := conversion.Equality(c.Config, nil).CheckCRDEquality(pCRD, vCRD)
		if updatedCRD != nil {
			_, err = vcapiextensionsClient.CustomResourceDefinitions().Update(context.TODO(), updatedCRD, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
