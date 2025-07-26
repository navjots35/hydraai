package main

import (
	"context"
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hydracontroller "github.com/hydraai/hydra-route/internal/controller"
	"github.com/hydraai/hydra-route/internal/metrics"
	"github.com/hydraai/hydra-route/internal/scaler"
	hydraconfig "github.com/hydraai/hydra-route/pkg/config"

	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
}

func main() {
	var (
		probeAddr            = flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
		enableLeaderElection = flag.Bool("leader-elect", false, "Enable leader election for controller manager.")
		configPath           = flag.String("config", "/etc/hydra-route/config.yaml", "Path to the configuration file.")
		logLevel             = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Setup logger
	setupLogger(*logLevel)

	// Load configuration
	cfg, err := hydraconfig.LoadConfig(*configPath)
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// Setup manager
	opts := ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: *probeAddr,
		LeaderElection:         *enableLeaderElection,
		LeaderElectionID:       "hydra-route-leader-election",
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup metrics collector
	metricsCollector := metrics.NewCollector(mgr.GetClient(), cfg.Metrics)

	// Setup AI scaler
	aiScaler := scaler.NewAIScaler(cfg.Scaling)

	// Setup controller
	hydraController := &hydracontroller.HydraRouteReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		MetricsCollector: metricsCollector,
		AIScaler:         aiScaler,
		Config:           cfg,
	}

	// Setup controller with manager
	if err := hydraController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller")
		os.Exit(1)
	}

	// Start metrics collection
	ctx := context.Background()
	go metricsCollector.Start(ctx)

	logrus.Info("Starting Hydra Route Controller")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupLogger(level string) {
	logrus.SetFormatter(&logrus.JSONFormatter{})

	switch level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Also setup controller-runtime logger
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}
