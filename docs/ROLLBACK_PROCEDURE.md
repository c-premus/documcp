# DocuMCP Rollback Procedure

This document outlines the manual rollback procedure for DocuMCP deployments.

## When to Rollback

Rollback is required when:

1. **Deployment workflow fails** and sends failure notification
2. **Application health check** reports "unhealthy" status
3. **Critical functionality** is broken after deployment
4. **Database migrations** cause data issues (requires special handling - see below)

## Quick Rollback

### Using the Rollback Script

```bash
# SSH to the server
ssh user@aughtside.999.haus

# Navigate to DocuMCP
cd /mnt/md0/stack/home-stack/services/documcp

# List recent deployments
./scripts/rollback.sh --list

# Rollback to previous deployment
./scripts/rollback.sh --last

# Rollback to specific commit
./scripts/rollback.sh abc1234

# Rollback to specific deploy tag
./scripts/rollback.sh deploy-20251207-143022
```

### What the Script Does

1. **Stops Horizon workers** gracefully (waits for current jobs to complete)
2. **Reverts code** to specified commit
3. **Installs dependencies** via Composer (no migrations by default)
4. **Rebuilds caches** (config, routes, views, events)
5. **Restarts services** (Horizon, Reverb, Pulse)
6. **Verifies health** via deep health check
7. **Sends notification** to Discord

## Manual Rollback Steps

If the script fails or is unavailable:

### Step 1: Stop Queue Workers

```bash
docker exec documcp-app php artisan horizon:terminate
sleep 10
```

### Step 2: Identify Target Commit

```bash
cd /mnt/md0/stack/home-stack/services/documcp

# List deploy tags
git tag -l "deploy-*" --sort=-version:refname | head -10

# Or list recent commits
git log --oneline -n 10 origin/main
```

### Step 3: Revert Code

```bash
# Using checkout (preserves uncommitted changes)
git checkout <commit-sha> -- .

# Or using reset (discards all changes)
git reset --hard <commit-sha>
```

### Step 4: Reinstall Dependencies

```bash
docker exec -w /var/www documcp-app composer install \
    --no-dev --optimize-autoloader --no-interaction
```

### Step 5: Rebuild Caches

```bash
docker exec -w /var/www documcp-app php artisan optimize:clear
docker exec -w /var/www documcp-app php artisan config:cache
docker exec -w /var/www documcp-app php artisan route:cache
docker exec -w /var/www documcp-app php artisan view:cache
docker exec -w /var/www documcp-app php artisan event:cache
```

### Step 6: Restart Services

```bash
docker restart documcp-horizon documcp-reverb documcp-pulse
sleep 15
```

### Step 7: Verify Health

```bash
# Quick check
docker exec documcp-app curl -sf http://localhost:9000/api/health/deep | jq

# Or use the health check script
/mnt/md0/stack/home-stack/services/airflow/scripts/documcp/deployment-health-check.sh
```

## Database Migration Rollback

**WARNING**: Database rollbacks are dangerous and may cause data loss.

### When Migration Rollback is Needed

- Migration adds column with invalid default
- Migration drops data unexpectedly
- Migration causes foreign key constraint issues

### Procedure

1. **Identify the migration to rollback**

```bash
docker exec documcp-app php artisan migrate:status
```

2. **Check if migration is reversible**

Look at the migration file's `down()` method:
```bash
docker exec documcp-app cat database/migrations/2025_xx_xx_migration.php
```

3. **Rollback specific migration(s)**

```bash
# Rollback last batch
docker exec documcp-app php artisan migrate:rollback --step=1

# Rollback specific number of migrations
docker exec documcp-app php artisan migrate:rollback --step=3
```

4. **Verify data integrity**

```bash
docker exec documcp-app php artisan tinker --execute="
    echo 'Documents: ' . \App\Models\Document::count();
    echo 'Users: ' . \App\Models\User::count();
"
```

### If Migration Cannot Be Rolled Back

1. **Restore from backup** (see `docs/BACKUP_STRATEGY.md`)
2. **Create fix migration** instead of rolling back

## Post-Rollback Actions

1. **Monitor the application** for 15-30 minutes
2. **Check Horizon dashboard** for queue health
3. **Verify search functionality** works correctly
4. **Notify the team** via Discord
5. **Create incident report** documenting:
   - What failed
   - When rollback was performed
   - What commit was rolled back to
   - Root cause (if known)
6. **Fix the issue** in a new branch
7. **Test thoroughly** before re-deploying

## Prevention

### Before Merging to Main

- Run full CI pipeline (all jobs must pass)
- Test migrations locally: `php artisan migrate:fresh --seed`
- Review migration `down()` methods
- Test with production-like data volumes

### During Deployment

- Watch Forgejo Actions workflow progress
- Monitor Docker container health
- Check application logs in real-time:
  ```bash
  docker logs -f documcp-app
  ```

### General Best Practices

- Keep deployments small and frequent
- Use feature flags for risky changes
- Test database changes with production backup
- Always have a rollback plan before deploying

## Emergency Contacts

- **Primary**: Check Forgejo Actions run logs first
- **Fallback**: Review recent commits and deploy tags
- **Discord**: Deployment notifications channel

## Related Documentation

- `docs/PRODUCTION_DEPLOYMENT.md` - Full deployment guide
- `docs/BACKUP_STRATEGY.md` - Backup and restore procedures
- `docs/HEALTH_CHECKS.md` - Health check endpoints
- `.forgejo/workflows/deploy.yml` - Deployment workflow
