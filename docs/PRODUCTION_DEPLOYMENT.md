# Production Deployment Guide

**DocuMCP v1.4.0** | PHP 8.5 | Laravel 12

## Overview

This guide covers deploying DocuMCP to a production environment with security hardening, monitoring, and high availability configuration.

**Before you begin**, ensure you have:
- Server with minimum 4GB RAM, 2 CPU cores, 50GB storage
- Domain name with DNS configured
- OIDC provider (Authentik, Keycloak, Auth0, Google, etc.)
- Basic Linux and Docker knowledge

## Architecture Overview

```
Internet → Traefik (SSL/TLS) → nginx → PHP-FPM → Laravel
                                      ↓
                    PostgreSQL, Redis, Meilisearch
                                      ↓
               Prometheus ← Metrics ← Application
                    ↓
                 Grafana → Dashboards
                    ↓
        Loki ← Grafana Alloy ← Application Logs
```

## Prerequisites

### Server Requirements

**Minimum**:
- CPU: 2 cores
- RAM: 4GB
- Storage: 50GB SSD
- OS: Ubuntu 22.04 LTS or Debian 12

**Recommended**:
- CPU: 4 cores
- RAM: 8GB
- Storage: 100GB SSD (NVMe preferred)
- OS: Ubuntu 22.04 LTS

### Software Requirements

```bash
# Update system
sudo apt-get update && sudo apt-get upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo apt-get install docker-compose-plugin -y

# Install utilities
sudo apt-get install git curl wget htop net-tools -y
```

### Network Requirements

- **Firewall**: Allow ports 80 (HTTP), 443 (HTTPS), 22 (SSH)
- **DNS**: Point domain to server IP (A record)
- **SSL Certificate**: Let's Encrypt via Traefik (automatic)

## Step 1: Clone Repository

```bash
# Clone repository
cd /opt
sudo git clone https://github.com/yourusername/DocuMCP.git
cd DocuMCP

# Set permissions
sudo chown -R $USER:$USER /opt/DocuMCP
```

## Step 2: Environment Configuration

### Copy Production Environment File

```bash
cp .env.production.example .env
```

### Configure Critical Settings

Edit `.env` with production values:

```bash
# Application
APP_NAME=DocuMCP
APP_ENV=production
APP_DEBUG=false
APP_URL=https://documcp.your-domain.com

# Generate secure app key
php artisan key:generate

# Database (strong password!)
DB_CONNECTION=pgsql
DB_HOST=postgres
DB_PORT=5432
DB_DATABASE=documcp
DB_USERNAME=documcp
DB_PASSWORD=$(openssl rand -base64 32)

# Redis (strong password!)
REDIS_PASSWORD=$(openssl rand -base64 32)

# Meilisearch (master key)
MEILISEARCH_KEY=$(openssl rand -base64 32)

# OIDC Authentication (configure with your provider)
OIDC_PROVIDER_URL=https://your-oidc-provider.com
OIDC_CLIENT_ID=your-client-id
OIDC_CLIENT_SECRET=your-client-secret
OIDC_REDIRECT_URI=https://documcp.your-domain.com/auth/oidc/callback

# Octane / Reverse Proxy
OCTANE_HTTPS=true                 # Behind TLS-terminating proxy (Traefik)
TRUSTED_PROXIES=172.20.0.0/16    # Docker network CIDR (never use *)

# Session Security
SESSION_ENCRYPT=true
SESSION_SECURE_COOKIE=true   # Works because OCTANE_HTTPS=true makes Laravel see HTTPS
SESSION_HTTP_ONLY=true       # Prevent JavaScript access (XSS protection)
SESSION_SAME_SITE=lax        # CSRF protection
BCRYPT_ROUNDS=14

# OAuth 2.1 Security (PKCE mandatory, S256 only)
OAUTH_PKCE_REQUIRED=true
OAUTH_CLIENT_REGISTRATION_ENABLED=true
OAUTH_CLIENT_REGISTRATION_AUTH_REQUIRED=false  # Rate-limited: 10/hr, 50/day

# Device Authorization Grant (RFC 8628) - for CLI tools without callbacks
OAUTH_DEVICE_CODE_LIFETIME=900      # 15 minutes
OAUTH_DEVICE_POLLING_INTERVAL=5     # 5 seconds

# Sentry Error Tracking
SENTRY_LARAVEL_DSN=https://your-sentry-dsn@sentry.io/project

# Apprise Notifications
APPRISE_WEBHOOK_URL=http://apprise:8000/notify/apprise?tag=documcp

# Git Templates (project scaffolding from Git repositories)
GIT_TEMPLATES_ENABLED=true
GIT_TEMPLATES_MAX_FILE_SIZE=1048576       # 1MB per file
GIT_TEMPLATES_MAX_TOTAL_SIZE=10485760     # 10MB total per template

# Confluence Integration (optional)
CONFLUENCE_ENABLED=true
CONFLUENCE_URL=https://your-domain.atlassian.net/wiki
CONFLUENCE_EMAIL=api@your-domain.com
CONFLUENCE_API_TOKEN=your-confluence-api-token

# ZIM Archive Integration (optional - requires Kiwix server)
ZIM_ENABLED=true
KIWIX_SERVE_URL=http://kiwix:8080
```

### Verify Configuration

```bash
# Check environment file
cat .env | grep -E "^(APP|DB|REDIS|MEILISEARCH|OIDC)" | grep -v PASSWORD

# Ensure no CHANGE_ME values remain
grep CHANGE_ME .env
# Should return no results
```

## Step 3: Traefik Configuration

### Update docker-compose.yml

Ensure Traefik labels are configured:

```yaml
nginx:
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.documcp.rule=Host(`documcp.your-domain.com`)"
    - "traefik.http.routers.documcp.entrypoints=websecure"
    - "traefik.http.routers.documcp.tls.certresolver=letsencrypt"
```

### Configure Traefik (if not already running)

**Static Configuration** (`traefik.yml`):
```yaml
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"

certificatesResolvers:
  letsencrypt:
    acme:
      email: admin@your-domain.com
      storage: /letsencrypt/acme.json
      httpChallenge:
        entryPoint: web

providers:
  docker:
    exposedByDefault: false
```

## Step 4: Docker Deployment

### Build and Start Services

```bash
# Build application image
docker-compose build

# Start all services
docker-compose up -d

# Verify containers are running
docker-compose ps
```

Expected output:
```
NAME                 IMAGE                    STATUS
documcp_postgres     postgres:18-alpine       Up
documcp_redis        redis:8-alpine           Up
documcp_meilisearch  meilisearch:v1.33        Up
documcp_nginx        nginx:alpine             Up
documcp_app          documcp_app              Up
documcp_queue        documcp_app              Up
```

### Run Database Migrations

```bash
docker-compose exec app php artisan migrate --force
```

### Configure Search Index

```bash
docker-compose exec app php artisan meilisearch:configure
```

### Create Admin User

```bash
docker-compose exec app php artisan tinker
```

In tinker:
```php
$user = User::create([
    'name' => 'Admin',
    'email' => 'admin@your-domain.com',
    'password' => Hash::make('STRONG_PASSWORD_HERE'),
    'email_verified_at' => now(),
    'is_admin' => true,
]);
```

## Step 5: Monitoring Setup

### Verify Prometheus Metrics

```bash
curl http://localhost:8000/metrics
# Should return Prometheus metrics in text format
```

### Import Grafana Dashboard

1. Navigate to Grafana
2. Go to **Dashboards** → **Import**
3. Upload `grafana/dist/documcp.json` (generate with `cd grafana && npm run generate`)
4. Select Prometheus, Tempo, and Loki data sources
5. Click **Import**

### Configure Grafana Alloy

See `docs/GRAFANA_ALLOY_LOGGING.md` for detailed configuration.

**Quick Start**:
```hcl
// /etc/alloy/config.alloy
discovery.docker "documcp" {
  host = "unix:///var/run/docker.sock"
  filter {
    name = "label"
    values = ["logging=alloy"]
  }
}

loki.source.docker "documcp" {
  host = "unix:///var/run/docker.sock"
  targets = discovery.docker.documcp.targets
  forward_to = [loki.write.loki.receiver]
}

loki.write "loki" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
```

Restart Alloy:
```bash
sudo systemctl restart alloy
```

## Step 6: Post-Deployment Verification

### Health Checks

```bash
# Basic health check
curl https://documcp.your-domain.com/api/health

# Deep health check
curl https://documcp.your-domain.com/api/health/deep | jq

# Expected:
# {
#   "status": "healthy",
#   "checks": {
#     "database": {"status": "healthy"},
#     "redis": {"status": "healthy"},
#     "meilisearch": {"status": "healthy"},
#     "filesystem": {"status": "healthy"},
#     "queue": {"status": "healthy"}
#   }
# }
```

### SSL Certificate

```bash
# Verify SSL certificate
curl -vI https://documcp.your-domain.com 2>&1 | grep "SSL certificate"

# Check expiration
openssl s_client -connect documcp.your-domain.com:443 -servername documcp.your-domain.com < /dev/null 2>/dev/null | openssl x509 -noout -dates
```

### Authentication

1. Navigate to https://documcp.your-domain.com/login
2. Click "Sign in with OIDC"
3. Authenticate with your OIDC provider
4. Verify successful login

### Document Upload

1. Navigate to https://documcp.your-domain.com/admin/documents
2. Upload a test PDF document
3. Verify processing completes (status: "indexed")
4. Search for document content

### API Access

```bash
# Get OIDC token (method varies by provider)
TOKEN="your-oidc-token"

# Test MCP endpoint
curl -H "Authorization: Bearer $TOKEN" \
  https://documcp.your-domain.com/documcp \
  -d '{"jsonrpc":"2.0","method":"resources/list","id":1}'

# Expected: List of documents
```

## Step 7: Performance Optimization

### Laravel Optimizations

```bash
# Cache configuration
docker-compose exec app php artisan config:cache

# Cache routes
docker-compose exec app php artisan route:cache

# Cache views
docker-compose exec app php artisan view:cache

# Cache events
docker-compose exec app php artisan event:cache

# Optimize autoloader
docker-compose exec app composer install --optimize-autoloader --no-dev

# Build frontend assets
npm run build
```

### Database Optimizations

```bash
# Analyze tables
docker-compose exec postgres psql -U documcp -d documcp -c "ANALYZE;"

# Vacuum
docker-compose exec postgres psql -U documcp -d documcp -c "VACUUM ANALYZE;"
```

### Enable OPcache (if not already enabled)

Edit `php.ini` in Docker image:
```ini
opcache.enable=1
opcache.memory_consumption=256
opcache.max_accelerated_files=20000
opcache.validate_timestamps=0
```

## Step 8: Security Hardening

### OAuth 2.1 Security

DocuMCP enforces OAuth 2.1 security requirements:

**PKCE (Proof Key for Code Exchange)**:
- Required for all public clients (MCP clients, CLI tools)
- Only S256 method allowed (plain method disabled)
- Configure with `OAUTH_PKCE_REQUIRED=true`

**Device Authorization Grant (RFC 8628)**:
- Alternative to redirect-based OAuth for CLI tools
- Avoids callback URL issues with port forwarding
- Polling interval: 5 seconds minimum
- Code lifetime: 15 minutes

**Client Registration**:
- Dynamic registration enabled by default
- Rate limited: 10 registrations per hour, 50 per day per IP
- Set `OAUTH_CLIENT_REGISTRATION_AUTH_REQUIRED=true` for admin approval

**MIME Type Validation**:
- File uploads validated by MIME type, not extension
- Supported types: PDF, DOCX, XLSX, HTML, Markdown

### Firewall Configuration

```bash
# Install UFW
sudo apt-get install ufw -y

# Default policies
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH (change port if needed)
sudo ufw allow 22/tcp

# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable firewall
sudo ufw enable

# Verify
sudo ufw status
```

### Fail2Ban (Optional but Recommended)

```bash
# Install Fail2Ban
sudo apt-get install fail2ban -y

# Configure for nginx
sudo tee /etc/fail2ban/jail.local <<EOF
[nginx-limit-req]
enabled = true
filter = nginx-limit-req
logpath = /var/log/nginx/error.log
maxretry = 5
findtime = 600
bantime = 3600
EOF

# Restart Fail2Ban
sudo systemctl restart fail2ban
```

### Disable Root SSH

```bash
# Edit SSH config
sudo nano /etc/ssh/sshd_config

# Set:
PermitRootLogin no
PasswordAuthentication no

# Restart SSH
sudo systemctl restart sshd
```

## Step 9: Backup Configuration

### Setup Automated Backups

```bash
# Create backup directory
mkdir -p /mnt/backups

# Install cron jobs
sudo tee /etc/cron.d/documcp <<EOF
0 2 * * * root cd /opt/DocuMCP && ./scripts/backup-database.sh /mnt/backups >> /var/log/documcp-backup.log 2>&1
0 3 * * * root cd /opt/DocuMCP && ./scripts/backup-storage.sh /mnt/backups >> /var/log/documcp-backup.log 2>&1
EOF

sudo chmod 0644 /etc/cron.d/documcp

# Test backup manually
./scripts/backup-database.sh
./scripts/backup-storage.sh

# Verify backups created
ls -lh backups/
```

### Configure Off-Site Backups

See `docs/BACKUP_STRATEGY.md` for detailed instructions on:
- NAS/NFS backups
- rsync to remote server
- S3/cloud storage

## Step 10: Monitoring Alerts

### Prometheus Alerting

Create `prometheus/alerts.yml`:
```yaml
groups:
  - name: documcp
    rules:
      - alert: DocuMCPDown
        expr: up{job="documcp"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "DocuMCP is down"

      - alert: HighErrorRate
        expr: rate(documcp_http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
```

### Apprise Notifications

Apprise is already configured via `.env`:
```env
APPRISE_WEBHOOK_URL=http://apprise:8000/notify/apprise?tag=documcp
```

Critical errors automatically notify via Apprise.

## Troubleshooting

### Application Won't Start

**Check logs**:
```bash
docker-compose logs app
docker-compose logs nginx
```

**Common issues**:
- Database credentials incorrect
- Port conflicts
- Permissions issues

**Fix**:
```bash
# Verify environment
docker-compose exec app php artisan config:clear
docker-compose exec app php artisan cache:clear

# Check database connection
docker-compose exec app php artisan tinker
>>> DB::connection()->getPdo();
```

### SSL Certificate Not Issuing

**Check Traefik logs**:
```bash
docker logs traefik
```

**Common issues**:
- DNS not pointing to server
- Port 80 blocked (needed for ACME challenge)
- Rate limit from Let's Encrypt

**Fix**:
```bash
# Verify DNS
dig documcp.your-domain.com

# Test port 80 accessibility
curl http://documcp.your-domain.com

# Check Traefik certificate
docker exec traefik cat /letsencrypt/acme.json | jq
```

### High Memory Usage

**Check container resources**:
```bash
docker stats
```

**Optimize**:
```bash
# Reduce PHP workers
# Edit docker-compose.yml: pm.max_children = 20 (default: 50)

# Increase swap
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

## Maintenance

### Regular Tasks

**Daily**:
- ✅ Monitor health checks
- ✅ Review error logs
- ✅ Check disk space

**Weekly**:
- ✅ Review Grafana dashboards
- ✅ Verify backups
- ✅ Check for security updates

**Monthly**:
- ✅ Test backup restoration
- ✅ Review access logs
- ✅ Update dependencies

### Updates

```bash
# Pull latest code
git pull origin main

# Rebuild containers
docker-compose build

# Restart services
docker-compose up -d

# Run migrations
docker-compose exec app php artisan migrate --force

# Clear caches
docker-compose exec app php artisan cache:clear
docker-compose exec app php artisan config:cache
```

## Production Checklist

Before going live, verify:

**Security**:
- [ ] APP_DEBUG=false
- [ ] SESSION_ENCRYPT=true
- [ ] SESSION_HTTP_ONLY=true
- [ ] SESSION_SAME_SITE=lax
- [ ] OAUTH_PKCE_REQUIRED=true
- [ ] Strong passwords (32+ characters)
- [ ] SSL/TLS enabled
- [ ] Firewall configured
- [ ] OIDC authentication working
- [ ] Admin user created

**Performance**:
- [ ] Laravel caches enabled (config, route, view)
- [ ] OPcache enabled
- [ ] Database indexed
- [ ] Redis configured

**Monitoring**:
- [ ] Health checks responding
- [ ] Prometheus metrics exposed
- [ ] Grafana dashboards imported
- [ ] Grafana Alloy collecting logs
- [ ] Alerts configured (Apprise)

**Backups**:
- [ ] Automated backups configured
- [ ] Off-site backups configured
- [ ] Backup restoration tested
- [ ] Retention policy set

**Documentation**:
- [ ] OIDC provider documented
- [ ] Admin credentials stored securely
- [ ] Disaster recovery plan documented

## Support

**Documentation**:
- [Traefik Integration](TRAEFIK_INTEGRATION.md)
- [Health Checks](HEALTH_CHECKS.md)
- [Backup Strategy](BACKUP_STRATEGY.md)
- [Prometheus Metrics](PROMETHEUS_METRICS.md)
- [Grafana Alloy Logging](GRAFANA_ALLOY_LOGGING.md)

## Conclusion

Your DocuMCP instance is now deployed. Monitor metrics, logs, and alerts to ensure continued operation.

**Next Steps**:
1. Configure your OIDC provider to add users
2. Import initial documents
3. Configure MCP clients (Claude Desktop, etc.)
4. Monitor dashboards for the first 48 hours
5. Test disaster recovery procedures
