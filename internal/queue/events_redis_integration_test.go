//go:build integration

package queue

import (
	"context"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

var testRedisClient *redis.Client

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		log.Printf("skipping integration tests: docker not found in PATH")
		os.Exit(0)
	}

	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:8-alpine")
	if err != nil {
		log.Printf("skipping integration tests: starting redis container: %v", err)
		os.Exit(0)
	}

	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			log.Printf("terminating redis container: %v", err)
		}
	}()

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("getting redis connection string: %v", err)
	}

	opts, err := redis.ParseURL(connStr)
	if err != nil {
		log.Fatalf("parsing redis URL: %v", err)
	}

	testRedisClient = redis.NewClient(opts)
	if err := testRedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("pinging redis: %v", err)
	}
	defer testRedisClient.Close()

	os.Exit(m.Run())
}

func newTestBus(t *testing.T) *RedisEventBus {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	bus, err := NewRedisEventBus(ctx, testRedisClient, testutil.DiscardLogger())
	require.NoError(t, err)
	t.Cleanup(func() {
		bus.Close()
		cancel()
	})
	return bus
}

func receiveEvent(t *testing.T, ch <-chan Event) (Event, bool) {
	t.Helper()
	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(2 * time.Second):
		return Event{}, false
	}
}

func TestRedisEventBus_PublishSubscribe(t *testing.T) {
	bus := newTestBus(t)
	ch := bus.Subscribe("sub-1")

	want := Event{
		Type:      EventJobCompleted,
		JobKind:   "document_extract",
		JobID:     42,
		Queue:     "high",
		Timestamp: time.Now().Truncate(time.Millisecond),
	}

	bus.Publish(want)

	got, ok := receiveEvent(t, ch)
	require.True(t, ok, "timed out waiting for event")
	assert.Equal(t, want.Type, got.Type)
	assert.Equal(t, want.JobKind, got.JobKind)
	assert.Equal(t, want.JobID, got.JobID)
	assert.Equal(t, want.Queue, got.Queue)
}

func TestRedisEventBus_MultipleSubscribers(t *testing.T) {
	bus := newTestBus(t)
	ch1 := bus.Subscribe("sub-1")
	ch2 := bus.Subscribe("sub-2")
	ch3 := bus.Subscribe("sub-3")

	want := Event{
		Type:    EventJobDispatched,
		JobKind: "sync_kiwix",
		JobID:   7,
		Queue:   "low",
	}

	bus.Publish(want)

	for i, ch := range []<-chan Event{ch1, ch2, ch3} {
		got, ok := receiveEvent(t, ch)
		require.True(t, ok, "timed out waiting for event on subscriber %d", i+1)
		assert.Equal(t, want.Type, got.Type)
		assert.Equal(t, want.JobID, got.JobID)
	}
}

func TestRedisEventBus_CrossInstance(t *testing.T) {
	busA := newTestBus(t)
	busB := newTestBus(t)

	chB := busB.Subscribe("sub-on-B")

	want := Event{
		Type:    EventJobCompleted,
		JobKind: "cross_instance_job",
		JobID:   99,
		Queue:   "default",
	}

	busA.Publish(want)

	got, ok := receiveEvent(t, chB)
	require.True(t, ok, "bus-B subscriber did not receive event published on bus-A")
	assert.Equal(t, want.Type, got.Type)
	assert.Equal(t, want.JobKind, got.JobKind)
	assert.Equal(t, want.JobID, got.JobID)
}

func TestRedisEventBus_Unsubscribe(t *testing.T) {
	bus := newTestBus(t)
	ch := bus.Subscribe("sub-1")

	bus.Unsubscribe("sub-1")

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after unsubscribe")

	// Publish after unsubscribe should not panic or deliver.
	bus.Publish(Event{Type: EventJobDispatched, JobID: 1})
}

func TestRedisEventBus_Close(t *testing.T) {
	bus, err := NewRedisEventBus(t.Context(), testRedisClient, testutil.DiscardLogger())
	require.NoError(t, err)

	ch1 := bus.Subscribe("sub-1")
	ch2 := bus.Subscribe("sub-2")

	bus.Close()

	for i, ch := range []<-chan Event{ch1, ch2} {
		_, ok := <-ch
		assert.False(t, ok, "channel %d should be closed after Close", i+1)
	}
}

func TestRedisEventBus_DropsSlowSubscriber(t *testing.T) {
	bus := newTestBus(t)
	slowCh := bus.Subscribe("slow")
	require.NotNil(t, slowCh)

	// Fill the subscriber buffer completely.
	for i := range eventBusBufferSize {
		bus.Publish(Event{Type: EventJobDispatched, JobID: int64(i)})
	}

	// Wait for Redis to deliver enough events that the subscriber buffer is
	// saturated. Any further publish must overflow the non-blocking send in
	// fanOut and be counted as dropped.
	require.Eventually(t, func() bool {
		return len(slowCh) == cap(slowCh)
	}, 5*time.Second, 10*time.Millisecond, "slow subscriber buffer should fill")

	// The next publish can't fit — fanOut should drop it.
	bus.Publish(Event{Type: EventJobFailed, JobID: 999})

	require.Eventually(t, func() bool {
		return bus.dropped.Load() >= 1
	}, 5*time.Second, 10*time.Millisecond, "at least one event should have been dropped")
}

func TestRedisEventBus_EventSerialization(t *testing.T) {
	bus := newTestBus(t)
	ch := bus.Subscribe("sub-1")

	want := Event{
		Type:      EventJobFailed,
		JobKind:   "document_extract",
		JobID:     12345,
		Queue:     "critical",
		Attempt:   3,
		Error:     "context deadline exceeded",
		Timestamp: time.Now().Truncate(time.Millisecond),
	}

	bus.Publish(want)

	got, ok := receiveEvent(t, ch)
	require.True(t, ok, "timed out waiting for event")
	assert.Equal(t, want.Type, got.Type)
	assert.Equal(t, want.JobKind, got.JobKind)
	assert.Equal(t, want.JobID, got.JobID)
	assert.Equal(t, want.Queue, got.Queue)
	assert.Equal(t, want.Attempt, got.Attempt)
	assert.Equal(t, want.Error, got.Error)
	assert.True(t, want.Timestamp.Equal(got.Timestamp),
		"timestamps should match: want %v, got %v", want.Timestamp, got.Timestamp)
}
