package observability

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracing returns HTTP middleware that creates a span for each incoming request.
// It records standard HTTP semantic convention attributes, sets span status on
// server errors, and propagates the trace context to downstream handlers.
func Tracing(tracerName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip tracing for health and metrics probe endpoints.
			// These generate high-volume, low-value spans that drown out
			// actual API traffic in trace backends.
			if strings.HasPrefix(r.URL.Path, "/health") || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			tracer := otel.Tracer(tracerName)
			propagator := otel.GetTextMapPropagator()

			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			span.SetAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.URLPath(r.URL.Path),
			)

			if r.ContentLength > 0 {
				span.SetAttributes(
					semconv.HTTPRequestBodySize(int(r.ContentLength)),
				)
			}

			// Inject trace context into response headers so downstream
			// consumers (browser dev tools, Traefik) can correlate.
			propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))

			statusCode := ww.Status()
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

			span.SetAttributes(
				semconv.HTTPResponseStatusCode(statusCode),
				attribute.Int("http.response_content_length", ww.BytesWritten()),
			)

			// Mark server errors (5xx) on the span so trace backends
			// surface them in error dashboards and alerting.
			if statusCode >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
			}

			// Update the span name with the matched route pattern for better
			// grouping in trace backends (reduces cardinality from raw paths).
			if rctx := chi.RouteContext(ctx); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					span.SetName(fmt.Sprintf("%s %s", r.Method, pattern))
					span.SetAttributes(semconv.HTTPRoute(pattern))
				}
			}
		})
	}
}
