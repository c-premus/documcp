package observability_test

import (
	"context"
	"testing"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/observability"
)

func TestInitTracer_Disabled(t *testing.T) {
	t.Parallel()

	cfg := config.OTELConfig{
		Enabled: false,
	}

	shutdown, err := observability.InitTracer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("InitTracer() with disabled config returned error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("InitTracer() returned nil shutdown function")
	}

	// Calling shutdown on a disabled tracer should succeed without error.
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown() returned error: %v", err)
	}
}

func TestInitTracer_DisabledReturnsNoOpShutdown(t *testing.T) {
	t.Parallel()

	cfg := config.OTELConfig{
		Enabled: false,
	}

	shutdown, err := observability.InitTracer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("InitTracer() unexpected error: %v", err)
	}

	// Calling shutdown multiple times should be safe.
	for i := range 3 {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown() call %d returned error: %v", i, err)
		}
	}
}

func TestInitTracer_DisabledIgnoresEndpoint(t *testing.T) {
	t.Parallel()

	// Even with an endpoint configured, disabled should return no-op.
	cfg := config.OTELConfig{
		Enabled:     false,
		Endpoint:    "http://nonexistent:4318",
		ServiceName: "test-service",
		Insecure:    true,
	}

	shutdown, err := observability.InitTracer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("InitTracer() unexpected error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("InitTracer() returned nil shutdown function")
	}
}

func TestInitTracer_DisabledIgnoresNewFields(t *testing.T) {
	t.Parallel()

	cfg := config.OTELConfig{
		Enabled:     false,
		SampleRate:  0.5,
		Environment: "production",
		Version:     "v1.0.0",
	}

	shutdown, err := observability.InitTracer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("InitTracer() unexpected error: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown() returned error: %v", err)
	}
}

func TestInitTracer_EndpointURLFormat(t *testing.T) {
	t.Parallel()

	// Both URL and host:port formats should initialize without error.
	// The exporter connects lazily, so these succeed even with unreachable hosts.
	tests := []struct {
		name     string
		endpoint string
		insecure bool
	}{
		{"bare host:port", "localhost:4318", true},
		{"http URL", "http://alloy:4318", false},
		{"https URL", "https://otel.example.com:4318", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.OTELConfig{
				Enabled:     true,
				Endpoint:    tt.endpoint,
				Insecure:    tt.insecure,
				ServiceName: "test",
			}

			shutdown, err := observability.InitTracer(context.Background(), cfg)
			if err != nil {
				t.Fatalf("InitTracer(%q) unexpected error: %v", tt.endpoint, err)
			}

			if err := shutdown(context.Background()); err != nil {
				t.Errorf("shutdown() returned error: %v", err)
			}
		})
	}
}
