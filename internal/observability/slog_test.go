package observability_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/c-premus/documcp/internal/observability"
)

func TestNewTracedHandler(t *testing.T) {
	t.Parallel()

	inner := slog.NewTextHandler(&bytes.Buffer{}, nil)
	handler := observability.NewTracedHandler(inner)

	if handler == nil {
		t.Fatal("NewTracedHandler returned nil")
	}

	// Verify it implements slog.Handler.
	var _ slog.Handler = handler //nolint:staticcheck // explicit interface check
}

func TestTracedHandler_Enabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		min   slog.Level
		want  bool
	}{
		{
			name:  "returns true when level meets minimum",
			level: slog.LevelInfo,
			min:   slog.LevelInfo,
			want:  true,
		},
		{
			name:  "returns true when level exceeds minimum",
			level: slog.LevelError,
			min:   slog.LevelInfo,
			want:  true,
		},
		{
			name:  "returns false when level below minimum",
			level: slog.LevelDebug,
			min:   slog.LevelInfo,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inner := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{
				Level: tt.min,
			})
			handler := observability.NewTracedHandler(inner)

			got := handler.Enabled(context.Background(), tt.level)
			if got != tt.want {
				t.Errorf("Enabled(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestTracedHandler_Handle_WithoutTraceContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := observability.NewTracedHandler(inner)

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test message")

	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Errorf("output should contain 'test message', got: %s", output)
	}

	// Without a valid span context, trace_id and span_id should NOT be present.
	if strings.Contains(output, "trace_id") {
		t.Errorf("output should not contain trace_id without span context, got: %s", output)
	}
	if strings.Contains(output, "span_id") {
		t.Errorf("output should not contain span_id without span context, got: %s", output)
	}
}

func TestTracedHandler_Handle_WithTraceContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := observability.NewTracedHandler(inner)

	// Create a valid span context with known trace and span IDs.
	traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	logger := slog.New(handler)
	logger.InfoContext(ctx, "traced message")

	output := buf.String()

	if !strings.Contains(output, "traced message") {
		t.Errorf("output should contain 'traced message', got: %s", output)
	}

	if !strings.Contains(output, "trace_id="+traceID.String()) {
		t.Errorf("output should contain trace_id=%s, got: %s", traceID.String(), output)
	}

	if !strings.Contains(output, "span_id="+spanID.String()) {
		t.Errorf("output should contain span_id=%s, got: %s", spanID.String(), output)
	}
}

func TestTracedHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := observability.NewTracedHandler(inner)

	// WithAttrs should return a new handler that includes the attributes.
	withAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("component", "test"),
	})

	if withAttrs == nil {
		t.Fatal("WithAttrs returned nil")
	}

	// The returned handler should be a different instance.
	if withAttrs == handler {
		t.Error("WithAttrs should return a new handler, not the same instance")
	}

	// Verify the attribute appears in output.
	logger := slog.New(withAttrs)
	logger.Info("attrs message")

	output := buf.String()
	if !strings.Contains(output, "component=test") {
		t.Errorf("output should contain 'component=test', got: %s", output)
	}
}

func TestTracedHandler_WithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := observability.NewTracedHandler(inner)

	// WithGroup should return a new handler that groups subsequent attributes.
	withGroup := handler.WithGroup("mygroup")

	if withGroup == nil {
		t.Fatal("WithGroup returned nil")
	}

	// The returned handler should be a different instance.
	if withGroup == handler {
		t.Error("WithGroup should return a new handler, not the same instance")
	}

	// Verify the group appears in output.
	logger := slog.New(withGroup)
	logger.Info("group message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "mygroup.key=value") {
		t.Errorf("output should contain 'mygroup.key=value', got: %s", output)
	}
}

func TestTracedHandler_WithAttrs_PreservesTracing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := observability.NewTracedHandler(inner)

	withAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("service", "documcp"),
	})

	// Create a valid span context.
	traceID := trace.TraceID{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160}
	spanID := trace.SpanID{10, 20, 30, 40, 50, 60, 70, 80}
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	logger := slog.New(withAttrs)
	logger.InfoContext(ctx, "traced with attrs")

	output := buf.String()

	// Should have both the pre-set attribute and trace IDs.
	if !strings.Contains(output, "service=documcp") {
		t.Errorf("output should contain 'service=documcp', got: %s", output)
	}
	if !strings.Contains(output, "trace_id="+traceID.String()) {
		t.Errorf("output should contain trace_id, got: %s", output)
	}
	if !strings.Contains(output, "span_id="+spanID.String()) {
		t.Errorf("output should contain span_id, got: %s", output)
	}
}

func TestTracedHandler_WithGroup_PreservesTracing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := observability.NewTracedHandler(inner)

	withGroup := handler.WithGroup("request")

	// Create a valid span context.
	traceID := trace.TraceID{0xAA, 0xBB, 0xCC, 0xDD, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	spanID := trace.SpanID{0xEE, 0xFF, 0, 0, 0, 0, 0, 1}
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanCtx)

	logger := slog.New(withGroup)
	logger.InfoContext(ctx, "grouped traced", "method", "GET")

	output := buf.String()

	if !strings.Contains(output, "request.method=GET") {
		t.Errorf("output should contain 'request.method=GET', got: %s", output)
	}
	if !strings.Contains(output, "trace_id="+traceID.String()) {
		t.Errorf("output should contain trace_id, got: %s", output)
	}
}
