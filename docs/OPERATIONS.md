# Operations

Backup, restore, and readiness monitoring for a DocuMCP deployment. The
code handles everything in pure Go and the storage is standard Postgres +
filesystem/S3 — backups rely on the tooling you already run. This doc
documents *which tool for which state* and an end-to-end restore drill.

See also:
- Metrics exposed at `/metrics` (HTTP) — documented in `docs/PROMETHEUS_METRICS.md`
- Grafana alert rules provisioned from `dist/alerts/documcp.json`
- Health endpoints: `/health` (liveness), `/health/ready` (JSON dependency check), worker `/readyz` on port `QUEUE_HEALTH_PORT`

## What needs backing up

| State | Source of truth | Backup tool |
|-------|-----------------|-------------|
| Documents, users, OAuth clients, tokens, scope grants, search index, River queue | Postgres (all data including River's own tables) | `pg_dump` |
| Document blobs (PDFs, DOCX, XLSX, EPUB) | `STORAGE_BASE_PATH/documents/` (FS mode) or S3 bucket (S3 mode) | `rsync` or `aws s3 sync` / `rclone` |
| Git template scratch clones | `STORAGE_BASE_PATH/git/` | **skip** — regenerated on next sync |
| Worker extraction scratch | `STORAGE_BASE_PATH/worker-tmp/` | **skip** — transient |

The `git/` and `worker-tmp/` dirs live under `STORAGE_BASE_PATH` but are
scratch. Including them inflates backup size without adding recoverability.

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

**Retention sketch**: keep daily dumps for 7 days, weekly for 4 weeks,
monthly for 12 months. Size scales with document count + search index;
expect a few hundred MB per 10,000 documents once the FTS vectors land.

### Restore

```bash
# 1. Stop the app (so no new writes land during restore).
docker compose stop documcp worker

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
docker compose up -d documcp worker

# 5. Verify.
curl -s http://localhost:8080/health/ready | jq
```

`--no-owner` and `--no-privileges` avoid replaying ownership/grant rows
that would fail against a different target DB role. The schema's role
needs are minimal — standard `GRANT ALL` on the DB is enough.

## Document blobs — filesystem driver

Primary data path is `STORAGE_BASE_PATH/documents/`. The tree structure
mirrors the DB `documents.file_path` column — keys look like
`{file_type}/{uuid}.{ext}`, so the tree is wide but shallow.

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
docker compose stop documcp worker

# Replace the documents tree wholesale.
rsync -aHAX --delete \
  "/backup/documcp/documents-20260416/" \
  "${STORAGE_BASE_PATH}/documents/"

docker compose up -d documcp worker
```

If the Postgres restore and the blob restore are from different snapshots,
`documents.file_path` rows may point at blobs that don't exist. The
startup `RecoverStuckDocuments` job won't catch this — the docs will
return 404 on download. Always snapshot Postgres and blobs in the same
window; use `pg_dump` immediately after `rsync` completes, or the reverse.

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
docker compose stop documcp worker

aws s3 sync \
  "s3://documcp-backup/" \
  "s3://documcp-primary/" \
  --delete

docker compose up -d documcp worker
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
curl -sf http://staging-documcp/health
curl -sf http://staging-documcp/health/ready
# 5. Curl a known-good document by UUID (copy from prod DB).
curl -sf \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  "http://staging-documcp/api/documents/$TEST_UUID" | jq .title
# 6. Search for a distinctive term and verify hits.
curl -sf \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  "http://staging-documcp/api/documents/search?q=hexaplex-quokka-42" | jq
# 7. Tear down the staging host.
```

If any step fails, the backup is not providing the recoverability it
claims to. Treat a failing drill as a production incident.

## Readiness monitoring

### Metrics

| Metric | What it means | Pair with |
|--------|---------------|-----------|
| `documcp_ready` | 1 when Postgres + Redis respond to Ping on the uninstrumented pool on the last scrape. Self-collecting gauge — no probe traffic required. | `ReadinessFailing` alert (fires at 0 for 2m) |
| `documcp_river_leader_active` | 1 when the `river_leader` Postgres row is non-expired. 0 means no replica holds the lease and periodic jobs are not firing. | `NoRiverLeader` alert (fires at 0 for 5m) |
| `documcp_db_open_connections` | pgxpool live connection count on the main (instrumented) pool | correlate with `ReadinessFailing` if Postgres is the failing dependency |
| `documcp_redis_pool_misses_total` | rate of Redis connection acquisition misses | correlate with `ReadinessFailing` if Redis is the failing dependency |

### Alerts

Provisioned from `dist/alerts/documcp.json` into Grafana's
`provisioning/alerting/` directory. Two rules:

- **`NoRiverLeader`** — `documcp_river_leader_active == 0 for 5m`. The
  top cause is deploying two `serve` replicas with no worker — both
  enqueue jobs but neither processes them, and periodic jobs stop firing
  silently. Fix: make sure at least one replica runs with
  `--with-worker` or as a dedicated `worker` container.
- **`ReadinessFailing`** — `documcp_ready == 0 for 2m`. Check
  `/health/ready` JSON for the specific dependency (`postgres` or
  `redis`) and investigate from there.

Notification routing (Matrix, email, PagerDuty, etc.) is configured in
Grafana outside the repo — contact points + notification policies on the
Alerting → Admin page, or via separate provisioning YAML. The rules-as-
code pattern handles the detection layer only.

### Endpoints

| Endpoint | Purpose | Auth |
|----------|---------|------|
| `/health` | Liveness — returns 200 if the process is running | none |
| `/health/ready` | Readiness — 200 when all dependencies respond to Ping, 503 otherwise. JSON body includes per-service status. | none |
| Worker `/readyz` (port `QUEUE_HEALTH_PORT`, default 9090) | Same readiness check, used by Docker/K8s worker healthchecks | none |
| `/metrics` | Prometheus scrape target | `INTERNAL_API_TOKEN` when set |

`/health/ready` and `/readyz` call `Ping` on an uninstrumented pgxpool
and the bare Redis client — they do not emit otelpgx/redisotel spans, so
probe traffic is invisible to tracing backends.
