package api //nolint:revive // package name matches REST API handler directory convention

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/c-premus/documcp/internal/queue"
)

// SSEHandler streams real-time queue events via Server-Sent Events.
type SSEHandler struct {
	eventBus          queue.EventSubscriber
	heartbeatInterval time.Duration
}

// NewSSEHandler creates an SSEHandler with the given heartbeat interval.
func NewSSEHandler(eventBus queue.EventSubscriber, heartbeatInterval time.Duration) *SSEHandler {
	return &SSEHandler{eventBus: eventBus, heartbeatInterval: heartbeatInterval}
}

// Stream handles GET /api/events/stream — an SSE endpoint for queue events.
func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Disable the server's write deadline for this connection so the SSE stream
	// can stay open indefinitely. Without this, http.Server.WriteTimeout kills
	// the connection after the configured duration (default 10 s).
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subID := uuid.New().String()
	events := h.eventBus.Subscribe(subID)
	if events == nil {
		http.Error(w, "too many concurrent connections", http.StatusServiceUnavailable)
		return
	}
	defer h.eventBus.Unsubscribe(subID)

	// Flush headers immediately so the browser's EventSource fires onopen now,
	// not 30 s later when the first heartbeat is sent. Without this flush the
	// HTTP 200 response is not committed until the first body write, causing
	// the browser to see a "pending" request and delaying connection-pool slots.
	// retry: 5000 tells EventSource to wait 5 s before reconnecting on error.
	_, _ = fmt.Fprint(w, "retry: 5000\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(h.heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
