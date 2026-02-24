# DAG: documcp__orphaned_files_cleanup

## Purpose

Removes orphaned files from DocuMCP storage that have no corresponding database records. This prevents storage bloat and maintains consistency between the filesystem and database.

## Schedule

- **Cron:** `0 2 * * 0`
- **Frequency:** Weekly on Sundays at 2:00 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan documcp:cleanup-orphaned-files --force
```

## What It Does

1. Scans all files in the storage directory
2. Compares file paths with database records
3. Identifies files that have no matching database entries
4. Removes orphaned files (with `--force` flag)
5. Reports statistics on files removed
6. Sends completion notification via Apprise

**Orphaned files can occur when:**
- Documents are deleted from database but file deletion fails
- Database transactions are rolled back after file upload
- Manual database cleanup is performed without file cleanup
- Application errors during document processing

## Command Options

```bash
# Dry-run to see what would be deleted
php artisan documcp:cleanup-orphaned-files

# Actually delete orphaned files
php artisan documcp:cleanup-orphaned-files --force

# Verbose output with file details
php artisan documcp:cleanup-orphaned-files --force -v
```

## Expected Output

### Success - Files Removed (Exit 0)
```
Scanning storage for orphaned files...
  Found 5 orphaned file(s)

Removing orphaned files...
  ✓ Removed: documents/abc123.pdf
  ✓ Removed: documents/def456.docx
  ✓ Removed: documents/ghi789.xlsx
  ✓ Removed: temp/upload_001.tmp
  ✓ Removed: temp/upload_002.tmp

Cleanup completed: 5 file(s) removed.
```

### Success - No Orphaned Files (Exit 0)
```
Scanning storage for orphaned files...
  No orphaned files found.

Storage is clean.
```

### Failure (Exit 1)
```
Scanning storage for orphaned files...
  ✗ Error: Permission denied accessing storage directory

Cleanup failed. Check storage permissions.
```

## Shell Script

**Location:** `/opt/airflow/scripts/documcp/orphaned-files-cleanup.sh`

```bash
#!/bin/bash
set -e

# Pass trace context if available
export TRACEPARENT="${TRACEPARENT:-}"
export TRACE_ID="${TRACE_ID:-}"

docker exec documcp-app php artisan documcp:cleanup-orphaned-files --force
```

## DAG Implementation

```python
"""
DocuMCP Orphaned Files Cleanup DAG

Removes orphaned files from DocuMCP storage that have no corresponding database records.
This task is managed by Airflow instead of Laravel's built-in scheduler to centralize
all scheduled maintenance tasks with better monitoring and error handling.

Schedule: Weekly on Sundays at 2 AM UTC
Duration: < 5 minutes (depends on number of orphaned files)

Workflow:
  1. Execute cleanup command in documcp-app container
  2. Scan storage directory for files
  3. Compare with database records
  4. Remove files with no matching database entries
  5. Send notification with statistics (number of files removed)

Orphaned files can occur when:
  - Documents are deleted from database but file deletion fails
  - Database transactions are rolled back after file upload
  - Manual database cleanup is performed without file cleanup

Connections Required:
  - apprise (notification service)
  - documcp-app container (via Docker socket)

Author: Airflow + Claude Code
"""

from datetime import datetime, timedelta
import logging
import re

from airflow.sdk import dag, task

from common.docker import container_is_running, docker_exec_artisan
from common.notifications import send_failure_notification, send_success_notification

logger = logging.getLogger(__name__)

default_args = {
    'owner': 'airflow',
    'depends_on_past': False,
    'email_on_failure': False,
    'email_on_retry': False,
    'retries': 2,
    'retry_delay': timedelta(minutes=10),
    'on_failure_callback': send_failure_notification,
}


@dag(
    dag_id='documcp__orphaned_files_cleanup',
    default_args=default_args,
    description='Orphaned files cleanup for DocuMCP storage (weekly)',
    schedule='0 2 * * 0',  # Sundays at 2 AM UTC
    start_date=datetime(2025, 11, 24),
    catchup=False,
    tags=['knowledge', 'documcp', 'storage', 'maintenance'],
)
def documcp_orphaned_files_cleanup_dag():
    """
    Weekly orphaned files cleanup workflow.

    Identifies and removes files in storage that have no corresponding database
    records, preventing storage bloat and maintaining consistency.
    """

    @task()
    def run_orphaned_files_cleanup():
        """
        Execute cleanup command in documcp-app container.

        The command:
        - Scans all files in storage directory
        - Queries database for matching records
        - Removes files with no database entry
        - Uses --force flag to actually delete files

        Returns:
            dict: Cleanup statistics including number of files removed
        """
        logger.info("Starting orphaned files cleanup for DocuMCP")

        # Verify container is running
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        # Execute cleanup command with --force to actually delete files
        output = docker_exec_artisan(
            command='documcp:cleanup-orphaned-files',
            args=['--force'],
            timeout=600,
        )

        # Extract number of files removed from output
        orphaned_count = 0
        if output:
            # Look for pattern like "removed 5 file(s)" or "5 orphaned file"
            match = re.search(r'(\d+)\s+(?:orphaned )?file', output, re.IGNORECASE)
            if match:
                orphaned_count = int(match.group(1))

        logger.info(f"Orphaned files cleanup completed - removed {orphaned_count} file(s)")

        return {
            'status': 'success',
            'files_removed': orphaned_count,
            'timestamp': datetime.now().isoformat(),
        }

    @task()
    def send_completion_notification(stats: dict):
        """
        Send completion notification via Apprise.

        Args:
            stats: Cleanup statistics

        Returns:
            str: Notification status
        """
        files_removed = stats.get('files_removed', 0)

        if files_removed > 0:
            message = (
                f"DocuMCP orphaned files cleanup completed.\n\n"
                f"Removed {files_removed} orphaned file(s)\n"
                f"Storage consistency maintained\n\n"
                f"Completed at: {stats['timestamp']}"
            )
        else:
            message = (
                f"DocuMCP orphaned files cleanup completed.\n\n"
                f"No orphaned files found\n"
                f"Storage is clean\n\n"
                f"Completed at: {stats['timestamp']}"
            )

        return send_success_notification(
            message=message,
            title="DocuMCP Files Cleanup Complete",
        )

    # Define task dependencies
    cleanup_stats = run_orphaned_files_cleanup()
    send_completion_notification(cleanup_stats)


# Instantiate the DAG
documcp_orphaned_files_cleanup_dag()
```

## Notifications

### Success
- Number of orphaned files removed
- Confirmation that storage is consistent
- Completion timestamp

### Failure
- Error message from command
- Permission or connectivity issues
- Suggests checking storage permissions

## Alerting

Consider adding alerts in Grafana when:
- Cleanup fails 2+ consecutive weeks
- Large number of orphaned files found (may indicate application bug)
- Cleanup duration exceeds 5 minutes

## Troubleshooting

### Permission Denied
1. Check storage directory ownership:
   ```bash
   docker exec documcp-app ls -la /var/www/html/storage/app
   ```

2. Verify www-data user permissions:
   ```bash
   docker exec documcp-app id www-data
   ```

3. Fix permissions if needed:
   ```bash
   docker exec documcp-app chown -R www-data:www-data /var/www/html/storage
   ```

### Large Number of Orphaned Files
1. Check recent application errors:
   ```bash
   docker exec documcp-app tail -100 /var/www/html/storage/logs/laravel.log
   ```

2. Review failed job history:
   ```bash
   docker exec documcp-app php artisan queue:failed
   ```

3. Check for database inconsistencies:
   ```bash
   docker exec documcp-app php artisan tinker
   >>> Document::count()
   ```

## Related Commands

```bash
# Dry-run cleanup (see what would be deleted)
docker exec documcp-app php artisan documcp:cleanup-orphaned-files

# Force cleanup
docker exec documcp-app php artisan documcp:cleanup-orphaned-files --force

# Check storage disk usage
docker exec documcp-app du -sh /var/www/html/storage/app

# List files in storage
docker exec documcp-app find /var/www/html/storage/app/documents -type f | wc -l
```
