# StreamSpace Disaster Recovery Guide

**Document Version**: 1.0
**Last Updated**: 2025-11-26
**Owner**: Operations Team
**Status**: Production Ready

---

## Executive Summary

This document provides comprehensive disaster recovery (DR) procedures for StreamSpace deployments. It covers backup strategies, restore procedures, and DR testing requirements for all critical components.

**Recovery Targets:**

| Component | RPO (Recovery Point Objective) | RTO (Recovery Time Objective) |
|-----------|-------------------------------|-------------------------------|
| PostgreSQL Database | 15 minutes (with WAL archiving) | < 1 hour |
| User Home Directories | Per-organization policy (default: 24h) | < 4 hours |
| Configuration/Secrets | 0 (versioned in secret manager) | < 30 minutes |
| Redis Cache | N/A (ephemeral, rebuilt on restore) | < 15 minutes |

---

## Table of Contents

1. [Components Overview](#components-overview)
2. [Backup Strategy](#backup-strategy)
3. [Database Backup & Restore](#database-backup--restore)
4. [Storage Backup & Restore](#storage-backup--restore)
5. [Secrets & Configuration](#secrets--configuration)
6. [Full Disaster Recovery](#full-disaster-recovery)
7. [Validation & Testing](#validation--testing)
8. [Cloud Provider Guides](#cloud-provider-guides)
9. [Monitoring & Alerts](#monitoring--alerts)

---

## Components Overview

### Critical Data Components

| Component | Data Type | Criticality | Backup Method |
|-----------|-----------|-------------|---------------|
| PostgreSQL | User accounts, sessions, templates, audit logs, organizations | Critical | pg_dump / WAL archiving |
| NFS/PVC Storage | User home directories, persistent session data | High | Volume snapshots |
| Kubernetes Secrets | JWT secrets, DB credentials, IdP keys, TLS certs | Critical | Secret manager / encrypted backup |
| Redis | Agent connections, session cache, pub/sub state | Low | Not backed up (ephemeral) |
| Session CRDs | Active session state | Medium | etcd backup / kubectl export |

### Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      Control Plane                               │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │  PostgreSQL  │    │    Redis     │    │   Secrets    │      │
│  │  (Critical)  │    │  (Ephemeral) │    │  (Critical)  │      │
│  └──────┬───────┘    └──────────────┘    └──────┬───────┘      │
│         │                                        │               │
│         └────────────────┬───────────────────────┘               │
│                          ▼                                       │
│                   ┌──────────────┐                              │
│                   │   API Pods   │                              │
│                   └──────────────┘                              │
└─────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Agent Layer                                │
│  ┌──────────────────┐         ┌──────────────────┐             │
│  │    K8s Agent     │         │   Docker Agent   │             │
│  └────────┬─────────┘         └────────┬─────────┘             │
│           │                            │                        │
│           ▼                            ▼                        │
│  ┌──────────────────┐         ┌──────────────────┐             │
│  │  NFS/PVC Storage │         │  Docker Volumes  │             │
│  │     (High)       │         │     (High)       │             │
│  └──────────────────┘         └──────────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

---

## Backup Strategy

### Backup Schedule

| Component | Frequency | Retention | Method |
|-----------|-----------|-----------|--------|
| PostgreSQL Full | Daily (02:00 UTC) | 30 days | pg_dump to object storage |
| PostgreSQL WAL | Continuous | 7 days | WAL archiving to object storage |
| NFS Snapshots | Daily (03:00 UTC) | 14 days | CSI VolumeSnapshot |
| Secrets Export | On change + weekly | 90 days | Encrypted export to vault |
| etcd (CRDs) | Daily (04:00 UTC) | 7 days | etcdctl snapshot |

### Backup Locations

```yaml
# Recommended backup destinations
primary:
  type: S3-compatible object storage
  bucket: streamspace-backups-${REGION}
  encryption: AES-256-GCM
  versioning: enabled

secondary:  # For cross-region DR
  type: S3-compatible object storage
  bucket: streamspace-backups-${DR_REGION}
  replication: async from primary
```

---

## Database Backup & Restore

### PostgreSQL Backup

#### Option 1: pg_dump (Logical Backup)

**Daily Full Backup Script:**

```bash
#!/bin/bash
# /opt/streamspace/scripts/backup-db.sh

set -euo pipefail

# Configuration
BACKUP_DIR="/backups/postgres"
S3_BUCKET="s3://streamspace-backups/postgres"
RETENTION_DAYS=30
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="streamspace_${TIMESTAMP}.sql.gz"

# Get database credentials from Kubernetes secret
DB_HOST=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.host}' | base64 -d)
DB_NAME=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.database}' | base64 -d)
DB_USER=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.username}' | base64 -d)
DB_PASS=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.password}' | base64 -d)

# Create backup
echo "[$(date)] Starting PostgreSQL backup..."
PGPASSWORD="${DB_PASS}" pg_dump \
  -h "${DB_HOST}" \
  -U "${DB_USER}" \
  -d "${DB_NAME}" \
  --format=custom \
  --compress=9 \
  --verbose \
  --file="${BACKUP_DIR}/${BACKUP_FILE}"

# Verify backup integrity
echo "[$(date)] Verifying backup integrity..."
pg_restore --list "${BACKUP_DIR}/${BACKUP_FILE}" > /dev/null

# Upload to S3
echo "[$(date)] Uploading to S3..."
aws s3 cp "${BACKUP_DIR}/${BACKUP_FILE}" "${S3_BUCKET}/${BACKUP_FILE}" \
  --sse AES256 \
  --storage-class STANDARD_IA

# Cleanup old local backups
find "${BACKUP_DIR}" -name "streamspace_*.sql.gz" -mtime +7 -delete

# Cleanup old S3 backups (handled by lifecycle policy, but double-check)
echo "[$(date)] Backup completed: ${BACKUP_FILE}"
```

#### Option 2: WAL Archiving (Point-in-Time Recovery)

**PostgreSQL Configuration:**

```ini
# postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'aws s3 cp %p s3://streamspace-backups/wal/%f --sse AES256'
archive_timeout = 300  # Archive every 5 minutes max
```

**WAL Archive Script:**

```bash
#!/bin/bash
# /opt/streamspace/scripts/archive-wal.sh

WAL_FILE=$1
S3_BUCKET="s3://streamspace-backups/wal"

aws s3 cp "${WAL_FILE}" "${S3_BUCKET}/$(basename ${WAL_FILE})" \
  --sse AES256 \
  --expected-size $(stat -f%z "${WAL_FILE}" 2>/dev/null || stat -c%s "${WAL_FILE}")
```

#### Option 3: Managed Database Backup

**AWS RDS:**
```bash
# Enable automated backups (console or Terraform)
aws rds modify-db-instance \
  --db-instance-identifier streamspace-db \
  --backup-retention-period 30 \
  --preferred-backup-window "02:00-03:00" \
  --enable-performance-insights

# Create manual snapshot before major changes
aws rds create-db-snapshot \
  --db-instance-identifier streamspace-db \
  --db-snapshot-identifier streamspace-pre-migration-$(date +%Y%m%d)
```

**Google Cloud SQL:**
```bash
# Configure automated backups
gcloud sql instances patch streamspace-db \
  --backup-start-time=02:00 \
  --retained-backups-count=30 \
  --enable-point-in-time-recovery

# Create on-demand backup
gcloud sql backups create --instance=streamspace-db
```

### PostgreSQL Restore

#### Restore from pg_dump

```bash
#!/bin/bash
# /opt/streamspace/scripts/restore-db.sh

set -euo pipefail

BACKUP_FILE=$1  # e.g., streamspace_20251126_020000.sql.gz
S3_BUCKET="s3://streamspace-backups/postgres"

# 1. Enable maintenance mode (if available)
echo "[$(date)] Enabling maintenance mode..."
kubectl set env deployment/streamspace-api -n streamspace MAINTENANCE_MODE=true
kubectl rollout status deployment/streamspace-api -n streamspace

# 2. Download backup from S3
echo "[$(date)] Downloading backup..."
aws s3 cp "${S3_BUCKET}/${BACKUP_FILE}" /tmp/${BACKUP_FILE}

# 3. Get database credentials
DB_HOST=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.host}' | base64 -d)
DB_NAME=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.database}' | base64 -d)
DB_USER=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.username}' | base64 -d)
DB_PASS=$(kubectl get secret streamspace-db-secret -n streamspace -o jsonpath='{.data.password}' | base64 -d)

# 4. Create restore database (don't overwrite production directly)
echo "[$(date)] Creating restore database..."
PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -U "${DB_USER}" -d postgres -c \
  "DROP DATABASE IF EXISTS streamspace_restore; CREATE DATABASE streamspace_restore;"

# 5. Restore backup
echo "[$(date)] Restoring backup..."
PGPASSWORD="${DB_PASS}" pg_restore \
  -h "${DB_HOST}" \
  -U "${DB_USER}" \
  -d streamspace_restore \
  --verbose \
  --clean \
  --if-exists \
  /tmp/${BACKUP_FILE}

# 6. Validate restore
echo "[$(date)] Validating restore..."
PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -U "${DB_USER}" -d streamspace_restore -c "
  SELECT 'users' as table_name, COUNT(*) as row_count FROM users
  UNION ALL SELECT 'sessions', COUNT(*) FROM sessions
  UNION ALL SELECT 'templates', COUNT(*) FROM templates
  UNION ALL SELECT 'organizations', COUNT(*) FROM organizations;
"

# 7. Swap databases (atomic)
echo "[$(date)] Swapping databases..."
PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -U "${DB_USER}" -d postgres -c "
  SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}';
  ALTER DATABASE ${DB_NAME} RENAME TO ${DB_NAME}_old;
  ALTER DATABASE streamspace_restore RENAME TO ${DB_NAME};
"

# 8. Restart API to pick up restored data
echo "[$(date)] Restarting API..."
kubectl rollout restart deployment/streamspace-api -n streamspace
kubectl rollout status deployment/streamspace-api -n streamspace

# 9. Disable maintenance mode
kubectl set env deployment/streamspace-api -n streamspace MAINTENANCE_MODE=false

# 10. Verify API health
echo "[$(date)] Verifying API health..."
kubectl exec -n streamspace deployment/streamspace-api -- wget -qO- http://localhost:8000/health

echo "[$(date)] Restore completed successfully!"
```

#### Point-in-Time Recovery (PITR)

```bash
#!/bin/bash
# Restore to specific point in time

TARGET_TIME="2025-11-26 10:30:00 UTC"

# 1. Find base backup before target time
BASE_BACKUP=$(aws s3 ls s3://streamspace-backups/postgres/ | \
  awk '{print $4}' | sort -r | head -1)

# 2. Restore base backup to new instance
# ... (similar to above)

# 3. Replay WAL files up to target time
# This requires PostgreSQL recovery.conf:
cat > /var/lib/postgresql/data/recovery.conf <<EOF
restore_command = 'aws s3 cp s3://streamspace-backups/wal/%f %p'
recovery_target_time = '${TARGET_TIME}'
recovery_target_action = 'promote'
EOF

# 4. Start PostgreSQL in recovery mode
pg_ctl start -D /var/lib/postgresql/data
```

---

## Storage Backup & Restore

### Kubernetes PVC Snapshots

#### Create VolumeSnapshot

```yaml
# volume-snapshot.yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: streamspace-homes-snapshot-20251126
  namespace: streamspace
spec:
  volumeSnapshotClassName: csi-snapclass  # Provider-specific
  source:
    persistentVolumeClaimName: streamspace-homes
```

```bash
# Create snapshot
kubectl apply -f volume-snapshot.yaml

# Verify snapshot
kubectl get volumesnapshot -n streamspace
kubectl describe volumesnapshot streamspace-homes-snapshot-20251126 -n streamspace
```

#### Automated Snapshot Script

```bash
#!/bin/bash
# /opt/streamspace/scripts/snapshot-storage.sh

set -euo pipefail

NAMESPACE="streamspace"
TIMESTAMP=$(date +%Y%m%d)
RETENTION_DAYS=14

# List all PVCs to snapshot
PVCS=$(kubectl get pvc -n ${NAMESPACE} -o jsonpath='{.items[*].metadata.name}')

for PVC in ${PVCS}; do
  SNAPSHOT_NAME="${PVC}-snapshot-${TIMESTAMP}"

  echo "[$(date)] Creating snapshot: ${SNAPSHOT_NAME}"

  cat <<EOF | kubectl apply -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: ${SNAPSHOT_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: streamspace
    backup-date: "${TIMESTAMP}"
spec:
  volumeSnapshotClassName: csi-snapclass
  source:
    persistentVolumeClaimName: ${PVC}
EOF

done

# Cleanup old snapshots
echo "[$(date)] Cleaning up snapshots older than ${RETENTION_DAYS} days..."
OLD_DATE=$(date -d "-${RETENTION_DAYS} days" +%Y%m%d 2>/dev/null || date -v-${RETENTION_DAYS}d +%Y%m%d)

kubectl get volumesnapshot -n ${NAMESPACE} -o json | \
  jq -r ".items[] | select(.metadata.labels.\"backup-date\" < \"${OLD_DATE}\") | .metadata.name" | \
  xargs -r -I {} kubectl delete volumesnapshot {} -n ${NAMESPACE}
```

#### Restore from VolumeSnapshot

```bash
#!/bin/bash
# Restore PVC from snapshot

SNAPSHOT_NAME="streamspace-homes-snapshot-20251126"
RESTORE_PVC_NAME="streamspace-homes-restored"
NAMESPACE="streamspace"

# 1. Create PVC from snapshot
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${RESTORE_PVC_NAME}
  namespace: ${NAMESPACE}
spec:
  dataSource:
    name: ${SNAPSHOT_NAME}
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 100Gi  # Match original PVC size
EOF

# 2. Wait for PVC to be bound
kubectl wait --for=condition=Bound pvc/${RESTORE_PVC_NAME} -n ${NAMESPACE} --timeout=300s

# 3. Mount and verify data
kubectl run verify-restore --rm -it --image=busybox -n ${NAMESPACE} \
  --overrides='{
    "spec": {
      "containers": [{
        "name": "verify",
        "image": "busybox",
        "command": ["ls", "-la", "/data"],
        "volumeMounts": [{
          "name": "restored-data",
          "mountPath": "/data"
        }]
      }],
      "volumes": [{
        "name": "restored-data",
        "persistentVolumeClaim": {
          "claimName": "'${RESTORE_PVC_NAME}'"
        }
      }]
    }
  }'

# 4. Swap PVCs (requires downtime)
# Scale down sessions using old PVC
# Update deployment to use restored PVC
# Scale back up
```

### NFS Backup (Alternative)

```bash
#!/bin/bash
# Direct NFS backup using rsync

NFS_SERVER="nfs.internal:2049"
NFS_PATH="/exports/streamspace"
BACKUP_DEST="s3://streamspace-backups/nfs"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Mount NFS
mkdir -p /mnt/streamspace-nfs
mount -t nfs ${NFS_SERVER}:${NFS_PATH} /mnt/streamspace-nfs

# Sync to S3 (incremental)
aws s3 sync /mnt/streamspace-nfs ${BACKUP_DEST}/${TIMESTAMP}/ \
  --sse AES256 \
  --storage-class STANDARD_IA \
  --exclude "*.tmp" \
  --exclude "*.lock"

# Unmount
umount /mnt/streamspace-nfs
```

---

## Secrets & Configuration

### Secrets Backup

**Export Kubernetes Secrets (Encrypted):**

```bash
#!/bin/bash
# /opt/streamspace/scripts/backup-secrets.sh

set -euo pipefail

NAMESPACE="streamspace"
BACKUP_DIR="/backups/secrets"
S3_BUCKET="s3://streamspace-backups/secrets"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
GPG_RECIPIENT="ops@streamspace.io"

# Export all secrets
kubectl get secrets -n ${NAMESPACE} -o yaml > ${BACKUP_DIR}/secrets_${TIMESTAMP}.yaml

# Encrypt with GPG
gpg --encrypt --recipient ${GPG_RECIPIENT} \
  --output ${BACKUP_DIR}/secrets_${TIMESTAMP}.yaml.gpg \
  ${BACKUP_DIR}/secrets_${TIMESTAMP}.yaml

# Remove unencrypted file
rm ${BACKUP_DIR}/secrets_${TIMESTAMP}.yaml

# Upload to S3
aws s3 cp ${BACKUP_DIR}/secrets_${TIMESTAMP}.yaml.gpg \
  ${S3_BUCKET}/secrets_${TIMESTAMP}.yaml.gpg \
  --sse AES256

echo "[$(date)] Secrets backup completed: secrets_${TIMESTAMP}.yaml.gpg"
```

### Secrets Restore

```bash
#!/bin/bash
# Restore secrets from encrypted backup

BACKUP_FILE=$1  # e.g., secrets_20251126_020000.yaml.gpg
S3_BUCKET="s3://streamspace-backups/secrets"

# Download encrypted backup
aws s3 cp ${S3_BUCKET}/${BACKUP_FILE} /tmp/${BACKUP_FILE}

# Decrypt
gpg --decrypt --output /tmp/secrets.yaml /tmp/${BACKUP_FILE}

# Apply secrets (will overwrite existing)
kubectl apply -f /tmp/secrets.yaml

# Cleanup
rm /tmp/secrets.yaml /tmp/${BACKUP_FILE}

# Restart deployments to pick up new secrets
kubectl rollout restart deployment -n streamspace
```

### HashiCorp Vault Integration (Recommended)

```hcl
# Terraform configuration for Vault secrets backup
resource "vault_policy" "backup" {
  name = "streamspace-backup"
  policy = <<EOT
path "secret/data/streamspace/*" {
  capabilities = ["read", "list"]
}
EOT
}

# Enable versioning for automatic history
resource "vault_mount" "streamspace" {
  path = "secret/streamspace"
  type = "kv"
  options = {
    version = "2"
  }
}
```

---

## Full Disaster Recovery

### DR Scenario: Complete Region Failure

**Prerequisites:**
- Infrastructure-as-Code (Terraform/Pulumi) in version control
- Database snapshots replicated to DR region
- Container images in multi-region registry
- DNS with low TTL (< 5 minutes)

**Recovery Procedure:**

```bash
#!/bin/bash
# /opt/streamspace/scripts/full-dr-recovery.sh

set -euo pipefail

DR_REGION="us-west-2"
PRIMARY_REGION="us-east-1"
S3_BUCKET="s3://streamspace-backups"

echo "=== StreamSpace Full Disaster Recovery ==="
echo "Target Region: ${DR_REGION}"
echo "Started: $(date)"

# 1. Deploy infrastructure in DR region
echo "[Step 1/8] Deploying infrastructure..."
cd /opt/streamspace/terraform
terraform workspace select ${DR_REGION} || terraform workspace new ${DR_REGION}
terraform apply -auto-approve -var="region=${DR_REGION}"

# 2. Restore database
echo "[Step 2/8] Restoring database..."
LATEST_BACKUP=$(aws s3 ls ${S3_BUCKET}/postgres/ --region ${DR_REGION} | sort -r | head -1 | awk '{print $4}')
./restore-db.sh ${LATEST_BACKUP}

# 3. Restore secrets
echo "[Step 3/8] Restoring secrets..."
LATEST_SECRETS=$(aws s3 ls ${S3_BUCKET}/secrets/ --region ${DR_REGION} | sort -r | head -1 | awk '{print $4}')
./restore-secrets.sh ${LATEST_SECRETS}

# 4. Deploy StreamSpace via Helm
echo "[Step 4/8] Deploying StreamSpace..."
helm upgrade --install streamspace ./chart \
  -n streamspace --create-namespace \
  -f values-${DR_REGION}.yaml

# 5. Wait for deployments
echo "[Step 5/8] Waiting for deployments..."
kubectl rollout status deployment/streamspace-api -n streamspace --timeout=300s
kubectl rollout status deployment/streamspace-k8s-agent -n streamspace --timeout=300s

# 6. Restore storage (if applicable)
echo "[Step 6/8] Restoring storage snapshots..."
# This depends on cross-region snapshot replication being enabled
./restore-storage.sh

# 7. Verify health
echo "[Step 7/8] Verifying health..."
kubectl exec -n streamspace deployment/streamspace-api -- wget -qO- http://localhost:8000/health

# Run smoke tests
./smoke-tests.sh

# 8. Update DNS
echo "[Step 8/8] Updating DNS..."
# Update Route53/Cloudflare to point to DR region
aws route53 change-resource-record-sets \
  --hosted-zone-id ${HOSTED_ZONE_ID} \
  --change-batch file://dns-failover.json

echo "=== DR Recovery Complete ==="
echo "Completed: $(date)"
echo ""
echo "Post-recovery checklist:"
echo "[ ] Verify user access"
echo "[ ] Check session creation works"
echo "[ ] Verify VNC connectivity"
echo "[ ] Monitor error rates"
echo "[ ] Notify stakeholders"
```

### DR Scenario: Database Corruption

```bash
#!/bin/bash
# Quick database recovery from corruption

# 1. Stop writes immediately
kubectl scale deployment/streamspace-api -n streamspace --replicas=0

# 2. Identify corruption point from audit logs
kubectl logs -n streamspace deployment/streamspace-api --since=1h | grep -i error

# 3. Find backup before corruption
# (manual step - identify timestamp)

# 4. Restore using PITR if available, or latest clean backup
./restore-db.sh streamspace_20251126_020000.sql.gz

# 5. Restart API
kubectl scale deployment/streamspace-api -n streamspace --replicas=3

# 6. Verify data integrity
kubectl exec -n streamspace deployment/streamspace-api -- \
  psql -c "SELECT COUNT(*) FROM sessions WHERE status = 'running';"
```

---

## Validation & Testing

### Backup Validation (Automated Daily)

```bash
#!/bin/bash
# /opt/streamspace/scripts/validate-backup.sh

set -euo pipefail

S3_BUCKET="s3://streamspace-backups"
REPORT_FILE="/var/log/streamspace/backup-validation-$(date +%Y%m%d).log"

echo "=== Backup Validation Report ===" | tee ${REPORT_FILE}
echo "Date: $(date)" | tee -a ${REPORT_FILE}

# Check PostgreSQL backup exists and is recent
echo -e "\n[PostgreSQL Backups]" | tee -a ${REPORT_FILE}
LATEST_PG=$(aws s3 ls ${S3_BUCKET}/postgres/ | sort -r | head -1)
LATEST_PG_DATE=$(echo ${LATEST_PG} | awk '{print $1}')
echo "Latest: ${LATEST_PG}" | tee -a ${REPORT_FILE}

if [[ $(date -d "${LATEST_PG_DATE}" +%s) -lt $(date -d "yesterday" +%s) ]]; then
  echo "WARNING: PostgreSQL backup is older than 24 hours!" | tee -a ${REPORT_FILE}
  exit 1
fi

# Verify backup can be read
BACKUP_FILE=$(echo ${LATEST_PG} | awk '{print $4}')
aws s3 cp ${S3_BUCKET}/postgres/${BACKUP_FILE} /tmp/validate.sql.gz
pg_restore --list /tmp/validate.sql.gz > /dev/null
echo "Integrity: OK" | tee -a ${REPORT_FILE}
rm /tmp/validate.sql.gz

# Check storage snapshots
echo -e "\n[Storage Snapshots]" | tee -a ${REPORT_FILE}
SNAPSHOT_COUNT=$(kubectl get volumesnapshot -n streamspace --no-headers | wc -l)
echo "Active snapshots: ${SNAPSHOT_COUNT}" | tee -a ${REPORT_FILE}

if [[ ${SNAPSHOT_COUNT} -lt 1 ]]; then
  echo "WARNING: No storage snapshots found!" | tee -a ${REPORT_FILE}
  exit 1
fi

# Check secrets backup
echo -e "\n[Secrets Backups]" | tee -a ${REPORT_FILE}
LATEST_SECRETS=$(aws s3 ls ${S3_BUCKET}/secrets/ | sort -r | head -1)
echo "Latest: ${LATEST_SECRETS}" | tee -a ${REPORT_FILE}

echo -e "\n=== Validation Complete: PASS ===" | tee -a ${REPORT_FILE}
```

### Quarterly DR Drill

**Drill Checklist:**

```markdown
# StreamSpace DR Drill Checklist

**Date**: _______________
**Participants**: _______________
**Drill Type**: [ ] Tabletop  [ ] Partial Restore  [ ] Full DR

## Pre-Drill
- [ ] Notify stakeholders of drill window
- [ ] Confirm backup systems are current
- [ ] Review runbooks with team
- [ ] Set up monitoring for drill environment

## Database Restore Test
- [ ] Download latest backup from S3
- [ ] Restore to isolated database instance
- [ ] Verify row counts match production (within RPO)
- [ ] Test application connectivity to restored DB
- [ ] Document restore time: _______ minutes

## Storage Restore Test
- [ ] Create test namespace
- [ ] Restore PVC from latest snapshot
- [ ] Verify file integrity (checksums)
- [ ] Mount to test pod and validate access
- [ ] Document restore time: _______ minutes

## Secrets Restore Test
- [ ] Decrypt and restore secrets to test namespace
- [ ] Verify all expected secrets present
- [ ] Test application starts with restored secrets

## Full DR Test (Annually)
- [ ] Deploy full stack in DR region
- [ ] Restore all data
- [ ] Run full smoke test suite
- [ ] Verify VNC connectivity
- [ ] Test DNS failover
- [ ] Document total RTO: _______ minutes

## Post-Drill
- [ ] Document lessons learned
- [ ] Update runbooks with improvements
- [ ] Create issues for any gaps found
- [ ] Schedule follow-up for action items

## Sign-off
- Operations Lead: _______________
- Security Lead: _______________
- Date: _______________
```

---

## Cloud Provider Guides

### AWS

**RDS Automated Backups:**
```bash
# Verify backup configuration
aws rds describe-db-instances \
  --db-instance-identifier streamspace-db \
  --query 'DBInstances[0].{BackupRetention:BackupRetentionPeriod,BackupWindow:PreferredBackupWindow}'

# List available snapshots
aws rds describe-db-snapshots \
  --db-instance-identifier streamspace-db \
  --query 'DBSnapshots[*].{ID:DBSnapshotIdentifier,Time:SnapshotCreateTime,Status:Status}'
```

**EBS Snapshots via AWS Backup:**
```bash
# Create backup plan
aws backup create-backup-plan --backup-plan '{
  "BackupPlanName": "streamspace-daily",
  "Rules": [{
    "RuleName": "daily-backup",
    "TargetBackupVaultName": "streamspace-vault",
    "ScheduleExpression": "cron(0 2 * * ? *)",
    "Lifecycle": {"DeleteAfterDays": 30}
  }]
}'
```

### Google Cloud

**Cloud SQL Backups:**
```bash
# List backups
gcloud sql backups list --instance=streamspace-db

# Restore to point in time
gcloud sql instances clone streamspace-db streamspace-db-restored \
  --point-in-time="2025-11-26T10:30:00Z"
```

**Persistent Disk Snapshots:**
```bash
# Create snapshot
gcloud compute disks snapshot streamspace-data \
  --snapshot-names=streamspace-data-$(date +%Y%m%d) \
  --zone=us-central1-a

# Restore from snapshot
gcloud compute disks create streamspace-data-restored \
  --source-snapshot=streamspace-data-20251126 \
  --zone=us-central1-a
```

### Azure

**Azure Database for PostgreSQL:**
```bash
# List backups
az postgres server backup list \
  --resource-group streamspace-rg \
  --server-name streamspace-db

# Restore to point in time
az postgres server restore \
  --resource-group streamspace-rg \
  --name streamspace-db-restored \
  --source-server streamspace-db \
  --restore-point-in-time "2025-11-26T10:30:00Z"
```

---

## Monitoring & Alerts

### Prometheus Alerts

```yaml
# prometheus-alerts.yaml
groups:
  - name: streamspace-backup
    rules:
      - alert: BackupMissing
        expr: time() - streamspace_last_backup_timestamp > 86400
        for: 1h
        labels:
          severity: critical
        annotations:
          summary: "StreamSpace backup older than 24 hours"
          description: "No successful backup in the last 24 hours"

      - alert: BackupFailed
        expr: streamspace_backup_success == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "StreamSpace backup failed"
          description: "Last backup attempt failed"

      - alert: SnapshotRetentionLow
        expr: streamspace_active_snapshots < 3
        for: 1h
        labels:
          severity: warning
        annotations:
          summary: "Low number of storage snapshots"
          description: "Only {{ $value }} snapshots available"
```

### Backup Success Metrics

```bash
# Add to backup scripts to push metrics
curl -X POST http://pushgateway:9091/metrics/job/backup/instance/postgres <<EOF
streamspace_last_backup_timestamp $(date +%s)
streamspace_backup_success 1
streamspace_backup_size_bytes $(stat -f%z ${BACKUP_FILE} 2>/dev/null || stat -c%s ${BACKUP_FILE})
EOF
```

---

## Appendix: Quick Reference

### Emergency Contacts

| Role | Contact | Phone |
|------|---------|-------|
| On-Call Operations | oncall@streamspace.io | +1-xxx-xxx-xxxx |
| Database Admin | dba@streamspace.io | +1-xxx-xxx-xxxx |
| Security Lead | security@streamspace.io | +1-xxx-xxx-xxxx |

### Recovery Time Summary

| Scenario | Expected RTO | Expected RPO |
|----------|-------------|--------------|
| Single pod failure | < 1 minute | 0 |
| Database restore (pg_dump) | 30-60 minutes | 24 hours |
| Database restore (PITR) | 45-90 minutes | 15 minutes |
| Storage restore (snapshot) | 15-30 minutes | 24 hours |
| Full region DR | 2-4 hours | 15-60 minutes |

### Runbook Quick Links

- [Database Backup](#database-backup--restore)
- [Database Restore](#postgresql-restore)
- [Storage Snapshot](#kubernetes-pvc-snapshots)
- [Full DR Recovery](#full-disaster-recovery)
- [Incident Response](./INCIDENT_RESPONSE.md)

---

**Document Maintenance**: Review quarterly or after any DR drill. Update cloud-specific commands when providers change APIs.
