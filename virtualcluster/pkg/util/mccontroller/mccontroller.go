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

package mccontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/metrics"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/scheme"
	utilconstants "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/errors"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/fairqueue"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/handler"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/util/record"
)

// Cache is the interface used by Controller to start and wait for caches to sync.
type Cache interface {
	Start() error
	WaitForCacheSync() bool
	Stop()
}

// Getter interface is used to get the CRD object that abstracts the cluster.
type Getter interface {
	GetObject(string, string) (client.Object, error)
}

// ClusterInterface decouples the controller package from the cluster package.
type ClusterInterface interface {
	GetClusterName() string
	GetOwnerInfo() (string, string, string)
	GetObject() (client.Object, error)
	AddEventHandler(client.Object, clientgocache.ResourceEventHandler) error
	GetInformer(objectType client.Object) (cache.Informer, error)
	GetClientSet() (clientset.Interface, error)
	GetDelegatingClient() (client.Client, error)
	GetRestConfig() *rest.Config
	Cache
}

type MultiClusterInterface interface {
	WatchClusterResource(cluster ClusterInterface, o WatchOptions) error
	RegisterClusterResource(cluster ClusterInterface, o WatchOptions) error
	TeardownClusterResource(cluster ClusterInterface)
	GetControllerName() string
	GetObjectKind() string
	Get(clusterName, namespace, name string, obj client.Object) error
	List(clusterName string, instanceList client.ObjectList, opts ...client.ListOption) error
	GetCluster(clusterName string) ClusterInterface
	GetClusterClient(clusterName string) (clientset.Interface, error)
	GetClusterObject(clusterName string) (client.Object, error)
	GetOwnerInfo(clusterName string) (string, string, string, error)
	GetClusterNames() []string
}

// MultiClusterController implements the multicluster controller pattern.
// A MultiClusterController owns a client-go workqueue. The WatchClusterResource methods set
// up the queue to receive reconcile requests, e.g., CRUD events from a tenant cluster.
// The Requests are processed by the user-provided Reconciler.
// MultiClusterController saves all watched tenant clusters in a set.
type MultiClusterController struct {
	sync.Mutex

	MultiClusterInterface

	// objectType is the type of object to watch.  e.g. &corev1.Pod{}
	objectType client.Object

	// objectKind is the kind of target object this controller watched.
	objectKind string

	// clusters is the internal cluster set this controller watches.
	clusters map[string]ClusterInterface

	Options
}

// Options are the arguments for creating a new Controller.
type Options struct {
	// JitterPeriod is the time to wait after an error to start working again.
	JitterPeriod time.Duration

	// MaxConcurrentReconciles is the number of concurrent control loops.
	MaxConcurrentReconciles int

	Reconciler reconciler.DWReconciler

	// Queue can be used to override the default queue.
	Queue workqueue.RateLimitingInterface

	// name is used to uniquely identify a Controller in tracing, logging and monitoring.  Name is required.
	name string
}

// NewMCController creates a new MultiClusterController.
func NewMCController(objectType client.Object, objectListType client.ObjectList, rc reconciler.DWReconciler, opts ...OptConfig) (*MultiClusterController, error) {
	kinds, _, err := scheme.Scheme.ObjectKinds(objectType)
	if err != nil || len(kinds) == 0 {
		return nil, fmt.Errorf("mccontroller: unknown object kind %+v", objectType)
	}

	c := &MultiClusterController{
		objectType: objectType,
		objectKind: kinds[0].Kind,
		clusters:   make(map[string]ClusterInterface),
		Options: Options{
			name:                    fmt.Sprintf("%s-mccontroller", strings.ToLower(kinds[0].Kind)),
			JitterPeriod:            1 * time.Second,
			MaxConcurrentReconciles: constants.DwsControllerWorkerLow,
			Reconciler:              rc,
			Queue:                   fairqueue.NewRateLimitingFairQueue(),
		},
	}

	for _, opt := range opts {
		opt(&c.Options)
	}

	if c.Reconciler == nil {
		return nil, fmt.Errorf("mccontroller %q: must specify DW Reconciler", c.objectKind)
	}

	return c, nil
}

// WatchOptions is used as an argument of WatchResource methods (just a placeholder for now).
// TODO: consider implementing predicates.
type WatchOptions struct {
	AttachUID bool // the object UID will be added to the reconciler request if it is true
}

// WatchClusterResource configures the Controller to watch resources of the same Kind as objectType,
// in the specified cluster, generating reconcile Requests from the ClusterInterface's context
// and the watched objects' namespaces and names.
func (c *MultiClusterController) WatchClusterResource(cluster ClusterInterface, o WatchOptions) error {
	c.Lock()
	defer c.Unlock()
	if _, exist := c.clusters[cluster.GetClusterName()]; !exist {
		return fmt.Errorf("please register cluster %s resource before watch", cluster.GetClusterName())
	}

	if c.objectType == nil {
		return nil
	}

	h := &handler.EnqueueRequestForObject{ClusterName: cluster.GetClusterName(), Queue: c.Queue, AttachUID: o.AttachUID}
	return cluster.AddEventHandler(c.objectType, h)
}

// RegisterClusterResource get the informer *before* trying to wait for the
// caches to sync so that we have a chance to register their intended caches.
func (c *MultiClusterController) RegisterClusterResource(cluster ClusterInterface, o WatchOptions) error {
	c.Lock()
	defer c.Unlock()
	if _, exist := c.clusters[cluster.GetClusterName()]; exist {
		return nil
	}
	c.clusters[cluster.GetClusterName()] = cluster

	if c.objectType == nil {
		return nil
	}

	_, err := cluster.GetInformer(c.objectType)
	return err
}

// TeardownClusterResource forget the cluster it watches.
// The cluster informer should stop together.
func (c *MultiClusterController) TeardownClusterResource(cluster ClusterInterface) {
	c.Lock()
	defer c.Unlock()
	delete(c.clusters, cluster.GetClusterName())
}

// Start starts the ClustersController's control loops (as many as MaxConcurrentReconciles) in separate channels
// and blocks until an empty struct is sent to the stop channel.
func (c *MultiClusterController) Start(stop <-chan struct{}) error {
	klog.Infof("start mc-controller %q", c.name)

	defer c.Queue.ShutDown()

	for i := 0; i < c.MaxConcurrentReconciles; i++ {
		go wait.Until(c.worker, c.JitterPeriod, stop)
	}

	<-stop
	return nil
}

// GetControllerName get the mccontroller name, is used to uniquely identify the Controller in tracing, logging and monitoring.
func (c *MultiClusterController) GetControllerName() string {
	return c.name
}

// GetObjectKind is the objectKind name this controller watch to.
func (c *MultiClusterController) GetObjectKind() string {
	return c.objectKind
}

// Get returns object with specific cluster, namespace and name.
func (c *MultiClusterController) Get(clusterName, namespace, name string, obj client.Object) error {
	cluster := c.GetCluster(clusterName)
	if cluster == nil {
		return errors.NewClusterNotFound(clusterName)
	}
	delegatingClient, err := cluster.GetDelegatingClient()
	if err != nil {
		return err
	}
	return delegatingClient.Get(context.TODO(), client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, obj)
}

// List returns a list of objects with specific cluster.
func (c *MultiClusterController) List(clusterName string, instanceList client.ObjectList, opts ...client.ListOption) error {
	cluster := c.GetCluster(clusterName)
	if cluster == nil {
		return errors.NewClusterNotFound(clusterName)
	}

	delegatingClient, err := cluster.GetDelegatingClient()
	if err != nil {
		return err
	}

	return delegatingClient.List(context.TODO(), instanceList, opts...)
}

func (c *MultiClusterController) GetCluster(clusterName string) ClusterInterface {
	c.Lock()
	defer c.Unlock()
	return c.clusters[clusterName]
}

// GetClusterClient returns the cluster's clientset client for direct access to tenant apiserver
func (c *MultiClusterController) GetClusterClient(clusterName string) (clientset.Interface, error) {
	cluster := c.GetCluster(clusterName)
	if cluster == nil {
		return nil, errors.NewClusterNotFound(clusterName)
	}
	return cluster.GetClientSet()
}

func (c *MultiClusterController) GetClusterObject(clusterName string) (client.Object, error) {
	cluster := c.GetCluster(clusterName)
	if cluster == nil {
		return nil, errors.NewClusterNotFound(clusterName)
	}
	obj, err := cluster.GetObject()
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *MultiClusterController) GetOwnerInfo(clusterName string) (string, string, string, error) {
	cluster := c.GetCluster(clusterName)
	if cluster == nil {
		return "", "", "", errors.NewClusterNotFound(clusterName)
	}
	name, namespace, uid := cluster.GetOwnerInfo()
	return name, namespace, uid, nil
}

// GetClusterNames returns the name list of all managed tenant clusters
func (c *MultiClusterController) GetClusterNames() []string {
	c.Lock()
	defer c.Unlock()
	names := make([]string, 0, len(c.clusters))
	for clusterName := range c.clusters {
		names = append(names, clusterName)
	}
	return names
}

// Eventf constructs an event from the given information and puts it in the queue for sending.
// 'ref' is the object this event is about. Event will make a reference or you may also
// pass a reference to the object directly.
// 'eventtype' of this event, and can be one of Normal, Warning. New types could be added in future
// 'reason' is the reason this event is generated. 'reason' should be short and unique; it
// should be in UpperCamelCase format (starting with a capital letter). "reason" will be used
// to automate handling of events, so imagine people writing switch statements to handle them.
// You want to make that easy.
// 'message' is intended to be human readable.
//
// The resulting event will be created in the same namespace as the reference object.
func (c *MultiClusterController) Eventf(clusterName string, ref *corev1.ObjectReference, eventtype string, reason, messageFmt string, args ...interface{}) error {
	tenantClient, err := c.GetClusterClient(clusterName)
	if err != nil {
		return fmt.Errorf("failed to create client from cluster %s config: %v", clusterName, err)
	}
	namespace := ref.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	eventTime := metav1.Now()
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, eventTime.UnixNano()),
			Namespace: namespace,
		},
		Source: corev1.EventSource{
			Host: clusterName,
		},
		Count:               1, // the count needs to be set for event sinker to work
		InvolvedObject:      *ref,
		Type:                eventtype,
		Reason:              reason,
		Message:             fmt.Sprintf(messageFmt, args...),
		FirstTimestamp:      eventTime,
		LastTimestamp:       eventTime,
		ReportingController: "vc-syncer",
	}

	sink := &v1core.EventSinkImpl{Interface: tenantClient.CoreV1().Events(namespace)}
	return record.EventSinkerInstance.RecordToSink(sink, event)
}

// RequeueObject requeues the cluster object, thus reconcileHandler can reconcile it again.
func (c *MultiClusterController) RequeueObject(clusterName string, obj interface{}) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	cluster := c.GetCluster(clusterName)
	if cluster == nil {
		return errors.NewClusterNotFound(clusterName)
	}
	r := reconciler.Request{}
	r.ClusterName = clusterName
	r.Namespace = o.GetNamespace()
	r.Name = o.GetName()
	r.UID = string(o.GetUID())

	c.Queue.Add(r)
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the reconcileHandler is never invoked concurrently with the same object.
func (c *MultiClusterController) worker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it.
func (c *MultiClusterController) processNextWorkItem() bool {
	obj, shutdown := c.Queue.Get()
	if obj == nil {
		c.Queue.Forget(obj)
	}

	if shutdown {
		// Stop working
		klog.V(4).Info("Shutting down. Ignore work item and stop working.")
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer c.Queue.Done(obj)

	var req reconciler.Request
	var ok bool
	if req, ok = obj.(reconciler.Request); !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process a work item that is invalid.
		c.Queue.Forget(obj)
		klog.Warning("Work item is not a Request. Ignore it. Next.")
		// Return true, don't take a break
		return true
	}
	if c.GetCluster(req.ClusterName) == nil {
		// The virtual cluster has been removed, do not reconcile for its dws requests.
		klog.Warningf("The cluster %s has been removed, drop the dws request %v", req.ClusterName, req)
		c.Queue.Forget(obj)
		return true
	}

	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterPooling) {
		if c.FilterObjectFromSchedulingResult(req) {
			c.Queue.Forget(req)
			c.Queue.Done(req)
			klog.Infof("drop request %+v which doesn't scheduled to this cluster", req)
			return true
		}
	}

	defer metrics.RecordDWSOperationDuration(c.objectKind, req.ClusterName, time.Now())

	// RunInformersAndControllers the syncHandler, passing it the cluster/namespace/Name
	// string of the resource to be synced.
	result, err := c.Reconciler.Reconcile(req)
	if err == nil {
		metrics.RecordDWSOperationStatus(c.objectKind, req.ClusterName, utilconstants.StatusCodeOK)
		if result.RequeueAfter > 0 {
			c.Queue.AddAfter(req, result.RequeueAfter)
		} else if result.Requeue {
			c.Queue.AddRateLimited(req)
		}
		// if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.Queue.Forget(obj)
		return true
	}

	// rejected by apiserver(maybe rejected by webhook or other admission plugins)
	// we take a negative attitude on this situation and fail fast.
	if apierr, ok := err.(apierrors.APIStatus); ok {
		if code := apierr.Status().Code; code == http.StatusBadRequest || code == http.StatusForbidden {
			metrics.RecordDWSOperationStatus(c.objectKind, req.ClusterName, utilconstants.StatusCodeBadRequest)
			klog.Errorf("%s dws request is rejected: %v", c.name, err)
			c.Queue.Forget(obj)
			return true
		}
	}

	// exceed max retry
	if c.Queue.NumRequeues(obj) >= utilconstants.MaxReconcileRetryAttempts {
		metrics.RecordDWSOperationStatus(c.objectKind, req.ClusterName, utilconstants.StatusCodeExceedMaxRetryAttempts)
		c.Queue.Forget(obj)
		klog.Warningf("%s dws request is dropped due to reaching max retry limit: %+v", c.name, obj)
		return true
	}

	metrics.RecordDWSOperationStatus(c.objectKind, req.ClusterName, utilconstants.StatusCodeError)
	c.Queue.AddRateLimited(req)
	klog.Errorf("%s dws request reconcile failed: %v", req, err)
	return true
}

func (c *MultiClusterController) FilterObjectFromSchedulingResult(req reconciler.Request) bool {
	var nsName string
	if c.objectKind == "Namespace" {
		nsName = req.Name
	} else {
		nsName = req.Namespace
	}

	if filterSuperClusterRelatedObject(c, req.ClusterName, nsName) {
		return true
	}

	if c.objectKind == "Pod" {
		if filterSuperClusterSchedulePod(c, req) {
			return true
		}
	}

	return false
}

func filterSuperClusterRelatedObject(c *MultiClusterController, clusterName, nsName string) bool {
	namespace := &corev1.Namespace{}
	if err := c.Get(clusterName, "", nsName, namespace); err != nil {
		klog.Errorf("failed to get ns %s of cluster %s: %v", nsName, clusterName, err)
		return true
	}

	if IsNamespaceScheduledToCluster(namespace, utilconstants.SuperClusterID) != nil {
		return true
	}

	return false
}

func filterSuperClusterSchedulePod(c *MultiClusterController, req reconciler.Request) bool {
	pod := &corev1.Pod{}
	if err := c.Get(req.ClusterName, req.Namespace, req.Name, pod); err != nil {
		klog.Errorf("failed to get pod %+v: %v", req, err)
		return true
	}

	cname, ok := pod.GetAnnotations()[utilconstants.LabelScheduledCluster]
	if !ok {
		return true
	}

	return cname != utilconstants.SuperClusterID
}

func IsNamespaceScheduledToCluster(obj client.Object, clusterID string) error {
	placements := make(map[string]int)
	clist, ok := obj.GetAnnotations()[utilconstants.LabelScheduledPlacements]
	if !ok {
		return fmt.Errorf("missing annotation %s", utilconstants.LabelScheduledPlacements)
	}
	if err := json.Unmarshal([]byte(clist), &placements); err != nil {
		return fmt.Errorf("unknown format %s of key %s: %v", clist, utilconstants.LabelScheduledPlacements, err)
	}

	_, ok = placements[clusterID]
	if !ok {
		return fmt.Errorf("not found")
	}

	return nil
}
