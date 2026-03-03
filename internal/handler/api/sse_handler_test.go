package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/queue"
)

// ---------------------------------------------------------------------------
// SSEHandler tests
//
// SSEHandler.Stream is an SSE endpoint that reads from an EventBus and writes
// events in text/event-stream format. We test the headers, SSE wire format,
// client disconnect handling, and the error path for non-flushable writers.
// ---------------------------------------------------------------------------

func newTestSSEHandler() (*SSEHandler, *queue.EventBus) {
	eb := queue.NewEventBus()
	h := NewSSEHandler(eb)
	return h, eb
}

// ---------------------------------------------------------------------------
// sseRecorder is a thread-safe http.ResponseWriter + http.Flusher wrapper
// around httptest.ResponseRecorder. It signals on first write so tests can
// know when the handler has started streaming without polling.
// ---------------------------------------------------------------------------

type sseRecorder struct {
	mu        sync.Mutex
	rr        *httptest.ResponseRecorder
	firstWrite chan struct{} // closed on the first Write call
	once       sync.Once
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{
		rr:         httptest.NewRecorder(),
		firstWrite: make(chan struct{}),
	}
}

func (s *sseRecorder) Header() http.Header {
	// Header() map access is safe before WriteHeader; no lock needed per
	// httptest.ResponseRecorder docs. We still lock for safety under race.
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rr.Header()
}

func (s *sseRecorder) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.rr.Write(b)
	s.once.Do(func() { close(s.firstWrite) })
	return n, err
}

func (s *sseRecorder) WriteHeader(statusCode int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rr.WriteHeader(statusCode)
}

func (s *sseRecorder) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rr.Flush()
}

// body returns a snapshot of the recorded body.
func (s *sseRecorder) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rr.Body.String()
}

// header returns a snapshot of the response header.
func (s *sseRecorder) header() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rr.Header()
}

// waitForFirstWrite blocks until the handler writes its first byte,
// which proves the handler goroutine has subscribed and processed an event.
func (s *sseRecorder) waitForFirstWrite(t *testing.T) {
	t.Helper()
	select {
	case <-s.firstWrite:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler to write first SSE event")
	}
}

// ---------------------------------------------------------------------------
// publishUntilReady publishes sentinel events to the EventBus until the
// recorder receives its first write, then publishes the real events.
// This avoids the race where Publish runs before the handler calls Subscribe.
// ---------------------------------------------------------------------------

func publishUntilReady(t *testing.T, eb *queue.EventBus, rec *sseRecorder, events ...queue.Event) {
	t.Helper()

	// Continuously publish sentinels until the handler writes something.
	go func() {
		sentinel := queue.Event{Type: "sentinel", Timestamp: time.Now()}
		for {
			select {
			case <-rec.firstWrite:
				return
			default:
				eb.Publish(sentinel)
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	rec.waitForFirstWrite(t)

	// Handler is now subscribed and processing. Publish real events.
	for _, e := range events {
		eb.Publish(e)
		time.Sleep(5 * time.Millisecond)
	}

	// Allow time for events to be flushed.
	time.Sleep(50 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// NewSSEHandler tests
// ---------------------------------------------------------------------------

func TestNewSSEHandler(t *testing.T) {
	t.Parallel()

	eb := queue.NewEventBus()
	h := NewSSEHandler(eb)

	assert.NotNil(t, h, "NewSSEHandler should return a non-nil handler")
}

// ---------------------------------------------------------------------------
// Stream handler tests
// ---------------------------------------------------------------------------

func TestSSEHandler_Stream(t *testing.T) {
	t.Parallel()

	t.Run("returns 500 when ResponseWriter does not support flushing", func(t *testing.T) {
		t.Parallel()

		h, _ := newTestSSEHandler()

		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
		w := &nonFlushableWriter{header: http.Header{}, code: http.StatusOK}

		h.Stream(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.code)
		assert.Contains(t, w.body.String(), "streaming not supported")
	})

	t.Run("sets correct SSE headers", func(t *testing.T) {
		t.Parallel()

		h, eb := newTestSSEHandler()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.Stream(rec, req)
			close(done)
		}()

		publishUntilReady(t, eb, rec, queue.Event{
			Type:      queue.EventJobCompleted,
			JobKind:   "test_job",
			JobID:     1,
			Queue:     "default",
			Timestamp: time.Now(),
		})

		cancel()
		<-done

		hdr := rec.header()
		assert.Equal(t, "text/event-stream", hdr.Get("Content-Type"))
		assert.Equal(t, "no-cache", hdr.Get("Cache-Control"))
		assert.Equal(t, "keep-alive", hdr.Get("Connection"))
		assert.Equal(t, "no", hdr.Get("X-Accel-Buffering"))
	})

	t.Run("writes events in SSE format", func(t *testing.T) {
		t.Parallel()

		h, eb := newTestSSEHandler()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.Stream(rec, req)
			close(done)
		}()

		event := queue.Event{
			Type:      queue.EventJobCompleted,
			JobKind:   "index_document",
			JobID:     42,
			Queue:     "default",
			Attempt:   1,
			Timestamp: time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC),
		}
		publishUntilReady(t, eb, rec, event)

		cancel()
		<-done

		body := rec.body()

		// Verify the SSE event line format.
		assert.Contains(t, body, "event: job.completed",
			"body should contain event type line")

		// Verify data line presence.
		assert.Contains(t, body, "data: ",
			"body should contain data line")

		// Verify double newline separates events (SSE spec).
		assert.Contains(t, body, "\n\n",
			"body should contain double newline separator")

		// Extract the JSON data payload for the job.completed event.
		lines := strings.Split(strings.TrimSpace(body), "\n")
		var dataLine string
		for i, line := range lines {
			if line == "event: job.completed" && i+1 < len(lines) {
				dataLine = strings.TrimPrefix(lines[i+1], "data: ")
				break
			}
		}
		require.NotEmpty(t, dataLine, "should have a data line for job.completed")

		var parsed queue.Event
		err := json.Unmarshal([]byte(dataLine), &parsed)
		require.NoError(t, err, "data payload should be valid JSON")
		assert.Equal(t, queue.EventJobCompleted, parsed.Type)
		assert.Equal(t, "index_document", parsed.JobKind)
		assert.Equal(t, int64(42), parsed.JobID)
		assert.Equal(t, "default", parsed.Queue)
	})

	t.Run("streams multiple events in order", func(t *testing.T) {
		t.Parallel()

		h, eb := newTestSSEHandler()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.Stream(rec, req)
			close(done)
		}()

		publishUntilReady(t, eb, rec,
			queue.Event{Type: queue.EventJobDispatched, JobKind: "job_a", JobID: 1, Queue: "high", Timestamp: time.Now()},
			queue.Event{Type: queue.EventJobCompleted, JobKind: "job_b", JobID: 2, Queue: "default", Timestamp: time.Now()},
			queue.Event{Type: queue.EventJobFailed, JobKind: "job_c", JobID: 3, Queue: "low", Timestamp: time.Now()},
		)

		cancel()
		<-done

		body := rec.body()

		// All three event types should appear.
		assert.Contains(t, body, "event: job.dispatched")
		assert.Contains(t, body, "event: job.completed")
		assert.Contains(t, body, "event: job.failed")

		// Verify ordering: dispatched before completed before failed.
		idxDispatched := strings.Index(body, "event: job.dispatched")
		idxCompleted := strings.Index(body, "event: job.completed")
		idxFailed := strings.Index(body, "event: job.failed")

		assert.Less(t, idxDispatched, idxCompleted, "dispatched should come before completed")
		assert.Less(t, idxCompleted, idxFailed, "completed should come before failed")
	})

	t.Run("stops streaming when client disconnects", func(t *testing.T) {
		t.Parallel()

		h, eb := newTestSSEHandler()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.Stream(rec, req)
			close(done)
		}()

		// Wait for handler to be ready and confirm it is streaming.
		publishUntilReady(t, eb, rec, queue.Event{
			Type:      queue.EventJobDispatched,
			JobKind:   "check_job",
			JobID:     99,
			Queue:     "default",
			Timestamp: time.Now(),
		})

		// Simulate client disconnect.
		cancel()

		// The handler goroutine should exit promptly.
		select {
		case <-done:
			// Success: handler returned after context cancellation.
		case <-time.After(2 * time.Second):
			t.Fatal("Stream did not stop within 2 seconds after client disconnect")
		}
	})

	t.Run("event with error field includes error in JSON payload", func(t *testing.T) {
		t.Parallel()

		h, eb := newTestSSEHandler()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.Stream(rec, req)
			close(done)
		}()

		publishUntilReady(t, eb, rec, queue.Event{
			Type:      queue.EventJobFailed,
			JobKind:   "broken_job",
			JobID:     7,
			Queue:     "default",
			Attempt:   3,
			Error:     "connection refused",
			Timestamp: time.Now(),
		})

		cancel()
		<-done

		body := rec.body()
		lines := strings.Split(strings.TrimSpace(body), "\n")

		var dataLine string
		for i, line := range lines {
			if line == "event: job.failed" && i+1 < len(lines) {
				dataLine = strings.TrimPrefix(lines[i+1], "data: ")
				break
			}
		}
		require.NotEmpty(t, dataLine, "should have a data line for job.failed")

		var parsed queue.Event
		err := json.Unmarshal([]byte(dataLine), &parsed)
		require.NoError(t, err)
		assert.Equal(t, "connection refused", parsed.Error)
		assert.Equal(t, 3, parsed.Attempt)
	})
}

// ---------------------------------------------------------------------------
// nonFlushableWriter is an http.ResponseWriter that does NOT implement
// http.Flusher, used to test the unsupported streaming error path.
// ---------------------------------------------------------------------------

type nonFlushableWriter struct {
	header http.Header
	body   strings.Builder
	code   int
}

func (w *nonFlushableWriter) Header() http.Header {
	return w.header
}

func (w *nonFlushableWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *nonFlushableWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}
