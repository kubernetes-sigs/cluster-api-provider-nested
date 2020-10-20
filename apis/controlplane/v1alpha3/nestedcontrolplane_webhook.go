/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var nestedcontrolplanelog = logf.Log.WithName("nestedcontrolplane-resource")

func (r *NestedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(christopherhein): we need to add validations here on whether the root
// namespace isn't taken

// +kubebuilder:webhook:path=/mutate/controlplane/v1alpha3/nestedcontrolplane,mutating=true,failurePolicy=fail,groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanes,verbs=create;update,versions=v1alpha3,name=mnestedcontrolplane.kb.io

var _ webhook.Defaulter = &NestedControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *NestedControlPlane) Default() {
	nestedcontrolplanelog.Info("default", "name", r.Name)

	// if clusterNamespace not set then set using {namespace}-{vcp.uid}-{vc-namespace}
	if r.Spec.ClusterNamespace == "" {
		digest := sha256.Sum256([]byte(r.GetUID()))
		namespaceSlice := []string{r.Namespace, hex.EncodeToString(digest[0:])[0:6], r.Name}
		namespace := strings.Join(namespaceSlice, "-")
		r.Spec.ClusterNamespace = namespace
	}
}
