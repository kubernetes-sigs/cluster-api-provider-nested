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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/webhook"

	cliflag "k8s.io/component-base/cli/flag"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/constants"
	logrutil "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/controller/util/logr"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/version"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/version/verflag"
)

func main() {
	var (
		logFile                           string
		metricsAddr                       string
		healthAddr                        string
		controlPlaneProvisioner           string
		controlPlaneProvisionerDeprecated string
		leaderElection                    bool
		leaderElectionCmName              string
		leaderElectionID                  string
		leaderElectionResourceLock        string
		maxConcurrentReconciles           int
		versionOpt                        bool
		disableStacktrace                 bool
		enableWebhook                     bool
		provisionerTimeout                time.Duration

		featureGates map[string]bool
	)
	flag.StringVar(&metricsAddr, "metrics-addr", ":0", "The address the metric endpoint binds to.")
	flag.StringVar(&healthAddr, "health-addr", ":8080", "The address of the healthz/readyz endpoint binds to.")
	flag.StringVar(&controlPlaneProvisionerDeprecated, "master-prov", "",
		"DEPRECATED. Use --provisioner flag instead.")
	flag.StringVar(&controlPlaneProvisioner, "provisioner", "native",
		"The underlying platform that will provision control plane for virtualcluster.")
	flag.BoolVar(&leaderElection, "leader-election", true, "If enable leaderelection for vc-manager")
	// Deprecated: the flag used resource type as part of the name. Replaced by le-name.
	flag.StringVar(&leaderElectionCmName, "le-cm-name", "", "DEPRECATED. Use --le-name instead")
	flag.StringVar(&leaderElectionID, "le-name", "vc-manager-leaderelection-lock",
		"The name of the resource that will be used as the resourcelock for leaderelection")
	flag.StringVar(&leaderElectionResourceLock, "le-resource", "configmapsleases",
		"The type of the resource that will be used as the resourcelock for leaderelection [configmaps, leases, configmapsleases]")
	flag.IntVar(&maxConcurrentReconciles, "num-reconciles", 10,
		"The max number reconcilers of virtualcluster controller")
	flag.StringVar(&logFile, "log-file", "", "The path of the logfile, if not set, only log to the stderr")
	flag.BoolVar(&versionOpt, "version", false, "Print the version information")
	flag.BoolVar(&disableStacktrace, "disable-stacktrace", false, "If set, the automatic stacktrace is disabled")
	flag.BoolVar(&enableWebhook, "enable-webhook", false, "If set, the virtualcluster webhook is enabled")
	flag.DurationVar(&provisionerTimeout, "provisioner-timeout", 10*time.Minute, "The timeout for provision control-plane statefulsets")

	flag.Var(cliflag.NewMapStringBool(&featureGates), "feature-gates", "A set of key=value pairs that describe featuregate gates for various features.")

	flag.Parse()

	// print version information
	if versionOpt {
		fmt.Printf("VirtualCluster %s\n", verflag.GetVersion(version.Get()))
		os.Exit(0)
	}

	loggr, err := logrutil.NewLogger(logFile, disableStacktrace)
	if err != nil {
		panic(fmt.Sprintf("fail to initialize logr: %s", err))
	}
	logf.SetLogger(loggr)
	log := logf.Log.WithName("entrypoint")

	featuregate.DefaultFeatureGate, err = featuregate.NewFeatureGate(featureGates)
	if err != nil {
		log.Error(err, "unable to set up feature gates")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")

	if leaderElectionCmName != "" {
		leaderElectionID = leaderElectionCmName
	}

	mgrOpt := manager.Options{
		MetricsBindAddress:         metricsAddr,
		LeaderElection:             leaderElection,
		LeaderElectionID:           leaderElectionID,
		LeaderElectionResourceLock: leaderElectionResourceLock,
		CertDir:                    constants.VirtualClusterWebhookCertDir,
		Port:                       constants.VirtualClusterWebhookPort,
		HealthProbeBindAddress:     healthAddr,
	}
	mgr, err := ctrl.NewManager(cfg, mgrOpt)
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		log.Error(err, "unable to create ready check")
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		log.Error(err, "unable to create health check")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	log.Info("setting up scheme")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	if err := clusterv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable add APIs to scheme")
		os.Exit(1)
	}

	if controlPlaneProvisionerDeprecated != "" {
		log.Info("The --master-prov flag is deprecated. Use --provisioner instead")
		controlPlaneProvisioner = controlPlaneProvisionerDeprecated
	}

	// Setup all Controllers
	log.Info("Setting up controller")
	if err := (&controller.Controllers{
		Log:                     log.WithName("Controllers"),
		Client:                  mgr.GetClient(),
		ProvisionerName:         controlPlaneProvisioner,
		ProvisionerTimeout:      provisionerTimeout,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to register controllers to the manager")
		os.Exit(1)
	}

	if enableWebhook {
		log.Info("setting up webhooks")
		if err := webhook.AddToManager(mgr, mgrOpt.CertDir); err != nil {
			log.Error(err, "unable to register webhooks to the manager")
			os.Exit(1)
		}
	}

	// Start the Cmd
	log.Info("Starting the Cmd.")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "unable to run the manager")
		os.Exit(1)
	}
}
