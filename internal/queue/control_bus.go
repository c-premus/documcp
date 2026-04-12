package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/redis/go-redis/v9"
)

// controlChannelPrefix namespaces control-bus Redis channels so they don't
// collide with the job-event channel (documcp:events) used by RedisEventBus.
const controlChannelPrefix = "documcp:control:"

// ControlBus is a Redis pub/sub transport for cross-replica control
// messages like cache invalidation. It lives in its own set of channels
// (documcp:control:<topic>) and deliberately does not share infrastructure
// with the job-event bus — control messages shouldn't compete with SSE
// subscribers for the 100-subscriber cap on events_redis.go, and treating
// them as job events would pollute the Event type and SSE fan-out.
//
// Use Publish from any goroutine after a state change that other replicas
// need to know about. Use Subscribe at startup to install a long-lived
// listener that reacts to those changes (typically by clearing a local
// cache).
type ControlBus interface {
	// Publish sends payload to all subscribers of topic across all replicas.
	// Errors are returned to the caller but should generally be logged and
	// swallowed — a failed invalidation broadcast must not fail the request
	// that triggered it.
	Publish(ctx context.Context, topic string, payload []byte) error

	// Subscribe starts a goroutine that calls handler for every message
	// received on topic. The goroutine exits when the provided context is
	// canceled. Returns an error only if the initial Redis subscribe fails.
	Subscribe(ctx context.Context, topic string, handler func([]byte)) error

	// Close cancels all active subscriptions and releases resources.
	// Safe to call multiple times.
	Close() error
}

// RedisControlBus is the Redis-backed implementation of ControlBus.
type RedisControlBus struct {
	client *redis.Client
	logger *slog.Logger

	mu     sync.Mutex
	subs   []*redis.PubSub
	closed bool
}

// NewRedisControlBus creates a RedisControlBus backed by the given client.
func NewRedisControlBus(client *redis.Client, logger *slog.Logger) *RedisControlBus {
	return &RedisControlBus{
		client: client,
		logger: logger,
	}
}

// Publish sends payload to all subscribers of topic.
func (b *RedisControlBus) Publish(ctx context.Context, topic string, payload []byte) error {
	if topic == "" {
		return errors.New("control bus: topic is required")
	}
	channel := controlChannelPrefix + topic
	if err := b.client.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("control bus: publish %q: %w", channel, err)
	}
	return nil
}

// Subscribe installs a handler for messages on topic. The handler runs on a
// dedicated goroutine and is called once per message. Handler panics are
// recovered so a misbehaving handler cannot tear down the subscription
// goroutine.
func (b *RedisControlBus) Subscribe(ctx context.Context, topic string, handler func([]byte)) error {
	if topic == "" {
		return errors.New("control bus: topic is required")
	}
	if handler == nil {
		return errors.New("control bus: handler is required")
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("control bus: closed")
	}
	channel := controlChannelPrefix + topic
	pubsub := b.client.Subscribe(ctx, channel)
	// Wait for the subscription to be confirmed so the caller gets a clean
	// error if Redis is unreachable at startup.
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		b.mu.Unlock()
		return fmt.Errorf("control bus: subscribe %q: %w", channel, err)
	}
	b.subs = append(b.subs, pubsub)
	b.mu.Unlock()

	go b.run(ctx, pubsub, topic, handler)
	return nil
}

// run is the receive loop for a single topic subscription.
func (b *RedisControlBus) run(ctx context.Context, pubsub *redis.PubSub, topic string, handler func([]byte)) {
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			b.dispatch(topic, handler, msg.Payload)
		}
	}
}

// dispatch calls the handler with panic recovery so a crashing handler
// doesn't bring down the subscription goroutine.
func (b *RedisControlBus) dispatch(topic string, handler func([]byte), payload string) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("control bus: handler panic",
				"topic", topic, "panic", r)
		}
	}()
	handler([]byte(payload))
}

// Close cancels all subscriptions. After Close, Subscribe returns an error.
func (b *RedisControlBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, sub := range b.subs {
		_ = sub.Close()
	}
	b.subs = nil
	return nil
}

// Compile-time check: RedisControlBus implements ControlBus.
var _ ControlBus = (*RedisControlBus)(nil)
