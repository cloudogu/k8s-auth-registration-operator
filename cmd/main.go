package main

import (
	"fmt"
	"os"

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
	casRegisteredServicesEndpointEnv = "CAS_REGISTEREDSERVICES_ENDPOINT"
	casRegisteredServicesUsernameEnv = "CAS_REGISTEREDSERVICES_USERNAME"
	casRegisteredServicesPasswordEnv = "CAS_REGISTEREDSERVICES_PASSWORD"
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

// nolint:gocyclo
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
	serviceRegistrationBackend, err := resolveServiceRegistrationBackendFromEnv()
	if err != nil {
		return fmt.Errorf("failed to configure service registration backend: %w", err)
	}

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

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("failed to startManager manager: %w", err)
	}

	return nil
}

func resolveServiceRegistrationBackendFromEnv() (*registration.NoOpServiceRegistrationBackend, error) {
	// FIXME use cas service registration backend
	setupLog.Info("Using no-op service registration backend (CAS endpoint not configured)")

	return &registration.NoOpServiceRegistrationBackend{}, nil
}
