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

package provisioner

import (
	"context"

	tenancyv1alpha1 "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
)

type Provisioner interface {
	CreateVirtualCluster(ctx context.Context, vc *tenancyv1alpha1.VirtualCluster) error
	DeleteVirtualCluster(ctx context.Context, vc *tenancyv1alpha1.VirtualCluster) error
	GetProvisioner() string
}
