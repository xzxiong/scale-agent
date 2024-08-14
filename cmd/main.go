/*
Copyright 2024.

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
	"context"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	//+kubebuilder:scaffold:imports

	"github.com/matrixorigin/scale-agent/cmd/setup"
	"github.com/matrixorigin/scale-agent/pkg/config"
	"github.com/matrixorigin/scale-agent/pkg/controller"
	selfpod "github.com/matrixorigin/scale-agent/pkg/pod"
	selfutil "github.com/matrixorigin/scale-agent/pkg/util"
	"github.com/matrixorigin/scale-agent/pkg/version"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	defaultMetricsAddr = ":8080"
	defaultProbeAddr   = ":8081"
	defaultPprofAddr   = ":8082"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var ctx = context.Background()
	var podManager *selfpod.DaemonSetManager
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var pprofAddr string
	var versionShow bool
	var cfgPath string
	flag.StringVar(&metricsAddr, "metrics-bind-address", defaultMetricsAddr, "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", defaultProbeAddr, "The address the probe endpoint binds to.")
	flag.StringVar(&pprofAddr, "pprof-address", defaultPprofAddr, "The address the pprof endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&versionShow, "version", false, "Show version and quit")
	flag.StringVar(&cfgPath, "cfg", "", "The config filepath")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	// END> cmd-line parsed.

	if versionShow {
		fmt.Println(version.GetInfoOneLine())
		os.Exit(0)
	}

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	selfutil.SetLogger(logger)

	// read config
	if cfgPath != "" {
		if _, err := config.InitConfiguration(setupLog, cfgPath); err != nil {
			os.Exit(1)
		}
		// load NodeName
		cfg := config.GetConfiguration()
		if cfg.App.NodeName == config.NodeNameAuto {
			if nodeName, err := setup.GetNodeName(ctx); err != nil {
				os.Exit(1)
			} else {
				cfg.App.SetNodeName(nodeName)
			}
		}
	}

	if err := config.GetConfiguration().Validate(); err != nil {
		setupLog.Error(err, "failed to load configuration")
		os.Exit(1)
	}

	if err := setup.GetKubeletServer(); err != nil {
		setupLog.Error(err, "failed to Load kubelet server")
		os.Exit(1)
	}
	// check cgroup manager
	isCgroupV2, err := setup.IsCgroupV2()
	if err != nil {
		setupLog.Error(err, "failed to check cgroup version")
	}
	if !isCgroupV2 {
		// do NOT start up the main flow.
		setupLog.Info("current cgroup version is not supported. please run in cgroup v2.")
	} else {
		// TODO: init strategy manager
		// TODO: init pod manager
		// 	- need wait mgr.Elected()
		podManager = selfpod.NewDaemonSetManager(ctx)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		//MetricsBindAddress: metricsAddr,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress: probeAddr,
		PprofBindAddress:       pprofAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f67856fc.core.matrixone-cloud",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.PodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Guestbook")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// init self-defined. indexer
	setup.InitManagerIndexer(mgr, ctx)

	if podManager != nil {
		go func() {
			<-mgr.Elected()
			setupLog.Info("starting pod manager")
			podManager.Start()
		}()
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
