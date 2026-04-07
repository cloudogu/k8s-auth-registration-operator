package config

import (
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	StageDevelopment  = "development"
	StageProduction   = "production"
	StageEnvVar       = "STAGE"
	namespaceEnvVar   = "NAMESPACE"
	logLevelEnvVar    = "LOG_LEVEL"
	casBaseURLEnvVar  = "CAS_BASE_URL"
	casUsernameEnvVar = "CAS_USERNAME"
	casPasswordEnvVar = "CAS_PASSWORD"
	casTimeoutEnvVar  = "CAS_TIMEOUT"
	defaultCASTimeout = 10 * time.Second
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
	// CasConf contains the connection settings for the Cas service registry API.
	CasConf CasConfig
	// ControllerOptions contains the options for the controller manager
	ControllerOptions ctrl.Options
}

type CasConfig struct {
	// BaseURL contains the URL to the CAS component service including schema, port, and context path
	BaseURL string
	// Username contains the basic auth username to access the registeredServices endpoint.
	Username string
	// Password contains the basic auth password to access the registeredServices endpoint.
	Password string
	// Timeout limits the CAS request time. Defaults to 10 seconds.
	Timeout time.Duration
}

// NewOperatorConfig creates a new operator config by reading values from the environment variables
func NewOperatorConfig(scheme *runtime.Scheme) (*OperatorConfig, error) {
	configureStage()

	namespace, err := GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace: %w", err)
	}
	log.Info(fmt.Sprintf("Deploying the k8s-auth-registration-operator in namespace %s", namespace))

	casConfig, err := getCASConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read Cas config: %w", err)
	}

	ctrlOptions := getControllerOptions(scheme, namespace)

	return &OperatorConfig{
		Namespace:         namespace,
		CasConf:           casConfig,
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

func getCASConfig() (CasConfig, error) {
	baseURL, err := getEnvVar(casBaseURLEnvVar)
	if err != nil {
		return CasConfig{}, err
	}

	username, err := getEnvVar(casUsernameEnvVar)
	if err != nil {
		return CasConfig{}, err
	}

	password, err := getEnvVar(casPasswordEnvVar)
	if err != nil {
		return CasConfig{}, err
	}

	timeout, err := getEnvDuration(casTimeoutEnvVar, defaultCASTimeout)
	if err != nil {
		return CasConfig{}, err
	}

	return CasConfig{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Timeout:  timeout,
	}, nil
}

func getEnvVar(name string) (string, error) {
	env, found := os.LookupEnv(name)
	if !found {
		return "", fmt.Errorf("environment variable %s must be set", name)
	}
	return env, nil
}

func getEnvDuration(name string, defaultValue time.Duration) (time.Duration, error) {
	value, found := os.LookupEnv(name)
	if !found || value == "" {
		return defaultValue, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("failed to parse env var [%s] as duration: %w", name, err)
	}

	return duration, nil
}
