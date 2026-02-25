package config

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	StageDevelopment = "development"
	StageProduction  = "production"
	StageEnvVar      = "STAGE"
	namespaceEnvVar  = "NAMESPACE"
	logLevelEnvVar   = "LOG_LEVEL"
)

var log = ctrl.Log.WithName("config")
var Stage = StageProduction

func IsStageDevelopment() bool {
	return Stage == StageDevelopment
}

// OperatorConfig contains all configurable values for the operator.
type OperatorConfig struct {
	// Namespace specifies the namespace that the operator is deployed to.
	Namespace string
	// ControllerOptions contains the options for the controller manager
	ControllerOptions ctrl.Options
}

// NewOperatorConfig creates a new operator config by reading values from the environment variables
func NewOperatorConfig(scheme *runtime.Scheme) (*OperatorConfig, error) {
	configureStage()

	namespace, err := GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace: %w", err)
	}
	log.Info(fmt.Sprintf("Deploying the k8s-auth-registration-operator in namespace %s", namespace))

	ctrlOptions := getControllerOptions(scheme, namespace)

	return &OperatorConfig{
		Namespace:         namespace,
		ControllerOptions: ctrlOptions,
	}, nil
}

func configureStage() {
	var err error
	Stage, err = getEnvVar(StageEnvVar)
	if err != nil {
		log.Error(err, "Error reading stage environment variable. Use stage production")
		Stage = StageProduction
	}

	if IsStageDevelopment() {
		log.Info("Starting in development mode! This is not recommended for production!")
	}
}

func GetLogLevel() (string, error) {
	logLevel, err := getEnvVar(logLevelEnvVar)
	if err != nil {
		return "", fmt.Errorf("failed to get env var [%s]: %w", logLevelEnvVar, err)
	}

	return logLevel, nil
}

func GetNamespace() (string, error) {
	namespace, err := getEnvVar(namespaceEnvVar)
	if err != nil {
		return "", fmt.Errorf("failed to get env var [%s]: %w", namespaceEnvVar, err)
	}

	return namespace, nil
}

func getEnvVar(name string) (string, error) {
	env, found := os.LookupEnv(name)
	if !found {
		return "", fmt.Errorf("environment variable %s must be set", name)
	}
	return env, nil
}
