package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/config"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/registration"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and startManager can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/cloudogu/k8s-auth-registration-operator/internal/controller"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	// +kubebuilder:scaffold:imports
)

const (
	gracefulShutdownTimeout = 15 * time.Second
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(authregistrationv1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	config.ConfigureLogger()

	cfg, err := config.NewOperatorConfig(scheme)
	if err != nil {
		setupLog.Error(err, "Failed to create operator config")
		os.Exit(1)
	}

	if err := startManager(cfg); err != nil {
		setupLog.Error(err, "Failed to start manager.")
		os.Exit(1)
	}
}

func startManager(cfg *config.OperatorConfig) error {
	//TODO replace with real cas backend
	serviceRegistrationBackend := &registration.NoOpServiceRegistrationBackend{}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), cfg.ControllerOptions)
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	authRegCtrl := controller.NewAuthRegistrationReconciler(mgr.GetClient(), mgr.GetScheme(), serviceRegistrationBackend)

	if err := authRegCtrl.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to set up ready check: %w", err)
	}

	return startManagerWithGracefulShutdown(mgr)
}

func startManagerWithGracefulShutdown(mgr ctrl.Manager) error {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	managerCtx, cancelManager := context.WithCancel(context.Background())
	defer cancelManager()

	managerErrCh := make(chan error, 1)
	go func() {
		managerErrCh <- mgr.Start(managerCtx)
	}()

	setupLog.Info("Starting manager")

	select {
	case err := <-managerErrCh:
		if err != nil {
			return fmt.Errorf("failed to run manager: %w", err)
		}
		return nil
	case <-signalCtx.Done():
		setupLog.Info("Shutdown signal received. Stopping manager gracefully.")
		cancelManager()
	}

	shutdownTimer := time.NewTimer(gracefulShutdownTimeout)
	defer shutdownTimer.Stop()

	select {
	case err := <-managerErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("manager stopped with an error during shutdown: %w", err)
		}
		setupLog.Info("Manager stopped gracefully")
		return nil
	case <-shutdownTimer.C:
		return fmt.Errorf("graceful shutdown timed out after %s", gracefulShutdownTimeout)
	}
}
