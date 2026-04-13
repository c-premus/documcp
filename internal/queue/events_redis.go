package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// publishTimeout is the deadline for Redis PUBLISH operations.
// Prevents goroutine hangs when Redis is unresponsive.
const publishTimeout = 3 * time.Second

const redisEventChannel = "documcp:events"

// RedisEventBus provides distributed pub/sub for queue events via Redis.
// It implements both EventPublisher and EventSubscriber.
type RedisEventBus struct {
	client      *redis.Client
	logger      *slog.Logger
	mu          sync.RWMutex
	subscribers map[string]chan Event
	cancel      context.CancelFunc
	done        chan struct{}
	dropped     atomic.Int64
	closeOnce   sync.Once
}

// NewRedisEventBus creates a RedisEventBus and starts a background goroutine
// that subscribes to the Redis channel and fans out messages to local subscribers.
func NewRedisEventBus(ctx context.Context, client *redis.Client, logger *slog.Logger) *RedisEventBus {
	subCtx, cancel := context.WithCancel(ctx)

	reb := &RedisEventBus{
		client:      client,
		logger:      logger,
		subscribers: make(map[string]chan Event),
		cancel:      cancel,
		done:        make(chan struct{}),
	}

	go reb.subscribe(subCtx)

	return reb
}

// subscribe listens on the Redis Pub/Sub channel and fans out received
// messages to all local subscriber channels.
func (reb *RedisEventBus) subscribe(ctx context.Context) {
	defer close(reb.done)

	pubsub := reb.client.Subscribe(ctx, redisEventChannel)
	ch := pubsub.Channel()

	// Close pubsub when context is canceled so ch drains and the loop exits.
	go func() {
		<-ctx.Done()
		_ = pubsub.Close()
	}()

	for msg := range ch {
		var event Event
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			reb.logger.Warn("redis event bus: invalid event payload", "error", err)
			continue
		}
		reb.fanOut(event)
	}
}

// fanOut delivers an event to all local subscribers using non-blocking sends.
func (reb *RedisEventBus) fanOut(event Event) {
	reb.mu.RLock()
	defer reb.mu.RUnlock()

	var droppedCount int
	for _, ch := range reb.subscribers {
		select {
		case ch <- event:
		default:
			droppedCount++
			reb.dropped.Add(1)
		}
	}
	if droppedCount > 0 {
		reb.logger.Warn("redis event bus: events dropped for slow subscribers",
			"event_type", event.Type,
			"dropped", droppedCount,
		)
	}
}

// Publish serializes the event to JSON and publishes it to the Redis channel.
func (reb *RedisEventBus) Publish(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		reb.logger.Error("redis event bus: marshaling event", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()
	if err := reb.client.Publish(ctx, redisEventChannel, data).Err(); err != nil {
		reb.logger.Error("redis event bus: publishing event", "error", err)
	}
}

// Subscribe returns a channel that receives events. Call Unsubscribe with the
// same ID when done. Returns nil if the subscriber limit is reached.
func (reb *RedisEventBus) Subscribe(id string) <-chan Event {
	reb.mu.Lock()
	defer reb.mu.Unlock()
	if len(reb.subscribers) >= maxSubscribers {
		reb.logger.Warn("redis event bus subscriber limit reached", "max", maxSubscribers)
		return nil
	}
	ch := make(chan Event, eventBusBufferSize)
	reb.subscribers[id] = ch
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (reb *RedisEventBus) Unsubscribe(id string) {
	reb.mu.Lock()
	defer reb.mu.Unlock()
	if ch, ok := reb.subscribers[id]; ok {
		close(ch)
		delete(reb.subscribers, id)
	}
}

// Close cancels the background Redis subscription and closes all subscriber channels.
// Safe to call multiple times.
func (reb *RedisEventBus) Close() {
	reb.closeOnce.Do(func() {
		reb.cancel()
		<-reb.done

		reb.mu.Lock()
		defer reb.mu.Unlock()
		for id, ch := range reb.subscribers {
			close(ch)
			delete(reb.subscribers, id)
		}
	})
}

