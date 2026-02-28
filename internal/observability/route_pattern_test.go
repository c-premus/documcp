package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRoutePattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func() *http.Request
		want    string
	}{
		{
			name: "returns unmatched when no chi RouteContext exists",
			setup: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/some/path", nil)
			},
			want: "unmatched",
		},
		{
			name: "returns unmatched when chi RouteContext has empty pattern",
			setup: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
				rctx := chi.NewRouteContext()
				// RoutePattern() returns "" by default on a fresh RouteContext.
				ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
				return req.WithContext(ctx)
			},
			want: "unmatched",
		},
		{
			name: "returns pattern when chi RouteContext has a pattern",
			setup: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/documents/abc-123", nil)
				rctx := chi.NewRouteContext()
				rctx.RoutePatterns = []string{"/api/documents/{uuid}"}
				ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
				return req.WithContext(ctx)
			},
			want: "/api/documents/{uuid}",
		},
		{
			name: "returns concatenated pattern for nested routes",
			setup: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42", nil)
				rctx := chi.NewRouteContext()
				rctx.RoutePatterns = []string{"/api/v1/*", "/users/{id}"}
				ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
				return req.WithContext(ctx)
			},
			want: "/api/v1/users/{id}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := tt.setup()
			got := routePattern(req)

			if got != tt.want {
				t.Errorf("routePattern() = %q, want %q", got, tt.want)
			}
		})
	}
}
