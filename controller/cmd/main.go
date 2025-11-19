// Package main is the entry point for the StreamSpace Kubernetes controller.
//
// This controller manages the lifecycle of StreamSpace custom resources:
//   - Session: User workspace sessions with auto-hibernation
//   - Template: Application template definitions
//   - ApplicationInstall: Application installations from catalog
//
// The controller uses Kubebuilder framework and implements reconciliation loops
// to ensure the actual cluster state matches the desired state defined in CRDs.
//
// Key responsibilities:
//   - Session lifecycle management (create, update, delete)
//   - Auto-hibernation based on idle timeouts
//   - User persistent volume provisioning
//   - Template validation and management
//   - Application installation and Template creation
//   - Prometheus metrics export for monitoring
//
// Architecture:
//   - SessionReconciler: Main reconciler for Session resources
//   - TemplateReconciler: Reconciler for Template resources
//   - HibernationReconciler: Handles automatic session hibernation
//   - ApplicationInstallReconciler: Creates Templates from ApplicationInstall CRDs
//
// Deployment:
//   The controller is designed to run as a Kubernetes Deployment with:
//   - Leader election for high availability
//   - Health and readiness probes
//   - Prometheus metrics endpoint on :8080
//   - Health probes on :8081
//
// Example usage:
//
//	# Run controller with leader election enabled
//	./controller --leader-elect=true
//
//	# Run with custom metrics address
//	./controller --metrics-bind-address=:9090
//
//	# Enable debug logging
//	./controller --zap-log-level=debug
package main

import (
	"context"
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
	"github.com/streamspace/streamspace/controllers"
	"github.com/streamspace/streamspace/pkg/events"
	_ "github.com/streamspace/streamspace/pkg/metrics" // Initialize custom metrics
)

var (
	// scheme defines the runtime scheme used by the controller.
	// It includes standard Kubernetes types and StreamSpace custom resources.
	scheme = runtime.NewScheme()

	// setupLog is the logger used during controller initialization.
	setupLog = ctrl.Log.WithName("setup")
)

// init registers all required schemes with the controller's runtime scheme.
// This must happen before the manager is created to ensure all types are recognized.
func init() {
	// Register standard Kubernetes types (Pods, Services, Deployments, etc.)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Register StreamSpace custom resource definitions (Session, Template)
	utilruntime.Must(streamv1alpha1.AddToScheme(scheme))
}

// main is the entry point for the StreamSpace controller.
//
// It performs the following initialization steps:
//  1. Parse command-line flags for configuration
//  2. Initialize structured logging with zap
//  3. Create controller manager with leader election
//  4. Register reconcilers for custom resources
//  5. Setup health and readiness probes
//  6. Start the manager and wait for shutdown signal
//
// The controller will exit with code 1 if any initialization step fails.
func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var natsURL string
	var natsUser string
	var natsPassword string
	var namespace string
	var controllerID string

	// Parse command-line flags
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&natsURL, "nats-url", getEnv("NATS_URL", "nats://localhost:4222"), "NATS server URL")
	flag.StringVar(&natsUser, "nats-user", getEnv("NATS_USER", ""), "NATS username")
	flag.StringVar(&natsPassword, "nats-password", getEnv("NATS_PASSWORD", ""), "NATS password")
	flag.StringVar(&namespace, "namespace", getEnv("NAMESPACE", "streamspace"), "Kubernetes namespace")
	flag.StringVar(&controllerID, "controller-id", getEnv("CONTROLLER_ID", "streamspace-kubernetes-controller-1"), "Unique controller ID")

	// Setup logging options (can be configured via flags like --zap-log-level=debug)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Initialize structured logger
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Create controller manager
	// The manager coordinates all controllers and provides shared dependencies:
	//   - Kubernetes client for CRUD operations
	//   - Cache for efficient resource watching
	//   - Metrics registry for Prometheus
	//   - Leader election for high availability
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,

		// Health probe endpoint for Kubernetes liveness/readiness checks
		HealthProbeBindAddress: probeAddr,

		// Leader election ensures only one controller instance is active
		// Critical for preventing race conditions in multi-replica deployments
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "streamspace.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Register SessionReconciler
	// Manages the lifecycle of Session resources:
	//   - Creates Deployments, Services, and PVCs for user sessions
	//   - Handles state transitions (running, hibernated, terminated)
	//   - Updates status with pod information and resource usage
	if err = (&controllers.SessionReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Session")
		os.Exit(1)
	}

	// Register TemplateReconciler
	// Validates and manages Template resources:
	//   - Ensures template specifications are valid
	//   - Tracks template usage and popularity
	//   - Handles template versioning
	if err = (&controllers.TemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Template")
		os.Exit(1)
	}

	// Register HibernationReconciler
	// Implements automatic session hibernation:
	//   - Monitors session idle timeouts
	//   - Scales Deployments to zero replicas when idle
	//   - Wakes sessions on user activity
	//   - Updates Session status and metrics
	if err = (&controllers.HibernationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Hibernation")
		os.Exit(1)
	}

	// Register ApplicationInstallReconciler
	// Handles application installation from the catalog:
	//   - Watches ApplicationInstall CRDs created by the API
	//   - Parses the manifest field to create Template CRDs
	//   - Sets owner references for cascading deletion
	//   - Updates status with creation progress (Pending → Creating → Ready/Failed)
	if err = (&controllers.ApplicationInstallReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ApplicationInstall")
		os.Exit(1)
	}

	// Setup health check endpoint
	// Kubernetes uses /healthz to determine if the controller is alive
	// Returns 200 OK when controller is running
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// Setup readiness check endpoint
	// Kubernetes uses /readyz to determine if the controller is ready to serve requests
	// Returns 200 OK when all reconcilers are initialized and caches are synced
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Initialize NATS event subscriber for platform-agnostic event handling
	setupLog.Info("initializing NATS event subscriber", "url", natsURL)
	subscriber, err := events.NewSubscriber(events.Config{
		URL:      natsURL,
		User:     natsUser,
		Password: natsPassword,
	}, mgr.GetClient(), namespace, controllerID)

	if err != nil {
		setupLog.Error(err, "unable to create NATS subscriber")
		setupLog.Info("continuing without NATS - controller will only watch CRDs directly")
	} else {
		// Start subscriber in background
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer subscriber.Close()

		go func() {
			if err := subscriber.Start(ctx); err != nil {
				setupLog.Error(err, "NATS subscriber error")
			}
		}()
		setupLog.Info("NATS event subscriber started", "controller_id", controllerID)
	}

	// Start the manager and begin reconciliation loops
	// SetupSignalHandler() ensures graceful shutdown on SIGTERM/SIGINT
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// getEnv gets an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
