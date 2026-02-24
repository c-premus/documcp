# DAG: documcp__zim_cleanup

## Purpose

Remove disabled ZIM archive records from the database. Archives become disabled when they're no longer available from Kiwix-Serve (e.g., replaced by newer versions or removed from the catalog).

## Schedule

- **Cron:** `0 6 * * *`
- **Frequency:** Daily at 6:00 AM UTC (after zim:sync at 5 AM)
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan zim:cleanup
```

## What It Does

1. Queries all ZIM archive records where `is_enabled = false`
2. Permanently deletes these records from the database
3. Logs the count of removed records

**Note:** This only removes database records. The actual ZIM files are managed by Kiwix-Serve, not DocuMCP.

## Command Options

```bash
# Preview what would be deleted (no actual deletion)
php artisan zim:cleanup --dry-run

# Keep archives disabled for N days before cleanup
php artisan zim:cleanup --retention=7

# Clean up only for specific service
php artisan zim:cleanup --service=kiwix-primary
```

## Expected Output

### Success (Exit 0)
```
Deleted 15 disabled archive(s).
```

### Dry Run
```
Would delete 15 disabled archive(s).
```

### No Archives to Clean
```
Deleted 0 disabled archive(s).
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
    'retries': 2,
    'retry_delay': timedelta(minutes=5),
    'on_failure_callback': send_failure_notification,
}

@dag(
    dag_id='documcp__zim_cleanup',
    default_args=default_args,
    description='Daily ZIM archive cleanup for DocuMCP',
    schedule='0 6 * * *',
    start_date=datetime(2025, 12, 1),
    catchup=False,
    tags=['documcp', 'zim', 'maintenance', 'daily', 'knowledge'],
)
def documcp_zim_cleanup_dag():

    @task()
    def run_zim_cleanup():
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        docker_exec_artisan(command='zim:cleanup', timeout=300)

    run_zim_cleanup()

documcp_zim_cleanup_dag()
```

## Trace Context Propagation

This DAG uses `docker_exec_artisan()` from `common.docker`, which automatically
propagates W3C Trace Context (`TRACEPARENT`) to DocuMCP via environment variable.

This enables distributed tracing in Grafana:
- **Airflow DAG span** → docker exec → **DocuMCP artisan command spans**
- Traces visible in Tempo, correlated with DocuMCP logs in Loki via `trace_id`

To disable trace propagation for debugging:
```python
docker_exec_artisan(command='zim:cleanup', propagate_trace=False)
```

## Notifications

### Success
- Count of archives removed
- Completion timestamp

### Failure
- Error message from command
- Suggests checking logs

## Why Archives Get Disabled

1. **Version updates**: When Kiwix updates a ZIM file, the old version is disabled
2. **Catalog removal**: Archive removed from Kiwix-Serve catalog
3. **Manual disable**: Admin disabled via UI

## Scheduling Note

This DAG runs **after** `documcp__zim_sync` to clean up any archives that were disabled during the sync process.

Schedule:
- `zim:sync` runs at 5:00 AM UTC
- `zim:cleanup` runs at 6:00 AM UTC

## Related Commands

```bash
# Preview cleanup
docker exec documcp-app php artisan zim:cleanup --dry-run

# Check current disabled count
docker exec documcp-app php artisan tinker
>>> ZimArchive::where('is_enabled', false)->count()

# View ZIM statistics
docker exec documcp-app php artisan zim:stats
```
