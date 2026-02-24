# DAG: documcp__zim_sync

## Purpose

Synchronize ZIM archive catalog from Kiwix-Serve instances. This imports new archives, updates metadata for existing ones, and disables archives that are no longer available.

## Schedule

- **Cron:** `0 5 * * *`
- **Frequency:** Daily at 5:00 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan zim:sync
```

## What It Does

1. Queries each enabled Kiwix-Serve instance for its OPDS catalog
2. Parses the catalog to extract available ZIM archives
3. Creates new archive records for newly discovered archives
4. Updates metadata (title, description, language, etc.) for existing archives
5. Disables archives that are no longer in the catalog
6. Logs sync statistics

**Note:** This command does NOT download ZIM files. It only syncs metadata from Kiwix-Serve's catalog API.

## Command Options

```bash
# Sync from all enabled services
php artisan zim:sync

# Sync from a specific service only
php artisan zim:sync --service=kiwix-primary

# Verbose output with per-archive details
php artisan zim:sync -v

# Dry-run to preview changes
php artisan zim:sync --dry-run
```

## Expected Output

### Success (Exit 0)
```
Syncing ZIM archives from Kiwix-Serve...
  Service: kiwix-primary (http://kiwix-serve:8080)
    ✓ Found 45 archives in catalog
    ✓ Created 3 new archives
    ✓ Updated 42 existing archives
    ✓ Disabled 0 archives

Sync completed: 45 archives synced from 1 service(s).
```

### Partial Success (Exit 0 with warnings)
```
Syncing ZIM archives from Kiwix-Serve...
  Service: kiwix-primary (http://kiwix-serve:8080)
    ✓ Found 45 archives in catalog
    ✓ Created 3 new archives
    ✓ Updated 40 existing archives
    ✓ Disabled 2 archives
  Service: kiwix-secondary (http://kiwix-backup:8080)
    ✗ Connection failed: Connection refused

Sync completed with warnings: 45 archives synced from 1 of 2 service(s).
```

### Failure (Exit 1)
```
Syncing ZIM archives from Kiwix-Serve...
  Service: kiwix-primary (http://kiwix-serve:8080)
    ✗ Connection failed: Connection refused

No archives synced. All services unreachable.
```

## DAG Implementation

```python
from datetime import datetime, timedelta

from airflow.sdk import dag, task

from common.docker import container_is_running, docker_exec_artisan
from common.notifications import send_failure_notification, send_success_notification

default_args = {
    'owner': 'airflow',
    'depends_on_past': False,
    'retries': 3,
    'retry_delay': timedelta(minutes=10),
    'on_failure_callback': send_failure_notification,
}

@dag(
    dag_id='documcp__zim_sync',
    default_args=default_args,
    description='Daily ZIM archive catalog sync for DocuMCP',
    schedule='0 5 * * *',
    start_date=datetime(2025, 12, 1),
    catchup=False,
    tags=['documcp', 'zim', 'sync', 'daily', 'knowledge'],
)
def documcp_zim_sync_dag():

    @task()
    def run_zim_sync():
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        docker_exec_artisan(command='zim:sync', timeout=600)

    run_zim_sync()

documcp_zim_sync_dag()
```

## Trace Context Propagation

This DAG uses `docker_exec_artisan()` from `common.docker`, which automatically
propagates W3C Trace Context (`TRACEPARENT`) to DocuMCP via environment variable.

This enables distributed tracing in Grafana:
- **Airflow DAG span** → docker exec → **DocuMCP artisan command spans**
- Traces visible in Tempo, correlated with DocuMCP logs in Loki via `trace_id`

To disable trace propagation for debugging:
```python
docker_exec_artisan(command='zim:sync', propagate_trace=False)
```

## Notifications

### Success
- Total archives synced
- Breakdown: created, updated, disabled
- Completion timestamp

### Failure
- Error message from command
- Which service(s) failed to connect
- Suggests checking network connectivity

## Dependencies

This DAG should run **before** `documcp__zim_cleanup` so that archives disabled during sync can be cleaned up afterward.

Schedule:
- `zim:sync` runs at 5:00 AM UTC
- `zim:cleanup` runs at 6:00 AM UTC (same day, after sync)

## Monitoring

### Metrics to Track
- Archives synced per run
- New archives discovered
- Archives disabled (may indicate Kiwix-Serve issues)
- Sync duration

### Alerts
- No archives synced (all services down)
- Large number of archives disabled (catalog issue?)
- Sync taking longer than 5 minutes

## Troubleshooting

### No archives synced
1. Check Kiwix-Serve connectivity:
   ```bash
   docker exec documcp-app curl -s http://kiwix-serve:8080/catalog/root.xml
   ```

2. Verify external service configuration:
   ```bash
   docker exec documcp-app php artisan tinker
   >>> ExternalService::where('type', 'kiwix_serve')->get()
   ```

### Archives being disabled unexpectedly
1. Check Kiwix-Serve catalog directly
2. Verify archive file exists on Kiwix-Serve
3. Review sync logs for specific error messages

## Related Commands

```bash
# Manual sync
docker exec documcp-app php artisan zim:sync

# View ZIM statistics
docker exec documcp-app php artisan zim:stats

# Check external service health
docker exec documcp-app php artisan services:health-check
```
