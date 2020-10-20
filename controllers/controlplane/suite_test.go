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
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	apis "sigs.k8s.io/cluster-api-provider-nested/apis"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sMgr ctrl.Manager
var testEnv *envtest.Environment
var scheme = clientgoscheme.Scheme

var timeout = time.Second * 10
var interval = time.Second * 1

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = clusterv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = apis.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sMgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&NestedControlPlaneTemplateReconciler{
		Client: k8sMgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NestedControlPlaneTemplate"),
		Scheme: k8sMgr.GetScheme(),
		Event:  k8sMgr.GetEventRecorderFor("NestedControlPlaneTemplate"),
	}).SetupWithManager(k8sMgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&NestedControlPlaneReconciler{
		Client: k8sMgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NestedControlPlane"),
		Scheme: k8sMgr.GetScheme(),
		Event:  k8sMgr.GetEventRecorderFor("NestedControlPlane"),
	}).SetupWithManager(k8sMgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&NestedPKIReconciler{
		Client: k8sMgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("NestedPKI"),
		Scheme: k8sMgr.GetScheme(),
		Event:  k8sMgr.GetEventRecorderFor("NestedPKI"),
	}).SetupWithManager(k8sMgr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sMgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sMgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
