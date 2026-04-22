package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMiniredisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

func TestDeviceFailureLimiter_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()
		l := NewDeviceFailureLimiter(nil, 5, time.Hour)
		allowed, err := l.Allowed(ctx, 42)
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.NoError(t, l.Record(ctx, 42))
		assert.NoError(t, l.Clear(ctx, 42))
	})

	t.Run("zero limit", func(t *testing.T) {
		t.Parallel()
		client, _ := newMiniredisClient(t)
		l := NewDeviceFailureLimiter(client, 0, time.Hour)
		for range 10 {
			require.NoError(t, l.Record(ctx, 42))
		}
		allowed, err := l.Allowed(ctx, 42)
		assert.NoError(t, err)
		assert.True(t, allowed, "zero limit disables enforcement")
	})

	t.Run("zero window", func(t *testing.T) {
		t.Parallel()
		client, _ := newMiniredisClient(t)
		l := NewDeviceFailureLimiter(client, 5, 0)
		for range 10 {
			require.NoError(t, l.Record(ctx, 42))
		}
		allowed, err := l.Allowed(ctx, 42)
		assert.NoError(t, err)
		assert.True(t, allowed, "zero window disables enforcement")
	})
}

func TestDeviceFailureLimiter_AllowsUntilThreshold(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, _ := newMiniredisClient(t)
	l := NewDeviceFailureLimiter(client, 3, time.Hour)

	for i := range 3 {
		allowed, err := l.Allowed(ctx, 42)
		require.NoError(t, err)
		assert.True(t, allowed, "attempt %d should be allowed", i+1)
		require.NoError(t, l.Record(ctx, 42))
	}

	allowed, err := l.Allowed(ctx, 42)
	require.NoError(t, err)
	assert.False(t, allowed, "4th attempt should be blocked (count == limit)")
}

func TestDeviceFailureLimiter_WindowIsFixed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, mr := newMiniredisClient(t)
	l := NewDeviceFailureLimiter(client, 3, time.Hour)

	require.NoError(t, l.Record(ctx, 42))
	firstTTL := mr.TTL(failureKey(42))
	require.Greater(t, firstTTL, time.Duration(0), "first record sets TTL")

	// Fast-forward partway through the window, record again, confirm TTL was
	// not refreshed — attacker can't extend the window by trickling failures.
	mr.FastForward(30 * time.Minute)
	remainingBefore := mr.TTL(failureKey(42))
	require.NoError(t, l.Record(ctx, 42))
	remainingAfter := mr.TTL(failureKey(42))
	assert.InDelta(t, remainingBefore.Seconds(), remainingAfter.Seconds(), 1.0,
		"TTL should not refresh on subsequent increments (EXPIRE NX)")
}

func TestDeviceFailureLimiter_WindowExpiryResetsCounter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, mr := newMiniredisClient(t)
	l := NewDeviceFailureLimiter(client, 3, time.Hour)

	for range 3 {
		require.NoError(t, l.Record(ctx, 42))
	}
	allowed, err := l.Allowed(ctx, 42)
	require.NoError(t, err)
	assert.False(t, allowed, "at threshold before window expiry")

	mr.FastForward(time.Hour + time.Second)

	allowed, err = l.Allowed(ctx, 42)
	require.NoError(t, err)
	assert.True(t, allowed, "counter resets after window expires")
}

func TestDeviceFailureLimiter_Clear(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, _ := newMiniredisClient(t)
	l := NewDeviceFailureLimiter(client, 3, time.Hour)

	for range 3 {
		require.NoError(t, l.Record(ctx, 42))
	}
	allowed, err := l.Allowed(ctx, 42)
	require.NoError(t, err)
	assert.False(t, allowed)

	require.NoError(t, l.Clear(ctx, 42))

	allowed, err = l.Allowed(ctx, 42)
	require.NoError(t, err)
	assert.True(t, allowed, "Clear resets the counter")
}

func TestDeviceFailureLimiter_PerUserIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, _ := newMiniredisClient(t)
	l := NewDeviceFailureLimiter(client, 3, time.Hour)

	for range 3 {
		require.NoError(t, l.Record(ctx, 42))
	}

	allowed, err := l.Allowed(ctx, 42)
	require.NoError(t, err)
	assert.False(t, allowed, "user 42 blocked")

	allowed, err = l.Allowed(ctx, 99)
	require.NoError(t, err)
	assert.True(t, allowed, "user 99 unaffected by user 42's failures")
}

func TestDeviceFailureLimiter_FailsOpenOnRedisError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, mr := newMiniredisClient(t)
	l := NewDeviceFailureLimiter(client, 3, time.Hour)

	// Record one failure so the key exists.
	require.NoError(t, l.Record(ctx, 42))
	mr.Close() // simulate Redis outage

	allowed, err := l.Allowed(ctx, 42)
	assert.Error(t, err, "Redis error surfaced")
	assert.True(t, allowed, "limiter fails open to preserve device flow on outage")
}
