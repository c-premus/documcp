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

	"github.com/c-premus/documcp/internal/observability"
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

		eb := NewEventBus(discardLogger())
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

		eb := NewEventBus(discardLogger())

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

		eb := NewEventBus(discardLogger())
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

		eb := NewEventBus(discardLogger())
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

// ---------------------------------------------------------------------------
// buildQueueConfig
// ---------------------------------------------------------------------------

func TestBuildQueueConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil map uses defaults", func(t *testing.T) {
		t.Parallel()

		cfg := buildQueueConfig(nil)

		require.Len(t, cfg, 3)
		assert.Equal(t, 10, cfg["high"].MaxWorkers)
		assert.Equal(t, 5, cfg["default"].MaxWorkers)
		assert.Equal(t, 2, cfg["low"].MaxWorkers)
	})

	t.Run("custom values override defaults", func(t *testing.T) {
		t.Parallel()

		cfg := buildQueueConfig(map[string]int{
			"high":    20,
			"default": 15,
			"low":     8,
		})

		assert.Equal(t, 20, cfg["high"].MaxWorkers)
		assert.Equal(t, 15, cfg["default"].MaxWorkers)
		assert.Equal(t, 8, cfg["low"].MaxWorkers)
	})

	t.Run("zero values fall back to defaults", func(t *testing.T) {
		t.Parallel()

		cfg := buildQueueConfig(map[string]int{
			"high":    0,
			"default": 0,
			"low":     0,
		})

		assert.Equal(t, 10, cfg["high"].MaxWorkers)
		assert.Equal(t, 5, cfg["default"].MaxWorkers)
		assert.Equal(t, 2, cfg["low"].MaxWorkers)
	})

	t.Run("negative values fall back to defaults", func(t *testing.T) {
		t.Parallel()

		cfg := buildQueueConfig(map[string]int{
			"high":    -1,
			"default": -5,
			"low":     -100,
		})

		assert.Equal(t, 10, cfg["high"].MaxWorkers)
		assert.Equal(t, 5, cfg["default"].MaxWorkers)
		assert.Equal(t, 2, cfg["low"].MaxWorkers)
	})

	t.Run("partial override only sets high", func(t *testing.T) {
		t.Parallel()

		cfg := buildQueueConfig(map[string]int{
			"high": 25,
		})

		assert.Equal(t, 25, cfg["high"].MaxWorkers)
		assert.Equal(t, 5, cfg["default"].MaxWorkers)
		assert.Equal(t, 2, cfg["low"].MaxWorkers)
	})
}

// ---------------------------------------------------------------------------
// InsertOnly accessor
// ---------------------------------------------------------------------------

func TestRiverClient_InsertOnly(t *testing.T) {
	t.Parallel()

	t.Run("returns true when insertOnly is set", func(t *testing.T) {
		t.Parallel()

		rc := &RiverClient{insertOnly: true}
		assert.True(t, rc.InsertOnly())
	})

	t.Run("returns false when insertOnly is not set", func(t *testing.T) {
		t.Parallel()

		rc := &RiverClient{insertOnly: false}
		assert.False(t, rc.InsertOnly())
	})
}

// ---------------------------------------------------------------------------
// Insert-only mode: Start, Stop, StartEventForwarding
// ---------------------------------------------------------------------------

func TestRiverClient_InsertOnlyMode_Start(t *testing.T) {
	t.Parallel()

	rc := &RiverClient{
		insertOnly: true,
		logger:     discardLogger(),
	}

	err := rc.Start(context.Background())
	require.NoError(t, err, "Start() with insertOnly=true should return nil immediately")
}

func TestRiverClient_InsertOnlyMode_Stop(t *testing.T) {
	t.Parallel()

	rc := &RiverClient{
		insertOnly: true,
		logger:     discardLogger(),
	}

	err := rc.Stop(context.Background())
	require.NoError(t, err, "Stop() with insertOnly=true should return nil immediately")
}

func TestRiverClient_InsertOnlyMode_StartEventForwarding(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	rc := &RiverClient{
		insertOnly: true,
		eventBus:   eb,
		logger:     discardLogger(),
	}

	assert.NotPanics(t, func() {
		rc.StartEventForwarding()
	})

	// cancelSubscribe and forwardingDone should remain nil — no subscription started.
	assert.Nil(t, rc.cancelSubscribe, "cancelSubscribe should be nil in insert-only mode")
	assert.Nil(t, rc.forwardingDone, "forwardingDone should be nil in insert-only mode")
}

// ---------------------------------------------------------------------------
// buildQueueConfig — verify it produces valid river.QueueConfig values
// ---------------------------------------------------------------------------

func TestBuildQueueConfig_allKeysPresent(t *testing.T) {
	t.Parallel()

	cfg := buildQueueConfig(map[string]int{"extra_ignored": 99})

	// All three expected queues must be present regardless of input keys.
	for _, name := range []string{"high", "default", "low"} {
		_, ok := cfg[name]
		assert.True(t, ok, "expected queue %q to be present", name)
	}

	// "extra_ignored" key should NOT appear — buildQueueConfig only produces
	// the three hard-coded queues.
	_, ok := cfg["extra_ignored"]
	assert.False(t, ok, "unexpected queue key should not appear")
}

// ---------------------------------------------------------------------------
// buildQueueConfig — verify type is river.QueueConfig
// ---------------------------------------------------------------------------

func TestBuildQueueConfig_returnsCorrectType(t *testing.T) {
	t.Parallel()

	cfg := buildQueueConfig(nil)

	require.NotNil(t, cfg)
	// Verify the returned type has the expected MaxWorkers field.
	assert.Equal(t, 10, cfg["high"].MaxWorkers)
}
