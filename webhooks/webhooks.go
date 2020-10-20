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

package webhooks

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	controlplanev1alpha3 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
)

// SetupWithManager will configure all the webhooks for the capn manager
func SetupWithManager(mgr ctrl.Manager) (err error) {
	if err = (&controlplanev1alpha3.NestedControlPlaneTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create webhook webhook NestedControlPlaneTemplate  err=%s", err)
	}

	if err = (&controlplanev1alpha3.NestedControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create webhook webhook NestedControlPlane err=%s", err)
	}
	return
}
