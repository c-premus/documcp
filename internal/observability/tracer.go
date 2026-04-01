// Package observability provides OpenTelemetry tracing initialization,
// HTTP middleware for span creation, and slog integration for trace correlation.
package observability

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/c-premus/documcp/internal/config"
)

// InitTracer sets up the OpenTelemetry TracerProvider with an OTLP HTTP
// exporter. It returns a shutdown function that flushes remaining spans and
// releases resources. If cfg.Enabled is false, it returns a no-op shutdown.
//
// The sampler uses AlwaysSample as the root decision, ignoring upstream
// sampling decisions from reverse proxies (e.g., Traefik). This ensures
// DocuMCP always generates its own traces regardless of whether the proxy
// marked the request as unsampled. When SampleRate < 1.0, TraceIDRatioBased
// is used instead.
func InitTracer(ctx context.Context, cfg config.OTELConfig) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	// WithEndpoint expects host:port, WithEndpointURL expects a full URL.
	// Accept either format so OTEL_EXPORTER_OTLP_ENDPOINT works as both
	// "alloy:4318" and "http://alloy:4318" (the OTLP convention).
	var opts []otlptracehttp.Option
	if strings.HasPrefix(cfg.Endpoint, "http://") || strings.HasPrefix(cfg.Endpoint, "https://") {
		opts = append(opts, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	} else {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP HTTP exporter: %w", err)
	}

	// Build resource attributes for trace metadata.
	attrs := []resource.Option{
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	}
	if cfg.Version != "" {
		attrs = append(attrs, resource.WithAttributes(semconv.ServiceVersion(cfg.Version)))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, resource.WithAttributes(semconv.DeploymentEnvironment(cfg.Environment)))
	}

	res, err := resource.New(ctx, attrs...)
	if err != nil {
		return nil, fmt.Errorf("creating OTEL resource: %w", err)
	}

	// Use AlwaysSample by default. This is critical: the default
	// ParentBased(AlwaysSample) respects the parent's sampling decision,
	// which means if a reverse proxy (Traefik) sends traceparent with
	// sampled=0, DocuMCP would silently drop the trace. By using
	// AlwaysSample (not wrapped in ParentBased), we always record spans
	// regardless of upstream decisions.
	sampler := sdktrace.AlwaysSample()
	if cfg.SampleRate > 0 && cfg.SampleRate < 1.0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
