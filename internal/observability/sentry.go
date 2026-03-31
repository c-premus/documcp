package observability

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/c-premus/documcp/internal/config"
)

// InitSentry initializes the Sentry SDK for error tracking. If cfg.DSN is
// empty, it returns a no-op flush function (Sentry remains disabled).
// The appEnv and appVersion parameters are used as fallbacks when the Sentry
// config does not specify environment or release explicitly.
func InitSentry(cfg config.SentryConfig, appEnv, appVersion string) (flush func(), err error) {
	if cfg.DSN == "" {
		return func() {}, nil
	}

	env := cfg.Environment
	if env == "" {
		env = appEnv
	}

	release := cfg.Release
	if release == "" {
		release = appVersion
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      env,
		Release:          release,
		SampleRate:        cfg.SampleRate,
		TracesSampleRate:  0, // OTEL handles tracing
		AttachStacktrace: true,
	}); err != nil {
		return nil, fmt.Errorf("initializing sentry: %w", err)
	}

	return func() {
		sentry.Flush(2 * time.Second)
	}, nil
}

// CaptureException reports an error to Sentry. It attempts to use a hub from
// the request context first, falling back to the current hub. When Sentry is
// not initialized (no DSN), this is a no-op.
func CaptureException(ctx context.Context, err error) {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.CaptureException(err)
	} else {
		sentry.CaptureException(err)
	}
}

// SetUser sets the Sentry user context for the current scope. When Sentry is
// not initialized, this is a no-op.
func SetUser(ctx context.Context, id int64, email string) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.Scope().SetUser(sentry.User{
		ID:    strconv.FormatInt(id, 10),
		Email: email,
	})
}
