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

	eb := NewEventBus(discardLogger())
	require.NotNil(t, eb)
	assert.NotNil(t, eb.subscribers)
	assert.Empty(t, eb.subscribers)
}

func TestEventBus_Subscribe(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	ch := eb.Subscribe("sub-1")

	require.NotNil(t, ch)
	assert.Len(t, eb.subscribers, 1)
}

func TestEventBus_Subscribe_multipleSubscribers(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	ch1 := eb.Subscribe("sub-1")
	ch2 := eb.Subscribe("sub-2")

	require.NotNil(t, ch1)
	require.NotNil(t, ch2)
	assert.Len(t, eb.subscribers, 2)
}

func TestEventBus_Publish_deliversToSubscriber(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
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

	eb := NewEventBus(discardLogger())
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

	eb := NewEventBus(discardLogger())
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

	eb := NewEventBus(discardLogger())
	// Should not panic when unsubscribing a non-existent ID.
	assert.NotPanics(t, func() {
		eb.Unsubscribe("does-not-exist")
	})
}

func TestEventBus_Unsubscribe_removesFromMap(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
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

	eb := NewEventBus(discardLogger())
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

	eb := NewEventBus(discardLogger())
	assert.NotPanics(t, func() {
		eb.Publish(Event{Type: EventJobCompleted, JobID: 1})
	})
}

func TestEventBus_ConcurrentPublishSubscribe(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
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

func TestEventBus_Close(t *testing.T) {
	t.Parallel()

	t.Run("closes all subscriber channels", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus(discardLogger())
		ch1 := eb.Subscribe("sub-1")
		ch2 := eb.Subscribe("sub-2")
		ch3 := eb.Subscribe("sub-3")

		eb.Close()

		// All channels should be closed (readable with ok=false).
		for i, ch := range []<-chan Event{ch1, ch2, ch3} {
			select {
			case _, ok := <-ch:
				if ok {
					t.Errorf("channel %d should be closed but received a value", i+1)
				}
			case <-time.After(time.Second):
				t.Errorf("channel %d was not closed within timeout", i+1)
			}
		}
	})

	t.Run("empties the subscriber map", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus(discardLogger())
		eb.Subscribe("sub-a")
		eb.Subscribe("sub-b")

		eb.Close()

		eb.mu.RLock()
		assert.Empty(t, eb.subscribers, "subscriber map should be empty after Close")
		eb.mu.RUnlock()
	})

	t.Run("close on empty bus does not panic", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus(discardLogger())
		assert.NotPanics(t, func() { eb.Close() })
	})

	t.Run("publish after close does not panic or block", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus(discardLogger())
		eb.Subscribe("sub-1")
		eb.Close()

		assert.NotPanics(t, func() {
			eb.Publish(Event{Type: EventJobCompleted, JobID: 1})
		})
	})
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
		{"snoozed", EventJobSnoozed, "job.snoozed"},
		{"retrying", EventJobRetrying, "job.retrying"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, EventType(tt.want), tt.et)
		})
	}
}

func TestEventBus_DroppedCount(t *testing.T) {
	t.Parallel()

	t.Run("zero when no events dropped", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus(discardLogger())
		ch := eb.Subscribe("sub-1")

		eb.Publish(Event{Type: EventJobCompleted, JobID: 1})

		// Drain the event so the channel is not full.
		<-ch

		assert.Equal(t, int64(0), eb.DroppedCount())
	})

	t.Run("increments for each dropped event per subscriber", func(t *testing.T) {
		t.Parallel()

		eb := NewEventBus(discardLogger())
		eb.Subscribe("slow-1")
		eb.Subscribe("slow-2")

		// Fill both subscriber buffers.
		for i := range eventBusBufferSize {
			eb.Publish(Event{JobID: int64(i), Type: EventJobDispatched})
		}
		assert.Equal(t, int64(0), eb.DroppedCount(), "no drops while buffers have capacity")

		// One more publish should drop for both subscribers.
		eb.Publish(Event{JobID: 999, Type: EventJobFailed})
		assert.Equal(t, int64(2), eb.DroppedCount(), "one drop per slow subscriber")

		// Another publish drops two more.
		eb.Publish(Event{JobID: 1000, Type: EventJobFailed})
		assert.Equal(t, int64(4), eb.DroppedCount(), "cumulative drops across publishes")
	})
}

func TestEventBus_Subscribe_duplicateID(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	ch1 := eb.Subscribe("dup")
	ch2 := eb.Subscribe("dup")

	// The second subscribe should overwrite the first. The subscriber map
	// should still have exactly one entry.
	eb.mu.RLock()
	assert.Len(t, eb.subscribers, 1)
	eb.mu.RUnlock()

	// Publishing should deliver to the new channel, not the old one.
	eb.Publish(Event{Type: EventJobCompleted, JobID: 1})

	select {
	case received := <-ch2:
		assert.Equal(t, int64(1), received.JobID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event on new channel")
	}

	// Old channel should NOT receive the event (it was replaced, not closed).
	select {
	case <-ch1:
		t.Fatal("old channel should not receive events after being replaced")
	default:
		// Expected: nothing on old channel.
	}
}

func TestEventBus_Unsubscribe_thenPublish(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	ch := eb.Subscribe("sub-1")
	eb.Subscribe("sub-2")

	eb.Unsubscribe("sub-1")

	// Publish should only reach sub-2, not panic or block.
	eb.Publish(Event{Type: EventJobDispatched, JobID: 42})

	// ch is closed so reading should return zero value with ok=false.
	_, ok := <-ch
	assert.False(t, ok, "unsubscribed channel should be closed")

	// sub-2 still in the map.
	eb.mu.RLock()
	assert.Len(t, eb.subscribers, 1)
	eb.mu.RUnlock()
}

func TestEventBus_ResubscribeAfterUnsubscribe(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	ch1 := eb.Subscribe("sub-1")
	eb.Unsubscribe("sub-1")

	// Old channel is closed.
	_, ok := <-ch1
	assert.False(t, ok)

	// Re-subscribe with the same ID.
	ch2 := eb.Subscribe("sub-1")
	require.NotNil(t, ch2)

	eb.Publish(Event{Type: EventJobCompleted, JobID: 7})

	select {
	case received := <-ch2:
		assert.Equal(t, int64(7), received.JobID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event on re-subscribed channel")
	}
}

func TestEventBus_Close_idempotent(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	eb.Subscribe("sub-1")
	eb.Subscribe("sub-2")

	eb.Close()

	// Second close should not panic (channels already closed, map empty).
	assert.NotPanics(t, func() {
		eb.Close()
	})

	eb.mu.RLock()
	assert.Empty(t, eb.subscribers)
	eb.mu.RUnlock()
}

func TestEventBus_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	const iterations = 100

	var wg sync.WaitGroup

	// Rapidly subscribe and unsubscribe from many goroutines.
	for i := range iterations {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			subID := "sub-" + string(rune('A'+id%26)) + string(rune('0'+id/26))
			eb.Subscribe(subID)
			eb.Unsubscribe(subID)
		}(i)
	}

	// Concurrent close while subscribe/unsubscribe is happening.
	wg.Go(func() {
		// Let some goroutines get started.
		time.Sleep(time.Millisecond)
		eb.Close()
	})

	wg.Wait()
	// Reaching here without deadlock or panic is the test.
}

func TestEvent_FieldValues(t *testing.T) {
	t.Parallel()

	now := time.Now()
	event := Event{
		Type:      EventJobFailed,
		JobKind:   "document_extract",
		JobID:     123,
		Queue:     "high",
		Attempt:   3,
		Error:     "timeout exceeded",
		Timestamp: now,
	}

	assert.Equal(t, EventJobFailed, event.Type)
	assert.Equal(t, "document_extract", event.JobKind)
	assert.Equal(t, int64(123), event.JobID)
	assert.Equal(t, "high", event.Queue)
	assert.Equal(t, 3, event.Attempt)
	assert.Equal(t, "timeout exceeded", event.Error)
	assert.Equal(t, now, event.Timestamp)
}

func TestEventBus_Publish_eventFieldsPreserved(t *testing.T) {
	t.Parallel()

	eb := NewEventBus(discardLogger())
	ch := eb.Subscribe("sub-1")

	now := time.Now()
	event := Event{
		Type:      EventJobSnoozed,
		JobKind:   "sync_kiwix",
		JobID:     55,
		Queue:     "low",
		Attempt:   2,
		Error:     "rate limited",
		Timestamp: now,
	}

	eb.Publish(event)

	select {
	case received := <-ch:
		assert.Equal(t, event, received, "published event should be received with all fields intact")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}
