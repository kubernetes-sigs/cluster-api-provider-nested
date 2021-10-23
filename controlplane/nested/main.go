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
package main

import (
	"flag"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	infrastructurev1alpha4 "sigs.k8s.io/cluster-api-provider-nested/api/v1alpha4"
	controlplanev1alpha4 "sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-nested/controlplane/nested/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// Command line flags for configuring the controller manager.
	metricsAddr                 string
	enableLeaderElection        bool
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	profilerAddress             string
	syncPeriod                  time.Duration
	webhookPort                 int
	healthAddr                  string
	templatePath                string
)

func init() {

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(controlplanev1alpha4.AddToScheme(scheme))
	utilruntime.Must(infrastructurev1alpha4.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	fs.StringVar(&metricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")

	fs.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	fs.DurationVar(&leaderElectionLeaseDuration, "leader-elect-lease-duration", 15*time.Second,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)")

	fs.DurationVar(&leaderElectionRenewDeadline, "leader-elect-renew-deadline", 10*time.Second,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)")

	fs.DurationVar(&leaderElectionRetryPeriod, "leader-elect-retry-period", 2*time.Second,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)")

	fs.StringVar(&profilerAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	fs.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	fs.IntVar(&webhookPort, "webhook-port", 0,
		"Webhook Server port, disabled by default. When enabled, the manager will only work as webhook server, no reconcilers are installed.")

	fs.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")

	fs.StringVar(&templatePath, "template-path", "/component-templates",
		"The address the health endpoint binds to.")

	feature.MutableGates.AddFlag(fs)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	InitFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if profilerAddress != "" {
		setupLog.Info("Profiler listening for requests", "address", profilerAddress)
		go func() {
			setupLog.Error(http.ListenAndServe(profilerAddress, nil), "address", profilerAddress)
		}()
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "controller-leader-election-capn",
		LeaseDuration:          &leaderElectionLeaseDuration,
		RenewDeadline:          &leaderElectionRenewDeadline,
		RetryPeriod:            &leaderElectionRetryPeriod,
		SyncPeriod:             &syncPeriod,
		Port:                   webhookPort,
		HealthProbeBindAddress: healthAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	// Setup health checks.
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	if err = (&controllers.NestedControlPlaneReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("controlplane").WithName("NestedControlPlane"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NestedControlPlane")
		os.Exit(1)
	}

	if err = (&controllers.NestedEtcdReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		TemplatePath: templatePath,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NestedEtcd")
		os.Exit(1)
	}

	if err = (&controllers.NestedAPIServerReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		TemplatePath: templatePath,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NestedAPIServer")
		os.Exit(1)
	}

	if err = (&controllers.NestedControllerManagerReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		TemplatePath: templatePath,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NestedControllerManager")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("Starting manager", "version", version.Get().String())
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
