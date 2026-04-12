//go:build integration

package queue

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestControlBus_PublishReceive verifies that a message published to a
// topic reaches a subscriber on the same topic. Uses the shared testRedisClient
// from events_redis_integration_test.go.
func TestControlBus_PublishReceive(t *testing.T) {
	t.Parallel()

	bus := NewRedisControlBus(testRedisClient, slog.New(slog.DiscardHandler))
	t.Cleanup(func() { _ = bus.Close() })

	received := make(chan []byte, 1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := bus.Subscribe(ctx, "test.publish", func(payload []byte) {
		received <- payload
	})
	require.NoError(t, err)

	// Allow the subscription to register in Redis.
	time.Sleep(50 * time.Millisecond)

	err = bus.Publish(context.Background(), "test.publish", []byte("hello"))
	require.NoError(t, err)

	select {
	case got := <-received:
		require.Equal(t, []byte("hello"), got)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

// TestControlBus_TopicIsolation verifies that subscribers only receive
// messages for their topic, not for unrelated topics on the same bus.
func TestControlBus_TopicIsolation(t *testing.T) {
	t.Parallel()

	bus := NewRedisControlBus(testRedisClient, slog.New(slog.DiscardHandler))
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var wantedCount atomic.Int32
	err := bus.Subscribe(ctx, "test.wanted", func(_ []byte) {
		wantedCount.Add(1)
	})
	require.NoError(t, err)

	var otherCount atomic.Int32
	err = bus.Subscribe(ctx, "test.other", func(_ []byte) {
		otherCount.Add(1)
	})
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	require.NoError(t, bus.Publish(context.Background(), "test.wanted", nil))
	require.NoError(t, bus.Publish(context.Background(), "test.wanted", nil))
	require.NoError(t, bus.Publish(context.Background(), "test.other", nil))

	time.Sleep(100 * time.Millisecond)

	require.EqualValues(t, 2, wantedCount.Load(), "wanted topic should receive its messages")
	require.EqualValues(t, 1, otherCount.Load(), "other topic should receive its own messages only")
}

// TestControlBus_HandlerPanicRecovered verifies that a panicking handler
// does not kill the subscription goroutine.
func TestControlBus_HandlerPanicRecovered(t *testing.T) {
	t.Parallel()

	bus := NewRedisControlBus(testRedisClient, slog.New(slog.DiscardHandler))
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var mu sync.Mutex
	var calls int
	err := bus.Subscribe(ctx, "test.panic", func(payload []byte) {
		mu.Lock()
		calls++
		mu.Unlock()
		if string(payload) == "boom" {
			panic("intentional")
		}
	})
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	require.NoError(t, bus.Publish(context.Background(), "test.panic", []byte("boom")))
	time.Sleep(50 * time.Millisecond)

	// After a panic-triggering message, a follow-up message should still
	// reach the handler — the goroutine must survive.
	require.NoError(t, bus.Publish(context.Background(), "test.panic", []byte("ok")))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, calls, "second message should be delivered after panic recovery")
}

// TestControlBus_CloseCancelsSubscriptions verifies that Close releases the
// Redis subscriptions and subsequent Subscribe calls fail.
func TestControlBus_CloseCancelsSubscriptions(t *testing.T) {
	t.Parallel()

	bus := NewRedisControlBus(testRedisClient, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := bus.Subscribe(ctx, "test.close", func(_ []byte) {})
	require.NoError(t, err)

	require.NoError(t, bus.Close())
	require.NoError(t, bus.Close(), "Close should be idempotent")

	err = bus.Subscribe(ctx, "test.close.after", func(_ []byte) {})
	require.Error(t, err, "Subscribe after Close must fail")
}
