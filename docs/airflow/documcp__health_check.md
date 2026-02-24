# DAG: documcp__health_check

## Purpose

Monitor the health of external services that DocuMCP depends on, including Kiwix-Serve instances for ZIM archive access.

## Schedule

- **Cron:** `0 3 * * *`
- **Frequency:** Daily at 3:00 AM UTC
- **Timezone:** UTC

## Command

```bash
docker exec documcp-app php artisan services:health-check
```

## What It Checks

The command queries each enabled external service configured in DocuMCP:

1. **Kiwix-Serve instances**
   - HTTP connectivity to the service URL
   - Response time measurement
   - Catalog availability

2. **Health status recorded**
   - `healthy`: Service responding normally
   - `unhealthy`: Service unreachable or returning errors
   - `degraded`: Service slow but functional

## Expected Output

### Success (Exit 0)
```
Checking external services...
✓ kiwix-serve-1: healthy (45ms)
✓ kiwix-serve-2: healthy (52ms)

All 2 services are healthy.
```

### Failure (Exit 1)
```
Checking external services...
✓ kiwix-serve-1: healthy (45ms)
✗ kiwix-serve-2: unhealthy - Connection refused

1 of 2 services are unhealthy.
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
    dag_id='documcp__health_check',
    default_args=default_args,
    description='Daily external service health check for DocuMCP',
    schedule='0 3 * * *',
    start_date=datetime(2025, 12, 1),
    catchup=False,
    tags=['documcp', 'health', 'maintenance', 'daily', 'knowledge'],
)
def documcp_health_check_dag():

    @task()
    def run_health_check():
        if not container_is_running('documcp-app'):
            raise RuntimeError("documcp-app container is not running")

        docker_exec_artisan(command='services:health-check', timeout=120)

    run_health_check()

documcp_health_check_dag()
```

## Trace Context Propagation

This DAG uses `docker_exec_artisan()` from `common.docker`, which automatically
propagates W3C Trace Context (`TRACEPARENT`) to DocuMCP via environment variable.

This enables distributed tracing in Grafana:
- **Airflow DAG span** → docker exec → **DocuMCP artisan command spans**
- Traces visible in Tempo, correlated with DocuMCP logs in Loki via `trace_id`

To disable trace propagation for debugging:
```python
docker_exec_artisan(command='services:health-check', propagate_trace=False)
```

## Notifications

### Success
- Brief summary of services checked
- All services healthy status

### Failure
- List of unhealthy services
- Error details for each failing service
- Suggests manual investigation

## Alerting

Consider adding alerts in Grafana when:
- Health check fails 2+ consecutive times
- Service response time exceeds 5 seconds
- Service status changes from healthy to unhealthy

## Related Commands

```bash
# Manual health check
docker exec documcp-app php artisan services:health-check

# View external service configuration
docker exec documcp-app php artisan tinker
>>> ExternalService::enabled()->get()
```
