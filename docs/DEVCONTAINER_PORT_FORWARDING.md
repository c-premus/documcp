# Devcontainer Port Forwarding for MCP and OAuth

This document explains how to configure VS Code devcontainers to work with MCP (Model Context Protocol) tools like `mcp-remote` that require OAuth callbacks on localhost.

**Environment**: PHP 8.4, Node.js 24, Docker CLI, Tea CLI (Forgejo)

## The Problem

When using MCP clients (like Claude Code with `mcp-remote`) inside a VS Code devcontainer, OAuth authentication often fails because:

1. **mcp-remote** binds to a dynamic localhost port (e.g., `http://localhost:18107/oauth/callback`)
2. **VS Code auto-detects** this port and forwards it to a different port (e.g., `30616`)
3. **OAuth callback fails** because the redirect URI no longer matches

Example failure:
```
Expected callback: http://localhost:18107/oauth/callback?code=...
Actual callback:   http://localhost:30616/oauth/callback?code=...  # FAILS
```

## Root Cause: VS Code's Two Port Forwarding Mechanisms

VS Code has **two separate** port forwarding systems:

| Mechanism | Trigger | Respects Settings? |
|-----------|---------|-------------------|
| **Auto-detection** | Process scanning for listening ports | Yes (`onAutoForward: ignore`) |
| **User-initiated** | Clicking localhost links in terminal | **No** (always forwards) |

This is a [known VS Code limitation](https://github.com/microsoft/vscode-remote-release/issues/6972).

## The Solution: Host Network Mode + Port Configuration

### 1. Use Host Network Mode

In `docker-compose.yaml`, configure the main development container with `network_mode: host`:

```yaml
services:
  documcp-dev:
    # ... other config ...

    # Host network mode: localhost in container = localhost on host
    # This allows mcp-remote OAuth callbacks to work properly
    # Trade-off: Cannot use Docker DNS (service names), must use 127.0.0.1
    network_mode: host

    environment:
      # Service connections via host network to exposed ports
      DB_HOST: "127.0.0.1"
      REDIS_HOST: "127.0.0.1"
      MEILISEARCH_HOST: "http://127.0.0.1:7700"
```

**Why host network mode?**
- Container shares the host's network stack directly
- `localhost:8000` in container = `localhost:8000` on host
- No port mapping or forwarding needed
- OAuth callbacks reach the correct port

**Trade-offs:**
- Cannot use Docker DNS names (e.g., `postgres`) - must use `127.0.0.1`
- Supporting services must expose ports to the host
- Container and host share the same network namespace

### 2. Configure Supporting Services with Port Exposure

Supporting services (database, cache, etc.) need to expose their ports since they cannot use Docker's internal DNS with host network mode:

```yaml
  postgres:
    image: postgres:18-alpine
    ports:
      - "5432:5432"  # Expose to host
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U documcp -d documcp"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:8-alpine
    ports:
      - "6379:6379"  # Expose to host
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

  meilisearch:
    image: getmeili/meilisearch:v1.33
    ports:
      - "7700:7700"  # Expose to host
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7700/health"]
      interval: 5s
      timeout: 5s
      retries: 5

  mailhog:
    image: mailhog/mailhog
    ports:
      - "8025:8025"  # Web UI
      - "2525:1025"  # SMTP
```

### 3. Disable VS Code Auto-Forwarding

In `devcontainer.json`, configure VS Code to stop auto-forwarding ports:

```jsonc
{
    // Explicitly list ports you DO want forwarded (for documentation/clarity)
    // With host network mode, these are already accessible - this is informational
    "forwardPorts": [8000, 8080, 7700, 5432, 6379],

    // CRITICAL: Ignore all unlisted ports
    "otherPortsAttributes": {
        "onAutoForward": "ignore"
    },

    // Configure known ports to not auto-forward
    "portsAttributes": {
        "8000": {
            "label": "Application",
            "protocol": "http",
            "onAutoForward": "ignore"
        },
        "7700": {
            "label": "Meilisearch",
            "protocol": "http",
            "onAutoForward": "ignore"
        },
        "5432": {
            "label": "PostgreSQL",
            "onAutoForward": "ignore"
        },
        "6379": {
            "label": "Redis",
            "onAutoForward": "ignore"
        },
        // mcp-remote default port
        "3334": {
            "label": "mcp-remote OAuth callback",
            "onAutoForward": "ignore"
        },
        // Dynamic/ephemeral port range
        "10000-65535": {
            "label": "Dynamic/Ephemeral",
            "onAutoForward": "ignore"
        }
    },

    // VS Code settings to disable auto-forwarding globally
    "customizations": {
        "vscode": {
            "settings": {
                "remote.autoForwardPorts": false,
                "remote.restoreForwardedPorts": false,
                "remote.autoForwardPortsSource": "output"
            }
        }
    }
}
```

### 4. Use a Fixed Port for mcp-remote

The most reliable solution is to use mcp-remote's fixed port option:

```bash
# RECOMMENDED: Use fixed port 3334
claude mcp add documcp -- npx -y mcp-remote http://localhost:8000/documcp 3334 --allow-http
```

**Why port 3334?**
- It's mcp-remote's default port
- Explicitly configured in `portsAttributes` with `onAutoForward: ignore`
- Consistent across sessions
- Less likely to conflict with other services

## Configuration Reference

### Complete devcontainer.json Example

```jsonc
{
    "name": "MCP Development",
    "dockerComposeFile": "docker-compose.yaml",
    "service": "app",
    "workspaceFolder": "/workspaces/project",
    "remoteUser": "developer",

    // Port forwarding configuration for host network mode
    "forwardPorts": [8000, 5432, 6379],

    "otherPortsAttributes": {
        "onAutoForward": "ignore"
    },

    "portsAttributes": {
        "8000": {
            "label": "Application",
            "onAutoForward": "ignore"
        },
        "3334": {
            "label": "mcp-remote OAuth",
            "onAutoForward": "ignore"
        },
        "10000-65535": {
            "label": "Dynamic",
            "onAutoForward": "ignore"
        }
    },

    "customizations": {
        "vscode": {
            "settings": {
                "remote.autoForwardPorts": false,
                "remote.restoreForwardedPorts": false,
                "remote.autoForwardPortsSource": "output"
            }
        }
    }
}
```

### Complete docker-compose.yaml Example

```yaml
services:
  documcp-dev:
    build:
      context: .
      dockerfile: Dockerfile
      args:
        - PHP_VER=8.4
    command: sleep infinity
    init: true
    network_mode: host
    environment:
      DB_HOST: "127.0.0.1"
      REDIS_HOST: "127.0.0.1"
      MEILISEARCH_HOST: "http://127.0.0.1:7700"
      MAIL_HOST: "127.0.0.1"
      MAIL_PORT: "2525"
      XDEBUG_CONFIG: "client_host=127.0.0.1 start_with_request=trigger client_port=9003"
      XDEBUG_MODE: "off"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      meilisearch:
        condition: service_healthy

  postgres:
    image: postgres:18-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: documcp
      POSTGRES_USER: documcp
      POSTGRES_PASSWORD: documcp
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U documcp -d documcp"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:8-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

  meilisearch:
    image: getmeili/meilisearch:v1.33
    ports:
      - "7700:7700"
    environment:
      MEILI_ENV: development
      MEILI_NO_ANALYTICS: "true"
      MEILI_MASTER_KEY: devcontainer-meilisearch-key
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7700/health"]
      interval: 5s
      timeout: 5s
      retries: 5

  mailhog:
    image: mailhog/mailhog
    ports:
      - "8025:8025"
      - "2525:1025"
```

## Troubleshooting

### OAuth Callback Still Fails

1. **Check VS Code Ports panel**: Look for "User Forwarded" entries and remove them
2. **Rebuild the devcontainer**: Settings only apply on container creation
3. **Verify mcp-remote port**: Ensure you specified port 3334 explicitly

### "Address already in use" Error

If port 3334 is already in use:
```bash
# Find what's using the port
lsof -i :3334

# Use a different fixed port
claude mcp add documcp -- npx -y mcp-remote http://localhost:8000/documcp 3335 --allow-http
```

Then add port 3335 to your `portsAttributes` in devcontainer.json.

### Services Can't Connect

With `network_mode: host`, you must use `127.0.0.1` instead of service names:

```yaml
# WRONG (with host network mode)
DB_HOST: "postgres"

# CORRECT (with host network mode)
DB_HOST: "127.0.0.1"
```

### Xdebug Configuration

With host network mode, Xdebug needs explicit host configuration:

```yaml
environment:
  XDEBUG_CONFIG: "client_host=127.0.0.1 start_with_request=trigger client_port=9003"
  XDEBUG_MODE: "off"  # Enable with "debug" when needed
```

## Alternative: Device Authorization Grant (RFC 8628)

For MCP servers that support it, the Device Authorization Grant eliminates the need for localhost callbacks entirely:

1. Server displays a URL and code
2. User visits URL in any browser, enters code
3. Server polls for authorization completion
4. No callback URL needed

This approach works regardless of port forwarding issues but requires server-side support.

## VS Code Issues Reference

- [#6972](https://github.com/microsoft/vscode-remote-release/issues/6972) - Localhost links bypass onAutoForward
- [#221888](https://github.com/microsoft/vscode/issues/221888) - No reliable way to disable auto-forwarding
- [#161045](https://github.com/microsoft/vscode/issues/161045) - remote.autoForwardPorts gets ignored

## Devcontainer Features

The DocuMCP devcontainer includes these features configured via `devcontainer.json`:

| Feature | Purpose |
|---------|---------|
| `common-utils` | Creates laravel user, installs git, vim, sudo, zsh with oh-my-zsh |
| `node` | Node.js 24 with nvm for npm/npx commands |
| `docker-outside-of-docker` | Docker CLI for building production images |

### Post-Create Commands

The devcontainer automatically runs:
- `composer install && npm install` - Install PHP and Node dependencies
- `tea-setup` - Configure Forgejo CLI (if FORGEJO_URL/FORGEJO_TOKEN set)
- Claude Code installation - For AI-assisted development

## Summary

| Setting | Purpose |
|---------|---------|
| `network_mode: host` | Container shares host network, localhost works directly |
| `onAutoForward: ignore` | Prevents VS Code from auto-forwarding detected ports |
| `remote.autoForwardPorts: false` | Disables VS Code's auto-forward feature |
| `remote.restoreForwardedPorts: false` | Prevents restoring previous forwards |
| Fixed port for mcp-remote | Ensures consistent, configured port for OAuth |

With host network mode, disabled auto-forwarding, and fixed ports, MCP OAuth authentication works in VS Code devcontainers.
