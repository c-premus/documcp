# DAG: documcp__git_template_sync

## Purpose

Synchronize Git template repositories by pulling latest changes, extracting files, and updating the Meilisearch index. This keeps template content up-to-date with upstream repositories.

## Schedule

- **Cron:** `0 6 * * *`
- **Frequency:** Daily at 6:00 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan git-template:sync --all
```

## What It Does

1. Iterates through all registered Git template repositories
2. Clones repository to temporary directory (or pulls if already cloned)
3. Extracts template files (skipping binary files and symlinks)
4. Detects essential files (CLAUDE.md, README.md, memory-bank/*)
5. Extracts template variables ({{placeholder}} patterns)
6. Updates README content in Meilisearch index
7. Cleans up temporary clone directory
8. Logs sync statistics per template

**Security:** Symlinks are skipped to prevent path traversal attacks. Binary files are detected and excluded.

## Command Options

```bash
# Sync all registered templates (required for batch sync)
php artisan git-template:sync --all

# Sync a specific template by UUID
php artisan git-template:sync --uuid=550e8400-e29b-41d4-a716-446655440000

# Force re-sync even if recently synced
php artisan git-template:sync --all --force

# Verbose output with per-file details
php artisan git-template:sync --all -v
```

## Expected Output

### Success (Exit 0)
```
Syncing Git templates...
  Template: claude-memory-bank (https://github.com/example/memory-bank)
    ✓ Cloned repository
    ✓ Extracted 12 files (3 essential)
    ✓ Found 4 template variables
    ✓ Updated Meilisearch index

  Template: laravel-starter (https://github.com/example/laravel-starter)
    ✓ Pulled latest changes
    ✓ Extracted 45 files (2 essential)
    ✓ Found 8 template variables
    ✓ Updated Meilisearch index

Sync completed: 2 templates synced, 57 files extracted.
```

### Partial Success (Exit 0 with warnings)
```
Syncing Git templates...
  Template: claude-memory-bank (https://github.com/example/memory-bank)
    ✓ Cloned repository
    ✓ Extracted 12 files (3 essential)
    ✓ Updated Meilisearch index

  Template: private-template (https://github.com/example/private)
    ✗ Clone failed: Authentication required

Sync completed with warnings: 1 of 2 templates synced.
```

### Failure (Exit 1)
```
Syncing Git templates...
  Template: claude-memory-bank (https://github.com/example/memory-bank)
    ✗ Clone failed: Repository not found

No templates synced. All repositories unreachable.
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
    'retry_delay': timedelta(minutes=15),
    'on_failure_callback': send_failure_notification,
}

@dag(
    dag_id='documcp__git_template_sync',
    default_args=default_args,
    description='Daily Git template sync for DocuMCP',
    schedule='0 6 * * *',
    start_date=datetime(2025, 12, 1),
    catchup=False,
    tags=['documcp', 'git-templates', 'sync', 'daily', 'knowledge'],
)
def documcp_git_template_sync_dag():

    @task()
    def run_git_template_sync():
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        docker_exec_artisan(
            command='git-template:sync',
            args=['--all'],
            timeout=1800,
        )

    run_git_template_sync()

documcp_git_template_sync_dag()
```

## Trace Context Propagation

This DAG uses `docker_exec_artisan()` from `common.docker`, which automatically
propagates W3C Trace Context (`TRACEPARENT`) to DocuMCP via environment variable.

This enables distributed tracing in Grafana:
- **Airflow DAG span** → docker exec → **DocuMCP artisan command spans**
- Traces visible in Tempo, correlated with DocuMCP logs in Loki via `trace_id`

To disable trace propagation for debugging:
```python
docker_exec_artisan(command='git-template:sync', args=['--all'], propagate_trace=False)
```

## Notifications

### Success
- Templates synced count
- Total files extracted
- Completion timestamp

### Failure
- Error message from command
- Which template(s) failed to sync
- Suggests checking repository access

## Dependencies

This DAG should run **after** other sync DAGs to spread load:
- `zim:sync` runs at 5:00 AM UTC
- `confluence:sync` runs at 5:30 AM UTC
- `git-template:sync` runs at 6:00 AM UTC

## Monitoring

### Metrics to Track
- Templates synced per run
- Files extracted per template
- Essential files found
- Sync duration per template
- Failed syncs (network, auth issues)

### Alerts
- No templates synced (all repos unreachable)
- Authentication failures (SSH key or token expired)
- Sync taking longer than 20 minutes
- Large increase in file count (unexpected repo changes)

## Troubleshooting

### No templates synced
1. Check Git connectivity from container:
   ```bash
   docker exec documcp-app git ls-remote https://github.com/example/repo
   ```

2. List registered templates:
   ```bash
   docker exec documcp-app php artisan git-template:stats
   ```

3. Check network connectivity:
   ```bash
   docker exec documcp-app curl -I https://github.com
   ```

### Authentication failures
1. For HTTPS repos: Check access token is valid
2. For SSH repos: Verify SSH key is configured
3. For private repos: Ensure credentials are stored in external service config

### Clone timeout
1. Large repositories may timeout - increase clone_timeout in config
2. Consider using shallow clones for large repos
3. Check network bandwidth

### Files not appearing
1. Check file isn't in skip_directories (node_modules, .git, vendor)
2. Verify file isn't binary (detected by extension or MIME type)
3. Ensure file size is under max_file_size limit (default 1MB)

## Security Considerations

- **Symlinks**: Automatically skipped to prevent path traversal
- **Binary files**: Detected and excluded based on extension/MIME type
- **Path validation**: All file paths validated with realpath()
- **Variable keys**: Only alphanumeric + underscore allowed
- **SSRF protection**: Internal IPs blocked for Git URLs

## Related Commands

```bash
# Manual sync all templates
docker exec documcp-app php artisan git-template:sync --all

# Sync specific template
docker exec documcp-app php artisan git-template:sync --uuid=UUID

# View template statistics
docker exec documcp-app php artisan git-template:stats

# Force re-sync all templates
docker exec documcp-app php artisan git-template:sync --all --force
```
