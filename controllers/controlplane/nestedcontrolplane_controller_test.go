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

	corev1 "k8s.io/api/core/v1"

	v1alpha3 "sigs.k8s.io/cluster-api-provider-nested/apis/controlplane/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-nested/constants"
	"sigs.k8s.io/cluster-api-provider-nested/testutils"
)

var _ = Describe("Run NestedControlPlane Controller", func() {

	Context("Without NestedControlPlane{} existing", func() {

		It("Should create NestedControlPlane{}", func() {
			ctx := context.Background()
			k8sclient := k8sMgr.GetClient()
			Expect(k8sClient).ToNot(BeNil())

			template := testutils.MakeNestedControlPlaneTemplate(func(tmp *v1alpha3.NestedControlPlaneTemplate) {
				tmp.Name = "template"
			})
			Expect(k8sClient.Create(ctx, template)).Should(Succeed())

			instance := testutils.MakeNestedControlPlane(func(ncp *v1alpha3.NestedControlPlane) {
				ncp.Name = "controlplane"
			})
			By("Creating new NestedControlPlane")
			Expect(k8sClient.Create(ctx, instance)).Should(Succeed())

			key := types.NamespacedName{
				Name:      instance.GetName(),
				Namespace: instance.GetNamespace(),
			}

			By("Expecting Finalizer")
			Eventually(func() bool {
				instance = &v1alpha3.NestedControlPlane{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				finalizers := instance.GetFinalizers()

				if len(finalizers) == 0 {
					return false
				}

				return finalizers[0] == constants.NestedControlPlaneFinalizer
			}, timeout, interval).Should(BeTrue())

			By("Expecting TemplateRef")
			Eventually(func() bool {
				instance = &v1alpha3.NestedControlPlane{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				return instance.Spec.TemplateRef != nil
			}, timeout, interval).Should(BeTrue())

			By("Expecting Namespace Created")
			var namespace *corev1.Namespace
			controlplaneNamespace := instance.Spec.ClusterNamespace

			Eventually(func() bool {
				instance = &v1alpha3.NestedControlPlane{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				nskey := types.NamespacedName{Name: controlplaneNamespace}
				namespace = &corev1.Namespace{}
				err = k8sClient.Get(ctx, nskey, namespace)
				if err != nil {
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				instance = &v1alpha3.NestedControlPlane{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				return instance.Status.ClusterNamespace != ""
			}, timeout, interval).Should(BeTrue())

			By("Expecting PKI to be setup")
			Eventually(func() bool {
				// TODO(christopherhein) test that a PKI resource is created first

				return true
			}, timeout, interval).Should(BeTrue())

			time.Sleep(2 * time.Second)

			Expect(k8sclient.Delete(ctx, instance)).Should(Succeed())

			By("Expecting Removed Finalizer")
			instance = &v1alpha3.NestedControlPlane{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, key, instance) != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
