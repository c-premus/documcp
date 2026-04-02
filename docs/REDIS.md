# Redis

## Overview

DocuMCP requires Redis for two features:

1. **Distributed rate limiting** -- httprate-redis uses `TxPipeline` (MULTI/INCR/EXPIRE/EXEC) on every rate-limited request to atomically increment and expire counters.
2. **Cross-instance event delivery** -- A Redis Pub/Sub EventBus broadcasts queue events (e.g., document indexed) to all instances for SSE fan-out.

Redis is not optional. The application validates `REDIS_ADDR` on startup and exits if it is empty or unreachable.

## Minimum Version

Redis 6+ is required for ACL support. The project uses `redis:8-alpine` in development and production.

## ACL Requirements

DocuMCP needs the following Redis ACL categories. Missing any of these causes failures that may not surface as obvious errors.

| Category | Used By | Notes |
|----------|---------|-------|
| `+@read +@write` | General data operations | GET, SET, DEL, etc. |
| `+@list +@set +@sortedset +@hash +@string` | Data type operations | Rate limit counters, general storage |
| `+@pubsub` | EventBus | PUBLISH, SUBSCRIBE on `documcp:events` channel |
| `+@connection` | Health checks | PING, CLIENT commands |
| `+@transaction` | httprate-redis TxPipeline | MULTI, EXEC, DISCARD |
| `+@scripting` | Future use | Lua scripts (not used today, included for forward compatibility) |

Restricted categories and specific overrides:

| Rule | Purpose |
|------|---------|
| `-@admin -@dangerous` | Block administrative commands |
| `+keys +flushdb` | Override category restrictions for specific commands |
| `~* &*` | Access all keys and all Pub/Sub channels |

### Full ACL Command

```
ACL SETUSER documcp on >PASSWORD +@read +@write +@list +@set +@sortedset +@hash +@string +@pubsub +@connection +@transaction +@scripting -@admin -@dangerous +keys +flushdb ~* &*
```

Replace `PASSWORD` with the actual password.

### Why `+@transaction` Is Critical

httprate-redis wraps every rate-limit check in a `TxPipeline`:

```
MULTI
INCR rate:key
EXPIRE rate:key 60
EXEC
```

When the ACL denies MULTI/EXEC, Redis returns an error response, but the pipelined commands have already been buffered. go-redis reads the error but leaves unread data in the connection buffer. This produces `Conn has unread data` warnings in logs and causes connection pool churn as poisoned connections are recycled.

The symptom is subtle -- rate limiting still appears to work intermittently, but the connection pool degrades under load.

## Configuration

All settings are read from environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | -- | Redis address (host:port). **Required.** |
| `REDIS_USERNAME` | `""` | ACL username (Redis 6+) |
| `REDIS_PASSWORD` | `""` | Password or ACL password |
| `REDIS_DB` | `0` | Database number |
| `REDIS_POOL_SIZE` | `10` | Maximum connections in the main client pool |
| `REDIS_MIN_IDLE_CONNS` | `2` | Minimum idle connections maintained |
| `REDIS_MAX_ACTIVE_CONNS` | `0` | Maximum active connections (0 = no limit) |
| `REDIS_CONN_MAX_IDLE_TIME` | `5m` | Close idle connections after this duration |
| `REDIS_DIAL_TIMEOUT` | `5s` | Timeout for new connections |
| `REDIS_READ_TIMEOUT` | `5s` | Timeout for read operations |
| `REDIS_WRITE_TIMEOUT` | `5s` | Timeout for write operations |
| `REDIS_MAX_RETRIES` | `3` | Maximum retries on failed commands (main client only) |

Only `REDIS_ADDR` is validated as non-empty. All other fields use their defaults when unset.

## Client Architecture

DocuMCP creates two separate Redis clients on startup. Both use `Protocol: 2` (RESP2) and `DisableIdentity: true`.

### Main Client

Used by the EventBus, health checks, and general operations.

- Pool size, timeouts, and retries are configurable via the environment variables above
- Instrumented with redisotel for OpenTelemetry tracing (see [OBSERVABILITY.md](OBSERVABILITY.md))
- Retry count from `REDIS_MAX_RETRIES` (default 3)

### Rate Limit Client

Dedicated to httprate-redis `TxPipeline` operations. Isolated from the main client to prevent retry-induced partial responses from poisoning shared connections.

- `PoolSize: 3` (hardcoded, not configurable)
- `MinIdleConns: 1`
- `MaxRetries: -1` (no retries -- a failed MULTI/EXEC should not be retried mid-pipeline)
- `ReadTimeout: 500ms`, `WriteTimeout: 500ms`
- No redisotel tracing -- rate-limit counter increments are high-frequency and low-value to trace

### Why RESP2

go-redis v9.18 defaults to RESP3, which introduces server push notifications. DocuMCP does not use RESP3-specific features (client-side caching, push notifications), so both clients pin `Protocol: 2` to avoid the overhead.

### Why DisableIdentity

go-redis sends `CLIENT SETINFO` on each new connection by default. Setting `DisableIdentity: true` skips these round-trips, which reduces connection setup latency and avoids stale buffer data on high-latency networks.

## Troubleshooting

### "Conn has unread data" warnings

The Redis ACL is missing `+@transaction`. MULTI/EXEC are denied, leaving partial error responses in the connection buffer. Add `+@transaction` to the user's ACL and restart the application.

See the [Why `+@transaction` Is Critical](#why-transaction-is-critical) section above.

### Connection refused on startup

Check that:

- `REDIS_ADDR` is set and points to a reachable Redis instance
- The password matches (if ACL authentication is configured)
- Network/firewall rules permit the connection

### Pool exhaustion

Monitor the `documcp_redis_*` Prometheus metrics:

- `documcp_redis_pool_timeouts_total` -- should be 0; non-zero indicates pool exhaustion
- `documcp_redis_pool_misses_total` -- frequent misses suggest the pool is undersized
- `documcp_redis_active_connections` -- compare against `REDIS_POOL_SIZE`

Increase `REDIS_POOL_SIZE` if the pool is consistently full. See [PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for the full metric listing and PromQL examples.
