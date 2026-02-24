# DAG: documcp__search_index_verify

## Purpose

Verifies Meilisearch index integrity for DocuMCP by comparing database records with the search index. Identifies orphaned entries (in index but not in database) and missing documents (in database but not in index).

## Schedule

- **Cron:** `0 4 * * *`
- **Frequency:** Daily at 4:00 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan documcp:verify-search-index
```

## What It Does

1. Retrieves document count from PostgreSQL database
2. Retrieves document count from Meilisearch index
3. Compares document IDs between both sources
4. Identifies orphaned entries (in index but not in database)
5. Identifies missing documents (in database but not in index)
6. Reports findings (runs in report-only mode by default)
7. Sends notification with health status

**Note:** This runs in REPORT-ONLY mode. To enable automatic repairs, use the `--repair` flag manually.

## Command Options

```bash
# Report-only mode (default)
php artisan documcp:verify-search-index

# Verify and repair issues automatically
php artisan documcp:verify-search-index --repair

# Verbose output with detailed statistics
php artisan documcp:verify-search-index -v
```

## Expected Output

### Success - Index Healthy (Exit 0)
```
Verifying DocuMCP search index...
  Database documents: 1,234
  Index documents: 1,234

  ✓ No orphaned entries in index
  ✓ No missing documents from index

Index verification completed: Index is healthy.
```

### Success - Issues Found (Exit 0)
```
Verifying DocuMCP search index...
  Database documents: 1,234
  Index documents: 1,238

  ⚠ Found 4 orphaned entries in index
  ⚠ Found 0 missing documents from index

Index verification completed: 4 issues found.
Run with --repair to fix these issues.
```

### Failure (Exit 1)
```
Verifying DocuMCP search index...
  ✗ Error: Could not connect to Meilisearch

Verification failed. Check Meilisearch connectivity.
```

## Shell Script

**Location:** `/opt/airflow/scripts/documcp/search-index-verify.sh`

```bash
#!/bin/bash
set -e

# Pass trace context if available
export TRACEPARENT="${TRACEPARENT:-}"
export TRACE_ID="${TRACE_ID:-}"

docker exec documcp-app php artisan documcp:verify-search-index
```

## DAG Implementation

```python
"""
DocuMCP Search Index Verification DAG

Verifies Meilisearch index integrity for DocuMCP by comparing database records
with the search index. This task is managed by Airflow instead of Laravel's
built-in scheduler to centralize all scheduled maintenance tasks with better
monitoring and error handling.

Schedule: Daily at 4 AM UTC
Duration: < 2 minutes (depends on index size)

Workflow:
  1. Execute verification command in documcp-app container
  2. Compare document count in database vs Meilisearch
  3. Identify orphaned entries (in index but not in database)
  4. Identify missing documents (in database but not in index)
  5. Report findings (runs in report-only mode by default)
  6. Send notification with health status

Note: This runs in REPORT-ONLY mode. To enable automatic repairs, the script
would need to be modified to pass the --repair flag to the artisan command.

Connections Required:
  - apprise (notification service)
  - documcp-app container (via Docker socket)
  - meilisearch (via documcp-app's configuration)

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
    'retry_delay': timedelta(minutes=5),
    'on_failure_callback': send_failure_notification,
}


@dag(
    dag_id='documcp__search_index_verify',
    default_args=default_args,
    description='Search index verification for DocuMCP (daily)',
    schedule='0 4 * * *',  # Daily at 4 AM UTC
    start_date=datetime(2025, 11, 24),
    catchup=False,
    tags=['knowledge', 'documcp', 'search', 'maintenance'],
)
def documcp_search_index_verify_dag():
    """
    Daily search index verification workflow.

    Checks the health of the DocuMCP search index by comparing database
    records with Meilisearch entries, identifying any discrepancies.
    """

    @task()
    def run_search_index_verification():
        """
        Execute search index verification in documcp-app container.

        The command checks:
        - Total document count (database vs index)
        - Orphaned entries (in index but not in database)
        - Missing documents (in database but not in index)

        Runs in report-only mode - does not modify the index.

        Returns:
            dict: Verification results and statistics
        """
        logger.info("Starting search index verification for DocuMCP")

        # Verify container is running
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        # Execute verification command (report-only mode)
        output = docker_exec_artisan(
            command='documcp:verify-search-index',
            timeout=300,
        )

        # Extract statistics from output
        orphaned_count = 0
        missing_count = 0
        has_issues = False

        if output:
            # Look for orphaned entries
            orphaned_match = re.search(r'(\d+)\s+orphaned', output, re.IGNORECASE)
            if orphaned_match:
                orphaned_count = int(orphaned_match.group(1))
                has_issues = True

            # Look for missing documents
            missing_match = re.search(r'(\d+)\s+missing', output, re.IGNORECASE)
            if missing_match:
                missing_count = int(missing_match.group(1))
                has_issues = True

        if has_issues:
            logger.warning(f"Index verification found issues: {orphaned_count} orphaned, {missing_count} missing")
        else:
            logger.info("Search index verification completed - index is healthy")

        return {
            'status': 'success',
            'has_issues': has_issues,
            'orphaned_count': orphaned_count,
            'missing_count': missing_count,
            'timestamp': datetime.now().isoformat(),
        }

    @task()
    def send_completion_notification(stats: dict):
        """
        Send completion notification via Apprise.

        Args:
            stats: Verification statistics

        Returns:
            str: Notification status
        """
        has_issues = stats.get('has_issues', False)
        orphaned = stats.get('orphaned_count', 0)
        missing = stats.get('missing_count', 0)

        if has_issues:
            message = (
                f"DocuMCP search index verification completed - ISSUES FOUND\n\n"
                f"{orphaned} orphaned entries in index\n"
                f"{missing} missing documents from index\n\n"
                f"Run 'documcp:verify-search-index --repair' to fix these issues.\n\n"
                f"Completed at: {stats['timestamp']}"
            )
            # Use warning level for issues
            title = "DocuMCP Index Verification - Issues Found"
        else:
            message = (
                f"DocuMCP search index verification completed.\n\n"
                f"Index is healthy\n"
                f"No orphaned entries\n"
                f"No missing documents\n\n"
                f"Completed at: {stats['timestamp']}"
            )
            title = "DocuMCP Index Verification Complete"

        return send_success_notification(
            message=message,
            title=title,
        )

    # Define task dependencies
    verification_stats = run_search_index_verification()
    send_completion_notification(verification_stats)


# Instantiate the DAG
documcp_search_index_verify_dag()
```

## Notifications

### Success - Healthy
- Confirmation that index is healthy
- No orphaned entries or missing documents
- Completion timestamp

### Success - Issues Found
- Count of orphaned entries
- Count of missing documents
- Suggestion to run with `--repair` flag
- Completion timestamp

### Failure
- Error message from command
- Meilisearch connectivity issues
- Suggests checking Meilisearch connection

## Alerting

Consider adding alerts in Grafana when:
- Verification fails 2+ consecutive times
- Issues found (orphaned or missing documents)
- Verification duration exceeds 2 minutes

## Troubleshooting

### Meilisearch Connection Failed
1. Check Meilisearch container status:
   ```bash
   docker ps | grep meilisearch
   ```

2. Verify Meilisearch is accessible:
   ```bash
   docker exec documcp-app curl -s http://meilisearch:7700/health
   ```

3. Check Meilisearch API key:
   ```bash
   docker exec documcp-app php artisan env | grep MEILI
   ```

### Large Number of Issues Found
1. Check recent document operations:
   ```bash
   docker exec documcp-app tail -100 /var/www/html/storage/logs/laravel.log | grep -i document
   ```

2. Review queue job status:
   ```bash
   docker exec documcp-app php artisan queue:failed
   ```

3. Run repair manually:
   ```bash
   docker exec documcp-app php artisan documcp:verify-search-index --repair
   ```

### Index Inconsistency Persists
1. Force full reindex:
   ```bash
   docker exec documcp-app php artisan documcp:reindex
   ```

2. Clear and rebuild Meilisearch index:
   ```bash
   docker exec documcp-app php artisan meilisearch:configure
   docker exec documcp-app php artisan documcp:reindex
   ```

## Related Commands

```bash
# Report-only verification
docker exec documcp-app php artisan documcp:verify-search-index

# Verify and repair
docker exec documcp-app php artisan documcp:verify-search-index --repair

# Full reindex
docker exec documcp-app php artisan documcp:reindex

# Configure Meilisearch index settings
docker exec documcp-app php artisan meilisearch:configure

# Check Meilisearch statistics
docker exec documcp-app php artisan tinker
>>> MeiliSearch\Client(env('MEILISEARCH_HOST'), env('MEILISEARCH_KEY'))->getStats()
```
