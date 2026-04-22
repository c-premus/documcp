package oauth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// deviceFailureKeyPrefix namespaces the device-flow failure counter in Redis.
// Tests share the prefix so they can simulate a pre-seeded counter.
const deviceFailureKeyPrefix = "documcp:device_fail:"

// DeviceFailureLimiter tracks failed `user_code` submissions per authenticated
// user using a fixed-window counter in Redis. Replaces the session-scoped
// `device_failed_attempts` cookie value that was trivially defeated by
// clearing cookies or opening a private window (security L6).
//
// Fixed-window (not sliding) is intentional: the TTL is set once on the first
// failure and not refreshed on subsequent increments. An attacker cannot
// extend the window by trickling failures. After the window expires the
// counter starts fresh.
type DeviceFailureLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

// NewDeviceFailureLimiter constructs the limiter. A nil client, zero limit,
// or zero window produces a no-op limiter that permits every attempt — used
// in tests that don't exercise brute-force paths and in deployments that
// explicitly disable the counter via config.
func NewDeviceFailureLimiter(client *redis.Client, limit int, window time.Duration) *DeviceFailureLimiter {
	return &DeviceFailureLimiter{client: client, limit: limit, window: window}
}

// Allowed reports whether the user is below the failure threshold. A disabled
// limiter always returns true. Any Redis error returns true alongside the
// error so callers can choose to log-and-proceed (fail-open) matching the
// existing rate-limit pattern; breaking the device-flow entirely on a Redis
// blip would be a worse outcome than missing a bounded number of failures.
func (l *DeviceFailureLimiter) Allowed(ctx context.Context, userID int64) (bool, error) {
	if !l.enabled() {
		return true, nil
	}
	n, err := l.client.Get(ctx, failureKey(userID)).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return true, nil
		}
		return true, fmt.Errorf("device failure counter get: %w", err)
	}
	return n < l.limit, nil
}

// Record increments the user's failure counter, setting the window only on
// the first failure via EXPIRE NX. The MULTI/EXEC pipeline matches the
// codebase's established rate-limit pattern and requires Redis ACL
// `+@transaction`.
func (l *DeviceFailureLimiter) Record(ctx context.Context, userID int64) error {
	if !l.enabled() {
		return nil
	}
	key := failureKey(userID)
	pipe := l.client.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, l.window)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("device failure counter record: %w", err)
	}
	return nil
}

// Clear deletes the user's failure counter. Called on a successful `user_code`
// lookup so a typo earlier in the same window doesn't eat into a legitimate
// user's budget.
func (l *DeviceFailureLimiter) Clear(ctx context.Context, userID int64) error {
	if !l.enabled() {
		return nil
	}
	if err := l.client.Del(ctx, failureKey(userID)).Err(); err != nil {
		return fmt.Errorf("device failure counter clear: %w", err)
	}
	return nil
}

func (l *DeviceFailureLimiter) enabled() bool {
	return l.client != nil && l.limit > 0 && l.window > 0
}

func failureKey(userID int64) string {
	return deviceFailureKeyPrefix + strconv.FormatInt(userID, 10)
}
