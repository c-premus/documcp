# Health Checks

DocuMCP v1.4.0

## Overview

DocuMCP provides two health check endpoints for monitoring application health and dependencies:

1. **Basic Health Check** (`/api/health`) - Fast, minimal check for uptime monitoring
2. **Deep Health Check** (`/api/health/deep`) - Comprehensive check of all dependencies

## Endpoints

### Basic Health Check

**Endpoint**: `GET /api/health`
**Authentication**: None (public)
**Purpose**: Fast health check for load balancers, uptime monitors, and Kubernetes liveness probes

**Response** (200 OK):
```json
{
  "status": "healthy",
  "timestamp": "2025-11-22T10:30:45.000000Z",
  "application": "DocuMCP",
  "environment": "production"
}
```

**Use Cases**:
- Load balancer health checks (Traefik, HAProxy, nginx)
- Uptime monitoring (UptimeRobot, Pingdom)
- Kubernetes liveness probes
- Basic alerting

**Performance**: < 5ms response time

### Deep Health Check

**Endpoint**: `GET /api/health/deep`
**Authentication**: Bearer token via `INTERNAL_API_TOKEN` env var (optional)
**Purpose**: Comprehensive dependency health check for detailed monitoring

When `INTERNAL_API_TOKEN` is set, requests must include `Authorization: Bearer <token>`.
When not configured, the endpoint remains publicly accessible (backward compatible).
The basic health check (`/api/health`) is always public regardless of this setting.

**Components Checked**:

| Component | Description |
|-----------|-------------|
| `database` | PostgreSQL connectivity and query execution |
| `redis` | Redis connectivity (ping test) |
| `meilisearch` | Meilisearch health endpoint |
| `filesystem` | Storage directory write/read permissions |
| `queue` | Horizon supervisor status (if installed) |
| `oidc` | OIDC discovery endpoint and configuration |

External services (Confluence, Kiwix) are not included in health checks. They are optional integrations and their availability does not affect core application health.

**Response** (200 OK - All Healthy):
```json
{
  "status": "healthy",
  "timestamp": "2025-11-22T10:30:45.000000Z",
  "application": "DocuMCP",
  "environment": "production",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 5.2,
      "connection": "pgsql"
    },
    "redis": {
      "status": "healthy",
      "latency_ms": 1.8
    },
    "meilisearch": {
      "status": "healthy",
      "latency_ms": 12.4
    },
    "filesystem": {
      "status": "healthy",
      "writable": true
    },
    "queue": {
      "status": "healthy",
      "driver": "horizon",
      "active_supervisors": 1
    },
    "oidc": {
      "status": "healthy",
      "latency_ms": 45.3,
      "issuer": "https://auth.example.com"
    }
  }
}
```

**Response** (503 Service Unavailable - Degraded):
```json
{
  "status": "degraded",
  "timestamp": "2025-11-22T10:30:45.000000Z",
  "application": "DocuMCP",
  "environment": "production",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 5.2,
      "connection": "pgsql"
    },
    "redis": {
      "status": "unhealthy",
      "error": "Connection refused"
    },
    "meilisearch": {
      "status": "unhealthy",
      "error": "Connection timeout"
    },
    "filesystem": {
      "status": "healthy",
      "writable": true
    },
    "queue": {
      "status": "degraded",
      "driver": "horizon",
      "active_supervisors": 0
    },
    "oidc": {
      "status": "degraded",
      "error": "OIDC provider URL not configured"
    }
  }
}
```

**Use Cases**:
- Kubernetes readiness probes
- Detailed monitoring dashboards
- Pre-deployment checks
- Post-deployment validation
- CI/CD integration

**Performance**: 50-200ms response time (depends on network latency to dependencies)

## Integration Examples

### Traefik Health Check

Configure Traefik to use the basic health check:

```yaml
# docker-compose.yml
services:
  nginx:
    labels:
      - "traefik.http.services.documcp.loadbalancer.healthcheck.path=/api/health"
      - "traefik.http.services.documcp.loadbalancer.healthcheck.interval=10s"
      - "traefik.http.services.documcp.loadbalancer.healthcheck.timeout=3s"
```

### Kubernetes Probes

**Liveness Probe** (basic check):
```yaml
# kubernetes/deployment.yaml
livenessProbe:
  httpGet:
    path: /api/health
    port: 80
    scheme: HTTP
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 3
```

**Readiness Probe** (deep check):
```yaml
readinessProbe:
  httpGet:
    path: /api/health/deep
    port: 80
    scheme: HTTP
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 5
  failureThreshold: 2
```

### Uptime Monitoring (UptimeRobot)

1. **Create HTTP(s) Monitor**
2. **URL**: `https://documcp.your-domain.com/api/health`
3. **Interval**: 5 minutes
4. **Alert Contacts**: Email, Slack, etc.

### Prometheus Monitoring

**Blackbox Exporter** configuration:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'documcp_health'
    metrics_path: /probe
    params:
      module: [http_2xx]
    static_configs:
      - targets:
          - https://documcp.your-domain.com/api/health
          - https://documcp.your-domain.com/api/health/deep
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: blackbox-exporter:9115
```

**PromQL Alert Rules**:

```yaml
# alerts.yml
groups:
  - name: documcp_health
    interval: 30s
    rules:
      - alert: DocuMCPDown
        expr: probe_success{job="documcp_health", instance=~".*api/health"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "DocuMCP is down"
          description: "DocuMCP health check failed for {{ $labels.instance }}"

      - alert: DocuMCPDegraded
        expr: probe_success{job="documcp_health", instance=~".*api/health/deep"} == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "DocuMCP is degraded"
          description: "DocuMCP deep health check failed for {{ $labels.instance }}"

      - alert: DocuMCPHighLatency
        expr: probe_duration_seconds{job="documcp_health"} > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "DocuMCP health check latency is high"
          description: "Health check latency is {{ $value }}s"
```

### Grafana Dashboard

**Health Status Panel**:
```promql
# Gauge visualization (0 = unhealthy, 1 = healthy)
probe_success{job="documcp_health"}
```

**Response Time Panel**:
```promql
# Time series visualization
probe_duration_seconds{job="documcp_health"}
```

**Availability Panel** (last 24h):
```promql
# Stat panel
avg_over_time(probe_success{job="documcp_health"}[24h]) * 100
```

### CI/CD Integration

**Forgejo Actions**:

```yaml
# .forgejo/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to production
        run: |
          # Your deployment steps here
          ...

      - name: Wait for deployment
        run: sleep 30

      - name: Verify health
        run: |
          HEALTH_URL="https://documcp.your-domain.com/api/health/deep"
          RESPONSE=$(curl -s -w "%{http_code}" -o /tmp/health.json "$HEALTH_URL")

          if [ "$RESPONSE" -ne 200 ]; then
            echo "Health check failed with status $RESPONSE"
            cat /tmp/health.json
            exit 1
          fi

          STATUS=$(jq -r '.status' /tmp/health.json)
          if [ "$STATUS" != "healthy" ]; then
            echo "Application is $STATUS"
            cat /tmp/health.json
            exit 1
          fi

          echo "Deployment successful, all health checks passed"
```

**GitLab CI**:

```yaml
# .gitlab-ci.yml
deploy:
  stage: deploy
  script:
    - # Your deployment commands
  after_script:
    - |
      HEALTH_URL="https://documcp.your-domain.com/api/health/deep"
      for i in {1..5}; do
        HTTP_CODE=$(curl -s -o /tmp/health.json -w "%{http_code}" "$HEALTH_URL")
        if [ "$HTTP_CODE" -eq 200 ]; then
          echo "Health check passed"
          exit 0
        fi
        echo "Health check attempt $i failed (HTTP $HTTP_CODE), retrying..."
        sleep 10
      done
      echo "Health check failed after 5 attempts"
      exit 1
```

## Troubleshooting

### Health Check Returns 503

**Symptoms**: Deep health check returns `503 Service Unavailable`

**Diagnosis**:
```bash
# Check which component is unhealthy
curl -s https://documcp.your-domain.com/api/health/deep | jq '.checks'
```

**Common Causes**:

**1. Database Unhealthy**:
```json
"database": {
  "status": "unhealthy",
  "error": "SQLSTATE[08006] [7] connection refused"
}
```

**Solution**:
```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Check database credentials in .env
grep DB_ .env

# Test database connection
docker exec documcp_app php artisan tinker
>>> DB::connection()->getPdo();
```

**2. Redis Unhealthy**:
```json
"redis": {
  "status": "unhealthy",
  "error": "Connection refused"
}
```

**Solution**:
```bash
# Check Redis is running
docker ps | grep redis

# Test Redis connection
docker exec documcp_app redis-cli -h redis ping
# Expected: PONG

# Check Redis password in .env
grep REDIS_PASSWORD .env
```

**3. Meilisearch Unhealthy**:
```json
"meilisearch": {
  "status": "unhealthy",
  "error": "Connection timeout"
}
```

**Solution**:
```bash
# Check Meilisearch is running
docker ps | grep meilisearch

# Test Meilisearch connection
curl http://meilisearch:7700/health

# Verify Meilisearch key in .env
grep MEILISEARCH_KEY .env
```

**4. Filesystem Not Writable**:
```json
"filesystem": {
  "status": "unhealthy",
  "writable": false,
  "error": "Permission denied"
}
```

**Solution**:
```bash
# Fix permissions
docker exec documcp_app chown -R www-data:www-data storage/
docker exec documcp_app chmod -R 775 storage/
```

**5. Queue Degraded**:
```json
"queue": {
  "status": "degraded",
  "driver": "horizon",
  "active_supervisors": 0
}
```

**Solution**:
```bash
# Restart Horizon
docker exec documcp_app php artisan horizon:terminate

# Check Horizon status
docker exec documcp_app php artisan horizon:status

# View Horizon logs
docker logs documcp_queue -f
```

**6. OIDC Provider Unhealthy**:
```json
"oidc": {
  "status": "unhealthy",
  "error": "cURL error 28: Connection timed out"
}
```

**Solution**:
```bash
# Verify OIDC provider URL in .env
grep OIDC .env

# Test OIDC discovery endpoint
curl -s https://your-oidc-provider/.well-known/openid-configuration | jq .

# Check network connectivity to OIDC provider
docker exec documcp_app curl -I https://your-oidc-provider/

# If OIDC is optional for your deployment, this can be ignored
# The application will still function but authentication may fail
```

### High Latency

**Symptoms**: Health check response time > 500ms

**Diagnosis**:
```bash
# Measure response time
time curl https://documcp.your-domain.com/api/health/deep
```

**Common Causes**:
- Network latency to dependencies (database, Redis, Meilisearch)
- Slow database queries
- Resource contention (CPU/memory)

**Solution**:
1. Check individual component latencies in health check response
2. Optimize slow components:
   - Database: Add indexes, connection pooling
   - Redis: Increase memory, check persistence settings
   - Meilisearch: Increase resources, optimize indexes

### False Positives

**Symptoms**: Health check reports healthy but application is not functioning

**Diagnosis**:
```bash
# Test actual functionality
curl -H "Authorization: Bearer $TOKEN" https://documcp.your-domain.com/api/documents
```

**Common Causes**:
- Health check only tests connectivity, not full functionality
- Permission issues not caught by health check
- Specific feature failures

**Solution**:
- Implement feature-specific health checks
- Use synthetic monitoring for end-to-end tests
- Monitor application metrics (Prometheus) alongside health checks

## Best Practices

### Load Balancer Configuration

- **Use basic check** (`/api/health`) for load balancer health checks
- **Interval**: 10-30 seconds (avoid too frequent checks)
- **Timeout**: 3-5 seconds
- **Failure Threshold**: 2-3 consecutive failures before marking unhealthy

### Monitoring Strategy

**Tier 1** (Always):
- Basic health check every 5 minutes (UptimeRobot, Pingdom)
- Basic health check every 10 seconds (Traefik load balancer)

**Tier 2** (Recommended):
- Deep health check every 1-5 minutes (Prometheus Blackbox Exporter)
- Alert on degraded status (5+ minute threshold)

**Tier 3** (Advanced):
- Synthetic end-to-end tests (document upload, search)
- Business metrics monitoring (upload rate, search latency)

### Alert Tuning

**Critical Alerts** (immediate response):
- Basic health check failure > 2 minutes
- All dependencies unhealthy
- Application completely down

**Warning Alerts** (investigate within hours):
- Deep health check degraded > 5 minutes
- Single dependency unhealthy
- High latency (> 500ms)
- Queue workers not running

### Security Considerations

- Health check endpoints are public (no authentication required)
- Error messages do not expose sensitive information
- Use deep check sparingly (more resource intensive)
- Rate limit health checks if abused (add to web.php if needed)

## References

- [Kubernetes Liveness and Readiness Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Prometheus Blackbox Exporter](https://github.com/prometheus/blackbox_exporter)
- [Traefik Health Checks](https://doc.traefik.io/traefik/routing/services/#health-check)
- [12-Factor App: Admin Processes](https://12factor.net/admin-processes)
