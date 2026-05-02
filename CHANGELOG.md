# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) with the
**pre-v1.0 convention that breaking changes bump minor, not major** — a
`feat!:` or `BREAKING CHANGE` commit while the major version is `0` produces a
minor bump; `v1.0.0` is reserved for an explicit manual cut.

This file lists end-user-facing changes only — `feat`, `fix`, `chore`, and
`BREAKING`. CI, test, refactor, and docs commits are visible in the git log
but intentionally omitted here to keep the changelog signal-dense.

## [0.28.0] - 2026-05-02

### Features

- feat(observability): expose /metrics + /health/ready aliases on worker mode

### Fixes

- fix(migrations): avoid +goose token in narrative comments
- fix(observability): drop user email from Sentry events

### Maintenance

- chore(migrations): add safety convention + CI lint

## [0.27.6] - 2026-05-02

### Fixes

- fix(security): override uuid to ^14.0.0

### Maintenance

- chore(changelog): update for v0.27.6

## [0.27.5] - 2026-05-01

### Fixes

- fix(ci): harden version-release for partial-success and tag-indexer races

### Maintenance

- chore(changelog): update for v0.27.5

## [0.27.4] - 2026-05-01

### Fixes

- fix(auth): re-discover OIDC in background on boot failure

### Maintenance

- chore(changelog): update for v0.27.4

## [0.27.3] - 2026-04-30

### Fixes

- fix(ci): materialize refs/heads/main before bare clone on tag dispatch
- fix(ci): explicitly dispatch downstream workflows after tag push

### Maintenance

- chore(changelog): update for v0.27.3

## [0.27.2] - 2026-04-30

### Fixes

- fix(ci): use github.repository_owner for registry login username
- fix(ci): use FORGE_TOKEN for container registry login
- fix(ci): use FORGE_TOKEN PAT for version-release pushes

### Maintenance

- chore(changelog): update for v0.27.2

## [0.27.1] - 2026-04-30

### Fixes

- fix(ci): use git clone --bare; the simpler form of the bypass
- fix(ci): bypass git clone entirely; use git init --bare + fetch
- fix(ci): detach workspace HEAD before mirror clone
- fix(ci): clean worktree registry at --git-common-dir, not workspace .git
- fix(deps): bump marked to 18.0.2 (GHSA-6v9c-7cg6-27q7)
- fix(ci): nuke .git/worktrees before mirror clone
- fix(observability): always re-root inbound HTTP traces

### Maintenance

- chore(changelog): update for v0.27.1
- chore(ci): Rename FORGEJO_TOKEN secret reference to FORGE_TOKEN
- chore(ci): switch GitHub mirror to filter-repo + auto-CHANGELOG

## [0.27.0] - 2026-04-29

### Features

- feat(frontend): mermaid diagram rendering in ContentViewer
- feat(frontend): mobile + markdown for git template file viewer
- feat(frontend): mobile card for QueueView with full error visibility
- feat(frontend): mobile cards for ZimArchives + GitTemplates views
- feat(frontend): mobile cards for OAuthClients + ExternalServices views
- feat(frontend): mobile cards for Users + DocumentTrash views
- feat(frontend): mobile card view for DataTable + Documents pilot

## [0.26.1] - 2026-04-28

### Maintenance

- chore(deps): update dependency postcss to v8.5.10 [security]
- chore(deps): update catthehacker/ubuntu:act-22.04 docker digest to c8b6f14

## [0.26.0] - 2026-04-25

### Features

- feat(auth): revoke admin sessions on demote/delete + admin UI

### Fixes

- fix(grafana): put panels in the correct section rows

## [0.25.2] - 2026-04-24

### Fixes

- fix(observability): discard unsampled upstream traceparent

## [0.25.1] - 2026-04-23

### Fixes

- fix(deps): bump golang.org/x/image to v0.39.0 (CVE-2026-33813)

## [0.25.0] - 2026-04-22

### Features

- feat(session): back sessions with redis for server-side revocation
- feat(crypto): rotate ENCRYPTION_KEY without a flag day
- feat(sse): revalidate user and token on each heartbeat

### Fixes

- fix(kiwix): read zim file_size from OPDS acquisition link length
- fix(security): back device-flow counter with redis
- fix(grafana): show real user traffic in traces + log rows

## [0.24.0] - 2026-04-22

### BREAKING CHANGES

- feat(api)!: align /api/search/unified with MCP discovery-only model

### Features

- feat(api)!: align /api/search/unified with MCP discovery-only model
- feat(api): wire tags + include_snippets query params on REST search

### Fixes

- fix(oauth): map /oauth/revoke errors per RFC 7009 §2.2
- fix(oauth): versioned HMAC token hashes with rotation support
- fix(security): anchor session lifetime and revoke in-session tokens
- fix(oauth): restore localhost as valid loopback host
- fix(search): close autocomplete + popular-queries cross-tenant leaks
- fix(frontend): close 2026-04-21 audit Tier 1 frontend items
- fix(git): install SSRF-safe HTTP transport into go-git
- fix: close 2026-04-21 audit code-quality + docs items
- fix(oauth): close 2026-04-21 audit Tier 1 OAuth security items

### Maintenance

- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to 2ccc5b1
- chore(deps): update dependency typescript to v6.0.3

## [0.23.2] - 2026-04-19

### Fixes

- fix(storage): migrate S3 upload to feature/s3/transfermanager
- fix(deps): update go dependencies
- fix(service): sanitize extractor metadata before JSONB persistence
- fix(config): require HKDF_SALT in every environment
- fix(observability): scrub sensitive data in Sentry events
- fix(extractor): enforce decompression budget at runtime

### Maintenance

- chore(deps): update sigstore/cosign-installer action to v3.10.1
- chore(deps): update actions/cache action to v5.0.5
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to e4abd88
- chore(deps): update docker dependencies
- chore(deps): update catthehacker/ubuntu:act-22.04 docker digest to 450293e
- chore(deps): pin dependencies

## [0.23.0] - 2026-04-18

### Features

- feat(frontend): withLoading composable + behavior-first test polish

## [0.22.0] - 2026-04-18

### Features

- feat(db): index metadata JSONB in search_vector
- feat(db): JSONB storage + extractor metadata persistence

### Fixes

- fix(deps): bump go-git to v5.18.0 (GHSA-3xc5-wrhm-f963)

## [0.21.1] - 2026-04-17

### Fixes

- fix(frontend): replace Scalar viewer with /openapi.yaml link
- fix(oauth): show granter identity on admin scope-grants listing
- fix(mcp): wire include_snippets on search_documents

## [0.21.0] - 2026-04-16

### BREAKING CHANGES

- fix(mcp)!: drop offset from unified_search; surface per-source totals
- fix(mcp)!: drop offset from unified_search; surface per-source totals
- ci(release): pre-v1 breaking downgrade + better release notes

### Features

- feat(ops): close architecture audit — backup docs, leader gauge, readiness probe, alerts

### Fixes

- fix(deps): bump dompurify 3.3.3 → 3.4.0
- fix(mcp)!: drop offset from unified_search; surface per-source totals

### Maintenance

- chore(supply-chain): close audit findings

## [0.20.1] - 2026-04-16

### Fixes

- fix(frontend): close frontend.md audit findings
- fix(search): restore trigram fallback operator
- fix(db): tighten schema constraints and cap unbounded lists
- fix(db): denormalize documents search_vector to STORED column
- fix(api): close findings from api-design audit
- fix(security): close HIGH + MEDIUM findings from v0.21.0 audit

## [0.20.0] - 2026-04-15

### Features

- feat(auth): bind OAuth tokens to RFC 8707 resource indicator

## [0.19.0] - 2026-04-13

### Features

- feat(mcp): add websiteUrl and data URI favicon to server icons

### Fixes

- fix(deps): update go dependencies

### Maintenance

- chore(deps): bump frontend deps + sentry-go, fix lint
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to b70f50f

## [0.18.2] - 2026-04-13

### Maintenance

- chore(deps): bump aws-sdk-go-v2/service/s3 v1.92.1 → v1.99.0

## [0.18.0] - 2026-04-12

### Features

- feat(epub): add EPUB document extraction support

### Fixes

- fix(auth): move id_token to separate cookie

## [0.17.0] - 2026-04-12

### Features

- feat(rbac): add non-admin user support with ownership-based document access
- feat(admin): add OAuth client detail view with scope grants and River UI integration

### Fixes

- fix(security): address 7 findings from round-6 audit

## [0.16.0] - 2026-04-12

### Features

- feat(scale): horizontal scale-out via Blob storage, sticky MCP sessions, and cross-replica Kiwix invalidation

## [0.15.0] - 2026-04-11

### Features

- feat(mcp): annotate write tools and test scope enforcement

### Fixes

- fix(devcontainer): decouple Go toolchain from base image
- fix(service): enforce document tag bounds at service layer
- fix(auth): validate OIDC provider URL and apply SSRF-safe transport

## [0.14.2] - 2026-04-10

### Fixes

- fix(deps): bump unhead to 2.1.13 via @scalar/api-reference

## [0.14.1] - 2026-04-10

### Fixes

- fix(deps): update dependency marked to v18
- fix(deps): update go dependencies

### Maintenance

- chore(deps): update https://github.com/docker/build-push-action action to v7.1.0
- chore(deps): update frontend dependencies
- chore(deps): update docker/build-push-action action to v7.1.0
- chore(deps): update dependency @types/node to v24.12.2
- chore(deps): update traefik:v3.6 docker digest to 5ae9c34
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to f3ba591
- chore(deps): update catthehacker/ubuntu:act-22.04 docker digest to 754f20d

## [0.14.0] - 2026-04-08

### Features

- feat(oauth): replace permanent scope widening with time-bounded grants

### Fixes

- fix(security): harden containers, clean partial clones, prune stale lint rule
- fix(security): close audit round 5 findings
- fix(security): close audit round 4 findings
- fix(security): close audit round 3 findings
- fix(security): close audit round 2 and tune Kiwix fan-out
- fix(security): harden PDF extractor and close audit findings

## [0.13.3] - 2026-04-06

### Fixes

- fix(deps): update vite 8.0.3 → 8.0.5 (security)

## [0.13.2] - 2026-04-06

### Fixes

- fix(pdf): rejoin fragmented text from PDF text objects
- fix(deps): update go dependencies

### Maintenance

- chore(deps): lock file maintenance
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to 5e66d07
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to 8248aad

## [0.13.0] - 2026-04-04

### Features

- feat(server): add native TLS termination with self-signed fallback

## [0.12.2] - 2026-04-04

### Fixes

- fix(auth): retry OIDC provider discovery with exponential backoff

## [0.12.1] - 2026-04-03

### Fixes

- fix(observability): eliminate trace noise from readiness probes

## [0.12.0] - 2026-04-03

### Features

- feat(mcp): add server icons and title to initialize response

## [0.11.0] - 2026-04-03

### Features

- feat(oidc): add otelhttp tracing to OIDC HTTP client
- feat(ui): modern favicon set with PWA manifest and OAuth dark mode

### Fixes

- fix: webmanifest Content-Type and CountClients test
- fix(ui): set webmanifest Content-Type for CI environments
- fix(version): consistent version across platform
- fix(search): cap ZIM fan-out and add concurrency semaphore
- fix(oauth): deduplicate scope strings at parse boundary
- fix(a11y): remaining WCAG 2.1 AA polish pass
- fix(oauth): handle 204 in apiFetch and return 404 on missing client
- fix(a11y): WCAG 2.1 AA compliance across OAuth pages and admin SPA
- fix(ui): serve favicons and manifest from root paths
- fix(oauth): match dark mode background to SPA theme
- fix(oauth): restore client scope expansion on admin approval
- fix(oauth): enforce scope intersection across all grant flows

## [0.10.4] - 2026-04-03

### Fixes

- fix(oauth): use JS redirect for Safari popup compat

## [0.10.3] - 2026-04-03

### Fixes

- fix(oauth): override CSP form-action on redirects

## [0.10.1] - 2026-04-03

### Fixes

- fix(server): serve favicon.ico at root for Claude.ai

## [0.10.0] - 2026-04-03

### Features

- feat(documents): add edit modal, visibility badges, content re-upload, and tag autocomplete

### Fixes

- fix(lint): resolve gosec and sloglint issues
- fix(ci): add resilience to Docker Hub cleanup step

## [0.9.7] - 2026-04-02

### Fixes

- fix(ci): delete Docker Hub images by digest, not just tags

### Maintenance

- chore(deps): lock file maintenance
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to 4cd17e1
- chore(deps): lock file maintenance
- chore(deps): update dependencies and CI actions

## [0.9.6] - 2026-04-02

### Fixes

- fix(ci): add network=host to buildx driver for registry access
- fix(docker): use native builds with cross-compilation for multi-arch
- fix(ci): sort Docker Hub tags client-side before cleanup

## [0.9.5] - 2026-04-02

### Fixes

- fix(observability): fix empty SQL panels and add db_system filtering

### Maintenance

- chore(frontend): bump CI Node.js 22→24 and update dev deps

## [0.9.4] - 2026-04-02

### Fixes

- fix(security): harden OAuth grant, path traversal, JSON response, and CI pinning

## [0.9.3] - 2026-04-02

### Fixes

- fix(redis): revert unnecessary go-redis downgrade

## [0.9.2] - 2026-04-02

### Fixes

- fix(redis): downgrade go-redis to v9.17.3 to restore connection pooling

### Maintenance

- chore(devcontainer): add gh, redis-cli, and psql

## [0.9.1] - 2026-04-02

### Fixes

- fix(redis): disable CLIENT SETINFO to eliminate unread data warnings

## [0.9.0] - 2026-04-02

### Features

- feat(mcp): add list_documents tool for document discovery
- feat(observability): add root spans to River workers, fix trace explorer
- feat(observability): trace external connections, fix Redis state management
- feat(ci): add Grafana dashboard validation and deploy workflow
- feat(observability): add Sentry/GlitchTip error tracking
- feat(hardening): security and code quality improvements from multi-agent review
- feat(redis): add Redis for distributed rate limiting and cross-instance SSE
- feat(security): add client_ip to request logs, auth failure logging
- feat(frontend): add interactive API docs page via Scalar
- feat(server): redirect root URL to admin panel
- feat(frontend): refine light mode palette with slate tones and indigo accents
- feat(security): encrypt external service API keys at rest
- feat(search): include DevDocs archives in unified search fan-out
- feat(zim): add search fallback and consolidate migrations
- feat(search): add file-level FTS for git templates
- feat: pre-release backlog cleanup
- feat(ci): add self-hosted Renovate workflow for Forgejo
- feat(search): use Meilisearch to select relevant archives for Kiwix fan-out
- feat(ci): add version-release workflow for automated releases
- feat(search): add two-way DB↔Meilisearch index reconciliation
- feat(mcp): add federated ZIM article search to unified_search
- feat(ui): mobile nav drawer, user dropdown, and responsive tables
- feat(oauth): track last_used_at on bearer token validation
- feat(frontend): add sync button for external services, rebuild dist
- feat(frontend): show document content inline, fix download button
- feat(frontend): dark mode + WCAG 2.1 AA accessibility
- feat(auth): OIDC admin groups, dual auth middleware, and SPA asset fix
- feat(auth): fine-grained access control with scopes and document ownership
- feat(auth): wire RequireScope middleware and implement OAuthClient.Show
- feat: test coverage, response standardization, and docs cleanup
- feat(phase11): Add security tests, remove old admin UI, mount SPA at /admin
- feat(frontend): Add remaining admin pages and complete Vue SPA
- feat(frontend): Add core admin pages and shared components
- feat(api): Add dashboard stats, user CRUD, document restore/purge
- feat(frontend): Add Vue 3 + TypeScript SPA scaffold
- feat(queue): Migrate to River Postgres-native job queue
- feat(scheduler,api): Add 6 maintenance jobs and 4 REST API endpoints
- feat(skills): Migrate Forgejo skills to fj CLI
- feat(security): Add trusted proxy RealIP middleware with CIDR validation
- feat(grafana): Add Grafana Foundation SDK dashboard generator for Go version
- feat(scheduler): Add cron scheduler for external service sync jobs
- feat(test): Add integration tests with testcontainers-go
- feat(security): Add CSRF, rate limiting, security headers, and hardening
- feat(observability): Add tracing, metrics, tests, CI/CD, deployment
- feat(admin): Implement admin web UI with templ + htmx
- feat(wire): Wire MCP tool handlers, DI, and routes
- feat(api): Implement REST API handlers for all external services
- feat(services): Add external service management, repository extensions, indexer methods
- feat(clients): Implement external service clients for ZIM, Confluence, Git
- feat(pipeline): Implement Phase 3 document processing pipeline and search
- feat(oauth): Implement Phase 2B OAuth 2.1 server and OIDC authentication
- feat: Implement Phase 2A MCP server with 18 tools and 7 prompts
- feat: Implement Phase 1 foundation (config, database, models, HTTP server, DI)
- feat: Initialize repo with claude-template and Go devcontainer

### Fixes

- fix(redis): dedicated rate limit client to prevent unread data warnings
- fix(oauth): use 303 See Other for Safari POST-redirect
- fix(redis): add ContextTimeoutEnabled, explicit timeouts and retries
- fix(security): add path traversal guard to document pipeline
- fix(redis): decouple readiness pings from HTTP request context
- fix(grafana): align dashboard with new tracing instrumentation
- fix(observability): document count gauge, Redis RESP2 switch
- fix(observability): trace correlation + Redis pool churn
- fix(observability): accept URL format for OTEL endpoint
- fix(grafana): correct service name filters and add observability docs
- fix(grafana): correct document count metric name in dashboard
- fix(observability): derive OTEL service version from build ldflags
- fix(ci): use FORGEJO_TOKEN secret for release creation
- fix(observability): use explicit sampler to fix missing traces behind reverse proxy
- fix(frontend): exclude generated API code from coverage and prettier
- fix(deps): upgrade typescript-eslint to 8.58.0 for TypeScript 6 support
- fix(deps): update go dependencies
- fix(redis): force RESP2 protocol to prevent connection pool churn
- fix(ci): remove Trivy scan from Forgejo release workflow
- fix(ci): use correct image tag for Trivy scan in GitHub release
- fix(ci): install Trivy binary on Forgejo runner
- fix(auth): resolve data race on token HMAC key
- fix(security): use session state for OAuth redirect URI
- fix(security): resolve 6 Dependabot alerts + 2 CodeQL findings
- fix(git): replace exec-based git with go-git, fix health check SSRF
- fix(deps): upgrade x/image to v0.38.0 (CVE-2026-33809)
- fix(docker): set WORKDIR / in distroless stage
- fix(ci): use internal registry hostname for Docker login
- fix(api): return 422 for URL validation errors, not 500
- fix(security): skip unspecified IPs in SSRF validation
- fix(server): close MCP sessions on shutdown
- fix(crypto): hex-decode ENCRYPTION_KEY for full AES-256 entropy
- fix(server): check XFF before X-Real-IP in RealIP
- fix(config): use comma splitting for env var slices
- fix(docker): bump runtime Alpine 3.21 → 3.22 for CVE remediation
- fix(ci): resolve Forgejo release workflow and Docker build failures
- fix(zim): overhaul Kiwix search with XML parsing and error resilience
- fix: resolve failing tests and lint warnings
- fix: anchor build artifact patterns in .gitignore
- fix(frontend): wire SSE events to stores for live reactivity
- fix(security): harden auth, ILIKE escaping, and SSRF policy
- fix(zim): use suggest endpoint for non-FT archives and clean HTML whitespace
- fix(migration): add goose StatementBegin/End for PL/pgSQL function
- fix(oauth): expand client scope on authenticated approval
- fix(ci): use github.token for Forgejo registry auth
- fix(security): add path traversal guard to purge handlers
- fix(security): comprehensive security remediation (14 work packages)
- fix(ci): replace bc with shell arithmetic, upgrade tsconfig to ES2023
- fix(mcp): correct stale parameter names and tool refs in 5 prompts
- fix(ci): replace rsync with find+cp in GitHub sync
- fix(sse): wire River job events to EventBus for real-time UI updates
- fix(mcp): add Kiwix hot-reload, full index verification, sources fix
- fix(security): resolve 11 findings from code quality audit
- fix(frontend): run openapi-ts before build to generate API client
- fix(security): resolve 5 low findings from security assessment
- fix(security): resolve 7 medium findings from security assessment
- fix(security): block unauthenticated MCP, enforce document ownership
- fix(oauth): add missing last_used_at column and fix null guard
- fix(api): resolve git template file 404 from encoded path slashes
- fix(security): replace gorilla/csrf with net/http.CrossOriginProtection
- fix(security): MaxBytesReader on all form endpoints, lint v2.11.3 fixes
- fix(shutdown): two-stage shutdown to handle persistent MCP SSE sessions
- fix(api): add real pagination to ZIM archives and git templates
- fix(ui): layout, pagination, cursors, and a11y polish
- fix(sse): singleton store, initial flush, and external theme script
- fix(ui): dashboard grid, SSE indicator, favicons, and CSP hash
- fix(sse): fix 504 timeout, write deadline, and slow shutdown
- fix(kiwix): resolve versioned content IDs and fix search parameters
- fix(queue): wire River sync jobs, fix UniqueOpts, track health status
- fix: resolve nil-interface crash, index cleanup, SSRF, and doc content
- fix(mcp): resolve tool discovery by replacing any with concrete types
- fix(server): OAuth CSRF, scope validation, and MCP timeout
- fix: pre-deploy hardening across security, compliance, and UX
- fix: comprehensive remediation across security, data integrity, and performance
- fix(security): comprehensive security audit fixes
- fix(security): production config, logger injection, ACL, CSP, DNS rebinding
- fix(security): SSRF, XSS, open redirect, header injection
- fix(security): Add admin auth guard, sanitize error messages, move SSE under admin
- fix(ci): Mount DinD socket instead of nested DinD service container
- fix(ci): Fix integration test and security scan failures
- fix(ci): Add DinD for integration tests, fix lint, drop GitHub CI
- fix(security): Remediate critical and high audit findings
- fix(ci): use correct action refs for Forgejo runner
- fix(devcontainer): workspace mount

### Maintenance

- chore(devcontainer): update PostgreSQL volume mount for PG 18
- chore(deps): remove deprecated @types/dompurify
- chore(deps): update dependency typescript to v6
- chore(deps): update docker dependencies
- chore(deps): update docker/setup-qemu-action action to v4
- chore(deps): update github/codeql-action action to v4
- chore(deps): update https://github.com/docker/setup-qemu-action action to v4
- chore(deps): update node.js to v24
- chore(deps): lock file maintenance
- chore(deps): update dependency typescript to v6
- chore(deps): update https://github.com/docker/setup-qemu-action action to v3.7.0
- chore(deps): update https://github.com/actions/setup-go action to v6.4.0
- chore(deps): update docker/setup-qemu-action action to v3.7.0
- chore(deps): update actions/setup-go action to v6.4.0
- chore(deps): update dependency @types/node to v22.19.15
- chore(deps): update github/codeql-action digest to 5c8a8a6
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to 7eec08c
- chore(deps): pin dependencies
- chore(deps): pin dependencies
- chore(deps): pin catthehacker/ubuntu docker tag to 5258195
- chore(frontend): bump major dependencies
- chore: sync skills from claude-template upstream
- chore: consolidate node_modules gitignore pattern
- chore: remove stale Meilisearch and Confluence references
- chore(ci): align workflows with mcp-gate patterns
- chore: add GitHub mirror, CI/CD, and rename Go module
- chore: add MIT license
- chore(frontend): add ESLint, Prettier, and coverage thresholds
- chore(devcontainer): auto-load .env via docker-compose env_file
- chore: upgrade Go 1.25.6 → 1.26.1, pin versions in CI
- chore(lint): upgrade to 3-tier golangci-lint config and fix 619 issues
- chore(lint): resolve all 368 golangci-lint issues to zero
- chore(lint): add golangci-lint v2 config with Google Go style rules
- chore: remove dead code (RollbackMigration, AccessTokenFromContext, RedisConfig)
- chore(docs): Remove PHP/Laravel-specific documentation
- chore: Update memory bank and enable gopls plugin

## [0.8.1] - 2026-04-02

### Fixes

- fix(redis): dedicated rate limit client to prevent unread data warnings
- fix(oauth): use 303 See Other for Safari POST-redirect

## [0.8.0] - 2026-04-01

### Features

- feat(mcp): add list_documents tool for document discovery

### Fixes

- fix(redis): add ContextTimeoutEnabled, explicit timeouts and retries

## [0.7.0] - 2026-04-01

### Features

- feat(observability): add root spans to River workers, fix trace explorer

### Fixes

- fix(security): add path traversal guard to document pipeline
- fix(redis): decouple readiness pings from HTTP request context

## [0.6.0] - 2026-04-01

### Features

- feat(observability): trace external connections, fix Redis state management

### Fixes

- fix(grafana): align dashboard with new tracing instrumentation
- fix(observability): document count gauge, Redis RESP2 switch

## [0.5.1] - 2026-04-01

### Fixes

- fix(observability): trace correlation + Redis pool churn

## [0.5.0] - 2026-04-01

### Features

- feat(ci): add Grafana dashboard validation and deploy workflow

### Fixes

- fix(observability): accept URL format for OTEL endpoint
- fix(grafana): correct service name filters and add observability docs
- fix(grafana): correct document count metric name in dashboard

## [0.4.1] - 2026-03-31

### Fixes

- fix(observability): derive OTEL service version from build ldflags
- fix(ci): use FORGEJO_TOKEN secret for release creation

## [0.4.0] - 2026-03-31

### Features

- feat(observability): add Sentry/GlitchTip error tracking

### Fixes

- fix(observability): use explicit sampler to fix missing traces behind reverse proxy

## [0.3.4] - 2026-03-31

### Fixes

- fix(frontend): exclude generated API code from coverage and prettier
- fix(deps): upgrade typescript-eslint to 8.58.0 for TypeScript 6 support
- fix(deps): update go dependencies

### Maintenance

- chore(deps): remove deprecated @types/dompurify
- chore(deps): update dependency typescript to v6
- chore(deps): update docker dependencies
- chore(deps): update docker/setup-qemu-action action to v4
- chore(deps): update github/codeql-action action to v4
- chore(deps): update https://github.com/docker/setup-qemu-action action to v4
- chore(deps): update node.js to v24
- chore(deps): lock file maintenance
- chore(deps): update dependency typescript to v6
- chore(deps): update https://github.com/docker/setup-qemu-action action to v3.7.0
- chore(deps): update https://github.com/actions/setup-go action to v6.4.0
- chore(deps): update docker/setup-qemu-action action to v3.7.0
- chore(deps): update actions/setup-go action to v6.4.0
- chore(deps): update dependency @types/node to v22.19.15
- chore(deps): update github/codeql-action digest to 5c8a8a6
- chore(deps): update ghcr.io/renovatebot/renovate:latest docker digest to 7eec08c
- chore(deps): pin dependencies
- chore(deps): pin dependencies
- chore(deps): pin catthehacker/ubuntu docker tag to 5258195

## [0.3.3] - 2026-03-31

### Fixes

- fix(redis): force RESP2 protocol to prevent connection pool churn
- fix(ci): remove Trivy scan from Forgejo release workflow

## [0.3.2] - 2026-03-31

### Fixes

- fix(ci): use correct image tag for Trivy scan in GitHub release
- fix(ci): install Trivy binary on Forgejo runner

## [0.3.1] - 2026-03-31

### Fixes

- fix(auth): resolve data race on token HMAC key

## [0.3.0] - 2026-03-31

### Features

- feat(hardening): security and code quality improvements from multi-agent review
- feat(redis): add Redis for distributed rate limiting and cross-instance SSE

## [0.2.2] - 2026-03-31

### Fixes

- fix(security): use session state for OAuth redirect URI

## [0.2.1] - 2026-03-31

### Fixes

- fix(security): resolve 6 Dependabot alerts + 2 CodeQL findings

## [0.2.0] - 2026-03-31

### Features

- feat(security): add client_ip to request logs, auth failure logging

## [0.1.6] - 2026-03-30

### Fixes

- fix(git): replace exec-based git with go-git, fix health check SSRF

## [0.1.5] - 2026-03-30

### Fixes

- fix(deps): upgrade x/image to v0.38.0 (CVE-2026-33809)

## [0.1.4] - 2026-03-30

### Fixes

- fix(docker): set WORKDIR / in distroless stage

## [0.1.3] - 2026-03-30

### Fixes

- fix(ci): use internal registry hostname for Docker login
- fix(api): return 422 for URL validation errors, not 500
- fix(security): skip unspecified IPs in SSRF validation

## [0.1.0] - 2026-03-30

### Features

- feat(frontend): add interactive API docs page via Scalar
- feat(server): redirect root URL to admin panel

### Fixes

- fix(server): close MCP sessions on shutdown
- fix(crypto): hex-decode ENCRYPTION_KEY for full AES-256 entropy

### Maintenance

- chore(frontend): bump major dependencies

## [0.0.3] - 2026-03-30

### Fixes

- fix(server): check XFF before X-Real-IP in RealIP
- fix(config): use comma splitting for env var slices
- fix(docker): bump runtime Alpine 3.21 → 3.22 for CVE remediation

### Maintenance

- chore: sync skills from claude-template upstream

## [0.0.2] - 2026-03-30

### Fixes

- fix(ci): resolve Forgejo release workflow and Docker build failures

## [0.0.1] - 2026-03-30

### Features

- feat(frontend): refine light mode palette with slate tones and indigo accents
- feat(security): encrypt external service API keys at rest
- feat(search): include DevDocs archives in unified search fan-out
- feat(zim): add search fallback and consolidate migrations
- feat(search): add file-level FTS for git templates
- feat: pre-release backlog cleanup
- feat(ci): add self-hosted Renovate workflow for Forgejo
- feat(search): use Meilisearch to select relevant archives for Kiwix fan-out
- feat(ci): add version-release workflow for automated releases
- feat(search): add two-way DB↔Meilisearch index reconciliation
- feat(mcp): add federated ZIM article search to unified_search
- feat(ui): mobile nav drawer, user dropdown, and responsive tables
- feat(oauth): track last_used_at on bearer token validation
- feat(frontend): add sync button for external services, rebuild dist
- feat(frontend): show document content inline, fix download button
- feat(frontend): dark mode + WCAG 2.1 AA accessibility
- feat(auth): OIDC admin groups, dual auth middleware, and SPA asset fix
- feat(auth): fine-grained access control with scopes and document ownership
- feat(auth): wire RequireScope middleware and implement OAuthClient.Show
- feat: test coverage, response standardization, and docs cleanup
- feat(phase11): Add security tests, remove old admin UI, mount SPA at /admin
- feat(frontend): Add remaining admin pages and complete Vue SPA
- feat(frontend): Add core admin pages and shared components
- feat(api): Add dashboard stats, user CRUD, document restore/purge
- feat(frontend): Add Vue 3 + TypeScript SPA scaffold
- feat(queue): Migrate to River Postgres-native job queue
- feat(scheduler,api): Add 6 maintenance jobs and 4 REST API endpoints
- feat(skills): Migrate Forgejo skills to fj CLI
- feat(security): Add trusted proxy RealIP middleware with CIDR validation
- feat(grafana): Add Grafana Foundation SDK dashboard generator for Go version
- feat(scheduler): Add cron scheduler for external service sync jobs
- feat(test): Add integration tests with testcontainers-go
- feat(security): Add CSRF, rate limiting, security headers, and hardening
- feat(observability): Add tracing, metrics, tests, CI/CD, deployment
- feat(admin): Implement admin web UI with templ + htmx
- feat(wire): Wire MCP tool handlers, DI, and routes
- feat(api): Implement REST API handlers for all external services
- feat(services): Add external service management, repository extensions, indexer methods
- feat(clients): Implement external service clients for ZIM, Confluence, Git
- feat(pipeline): Implement Phase 3 document processing pipeline and search
- feat(oauth): Implement Phase 2B OAuth 2.1 server and OIDC authentication
- feat: Implement Phase 2A MCP server with 18 tools and 7 prompts
- feat: Implement Phase 1 foundation (config, database, models, HTTP server, DI)
- feat: Initialize repo with claude-template and Go devcontainer

### Fixes

- fix(zim): overhaul Kiwix search with XML parsing and error resilience
- fix: resolve failing tests and lint warnings
- fix: anchor build artifact patterns in .gitignore
- fix(frontend): wire SSE events to stores for live reactivity
- fix(security): harden auth, ILIKE escaping, and SSRF policy
- fix(zim): use suggest endpoint for non-FT archives and clean HTML whitespace
- fix(migration): add goose StatementBegin/End for PL/pgSQL function
- fix(oauth): expand client scope on authenticated approval
- fix(ci): use github.token for Forgejo registry auth
- fix(security): add path traversal guard to purge handlers
- fix(security): comprehensive security remediation (14 work packages)
- fix(ci): replace bc with shell arithmetic, upgrade tsconfig to ES2023
- fix(mcp): correct stale parameter names and tool refs in 5 prompts
- fix(ci): replace rsync with find+cp in GitHub sync
- fix(sse): wire River job events to EventBus for real-time UI updates
- fix(mcp): add Kiwix hot-reload, full index verification, sources fix
- fix(security): resolve 11 findings from code quality audit
- fix(frontend): run openapi-ts before build to generate API client
- fix(security): resolve 5 low findings from security assessment
- fix(security): resolve 7 medium findings from security assessment
- fix(security): block unauthenticated MCP, enforce document ownership
- fix(oauth): add missing last_used_at column and fix null guard
- fix(api): resolve git template file 404 from encoded path slashes
- fix(security): replace gorilla/csrf with net/http.CrossOriginProtection
- fix(security): MaxBytesReader on all form endpoints, lint v2.11.3 fixes
- fix(shutdown): two-stage shutdown to handle persistent MCP SSE sessions
- fix(api): add real pagination to ZIM archives and git templates
- fix(ui): layout, pagination, cursors, and a11y polish
- fix(sse): singleton store, initial flush, and external theme script
- fix(ui): dashboard grid, SSE indicator, favicons, and CSP hash
- fix(sse): fix 504 timeout, write deadline, and slow shutdown
- fix(kiwix): resolve versioned content IDs and fix search parameters
- fix(queue): wire River sync jobs, fix UniqueOpts, track health status
- fix: resolve nil-interface crash, index cleanup, SSRF, and doc content
- fix(mcp): resolve tool discovery by replacing any with concrete types
- fix(server): OAuth CSRF, scope validation, and MCP timeout
- fix: pre-deploy hardening across security, compliance, and UX
- fix: comprehensive remediation across security, data integrity, and performance
- fix(security): comprehensive security audit fixes
- fix(security): production config, logger injection, ACL, CSP, DNS rebinding
- fix(security): SSRF, XSS, open redirect, header injection
- fix(security): Add admin auth guard, sanitize error messages, move SSE under admin
- fix(ci): Mount DinD socket instead of nested DinD service container
- fix(ci): Fix integration test and security scan failures
- fix(ci): Add DinD for integration tests, fix lint, drop GitHub CI
- fix(security): Remediate critical and high audit findings
- fix(ci): use correct action refs for Forgejo runner
- fix(devcontainer): workspace mount

### Maintenance

- chore: consolidate node_modules gitignore pattern
- chore: remove stale Meilisearch and Confluence references
- chore(ci): align workflows with mcp-gate patterns
- chore: add GitHub mirror, CI/CD, and rename Go module
- chore: add MIT license
- chore(frontend): add ESLint, Prettier, and coverage thresholds
- chore(devcontainer): auto-load .env via docker-compose env_file
- chore: upgrade Go 1.25.6 → 1.26.1, pin versions in CI
- chore(lint): upgrade to 3-tier golangci-lint config and fix 619 issues
- chore(lint): resolve all 368 golangci-lint issues to zero
- chore(lint): add golangci-lint v2 config with Google Go style rules
- chore: remove dead code (RollbackMigration, AccessTokenFromContext, RedisConfig)
- chore(docs): Remove PHP/Laravel-specific documentation
- chore: Update memory bank and enable gopls plugin

[0.28.0]: https://github.com/c-premus/documcp/compare/v0.27.6...v0.28.0
[0.27.6]: https://github.com/c-premus/documcp/compare/v0.27.5...v0.27.6
[0.27.5]: https://github.com/c-premus/documcp/compare/v0.27.4...v0.27.5
[0.27.4]: https://github.com/c-premus/documcp/compare/v0.27.3...v0.27.4
[0.27.3]: https://github.com/c-premus/documcp/compare/v0.27.2...v0.27.3
[0.27.2]: https://github.com/c-premus/documcp/compare/v0.27.1...v0.27.2
[0.27.1]: https://github.com/c-premus/documcp/compare/v0.27.0...v0.27.1
[0.27.0]: https://github.com/c-premus/documcp/compare/v0.26.1...v0.27.0
[0.26.1]: https://github.com/c-premus/documcp/compare/v0.26.0...v0.26.1
[0.26.0]: https://github.com/c-premus/documcp/compare/v0.25.2...v0.26.0
[0.25.2]: https://github.com/c-premus/documcp/compare/v0.25.1...v0.25.2
[0.25.1]: https://github.com/c-premus/documcp/compare/v0.25.0...v0.25.1
[0.25.0]: https://github.com/c-premus/documcp/compare/v0.24.1...v0.25.0
[0.24.1]: https://github.com/c-premus/documcp/compare/v0.24.0...v0.24.1
[0.24.0]: https://github.com/c-premus/documcp/compare/v0.23.2...v0.24.0
[0.23.2]: https://github.com/c-premus/documcp/compare/v0.23.1...v0.23.2
[0.23.1]: https://github.com/c-premus/documcp/compare/v0.23.0...v0.23.1
[0.23.0]: https://github.com/c-premus/documcp/compare/v0.22.0...v0.23.0
[0.22.0]: https://github.com/c-premus/documcp/compare/v0.21.1...v0.22.0
[0.21.1]: https://github.com/c-premus/documcp/compare/v0.21.0...v0.21.1
[0.21.0]: https://github.com/c-premus/documcp/compare/v0.20.1...v0.21.0
[0.20.1]: https://github.com/c-premus/documcp/compare/v0.20.0...v0.20.1
[0.20.0]: https://github.com/c-premus/documcp/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/c-premus/documcp/compare/v0.18.2...v0.19.0
[0.18.2]: https://github.com/c-premus/documcp/compare/v0.18.1...v0.18.2
[0.18.1]: https://github.com/c-premus/documcp/compare/v0.18.0...v0.18.1
[0.18.0]: https://github.com/c-premus/documcp/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/c-premus/documcp/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/c-premus/documcp/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/c-premus/documcp/compare/v0.14.2...v0.15.0
[0.14.2]: https://github.com/c-premus/documcp/compare/v0.14.1...v0.14.2
[0.14.1]: https://github.com/c-premus/documcp/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/c-premus/documcp/compare/v0.13.3...v0.14.0
[0.13.3]: https://github.com/c-premus/documcp/compare/v0.13.2...v0.13.3
[0.13.2]: https://github.com/c-premus/documcp/compare/v0.13.1...v0.13.2
[0.13.1]: https://github.com/c-premus/documcp/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/c-premus/documcp/compare/v0.12.2...v0.13.0
[0.12.2]: https://github.com/c-premus/documcp/compare/v0.12.1...v0.12.2
[0.12.1]: https://github.com/c-premus/documcp/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/c-premus/documcp/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/c-premus/documcp/compare/v0.10.4...v0.11.0
[0.10.4]: https://github.com/c-premus/documcp/compare/v0.10.3...v0.10.4
[0.10.3]: https://github.com/c-premus/documcp/compare/v0.10.2...v0.10.3
[0.10.2]: https://github.com/c-premus/documcp/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/c-premus/documcp/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/c-premus/documcp/compare/v0.9.7...v0.10.0
[0.9.7]: https://github.com/c-premus/documcp/compare/v0.9.6...v0.9.7
[0.9.6]: https://github.com/c-premus/documcp/compare/v0.9.5...v0.9.6
[0.9.5]: https://github.com/c-premus/documcp/compare/v0.9.4...v0.9.5
[0.9.4]: https://github.com/c-premus/documcp/compare/v0.9.3...v0.9.4
[0.9.3]: https://github.com/c-premus/documcp/compare/v0.9.2...v0.9.3
[0.9.2]: https://github.com/c-premus/documcp/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/c-premus/documcp/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/c-premus/documcp/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/c-premus/documcp/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/c-premus/documcp/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/c-premus/documcp/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/c-premus/documcp/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/c-premus/documcp/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/c-premus/documcp/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/c-premus/documcp/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/c-premus/documcp/compare/v0.3.4...v0.4.0
[0.3.4]: https://github.com/c-premus/documcp/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/c-premus/documcp/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/c-premus/documcp/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/c-premus/documcp/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/c-premus/documcp/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/c-premus/documcp/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/c-premus/documcp/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/c-premus/documcp/compare/v0.1.6...v0.2.0
[0.1.6]: https://github.com/c-premus/documcp/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/c-premus/documcp/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/c-premus/documcp/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/c-premus/documcp/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/c-premus/documcp/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/c-premus/documcp/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/c-premus/documcp/compare/v0.0.3...v0.1.0
[0.0.3]: https://github.com/c-premus/documcp/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/c-premus/documcp/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/c-premus/documcp/releases/tag/v0.0.1
