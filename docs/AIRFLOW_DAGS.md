# DocuMCP Airflow DAGs

All DocuMCP scheduled maintenance tasks are orchestrated by Apache Airflow instead of Laravel's built-in scheduler. This provides centralized monitoring, configurable retry policies, and integration with OpenTelemetry for distributed tracing.

## Overview

| DAG ID | Schedule | Command | Duration |
|--------|----------|---------|----------|
| `documcp__oauth_cleanup_daily` | Daily 2 AM UTC | `oauth:prune` | < 1 min |
| `documcp__orphaned_files_cleanup_weekly` | Sun 2 AM UTC | `documcp:cleanup-orphaned-files --force` | < 5 min |
| `documcp__health_check` | Daily 3 AM UTC | `services:health-check` | < 1 min |
| `documcp__zim_cleanup` | Daily 6 AM UTC | `zim:cleanup` | < 1 min |
| `documcp__search_index_verify_daily` | Daily 4 AM UTC | `documcp:verify-search-index --repair` | < 2 min |
| `documcp__zim_sync` | Daily 5 AM UTC | `zim:sync` | < 5 min |
| `documcp__confluence_sync` | Daily 5:30 AM UTC | `confluence:sync` | < 5 min |
| `documcp__git_template_sync` | Daily 6 AM UTC | `git-template:sync` | < 10 min |
| `documcp__soft_deleted_cleanup` | Sun 3 AM UTC | `documcp:cleanup-soft-deleted --retention=30` | < 2 min |

## Architecture

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Airflow   │────▶│  common.docker  │────▶│  DocuMCP App    │
│  Scheduler  │     │  docker_exec_   │     │  (php artisan)  │
└─────────────┘     │  artisan()      │     └─────────────────┘
       │            └─────────────────┘              │
       │              TRACEPARENT ──────────────▶     │
       ▼                                             ▼
┌─────────────┐                              ┌─────────────────┐
│   Apprise   │◀─────────────────────────────│  Structured     │
│ Notifications│                              │  JSON Logs      │
└─────────────┘                              └─────────────────┘
       │                                             │
       ▼                                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Grafana Alloy → Loki                     │
└─────────────────────────────────────────────────────────────┘
```

## DAG Reference

### documcp__oauth_cleanup_daily

Prunes expired OAuth tokens to maintain database health and security.

- **Schedule:** `0 2 * * *` (Daily at 2:00 AM UTC)
- **Command:** `php artisan oauth:prune`
- **Duration:** < 1 minute

**What it removes:**
- Expired authorization codes (> 10 minutes old)
- Expired access tokens (> 1 hour by default)
- Expired refresh tokens (> 30 days by default)
- Revoked tokens older than 7 days

**Exit codes:**
- `0`: Cleanup completed successfully
- `1`: Error during cleanup

---

### documcp__orphaned_files_cleanup_weekly

Removes orphaned files from storage that have no corresponding database records.

- **Schedule:** `0 2 * * 0` (Sundays at 2:00 AM UTC)
- **Command:** `php artisan documcp:cleanup-orphaned-files --force`
- **Duration:** < 5 minutes (depends on orphan count)

**When orphans occur:**
- Document deleted from database but file deletion failed
- Database transaction rolled back after file upload
- Manual database cleanup without file cleanup

**Exit codes:**
- `0`: Cleanup completed (output includes count of files removed)
- `1`: Error during cleanup

---

### documcp__search_index_verify_daily

Verifies Meilisearch index integrity by comparing database records with the search index.

- **Schedule:** `0 4 * * *` (Daily at 4:00 AM UTC)
- **Command:** `php artisan documcp:verify-search-index --repair`
- **Duration:** < 2 minutes

**What it checks:**
- Document count (database vs index)
- Orphaned entries (in index but not in database)
- Missing documents (in database but not in index)

**With `--repair` flag:**
- Removes orphaned entries from index
- Reindexes missing documents

**Exit codes:**
- `0`: Verification passed (or repairs completed)
- `1`: Verification failed or repair error

---

### documcp__health_check

Monitors the health of external services that DocuMCP depends on.

- **Schedule:** `0 3 * * *` (Daily at 3:00 AM UTC)
- **Command:** `php artisan services:health-check`
- **Duration:** < 1 minute
- **Spec:** `docs/airflow/documcp__health_check.md`

**What it checks:**
- HTTP connectivity to Kiwix-Serve instances
- Response time measurement
- Catalog availability

**Exit codes:**
- `0`: All services healthy
- `1`: One or more services unhealthy

---

### documcp__zim_cleanup

Removes disabled ZIM archive records from the database.

- **Schedule:** `0 6 * * *` (Daily at 6:00 AM UTC, after zim:sync at 5 AM)
- **Command:** `php artisan zim:cleanup`
- **Duration:** < 1 minute
- **Spec:** `docs/airflow/documcp__zim_cleanup.md`

**What it does:**
- Queries ZIM archives where `is_enabled = false`
- Permanently deletes these records from the database

**Exit codes:**
- `0`: Cleanup completed (output includes count)
- `1`: Error during cleanup

---

### documcp__zim_sync

Synchronizes ZIM archive catalog from Kiwix-Serve instances.

- **Schedule:** `0 5 * * *` (Daily at 5:00 AM UTC)
- **Command:** `php artisan zim:sync`
- **Duration:** < 5 minutes
- **Spec:** `docs/airflow/documcp__zim_sync.md`

**What it does:**
- Queries each Kiwix-Serve instance for its OPDS catalog
- Creates new archive records for discovered archives
- Updates metadata for existing archives
- Disables archives no longer in the catalog
- Syncs changes to Meilisearch index

**Exit codes:**
- `0`: Sync completed (may include warnings)
- `1`: All services unreachable

---

### documcp__confluence_sync

Synchronizes Confluence spaces from configured Confluence instances.

- **Schedule:** `30 5 * * *` (Daily at 5:30 AM UTC)
- **Command:** `php artisan confluence:sync`
- **Duration:** < 5 minutes
- **Spec:** `docs/airflow/documcp__confluence_sync.md`

**What it does:**
- Queries each configured Confluence instance for available spaces
- Creates new space records for discovered spaces
- Updates metadata for existing spaces (when forced or first sync)
- Disables spaces no longer in Confluence
- Syncs changes to Meilisearch index

**Exit codes:**
- `0`: Sync completed successfully
- `1`: All services unreachable or sync failed

---

### documcp__git_template_sync

Synchronizes Git template repositories by pulling latest changes.

- **Schedule:** `0 6 * * *` (Daily at 6:00 AM UTC)
- **Command:** `php artisan git-template:sync`
- **Duration:** < 10 minutes (depends on repository count and size)
- **Spec:** `docs/airflow/documcp__git_template_sync.md`

**What it does:**
- Iterates through all registered Git template repositories
- Clones/pulls latest changes from remote repositories
- Extracts and indexes template files
- Updates README content in Meilisearch index
- Detects essential files (CLAUDE.md, memory-bank/*, README.md)

**Exit codes:**
- `0`: Sync completed successfully
- `1`: Sync failed (check logs for specific repository errors)

---

### documcp__soft_deleted_cleanup

Permanently deletes documents that have been soft-deleted beyond the retention period.

- **Schedule:** `0 3 * * 0` (Sundays at 3:00 AM UTC)
- **Command:** `php artisan documcp:cleanup-soft-deleted --retention=30`
- **Duration:** < 2 minutes
- **Spec:** `docs/airflow/documcp__soft_deleted_cleanup.md`

**What it does:**
- Queries documents where `deleted_at` is older than retention period
- Permanently deletes documents from database
- Deletes associated files from storage
- Removes entries from Meilisearch index

**Exit codes:**
- `0`: Cleanup completed (output includes count)
- `1`: Error during cleanup

---

## Execution Pattern

All DocuMCP DAGs use `docker_exec_artisan()` from `common.docker` to execute artisan commands in the DocuMCP container:

```python
from common.docker import container_is_running, docker_exec_artisan

@task()
def run_command():
    if not container_is_running('documcp-app'):
        raise RuntimeError("documcp-app container is not running")

    docker_exec_artisan(command='<command>', timeout=120)
```

This replaces the previous shell script pattern. Benefits:
- Automatic W3C Trace Context propagation (`TRACEPARENT`)
- Container health check before execution
- Consistent error handling and timeout management

---

## Trace Context Propagation

`docker_exec_artisan()` automatically propagates W3C Trace Context (`TRACEPARENT`) to DocuMCP via environment variable. This enables distributed tracing in Grafana:

- **Airflow DAG span** → docker exec → **DocuMCP artisan command spans**
- Traces visible in Tempo, correlated with DocuMCP logs in Loki via `trace_id`

DocuMCP reads `TRACEPARENT` in `AppServiceProvider::boot()` and activates the trace context for the artisan command via `Tracing::activateEnvironmentTraceContext()`.

### Log Correlation

```json
{
  "timestamp": "2025-12-06T02:00:00Z",
  "level": "info",
  "message": "OAuth token cleanup completed",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "command": "oauth:prune"
}
```

### Disable Propagation

```python
docker_exec_artisan(command='services:health-check', propagate_trace=False)
```

---

## Notifications

All DAGs use a common notification pattern via Apprise:

### Success Notification

```python
from common.notifications import send_success_notification

@task()
def send_completion_notification(stats: dict):
    return send_success_notification(
        message=f"Task completed at {stats['timestamp']}",
        title="DocuMCP Task Complete",
    )
```

### Failure Notification

Configured via `default_args`:

```python
from common.notifications import send_failure_notification

default_args = {
    'on_failure_callback': send_failure_notification,
    'retries': 2,
    'retry_delay': timedelta(minutes=5),
}
```

---

## Monitoring

### Airflow UI

- View DAG runs and task status
- Check logs for each task execution
- Monitor run duration trends

### Grafana Dashboards

- Query Loki for DocuMCP command logs
- Filter by `trace_id` to correlate with Airflow runs
- Alert on command failures

### LogQL Queries

```logql
# All DocuMCP command logs
{app="documcp"} |= "artisan"

# Logs for specific trace ID
{app="documcp"} | json | trace_id="4bf92f3577b34da6a3ce929d0e0e4736"

# Failed commands
{app="documcp"} | json | level="error"
```

---

## Adding New DAGs

1. Create command in DocuMCP (`app/Console/Commands/`)
2. Create DAG spec document in `docs/airflow/`
3. Create DAG Python file using `docker_exec_artisan()` from `common.docker`
4. Update this document with new DAG entry

### DAG Naming Convention

```
documcp__{task}

Examples:
- documcp__health_check
- documcp__zim_sync
- documcp__soft_deleted_cleanup
```

### Tags

All DocuMCP DAGs should include these tags:
- `documcp` - Identifies as DocuMCP-related
- `knowledge` - Knowledge management system
- Category tag: `oauth`, `storage`, `search`, `zim`, `maintenance`
- Frequency tag: `daily`, `weekly`, `hourly`
