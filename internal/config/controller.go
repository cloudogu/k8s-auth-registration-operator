package config

import (
	"crypto/tls"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func getControllerOptions(scheme *runtime.Scheme, namespace string) ctrl.Options {
	flagsConfig := parseFlags()

	tlsOpts := createTLSOptions(flagsConfig)
	webhookServer := createWebhookServer(flagsConfig, tlsOpts)
	metricsOptions := createMetricsServerOptions(flagsConfig, tlsOpts)

	return ctrl.Options{
		Scheme:        scheme,
		Metrics:       metricsOptions,
		WebhookServer: webhookServer,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
		},
		HealthProbeBindAddress: flagsConfig.ProbeAddr,
		LeaderElection:         flagsConfig.EnableLeaderElection,
		LeaderElectionID:       "ec33e801.k8s.cloudogu.com",
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
	}
}

func createTLSOptions(flagsConfig Flags) []func(*tls.Config) {
	var tlsOpts []func(*tls.Config)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		log.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !flagsConfig.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	return tlsOpts
}

func createWebhookServer(flagsConfig Flags, tlsOpts []func(*tls.Config)) webhook.Server {
	webhookServerOptions := webhook.Options{
		TLSOpts: tlsOpts,
	}

	if len(flagsConfig.WebhookCertPath) > 0 {
		log.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", flagsConfig.WebhookCertPath,
			"webhook-cert-name", flagsConfig.WebhookCertName,
			"webhook-cert-key", flagsConfig.WebhookCertKey)

		webhookServerOptions.CertDir = flagsConfig.WebhookCertPath
		webhookServerOptions.CertName = flagsConfig.WebhookCertName
		webhookServerOptions.KeyName = flagsConfig.WebhookCertKey
	}

	return webhook.NewServer(webhookServerOptions)
}

func createMetricsServerOptions(flagsConfig Flags, tlsOpts []func(*tls.Config)) metricsserver.Options {
	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsOptions := metricsserver.Options{
		BindAddress:   flagsConfig.MetricsAddr,
		SecureServing: flagsConfig.SecureMetrics,
		TLSOpts:       tlsOpts,
	}

	if flagsConfig.SecureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(flagsConfig.MetricsCertPath) > 0 {
		log.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", flagsConfig.MetricsCertPath,
			"metrics-cert-name", flagsConfig.MetricsCertName,
			"metrics-cert-key", flagsConfig.MetricsCertKey)

		metricsOptions.CertDir = flagsConfig.MetricsCertPath
		metricsOptions.CertName = flagsConfig.MetricsCertName
		metricsOptions.KeyName = flagsConfig.MetricsCertKey
	}

	return metricsOptions
}
