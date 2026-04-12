package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authmiddleware "github.com/c-premus/documcp/internal/auth/middleware"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
)

// ---------------------------------------------------------------------------
// SSEHandler tests
//
// SSEHandler.Stream is an SSE endpoint that reads from an EventBus and writes
// events in text/event-stream format. We test the headers, SSE wire format,
// client disconnect handling, and the error path for non-flushable writers.
// ---------------------------------------------------------------------------

func newTestSSEHandler() (*SSEHandler, *queue.EventBus) {
	eb := queue.NewEventBus(slog.New(slog.DiscardHandler))
	h := NewSSEHandler(eb, 15*time.Second)
	return h, eb
}

// ---------------------------------------------------------------------------
// sseRecorder is a thread-safe http.ResponseWriter + http.Flusher wrapper
// around httptest.ResponseRecorder. It signals on first write so tests can
// know when the handler has started streaming without polling.
// ---------------------------------------------------------------------------

type sseRecorder struct {
	mu         sync.Mutex
	rr         *httptest.ResponseRecorder
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
	for i := range events {
		eb.Publish(events[i])
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

	eb := queue.NewEventBus(slog.New(slog.DiscardHandler))
	h := NewSSEHandler(eb, 15*time.Second)

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

		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
		w := &nonFlushableWriter{header: http.Header{}, code: http.StatusOK}

		h.Stream(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.code)
		assert.Contains(t, w.body.String(), "streaming not supported")
	})

	t.Run("sets correct SSE headers", func(t *testing.T) {
		t.Parallel()

		h, eb := newTestSSEHandler()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
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
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
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

		// Verify data line presence (unnamed SSE events — no event: field).
		assert.Contains(t, body, "data: ",
			"body should contain data line")

		// Verify double newline separates events (SSE spec).
		assert.Contains(t, body, "\n\n",
			"body should contain double newline separator")

		// Should NOT contain named event lines (frontend uses onmessage).
		assert.NotContains(t, body, "event: job.completed",
			"should not use named SSE events")

		// Extract the JSON data payload for the job.completed event.
		lines := strings.Split(strings.TrimSpace(body), "\n")
		var dataLine string
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, "job.completed") {
				dataLine = strings.TrimPrefix(line, "data: ")
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
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
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

		// All three event types should appear in data payloads.
		assert.Contains(t, body, `"type":"job.dispatched"`)
		assert.Contains(t, body, `"type":"job.completed"`)
		assert.Contains(t, body, `"type":"job.failed"`)

		// Verify ordering: dispatched before completed before failed.
		idxDispatched := strings.Index(body, `"type":"job.dispatched"`)
		idxCompleted := strings.Index(body, `"type":"job.completed"`)
		idxFailed := strings.Index(body, `"type":"job.failed"`)

		assert.Less(t, idxDispatched, idxCompleted, "dispatched should come before completed")
		assert.Less(t, idxCompleted, idxFailed, "completed should come before failed")
	})

	t.Run("stops streaming when client disconnects", func(t *testing.T) {
		t.Parallel()

		h, eb := newTestSSEHandler()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
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
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
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
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, "job.failed") {
				dataLine = strings.TrimPrefix(line, "data: ")
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

// ---------------------------------------------------------------------------
// stubEventSubscriber returns a pre-filled channel and tracks unsubscribe.
// ---------------------------------------------------------------------------

type stubEventSubscriber struct {
	ch           chan queue.Event
	unsubscribed bool
}

func (s *stubEventSubscriber) Subscribe(_ string) <-chan queue.Event {
	return s.ch
}

func (s *stubEventSubscriber) Unsubscribe(_ string) {
	s.unsubscribed = true
}

func (s *stubEventSubscriber) Close() {
	if s.ch != nil {
		close(s.ch)
	}
}

// nilEventSubscriber always returns nil from Subscribe (subscriber limit).
type nilEventSubscriber struct{}

func (nilEventSubscriber) Subscribe(_ string) <-chan queue.Event { return nil }
func (nilEventSubscriber) Unsubscribe(_ string)                 {}
func (nilEventSubscriber) Close()                                {}

// ctxWithUser returns a context with the given user set via the auth middleware key.
func ctxWithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, authmiddleware.UserContextKey, user)
}

// ---------------------------------------------------------------------------
// UserStream handler tests
// ---------------------------------------------------------------------------

func TestSSEHandler_UserStream(t *testing.T) {
	t.Parallel()

	t.Run("returns 401 when no user in context", func(t *testing.T) {
		t.Parallel()

		h := NewSSEHandler(&stubEventSubscriber{}, 15*time.Second)

		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
		w := httptest.NewRecorder()

		h.UserStream(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "unauthorized")
	})

	t.Run("returns 503 when subscriber limit reached", func(t *testing.T) {
		t.Parallel()

		h := NewSSEHandler(nilEventSubscriber{}, 15*time.Second)

		user := &model.User{ID: 1, IsAdmin: false}
		ctx := ctxWithUser(context.Background(), user)
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		h.UserStream(rec, req)

		rec.mu.Lock()
		code := rec.rr.Code
		body := rec.rr.Body.String()
		rec.mu.Unlock()

		assert.Equal(t, http.StatusServiceUnavailable, code)
		assert.Contains(t, body, "too many concurrent connections")
	})

	t.Run("admin user receives all events including UserID=0", func(t *testing.T) {
		t.Parallel()

		ch := make(chan queue.Event, 10)
		stub := &stubEventSubscriber{ch: ch}
		h := NewSSEHandler(stub, 15*time.Second)

		admin := &model.User{ID: 1, IsAdmin: true}
		ctx, cancel := context.WithCancel(ctxWithUser(context.Background(), admin))
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.UserStream(rec, req)
			close(done)
		}()

		// Wait for handler to start (retry line is the first write).
		rec.waitForFirstWrite(t)

		// Send events: one with UserID=0 (scheduler), one with UserID=99.
		ch <- queue.Event{Type: queue.EventJobCompleted, JobKind: "scheduler_job", JobID: 1, UserID: 0, Timestamp: time.Now()}
		ch <- queue.Event{Type: queue.EventJobCompleted, JobKind: "user_job", JobID: 2, UserID: 99, Timestamp: time.Now()}
		time.Sleep(50 * time.Millisecond)

		cancel()
		<-done

		body := rec.body()
		assert.Contains(t, body, "scheduler_job", "admin should see UserID=0 events")
		assert.Contains(t, body, "user_job", "admin should see all user events")
	})

	t.Run("non-admin user receives events matching their UserID", func(t *testing.T) {
		t.Parallel()

		ch := make(chan queue.Event, 10)
		stub := &stubEventSubscriber{ch: ch}
		h := NewSSEHandler(stub, 15*time.Second)

		user := &model.User{ID: 42, IsAdmin: false}
		ctx, cancel := context.WithCancel(ctxWithUser(context.Background(), user))
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.UserStream(rec, req)
			close(done)
		}()

		rec.waitForFirstWrite(t)

		ch <- queue.Event{Type: queue.EventJobCompleted, JobKind: "my_job", JobID: 1, UserID: 42, Timestamp: time.Now()}
		time.Sleep(50 * time.Millisecond)

		cancel()
		<-done

		body := rec.body()
		assert.Contains(t, body, "my_job", "non-admin should see events matching their UserID")
	})

	t.Run("non-admin user does NOT receive events with UserID=0", func(t *testing.T) {
		t.Parallel()

		ch := make(chan queue.Event, 10)
		stub := &stubEventSubscriber{ch: ch}
		h := NewSSEHandler(stub, 15*time.Second)

		user := &model.User{ID: 42, IsAdmin: false}
		ctx, cancel := context.WithCancel(ctxWithUser(context.Background(), user))
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.UserStream(rec, req)
			close(done)
		}()

		rec.waitForFirstWrite(t)

		// UserID=0 is a scheduler/system event — non-admins should not see it.
		ch <- queue.Event{Type: queue.EventJobCompleted, JobKind: "scheduler_job", JobID: 1, UserID: 0, Timestamp: time.Now()}
		// Also send a matching event so the handler processes both before cancel.
		ch <- queue.Event{Type: queue.EventJobCompleted, JobKind: "own_job", JobID: 2, UserID: 42, Timestamp: time.Now()}
		time.Sleep(50 * time.Millisecond)

		cancel()
		<-done

		body := rec.body()
		assert.NotContains(t, body, "scheduler_job", "non-admin should NOT see UserID=0 events")
		assert.Contains(t, body, "own_job", "non-admin should see their own events")
	})

	t.Run("non-admin user does NOT receive events with different UserID", func(t *testing.T) {
		t.Parallel()

		ch := make(chan queue.Event, 10)
		stub := &stubEventSubscriber{ch: ch}
		h := NewSSEHandler(stub, 15*time.Second)

		user := &model.User{ID: 42, IsAdmin: false}
		ctx, cancel := context.WithCancel(ctxWithUser(context.Background(), user))
		req := httptest.NewRequest(http.MethodGet, "/api/events/stream", http.NoBody)
		req = req.WithContext(ctx)
		rec := newSSERecorder()

		done := make(chan struct{})
		go func() {
			h.UserStream(rec, req)
			close(done)
		}()

		rec.waitForFirstWrite(t)

		// Event for a different user.
		ch <- queue.Event{Type: queue.EventJobCompleted, JobKind: "other_user_job", JobID: 1, UserID: 99, Timestamp: time.Now()}
		// Own event to confirm filtering.
		ch <- queue.Event{Type: queue.EventJobCompleted, JobKind: "own_job", JobID: 2, UserID: 42, Timestamp: time.Now()}
		time.Sleep(50 * time.Millisecond)

		cancel()
		<-done

		body := rec.body()
		assert.NotContains(t, body, "other_user_job", "non-admin should NOT see events for other users")
		assert.Contains(t, body, "own_job", "non-admin should see their own events")
	})
}
