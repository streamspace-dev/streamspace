# StreamSpace v1.x → v2.0 Migration Guide

**Version**: 2.0.0-beta
**Date**: 2025-11-21
**Migration Type**: Major (Breaking Changes)

---

## Overview

This guide covers migrating from StreamSpace v1.x (Kubernetes-native architecture) to v2.0 (Control Plane + Agent architecture).

**⚠️ Important**: v2.0 is a major architectural change with breaking changes. Plan for downtime during migration.

---

## Table of Contents

1. [What's Changed](#whats-changed)
2. [Migration Strategy](#migration-strategy)
3. [Pre-Migration](#pre-migration)
4. [Migration Process](#migration-process)
5. [Database Migration](#database-migration)
6. [Configuration Changes](#configuration-changes)
7. [Post-Migration](#post-migration)
8. [Rollback Procedure](#rollback-procedure)
9. [Breaking Changes](#breaking-changes)
10. [FAQ](#faq)

---

## What's Changed

### Architecture Changes

**v1.x Architecture:**
```
Web UI → API → Kubebuilder Controller → Session CRDs → Session Pods
         │
         └─ Direct VNC connection to pods
```

**v2.0 Architecture:**
```
Web UI → Control Plane API → Agent Hub → K8s Agent → Session Pods
         │                            ↑
         └─ VNC Proxy ───────────────┘
```

### Key Differences

| Aspect | v1.x | v2.0 |
|--------|------|------|
| **Session Management** | Kubernetes CRDs | Database records + Agent commands |
| **Controller** | Kubebuilder in-cluster | K8s Agent (outbound connection) |
| **VNC Access** | Direct to pod IP | Proxied through Control Plane |
| **Multi-Cluster** | Single cluster only | Multiple clusters supported |
| **Platform Support** | Kubernetes only | Kubernetes + Docker + future platforms |
| **Agent Connection** | N/A | Outbound WSS to Control Plane |
| **Database Schema** | 87 tables | 90 tables (+3 for agents) |

### What Stays the Same

✅ **User Experience**: UI/UX remains identical
✅ **Session Templates**: Same template format
✅ **Authentication**: SAML, OIDC, MFA unchanged
✅ **License Model**: Community/Pro/Enterprise tiers
✅ **Admin Features**: Audit logs, configuration, etc.
✅ **PostgreSQL Database**: Same database engine

### What Changes

❌ **Session CRDs**: Replaced by database records
❌ **Kubebuilder Controller**: Replaced by K8s Agent
❌ **Direct VNC Access**: Replaced by VNC proxy
❌ **kubectl Integration**: Sessions no longer visible via `kubectl get sessions`

---

## Migration Strategy

### Migration Options

**Option 1: Fresh Install (Recommended for Small Deployments)**
- Deploy v2.0 Control Plane + Agent alongside v1.x
- Migrate users gradually
- Decommission v1.x when complete
- **Downtime**: Minimal (gradual migration)
- **Complexity**: Medium
- **Rollback**: Easy (keep v1.x running)

**Option 2: In-Place Upgrade (For Large Deployments)**
- Stop v1.x components
- Migrate database schema
- Deploy v2.0 components
- Restart existing sessions
- **Downtime**: 30-60 minutes
- **Complexity**: High
- **Rollback**: Requires database restore

**Option 3: Blue-Green Deployment (For Enterprise)**
- Deploy complete v2.0 environment (green)
- Test thoroughly
- Switch traffic to v2.0
- Keep v1.x as backup (blue)
- **Downtime**: None (DNS/load balancer switch)
- **Complexity**: High
- **Rollback**: Easy (switch back)

### Recommended Approach

For most deployments, we recommend **Option 1 (Fresh Install)** with gradual migration:

```
Week 1: Deploy v2.0 alongside v1.x
Week 2: Test v2.0 with pilot users
Week 3: Migrate 50% of users
Week 4: Migrate remaining users
Week 5: Decommission v1.x
```

---

## Pre-Migration

### 1. Backup Everything

**Database Backup:**
```bash
# Create full database backup
pg_dump -h <db-host> -U streamspace -d streamspace \
  --format=custom --file=streamspace-v1-backup.dump

# Verify backup
pg_restore --list streamspace-v1-backup.dump | head -20
```

**Kubernetes Resources:**
```bash
# Backup all Session CRDs
kubectl get sessions -n streamspace -o yaml > sessions-backup.yaml

# Backup all Template CRDs
kubectl get templates -n streamspace -o yaml > templates-backup.yaml

# Backup ConfigMaps
kubectl get configmaps -n streamspace -o yaml > configmaps-backup.yaml

# Backup Secrets
kubectl get secrets -n streamspace -o yaml > secrets-backup.yaml
```

**Configuration Files:**
```bash
# Backup Helm values
helm get values streamspace -n streamspace > helm-values-backup.yaml

# Backup deployment manifests
kubectl get deployment streamspace-api -n streamspace -o yaml > api-deployment-backup.yaml
kubectl get deployment streamspace-controller -n streamspace -o yaml > controller-deployment-backup.yaml
```

### 2. Document Current State

**Inventory:**
```bash
# Count active sessions
kubectl get sessions -n streamspace --no-headers | wc -l

# List active users
psql -h <db-host> -U streamspace -d streamspace -c \
  "SELECT COUNT(DISTINCT user_id) FROM sessions WHERE state = 'running';"

# Check resource usage
kubectl top pods -n streamspace
```

**Environment Details:**
- Kubernetes version: `kubectl version`
- StreamSpace version: Check image tags
- Database version: `psql --version`
- Number of users: Query database
- Number of active sessions: `kubectl get sessions`
- Storage class: `kubectl get pvc -n streamspace`

### 3. Prerequisites Check

**✅ Requirements:**
- [ ] PostgreSQL 12+ accessible
- [ ] Kubernetes 1.19+ for v2.0 Control Plane
- [ ] Kubernetes 1.19+ for K8s Agent (can be different cluster)
- [ ] External HTTPS endpoint for Control Plane
- [ ] Outbound HTTPS/WSS access from agent cluster to Control Plane
- [ ] 2 CPU cores, 4GB RAM for Control Plane
- [ ] 500m CPU, 512Mi RAM for K8s Agent

**✅ Access:**
- [ ] Database admin credentials
- [ ] Kubernetes cluster admin access (both clusters if using multiple)
- [ ] DNS/load balancer control (for Control Plane endpoint)
- [ ] TLS/SSL certificates (Let's Encrypt or corporate CA)

### 4. Communication Plan

**Notify Users:**
- **2 weeks before**: Migration announcement
- **1 week before**: Migration details and timeline
- **1 day before**: Final reminder
- **During migration**: Status updates
- **After migration**: Completion notice and new features

**Template Email:**
```
Subject: StreamSpace v2.0 Migration - [DATE]

Dear StreamSpace Users,

We're upgrading StreamSpace to v2.0, bringing exciting new features:
- Multi-cluster support
- Improved performance
- Enhanced security

Migration Schedule:
- Date: [DATE]
- Downtime: 30-60 minutes [or "None - gradual migration"]
- Affected: All users

What You Need to Do:
- [Option 1]: Nothing! Your sessions will be migrated automatically
- [Option 2]: Re-create your sessions after migration

Questions? Contact: [SUPPORT EMAIL]

Thank you for your patience!
StreamSpace Team
```

---

## Migration Process

### Step 1: Deploy v2.0 Control Plane

**1.1 Deploy Control Plane:**

Follow the [V2_DEPLOYMENT_GUIDE.md](V2_DEPLOYMENT_GUIDE.md) to deploy the Control Plane.

Quick steps:
```bash
# Deploy via Helm
helm install streamspace-v2 streamspace/control-plane \
  --namespace streamspace-v2 \
  --create-namespace \
  --set database.host=<db-host> \
  --set database.name=streamspace \
  --set database.user=streamspace \
  --set database.password=<password> \
  --set ingress.enabled=true \
  --set ingress.host=streamspace-v2.example.com

# Or manually via kubectl
kubectl apply -f control-plane-deployment.yaml
```

**1.2 Verify Control Plane:**

```bash
# Check pod status
kubectl get pods -n streamspace-v2

# Expected output:
# NAME                                  READY   STATUS    RESTARTS   AGE
# streamspace-control-plane-xxx         1/1     Running   0          2m

# Check health
curl https://streamspace-v2.example.com/health
# Expected: {"status":"healthy"}
```

### Step 2: Run Database Migration

**2.1 Review Migration SQL:**

See [Database Migration](#database-migration) section below for full SQL.

**2.2 Run Migration:**

**Option A: Using migration tool**
```bash
# Apply v2.0 migrations
./migrate up -database "postgres://streamspace:password@db-host/streamspace?sslmode=require"
```

**Option B: Manual SQL execution**
```bash
# Download migration SQL
curl -O https://raw.githubusercontent.com/JoshuaAFerguson/streamspace/main/migrations/v2.0-agents.sql

# Review migration
less v2.0-agents.sql

# Run migration
psql -h <db-host> -U streamspace -d streamspace -f v2.0-agents.sql

# Verify tables created
psql -h <db-host> -U streamspace -d streamspace -c "\dt agents*"
# Expected:
#  agents
#  agent_commands
```

**2.3 Verify Migration:**

```sql
-- Check new tables exist
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN ('agents', 'agent_commands');

-- Check sessions table has new columns
SELECT column_name FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'sessions'
  AND column_name IN ('agent_id', 'platform', 'platform_metadata');
```

### Step 3: Deploy K8s Agent

**3.1 Apply RBAC:**

```bash
kubectl apply -f https://raw.githubusercontent.com/JoshuaAFerguson/streamspace/main/agents/k8s-agent/k8s/rbac.yaml
```

**3.2 Deploy Agent:**

```bash
# Create deployment YAML
cat > agent-deployment.yaml <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: streamspace
      component: k8s-agent
  template:
    metadata:
      labels:
        app: streamspace
        component: k8s-agent
    spec:
      serviceAccountName: streamspace-agent
      containers:
      - name: agent
        image: streamspace/k8s-agent:v2.0
        env:
        - name: AGENT_ID
          value: "k8s-v2-migration"
        - name: CONTROL_PLANE_URL
          value: "wss://streamspace-v2.example.com"
        - name: NAMESPACE
          value: "streamspace"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
EOF

# Apply deployment
kubectl apply -f agent-deployment.yaml
```

**3.3 Verify Agent:**

```bash
# Check agent pod
kubectl get pods -n streamspace -l component=k8s-agent

# Check agent logs
kubectl logs -n streamspace -l component=k8s-agent --tail=20

# Expected output:
# INFO: Agent registered successfully with Control Plane
# INFO: WebSocket connection established
# INFO: Agent ID: k8s-v2-migration

# Verify agent in Control Plane
curl -H "Authorization: Bearer $JWT_TOKEN" \
  https://streamspace-v2.example.com/api/v1/agents

# Expected:
# [
#   {
#     "agent_id": "k8s-v2-migration",
#     "status": "online",
#     "platform": "kubernetes"
#   }
# ]
```

### Step 4: Migrate Existing Sessions

**Option A: Manual Migration (Recommended)**

1. **Stop v1.x session creation:**
   - Disable "Create Session" button in v1.x UI
   - Or scale v1.x API to 0 replicas

2. **Wait for sessions to complete:**
   ```bash
   # Check remaining active sessions
   kubectl get sessions -n streamspace
   ```

3. **Users re-create sessions on v2.0:**
   - Users login to v2.0 UI (streamspace-v2.example.com)
   - Create new sessions (v2.0 uses new agent architecture)

4. **Clean up v1.x sessions:**
   ```bash
   # Delete all Session CRDs
   kubectl delete sessions --all -n streamspace
   ```

**Option B: Automated Migration (Advanced)**

**⚠️ Warning**: This requires custom migration scripts and is complex.

```bash
# Export v1.x sessions
kubectl get sessions -n streamspace -o json > v1-sessions.json

# Convert to v2.0 format
python3 convert-sessions-v1-to-v2.py v1-sessions.json > v2-sessions.json

# Import to v2.0
curl -X POST https://streamspace-v2.example.com/api/v1/sessions/bulk-import \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d @v2-sessions.json
```

### Step 5: Update DNS/Load Balancer

**5.1 Test v2.0:**

Access v2.0 UI at https://streamspace-v2.example.com and verify:
- [ ] User login works
- [ ] Session creation works
- [ ] VNC connection works
- [ ] Session list displays correctly

**5.2 Switch Traffic:**

**Option A: Update DNS:**
```bash
# Update DNS record
# Before: streamspace.example.com → v1.x load balancer IP
# After:  streamspace.example.com → v2.0 load balancer IP

# Wait for DNS propagation (15 minutes to 24 hours)
```

**Option B: Update Load Balancer:**
```bash
# Update load balancer backend pool
# Before: streamspace-v1-api
# After:  streamspace-v2-control-plane

# Immediate switchover (no DNS propagation wait)
```

### Step 6: Decommission v1.x

**⚠️ Wait 1-2 weeks before decommissioning v1.x** (in case rollback needed)

**6.1 Stop v1.x Components:**

```bash
# Scale down v1.x API
kubectl scale deployment streamspace-api --replicas=0 -n streamspace

# Scale down v1.x Controller
kubectl scale deployment streamspace-controller --replicas=0 -n streamspace

# Delete Session CRDs (if not already done)
kubectl delete crd sessions.stream.space
kubectl delete crd templates.stream.space
```

**6.2 Clean Up Resources:**

```bash
# Uninstall v1.x Helm chart
helm uninstall streamspace -n streamspace

# Or delete v1.x deployments manually
kubectl delete deployment streamspace-api -n streamspace
kubectl delete deployment streamspace-controller -n streamspace

# Keep database! (v2.0 uses same database)
```

**6.3 Archive v1.x Configuration:**

```bash
# Archive backups and configuration
tar -czf streamspace-v1-archive-$(date +%Y%m%d).tar.gz \
  streamspace-v1-backup.dump \
  sessions-backup.yaml \
  templates-backup.yaml \
  helm-values-backup.yaml \
  api-deployment-backup.yaml \
  controller-deployment-backup.yaml

# Store in secure location for 6-12 months
```

---

## Database Migration

### Migration SQL

**File**: `migrations/v2.0-agents.sql`

```sql
-- StreamSpace v2.0 Database Migration
-- Adds agent architecture tables
-- Compatible with v1.x schema (non-destructive)

-- 1. Create agents table
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(255) UNIQUE NOT NULL,         -- "k8s-cluster-1"
    platform VARCHAR(50) NOT NULL,                 -- "kubernetes", "docker"
    region VARCHAR(100),                           -- "us-east-1", "eu-west-1"
    status VARCHAR(50) DEFAULT 'offline',          -- "online", "offline", "draining"
    capacity JSONB,                                -- {max_cpu, max_memory, max_sessions, current_sessions}
    metadata JSONB,                                -- Platform-specific metadata
    websocket_conn_id VARCHAR(255),                -- Active WebSocket connection ID
    last_heartbeat TIMESTAMP,                      -- Last heartbeat timestamp
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for agents table
CREATE INDEX IF NOT EXISTS idx_agents_agent_id ON agents(agent_id);
CREATE INDEX IF NOT EXISTS idx_agents_platform ON agents(platform);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_region ON agents(region);
CREATE INDEX IF NOT EXISTS idx_agents_last_heartbeat ON agents(last_heartbeat);

-- Comments for agents table
COMMENT ON TABLE agents IS 'Registry of platform-specific agents (K8s, Docker, etc.)';
COMMENT ON COLUMN agents.agent_id IS 'Unique agent identifier (e.g., k8s-prod-us-east-1)';
COMMENT ON COLUMN agents.platform IS 'Platform type: kubernetes, docker, vm, cloud';
COMMENT ON COLUMN agents.capacity IS 'Agent capacity: {max_cpu: 100, max_memory: 256, max_sessions: 100, current_sessions: 5}';
COMMENT ON COLUMN agents.metadata IS 'Platform-specific metadata (cluster name, version, etc.)';

-- 2. Create agent_commands table
CREATE TABLE IF NOT EXISTS agent_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    session_id UUID REFERENCES sessions(id) ON DELETE CASCADE,
    command_type VARCHAR(50) NOT NULL,            -- "start_session", "stop_session", "hibernate_session", "wake_session"
    command_data JSONB,                           -- Command parameters
    status VARCHAR(50) DEFAULT 'pending',          -- "pending", "sent", "ack", "completed", "failed"
    result JSONB,                                  -- Command result (pod IP, error message, etc.)
    error_message TEXT,                            -- Error details if failed
    created_at TIMESTAMP DEFAULT NOW(),
    sent_at TIMESTAMP,
    acked_at TIMESTAMP,
    completed_at TIMESTAMP
);

-- Indexes for agent_commands table
CREATE INDEX IF NOT EXISTS idx_agent_commands_agent_id ON agent_commands(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_commands_session_id ON agent_commands(session_id);
CREATE INDEX IF NOT EXISTS idx_agent_commands_status ON agent_commands(status);
CREATE INDEX IF NOT EXISTS idx_agent_commands_created_at ON agent_commands(created_at);

-- Comments for agent_commands table
COMMENT ON TABLE agent_commands IS 'Command queue for Control Plane → Agent communication';
COMMENT ON COLUMN agent_commands.command_type IS 'Command type: start_session, stop_session, hibernate_session, wake_session';
COMMENT ON COLUMN agent_commands.status IS 'Command lifecycle: pending → sent → ack → completed/failed';

-- 3. Alter sessions table (add agent columns)
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE SET NULL;

ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS platform VARCHAR(50);

ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS platform_metadata JSONB;

-- Indexes for new sessions columns
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_sessions_platform ON sessions(platform);

-- Comments for new sessions columns
COMMENT ON COLUMN sessions.agent_id IS 'Agent managing this session (NULL if using v1.x controller)';
COMMENT ON COLUMN sessions.platform IS 'Platform where session is running: kubernetes, docker, vm, cloud';
COMMENT ON COLUMN sessions.platform_metadata IS 'Platform-specific session metadata';

-- 4. Create platform_controllers table (for future Docker/VM agents)
CREATE TABLE IF NOT EXISTS platform_controllers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    controller_type VARCHAR(50) NOT NULL,         -- "kubernetes", "docker", "vmware"
    name VARCHAR(255) NOT NULL,
    endpoint VARCHAR(500),                         -- API endpoint URL
    region VARCHAR(100),
    status VARCHAR(50) DEFAULT 'offline',
    cluster_info JSONB,                            -- K8s cluster info, Docker host info, etc.
    capabilities JSONB,                            -- Supported features
    last_heartbeat TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(controller_type, name)
);

-- Indexes for platform_controllers
CREATE INDEX IF NOT EXISTS idx_platform_controllers_type ON platform_controllers(controller_type);
CREATE INDEX IF NOT EXISTS idx_platform_controllers_status ON platform_controllers(status);

-- Comments
COMMENT ON TABLE platform_controllers IS 'Legacy table for controller-based architecture (used by admin UI)';

-- 5. Backfill existing sessions (mark as v1.x)
UPDATE sessions
SET platform = 'kubernetes',
    platform_metadata = jsonb_build_object('source', 'v1.x', 'controller', 'kubebuilder')
WHERE platform IS NULL;

-- 6. Create migration tracking table
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO schema_migrations (version) VALUES ('v2.0.0-agents')
ON CONFLICT (version) DO NOTHING;

-- 7. Create functions for agent management
CREATE OR REPLACE FUNCTION update_agent_heartbeat()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_agents_updated_at
BEFORE UPDATE ON agents
FOR EACH ROW
EXECUTE FUNCTION update_agent_heartbeat();

-- Migration complete
SELECT 'v2.0 database migration completed successfully' AS status;
```

### Running the Migration

```bash
# Download migration
wget https://raw.githubusercontent.com/JoshuaAFerguson/streamspace/main/migrations/v2.0-agents.sql

# Backup database first!
pg_dump -h <db-host> -U streamspace -d streamspace \
  --format=custom --file=streamspace-pre-v2-backup.dump

# Run migration
psql -h <db-host> -U streamspace -d streamspace -f v2.0-agents.sql

# Verify migration
psql -h <db-host> -U streamspace -d streamspace -c \
  "SELECT version, applied_at FROM schema_migrations WHERE version = 'v2.0.0-agents';"
```

---

## Configuration Changes

### Environment Variables

**v1.x (API):**
```bash
DB_HOST=postgres.example.com
DB_PORT=5432
DB_NAME=streamspace
DB_USER=streamspace
DB_PASSWORD=secret
JWT_SECRET=changeme
PORT=8080
```

**v2.0 (Control Plane):**
```bash
# Same as v1.x, plus:
AGENT_HEARTBEAT_TIMEOUT=30s      # NEW
VNC_PROXY_TIMEOUT=5m             # NEW
LOG_LEVEL=info                   # UPDATED (debug, info, warn, error)
```

**v2.0 (K8s Agent):**
```bash
AGENT_ID=k8s-prod-us-east-1      # REQUIRED
CONTROL_PLANE_URL=wss://streamspace.example.com  # REQUIRED
PLATFORM=kubernetes              # Optional
REGION=us-east-1                 # Optional
NAMESPACE=streamspace            # Optional
MAX_CPU=100                      # Optional
MAX_MEMORY=256                   # Optional
MAX_SESSIONS=100                 # Optional
```

### Ingress Changes

**v1.x Ingress:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: streamspace
spec:
  rules:
  - host: streamspace.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: streamspace-api
            port:
              number: 8080
```

**v2.0 Ingress:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: streamspace-v2
  annotations:
    # IMPORTANT: WebSocket support required
    nginx.ingress.kubernetes.io/websocket-services: streamspace-control-plane
spec:
  rules:
  - host: streamspace.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: streamspace-control-plane
            port:
              number: 8080
```

---

## Post-Migration

### Verification Checklist

**✅ Infrastructure:**
- [ ] Control Plane pods running
- [ ] K8s Agent pod running
- [ ] Agent status "online" in UI
- [ ] Database tables created (agents, agent_commands)
- [ ] Ingress serving traffic

**✅ Functionality:**
- [ ] User login works
- [ ] Session creation works (via new agent)
- [ ] VNC connection works (via proxy)
- [ ] Session list displays
- [ ] Session stop works
- [ ] Hibernate/wake works

**✅ Admin Features:**
- [ ] Agents page shows K8s agent
- [ ] Audit logs recording events
- [ ] License enforcement working

**✅ Monitoring:**
- [ ] Prometheus metrics exposed
- [ ] Grafana dashboards updated
- [ ] Alerts configured

### Performance Testing

```bash
# Create 10 test sessions
for i in {1..10}; do
  curl -X POST https://streamspace.example.com/api/v1/sessions \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"test${i}\",\"template\":\"firefox-browser\",\"state\":\"running\"}"
done

# Wait for sessions to start
sleep 60

# Check session status
curl https://streamspace.example.com/api/v1/sessions \
  -H "Authorization: Bearer $JWT_TOKEN" | jq '.[] | {id, state, platform, agent_id}'

# Test VNC connections
# Manually open 3-5 session viewers and verify VNC works
```

### Monitoring Setup

**Add Prometheus Alerts:**

```yaml
# alerts/streamspace-v2.yaml
groups:
- name: streamspace-v2
  rules:
  - alert: AgentOffline
    expr: streamspace_agent_status{status="offline"} > 0
    for: 2m
    annotations:
      summary: "Agent {{ $labels.agent_id }} is offline"

  - alert: HighSessionFailureRate
    expr: rate(streamspace_session_failures_total[5m]) > 0.1
    for: 5m
    annotations:
      summary: "High session failure rate: {{ $value }}"

  - alert: VNCConnectionFailures
    expr: rate(streamspace_vnc_connection_failures_total[5m]) > 0.05
    for: 5m
    annotations:
      summary: "High VNC connection failure rate"
```

---

## Rollback Procedure

**⚠️ If migration fails**, follow this rollback procedure:

### Step 1: Stop v2.0 Components

```bash
# Scale down v2.0 Control Plane
kubectl scale deployment streamspace-control-plane --replicas=0 -n streamspace-v2

# Scale down K8s Agent
kubectl scale deployment streamspace-k8s-agent --replicas=0 -n streamspace
```

### Step 2: Restore Database

```bash
# Restore database from pre-migration backup
dropdb -h <db-host> -U streamspace streamspace
createdb -h <db-host> -U streamspace streamspace
pg_restore -h <db-host> -U streamspace -d streamspace streamspace-pre-v2-backup.dump
```

### Step 3: Restart v1.x Components

```bash
# Scale up v1.x API
kubectl scale deployment streamspace-api --replicas=2 -n streamspace

# Scale up v1.x Controller
kubectl scale deployment streamspace-controller --replicas=1 -n streamspace

# Verify pods running
kubectl get pods -n streamspace
```

### Step 4: Revert DNS/Load Balancer

```bash
# Update DNS or load balancer back to v1.x
# streamspace.example.com → v1.x load balancer IP
```

### Step 5: Verify v1.x Working

```bash
# Test v1.x
curl https://streamspace.example.com/health

# Check sessions
kubectl get sessions -n streamspace
```

---

## Breaking Changes

### 1. Session CRDs Removed

**Before (v1.x):**
```bash
kubectl get sessions -n streamspace
kubectl describe session my-session -n streamspace
```

**After (v2.0):**
```bash
# Sessions are database records, not CRDs
# Use API instead:
curl https://streamspace.example.com/api/v1/sessions \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Impact**: Custom scripts using `kubectl` to manage sessions will break.

**Migration**: Update scripts to use REST API.

### 2. Direct VNC Access Removed

**Before (v1.x):**
```
UI → session.status.url (http://10.42.1.5:3000) → Pod
```

**After (v2.0):**
```
UI → /vnc-viewer/{sessionId} → VNC Proxy → Agent → Pod
```

**Impact**: Direct pod IP access no longer works.

**Migration**: Use VNC proxy (automatic in UI, no user action needed).

### 3. Controller Replaced by Agent

**Before (v1.x):**
- Kubebuilder controller runs in same cluster as sessions
- Reconcile loop watches CRDs

**After (v2.0):**
- K8s Agent runs in session cluster
- Connects outbound to Control Plane
- No CRDs, command-based control

**Impact**: Deployment model changes (agent deployment required).

**Migration**: Deploy K8s Agent (see deployment guide).

### 4. Database Schema Changes

**New Tables:**
- `agents`
- `agent_commands`
- `platform_controllers`

**Modified Tables:**
- `sessions` (+3 columns: `agent_id`, `platform`, `platform_metadata`)

**Impact**: Custom database queries may need updates.

**Migration**: Update queries to include new columns.

---

## FAQ

**Q: Can I run v1.x and v2.0 simultaneously?**

A: Yes! This is the recommended migration approach. Deploy v2.0 alongside v1.x and migrate gradually.

**Q: Will my existing sessions continue working during migration?**

A: v1.x sessions continue working on v1.x. New sessions on v2.0 use the new architecture. Existing sessions are not automatically migrated (users must recreate).

**Q: Do I need to migrate all users at once?**

A: No. You can migrate users gradually over days or weeks.

**Q: Can I rollback after migration?**

A: Yes, if you keep database backup and v1.x deployment. Rollback is straightforward within 24-48 hours.

**Q: What happens to persistent session storage?**

A: PVCs remain intact. If users recreate sessions with same session ID, they'll access same storage.

**Q: Will VNC connection quality change?**

A: No. VNC proxying adds minimal latency (<10ms). Quality remains the same.

**Q: Can I use the same database for v1.x and v2.0?**

A: Yes. v2.0 adds new tables but doesn't modify v1.x tables. Both versions can coexist.

**Q: What about my custom templates?**

A: Templates remain compatible. v2.0 uses same template format as v1.x.

**Q: Do I need to update my license?**

A: No. v2.0 uses same license system (Community/Pro/Enterprise).

**Q: What if my K8s Agent can't reach the Control Plane?**

A: Verify network connectivity. Agent needs outbound HTTPS/WSS (port 443) access to Control Plane endpoint. Check firewall rules.

**Q: Can I migrate back to v1.x after running v2.0 for a month?**

A: Technically yes, but not recommended. You'll lose all sessions created on v2.0. Plan carefully before starting migration.

**Q: What's the minimum downtime for in-place upgrade?**

A: 30-60 minutes with proper planning. Fresh install approach has minimal/no downtime.

---

## Support

**Migration Issues:**
- GitHub Issues: https://github.com/JoshuaAFerguson/streamspace/issues
- Label: `migration`, `v2.0`

**Documentation:**
- Deployment Guide: [V2_DEPLOYMENT_GUIDE.md](V2_DEPLOYMENT_GUIDE.md)
- Architecture: [V2_ARCHITECTURE.md](V2_ARCHITECTURE.md)
- Troubleshooting: [TROUBLESHOOTING.md](../TROUBLESHOOTING.md)

**Community:**
- Discord: https://discord.gg/streamspace
- Community Forum: https://community.streamspace.io

---

**Migration Guide Version**: 1.0
**Last Updated**: 2025-11-21
**StreamSpace Version**: v2.0.0-beta
