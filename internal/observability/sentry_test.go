package observability_test

import (
	"context"
	"errors"
	"testing"

	"github.com/getsentry/sentry-go"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/observability"
)

func TestInitSentry_EmptyDSN(t *testing.T) {
	t.Parallel()

	cfg := config.SentryConfig{DSN: ""}

	flush, err := observability.InitSentry(cfg, "development", "1.0.0")
	if err != nil {
		t.Fatalf("InitSentry() returned error: %v", err)
	}
	if flush == nil {
		t.Fatal("InitSentry() returned nil flush function")
	}

	// Calling flush on disabled Sentry should be safe.
	flush()
}

func TestInitSentry_InvalidDSN(t *testing.T) {
	t.Parallel()

	cfg := config.SentryConfig{
		DSN:        "not-a-valid-dsn",
		SampleRate: 1.0,
	}

	_, err := observability.InitSentry(cfg, "development", "1.0.0")
	if err == nil {
		t.Fatal("InitSentry() expected error for invalid DSN, got nil")
	}
}

func TestInitSentry_ValidDSN(t *testing.T) {
	// Use a syntactically valid DSN pointing to a non-existent host.
	// sentry.Init() validates the DSN format but does not connect.
	cfg := config.SentryConfig{
		DSN:         "https://key@errors.example.com/1",
		Environment: "testing",
		Release:     "v0.1.0",
		SampleRate:  1.0,
	}

	flush, err := observability.InitSentry(cfg, "development", "1.0.0")
	if err != nil {
		t.Fatalf("InitSentry() returned error: %v", err)
	}
	if flush == nil {
		t.Fatal("InitSentry() returned nil flush function")
	}
	flush()
}

func TestInitSentry_FallbackEnvironmentAndRelease(t *testing.T) {
	t.Parallel()

	cfg := config.SentryConfig{
		DSN:        "https://key@errors.example.com/1",
		SampleRate: 1.0,
		// Environment and Release left empty — should fall back to args.
	}

	flush, err := observability.InitSentry(cfg, "staging", "v2.0.0")
	if err != nil {
		t.Fatalf("InitSentry() returned error: %v", err)
	}
	flush()
}

func TestCaptureException_NoHub(t *testing.T) {
	t.Parallel()

	// CaptureException with a plain context (no hub) should not panic.
	observability.CaptureException(context.Background(), errors.New("test error"))
}

func TestCaptureException_WithHub(t *testing.T) {
	t.Parallel()

	hub := sentry.NewHub(nil, sentry.NewScope())
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	// Should use the hub from context and not panic.
	observability.CaptureException(ctx, errors.New("test error with hub"))
}

func TestSetUser_NoHub(t *testing.T) {
	t.Parallel()

	// SetUser with a plain context should not panic.
	observability.SetUser(context.Background(), 42, "user@example.com")
}

func TestSetUser_WithHub(t *testing.T) {
	t.Parallel()

	hub := sentry.NewHub(nil, sentry.NewScope())
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	// Should use the hub from context and not panic.
	observability.SetUser(ctx, 99, "admin@example.com")
}
