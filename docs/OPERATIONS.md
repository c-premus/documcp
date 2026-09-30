# Operations

Backup, restore, and readiness monitoring for a DocuMCP deployment. The
code handles everything in pure Go and the storage is standard Postgres +
filesystem/S3 — backups rely on the tooling you already run. This doc
documents *which tool for which state* and an end-to-end restore drill.

The `docker compose` commands below assume the bundled `docker-compose.yml`,
where a single `app` service runs `serve --with-worker` (HTTP and River
workers in one container). If you run separate `serve` and `worker`
containers, stop and start both.

See also:
- Metrics exposed at `/metrics` (HTTP) — documented in `docs/PROMETHEUS_METRICS.md`
- Grafana alert rules provisioned from `dist/alerts/documcp.json`
- Health endpoints: `/health` (liveness), `/health/ready` (JSON dependency check), worker `/readyz` on port `WORKER_HEALTH_PORT` (worker-only mode)

## What needs backing up

| State | Source of truth | Backup tool |
|-------|-----------------|-------------|
| Documents, users, OAuth clients, tokens, scope grants, search index, River queue | Postgres (all data including River's own tables) | `pg_dump` |
| Document blobs (PDFs, DOCX, XLSX, EPUB) | `STORAGE_BASE_PATH/STORAGE_DOCUMENT_PATH` (FS mode; `STORAGE_DOCUMENT_PATH` defaults to `documents`) or S3 bucket (S3 mode) | `rsync` or `aws s3 sync` / `rclone` |
| `ENCRYPTION_KEY` (and `ENCRYPTION_KEY_PREVIOUS` during a rotation) | Your secret store / `.env` | Store with, but separate from, the dumps — see [Encryption keys](#encryption-keys) |
| Git template scratch clones | `STORAGE_BASE_PATH/git/` | **skip** — regenerated on next sync |
| Worker extraction scratch | `STORAGE_BASE_PATH/worker-tmp/` | **skip** — transient |
| Browser sessions, rate-limit counters, device-flow failure counters | Redis | **skip** — transient; users sign in again after a Redis loss |

The `git/` and `worker-tmp/` dirs are fixed names under `STORAGE_BASE_PATH`
and are scratch. Including them inflates backup size without adding
recoverability.

## Postgres

### Back up

```bash
# Online logical backup — runs against a live database, no downtime.
# Compressed dump, ~5-10× smaller than plain SQL.
pg_dump \
  --host="$POSTGRES_HOST" \
  --port="$POSTGRES_PORT" \
  --username="$POSTGRES_USER" \
  --dbname="$POSTGRES_DB" \
  --format=custom \
  --file="documcp-$(date -u +%Y%m%dT%H%M%SZ).dump"
```

Format `custom` (not `plain`) is required for `pg_restore` selective
operations later. Schedule via cron / systemd timer on the host that has
direct Postgres access (typically the DB host itself, not the app host).

The bundled compose file does not publish the Postgres port to the host.
Run `pg_dump` inside the `postgres` container instead:

```bash
docker compose exec -T postgres \
  pg_dump --username="$DB_USERNAME" --dbname="$DB_DATABASE" --format=custom \
  > "documcp-$(date -u +%Y%m%dT%H%M%SZ).dump"
```

The `postgres_data` volume is mounted at `/var/lib/postgresql` (PostgreSQL
18 layout; the cluster lives in `/var/lib/postgresql/18/docker`). A logical
dump is the supported backup path; copying that directory from a running
container does not produce a consistent backup.

**Retention sketch**: keep daily dumps for 7 days, weekly for 4 weeks,
monthly for 12 months. Size scales with document count + search index;
expect a few hundred MB per 10,000 documents once the FTS vectors land.

### Restore

```bash
# 1. Stop the app (so no new writes land during restore).
docker compose stop app

# 2. Drop and recreate the database. DESTRUCTIVE — make sure this is
# the right environment.
psql --host="$POSTGRES_HOST" --username="$POSTGRES_USER" \
  -c "DROP DATABASE IF EXISTS documcp;" \
  -c "CREATE DATABASE documcp OWNER $POSTGRES_USER;"

# 3. Restore. -j flag parallelizes across tables; pick ~half your CPU count.
pg_restore \
  --host="$POSTGRES_HOST" \
  --port="$POSTGRES_PORT" \
  --username="$POSTGRES_USER" \
  --dbname=documcp \
  --jobs=4 \
  --no-owner \
  --no-privileges \
  documcp-20260416T120000Z.dump

# 4. Start the app. Startup runs migrations against the restored schema
# — this is a no-op when the dump is from the same schema version.
docker compose up -d app

# 5. Verify.
curl -s http://localhost:8080/health/ready | jq
```

`--no-owner` and `--no-privileges` avoid replaying ownership/grant rows
that would fail against a different target DB role. The schema's role
needs are minimal — standard `GRANT ALL` on the DB is enough.

Both `serve` and `worker` apply pending goose and River migrations on
startup. To apply them as a separate step (for example, from an init
container before the app starts), run `documcp migrate`, which runs the
migrations and exits.

### Encryption keys

When `ENCRYPTION_KEY` is set, `external_services.api_key` and
`git_templates.git_token` are stored encrypted. A restored dump is only
readable with the key that wrote it, so keep the key with your backups.

If you restore a dump taken before a key rotation:

1. Set `ENCRYPTION_KEY` to the current key and `ENCRYPTION_KEY_PREVIOUS`
   to the key the dump was written under. The app decrypts with either key.
2. Start the app.
3. Run `documcp rekey`. It re-encrypts every row not already under
   `ENCRYPTION_KEY`, skips rows that are, and exits non-zero if any row
   fails or if `ENCRYPTION_KEY` is empty. It is safe to run repeatedly.
4. Remove `ENCRYPTION_KEY_PREVIOUS` on the next deploy.

The same steps apply to a planned key rotation (`documcp rekey --help`
lists them).

## Document blobs — filesystem driver

The blob root is `STORAGE_BASE_PATH/STORAGE_DOCUMENT_PATH`
(`STORAGE_DOCUMENT_PATH` defaults to `documents`). The examples below use
the default. The tree structure mirrors the DB `documents.file_path`
column — keys look like `{file_type}/{uuid}.{ext}`, so the tree is wide
but shallow.

In the bundled compose file, `STORAGE_BASE_PATH` is `/data/storage` inside
the `app` container, backed by the `document_storage` named volume. Run
`rsync` against the volume's host path or from a container that mounts it.

### Back up

```bash
# Preserve permissions, hardlinks, ACLs, xattrs.
rsync -aHAX --delete \
  "${STORAGE_BASE_PATH}/documents/" \
  "/backup/documcp/documents-$(date -u +%Y%m%d)/"
```

Run from the DocuMCP host. `rsync` is incremental — running nightly after
the first full backup takes minutes even on large corpora.

### Restore

```bash
# Stop the app so half-restored state can't be read.
docker compose stop app

# Replace the documents tree wholesale.
rsync -aHAX --delete \
  "/backup/documcp/documents-20260416/" \
  "${STORAGE_BASE_PATH}/documents/"

docker compose up -d app
```

If the Postgres restore and the blob restore are from different snapshots,
`documents.file_path` rows may point at blobs that don't exist.
`RecoverStuckDocuments` won't catch this: it runs at startup wherever River
workers run (`serve --with-worker` and `worker`) and only re-dispatches
jobs for documents stuck in the `uploaded` or `extracted` state. The
affected documents return 404 on download. Always snapshot Postgres and
blobs in the same window; use `pg_dump` immediately after `rsync`
completes, or the reverse.

## Document blobs — S3 driver

### Back up

S3 buckets should have **versioning enabled** — point-in-time recovery is
native to the object store. For cross-region redundancy or cold storage,
replicate the versioned bucket:

```bash
# aws-cli approach.
aws s3 sync \
  "s3://documcp-primary/" \
  "s3://documcp-backup/"

# rclone alternative — works with non-AWS backends (Garage, SeaweedFS).
rclone sync \
  primary:documcp-primary \
  backup:documcp-backup \
  --progress
```

### Restore

Either restore specific object versions (`aws s3api list-object-versions`
+ `aws s3api copy-object` with `VersionId`) or sync the backup bucket
back:

```bash
docker compose stop app

aws s3 sync \
  "s3://documcp-backup/" \
  "s3://documcp-primary/" \
  --delete

docker compose up -d app
```

Same Postgres/S3 snapshot-window rule applies.

## End-to-end restore drill

Running this drill quarterly against a staging environment is the only
way to know the backups work. Script the drill; the write-up matters less
than the exercise.

```bash
# 1. Provision an empty staging host with the deploy compose.
# 2. Copy the most recent production Postgres dump + blob archive over.
# 3. Run the restore sequence above.
# 4. Curl the health endpoints.
curl -sf https://staging.example.com/health
curl -sf https://staging.example.com/health/ready
# 5. Curl a known-good document by UUID (copy from prod DB).
curl -sf \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  "https://staging.example.com/api/documents/$TEST_UUID" | jq .data.title
# 6. Search for a distinctive term and verify hits (token needs search:read).
curl -sf \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  "https://staging.example.com/api/search?q=hexaplex-quokka-42" | jq
# 7. Tear down the staging host.
```

If any step fails, the backup is not providing the recoverability it
claims to. Treat a failing drill as a production incident.

## Readiness monitoring

### Metrics

| Metric | What it means | Pair with |
|--------|---------------|-----------|
| `documcp_ready` | 1 when Postgres + Redis respond to Ping on the uninstrumented pool and bare Redis client on the last scrape. Self-collecting gauge — no probe traffic required. | `DocuMCP — readiness failing` alert (fires at 0 for 2m) |
| `documcp_river_leader_active` | 1 when the `river_leader` Postgres row is non-expired. 0 means no replica holds the lease and periodic jobs are not firing. | `DocuMCP — no River leader` alert (fires at 0 for 5m) |
| `documcp_db_open_connections` | pgxpool live connection count on the main (instrumented) pool | correlate with the readiness alert if Postgres is the failing dependency |
| `documcp_redis_pool_misses_total` | rate of connection acquisition misses on the main Redis client | correlate with the readiness alert if Redis is the failing dependency |

### Alerts

Provisioned from `dist/alerts/documcp.json` (generated from
`grafana/src/alerts/rules.ts`) into Grafana's `provisioning/alerting/`
directory. Both rules live in the `documcp` group of the `DocuMCP` folder,
evaluate every 30s, and also fire when the metric has no data (for
example, when no instance is being scraped). Two rules:

- **`DocuMCP — no River leader`** (UID `documcp_no_river_leader`) —
  `documcp_river_leader_active == 0` for 5m. The top cause is deploying
  two `serve` replicas with no worker — both enqueue jobs but neither
  processes them, and periodic jobs stop firing silently. Fix: make sure
  at least one replica runs with `--with-worker` or as a dedicated
  `documcp worker` container.
- **`DocuMCP — readiness failing`** (UID `documcp_readiness_failing`) —
  `documcp_ready == 0` for 2m. Check the serve-mode `/health/ready` JSON
  for the specific dependency (`postgres` or `redis`) and investigate
  from there. During a Redis outage, rate limiting keeps working per
  instance and logs `rate limiter lost Redis; enforcing per-process
  limits`; sessions and live events do not (see
  [Redis troubleshooting](REDIS.md#rate-limits-stop-being-shared-across-instances)).

Notification routing (Matrix, email, PagerDuty, etc.) is configured in
Grafana outside the repo — contact points + notification policies on the
Alerting → Admin page, or via separate provisioning YAML. The rules-as-
code pattern handles the detection layer only.

### Endpoints

Serve mode (`documcp serve`, port `SERVER_PORT`, default 8080):

| Endpoint | Purpose | Auth |
|----------|---------|------|
| `/health` | Liveness — returns 200 with a JSON `status` and `version` if the process is running | none |
| `/health/ready` | Readiness — 200 when all dependencies respond to Ping, 503 otherwise. JSON body: `status` (`ready` or `not_ready`), `version`, and `services` with `postgres` and `redis` each `healthy` or `unhealthy`. | none |
| `/metrics` | Prometheus scrape target | `INTERNAL_API_TOKEN` when set |

Worker-only mode (`documcp worker`, port `WORKER_HEALTH_PORT`, default
9090). `serve --with-worker` does not start this server; use the serve-mode
endpoints above.

| Endpoint | Purpose | Auth |
|----------|---------|------|
| `/healthz`, `/health` | Liveness — plain-text `ok` with 200 | none |
| `/readyz`, `/health/ready` | Readiness — plain-text `ready` with 200, or `database not ready` / `redis not ready` with 503. Not JSON. | none |
| `/metrics` | Prometheus scrape target | `INTERNAL_API_TOKEN` when set |

`documcp health` probes `/health/ready` on port 8080. For a worker-only
container, pass `--port` with the `WORKER_HEALTH_PORT` value.

When `INTERNAL_API_TOKEN` is unset, `/metrics` is served without
authentication and the app logs a warning at startup.

The readiness endpoints call `Ping` on an uninstrumented pgxpool and the
bare Redis client — they do not emit otelpgx/redisotel spans, so probe
traffic is invisible to tracing backends.
