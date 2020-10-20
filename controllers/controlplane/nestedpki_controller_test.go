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
	"sigs.k8s.io/cluster-api-provider-nested/secret"
	"sigs.k8s.io/cluster-api-provider-nested/testutils"
)

var _ = Describe("Run NestedPKI Controller", func() {

	Context("Without NestedPKI{} existing", func() {

		It("Should create NestedPKI{}", func() {
			ctx := context.Background()
			k8sclient := k8sMgr.GetClient()
			Expect(k8sClient).ToNot(BeNil())

			namespace := testutils.MakeNamespace(func(ns *corev1.Namespace) {
				ns.Name = "ncp"
			})
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())

			template := testutils.MakeNestedControlPlaneTemplate()
			Expect(k8sClient.Create(ctx, template)).Should(Succeed())

			controlplane := testutils.MakeNestedControlPlane(func(ncp *v1alpha3.NestedControlPlane) {
				ncp.Spec.ClusterNamespace = "ncp"

			})
			Expect(k8sClient.Create(ctx, controlplane)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: controlplane.GetName(), Namespace: controlplane.GetNamespace()}, controlplane)
				if err != nil {
					return false
				}

				controlplane.Status.ClusterNamespace = "ncp"
				err = k8sClient.Status().Update(ctx, controlplane)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			instance := testutils.MakeNestedPKI(func(pki *v1alpha3.NestedPKI) {
				pki.Namespace = "ncp"
				pki.Spec.ControlPlaneRef = &corev1.ObjectReference{
					APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
					Kind:       "NestedControlPlane",
					Name:       controlplane.GetName(),
					UID:        controlplane.GetUID(),
					Namespace:  controlplane.GetNamespace(),
				}
			})
			By("Creating new NestedPKI")
			Expect(k8sClient.Create(ctx, instance)).Should(Succeed())

			key := types.NamespacedName{
				Name:      instance.GetName(),
				Namespace: instance.GetNamespace(),
			}

			By("Expecting Finalizer")
			Eventually(func() bool {
				instance = &v1alpha3.NestedPKI{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				finalizers := instance.GetFinalizers()

				if len(finalizers) == 0 {
					return false
				}

				return finalizers[0] == constants.NestedPKIFinalizer
			}, timeout, interval).Should(BeTrue())

			By("Expecting Ready == False")
			Eventually(func() bool {
				instance = &v1alpha3.NestedPKI{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				return instance.Status.Ready == false
			}, timeout, interval).Should(BeTrue())

			controlplaneName := instance.Spec.ControlPlaneRef.Name
			controlplaneNamespace := instance.GetNamespace()

			By("Expecting PKI to be setup")
			Eventually(func() bool {
				secretName := types.NamespacedName{Namespace: controlplaneNamespace, Name: controlplaneName + "-ca"}
				sec := &corev1.Secret{}
				err := k8sClient.Get(ctx, secretName, sec)
				if err != nil {
					return false
				}

				secretName = types.NamespacedName{Namespace: controlplaneNamespace, Name: controlplaneName + "-proxy"}
				sec = &corev1.Secret{}
				err = k8sClient.Get(ctx, secretName, sec)
				if err != nil {
					return false
				}

				secretName = types.NamespacedName{Namespace: controlplaneNamespace, Name: controlplaneName + "-etcd"}
				sec = &corev1.Secret{}
				err = k8sClient.Get(ctx, secretName, sec)
				if err != nil {
					return false
				}

				secretName = types.NamespacedName{Namespace: controlplaneNamespace, Name: secret.ControllerManagerSecretName}
				sec = &corev1.Secret{}
				err = k8sClient.Get(ctx, secretName, sec)
				if err != nil {
					return false
				}

				secretName = types.NamespacedName{Namespace: controlplaneNamespace, Name: secret.AdminSecretName}
				sec = &corev1.Secret{}
				err = k8sClient.Get(ctx, secretName, sec)
				if err != nil {
					return false
				}

				secretName = types.NamespacedName{Namespace: controlplaneNamespace, Name: secret.ServiceAccountSecretName}
				sec = &corev1.Secret{}
				err = k8sClient.Get(ctx, secretName, sec)
				if err != nil {
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			By("Expecting Ready == Tue")
			Eventually(func() bool {
				instance = &v1alpha3.NestedPKI{}
				err := k8sClient.Get(ctx, key, instance)
				if err != nil {
					return false
				}

				return instance.Status.Ready == true
			}, timeout, interval).Should(BeTrue())

			time.Sleep(2 * time.Second)

			Expect(k8sclient.Delete(ctx, instance)).Should(Succeed())

			By("Expecting Removed Finalizer")
			instance = &v1alpha3.NestedPKI{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, key, instance) != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
