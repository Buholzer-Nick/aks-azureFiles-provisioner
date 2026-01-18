package main

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"aks-azureFiles-controller/internal/azure"
	"aks-azureFiles-controller/internal/config"
	"aks-azureFiles-controller/internal/controller"
	"aks-azureFiles-controller/internal/logging"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
}

func main() {
	logger := logging.NewLogger()
	ctrl.SetLogger(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error(err, "load config")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: cfg.MetricsAddr},
		HealthProbeBindAddress: cfg.HealthAddr,
		LeaderElection:         cfg.LeaderElectionEnabled,
		LeaderElectionID:       cfg.LeaderElectionID,
	})
	if err != nil {
		logger.Error(err, "create manager")
		os.Exit(1)
	}

	cred, authMode, err := azure.NewCredential(azure.CredentialConfig{
		AuthMode: cfg.AuthMode,
		TenantID: cfg.TenantID,
		ClientID: cfg.ClientID,
	})
	if err != nil {
		logger.Error(err, "create azure credential")
		os.Exit(1)
	}
	logger.Info("azure authentication configured", "mode", authMode)

	shareClient, err := azure.NewClientWithCredential(cfg.StorageAccount, cred)
	if err != nil {
		logger.Error(err, "create share client")
		os.Exit(1)
	}

	reconcileMetrics := controller.NewReconcileMetrics()
	if err := reconcileMetrics.Register(metrics.Registry); err != nil {
		logger.Error(err, "register metrics")
		os.Exit(1)
	}

	reconciler := &controller.PVCReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("azurefile-provisioner"),
		Config: controller.ReconcilerConfig{
			ResourceGroup:  cfg.ResourceGroup,
			StorageAccount: cfg.StorageAccount,
			Server:         cfg.Server,
		},
		Shares:  shareClient,
		Metrics: reconcileMetrics,
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		logger.Error(err, "setup controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "add health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error(err, "add ready check")
		os.Exit(1)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(fmt.Errorf("start manager: %w", err), "manager exited")
		os.Exit(1)
	}
}
