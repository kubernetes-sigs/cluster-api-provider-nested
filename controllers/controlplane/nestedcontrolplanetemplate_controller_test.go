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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"

	v1alpha3 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-nested/constants"
	"sigs.k8s.io/cluster-api-provider-nested/testutils"
)

var _ = Describe("Run NestedControlPlaneTemplate Controller", func() {

	Context("Without NestedControlPlaneTemplate{} existing", func() {

		It("Should create NestedControlPlaneTemplate{}", func() {
			ctx := context.Background()
			k8sclient := k8sMgr.GetClient()
			Expect(k8sClient).ToNot(BeNil())

			instance := testutils.MakeNestedControlPlaneTemplate()
			By("Creating new NestedControlPlaneTemplate")
			Expect(k8sClient.Create(ctx, instance)).Should(Succeed())

			key := types.NamespacedName{
				Name:      instance.GetName(),
				Namespace: instance.GetNamespace(),
			}

			By("Expecting Finalizer")
			Eventually(func() bool {
				instance = &v1alpha3.NestedControlPlaneTemplate{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				finalizers := instance.GetFinalizers()

				if len(finalizers) == 0 {
					return false
				}

				return finalizers[0] == constants.NestedControlPlaneTemplateFinalizer
			}, timeout, interval).Should(BeTrue())

			time.Sleep(2 * time.Second)

			Expect(k8sclient.Delete(ctx, instance)).Should(Succeed())

			By("Expecting Removed Finalizer")
			instance = &v1alpha3.NestedControlPlaneTemplate{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, key, instance) != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
