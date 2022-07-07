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

package constants

import (
	"fmt"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/version"
)

// SchedulerContextKey is a type for context key
type SchedulerContextKey string

const (

	// Override the client-go default 5 qps and 10 burst

	// DefaultSchedulerClientQPS allows 100 qps
	DefaultSchedulerClientQPS = 100
	// DefaultSchedulerClientBurst allows burst of 500 qps
	DefaultSchedulerClientBurst = 500

	// DefaultRequestTimeout is set for all client-go request. This is the absolute
	// timeout of the HTTP request, including reading the response body.
	DefaultRequestTimeout = 30 * time.Second

	// VirtualClusterWorker set amount of workers
	VirtualClusterWorker = 3
	// SuperClusterWorker set amount of workers
	SuperClusterWorker = 3

	// KubeconfigAdminSecretName name of secret with kubeconfig for admin
	KubeconfigAdminSecretName = "admin-kubeconfig"

	// InternalSchedulerCache name of the context key with cache settings
	InternalSchedulerCache SchedulerContextKey = "tenancy.x-k8s.io/schedulercache"
	// InternalSchedulerEngine name of the context key with engine
	InternalSchedulerEngine SchedulerContextKey = "tenancy.x-k8s.io/schedulerengine"
	// InternalSchedulerManager name of the context key with manager
	InternalSchedulerManager SchedulerContextKey = "tenancy.x-k8s.io/schedulermanager"
)

// SchedulerUserAgent is a useragent for scheduler
var SchedulerUserAgent = "scheduler" + version.BriefVersion()

// ShadowClusterCapacity has a fake "unlimited" capacity
var ShadowClusterCapacity = corev1.ResourceList{
	corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", math.MaxInt32)),
	corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", math.MaxInt32)),
}
