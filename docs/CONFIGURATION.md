# Configuration

DocuMCP is configured entirely through environment variables. `.env.example` is the authoritative list; this document groups those variables by concern and explains how each one is used.

"Required" in the tables below means startup fails when the variable is unset. "Prod" means the variable is required only when `APP_ENV=production`. Defaults shown are what the application applies when the variable is unset.

For the minimum set needed to boot a deployment, see [Required for startup](../README.md#configuration) in the README. Everything else lives below.

## Application

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_NAME` | No | `DocuMCP` | Display name used in logs and the admin UI |
| `APP_ENV` | No | `development` | Environment: `development`, `staging`, `production`, `testing` |
| `APP_DEBUG` | No | `false` | Enables verbose debug logging |
| `APP_URL` | No | `http://localhost` | Public application URL (also seeds the OAuth resource indicator allowlist) |
| `APP_TIMEZONE` | No | `UTC` | Server timezone |
| `INTERNAL_API_TOKEN` | Prod | -- | Bearer token guarding `/metrics` and `/health/ready`. Generate `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | Prod | -- | 64-char hex (32 bytes) for AES-256-GCM encryption of stored Git tokens. Generate `openssl rand -hex 32` |

## Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERVER_HOST` | No | `0.0.0.0` | Listen address |
| `SERVER_PORT` | No | `8080` | Listen port (becomes the HTTP→HTTPS redirect port when TLS is enabled) |
| `TRUSTED_PROXIES` | No | -- | CIDR ranges for trusted reverse proxies (comma-separated). Required for correct `X-Forwarded-For` handling |
| `SERVER_READ_TIMEOUT` | No | `30s` | Full request read deadline |
| `SERVER_WRITE_TIMEOUT` | No | `30s` | Response write deadline |
| `SERVER_IDLE_TIMEOUT` | No | `120s` | Keep-alive idle timeout |
| `SERVER_READ_HEADER_TIMEOUT` | No | `5s` | Header read deadline (slowloris guard) |
| `SERVER_SHUTDOWN_TIMEOUT` | No | `5s` | Graceful shutdown deadline |
| `SERVER_REQUEST_TIMEOUT` | No | `60s` | Per-request context timeout (excludes `/documcp` and SSE streams) |
| `SERVER_MAX_BODY_SIZE` | No | `1048576` | Max request body in bytes (1 MiB; uploads use a separate limit) |
| `SERVER_HSTS_MAX_AGE` | No | `63072000` | HSTS max-age in seconds (2 years; `0` disables) |
| `SERVER_SSE_HEARTBEAT_INTERVAL` | No | `15s` | Server-Sent Events keepalive interval |

## TLS (direct HTTPS, no reverse proxy)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TLS_ENABLED` | No | `false` | Terminate TLS directly in the Go process |
| `TLS_PORT` | No | `8443` | HTTPS listen port |
| `TLS_CERT_FILE` | No | -- | PEM certificate path (empty + TLS enabled = ephemeral self-signed) |
| `TLS_KEY_FILE` | No | -- | PEM private key path |

## Database (PostgreSQL)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_HOST` | Yes | `127.0.0.1` | PostgreSQL host |
| `DB_PORT` | No | `5432` | PostgreSQL port |
| `DB_DATABASE` | Yes | `documcp` | Database name |
| `DB_USERNAME` | Yes | `documcp` | Database user |
| `DB_PASSWORD` | Prod | -- | Database password |
| `DB_SSLMODE` | No | `require` | SSL mode: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` |
| `DB_MAX_OPEN_CONNS` | No | `25` | Max pool size (raise to 40–50 for combined `serve --with-worker` mode) |
| `DB_MAX_IDLE_CONNS` | No | `5` | Max idle connections in the pool |
| `DB_MAX_LIFETIME` | No | `5m` | Max lifetime per connection |
| `DB_PGX_MIN_CONNS` | No | `5` | Minimum idle connections kept warm |
| `DB_PGX_MAX_CONN_LIFETIME` | No | `30m` | pgx-level max connection lifetime |
| `DB_PGX_MAX_CONN_IDLE_TIME` | No | `5m` | pgx-level max idle time before close |

## Redis

See [docs/REDIS.md](REDIS.md) for ACL requirements, client architecture, and troubleshooting.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_ADDR` | Yes | `localhost:6379` | Redis address (`host:port`) |
| `REDIS_USERNAME` | No | -- | Redis 6+ ACL username (empty = default user) |
| `REDIS_PASSWORD` | No | -- | Redis password |
| `REDIS_DB` | No | `0` | Redis database number |
| `REDIS_POOL_SIZE` | No | `10` | Connection pool size (`0` = `10 * GOMAXPROCS`) |
| `REDIS_MIN_IDLE_CONNS` | No | `2` | Idle connections kept warm |
| `REDIS_CONN_MAX_IDLE_TIME` | No | `5m` | Max idle time before close |
| `REDIS_DIAL_TIMEOUT` | No | `5s` | Connection establishment timeout |
| `REDIS_READ_TIMEOUT` | No | `5s` | Socket read timeout |
| `REDIS_WRITE_TIMEOUT` | No | `5s` | Socket write timeout |
| `REDIS_MAX_RETRIES` | No | `3` | Max command retries (`0` disables) |
| `REDIS_MAX_ACTIVE_CONNS` | No | `0` | Max active connections (`0` = unlimited) |
| `REDIS_TLS_ENABLED` | No | `false` | TLS for Redis (required for cloud-managed Redis) |
| `REDIS_TLS_CA_FILE` | No | -- | Optional CA certificate (PEM); empty = system CA pool |

## OIDC Authentication

DocuMCP requires an OpenID Connect provider for user login. Set `OIDC_PROVIDER_URL` + `OIDC_CLIENT_ID` to enable auto-discovery; at least one of `OIDC_ADMIN_GROUPS` or `OIDC_BOOTSTRAP_ADMIN_EMAIL` must also be set so an admin can ever exist.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OIDC_PROVIDER_URL` | No | -- | OpenID Connect provider URL (enables auto-discovery) |
| `OIDC_CLIENT_ID` | No | -- | OIDC client ID |
| `OIDC_CLIENT_SECRET` | No | -- | OIDC client secret |
| `OIDC_REDIRECT_URI` | No | -- | OIDC callback URL |
| `OIDC_SCOPES` | No | `openid,profile,email` | Comma-separated requested scopes |
| `OIDC_ADMIN_GROUPS` | No | -- | Comma-separated group names that grant admin access |
| `OIDC_BOOTSTRAP_ADMIN_EMAIL` | No | -- | Promotes first OIDC user with this verified email to admin (create branch only) |
| `OIDC_AUTHORIZATION_URL` | No | -- | Manual override (skips discovery) |
| `OIDC_TOKEN_URL` | No | -- | Manual override |
| `OIDC_USERINFO_URL` | No | -- | Manual override |
| `OIDC_JWKS_URL` | No | -- | Manual override |
| `OIDC_END_SESSION_URL` | No | -- | RP-Initiated Logout endpoint (auto-discovered when `OIDC_PROVIDER_URL` is set) |

## OAuth 2.1 Authorization Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OAUTH_SESSION_SECRET` | Prod | -- | Session secret (min 32 bytes); derives CSRF and token HMAC keys via HKDF. Generate `openssl rand -base64 32` |
| `OAUTH_SESSION_SECRET_PREVIOUS` | No | -- | Previous session secret for key rotation |
| `OAUTH_SESSION_MAX_AGE` | No | `720h` | Session lifetime (30 days) |
| `HKDF_SALT` | Yes | -- | Per-deployment salt for HKDF key derivation. Required (min 16 chars) in every environment. Generate `openssl rand -base64 24` |
| `OAUTH_AUTHORIZATION_CODE_LIFETIME` | No | `10m` | Authorization code TTL |
| `OAUTH_ACCESS_TOKEN_LIFETIME` | No | `1h` | Access token TTL |
| `OAUTH_REFRESH_TOKEN_LIFETIME` | No | `720h` | Refresh token TTL (30 days) |
| `OAUTH_DEVICE_CODE_LIFETIME` | No | `10m` | Device code TTL |
| `OAUTH_DEVICE_POLLING_INTERVAL` | No | `5s` | Minimum device polling interval |
| `OAUTH_REGISTRATION_ENABLED` | No | `true` | Enables RFC 7591 dynamic client registration |
| `OAUTH_REGISTRATION_REQUIRE_AUTH` | No | `true` | When `false`, anonymous registration is allowed but constrained (public clients, read-only scopes, no device_code; rate-limited) |
| `OAUTH_CLIENT_TOUCH_TIMEOUT` | No | `3s` | Timeout for fire-and-forget `last_used_at` updates |
| `OAUTH_SCOPE_GRANT_TTL` | No | `720h` | Time-bounded scope-grant lifetime (30 days; `0` = no expiry) |
| `OAUTH_ALLOWED_RESOURCES` | No | _derived_ | RFC 8707 resource indicator allowlist (comma-separated absolute URIs). Defaults to `[APP_URL, APP_URL+DOCUMCP_ENDPOINT]` |

## Storage

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `STORAGE_DRIVER` | No | `local` | Blob backend: `local` / `fs` (node-local), `s3` (any S3-compatible service) |
| `STORAGE_BASE_PATH` | No | `./storage` | Filesystem root — always required (workers stage git clones and extraction scratch here, even with `s3`) |
| `STORAGE_DOCUMENT_PATH` | No | `documents` | Subdirectory under `STORAGE_BASE_PATH` for the FSBlob document tree |
| `STORAGE_TEMP_PATH` | No | `tmp` | Subdirectory for transient worker scratch |
| `STORAGE_MAX_UPLOAD_SIZE` | No | `52428800` | Max upload file size in bytes (50 MiB) |
| `STORAGE_MAX_EXTRACTED_TEXT` | No | `52428800` | Max decompressed text per file in bytes (50 MiB) |
| `STORAGE_MAX_ZIP_FILES` | No | `100` | Max files in a DOCX/EPUB ZIP archive |
| `STORAGE_MAX_SHEETS` | No | `100` | Max sheets in an XLSX file |
| `STORAGE_S3_ENDPOINT` | No† | -- | S3 endpoint URL (empty = AWS default; required for R2, B2, Garage, SeaweedFS, etc.) |
| `STORAGE_S3_BUCKET` | No† | -- | Target bucket name |
| `STORAGE_S3_REGION` | No† | `us-east-1` | AWS region string (`us-east-1` is a safe placeholder for Garage/SeaweedFS) |
| `STORAGE_S3_ACCESS_KEY_ID` | No† | -- | Static access key |
| `STORAGE_S3_SECRET_ACCESS_KEY` | No† | -- | Static secret key |
| `STORAGE_S3_USE_PATH_STYLE` | No | `true` | Force path-style addressing; required for most self-hosted backends |
| `STORAGE_S3_FORCE_SSL` | No | `true` | Reject plaintext S3 endpoints at startup |

† Required when `STORAGE_DRIVER=s3`. The `s3` driver speaks the S3 API and works against AWS S3, Cloudflare R2, Backblaze B2, Wasabi, Garage, SeaweedFS, and any other S3-compatible service. Keys use the same `{file_type}/{uuid}.{ext}` layout as the filesystem driver, so switching backends requires no database migration.

## External Services: Kiwix

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `KIWIX_CACHE_TTL` | No | `1h` | TTL for the Kiwix client factory cache |
| `KIWIX_HTTP_TIMEOUT` | No | `10s` | Per-request HTTP timeout |
| `KIWIX_HEALTH_CHECK_TIMEOUT` | No | `5s` | Health probe timeout |
| `KIWIX_FEDERATED_SEARCH_TIMEOUT` | No | `3s` | Deadline for Kiwix fan-out during `unified_search` |
| `KIWIX_FEDERATED_MAX_ARCHIVES` | No | `10` | Max archives searched in parallel |
| `KIWIX_FEDERATED_PER_ARCHIVE_LIMIT` | No | `3` | Max results per archive |

## External Services: Git Templates

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GIT_MAX_FILE_SIZE` | No | `1048576` | Max bytes per extracted file (1 MiB) |
| `GIT_MAX_TOTAL_SIZE` | No | `10485760` | Max bytes per template after extraction (10 MiB) |

## Security

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SSRF_DIALER_TIMEOUT` | No | `10s` | Timeout for the SSRF-guarded outbound HTTP dialer (Kiwix, Git, OIDC) |

## Queue Workers

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `QUEUE_HIGH_WORKERS` | No | `10` | River high-priority queue concurrency |
| `QUEUE_DEFAULT_WORKERS` | No | `5` | River default queue concurrency |
| `QUEUE_LOW_WORKERS` | No | `2` | River low-priority queue concurrency |
| `WORKER_HEALTH_PORT` | No | `9090` | Health endpoint port for worker-only mode |

## Lifecycle

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `APP_QUEUE_STOP_TIMEOUT` | No | `10s` | Deadline for River queue drain on shutdown |
| `APP_TRACER_STOP_TIMEOUT` | No | `5s` | Deadline for OpenTelemetry tracer flush on shutdown |

## MCP Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DOCUMCP_ENDPOINT` | No | `/documcp` | URL path the MCP server is mounted at |
| `DOCUMCP_NAME` | No | `DocuMCP` | Server name advertised in MCP `initialize` |
| `DOCUMCP_VERSION` | No | `dev` | Set automatically from the git tag via ldflags — do not override manually |

## Observability

See [docs/OBSERVABILITY.md](OBSERVABILITY.md) for architecture and [docs/PROMETHEUS_METRICS.md](PROMETHEUS_METRICS.md) for metric listings.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_ENABLED` | No | `false` | Enable OpenTelemetry tracing |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | -- | OTLP HTTP exporter endpoint (e.g., `tempo:4318`) |
| `OTEL_SERVICE_NAME` | No | `documcp` | Service name in traces |
| `OTEL_INSECURE` | No | `false` | Use HTTP instead of HTTPS for the OTLP exporter |
| `OTEL_SAMPLE_RATE` | No | `1.0` | Trace sampling rate (0.0–1.0); ignores upstream sampling decisions |
| `OTEL_ENVIRONMENT` | No | -- | `deployment.environment` resource attribute |
| `SENTRY_DSN` | No | -- | Sentry/GlitchTip DSN for error tracking (empty = disabled) |
| `SENTRY_SAMPLE_RATE` | No | `1.0` | Error sample rate (0.0–1.0) |
| `VITE_SENTRY_DSN` | No | -- | Frontend Sentry DSN — read by Vite at `npm run build` time only |

## Scheduler (cron expressions, empty disables a job)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SCHEDULER_ENABLED` | No | `false` | Master switch for periodic jobs |
| `SCHEDULER_KIWIX_SCHEDULE` | No | `0 */6 * * *` | Kiwix archive metadata refresh |
| `SCHEDULER_GIT_SCHEDULE` | No | `0 * * * *` | Git template repository sync |
| `SCHEDULER_OAUTH_CLEANUP_SCHEDULE` | No | `0 * * * *` | Expired OAuth token / scope-grant cleanup |
| `SCHEDULER_ORPHANED_FILES_SCHEDULE` | No | `0 2 * * *` | Orphan blob cleanup (no DB row) |
| `SCHEDULER_SEARCH_VERIFY_SCHEDULE` | No | `0 3 * * *` | Search-index integrity verification |
| `SCHEDULER_SOFT_DELETE_PURGE_SCHEDULE` | No | `0 4 * * *` | Permanent deletion of soft-deleted documents |
| `SCHEDULER_ZIM_CLEANUP_SCHEDULE` | No | `0 5 * * *` | Stale ZIM archive cache cleanup |
| `SCHEDULER_HEALTH_CHECK_SCHEDULE` | No | `*/15 * * * *` | External service health probing |
