# Redis

## Overview

DocuMCP requires Redis for the following features:

| Feature | Keys or channels | Client | Commands |
|---------|------------------|--------|----------|
| **Distributed rate limiting** (httprate-redis) | `documcp:rate:<hash>` | bare | `TxPipeline` (MULTI/INCRBY/EXPIRE/EXEC) per request, plus MGET |
| **Cross-instance event delivery** -- a Pub/Sub EventBus broadcasts queue events (e.g., document indexed) to all instances for SSE fan-out | channel `documcp:events` | main | PUBLISH, SUBSCRIBE |
| **Control bus** -- cross-replica control messages; the only topic today is `cache.kiwix.invalidate` | channels `documcp:control:<topic>` | main | PUBLISH, SUBSCRIBE |
| **Session store** -- browser sessions for the admin panel and OAuth flows | `session:<id>`, `user-sessions:<user id>` | main | SET, GET, DEL, SADD, SREM, SMEMBERS, EXPIRE (pipelined) |
| **Device-flow failure limiter** -- counts failed `user_code` submissions per user | `documcp:device_fail:<user id>` | bare | `TxPipeline` (MULTI/INCR/EXPIRE NX/EXEC), GET, DEL |
| **Readiness checks** -- `/health/ready`, worker `/readyz`, and the `documcp_ready` gauge | -- | bare | PING |

Redis is not optional. The application validates `REDIS_ADDR` on startup and exits if it is empty or unreachable.

## Minimum Version

Redis 7.0+ is required. The device-flow failure limiter uses `EXPIRE ... NX`, which Redis added in 7.0. ACL support (used below) is available from Redis 6. The project uses `redis:8-alpine` in development and production.

## ACL Requirements

DocuMCP needs the following Redis ACL categories. Missing any of these causes failures that may not surface as obvious errors.

| Category | Used By | Notes |
|----------|---------|-------|
| `+@read +@write` | General data operations | GET, MGET, SET, DEL, INCR, INCRBY, EXPIRE, etc. |
| `+@list +@set +@sortedset +@hash +@string` | Data type operations | Rate limit and device-flow counters (string), session index (set) |
| `+@pubsub` | EventBus, control bus | PUBLISH, SUBSCRIBE on `documcp:events` and `documcp:control:*` |
| `+@connection` | Health checks | PING, CLIENT commands |
| `+@transaction` | httprate-redis and device-flow `TxPipeline` | MULTI, EXEC, DISCARD |

DocuMCP does not call EVAL, EVALSHA, SCRIPT, KEYS, or FLUSHDB, so the ACL does not need `+@scripting` or per-command overrides for them.

Restricted categories and key patterns:

| Rule | Purpose |
|------|---------|
| `-@admin -@dangerous` | Block administrative commands |
| `~* &*` | Access all keys and all Pub/Sub channels. Session keys (`session:*`, `user-sessions:*`) do not share the `documcp:` prefix, so a `~documcp:*` pattern is not enough. |

### Full ACL Command

```
ACL SETUSER documcp on >PASSWORD +@read +@write +@list +@set +@sortedset +@hash +@string +@pubsub +@connection +@transaction -@admin -@dangerous ~* &*
```

Replace `PASSWORD` with the actual password.

### Why `+@transaction` Is Critical

httprate-redis wraps every rate-limit increment in a `TxPipeline`:

```
MULTI
INCRBY documcp:rate:<hash> 1
EXPIRE documcp:rate:<hash> <3 × window>
EXEC
```

The device-flow failure limiter uses the same pattern (`INCR` + `EXPIRE ... NX`).

When the ACL denies MULTI/EXEC, Redis returns an error response, but the pipelined commands have already been buffered. go-redis reads the error but leaves unread data in the connection buffer. This produces `Conn has unread data` warnings in logs and causes connection pool churn as poisoned connections are recycled.

The symptom is subtle -- rate limiting still appears to work intermittently, but the connection pool degrades under load.

## Configuration

All settings are read from environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | -- | Redis address (host:port). **Required.** |
| `REDIS_USERNAME` | `""` | ACL username |
| `REDIS_PASSWORD` | `""` | Password or ACL password |
| `REDIS_DB` | `0` | Database number |
| `REDIS_POOL_SIZE` | `10` | Maximum connections in the main client pool. Must be non-negative. |
| `REDIS_MIN_IDLE_CONNS` | `2` | Minimum idle connections maintained (main client) |
| `REDIS_MAX_ACTIVE_CONNS` | `0` | Maximum active connections (0 = no limit; main client) |
| `REDIS_CONN_MAX_IDLE_TIME` | `5m` | Close idle connections after this duration (main client) |
| `REDIS_DIAL_TIMEOUT` | `5s` | Timeout for new connections (main client) |
| `REDIS_READ_TIMEOUT` | `5s` | Timeout for read operations (main client) |
| `REDIS_WRITE_TIMEOUT` | `5s` | Timeout for write operations (main client) |
| `REDIS_MAX_RETRIES` | `3` | Maximum retries on failed commands (main client only) |
| `REDIS_TLS_ENABLED` | `false` | Connect over TLS (minimum TLS 1.2). Applies to both clients. |
| `REDIS_TLS_CA_FILE` | `""` | PEM CA bundle used to verify the Redis server certificate. Empty = system trust store. Only read when `REDIS_TLS_ENABLED=true`. |

`REDIS_ADDR` must be non-empty and `REDIS_POOL_SIZE` must be non-negative; startup fails otherwise. All other fields use their defaults when unset.

## Client Architecture

DocuMCP creates two separate Redis clients on startup. Both use `Protocol: 2` (RESP2), `DisableIdentity: true`, `ContextTimeoutEnabled: true`, and the same address, credentials, database, and TLS settings.

### Main Client

Used by the EventBus, the control bus, and the session store.

- Pool size, timeouts, and retries are configurable via the environment variables above
- Instrumented with redisotel for OpenTelemetry tracing (see [OBSERVABILITY.md](OBSERVABILITY.md))
- Retry count from `REDIS_MAX_RETRIES` (default 3)
- Source of the `documcp_redis_*` pool metrics

### Bare Client

Used by rate limiting (httprate-redis), the device-flow failure limiter, and readiness checks (`/health/ready`, worker `/readyz`, and the `documcp_ready` gauge). Isolated from the main client so retry-induced partial responses cannot poison shared connections, and so probe traffic does not emit trace spans.

- `PoolSize: 3` (hardcoded, not configurable)
- `MinIdleConns: 1`
- `MaxRetries: -1` (no retries -- a failed MULTI/EXEC should not be retried mid-pipeline)
- `ReadTimeout: 500ms`, `WriteTimeout: 500ms`
- Dial timeout, idle-connection timeout, and active-connection limit use go-redis defaults; the `REDIS_*` pool and timeout variables do not apply
- No redisotel tracing -- rate-limit counter increments are high-frequency and low-value to trace
- Not covered by the `documcp_redis_*` pool metrics

### Why RESP2

go-redis v9 defaults to RESP3, which introduces server push notifications. DocuMCP does not use RESP3-specific features (client-side caching, push notifications), so both clients pin `Protocol: 2` to avoid the overhead.

### Why DisableIdentity

go-redis sends `CLIENT SETINFO` on each new connection by default. Setting `DisableIdentity: true` skips these round-trips, which reduces connection setup latency and avoids stale buffer data on high-latency networks.

## Troubleshooting

### "Conn has unread data" warnings

The Redis ACL is missing `+@transaction`. MULTI/EXEC are denied, leaving partial error responses in the connection buffer. Add `+@transaction` to the user's ACL and restart the application.

See the [Why `+@transaction` Is Critical](#why-transaction-is-critical) section above.

### Rate limits stop being shared across instances

When a rate-limit call to Redis fails, httprate-redis switches that limiter to a per-instance in-memory counter and pings Redis every 200 ms until it answers, then switches back. While the fallback is active, each instance enforces the limit on its own, so a client spread across N instances can make up to N times the configured requests. Requests are not rejected because of the Redis error.

The rate limiter also installs an error handler that returns 503 with a JSON error envelope (`Rate limiting is temporarily unavailable. Please retry shortly.`) and logs `rate limiter backend error; rejecting request`. With the in-memory fallback enabled (the httprate-redis default, which DocuMCP does not change), a Redis error does not reach that handler.

The other Redis-backed features have no in-memory fallback: the session store, EventBus, and control bus depend on Redis being reachable, and `/health/ready` returns 503 while it is not. The device-flow failure limiter fails open: on a Redis error it logs a warning and allows the attempt.

### Connection refused on startup

Check that:

- `REDIS_ADDR` is set and points to a reachable Redis instance
- The password matches (if ACL authentication is configured)
- `REDIS_TLS_ENABLED` matches the server (a TLS client against a plaintext port, or the reverse, fails the startup Ping)
- Network/firewall rules permit the connection

### Pool exhaustion

Monitor the `documcp_redis_*` Prometheus metrics (main client only):

- `documcp_redis_pool_timeouts_total` -- should be 0; non-zero indicates pool exhaustion
- `documcp_redis_pool_misses_total` -- frequent misses suggest the pool is undersized
- `documcp_redis_active_connections` -- compare against `REDIS_POOL_SIZE`

Increase `REDIS_POOL_SIZE` if the pool is consistently full. See [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for the full metric listing and PromQL examples.
