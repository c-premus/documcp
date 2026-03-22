package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventBus(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	require.NotNil(t, eb)
	assert.NotNil(t, eb.subscribers)
	assert.Empty(t, eb.subscribers)
}

func TestEventBus_Subscribe(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	ch := eb.Subscribe("sub-1")

	require.NotNil(t, ch)
	assert.Len(t, eb.subscribers, 1)
}

func TestEventBus_Subscribe_multipleSubscribers(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	ch1 := eb.Subscribe("sub-1")
	ch2 := eb.Subscribe("sub-2")

	require.NotNil(t, ch1)
	require.NotNil(t, ch2)
	assert.Len(t, eb.subscribers, 2)
}

func TestEventBus_Publish_deliversToSubscriber(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	ch := eb.Subscribe("sub-1")

	event := Event{
		Type:      EventJobCompleted,
		JobKind:   "document_extract",
		JobID:     42,
		Queue:     "high",
		Timestamp: time.Now(),
	}

	eb.Publish(event)

	select {
	case received := <-ch:
		assert.Equal(t, event.Type, received.Type)
		assert.Equal(t, event.JobKind, received.JobKind)
		assert.Equal(t, event.JobID, received.JobID)
		assert.Equal(t, event.Queue, received.Queue)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventBus_Publish_deliversToMultipleSubscribers(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	ch1 := eb.Subscribe("sub-1")
	ch2 := eb.Subscribe("sub-2")
	ch3 := eb.Subscribe("sub-3")

	event := Event{
		Type:    EventJobDispatched,
		JobKind: "sync_kiwix",
		JobID:   7,
		Queue:   "low",
	}

	eb.Publish(event)

	for _, ch := range []<-chan Event{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			assert.Equal(t, event.JobKind, received.JobKind)
			assert.Equal(t, event.JobID, received.JobID)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event on subscriber")
		}
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	ch := eb.Subscribe("sub-1")

	eb.Unsubscribe("sub-1")

	// Channel should be closed after unsubscribe.
	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after unsubscribe")

	// Subscriber map should be empty.
	eb.mu.RLock()
	assert.Empty(t, eb.subscribers)
	eb.mu.RUnlock()
}

func TestEventBus_Unsubscribe_nonexistent(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	// Should not panic when unsubscribing a non-existent ID.
	assert.NotPanics(t, func() {
		eb.Unsubscribe("does-not-exist")
	})
}

func TestEventBus_Unsubscribe_removesFromMap(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	eb.Subscribe("sub-1")
	eb.Subscribe("sub-2")

	eb.Unsubscribe("sub-1")

	eb.mu.RLock()
	assert.Len(t, eb.subscribers, 1)
	_, exists := eb.subscribers["sub-2"]
	assert.True(t, exists, "sub-2 should still exist")
	eb.mu.RUnlock()
}

func TestEventBus_Publish_dropsEventForSlowSubscriber(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	ch := eb.Subscribe("slow")

	// Fill the channel buffer (capacity is 64).
	for i := range 64 {
		eb.Publish(Event{JobID: int64(i), Type: EventJobDispatched})
	}

	// The 65th publish should be dropped without blocking.
	done := make(chan struct{})
	go func() {
		eb.Publish(Event{JobID: 999, Type: EventJobFailed})
		close(done)
	}()

	select {
	case <-done:
		// Publish returned without blocking.
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on full subscriber channel")
	}

	// Drain and verify the channel has only the first 64 events.
	var count int
	for range ch {
		count++
		if count == 64 {
			break
		}
	}
	assert.Equal(t, 64, count)
}

func TestEventBus_Publish_noSubscribersDoesNotPanic(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	assert.NotPanics(t, func() {
		eb.Publish(Event{Type: EventJobCompleted, JobID: 1})
	})
}

func TestEventBus_ConcurrentPublishSubscribe(t *testing.T) {
	t.Parallel()

	eb := NewEventBus()
	const numGoroutines = 20
	const numEvents = 50

	var wg sync.WaitGroup

	// Spawn concurrent subscribers.
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subID := "sub-" + string(rune('A'+id))
			ch := eb.Subscribe(subID)
			// Read a few events then unsubscribe.
			for range min(numEvents/2, 10) {
				select {
				case <-ch:
				case <-time.After(500 * time.Millisecond):
					return
				}
			}
			eb.Unsubscribe(subID)
		}(i)
	}

	// Spawn concurrent publishers.
	for range numGoroutines {
		wg.Go(func() {
			for j := range numEvents {
				eb.Publish(Event{
					Type:  EventJobDispatched,
					JobID: int64(j),
				})
			}
		})
	}

	wg.Wait()
	// If we reach here without a race or deadlock, the test passes.
}

func TestEventType_constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		et   EventType
		want string
	}{
		{"dispatched", EventJobDispatched, "job.dispatched"},
		{"completed", EventJobCompleted, "job.completed"},
		{"failed", EventJobFailed, "job.failed"},
		{"retrying", EventJobRetrying, "job.retrying"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, EventType(tt.want), tt.et)
		})
	}
}
