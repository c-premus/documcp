package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// MetricsMiddleware returns HTTP middleware that records request duration,
// increments the request counter, and tracks active connections using the
// provided Metrics collectors.
//
// It uses chi's RouteContext to extract the matched route pattern for the
// "route" label, which keeps cardinality bounded (no per-path explosion).
func MetricsMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			m.HTTPActiveConnections.Inc()
			defer m.HTTPActiveConnections.Dec()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.Status())
			route := routePattern(r)

			m.HTTPRequestDuration.WithLabelValues(r.Method, route, status).Observe(duration)
			m.HTTPRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		})
	}
}

// routePattern extracts the matched route pattern from chi's RouteContext.
// If no pattern is available (e.g., 404 or middleware-only paths), it falls
// back to "unmatched" to avoid high-cardinality label values.
func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return "unmatched"
	}

	pattern := rctx.RoutePattern()
	if pattern == "" {
		return "unmatched"
	}

	return pattern
}
