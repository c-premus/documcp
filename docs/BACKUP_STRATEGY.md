# Backup Strategy

## Overview

DocuMCP requires backups for databases, uploaded files, and configuration. This document covers backup procedures, automation, retention policies, and disaster recovery.

## What to Backup

### 1. PostgreSQL Database (Critical)

**Contains**:
- User accounts and authentication data
- Document metadata (titles, descriptions, tags)
- Search index mappings
- OAuth clients and tokens
- Application configuration

**Backup Method**: PostgreSQL `pg_dump` with custom format and compression

**Frequency**: Daily at 2:00 AM

**Retention**: 30 days

### 2. Storage Directory (Critical)

**Contains**:
- Uploaded PDF, DOCX, XLSX, HTML, Markdown files
- Extracted text content
- Generated thumbnails
- User-uploaded assets
- Git Templates repository clones (`storage/app/git-templates/`)

**Backup Method**: tar.gz archive

**Frequency**: Daily at 3:00 AM

**Retention**: 30 days

**Note**: The backup script backs up the entire `storage/app` directory, which includes both document uploads and git-templates. Git Templates can be re-synced from source repositories if lost, but backing them up avoids re-cloning and preserves any local modifications.

### 3. Environment Configuration (Important)

**Contains**:
- `.env` file with credentials
- Docker Compose configuration
- nginx configuration
- Application configuration files

**Backup Method**: Manual backup before changes, stored in secure location

**Frequency**: On-demand before deployments

**Retention**: Keep all versions

### 4. Meilisearch Index (Optional - Can be Rebuilt)

**Contains**:
- Full-text search index
- Can be rebuilt from database via `php artisan documcp:reindex`

**Backup Method**: Not backed up (index is rebuilt from database)

**Frequency**: N/A

**Retention**: N/A

## Backup Scripts

### Database Backup

**Script**: `scripts/backup-database.sh`

**Usage**:
```bash
# Backup to default location (./backups/)
./scripts/backup-database.sh

# Backup to custom location
./scripts/backup-database.sh /mnt/backups

# Backup to network storage
./scripts/backup-database.sh /mnt/nfs/documcp-backups
```

**Output**:
```
./backups/documcp_database_20251122_020000.sql.gz
./backups/documcp_database_latest.sql.gz (symlink)
```

**Features**:
- PostgreSQL custom format with level-9 compression
- Timestamped filenames
- Latest backup symlink for access
- 30-day automatic retention
- Validates backup file creation
- Logging with timestamps

### Storage Backup

**Script**: `scripts/backup-storage.sh`

**Usage**:
```bash
# Backup to default location (./backups/)
./scripts/backup-storage.sh

# Backup to custom location
./scripts/backup-storage.sh /mnt/backups
```

**Output**:
```
./backups/documcp_storage_20251122_030000.tar.gz
./backups/documcp_storage_latest.tar.gz (symlink)
```

**Features**:
- tar.gz compression
- Excludes cache, temp files, and logs
- Timestamped filenames
- Latest backup symlink
- 30-day automatic retention
- File count verification

**Included Directories**:
- `storage/app/documents/` - Uploaded documents
- `storage/app/git-templates/` - Cloned Git Template repositories

### Database Restore

**Script**: `scripts/restore-database.sh`

**Usage**:
```bash
# Restore from specific backup
./scripts/restore-database.sh backups/documcp_database_20251122_020000.sql.gz

# Restore from latest backup
./scripts/restore-database.sh backups/documcp_database_latest.sql.gz
```

**DANGER**: ⚠️ This script will **DROP and recreate the database**, deleting all current data!

**Safety Features**:
- Requires explicit "yes" confirmation
- Stops application containers before restore
- Displays backup metadata before proceeding
- Verifies database after restore
- Restarts application automatically

**Restore Steps**:
1. Stop application and queue containers
2. Drop existing database
3. Create fresh database
4. Restore from backup file
5. Start application containers
6. Verify database connectivity

## Automation

### Cron Jobs (Recommended)

Add to `/etc/cron.d/documcp` or root/user crontab:

```bash
# DocuMCP Backups (runs at 2 AM and 3 AM daily)
0 2 * * * cd /path/to/documcp && ./scripts/backup-database.sh >> /var/log/documcp-backup.log 2>&1
0 3 * * * cd /path/to/documcp && ./scripts/backup-storage.sh >> /var/log/documcp-backup.log 2>&1
```

**Schedule Explanation**:
- Database backup: 2:00 AM (low traffic time)
- Storage backup: 3:00 AM (after database backup completes)
- Logs: Append to `/var/log/documcp-backup.log`

**Install cron jobs**:
```bash
# Create cron file
sudo tee /etc/cron.d/documcp <<EOF
0 2 * * * root cd /workspaces/DocuMCP && ./scripts/backup-database.sh >> /var/log/documcp-backup.log 2>&1
0 3 * * * root cd /workspaces/DocuMCP && ./scripts/backup-storage.sh >> /var/log/documcp-backup.log 2>&1
EOF

# Set permissions
sudo chmod 0644 /etc/cron.d/documcp

# Verify cron jobs
sudo crontab -l
```

### Docker Compose Service (Alternative)

Add a backup service to `docker-compose.yml`:

```yaml
services:
  backup:
    image: alpine:latest
    volumes:
      - ./scripts:/scripts
      - ./backups:/backups
      - ./storage:/storage
      - /var/run/docker.sock:/var/run/docker.sock
    command: |
      sh -c "
        apk add --no-cache dcron docker-cli postgresql-client
        echo '0 2 * * * /scripts/backup-database.sh /backups' > /etc/crontabs/root
        echo '0 3 * * * /scripts/backup-storage.sh /backups' >> /etc/crontabs/root
        crond -f -l 2
      "
    restart: unless-stopped
```

### Systemd Timers (Alternative)

**Timer**: `/etc/systemd/system/documcp-backup.timer`
```ini
[Unit]
Description=DocuMCP Backup Timer
Requires=documcp-backup.service

[Timer]
OnCalendar=*-*-* 02:00:00
Persistent=true

[Install]
WantedBy=timers.target
```

**Service**: `/etc/systemd/system/documcp-backup.service`
```ini
[Unit]
Description=DocuMCP Backup Service
After=network.target

[Service]
Type=oneshot
WorkingDirectory=/workspaces/DocuMCP
ExecStart=/workspaces/DocuMCP/scripts/backup-database.sh
ExecStart=/workspaces/DocuMCP/scripts/backup-storage.sh
StandardOutput=journal
StandardError=journal
```

**Enable**:
```bash
sudo systemctl enable documcp-backup.timer
sudo systemctl start documcp-backup.timer
sudo systemctl status documcp-backup.timer
```

## Off-Site Backups

Local backups protect against application failures but not against hardware failures, disasters, or ransomware. **Always maintain off-site backups**.

### Option 1: Network Attached Storage (NAS)

**Mount NFS share**:
```bash
# Install NFS client
sudo apt-get install nfs-common

# Create mount point
sudo mkdir -p /mnt/nfs/documcp-backups

# Add to /etc/fstab
nas.local:/volume1/backups /mnt/nfs/documcp-backups nfs defaults,_netdev 0 0

# Mount
sudo mount -a
```

**Update backup scripts**:
```bash
./scripts/backup-database.sh /mnt/nfs/documcp-backups
./scripts/backup-storage.sh /mnt/nfs/documcp-backups
```

### Option 2: rsync to Remote Server

**Setup SSH key authentication**:
```bash
ssh-keygen -t ed25519 -f ~/.ssh/documcp_backup
ssh-copy-id -i ~/.ssh/documcp_backup.pub backup@remote.example.com
```

**Create rsync script** (`scripts/sync-to-remote.sh`):
```bash
#!/bin/bash
rsync -avz --delete \
    -e "ssh -i ~/.ssh/documcp_backup" \
    ./backups/ \
    backup@remote.example.com:/backups/documcp/
```

**Add to cron**:
```bash
0 4 * * * cd /workspaces/DocuMCP && ./scripts/sync-to-remote.sh >> /var/log/documcp-backup.log 2>&1
```

### Option 3: Cloud Storage (S3, Google Cloud Storage, Azure Blob)

**Install AWS CLI**:
```bash
sudo apt-get install awscli
```

**Configure credentials**:
```bash
aws configure
# Enter access key, secret key, region
```

**Create S3 sync script** (`scripts/sync-to-s3.sh`):
```bash
#!/bin/bash
aws s3 sync ./backups/ s3://my-bucket/documcp-backups/ \
    --storage-class GLACIER \
    --exclude "*" \
    --include "documcp_database_*.sql.gz" \
    --include "documcp_storage_*.tar.gz"
```

**Add to cron**:
```bash
0 4 * * * cd /workspaces/DocuMCP && ./scripts/sync-to-s3.sh >> /var/log/documcp-backup.log 2>&1
```

**Cost Optimization**:
- Use **S3 Glacier** or **GCS Nearline** for long-term retention (cheaper)
- Use **lifecycle policies** to transition old backups to cold storage
- Compress backups before upload (already done by scripts)

## Retention Policies

### Default Retention (Local)

- **Database**: 30 days (modify `RETENTION_DAYS` in `backup-database.sh`)
- **Storage**: 30 days (modify `RETENTION_DAYS` in `backup-storage.sh`)

### Recommended Retention Strategy

**Tier 1 - Recent (Daily)**:
- Retain: Last 7 days
- Location: Local disk (`./backups/`)
- Purpose: Quick recovery from recent mistakes

**Tier 2 - Medium-Term (Weekly)**:
- Retain: Last 4 weeks (1 backup per week)
- Location: NAS or remote server
- Purpose: Recovery from issues discovered after a few days

**Tier 3 - Long-Term (Monthly)**:
- Retain: Last 12 months (1 backup per month)
- Location: Cloud storage (S3 Glacier, GCS Nearline)
- Purpose: Compliance, audit, disaster recovery

**Example Script** (`scripts/tiered-retention.sh`):
```bash
#!/bin/bash
# Keep daily backups for 7 days
find ./backups -name "documcp_*.sql.gz" -mtime +7 -delete

# Copy weekly backups (Sundays) to NAS
if [ $(date +%u) -eq 7 ]; then
    rsync -avz ./backups/ /mnt/nfs/documcp-backups/weekly/
    find /mnt/nfs/documcp-backups/weekly -mtime +28 -delete
fi

# Copy monthly backups (1st of month) to S3
if [ $(date +%d) -eq 01 ]; then
    aws s3 cp ./backups/documcp_database_latest.sql.gz \
        s3://my-bucket/documcp/monthly/documcp_$(date +%Y%m).sql.gz
    aws s3 cp ./backups/documcp_storage_latest.tar.gz \
        s3://my-bucket/documcp/monthly/documcp_$(date +%Y%m).tar.gz
fi
```

## Testing Backups

**CRITICAL**: Untested backups are worthless. Test restoration regularly.

### Monthly Backup Test

**1. Create test environment**:
```bash
# Clone docker-compose.yml to docker-compose.test.yml
cp docker-compose.yml docker-compose.test.yml

# Modify ports to avoid conflicts (8081, 5433, etc.)
sed -i 's/8000:80/8081:80/g' docker-compose.test.yml
sed -i 's/5432:5432/5433:5432/g' docker-compose.test.yml
```

**2. Start test environment**:
```bash
docker-compose -f docker-compose.test.yml up -d
```

**3. Restore from backup**:
```bash
# Modify restore script to use test database
DB_PORT=5433 ./scripts/restore-database.sh backups/documcp_database_latest.sql.gz
```

**4. Verify functionality**:
```bash
# Test API
curl http://localhost:8081/api/health/deep

# Test login
# Navigate to http://localhost:8081/login

# Test document search
curl -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/search?q=test
```

**5. Cleanup**:
```bash
docker-compose -f docker-compose.test.yml down -v
```

### Automated Backup Verification

**Script** (`scripts/verify-backup.sh`):
```bash
#!/bin/bash
# Verify backup file integrity
BACKUP_FILE="backups/documcp_database_latest.sql.gz"

# Check file exists
if [ ! -f "$BACKUP_FILE" ]; then
    echo "ERROR: Backup file not found"
    exit 1
fi

# Check file is not empty
if [ ! -s "$BACKUP_FILE" ]; then
    echo "ERROR: Backup file is empty"
    exit 1
fi

# Check file is valid gzip
if ! gunzip -t "$BACKUP_FILE" 2>/dev/null; then
    echo "ERROR: Backup file is corrupted (invalid gzip)"
    exit 1
fi

echo "Backup verification passed"
```

**Add to cron** (runs after backup):
```bash
0 4 * * * cd /workspaces/DocuMCP && ./scripts/verify-backup.sh || mail -s "DocuMCP Backup Failed" admin@example.com
```

## Disaster Recovery Procedures

### Scenario 1: Accidental Data Deletion

**Symptoms**: User accidentally deleted documents or data

**Recovery**:
1. Identify when data was deleted (check logs)
2. Find last backup before deletion
3. Restore to test environment
4. Export affected data
5. Import to production

### Scenario 2: Database Corruption

**Symptoms**: Database errors, integrity constraint violations

**Recovery**:
1. Stop application: `docker-compose stop app queue`
2. Restore from latest backup: `./scripts/restore-database.sh backups/documcp_database_latest.sql.gz`
3. Verify database: `curl http://localhost:8000/api/health/deep`
4. If successful, resume operation

### Scenario 3: Complete Server Failure

**Symptoms**: Server hardware failure, ransomware, catastrophic data loss

**Recovery**:
1. Provision new server
2. Install Docker and Docker Compose
3. Clone DocuMCP repository
4. Copy `.env` from secure backup
5. Restore database from off-site backup
6. Restore storage from off-site backup
7. Start application: `docker-compose up -d`
8. Rebuild search index: `php artisan documcp:reindex`
9. Re-sync Git Templates (if not restored from backup): `php artisan git-template:sync --all`
10. Verify all functionality

**Estimated RTO** (Recovery Time Objective): 2-4 hours

**Estimated RPO** (Recovery Point Objective): 24 hours (daily backups)

## Monitoring Backups

### Backup Success Monitoring

**Check backup logs**:
```bash
tail -f /var/log/documcp-backup.log
```

**Verify backup files**:
```bash
ls -lh backups/
```

**Monitor disk space**:
```bash
df -h backups/
```

### Alerting

**Send email on backup failure**:
```bash
#!/bin/bash
# scripts/backup-with-alert.sh
if ! ./scripts/backup-database.sh; then
    echo "Database backup failed at $(date)" | mail -s "DocuMCP Backup Failed" admin@example.com
fi

if ! ./scripts/backup-storage.sh; then
    echo "Storage backup failed at $(date)" | mail -s "DocuMCP Backup Failed" admin@example.com
fi
```

**Prometheus metrics** (custom):
```bash
# Export backup success/failure to Prometheus
echo "documcp_backup_last_success $(date +%s)" > /var/lib/node_exporter/textfile_collector/documcp_backup.prom
```

## Best Practices

### Security

- **Encrypt backups** if stored off-site (use GPG or backup tool encryption)
- **Secure backup credentials** (use environment variables, not hardcoded)
- **Limit access** to backup files (chmod 600, restrict network access)
- **Test restore procedures** monthly

### Performance

- **Schedule backups during low traffic** (2-3 AM)
- **Use compression** to reduce storage and transfer time
- **Monitor backup duration** (should complete in < 10 minutes for typical workload)
- **Avoid backing up temp/cache** files

### Reliability

- **Verify backup integrity** after creation
- **Test restores regularly** (monthly minimum)
- **Maintain multiple backup destinations** (local + off-site)
- **Document recovery procedures** (this document)

### Compliance

- **Define retention policies** based on legal/regulatory requirements
- **Log all backup/restore operations** for audit trail
- **Implement access controls** on backup files
- **Encrypt sensitive data** in backups

## Troubleshooting

### Backup Script Fails

**Check permissions**:
```bash
ls -l scripts/backup-*.sh
# Should be: -rwxr-xr-x
chmod +x scripts/backup-*.sh
```

**Check Docker access**:
```bash
docker ps | grep documcp_postgres
# Should show running container
```

**Check disk space**:
```bash
df -h backups/
# Should have >10GB free
```

### Restore Fails

**Check backup file integrity**:
```bash
gunzip -t backups/documcp_database_latest.sql.gz
# Should exit with no errors
```

**Check database credentials**:
```bash
grep DB_ .env
# Verify credentials match
```

**Check PostgreSQL version compatibility**:
```bash
docker exec documcp_postgres psql --version
# Should match version used for backup
```

## References

- [PostgreSQL Backup and Restore](https://www.postgresql.org/docs/current/backup.html)
- [Docker Volumes Backup](https://docs.docker.com/storage/volumes/#back-up-restore-or-migrate-data-volumes)
- [AWS S3 Glacier](https://aws.amazon.com/s3/storage-classes/glacier/)
- [3-2-1 Backup Rule](https://www.backblaze.com/blog/the-3-2-1-backup-strategy/)
