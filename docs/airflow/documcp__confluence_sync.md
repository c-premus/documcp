# DAG: documcp__confluence_sync

## Purpose

Synchronize Confluence spaces from configured Confluence Cloud/Server instances. This imports new spaces, updates metadata for existing ones, and disables spaces that are no longer available.

## Schedule

- **Cron:** `30 5 * * *`
- **Frequency:** Daily at 5:30 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan confluence:sync
```

## What It Does

1. Queries each enabled Confluence service for available spaces
2. Fetches space metadata (key, name, description, type, status)
3. Creates new space records for newly discovered spaces
4. Updates metadata for existing spaces (when forced or first sync)
5. Disables spaces that are no longer in Confluence
6. Syncs changes to Meilisearch index for full-text search
7. Logs sync statistics

**Note:** This command syncs space metadata only, not page content. Page content is fetched on-demand via MCP tools.

## Command Options

```bash
# Sync from all enabled services
php artisan confluence:sync

# Sync from a specific service only
php artisan confluence:sync --service=confluence-primary

# Force update all spaces (even if already synced)
php artisan confluence:sync --force

# Limit number of spaces to sync
php artisan confluence:sync --limit=100

# Verbose output with per-space details
php artisan confluence:sync -v
```

## Expected Output

### Success (Exit 0)
```
Syncing Confluence spaces...
  Service: confluence-primary (https://company.atlassian.net)
    ✓ Created 5 new spaces
    ✓ Updated 12 existing spaces
    ✓ Disabled 0 spaces

Sync completed: 17 spaces synced from 1 service(s).
```

### Partial Success (Exit 0 with warnings)
```
Syncing Confluence spaces...
  Service: confluence-primary (https://company.atlassian.net)
    ✓ Created 5 new spaces
    ✓ Updated 10 existing spaces
    ✓ Disabled 2 spaces
  Service: confluence-secondary (https://backup.atlassian.net)
    ✗ Connection failed: 401 Unauthorized

Sync completed with warnings: 17 spaces synced from 1 of 2 service(s).
```

### Failure (Exit 1)
```
Syncing Confluence spaces...
  Service: confluence-primary (https://company.atlassian.net)
    ✗ Connection failed: 401 Unauthorized

No spaces synced. All services unreachable or authentication failed.
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
    dag_id='documcp__confluence_sync',
    default_args=default_args,
    description='Daily Confluence space sync for DocuMCP',
    schedule='30 5 * * *',
    start_date=datetime(2025, 12, 1),
    catchup=False,
    tags=['documcp', 'confluence', 'sync', 'daily', 'knowledge'],
)
def documcp_confluence_sync_dag():

    @task()
    def run_confluence_sync():
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        docker_exec_artisan(command='confluence:sync', timeout=600)

    run_confluence_sync()

documcp_confluence_sync_dag()
```

## Trace Context Propagation

This DAG uses `docker_exec_artisan()` from `common.docker`, which automatically
propagates W3C Trace Context (`TRACEPARENT`) to DocuMCP via environment variable.

This enables distributed tracing in Grafana:
- **Airflow DAG span** → docker exec → **DocuMCP artisan command spans**
- Traces visible in Tempo, correlated with DocuMCP logs in Loki via `trace_id`

To disable trace propagation for debugging:
```python
docker_exec_artisan(command='confluence:sync', propagate_trace=False)
```

## Notifications

### Success
- Total spaces synced
- Breakdown: created, updated, disabled
- Completion timestamp

### Failure
- Error message from command
- Which service(s) failed to connect
- Suggests checking API credentials

## Dependencies

This DAG should run **after** `documcp__zim_sync` to spread load:
- `zim:sync` runs at 5:00 AM UTC
- `confluence:sync` runs at 5:30 AM UTC
- `git-template:sync` runs at 6:00 AM UTC

## Monitoring

### Metrics to Track
- Spaces synced per run
- New spaces discovered
- Spaces disabled (may indicate Confluence issues)
- Sync duration
- API rate limit usage

### Alerts
- No spaces synced (all services down)
- Authentication failures (expired API token)
- Large number of spaces disabled (permissions change?)
- Sync taking longer than 5 minutes

## Troubleshooting

### No spaces synced
1. Check Confluence API connectivity:
   ```bash
   docker exec documcp-app php artisan confluence:stats
   ```

2. Verify external service configuration:
   ```bash
   docker exec documcp-app php artisan tinker
   >>> ExternalService::where('type', 'confluence')->get()
   ```

3. Test API credentials:
   ```bash
   docker exec documcp-app php artisan services:health-check
   ```

### Authentication failures
1. Verify API token hasn't expired
2. Check API token permissions (read access to spaces)
3. For Confluence Cloud, ensure using API token not password
4. Update credentials in DocuMCP admin panel

### Spaces being disabled unexpectedly
1. Check space permissions in Confluence
2. Verify API user has access to all spaces
3. Review sync logs for specific error messages

## Related Commands

```bash
# Manual sync
docker exec documcp-app php artisan confluence:sync

# View Confluence statistics
docker exec documcp-app php artisan confluence:stats

# Check external service health
docker exec documcp-app php artisan services:health-check

# Force sync with updates
docker exec documcp-app php artisan confluence:sync --force
```
