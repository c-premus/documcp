# DAG: documcp__oauth_cleanup

## Purpose

Prunes expired OAuth tokens from DocuMCP to maintain database health and security. Removes expired authorization codes, access tokens, refresh tokens, and old revoked tokens.

## Schedule

- **Cron:** `0 2 * * *`
- **Frequency:** Daily at 2:00 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan oauth:prune
```

## What It Does

1. Removes expired authorization codes (>10 minutes old)
2. Removes expired access tokens (>1 hour old by default)
3. Removes expired refresh tokens (>30 days old by default)
4. Removes revoked tokens older than 7 days
5. Logs cleanup statistics
6. Sends completion notification via Apprise

**Note:** Token expiration times are configurable via `OAUTH_ACCESS_TOKEN_LIFETIME` and `OAUTH_REFRESH_TOKEN_LIFETIME` environment variables.

## Expected Output

### Success (Exit 0)
```
Pruning expired OAuth tokens...
  ✓ Pruned 12 expired authorization codes
  ✓ Pruned 45 expired access tokens
  ✓ Pruned 8 expired refresh tokens
  ✓ Pruned 3 revoked tokens

OAuth cleanup completed: 68 tokens removed.
```

### No Tokens to Prune (Exit 0)
```
Pruning expired OAuth tokens...
  ✓ No expired authorization codes
  ✓ No expired access tokens
  ✓ No expired refresh tokens
  ✓ No revoked tokens to prune

OAuth cleanup completed: 0 tokens removed.
```

### Failure (Exit 1)
```
Pruning expired OAuth tokens...
  ✗ Database connection failed

Error: Could not connect to database server.
```

## Shell Script

**Location:** `/opt/airflow/scripts/documcp/oauth-cleanup.sh`

```bash
#!/bin/bash
set -e

# Pass trace context if available
export TRACEPARENT="${TRACEPARENT:-}"
export TRACE_ID="${TRACE_ID:-}"

docker exec documcp-app php artisan oauth:prune
```

## DAG Implementation

```python
"""
DocuMCP OAuth Token Cleanup DAG

Prunes expired OAuth tokens from DocuMCP to maintain database health and security.
This task is managed by Airflow instead of Laravel's built-in scheduler to centralize
all scheduled maintenance tasks with better monitoring and error handling.

Schedule: Daily at 2 AM UTC
Duration: < 1 minute (database cleanup operations)

Workflow:
  1. Execute oauth:prune command in documcp-app container
  2. Remove expired authorization codes (>10 minutes old)
  3. Remove expired access tokens (>1 hour old by default)
  4. Remove expired refresh tokens (>30 days old by default)
  5. Remove revoked tokens older than 7 days
  6. Send notification with completion status

Connections Required:
  - apprise (notification service)
  - documcp-app container (via Docker socket)

Author: Airflow + Claude Code
"""

from datetime import datetime, timedelta
import logging

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
    dag_id='documcp__oauth_cleanup',
    default_args=default_args,
    description='OAuth token cleanup for DocuMCP (daily)',
    schedule='0 2 * * *',  # Daily at 2 AM UTC
    start_date=datetime(2025, 11, 24),
    catchup=False,
    tags=['knowledge', 'documcp', 'oauth', 'maintenance'],
)
def documcp_oauth_cleanup_dag():
    """
    Daily OAuth token pruning workflow.

    Executes Laravel's oauth:prune command to remove expired and revoked tokens,
    preventing database bloat and maintaining optimal performance.
    """

    @task()
    def run_oauth_cleanup():
        """
        Execute oauth:prune command in documcp-app container.

        The command removes:
        - Expired authorization codes (>10 minutes old)
        - Expired access tokens (configurable, default >1 hour)
        - Expired refresh tokens (configurable, default >30 days)
        - Revoked tokens older than 7 days

        Returns:
            dict: Cleanup completion status and timestamp
        """
        logger.info("Starting OAuth token cleanup for DocuMCP")

        # Verify container is running
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        # Execute OAuth prune command
        docker_exec_artisan(command='oauth:prune', timeout=300)

        logger.info("OAuth token cleanup completed successfully")

        return {
            'status': 'success',
            'timestamp': datetime.now().isoformat(),
        }

    @task()
    def send_completion_notification(stats: dict):
        """
        Send completion notification via Apprise.

        Args:
            stats: Cleanup statistics and status

        Returns:
            str: Notification status
        """
        message = (
            "DocuMCP OAuth token cleanup completed successfully.\n\n"
            "Expired authorization codes pruned\n"
            "Expired access tokens pruned\n"
            "Expired refresh tokens pruned\n"
            "Old revoked tokens pruned\n\n"
            f"Completed at: {stats['timestamp']}"
        )

        return send_success_notification(
            message=message,
            title="DocuMCP OAuth Cleanup Complete",
        )

    # Define task dependencies
    cleanup_stats = run_oauth_cleanup()
    send_completion_notification(cleanup_stats)


# Instantiate the DAG
documcp_oauth_cleanup_dag()
```

## Notifications

### Success
- Confirmation that all token types were pruned
- Completion timestamp

### Failure
- Error message from command
- Database connectivity issues
- Suggests checking database connection

## Alerting

Consider adding alerts in Grafana when:
- OAuth cleanup fails 2+ consecutive times
- Large number of tokens pruned (may indicate security issue)
- Cleanup duration exceeds 1 minute

## Related Commands

```bash
# Manual OAuth cleanup
docker exec documcp-app php artisan oauth:prune

# View OAuth statistics
docker exec documcp-app php artisan tinker
>>> OAuthAccessToken::where('expires_at', '<', now())->count()

# Check token expiration settings
docker exec documcp-app php artisan env | grep OAUTH
```
