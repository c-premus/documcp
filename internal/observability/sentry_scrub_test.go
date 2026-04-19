package observability

import (
	"maps"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestScrubSensitiveData_MixedCaseAuthorizationHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
	}{
		{"titlecase", "Authorization"},
		{"lowercase", "authorization"},
		{"uppercase", "AUTHORIZATION"},
		{"mixedcase", "AuThOrIzAtIoN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			event := &sentry.Event{
				Request: &sentry.Request{
					Headers: map[string]string{
						tt.header: "Bearer secret-token",
					},
				},
			}

			scrubSensitiveData(event)

			got := event.Request.Headers[tt.header]
			if got != redactedPlaceholder {
				t.Fatalf("%s header = %q, want %q", tt.header, got, redactedPlaceholder)
			}
		})
	}
}

func TestScrubSensitiveData_AllSensitiveHeaders(t *testing.T) {
	t.Parallel()

	headers := map[string]string{
		"Authorization":       "Bearer xyz",
		"Cookie":              "session=abc",
		"Set-Cookie":          "session=abc; HttpOnly",
		"X-Api-Key":           "k1",
		"Proxy-Authorization": "Basic YWxhZGRpbjpvcGVuc2VzYW1l",
		"X-Api-Token":         "internal-token",
	}
	event := &sentry.Event{Request: &sentry.Request{Headers: headers}}

	scrubSensitiveData(event)

	for k, v := range event.Request.Headers {
		if v != redactedPlaceholder {
			t.Errorf("header %q = %q, want %q", k, v, redactedPlaceholder)
		}
	}
}

func TestScrubSensitiveData_UnrelatedHeadersPassThrough(t *testing.T) {
	t.Parallel()

	original := map[string]string{
		"User-Agent":      "curl/8.0",
		"Accept":          "application/json",
		"Content-Type":    "application/json",
		"X-Forwarded-For": "10.0.0.1",
	}

	// Copy so we can compare against originals after scrub.
	headers := make(map[string]string, len(original))
	maps.Copy(headers, original)

	event := &sentry.Event{Request: &sentry.Request{Headers: headers}}
	scrubSensitiveData(event)

	for k, want := range original {
		if got := event.Request.Headers[k]; got != want {
			t.Errorf("header %q = %q, want %q (should not be redacted)", k, got, want)
		}
	}
}

func TestScrubSensitiveData_ClearsRequestBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"small json", `{"password":"hunter2"}`},
		{"large", strings.Repeat("x", 10_000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			event := &sentry.Event{Request: &sentry.Request{Data: tt.body}}
			scrubSensitiveData(event)
			if event.Request.Data != "" {
				t.Fatalf("Request.Data = %q, want empty", event.Request.Data)
			}
		})
	}
}

func TestScrubSensitiveData_ClearsCookiesString(t *testing.T) {
	t.Parallel()

	event := &sentry.Event{
		Request: &sentry.Request{Cookies: "session=abc; csrf=xyz"},
	}
	scrubSensitiveData(event)
	if event.Request.Cookies != "" {
		t.Fatalf("Request.Cookies = %q, want empty", event.Request.Cookies)
	}
}

func TestScrubSensitiveData_BreadcrumbDataRedacted(t *testing.T) {
	t.Parallel()

	event := &sentry.Event{
		Breadcrumbs: []*sentry.Breadcrumb{
			{
				Category: "http",
				Data: map[string]any{
					"authorization": "Bearer secret",
					"url":           "https://example.com/api",
				},
			},
		},
	}

	scrubSensitiveData(event)

	bc := event.Breadcrumbs[0]
	if got := bc.Data["authorization"]; got != redactedPlaceholder {
		t.Errorf("breadcrumb authorization = %v, want %q", got, redactedPlaceholder)
	}
	if got := bc.Data["url"]; got != "https://example.com/api" {
		t.Errorf("breadcrumb url = %v, want unchanged", got)
	}
}

func TestScrubSensitiveData_NilEvent(t *testing.T) {
	t.Parallel()

	// Must not panic.
	scrubSensitiveData(nil)
}

func TestScrubSensitiveData_NilRequest(t *testing.T) {
	t.Parallel()

	event := &sentry.Event{
		Request: nil,
		Breadcrumbs: []*sentry.Breadcrumb{
			{Data: map[string]any{"Cookie": "session=abc", "path": "/foo"}},
		},
	}

	scrubSensitiveData(event)

	// Breadcrumbs should still be scrubbed even with a nil Request.
	bc := event.Breadcrumbs[0]
	if got := bc.Data["Cookie"]; got != redactedPlaceholder {
		t.Errorf("breadcrumb Cookie = %v, want %q", got, redactedPlaceholder)
	}
	if got := bc.Data["path"]; got != "/foo" {
		t.Errorf("breadcrumb path = %v, want unchanged", got)
	}
}

func TestScrubSensitiveData_EmptyBreadcrumbs(t *testing.T) {
	t.Parallel()

	event := &sentry.Event{
		Request:     &sentry.Request{Headers: map[string]string{"Authorization": "Bearer x"}},
		Breadcrumbs: []*sentry.Breadcrumb{},
	}

	// Must not panic; headers still scrubbed.
	scrubSensitiveData(event)

	if got := event.Request.Headers["Authorization"]; got != redactedPlaceholder {
		t.Errorf("Authorization = %q, want %q", got, redactedPlaceholder)
	}
}

func TestScrubSensitiveData_NilBreadcrumbEntry(t *testing.T) {
	t.Parallel()

	event := &sentry.Event{
		Breadcrumbs: []*sentry.Breadcrumb{nil},
	}

	// Must not panic on a nil breadcrumb pointer.
	scrubSensitiveData(event)
}
