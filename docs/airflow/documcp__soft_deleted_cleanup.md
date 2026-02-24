# DAG: documcp__soft_deleted_cleanup

## Purpose

Permanently delete documents that have been soft-deleted for longer than the retention period (default: 30 days). This frees up database storage and ensures GDPR/data retention compliance.

## Schedule

- **Cron:** `0 3 * * 0`
- **Frequency:** Weekly on Sundays at 3:00 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan documcp:cleanup-soft-deleted --retention=30
```

## What It Does

1. Queries all documents where `deleted_at` is older than the retention period
2. Permanently deletes (force delete) these documents from the database
3. Deletes associated files from storage
4. Removes entries from Meilisearch index
5. Logs the count of permanently deleted documents

**Note:** This is a destructive operation. Documents cannot be recovered after this cleanup runs.

## Command Options

```bash
# Preview what would be deleted (no actual deletion)
php artisan documcp:cleanup-soft-deleted --dry-run

# Custom retention period (days)
php artisan documcp:cleanup-soft-deleted --retention=60

# Default retention (30 days)
php artisan documcp:cleanup-soft-deleted --retention=30

# Verbose output with document details
php artisan documcp:cleanup-soft-deleted --retention=30 -v
```

## Expected Output

### Success (Exit 0)
```
Cleaning up soft-deleted documents older than 30 days...
Permanently deleted 12 document(s).
```

### Dry Run
```
Cleaning up soft-deleted documents older than 30 days...
Would permanently delete 12 document(s):
  - Project Proposal.pdf (deleted 45 days ago)
  - Meeting Notes.docx (deleted 32 days ago)
  - ...
```

### No Documents to Clean
```
Cleaning up soft-deleted documents older than 30 days...
Permanently deleted 0 document(s).
```

## DAG Implementation

```python
from datetime import datetime, timedelta

from airflow.models import Variable
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
    dag_id='documcp__soft_deleted_cleanup',
    default_args=default_args,
    description='Weekly soft-deleted document cleanup for DocuMCP',
    schedule='0 3 * * 0',
    start_date=datetime(2025, 12, 1),
    catchup=False,
    tags=['documcp', 'cleanup', 'maintenance', 'weekly', 'knowledge'],
)
def documcp_soft_deleted_cleanup_dag():

    @task()
    def run_soft_deleted_cleanup():
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        retention_days = Variable.get("documcp_soft_delete_retention_days", default_var="30")

        docker_exec_artisan(
            command='documcp:cleanup-soft-deleted',
            args=[f'--retention={retention_days}'],
            timeout=300,
        )

    run_soft_deleted_cleanup()

documcp_soft_deleted_cleanup_dag()
```

## Trace Context Propagation

This DAG uses `docker_exec_artisan()` from `common.docker`, which automatically
propagates W3C Trace Context (`TRACEPARENT`) to DocuMCP via environment variable.

This enables distributed tracing in Grafana:
- **Airflow DAG span** -> docker exec -> **DocuMCP artisan command spans**
- Traces visible in Tempo, correlated with DocuMCP logs in Loki via `trace_id`

To disable trace propagation for debugging:
```python
docker_exec_artisan(command='documcp:cleanup-soft-deleted', args=['--retention=30'], propagate_trace=False)
```

## Notifications

### Success
- Count of documents permanently deleted
- Retention period used
- Completion timestamp

### Failure
- Error message from command
- Suggests checking storage permissions or database connectivity

## Configuration

### Airflow Variable
You can configure the retention period via Airflow Variable:

```python
# In Airflow UI: Admin > Variables
# Key: documcp_soft_delete_retention_days
# Value: 30 (or your preferred retention period)
```

## Data Retention Compliance

This DAG helps with data retention compliance by:

1. **GDPR Right to Erasure**: Ensures deleted data is permanently removed
2. **Audit Trail**: Logs all permanent deletions with timestamps
3. **Configurable Retention**: Allows adjustment based on legal requirements
4. **Grace Period**: 30-day default gives time to recover accidentally deleted documents

### Retention Period Guidelines

| Use Case | Suggested Retention |
|----------|---------------------|
| General documents | 30 days |
| Sensitive data | 7 days |
| Legal/compliance | 90 days |
| Archive policy | 180 days |

## Monitoring

### Metrics to Track
- Documents permanently deleted per run
- Storage space reclaimed
- Run duration

### Alerts
- Large number of deletions (> 100) - may indicate bulk delete issue
- Failure to delete - storage or permission issues

## Troubleshooting

### Documents not being deleted

1. Check soft-deleted documents exist:
   ```bash
   docker exec documcp-app php artisan tinker
   >>> Document::onlyTrashed()->where('deleted_at', '<', now()->subDays(30))->count()
   ```

2. Verify retention period:
   ```bash
   docker exec documcp-app php artisan documcp:cleanup-soft-deleted --dry-run --retention=30
   ```

### Storage permission errors

1. Check storage permissions:
   ```bash
   docker exec documcp-app ls -la storage/app/documents/
   ```

2. Verify the app user can delete files:
   ```bash
   docker exec documcp-app touch storage/app/test && docker exec documcp-app rm storage/app/test
   ```

## Related Commands

```bash
# Preview cleanup
docker exec documcp-app php artisan documcp:cleanup-soft-deleted --dry-run --retention=30

# Check soft-deleted document count
docker exec documcp-app php artisan tinker
>>> Document::onlyTrashed()->count()

# View recently soft-deleted documents
docker exec documcp-app php artisan tinker
>>> Document::onlyTrashed()->latest('deleted_at')->take(10)->get(['id', 'title', 'deleted_at'])

# Restore a soft-deleted document (before cleanup)
docker exec documcp-app php artisan tinker
>>> Document::onlyTrashed()->find('uuid-here')->restore()
```

## Safety Considerations

1. **No Undo**: Permanent deletion cannot be undone. Ensure backups are current.
2. **File Deletion**: Associated files are deleted from storage.
3. **Index Cleanup**: Meilisearch entries are removed.
4. **Cascade**: Related records (tags, metadata) are also deleted.

Consider running a dry-run before production deployments to understand the impact:

```bash
docker exec documcp-app php artisan documcp:cleanup-soft-deleted --dry-run --retention=30
```
