package queue

import (
	"sync"
	"time"
)

// EventType identifies the kind of queue event.
type EventType string

const (
	// EventJobDispatched is emitted when a background job is enqueued.
	EventJobDispatched EventType = "job.dispatched"
	// EventJobCompleted is emitted when a background job finishes successfully.
	EventJobCompleted EventType = "job.completed"
	// EventJobFailed is emitted when a background job exhausts all retries.
	EventJobFailed EventType = "job.failed"
	// EventJobSnoozed is emitted when a background job is snoozed for later retry.
	EventJobSnoozed EventType = "job.snoozed"
	// EventJobRetrying is emitted when a background job is scheduled for retry.
	EventJobRetrying EventType = "job.retrying"
)

// Event represents a queue lifecycle event.
type Event struct {
	Type      EventType `json:"type"`
	JobKind   string    `json:"job_kind"`
	JobID     int64     `json:"job_id"`
	Queue     string    `json:"queue"`
	Attempt   int       `json:"attempt,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// EventBus provides in-memory pub/sub for queue events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]chan Event
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[string]chan Event)}
}

// Subscribe returns a channel that receives events. Call Unsubscribe with the
// returned ID when done.
func (eb *EventBus) Subscribe(id string) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan Event, 64)
	eb.subscribers[id] = ch
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (eb *EventBus) Unsubscribe(id string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if ch, ok := eb.subscribers[id]; ok {
		close(ch)
		delete(eb.subscribers, id)
	}
}

// Close unsubscribes all current subscribers by closing their channels.
// SSE handlers watching those channels will exit cleanly, allowing
// http.Server.Shutdown to complete without waiting for the full timeout.
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	for id, ch := range eb.subscribers {
		close(ch)
		delete(eb.subscribers, id)
	}
}

// Publish sends an event to all subscribers. Non-blocking: drops the event
// if a subscriber's buffer is full.
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for _, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
