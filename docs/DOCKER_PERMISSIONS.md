# Docker Permission Management - DocuMCP

**Date**: 2025-11-12
**Status**: Implemented and Active

---

## Problem Overview

When Docker containers run as **root (UID 0)** or **www-data (UID 82)**, files created inside containers on bind-mounted volumes are owned by those UIDs on the host filesystem. This causes permission issues:

- Host user (chris, UID 1000) cannot delete or modify container-created files
- Requires `sudo` to clean up `vendor/`, `storage/`, etc.
- Breaks development workflow

---

## Solution: User Mapping

We map container users to match your **host UID/GID** so all files created inside containers are owned by your host user.

### How It Works

1. **Build time**: Pass `UID` and `GID` as build arguments
2. **Container setup**: Recreate container users with host UID/GID
3. **Runtime**: Container runs as user with matching UID/GID
4. **Result**: Files created in container = owned by host user

---

## Implementation

### 1. Environment Variables (`.env`)

```bash
# Docker user mapping - matches your host user
UID=1000  # Your user ID (run `id -u` to check)
GID=1000  # Your group ID (run `id -g` to check)
```

**Why**: Docker Compose reads these values and passes them as build arguments.

---

### 2. Production Dockerfile (`docker/Dockerfile`)

```dockerfile
# User mapping for development - match host UID/GID to avoid permission issues
ARG UID=1000
ARG GID=1000

# Recreate www-data user with host UID/GID
RUN deluser --remove-home www-data 2>/dev/null || true \
    && addgroup -g ${GID} www-data \
    && adduser -D -u ${UID} -G www-data www-data

# Set permissions for Laravel storage and cache
RUN mkdir -p /var/www/storage /var/www/bootstrap/cache \
    && chown -R www-data:www-data /var/www

# Switch to www-data user
USER www-data
```

**Key Points**:
- `ARG UID=1000` - Accepts build argument, defaults to 1000
- `deluser --remove-home www-data` - Removes default www-data (UID 82)
- `addgroup -g ${GID}` - Creates group with host GID
- `adduser -D -u ${UID}` - Creates user with host UID
- `USER www-data` - All subsequent commands run as www-data (UID 1000)

---

### 3. DevContainer Dockerfile (`.devcontainer/Dockerfile`)

```dockerfile
# User mapping for devcontainer - match host UID/GID to avoid permission issues
ARG UID=1000
ARG GID=1000

# Create vscode user with host UID/GID
RUN addgroup -g ${GID} vscode \
    && adduser -D -u ${UID} -G vscode vscode \
    && mkdir -p /home/vscode/.composer \
    && chown -R vscode:vscode /home/vscode

# Give vscode user ownership of workspace
RUN chown -R vscode:vscode /var/www

# Switch to vscode user
USER vscode
```

**Key Points**:
- Uses `vscode` username instead of `www-data` (VS Code convention)
- Same UID/GID mapping approach
- Creates `.composer` directory for Composer cache

---

### 4. Docker Compose (`docker-compose.yml`)

```yaml
services:
  app:
    build:
      context: .
      dockerfile: docker/Dockerfile
      args:
        UID: ${UID:-1000}  # From .env, defaults to 1000
        GID: ${GID:-1000}
    user: "${UID:-1000}:${GID:-1000}"  # Runtime user
    # ... rest of config

  queue:
    build:
      context: .
      dockerfile: docker/Dockerfile
      args:
        UID: ${UID:-1000}
        GID: ${GID:-1000}
    user: "${UID:-1000}:${GID:-1000}"
    # ... rest of config
```

**Key Points**:
- `args:` - Passes UID/GID to Dockerfile during build
- `user:` - Forces container to run as specific user at runtime
- `${UID:-1000}` - Uses .env value, defaults to 1000 if not set

---

### 5. DevContainer Config (`.devcontainer/devcontainer.json`)

```json
{
  "build": {
    "args": {
      "UID": "1000",
      "GID": "1000"
    }
  },
  "remoteUser": "vscode"
}
```

**Key Points**:
- `args:` - Build arguments for devcontainer Dockerfile
- `remoteUser:` - VS Code terminal runs as vscode user

---

## Verification

### Check Container User

```bash
# Check current user in container
docker compose exec app whoami
# Output: www-data

# Check UID/GID in container
docker compose exec app id
# Output: uid=1000(www-data) gid=1000(www-data) groups=1000(www-data)

# Compare with host
id
# Output: uid=1000(chris) gid=1000(chris) groups=1000(chris),...
```

**Result**: Container UID/GID matches host UID/GID

### Test File Ownership

```bash
# Create file inside container
docker compose exec app touch /var/www/test.txt

# Check ownership on host
ls -la test.txt
# Output: -rw-r--r-- 1 chris chris 0 Nov 12 20:00 test.txt

# Delete without sudo
rm test.txt
# Success! No permission denied
```

---

## Before vs After

### Before (Containers as Root)

```bash
# Container creates vendor directory
docker compose exec app composer install

# Check ownership
ls -ld vendor/
# drwxr-xr-x 1 root root 4096 Nov 12 20:00 vendor/

# Try to delete
rm -rf vendor/
# rm: cannot remove 'vendor/': Permission denied

# Need sudo
sudo rm -rf vendor/
# Success, but annoying
```

### After (Containers as Host User)

```bash
# Container creates vendor directory
docker compose exec app composer install

# Check ownership
ls -ld vendor/
# drwxr-xr-x 1 chris chris 4096 Nov 12 20:00 vendor/

# Delete normally
rm -rf vendor/
# Success!
```

---

## Common Scenarios

### Scenario 1: Composer Install

```bash
docker compose exec app composer install
ls -la vendor/
# All files owned by chris:chris
```

### Scenario 2: Laravel Cache

```bash
docker compose exec app php artisan config:cache
ls -la bootstrap/cache/
# All cache files owned by chris:chris
```

### Scenario 3: Log Files

```bash
docker compose exec app php artisan octane:start
ls -la storage/logs/
# laravel.log owned by chris:chris
```

### Scenario 4: Upload Files

When PHP-FPM (running as www-data UID 1000) saves uploaded files:
```bash
# Simulated file upload
docker compose exec app bash -c 'echo "test" > storage/app/upload.txt'
ls -la storage/app/upload.txt
# -rw-r--r-- 1 chris chris 5 Nov 12 20:00 storage/app/upload.txt
```

---

## Troubleshooting

### Issue: "Permission denied" when container tries to write

**Cause**: Storage/cache directories not writable by container user

**Solution**:
```bash
# On host, ensure directories exist and are writable
chmod -R 775 storage bootstrap/cache
```

### Issue: Files still owned by root

**Cause**: Container not rebuilt with new user mapping

**Solution**:
```bash
# Rebuild containers from scratch
docker compose down
docker compose build --no-cache
docker compose up -d
```

### Issue: Different UID/GID on your system

**Cause**: Your user has different UID/GID than 1000

**Solution**:
```bash
# Check your UID/GID
id -u  # Your UID
id -g  # Your GID

# Update .env
UID=<your-uid>
GID=<your-gid>

# Rebuild
docker compose build --no-cache
docker compose up -d
```

---

## Production Deployment

For production, you may want different UIDs:

### Option 1: Override in Production .env

```bash
# production.env
UID=33    # www-data on Debian/Ubuntu
GID=33
```

### Option 2: Use Standard www-data

Remove `.env` file in production, Dockerfile defaults to UID 1000 will be used, or modify Dockerfile to use system www-data:

```dockerfile
ARG UID=82   # Alpine www-data UID
ARG GID=82   # Alpine www-data GID
```

---

## Benefits

**No permission issues** - Files owned by host user
**No sudo required** - Delete/modify files normally
**Clean workflow** - Works same in both docker-compose and devcontainer
**Production ready** - Can override UID/GID per environment
**Secure** - Containers don't run as root

---

## Files Changed

| File | Change |
|------|--------|
| `docker/Dockerfile` | Added UID/GID args, USER directive |
| `.devcontainer/Dockerfile` | Added UID/GID args, vscode user |
| `docker-compose.yml` | Added build args and user directives |
| `.devcontainer/devcontainer.json` | Added remoteUser |
| `.env` | Created with UID/GID variables |
| `scripts/setup-phase1.sh` | Removed chown (no longer needed) |

---

**Status**: All files updated and working. Permission issues resolved.
