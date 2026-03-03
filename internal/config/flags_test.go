package config

import (
	"flag"
	"io"
	"os"
	"testing"
)

func TestParseFlags_DefaultValues(t *testing.T) {
	t.Run("returns default values when no command line flags are provided", func(t *testing.T) {
		resetFlagStateForTest(t, nil)

		cfg := parseFlags()

		if cfg.MetricsAddr != "0" {
			t.Fatalf("expected MetricsAddr to be %q, got %q", "0", cfg.MetricsAddr)
		}
		if cfg.ProbeAddr != ":8081" {
			t.Fatalf("expected ProbeAddr to be %q, got %q", ":8081", cfg.ProbeAddr)
		}
		if cfg.EnableLeaderElection {
			t.Fatalf("expected EnableLeaderElection to be false")
		}
		if !cfg.SecureMetrics {
			t.Fatalf("expected SecureMetrics to be true")
		}
		if cfg.WebhookCertPath != "" {
			t.Fatalf("expected WebhookCertPath to be empty, got %q", cfg.WebhookCertPath)
		}
		if cfg.WebhookCertName != "tls.crt" {
			t.Fatalf("expected WebhookCertName to be %q, got %q", "tls.crt", cfg.WebhookCertName)
		}
		if cfg.WebhookCertKey != "tls.key" {
			t.Fatalf("expected WebhookCertKey to be %q, got %q", "tls.key", cfg.WebhookCertKey)
		}
		if cfg.MetricsCertPath != "" {
			t.Fatalf("expected MetricsCertPath to be empty, got %q", cfg.MetricsCertPath)
		}
		if cfg.MetricsCertName != "tls.crt" {
			t.Fatalf("expected MetricsCertName to be %q, got %q", "tls.crt", cfg.MetricsCertName)
		}
		if cfg.MetricsCertKey != "tls.key" {
			t.Fatalf("expected MetricsCertKey to be %q, got %q", "tls.key", cfg.MetricsCertKey)
		}
		if cfg.EnableHTTP2 {
			t.Fatalf("expected EnableHTTP2 to be false")
		}
	})
}

func TestParseFlags_UsesProvidedValues(t *testing.T) {
	t.Run("returns overridden values when command line flags are provided", func(t *testing.T) {
		resetFlagStateForTest(t, []string{
			"--metrics-bind-address=:8443",
			"--health-probe-bind-address=:18081",
			"--leader-elect=true",
			"--metrics-secure=false",
			"--webhook-cert-path=/tmp/webhook",
			"--webhook-cert-name=custom-webhook.crt",
			"--webhook-cert-key=custom-webhook.key",
			"--metrics-cert-path=/tmp/metrics",
			"--metrics-cert-name=custom-metrics.crt",
			"--metrics-cert-key=custom-metrics.key",
			"--enable-http2=true",
		})

		cfg := parseFlags()

		if cfg.MetricsAddr != ":8443" {
			t.Fatalf("expected MetricsAddr to be %q, got %q", ":8443", cfg.MetricsAddr)
		}
		if cfg.ProbeAddr != ":18081" {
			t.Fatalf("expected ProbeAddr to be %q, got %q", ":18081", cfg.ProbeAddr)
		}
		if !cfg.EnableLeaderElection {
			t.Fatalf("expected EnableLeaderElection to be true")
		}
		if cfg.SecureMetrics {
			t.Fatalf("expected SecureMetrics to be false")
		}
		if cfg.WebhookCertPath != "/tmp/webhook" {
			t.Fatalf("expected WebhookCertPath to be %q, got %q", "/tmp/webhook", cfg.WebhookCertPath)
		}
		if cfg.WebhookCertName != "custom-webhook.crt" {
			t.Fatalf("expected WebhookCertName to be %q, got %q", "custom-webhook.crt", cfg.WebhookCertName)
		}
		if cfg.WebhookCertKey != "custom-webhook.key" {
			t.Fatalf("expected WebhookCertKey to be %q, got %q", "custom-webhook.key", cfg.WebhookCertKey)
		}
		if cfg.MetricsCertPath != "/tmp/metrics" {
			t.Fatalf("expected MetricsCertPath to be %q, got %q", "/tmp/metrics", cfg.MetricsCertPath)
		}
		if cfg.MetricsCertName != "custom-metrics.crt" {
			t.Fatalf("expected MetricsCertName to be %q, got %q", "custom-metrics.crt", cfg.MetricsCertName)
		}
		if cfg.MetricsCertKey != "custom-metrics.key" {
			t.Fatalf("expected MetricsCertKey to be %q, got %q", "custom-metrics.key", cfg.MetricsCertKey)
		}
		if !cfg.EnableHTTP2 {
			t.Fatalf("expected EnableHTTP2 to be true")
		}
	})
}

func resetFlagStateForTest(t *testing.T, args []string) {
	t.Helper()

	oldCommandLine := flag.CommandLine
	oldArgs := os.Args

	testFlagSet := flag.NewFlagSet("config-flags-test", flag.ContinueOnError)
	testFlagSet.SetOutput(io.Discard)
	flag.CommandLine = testFlagSet

	os.Args = append([]string{"config-flags-test"}, args...)

	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
		os.Args = oldArgs
	})
}
