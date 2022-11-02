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

package cluster

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/mccontroller"
)

// Cluster object represents a tenant control plane in Virtual Cluster architecture.
//
// Cluster implements the ClusterInterface used by MultiClusterController in
// sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/mccontroller/mccontroller.go.
//
// It stores a Kubernetes client, cache, and other cluster-scoped dependencies.
// The dependencies are lazily created in getters and cached for reuse.
// It is not thread safe.
type Cluster struct {
	// The key of this cluster. For vc, it is the root cluster namespace.
	key string
	// name of the corresponding cluster object.
	name string
	// namespace of the corresponding cluster object.
	namespace string
	// uid of the corresponding cluster object.
	uid string

	// Config is the rest.config used to talk to the apiserver.  Required.
	RestConfig *rest.Config

	// getter is used to get cluster CRD object.
	getter mccontroller.Getter

	// scheme is injected by the controllerManager when controllerManager.Start is called
	scheme *runtime.Scheme // nolint:structcheck

	mapper meta.RESTMapper

	// informer cache and delegating client for watched tenant control plane objects
	cache            cache.Cache
	delegatingClient client.Client

	// a clientset client for unwatched tenant control plane objects (rw directly to tenant apiserver)
	client *clientset.Clientset

	options Options

	// a flag indicates that the cluster cache has been synced
	synced bool

	cancelContext context.CancelFunc

	context context.Context
}

// Options are the arguments for creating a new Cluster.
type Options struct {
	CacheOptions
	// RequestTimeout is the rest client request timeout.
	// Set this to something reasonable so request to apiserver don't hang forever.
	RequestTimeout time.Duration
}

// CacheOptions is embedded in Options to configure the new Cluster's cache.
type CacheOptions struct {
	// Resync is the period between cache resyncs.
	// A cache resync triggers event handlers for each object watched by the cache.
	// It can be useful if your level-based logic isn't perfect.
	Resync *time.Duration
	// WatchNamespace can be used to watch only a single namespace.
	// If unset (Namespace == ""), all namespaces are watched.
	WatchNamespace string
}

var _ mccontroller.ClusterInterface = &Cluster{}

func NewCluster(key, namespace, name, uid string, getter mccontroller.Getter, configBytes []byte, o Options) (*Cluster, error) {
	clusterRestConfig, err := clientcmd.RESTConfigFromKubeConfig(configBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to build rest config: %v", err)
	}

	if o.RequestTimeout == 0 {
		clusterRestConfig.Timeout = constants.DefaultRequestTimeout
	}

	if clusterRestConfig.QPS == 0 {
		clusterRestConfig.QPS = constants.DefaultSyncerClientQPS
	}
	if clusterRestConfig.Burst == 0 {
		clusterRestConfig.Burst = constants.DefaultSyncerClientBurst
	}

	return &Cluster{
		key:           key,
		name:          name,
		namespace:     namespace,
		uid:           uid,
		getter:        getter,
		RestConfig:    clusterRestConfig,
		options:       o,
		synced:        false,
		context:       context.Background(),
		cancelContext: func() {},
	}, nil
}

// GetClusterName returns the unique cluster name, aka, the root namespace name.
func (c *Cluster) GetClusterName() string {
	return c.key
}

func (c *Cluster) GetOwnerInfo() (string, string, string) {
	return c.name, c.namespace, c.uid
}

// GetObject returns the cluster object.
func (c *Cluster) GetObject() (client.Object, error) {
	obj, err := c.getter.GetObject(c.namespace, c.name)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *Cluster) getScheme() *runtime.Scheme {
	return scheme.Scheme
}

// GetClientSet returns a clientset client without any informer caches. All client requests go to apiserver directly.
func (c *Cluster) GetClientSet() (clientset.Interface, error) {
	if c.client != nil {
		return c.client, nil
	}
	var err error
	c.client, err = clientset.NewForConfig(rest.AddUserAgent(c.RestConfig, constants.ResourceSyncerUserAgent))
	if err != nil {
		return nil, err
	}
	return c.client, nil
}

// getMapper returns a lazily created apimachinery RESTMapper.
func (c *Cluster) getMapper() (meta.RESTMapper, error) {
	if c.mapper != nil {
		return c.mapper, nil
	}

	mapper, err := apiutil.NewDynamicRESTMapper(c.RestConfig)
	if err != nil {
		return nil, err
	}

	c.mapper = mapper
	return mapper, nil
}

// getCache returns a lazily created controller-runtime Cache.
func (c *Cluster) getCache() (cache.Cache, error) {
	if c.cache != nil {
		return c.cache, nil
	}

	m, err := c.getMapper()
	if err != nil {
		return nil, err
	}

	ca, err := cache.New(c.RestConfig, cache.Options{
		Scheme:    c.getScheme(),
		Mapper:    m,
		Resync:    c.options.Resync,
		Namespace: c.options.WatchNamespace,
	})
	if err != nil {
		return nil, err
	}

	c.cache = ca
	return ca, nil
}

// GetDelegatingClient returns a lazily created controller-runtime DelegatingClient.
// It is used by other Cluster getters, and by reconcilers.
// TODO: consider implementing Reader, Writer and StatusClient in Cluster
// and forwarding to actual delegating client.
func (c *Cluster) GetDelegatingClient() (client.Client, error) {
	if !c.synced {
		return nil, fmt.Errorf("the client cache has not been synced yet")
	}

	if c.delegatingClient != nil {
		return c.delegatingClient, nil
	}

	ca, err := c.getCache()
	if err != nil {
		return nil, err
	}

	m, err := c.getMapper()
	if err != nil {
		return nil, err
	}

	cl, err := client.New(c.RestConfig, client.Options{
		Scheme: c.getScheme(),
		Mapper: m,
	})
	if err != nil {
		return nil, err
	}

	dc, err := client.NewDelegatingClient(client.NewDelegatingClientInput{
		Client:      cl,
		CacheReader: ca,
	})
	if err != nil {
		return nil, err
	}

	c.delegatingClient = dc
	return dc, nil
}

// GetRestConfig returns restful configuration of virtual cluster client
func (c *Cluster) GetRestConfig() *rest.Config {
	return c.RestConfig
}

// AddEventHandler instructs the Cluster's cache to watch objectType's resource,
// if it doesn't already, and to add handler as an event handler.
func (c *Cluster) AddEventHandler(objectType client.Object, handler clientgocache.ResourceEventHandler) error {
	ca, err := c.getCache()
	if err != nil {
		return err
	}

	i, err := ca.GetInformer(context.TODO(), objectType)
	if err != nil {
		return err
	}

	i.AddEventHandler(handler)
	return nil
}

// GetInformer fetches or constructs an informer for the given object that corresponds to a single
// API kind and resource.
func (c *Cluster) GetInformer(objectType client.Object) (cache.Informer, error) {
	ca, err := c.getCache()
	if err != nil {
		return nil, err
	}

	i, err := ca.GetInformer(context.TODO(), objectType)
	if err != nil {
		return nil, err
	}

	return i, nil
}

// Start starts the Cluster's cache and blocks,
// until context for the cache is cancelled.
func (c *Cluster) Start() error {
	ctx, cancel := context.WithCancel(c.context)
	c.cancelContext = cancel

	ca, err := c.getCache()
	if err != nil {
		return err
	}
	return ca.Start(ctx)
}

// WaitForCacheSync waits for the Cluster's cache to sync,
// OR until an empty struct is sent to the stop channel.
func (c *Cluster) WaitForCacheSync() bool {
	ca, err := c.getCache()
	if err != nil {
		klog.Errorf("Fail to get cache: %v", err)
		return false
	}
	return ca.WaitForCacheSync(c.context)
}

func (c *Cluster) SetSynced() {
	c.synced = true
}

func (c *Cluster) SetKey(k string) {
	c.key = k
}

// Stop cancel/close the cache to terminate informers.
func (c *Cluster) Stop() {
	if c.cancelContext == nil {
		return
	}
	c.cancelContext()
}
