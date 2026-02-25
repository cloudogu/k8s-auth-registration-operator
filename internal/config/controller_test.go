package config

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func TestCreateTLSOptions(t *testing.T) {
	t.Run("should add HTTP/2 disable option by default", func(t *testing.T) {
		tlsOpts := createTLSOptions(Flags{EnableHTTP2: false})
		require.Len(t, tlsOpts, 1)

		tlsConfig := &tls.Config{NextProtos: []string{"h2"}}
		tlsOpts[0](tlsConfig)

		assert.Equal(t, []string{"http/1.1"}, tlsConfig.NextProtos)
	})

	t.Run("should not add TLS option when HTTP/2 is enabled", func(t *testing.T) {
		tlsOpts := createTLSOptions(Flags{EnableHTTP2: true})
		assert.Empty(t, tlsOpts)
	})
}

func TestCreateWebhookServer(t *testing.T) {
	tlsOpts := createTLSOptions(Flags{EnableHTTP2: false})
	flagsConfig := Flags{
		WebhookCertPath: "/tmp/webhook-certs",
		WebhookCertName: "webhook.crt",
		WebhookCertKey:  "webhook.key",
	}

	server := createWebhookServer(flagsConfig, tlsOpts)

	defaultServer, ok := server.(*webhook.DefaultServer)
	require.Truef(t, ok, "expected webhook server to be *webhook.DefaultServer, got %T", server)
	assert.Equal(t, "/tmp/webhook-certs", defaultServer.Options.CertDir)
	assert.Equal(t, "webhook.crt", defaultServer.Options.CertName)
	assert.Equal(t, "webhook.key", defaultServer.Options.KeyName)
	assert.Len(t, defaultServer.Options.TLSOpts, 1)
}

func TestCreateMetricsServerOptions(t *testing.T) {
	t.Run("should configure secure metrics including filter provider and certs", func(t *testing.T) {
		tlsOpts := createTLSOptions(Flags{EnableHTTP2: false})
		flagsConfig := Flags{
			MetricsAddr:     ":8443",
			SecureMetrics:   true,
			MetricsCertPath: "/tmp/metrics-certs",
			MetricsCertName: "metrics.crt",
			MetricsCertKey:  "metrics.key",
		}

		opts := createMetricsServerOptions(flagsConfig, tlsOpts)

		assert.Equal(t, ":8443", opts.BindAddress)
		assert.True(t, opts.SecureServing)
		assert.NotNil(t, opts.FilterProvider)
		assert.Equal(t, "/tmp/metrics-certs", opts.CertDir)
		assert.Equal(t, "metrics.crt", opts.CertName)
		assert.Equal(t, "metrics.key", opts.KeyName)
		assert.Len(t, opts.TLSOpts, 1)
	})

	t.Run("should not configure auth filter for insecure metrics", func(t *testing.T) {
		opts := createMetricsServerOptions(Flags{
			MetricsAddr:   ":8080",
			SecureMetrics: false,
		}, nil)

		assert.Equal(t, ":8080", opts.BindAddress)
		assert.False(t, opts.SecureServing)
		assert.Nil(t, opts.FilterProvider)
		assert.Empty(t, opts.CertDir)
		assert.Empty(t, opts.CertName)
		assert.Empty(t, opts.KeyName)
	})
}

func TestGetControllerOptions(t *testing.T) {
	resetFlagStateForTest(t, []string{
		"--metrics-bind-address=:9443",
		"--health-probe-bind-address=:18081",
		"--leader-elect=true",
		"--metrics-secure=false",
		"--webhook-cert-path=/tmp/webhook-certs",
		"--webhook-cert-name=webhook.crt",
		"--webhook-cert-key=webhook.key",
		"--metrics-cert-path=/tmp/metrics-certs",
		"--metrics-cert-name=metrics.crt",
		"--metrics-cert-key=metrics.key",
		"--enable-http2=true",
	})

	scheme := runtime.NewScheme()
	namespace := "ecosystem"

	options := getControllerOptions(scheme, namespace)

	assert.Same(t, scheme, options.Scheme)
	assert.Equal(t, ":18081", options.HealthProbeBindAddress)
	assert.True(t, options.LeaderElection)
	assert.Equal(t, "ec33e801.k8s.cloudogu.com", options.LeaderElectionID)
	_, exists := options.Cache.DefaultNamespaces[namespace]
	assert.Truef(t, exists, "expected namespace %q to be configured in cache options", namespace)

	assert.Equal(t, ":9443", options.Metrics.BindAddress)
	assert.False(t, options.Metrics.SecureServing)
	assert.Nil(t, options.Metrics.FilterProvider)
	assert.Empty(t, options.Metrics.TLSOpts)
	assert.Equal(t, "/tmp/metrics-certs", options.Metrics.CertDir)
	assert.Equal(t, "metrics.crt", options.Metrics.CertName)
	assert.Equal(t, "metrics.key", options.Metrics.KeyName)

	webhookServer, ok := options.WebhookServer.(*webhook.DefaultServer)
	require.Truef(t, ok, "expected WebhookServer to be *webhook.DefaultServer, got %T", options.WebhookServer)
	assert.Equal(t, "/tmp/webhook-certs", webhookServer.Options.CertDir)
	assert.Equal(t, "webhook.crt", webhookServer.Options.CertName)
	assert.Equal(t, "webhook.key", webhookServer.Options.KeyName)
	assert.Empty(t, webhookServer.Options.TLSOpts)
}
