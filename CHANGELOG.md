# Changelog

All notable changes to DocuMCP-Go are documented in this file.

The format is based on [Keep a Changelog]. The project follows
[Semantic Versioning] with the **pre-v1.0 convention that breaking changes bump
minor, not major** — a `feat!:` or `BREAKING CHANGE` commit while the major
version is `0` produces a minor bump; `v1.0.0` is reserved for an explicit
manual cut.

[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html

## [Unreleased]

## [0.27.0] — 2026-04-29

### Added

- **Frontend mobile-card layouts on every list view.** The shared
  `DataTable.vue` gained a `mobile-card` slot that renders a card-stacked
  layout at `<md` while the existing table stays at `md+`. Per-domain card
  SFCs follow the same convention as the row-action SFCs:
  `DocumentMobileCard`, `DocumentTrashMobileCard`, `UserMobileCard` (with
  inline admin toggle), `OAuthClientMobileCard`, `ExternalServiceMobileCard`
  (toggle + priority controls), `ZimArchiveMobileCard`,
  `GitTemplateMobileCard` (admin-gated actions), `QueueJobMobileCard` (full
  error message in `role="alert"` rather than the desktop's 80-char title-attr
  truncation). Desktop is pixel-identical; views without a `mobile-card` slot
  keep the existing table at all viewports.
- **Mermaid diagram rendering in `ContentViewer`.** Fenced ` ```mermaid `
  blocks in markdown documents (DocumentDetailView) and git template files
  (GitTemplateFilesView) now render as SVG via dynamically-imported Mermaid
  v11.14.0. Main bundle unchanged at 38.51 KB / 12.90 KB gzipped — Mermaid
  core + per-diagram-type chunks (flowchart, sequence, gantt, …) load only
  when a diagram is rendered, and only for the specific diagram type used.
  `securityLevel: 'strict'` disables click handlers and external href in
  Mermaid's output. Render failures `console.warn` and leave the source
  block intact rather than failing silently. Light/dark theme detected via
  `html.dark` class.
- **GitTemplateFilesView mobile + markdown.** State-machine pane swap on
  `<md` (tree visible until selection, content visible after, mobile-only
  "Back to files" button); markdown / HTML files now route through
  `ContentViewer` (the desktop-only `<pre>` block kept rendering README.md
  as raw text on every viewport). Code files keep their existing raw `<pre>`
  display — syntax highlighting is a separate scope.
- **`CHANGELOG.md`** at the repo root, in [Keep a Changelog] format.
  Canonical record of release content because the GitHub mirror
  squash-merges every dev → main change, collapsing per-commit history.

### Notes

- No re-auth required at deploy. No schema changes. Pure frontend release.
- Mermaid v11 transitively pulls `uuid <14.0.0` (moderate advisory on a
  buffer-bounds path that requires caller-supplied buffers). Not exploitable
  in our integration; watching for an upstream `uuid` bump.

## [0.26.1] — 2026-04-28

### Security

- **postcss → 8.5.10** (Renovate, transitive dev-dep CVE).

## [0.26.0] — 2026-04-25

### Added

- **Admin sessions revocation bundle.** Three new admin endpoints under
  `/api/admin/users/{id}/sessions` — `GET` (list session IDs), `DELETE
  /{sessionID}` (revoke one), `DELETE` (revoke all + return count). Backed
  by the v0.25.0 Redis-backed session store's `RevokeSession` /
  `RevokeUserSessions` / `ListUserSessions` primitives. Vue 3 admin UI
  surfaces this via `UserSessionsModal` reachable from `UserRowActions`.
- **Auto-revoke on user delete + admin demote.** `UserHandler.Delete` calls
  `RevokeUserSessions` after the repo write succeeds; `ToggleAdmin` calls it
  only when the post-toggle `IsAdmin` is `false` (demote — the threat-shaped
  event). Promote is a no-op because middleware re-reads `is_admin` per
  request. Best-effort: revoker errors `Warn`-log without failing the user
  op, mirroring the orphaned-blob cleanup pattern.

### Fixed

- **Grafana dashboard panel rows**: panels were placed in the wrong section
  rows after a previous reorganization; the correct groupings are now
  restored.

### Notes

- No re-auth required at deploy. Schema unchanged from v0.25.x.
- Local-test wiring: `UserHandler` accepts a narrow `sessionRevoker`
  interface; nil revoker keeps existing test fixtures compiling and running
  with the auto-revoke + new endpoints as no-ops.

## [0.25.2] — 2026-04-24

### Fixed

- **Tracing root-span orphan in Grafana / Tempo.** HTTP middleware now
  checks `SpanContextFromContext(parentCtx).IsSampled()` before starting the
  request span. When upstream (Traefik / mcp-gate) sends a valid but
  unsampled `traceparent`, the middleware swaps `parentCtx` back to
  `r.Context()` so DocuMCP starts a fresh trace root. Cross-service
  correlation preserved when upstream did sample; orphan traces eliminated
  when it didn't. The `AlwaysSample` choice (see `tracer.go` header) by
  itself didn't undo the parent link extracted from inbound traceparent.

## [0.25.1] — 2026-04-23

### Security

- **`golang.org/x/image` → v0.39.0** (CVE-2026-33813).

### Fixed

- **`deploy.yaml` Grafana dashboard path.** Provisioned dashboards now go to
  `dashboards/documcp/` instead of the retired `dashboards/applications/`
  bucket; the missing-path issue had blocked v0.25.0 from production.
- **SSE reconnect goroutine leak in tests**: fake timers stop the reconnect
  goroutine that kept running after test teardown.

## [0.25.0] — 2026-04-22

### Added

- **Redis-backed session store** replacing `sessions.NewCookieStore`. Cookie
  carries only a securecookie-signed session ID; the Values payload (`user_id`,
  `login_at`, OIDC state, `id_token`) lives at Redis `session:<id>`. Per-user
  index at `user-sessions:<id>` (Set) populated on every `Save` carrying a
  non-zero `user_id`. New `RevokeSession` / `RevokeUserSessions` /
  `ListUserSessions` primitives unlock server-side revocation. TTL =
  min(`SessionMaxAge`, `SessionAbsoluteMaxAge`).
- **`ENCRYPTION_KEY` rotation runbook** with versioned ciphertext
  `v<hex>$<base64>`. New `ENCRYPTION_KEY_PREVIOUS` env var holds a retired
  key for decrypt-only fallback. `documcp rekey` Cobra subcommand walks
  `external_services.api_key` and `git_templates.git_token`, re-encrypting
  every row that isn't already under the primary. Idempotent; exits non-zero
  when `ENCRYPTION_KEY` is empty.
- **SSE stream session re-validation** on every heartbeat tick.
  `SSEHandler` accepts a `SessionValidator` (FindUserByID +
  FindAccessTokenByID) and re-checks user existence, token revocation /
  expiry, and admin status. Failure drops the stream; client reconnects
  through fresh middleware. Demoted users on the user stream update their
  filter state in place.
- **Device-flow brute-force counter in Redis.** `oauth.DeviceFailureLimiter`
  wraps `BareRedisClient` with `INCR` + `EXPIRE NX` MULTI pipeline.
  Fixed-window (TTL set once, never refreshed) so attackers can't extend the
  window by trickling. Keyed on `user_id`. Configurable via
  `OAUTH_DEVICE_FAILURE_LIMIT` (default 5) and `OAUTH_DEVICE_FAILURE_WINDOW`
  (default `1h`). Replaces the session-cookie counter that was defeated by
  clearing cookies.

### Breaking

- **First boot invalidates all existing session cookies.** Users
  re-authenticate once through OIDC. Same no-grandfathering precedent as
  the v0.24.0 `login_at` rollout and the v0.20.0 RFC 8707 audience binding.
  Pre-v1 minor bump (feat! → minor per project convention).
- **30-day session `MaxAge`** is now bounded by the 7-day
  `OAUTH_SESSION_ABSOLUTE_MAX_AGE` cap introduced in v0.24.0.

## [0.24.1] — 2026-04-22

### Added

- **MCP contract regression test** drives the real SDK client against all
  four `{ZimEnabled, GitTemplatesEnabled}` combinations; asserts the
  SDK-published `ListTools` / `ListPrompts` set matches the contract's
  conditional-registration partitions.
- **Search query retention** — `SearchQueryRepository.DeleteOlderThan` +
  `CleanupSearchQueriesWorker` (River periodic, default `0 3 * * *`).
  Configurable via `SEARCH_QUERY_RETENTION` (default `2160h` / 90 days; `0`
  disables). Bounds table growth at the source; `PopularQueries` aggregation
  scan is bounded as a consequence.

### Changed

- **`ZimArchiveRepository.List` returns `(rows, total, error)`** via
  `COUNT(*) OVER ()` in a single query. `CountFiltered` deleted.
  Single-RTT pagination now applies across all four list repos: documents,
  oauth_clients, users, zim_archives.
- **Stable query text** on external service / zim archive list queries:
  absent filters bind as typed NULL via `($N::text IS NULL OR col = $N)`,
  preserving pgx prepared-statement cache hits across repeated admin renders.
- **`viewHarness.ts`** consolidates Pinia + fetch + auth scaffolding across
  8 view test files (−188 lines net).
- **`QueueJobErrorCell.vue`** extracted; `grep "h('"` across `src/views/*.vue`
  now returns zero hits — every cell is a one-line wrapper over a shared SFC.

### Removed

- Dead `notifications` / `add()` / `remove()` toast API in
  `stores/notifications.ts` (zero consumers; `vue-sonner`'s `toast` covers
  the actual use case).

## [0.24.0] — 2026-04-22

### Added

- **Session `login_at` + absolute lifetime.** `OAUTH_SESSION_ABSOLUTE_MAX_AGE`
  env (default `168h` / 7d). Sessions without a `login_at` anchor are treated
  as stale.
- **Versioned HMAC token hashes.** Hashes prefixed `v<version>$<hex>`;
  version byte derived as the first hex char of `sha256(secret)` so it
  remains stable when a secret moves from primary to retired. `ValidateAccessToken`,
  `ExchangeAuthorizationCode`, `RefreshAccessToken`, `ExchangeDeviceCode`
  iterate configured keys on verify. Boot fails when both keys derive to the
  same version byte (1/16 collision).
- **Logout opt-in OAuth token revocation.** `?revoke_oauth=true` query param
  triggers `RevokeUserTokensSince(userID, login_at)` in one transaction.
  Frontend store action `logout({ revokeOAuth: true })` appends the param.
- **Per-route rate limits** on `/api` root (300/min before bearer-token DB
  lookup) and `/oauth/authorize*` (30/min).
- **Strict `client_secret_basic` discipline.** Token endpoint accepts
  `Authorization: Basic` per RFC 6749 §2.3.1. Both Basic and body credentials
  → 400. `application/json` → 415 (form-encoded only per RFC 6749 §3.2).
- **Token replay detection.** Auth-code + refresh lineage share one
  `authorization_code_id`; reuse on either side revokes the entire
  descendant set. Prometheus counter exposes the signal.
- **`SafeTransportAllowPrivate` installed into go-git** via `InstallProtocol`
  registry under `sync.Once`. Plugs the DNS-rebinding TOCTOU on private-IP
  rebind for git clone.
- **Foundation rollback** is now a LIFO `[]func()` stack — each resource
  acquisition pushes a cleanup; a single top-of-function deferred loop drains
  in reverse on init failure.
- **`foundationCtx`** threaded into `NewRedisEventBus` + `ControlBus.Subscribe`
  so long-running subscribers unwind on shutdown.
- **`DocumentPipeline` role-interface split** into `documentReader`,
  `documentWriter`, `documentTrash`. Future sub-handlers can take the
  narrower role they need.

### Breaking

- **`/api/search/unified` REST shape aligned with MCP** (pre-v1 breaking).
  Drops `offset` (returns 400 when sent), flattens the `data`/`meta` wrapper
  to top-level `query` / `results` / `returned` / `totals` /
  `sources_searched` / `processing_time_ms` / `hint`.
- **No grandfathering for sessions without `login_at`.** Users re-auth once
  through OIDC after deploy.

### Fixed

- **`/oauth/revoke` 500 → 401** on bad client credentials. New
  `oauth.ErrInvalidClientCredentials` sentinel maps to `401 invalid_client`
  with `WWW-Authenticate: Basic realm="oauth"` per RFC 6749 §5.2.
- **HMAC-to-SHA256 silent fallback removed.** `oauth.NewService` returns an
  error when `hmacKeys` is empty; `ServerApp` propagates so `serve` refuses
  to boot without a derivable HMAC key.
- **`RevokeScopeGrant` client_id check.** `DeleteScopeGrant(id, clientID)`
  scopes the DELETE to both columns; returns 404 when no grant with that ID
  belongs to the client in the URL.
- **Autocomplete + popular-queries scoping.** `SuggestTitles(userID, isAdmin)`
  applies the same visibility predicate as full-text search.
  `/api/search/popular` is now admin-only — global aggregation has no
  per-caller scoping.
- **`ReplaceTags` 50 RTT → 1 RTT** via single multi-row VALUES.
- **List endpoints two-RTT → one-RTT** via `COUNT(*) OVER () AS total`
  pattern across `documents`, `oauth_clients`, `users` (zim_archives in
  v0.24.1).
- **Debounced `TouchClientLastUsed`** with 30 s in-memory TTL.
- **DashboardView raw fetch** replaced with `apiFetch` (now triggers the
  401 redirect interceptor on session expiry).

### Notes

- L2 reverted: `localhost` restored as a valid loopback host. The stricter
  RFC 8252 §7.3 numeric-only reading broke every MCP dev client that
  registers `http://localhost:PORT/callback`.

## [0.23.2] — 2026-04-19

### Added

- **Decompression-bomb runtime budget** across PDF / DOCX / XLSX / EPUB
  extractors.
- **Sentry event scrubbing** before send.
- **Extractor metadata sanitization** before JSONB persistence.

### Changed

- **`HKDF_SALT` required in every environment** (validation rejects empty
  and values under 16 characters).
- **S3 upload migrated to `feature/s3/transfermanager`** (replaces deprecated
  `s3manager`).

### Security

- Renovate-driven Go / Docker / CI action bumps.

## [0.23.1] — 2026-04-18

Idempotent re-tag — same commit as v0.23.0. The auto-release workflow
re-emitted the tag; no new commits.

## [0.23.0] — 2026-04-18

### Added

- **`withLoading` composable** wraps the standard Pinia async-action pattern
  (`loading=true / error=null / try / catch / set-error / finally /
  loading=false`). Required `fallbackMessage` forces per-action wording.
  Adopted across 6 stores (~168 lines removed).
- **`aria-current="true"`** on `TreeNode` selected file.

### Changed

- **3 component tests rewritten behavior-first** (`ConfirmDialog`,
  `DataTable`, `TreeNode`) — Tailwind-class assertions removed.

## [0.22.0] — 2026-04-18

### Added

- **JSONB metadata storage** (migrations 000015–000019) — `metadata`,
  `tags`, `manifest` columns converted from TEXT to JSONB.
- **Search vector JSONB-path extraction** — `documents.search_vector` extends
  to include `metadata` JSONB-path tokens via STORED expression with
  `CASE WHEN jsonb_typeof(col) = 'array' THEN col::text ELSE ''`.
- **Extractor metadata persistence** — `DocumentPipeline` marshals
  `result.Metadata` to JSONB on extract.

### Removed

- **EPUB content-header baking** — Dublin Core metadata now flows through the
  JSONB path instead of being concatenated into extracted content.

## [0.21.1] — 2026-04-17

### Added

- **`search_documents` snippets** flag-gated via `WithSnippets`. `ts_headline`
  on full content is expensive (worst case 1 GB at LIMIT 100 over 10 MB
  docs); off by default. Markdown `**` highlight markers (MCP clients are
  LLMs that render JSON as-is).

### Fixed

- **OAuth scope grants identity** — `LEFT JOIN users` so a grant whose
  granter's user row was deleted is still listed (and revocable).

## [0.21.0] — 2026-04-16

### Changed

- **Stateless MCP handler.** `StreamableHTTPOptions.Stateless: true`. Each
  POST creates a temporary session that closes after the response; any
  replica serves any request without sticky affinity. `GET /documcp` returns
  405. Traefik `sticky.cookie.*` labels removed.
- **`unified_search` is discovery-only.** Single merged page; pagination
  unsupported. Response includes per-source `totals` and a `hint` redirecting
  to source-specific paginated tools.
- **MCP `variables` shape** changed from JSON-encoded string to typed
  `map[string]string` with `additionalProperties: { type: string,
  maxLength: 10240 }`. LLMs emit malformed JSON in string params often
  enough to make the typed shape a real bug fix.

### Added

- **Grafana alert rules as code** — `NoRiverLeader` (5 min on
  `documcp_river_leader_active == 0`) and `ReadinessFailing` (2 min on
  `documcp_ready == 0`).
- **`/health/ready` real-query check** via uninstrumented `BarePgxPool` ping.
  Self-collecting `documcp_ready` gauge so Prometheus sees the same signal
  even without probe traffic.
- **Operations runbook** (`docs/OPERATIONS.md`) — pg_dump / pg_restore,
  FSBlob rsync, S3Blob `aws s3 sync` / `rclone`.

### Removed

- **Sticky-session config** in Traefik (now superseded by stateless MCP).

## [0.20.1] — 2026-04-16

Cross-cutting fix bundle across security, api-design, database,
code-quality, and frontend. Sets up the v0.21.0 release.

## [0.20.0] — 2026-04-15

### Added

- **RFC 8707 audience binding (strict).** Bearer tokens carry a `resource`
  column bound at `/oauth/authorize`. `/documcp` and `/api` middleware
  reject tokens with NULL or mismatched audience.

### Breaking

- **No grandfathering.** Legacy tokens (NULL `resource`) require re-auth.
  No observe mode.

### Changed

- **README configuration audit** — env vars cross-checked against
  `.env.example` and `docs/CONFIGURATION.md`; doc tables match deployed
  binary.

## [0.19.0] — 2026-04-13

### Added

- **MCP server icons** in the `initialize` response (favicon + admin-panel
  branding).

### Changed

- Renovate-driven dependency bumps.
- Lint fixes — golangci-lint config refinement.

## [0.18.2] — 2026-04-13

### Added

- **AWS SDK S3 EventStream DoS mitigation** — `aws-sdk-go-v2/service/s3`
  bumped for advisory.

## [0.18.1] — 2026-04-13

Code-quality refactor + blog updates.

## [0.18.0] — 2026-04-12

### Added

- **EPUB extractor** — stdlib-only (`archive/zip` + `encoding/xml` +
  `bluemonday` + `htmltomarkdown`). Parses `META-INF/container.xml` → OPF
  rootfile → spine-ordered XHTML chapters. Dublin Core metadata persists to
  `documents.metadata` JSONB.

### Fixed

- **Session cookie SameSite + Secure flags** in production.

## [0.17.0] — 2026-04-12

### Added

- **Non-admin RBAC.** Regular users upload, edit, delete their own documents
  (ownership enforced server-side via `checkOwnership` returning 404 — not
  403 — to prevent information disclosure). Admin surface (users, OAuth
  clients, external services, queue) hidden via `v-if="auth.isAdmin"` +
  `requiresAdmin` route meta.
- **User-scoped SSE stream** — non-admin events filter on
  `event.UserID == user.ID`. Events with `UserID=0` (scheduler jobs, legacy
  in-flight) are admin-only.
- **RP-Initiated Logout** with discovery + `id_token_hint`.

## [0.16.0] — 2026-04-12

### Added

- **Horizontal scale-out infrastructure** — Redis EventBus with synchronous
  `SUBSCRIBE` ACK; cross-instance SSE fan-out; distributed rate limiting via
  `httprate-redis`.

## [0.15.0] — 2026-04-11

### Security

- Security follow-up on the v0.14.x scope grants redesign.
- Devcontainer fix.

## [0.14.x] — 2026-04-08 to 2026-04-10

### Added

- **OAuth scope grants** — time-bounded per `(client_id, granted_by)`,
  TTL via `OAUTH_SCOPE_GRANT_TTL`. Replaces permanent `ExpandClientScope`
  widening. Grant fires at POST approve, never at GET render.
- **Third-party-grantable ceiling** = registered scopes ∖ {`admin`,
  `services:write`}.

### Security

- `unhead` bumped to 2.1.13.

## [0.13.x] — 2026-04-04 to 2026-04-06

### Added

- **Native TLS termination** (`TLS_ENABLED=true`) with self-signed ECDSA
  P-256 fallback for loopback. TLS 1.2 minimum, X25519 + P-256, AEAD-only.
- **Typed status constants** — `DocumentStatus`, `GitTemplateStatus`,
  `ExternalServiceStatus`, `DeviceCodeStatus` in `internal/model/`. CHECK
  constraints (migration 000014) pin the DB.

### Fixed

- **PDF text fragmentation** — `cleanText` post-processor joins continuation
  lines that the library splits at text-object boundaries.
- **OIDC provider discovery retry** with exponential backoff.

### Security

- Vite 8.0.3 → 8.0.5 (advisory).

## [0.12.x] — 2026-04-03 to 2026-04-04

### Added

- **MCP server icons** preview.
- **Document management UI polish** — bulk operations, filter persistence,
  test coverage expansion.
- **Secret scanning + frontend dependency audit** in CI.

### Fixed

- **Trace noise from readiness probes** — `BarePgxPool` and
  `BareRedisClient` carry no tracer attached, so `/health/ready` emits no
  spans.
- **Safari OAuth popup compat** — JS redirect; CSP `form-action` override.
- **Favicon at root** for Claude.ai connector discovery.

## [0.11.0] — 2026-04-03

### Added

- **OAuth + a11y + version consistency hardening.**
- **ZIM fan-out caps** — `selectArchives()` capped at `federatedMaxArchives`
  (default 10). Semaphore capacity 10. `MaxIdleConnsPerHost=10`. 5 s fan-out
  timeout.

## [0.10.x] — 2026-04-03

### Added

- **`list_documents` MCP tool.**
- **External-connection tracing** (Kiwix, Git, OIDC).
- **Trace correlation** via `slog` + OTEL.

### Fixed

- **Path traversal guard** — `security.SafeStoragePath` before any `os.Open`
  / `os.Remove` on DB-sourced paths.
- **Worker tracing** — River v0.32 doesn't propagate OTEL context; workers
  open root spans via `workerTracer.Start`.
- **Redis readiness** — dedicated rate-limit client with no retries; main
  client used for EventBus and app queries.
- **OAuth grant + path traversal hardening.**
- **Safari 303 redirect** for OAuth POST (prevents POST replay).
- **Multi-arch Docker via cross-compilation** (replaces emulated builds).
- **Docker Hub cleanup by digest, not tag** (handles re-pushed tags).

## [0.0.1 – 0.9.x] — 2026-03-30 to 2026-04-02 (bootstrap)

Project bootstrap phase — rapid iteration as scaffolding came together. Key
landings:

- **MCP protocol server** (Go SDK, 16 tools + 6 prompts).
- **OAuth 2.1 authorization server** — auth code + PKCE, refresh,
  revocation, device flow (RFC 8628), dynamic registration (RFC 7591),
  RFC 9728 PRM.
- **OIDC authentication** — sub-only identity, admin groups +
  bootstrap-email fallback.
- **Document pipeline** — PDF / DOCX / XLSX / HTML / Markdown extractors
  (EPUB landed in v0.18.0). Dual-path storage (upload vs inline create).
- **Search** — PostgreSQL FTS on STORED tsvector columns. Federated across
  documents / ZIM / git templates with per-source totals.
- **External services** — Kiwix (XML fulltext + JSON suggest + DOM walker
  HTML→text), git templates (go-git pure-Go), Confluence (later removed in
  v0.25.x line — see project brief).
- **Vue 3 admin SPA** — Pinia 3, Vite 8, Tailwind v4, behavior-first tests.
- **River v0.32 queue** on shared pgxpool — 7 workers / 3 queues / 6
  periodic jobs. Redis EventBus.
- **OTEL tracing + Prometheus metrics + slog** — `AlwaysSample`, trimmed
  span names via `otelpgx`, trace-free readiness via bare pool.
- **AES-256-GCM at rest**, comprehensive SSRF allowlist, path traversal via
  `os.Root`, bcrypt 72-byte guard, rate-limit-before-auth on unauth paths.
- **Forgejo (primary) + GitHub Actions (mirror) CI**, SHA-pinned, multi-arch
  Docker, Sigstore keyless signing on GitHub release, Renovate on `dev`.
- **Distroless ~45 MB single Cobra binary** (`serve`, `worker`, `migrate`,
  `version`, `health` subcommands).
- **Native TLS** with self-signed fallback; FSBlob + S3Blob (direct on
  `aws-sdk-go-v2`).
- **Several CVE remediations + Dependabot / CodeQL findings closures.**

Per-tag detail for this range lives in git tags (`git log v0.0.1..v0.9.7
--first-parent --pretty='%h %ai %s'`).

[Unreleased]: https://github.com/c-premus/DocuMCP-go/compare/v0.27.0...HEAD
[0.27.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.27.0
[0.26.1]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.26.1
[0.26.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.26.0
[0.25.2]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.25.2
[0.25.1]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.25.1
[0.25.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.25.0
[0.24.1]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.24.1
[0.24.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.24.0
[0.23.2]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.23.2
[0.23.1]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.23.1
[0.23.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.23.0
[0.22.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.22.0
[0.21.1]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.21.1
[0.21.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.21.0
[0.20.1]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.20.1
[0.20.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.20.0
[0.19.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.19.0
[0.18.2]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.18.2
[0.18.1]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.18.1
[0.18.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.18.0
[0.17.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.17.0
[0.16.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.16.0
[0.15.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.15.0
[0.14.x]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.14.2
[0.13.x]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.13.3
[0.12.x]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.12.2
[0.11.0]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.11.0
[0.10.x]: https://github.com/c-premus/DocuMCP-go/releases/tag/v0.10.4
