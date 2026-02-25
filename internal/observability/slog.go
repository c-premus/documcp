package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// tracedHandler wraps a slog.Handler to inject trace_id and span_id attributes
// from the context into every log record.
type tracedHandler struct {
	inner slog.Handler
}

// NewTracedHandler returns a slog.Handler that extracts the active span's trace
// ID and span ID from the context and appends them as "trace_id" and "span_id"
// attributes to each log record.
func NewTracedHandler(inner slog.Handler) slog.Handler {
	return &tracedHandler{inner: inner}
}

// Enabled delegates to the inner handler.
func (h *tracedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle extracts trace context and adds trace_id/span_id attributes before
// delegating to the inner handler.
func (h *tracedHandler) Handle(ctx context.Context, record slog.Record) error {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}

	return h.inner.Handle(ctx, record)
}

// WithAttrs returns a new tracedHandler wrapping the inner handler with the
// given attributes.
func (h *tracedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &tracedHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup returns a new tracedHandler wrapping the inner handler with the
// given group name.
func (h *tracedHandler) WithGroup(name string) slog.Handler {
	return &tracedHandler{inner: h.inner.WithGroup(name)}
}
