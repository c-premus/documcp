package observability

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/c-premus/documcp/internal/config"
)

// redactedPlaceholder is the literal string substituted for the value of any
// sensitive header, cookie, or breadcrumb data entry before the event leaves
// the process. The key is preserved so triage views still show that an auth
// header was present on the request without leaking its value.
const redactedPlaceholder = "[redacted]"

// sensitiveHeaders lists request header names whose values must be scrubbed
// before an event is transmitted to Sentry. Keys are stored in lowercase and
// matched case-insensitively because the Sentry SDK preserves the original
// casing from the source http.Request (Authorization, authorization, and
// AUTHORIZATION are all possible).
var sensitiveHeaders = map[string]struct{}{
	"authorization":       {},
	"cookie":              {},
	"set-cookie":          {},
	"x-api-key":           {},
	"proxy-authorization": {},
	"x-api-token":         {},
}

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
		SampleRate:       cfg.SampleRate,
		TracesSampleRate: 0, // OTEL handles tracing
		AttachStacktrace: true,
		BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			scrubSensitiveData(event)
			return event
		},
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

// scrubSensitiveData walks the event request and breadcrumbs, redacting
// sensitive header values and dropping request bodies so bearer tokens,
// session cookies, and internal API tokens never leave the process. It is
// safe to call with a nil event or a nil Request.
func scrubSensitiveData(event *sentry.Event) {
	if event == nil {
		return
	}
	if event.Request != nil {
		scrubHeaders(event.Request.Headers)
		event.Request.Data = ""
		event.Request.Cookies = ""
	}
	for _, bc := range event.Breadcrumbs {
		scrubBreadcrumbData(bc)
	}
}

// scrubHeaders replaces the value of any header in headers whose name
// (case-insensitively) appears in sensitiveHeaders with redactedPlaceholder.
// The key is preserved so the presence of the header is still visible in the
// Sentry UI. A nil or empty map is a no-op.
func scrubHeaders(headers map[string]string) {
	for k := range headers {
		if _, ok := sensitiveHeaders[strings.ToLower(k)]; ok {
			headers[k] = redactedPlaceholder
		}
	}
}

// scrubBreadcrumbData redacts sensitive entries in a breadcrumb's Data map
// using the same rules as scrubHeaders. A nil breadcrumb or nil Data map is a
// no-op. Non-sensitive keys are left untouched.
func scrubBreadcrumbData(bc *sentry.Breadcrumb) {
	if bc == nil {
		return
	}
	for k := range bc.Data {
		if _, ok := sensitiveHeaders[strings.ToLower(k)]; ok {
			bc.Data[k] = redactedPlaceholder
		}
	}
}
