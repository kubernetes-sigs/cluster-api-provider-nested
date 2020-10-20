/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var nestedcontrolplanetemplatelog = logf.Log.WithName("nestedtemplate-resource")

func (r *NestedControlPlaneTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(christopherhein): we need to add validations on "default", pull
// implementation from storageClass setup,

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate/controlplane/v1alpha3/nestedcontrolplanetemplate,mutating=false,failurePolicy=fail,groups=controlplane.cluster.x-k8s.io,resources=nestedcontrolplanetemplates,versions=v1alpha3,name=vnestedcontrolplanetemplate.kb.io

var _ webhook.Validator = &NestedControlPlaneTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *NestedControlPlaneTemplate) ValidateCreate() error {
	nestedcontrolplanetemplatelog.Info("validate create", "name", r.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *NestedControlPlaneTemplate) ValidateUpdate(old runtime.Object) error {
	nestedcontrolplanetemplatelog.Info("validate update", "name", r.Name)
	oldObj := old.(*NestedControlPlaneTemplate)

	if err := r.Spec.KubernetesVersion.SupportedVersionChange(oldObj.Spec.KubernetesVersion); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *NestedControlPlaneTemplate) ValidateDelete() error {
	nestedcontrolplanetemplatelog.Info("validate delete", "name", r.Name)

	return nil
}
