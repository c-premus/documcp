# Traefik Integration Guide

**DocuMCP v1.9+** | PHP 8.5 | Laravel 12 | Octane (RoadRunner)

## Overview

DocuMCP runs Laravel Octane with RoadRunner, serving HTTP directly on port 8000. Traefik sits in front as the reverse proxy for TLS termination, routing, and automatic HTTPS certificate management.

## Prerequisites

- Traefik v2.0+ or v3.0+ running as Docker container
- Docker network for Traefik-enabled services
- Domain name configured with DNS pointing to your server

## Quick Start

### 1. Create Traefik Network

If you haven't already created the Traefik network:

```bash
docker network create traefik_network
```

### 2. Update Docker Compose Labels

The `docker-compose.yml` file already includes Traefik labels. Update the domain:

```yaml
- "traefik.http.routers.documcp.rule=Host(`documcp.your-domain.com`)"
```

Replace `documcp.your-domain.com` with your actual domain.

### 3. Start DocuMCP

```bash
docker-compose up -d
```

Traefik handles:
- Routing traffic from `documcp.your-domain.com` to DocuMCP
- Obtaining Let's Encrypt SSL certificate
- Applying security headers
- Running health checks every 30 seconds

## Configuration Details

### Docker Compose Labels Explained

#### Traefik Enablement
```yaml
- "traefik.enable=true"
```
Enables Traefik routing for this container.

#### HTTP Router
```yaml
- "traefik.http.routers.documcp.rule=Host(`documcp.your-domain.com`)"
- "traefik.http.routers.documcp.entrypoints=websecure"
- "traefik.http.routers.documcp.tls.certresolver=letsencrypt"
```

- **Rule**: Routes requests from `documcp.your-domain.com` to DocuMCP
- **Entrypoints**: Uses `websecure` (HTTPS, typically port 443)
- **TLS**: Automatically obtains Let's Encrypt certificate via `letsencrypt` resolver

#### Load Balancer
```yaml
- "traefik.http.services.documcp.loadbalancer.server.port=8000"
```

Tells Traefik to forward traffic to port 8000 on the app container (RoadRunner serves HTTP directly — no nginx).

#### Health Checks
```yaml
- "traefik.http.services.documcp.loadbalancer.healthcheck.path=/api/health"
- "traefik.http.services.documcp.loadbalancer.healthcheck.interval=30s"
- "traefik.http.services.documcp.loadbalancer.healthcheck.timeout=5s"
```

- **Path**: Traefik checks `/api/health` endpoint
- **Interval**: Health check every 30 seconds
- **Timeout**: Fail if no response within 5 seconds

If health checks fail, Traefik marks the service as unhealthy and stops routing traffic.

#### Security Headers Middleware
```yaml
- "traefik.http.middlewares.documcp-security.headers.stsSeconds=31536000"
- "traefik.http.middlewares.documcp-security.headers.stsIncludeSubdomains=true"
- "traefik.http.middlewares.documcp-security.headers.stsPreload=true"
- "traefik.http.middlewares.documcp-security.headers.forceSTSHeader=true"
- "traefik.http.middlewares.documcp-security.headers.frameDeny=true"
- "traefik.http.middlewares.documcp-security.headers.contentTypeNosniff=true"
- "traefik.http.middlewares.documcp-security.headers.browserXssFilter=true"
- "traefik.http.routers.documcp.middlewares=documcp-security"
```

Applies these security headers:
- **HSTS**: HTTP Strict Transport Security (1 year, includeSubDomains, preload)
- **Frame Denial**: Prevents clickjacking (X-Frame-Options: DENY)
- **Content Type Sniffing**: Prevents MIME sniffing (X-Content-Type-Options: nosniff)
- **XSS Filter**: Enables browser XSS protection (X-XSS-Protection: 1; mode=block)

### HTTPS Configuration

Traefik terminates TLS and forwards plain HTTP to RoadRunner on port 8000. Two settings ensure Laravel generates HTTPS URLs and treats requests as secure:

#### Required Environment Variables

```env
OCTANE_HTTPS=true                    # Octane tells Laravel all requests are HTTPS
APP_URL=https://mcp.999.haus         # URL generation uses HTTPS scheme
TRUSTED_PROXIES=172.20.0.0/16        # Trust only the Docker network CIDR
```

With `OCTANE_HTTPS=true`:
- `request()->secure()` returns `true`
- `url('/')` generates `https://...`
- `asset('css/app.css')` generates `https://...` URLs
- No mixed content warnings in browser

#### Session Cookies

```env
SESSION_SECURE_COOKIE=true   # Safe because OCTANE_HTTPS makes Laravel see HTTPS
SESSION_HTTP_ONLY=true       # Prevent JavaScript access (XSS protection)
SESSION_SAME_SITE=lax        # CSRF protection
```

With `OCTANE_HTTPS=true`, Laravel treats all requests as HTTPS, so secure cookies work correctly even though the internal transport between Traefik and RoadRunner is plain HTTP.

### Trusted Proxies

DocuMCP trusts only the Docker network CIDR (configured in `bootstrap/app.php`):

```php
$middleware->trustProxies(
    at: env('TRUSTED_PROXIES', '172.20.0.0/16'),
    headers: Request::HEADER_X_FORWARDED_FOR |
            Request::HEADER_X_FORWARDED_HOST |
            Request::HEADER_X_FORWARDED_PORT |
            Request::HEADER_X_FORWARDED_PROTO |
            Request::HEADER_X_FORWARDED_PREFIX
);
```

**Why not `'*'`?** RoadRunner serves HTTP directly (no nginx buffer). If proxies were set to wildcard, any client could spoof `X-Forwarded-*` headers. Restricting to the Docker network CIDR (`172.20.0.0/16`) ensures only Traefik can set these headers.

Override via `TRUSTED_PROXIES` env var if your Docker network uses a different CIDR.

Traefik sets these headers:
- `X-Forwarded-For` — Client IP address
- `X-Forwarded-Proto` — Original protocol (https)
- `X-Forwarded-Host` — Original hostname
- `X-Forwarded-Port` — Original port

This ensures Laravel correctly:
- Generates HTTPS URLs (not HTTP)
- Logs real client IPs (not Traefik's IP)
- Detects secure connections via `request()->secure()`

## Traefik Configuration Example

### Basic Traefik Docker Compose

If you don't have Traefik running yet, here's a minimal example:

```yaml
version: '3.8'

services:
  traefik:
    image: traefik:v3.0
    container_name: traefik
    restart: unless-stopped
    ports:
      - "80:80"     # HTTP
      - "443:443"   # HTTPS
      - "8080:8080" # Dashboard (optional)
    command:
      # Enable Docker provider
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"

      # Entrypoints
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"

      # Let's Encrypt
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@your-domain.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"

      # Dashboard (optional)
      - "--api.dashboard=true"
      - "--api.insecure=true"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "./letsencrypt:/letsencrypt"
    networks:
      - traefik_network

networks:
  traefik_network:
    external: true
```

**Important**:
- Replace `admin@your-domain.com` with your email
- Ensure `letsencrypt/acme.json` has permissions: `chmod 600 letsencrypt/acme.json`

### HTTP to HTTPS Redirect

Add automatic HTTP → HTTPS redirect:

```yaml
# In Traefik command:
- "--entrypoints.web.http.redirections.entrypoint.to=websecure"
- "--entrypoints.web.http.redirections.entrypoint.scheme=https"
```

## Verification

### 1. Check Container Connectivity

```bash
# Verify DocuMCP app container is on traefik_network
docker inspect documcp-app | grep -A 10 Networks
```

Should show the traefik network.

### 2. Test HTTP Headers

```bash
# Check security headers are applied
curl -I https://documcp.your-domain.com

# Should see:
# Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
# X-Frame-Options: DENY
# X-Content-Type-Options: nosniff
```

### 3. Verify X-Forwarded Headers

Check Laravel sees correct headers:

```bash
# Add temporary route to dump request info:
# routes/web.php
Route::get('/debug-headers', function () {
    return response()->json([
        'ip' => request()->ip(),
        'scheme' => request()->getScheme(),
        'url' => request()->url(),
        'headers' => request()->headers->all(),
    ]);
});

# Test:
curl https://documcp.your-domain.com/debug-headers
```

You should see:
- `scheme`: `https`
- `url`: `https://documcp.your-domain.com/debug-headers`
- `headers.x-forwarded-proto`: `['https']`
- `headers.x-forwarded-for`: `['your-public-ip']`

### 4. Health Check Verification

```bash
# Check Traefik dashboard or logs
docker logs traefik | grep documcp

# Should see health check requests every 30s:
# "GET /api/health HTTP/1.1" 200
```

## Troubleshooting

### Issue: 404 Not Found

**Cause**: Traefik can't find the service.

**Solution**:
1. Verify app container is running: `docker ps | grep documcp-app`
2. Check container is on traefik network: `docker inspect documcp-app`
3. Verify Traefik labels: `docker inspect documcp-app | grep traefik`
4. Check Traefik logs: `docker logs traefik`

### Issue: 502 Bad Gateway

**Cause**: App container is unhealthy or RoadRunner not responding.

**Solution**:
1. Check health endpoint directly: `docker exec documcp-app curl -s http://localhost:8000/api/health`
2. Check app logs: `docker logs documcp-app`
3. Verify database connectivity: `docker exec documcp-app php artisan db:show`

### Issue: SSL Certificate Not Issued

**Cause**: Let's Encrypt can't verify domain.

**Solution**:
1. Verify DNS points to server: `dig documcp.your-domain.com`
2. Check ports 80/443 are open: `netstat -tuln | grep -E ':80|:443'`
3. Check Traefik logs: `docker logs traefik | grep acme`
4. Verify email in Traefik config is valid

### Issue: Session Not Persisting

**Cause**: `OCTANE_HTTPS` not set, so Laravel sees HTTP and won't send secure cookies.

**Solution**:
```env
# .env
OCTANE_HTTPS=true             # Laravel sees all requests as HTTPS
SESSION_SECURE_COOKIE=true    # Works because OCTANE_HTTPS makes Laravel see HTTPS
SESSION_HTTP_ONLY=true        # Prevent JavaScript access
SESSION_SAME_SITE=lax         # CSRF protection
```

### Issue: OAuth Callbacks Fail

**Cause**: Dynamic ports or port forwarding issues with mcp-remote callbacks.

**Solution**: Use Device Authorization Grant (RFC 8628) instead:
```bash
# Device auth flow avoids callback URL issues
# Users authenticate via browser at verification URL
# CLI polls for token without requiring callback
```

Or use fixed port for mcp-remote:
```bash
claude mcp add documcp -- npx -y mcp-remote https://documcp.your-domain.com/documcp 3334
```

### Issue: CORS Errors from Frontend

**Cause**: Missing CORS configuration.

**Solution**:
1. Configure CORS in `config/cors.php`:
   ```php
   'allowed_origins' => explode(',', env('CORS_ALLOWED_ORIGINS', '*')),
   ```

2. Set in `.env`:
   ```env
   CORS_ALLOWED_ORIGINS=https://documcp.your-domain.com,https://admin.your-domain.com
   ```

## Advanced Configuration

### Multiple Domains

Route multiple domains to DocuMCP:

```yaml
- "traefik.http.routers.documcp.rule=Host(`documcp.com`) || Host(`www.documcp.com`) || Host(`docs.example.com`)"
```

### Rate Limiting

Add Traefik-level rate limiting:

```yaml
- "traefik.http.middlewares.documcp-ratelimit.ratelimit.average=100"
- "traefik.http.middlewares.documcp-ratelimit.ratelimit.burst=50"
- "traefik.http.routers.documcp.middlewares=documcp-security,documcp-ratelimit"
```

- **Average**: 100 requests per second
- **Burst**: Allow 50 request burst

### IP Whitelisting

Restrict access to specific IPs:

```yaml
- "traefik.http.middlewares.documcp-ipwhitelist.ipwhitelist.sourcerange=192.168.1.0/24,10.0.0.0/8"
- "traefik.http.routers.documcp.middlewares=documcp-security,documcp-ipwhitelist"
```

### Custom Certificate

Use custom SSL certificate instead of Let's Encrypt:

```yaml
- "traefik.http.routers.documcp.tls.certresolver="
- "traefik.http.routers.documcp.tls.domains[0].main=documcp.your-domain.com"
- "traefik.http.routers.documcp.tls.domains[0].sans=*.documcp.your-domain.com"
```

Mount certificate in Traefik:
```yaml
# Traefik service
volumes:
  - "/path/to/cert.pem:/certs/cert.pem"
  - "/path/to/key.pem:/certs/key.pem"

command:
  - "--providers.file.directory=/certs"
```

## Security Best Practices

1. **Always use HTTPS in production** - Configure `letsencrypt` resolver
2. **Enable HSTS** - Already configured in security headers
3. **Restrict Traefik dashboard** - Never expose on public internet
4. **Use strong certificate email** - Real email for Let's Encrypt notifications
5. **Monitor certificate renewal** - Let's Encrypt certs expire every 90 days (auto-renewed by Traefik)
6. **Keep Traefik updated** - Security patches released regularly

### Issue: Mixed Content Warnings

**Cause**: `OCTANE_HTTPS` or `APP_URL` not set, so Laravel generates `http://` URLs.

**Solution**:
```env
OCTANE_HTTPS=true
APP_URL=https://mcp.999.haus
```

## Production Deployment Checklist

- [ ] DNS configured and propagated
- [ ] Traefik network created (`172.20.0.0/16` CIDR)
- [ ] Domain updated in docker-compose.yml labels
- [ ] Email configured in Traefik Let's Encrypt resolver
- [ ] Ports 80/443 accessible from internet
- [ ] `APP_URL=https://your-domain.com` in `.env`
- [ ] `OCTANE_HTTPS=true` in `.env`
- [ ] `TRUSTED_PROXIES=172.20.0.0/16` in `.env` (match your Docker network)
- [ ] `SESSION_SECURE_COOKIE=true` in `.env`
- [ ] `SESSION_HTTP_ONLY=true` in `.env`
- [ ] `SESSION_SAME_SITE=lax` in `.env`
- [ ] Health checks responding: `curl https://your-domain.com/api/health`
- [ ] HTTPS working: `curl -I https://your-domain.com`
- [ ] No mixed content: `request()->secure()` returns `true`
- [ ] Security headers present: Check `Strict-Transport-Security` header
- [ ] Certificate valid: Check browser padlock icon

## References

- [Traefik Documentation](https://doc.traefik.io/traefik/)
- [Let's Encrypt](https://letsencrypt.org/)
- [Laravel Behind Proxies](https://laravel.com/docs/12.x/requests#configuring-trusted-proxies)
- [Traefik Docker Provider](https://doc.traefik.io/traefik/providers/docker/)
