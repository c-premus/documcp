package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"git.999.haus/chris/DocuMCP-go/internal/queue"
)

// SSEHandler streams real-time queue events via Server-Sent Events.
type SSEHandler struct {
	eventBus *queue.EventBus
}

// NewSSEHandler creates an SSEHandler.
func NewSSEHandler(eventBus *queue.EventBus) *SSEHandler {
	return &SSEHandler{eventBus: eventBus}
}

// Stream handles GET /api/events/stream — an SSE endpoint for queue events.
func (h *SSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subID := uuid.New().String()
	events := h.eventBus.Subscribe(subID)
	defer h.eventBus.Unsubscribe(subID)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}
