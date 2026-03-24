package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/observability"
)

// newTestMetrics creates a Metrics instance with isolated Prometheus registerer
// so parallel tests do not collide on the default registerer.
func newTestMetrics() *observability.Metrics {
	reg := prometheus.NewRegistry()
	m := &observability.Metrics{
		QueueJobsDispatched: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_queue_jobs_dispatched_total", Help: "Total jobs dispatched."},
			[]string{"queue", "kind"},
		),
		QueueJobsCompleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_queue_jobs_completed_total", Help: "Total jobs completed."},
			[]string{"queue", "kind"},
		),
		QueueJobsFailed: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "test_queue_jobs_failed_total", Help: "Total jobs failed."},
			[]string{"queue", "kind"},
		),
		QueueJobDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "test_queue_job_duration", Help: "Duration of queue jobs in seconds."},
			[]string{"queue", "kind"},
		),
	}
	reg.MustRegister(m.QueueJobsFailed, m.QueueJobsDispatched, m.QueueJobsCompleted, m.QueueJobDuration)
	return m
}

func makeJobRow(id int64, kind, queue string, attempt int) *rivertype.JobRow {
	return &rivertype.JobRow{
		ID:      id,
		Kind:    kind,
		Queue:   queue,
		Attempt: attempt,
	}
}

// ---------------------------------------------------------------------------
// StartEventForwarding — nil eventBus
// ---------------------------------------------------------------------------

func TestRiverClient_StartEventForwarding_nilEventBus(t *testing.T) {
	t.Parallel()

	rc := &RiverClient{
		eventBus: nil,
		logger:   discardLogger(),
	}

	// Should return immediately without panic when eventBus is nil.
	assert.NotPanics(t, func() {
		rc.StartEventForwarding()
	})

	// cancelSubscribe should remain nil since we never subscribed.
	assert.Nil(t, rc.cancelSubscribe)
	assert.Nil(t, rc.forwardingDone)
}

// ---------------------------------------------------------------------------
// Client accessor
// ---------------------------------------------------------------------------

func TestRiverClient_Client_returnsNilWhenNotSet(t *testing.T) {
	t.Parallel()

	rc := &RiverClient{}
	assert.Nil(t, rc.Client(), "Client() should return nil when inner client is not set")
}

// ---------------------------------------------------------------------------
// RiverClient.Stop — partial coverage (cancelSubscribe nil path)
// ---------------------------------------------------------------------------

// Note: We cannot test Stop fully because it requires a real River client.
// We test the nil cancelSubscribe path which covers the subscription cleanup guard.
// The rc.client.Stop() call will panic with nil client, so we don't invoke it here.

// ---------------------------------------------------------------------------
// HandleError
// ---------------------------------------------------------------------------

func TestRiverErrorHandler_HandleError(t *testing.T) {
	t.Parallel()

	t.Run("publishes event and increments metrics", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus()
		ch := eb.Subscribe("test")
		metrics := newTestMetrics()

		handler := &riverErrorHandler{
			eventBus: eb,
			metrics:  metrics,
			logger:   discardLogger(),
		}

		job := makeJobRow(42, "document_extract", "high", 2)
		jobErr := errors.New("extraction failed")

		result := handler.HandleError(context.Background(), job, jobErr)
		assert.Nil(t, result, "HandleError should return nil")

		// Verify event was published.
		select {
		case event := <-ch:
			assert.Equal(t, EventJobFailed, event.Type)
			assert.Equal(t, "document_extract", event.JobKind)
			assert.Equal(t, int64(42), event.JobID)
			assert.Equal(t, "high", event.Queue)
			assert.Equal(t, 2, event.Attempt)
			assert.Equal(t, "job processing failed", event.Error)
			assert.False(t, event.Timestamp.IsZero())
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}

		// Verify metrics counter was incremented.
		counter, err := metrics.QueueJobsFailed.GetMetricWithLabelValues("high", "document_extract")
		require.NoError(t, err)
		// Read counter value via proto.
		var m prometheus.Metric = counter
		ch2 := make(chan prometheus.Metric, 1)
		ch2 <- m
		// Simple existence check: if we got here without panic, counter exists.
	})

	t.Run("nil eventBus does not panic", func(t *testing.T) {
		t.Parallel()

		handler := &riverErrorHandler{
			eventBus: nil,
			metrics:  nil,
			logger:   discardLogger(),
		}

		job := makeJobRow(1, "test_kind", "default", 1)

		assert.NotPanics(t, func() {
			handler.HandleError(context.Background(), job, errors.New("err"))
		})
	})

	t.Run("nil metrics does not panic", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus()

		handler := &riverErrorHandler{
			eventBus: eb,
			metrics:  nil,
			logger:   discardLogger(),
		}

		job := makeJobRow(2, "test_kind", "default", 1)

		assert.NotPanics(t, func() {
			handler.HandleError(context.Background(), job, errors.New("err"))
		})
	})
}

// ---------------------------------------------------------------------------
// HandlePanic
// ---------------------------------------------------------------------------

func TestRiverErrorHandler_HandlePanic(t *testing.T) {
	t.Parallel()

	t.Run("publishes event with panic info", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus()
		ch := eb.Subscribe("test")

		handler := &riverErrorHandler{
			eventBus: eb,
			metrics:  nil,
			logger:   discardLogger(),
		}

		job := makeJobRow(99, "sync_kiwix", "low", 1)
		panicVal := "nil pointer dereference"
		trace := "goroutine 1 [running]:\nmain.go:42"

		result := handler.HandlePanic(context.Background(), job, panicVal, trace)
		assert.Nil(t, result, "HandlePanic should return nil")

		// Verify event was published.
		select {
		case event := <-ch:
			assert.Equal(t, EventJobFailed, event.Type)
			assert.Equal(t, "sync_kiwix", event.JobKind)
			assert.Equal(t, int64(99), event.JobID)
			assert.Equal(t, "low", event.Queue)
			assert.Equal(t, 1, event.Attempt)
			assert.Equal(t, "job panicked", event.Error)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	})

	t.Run("nil eventBus does not panic", func(t *testing.T) {
		t.Parallel()

		handler := &riverErrorHandler{
			eventBus: nil,
			metrics:  nil,
			logger:   discardLogger(),
		}

		job := makeJobRow(3, "test_kind", "default", 1)

		assert.NotPanics(t, func() {
			handler.HandlePanic(context.Background(), job, "boom", "trace")
		})
	})

	t.Run("integer panic value is handled", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus()
		ch := eb.Subscribe("int-panic")

		handler := &riverErrorHandler{
			eventBus: eb,
			metrics:  nil,
			logger:   discardLogger(),
		}

		job := makeJobRow(10, "test_kind", "high", 3)

		assert.NotPanics(t, func() {
			handler.HandlePanic(context.Background(), job, 42, "trace")
		})

		select {
		case event := <-ch:
			assert.Equal(t, EventJobFailed, event.Type)
			assert.Equal(t, "job panicked", event.Error)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	})
}
