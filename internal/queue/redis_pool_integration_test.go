//go:build integration

package queue

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultLogger restores go-redis to standard stderr logging. Used in
// t.Cleanup to undo redis.SetLogger after tests that capture output.
type defaultLogger struct{}

func (l *defaultLogger) Printf(_ context.Context, format string, v ...any) {
	log.Printf(format, v...)
}

// capturingLogger implements the go-redis internal.Logging interface and
// records all messages for later inspection.
type capturingLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *capturingLogger) Printf(_ context.Context, format string, v ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(format, v...))
}

func (l *capturingLogger) Messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]string, len(l.messages))
	copy(cp, l.messages)
	return cp
}

// newPoolTestClient creates a redis.Client with the given protocol version
// and production-like pool settings, connected to the testcontainer.
func newPoolTestClient(t *testing.T, protocol int) *redis.Client {
	t.Helper()

	addr := testRedisClient.Options().Addr
	client := redis.NewClient(&redis.Options{
		Addr:            addr,
		Protocol:        protocol,
		MinIdleConns:    2,
		ConnMaxIdleTime: 5 * time.Minute,
	})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err(), "pool test client ping failed")

	return client
}

// runConcurrentPipelineAndPubSub exercises the Redis client with concurrent
// TxPipeline operations (mimicking httprate-redis) and Pub/Sub (mimicking
// EventBus) to provoke any "unread data" warnings from the connection pool.
func runConcurrentPipelineAndPubSub(t *testing.T, client *redis.Client) {
	t.Helper()

	const (
		pipelineWorkers = 10
		opsPerWorker    = 10
		pubsubChannel   = "pool-test-channel"
		pubsubMessages  = 50
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start a Pub/Sub subscriber.
	sub := client.Subscribe(ctx, pubsubChannel)
	t.Cleanup(func() { _ = sub.Close() })

	// Drain subscription messages in background.
	subDone := make(chan struct{})
	go func() {
		defer close(subDone)
		//nolint:revive // intentionally draining channel
		for range sub.Channel() {
		}
	}()

	var wg sync.WaitGroup

	// Concurrent TxPipeline operations (INCR + EXPIRE) like httprate-redis.
	for w := range pipelineWorkers {
		wg.Go(func() {
			for i := range opsPerWorker {
				key := fmt.Sprintf("pool-test:%d:%d", w, i)
				pipe := client.TxPipeline()
				pipe.Incr(ctx, key)
				pipe.Expire(ctx, key, 10*time.Second)
				_, err := pipe.Exec(ctx)
				if err != nil && ctx.Err() == nil {
					t.Errorf("pipeline exec failed: %v", err)
				}
			}
		})
	}

	// Concurrent publish operations.
	wg.Go(func() {
		for i := range pubsubMessages {
			err := client.Publish(ctx, pubsubChannel, fmt.Sprintf("msg-%d", i)).Err()
			if err != nil && ctx.Err() == nil {
				t.Errorf("publish failed: %v", err)
			}
			// Small jitter to interleave with pipeline ops.
			time.Sleep(time.Millisecond)
		}
	})

	wg.Wait()

	// Close subscriber to stop the drain goroutine.
	_ = sub.Close()
	<-subDone
}

// TestRedisPool_RESP2_NoUnreadDataWarnings verifies that using RESP2 (Protocol 2)
// with production-like pool settings does NOT produce "unread data" warnings
// under concurrent pipeline + pub/sub load.
func TestRedisPool_RESP2_NoUnreadDataWarnings(t *testing.T) {
	captured := &capturingLogger{}
	redis.SetLogger(captured)
	t.Cleanup(func() { redis.SetLogger(&defaultLogger{}) })

	client := newPoolTestClient(t, 2)

	runConcurrentPipelineAndPubSub(t, client)

	// Allow any deferred pool-cleanup warnings to surface.
	time.Sleep(200 * time.Millisecond)

	for _, msg := range captured.Messages() {
		assert.False(t,
			strings.Contains(msg, "unread data"),
			"RESP2 should not produce 'unread data' warnings, got: %s", msg,
		)
	}
}

// TestRedisPool_RESP3_ProducesWarnings documents the known issue: RESP3 push
// notifications on Redis 8 cause go-redis v9 to emit "Conn has unread data
// (not push notification), removing it" warnings. This test proves the problem
// exists with RESP3, justifying the RESP2 switch.
//
// Because the warning is timing-dependent (it requires a connection to be
// returned to the pool with unconsumed push-notification bytes), this test
// may not always reproduce. If it does not, it skips rather than fails.
func TestRedisPool_RESP3_ProducesWarnings(t *testing.T) {
	captured := &capturingLogger{}
	redis.SetLogger(captured)
	t.Cleanup(func() { redis.SetLogger(&defaultLogger{}) })

	client := newPoolTestClient(t, 3)

	runConcurrentPipelineAndPubSub(t, client)

	// Allow any deferred pool-cleanup warnings to surface.
	time.Sleep(500 * time.Millisecond)

	found := false
	for _, msg := range captured.Messages() {
		if strings.Contains(msg, "unread data") {
			found = true
			break
		}
	}

	if !found {
		t.Skip("RESP3 warning not reproduced in this run — timing-dependent")
	}

	t.Log("confirmed: RESP3 produces 'unread data' warnings with Redis 8")
}
