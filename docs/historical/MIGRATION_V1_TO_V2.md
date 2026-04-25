# StreamSpace v1.x → v2.0 Migration Guide

> **Status**: Production Ready
> **Version**: v2.0-beta.1
> **Last Updated**: 2025-11-22
> **Migration Guide Version**: 1.1

---

## Executive Summary

This guide covers migrating from **StreamSpace v1.x** (Kubebuilder controller-based) to **StreamSpace v2.0** (Control Plane + Multi-Platform Agent architecture with High Availability).

**Key Changes in v2.0**:
- ✅ **Multi-Platform Architecture**: Control Plane + Agents (Kubernetes, Docker)
- ✅ **High Availability**: Redis-backed AgentHub, K8s Leader Election, Docker Agent HA
- ✅ **End-to-End VNC Proxy**: Secure firewall-friendly VNC tunneling
- ✅ **Database-Driven Sessions**: Replaces Kubernetes CRDs
- ✅ **WebSocket-Based Agent Communication**: Replaces watch-based reconciliation
- ✅ **Scalable API**: 2-10 pod replicas with Redis-backed agent connections
- ✅ **Agent Failover**: 23-second reconnection with 100% session survival

**Migration Timeline**:
- **Small Deployments** (<50 users): 4-8 hours
- **Medium Deployments** (50-500 users): 1-2 days
- **Large Deployments** (500+ users): 3-5 days

**Recommended Approach**: **Blue-Green Deployment** (deploy v2.0 alongside v1.x, migrate gradually)

---

## Who Should Migrate?

### Migrate to v2.0 if you:
- ✅ Want multi-platform support (Kubernetes + Docker + future VM/Cloud)
- ✅ Need high availability (multi-pod API, agent failover)
- ✅ Run sessions in firewall-restricted environments (VNC proxy required)
- ✅ Want centralized control across multiple K8s clusters
- ✅ Need production-grade scalability (2-10 API replicas, 3-10 agent replicas)

### Stay on v1.x if you:
- ⚠️ Only use single Kubernetes cluster with no HA requirements
- ⚠️ Happy with direct pod IP VNC access (no proxy needed)
- ⚠️ Don't need multi-platform support
- ⚠️ Running simple development/testing environment

---

## Migration Overview

### Architecture Changes

**v1.x Architecture:**
```
User → Traefik Ingress → API (1 pod) → PostgreSQL
                        ↓
                   Controller (1 pod, watches CRDs)
                        ↓
                   Session Pods (direct VNC access)
```

**v2.0 Architecture:**
```
User → Ingress → Control Plane API (2-10 pods) → Redis AgentHub → PostgreSQL
                        ↓ WebSocket
               K8s Agent (3-10 pods, leader election)
                        ↓ kubectl
                   Session Pods
                        ↑ VNC Tunnel
User → /vnc-viewer/{id} → VNC Proxy → Agent → Pod
```

### High-Level Changes

| Component | v1.x | v2.0 | Impact |
|-----------|------|------|--------|
| **API** | Single pod | 2-10 pods | HA, scalability |
| **Agent Hub** | N/A | Redis-backed | Distributed agent connections |
| **Session Mgmt** | CRDs | Database + Commands | Breaking change |
| **VNC Access** | Direct Pod IP | Proxy via Agent | Breaking change |
| **Controller** | Kubebuilder (1 pod) | K8s Agent (3-10 pods) | HA, failover |
| **Platforms** | Kubernetes only | K8s + Docker | Multi-platform |
| **Communication** | Watch CRDs | WebSocket commands | New protocol |
| **Failover** | Manual restart | Auto-reconnect (23s) | HA |

---

## Pre-Migration Checklist

### 1. Backup Everything

**1.1 Database Backup:**

```bash
# Full database backup
pg_dump -h <db-host> -U streamspace -d streamspace \
  --format=custom --file=streamspace-v1-backup-$(date +%Y%m%d).dump

# Verify backup
pg_restore --list streamspace-v1-backup-$(date +%Y%m%d).dump | head -20
```

**1.2 Session CRD Backup:**

```bash
# Export all Session CRDs
kubectl get sessions -n streamspace -o yaml > sessions-backup-$(date +%Y%m%d).yaml

# Export all Template CRDs
kubectl get templates -n streamspace -o yaml > templates-backup-$(date +%Y%m%d).yaml

# Verify exports
wc -l sessions-backup-*.yaml templates-backup-*.yaml
```

**1.3 Configuration Backup:**

```bash
# Helm values
helm get values streamspace -n streamspace > helm-values-backup.yaml

# Deployments
kubectl get deployment streamspace-api -n streamspace -o yaml > api-deployment-backup.yaml
kubectl get deployment streamspace-controller -n streamspace -o yaml > controller-deployment-backup.yaml

# Secrets
kubectl get secret streamspace-secrets -n streamspace -o yaml > secrets-backup.yaml
```

### 2. Verify v1.x Status

```bash
# Check all components healthy
kubectl get pods -n streamspace
kubectl get sessions -n streamspace

# Check database connectivity
psql -h <db-host> -U streamspace -d streamspace -c "SELECT COUNT(*) FROM sessions;"

# Document current session count
ACTIVE_SESSIONS=$(kubectl get sessions -n streamspace --no-headers | wc -l)
echo "Active sessions: $ACTIVE_SESSIONS"
```

### 3. Resource Requirements

**Control Plane (API):**
- **Minimum**: 2 pods × (512Mi RAM, 500m CPU)
- **Recommended**: 4 pods × (1Gi RAM, 1 CPU)
- **Large Scale**: 10 pods × (2Gi RAM, 2 CPU)

**Redis (AgentHub):**
- **Minimum**: 256Mi RAM, 100m CPU
- **Recommended**: 512Mi RAM, 250m CPU
- **Persistence**: Optional (reconnection state only)

**K8s Agent:**
- **Minimum**: 3 pods × (256Mi RAM, 250m CPU) - for HA
- **Recommended**: 5 pods × (512Mi RAM, 500m CPU)

**Docker Agent (if using Docker platform):**
- **Minimum**: 3 instances × (256Mi RAM, 250m CPU) - for HA
- **Recommended**: 5 instances × (512Mi RAM, 500m CPU)

### 4. Network Requirements

**Inbound:**
- HTTPS/WSS (443): User → Control Plane
- HTTPS (443): Agent → Control Plane WebSocket

**Outbound:**
- PostgreSQL (5432): Control Plane → Database
- Redis (6379): Control Plane → Redis (if multi-pod)
- VNC (varies): VNC Proxy → Session Pods

---

## Migration Strategies

### Strategy 1: Fresh Install (Recommended for Small Deployments)

**Pros**:
- ✅ Clean slate, no conflicts
- ✅ Easy rollback (keep v1.x running)
- ✅ Test v2.0 before switching traffic

**Cons**:
- ⚠️ Users must recreate sessions

**Steps**:
1. Deploy v2.0 in new namespace (`streamspace-v2`)
2. Test thoroughly
3. Switch DNS/load balancer
4. Users recreate sessions on v2.0
5. Decommission v1.x after 1-2 weeks

**Best For**: <50 users, can tolerate session recreation

---

### Strategy 2: In-Place Upgrade (For Experienced Teams)

**Pros**:
- ✅ Same namespace/database
- ✅ Faster migration

**Cons**:
- ⚠️ Requires downtime (30-60 minutes)
- ⚠️ Higher rollback complexity

**Steps**:
1. Announce maintenance window
2. Stop v1.x API/Controller
3. Run database migration
4. Deploy v2.0 Control Plane + Agent
5. Verify, resume traffic

**Best For**: Teams comfortable with database migrations, can schedule downtime

---

### Strategy 3: Blue-Green Deployment (Recommended for Production)

**Pros**:
- ✅ Zero downtime
- ✅ Easy rollback
- ✅ Gradual user migration
- ✅ Test under real load

**Cons**:
- ⚠️ Requires 2x infrastructure temporarily

**Steps**:
1. Deploy v2.0 Control Plane + Agent in new namespace
2. Run database migration (non-destructive, adds tables)
3. Deploy v2.0 at `streamspace-v2.example.com`
4. Migrate users gradually (pilot → beta → full)
5. Switch DNS once validated
6. Decommission v1.x after 1-2 weeks

**Best For**: Production deployments, large user bases, risk-averse teams

---

## Step-by-Step Migration (Blue-Green Approach)

### Step 1: Deploy v2.0 Control Plane

**1.1 Create Namespace:**

```bash
kubectl create namespace streamspace-v2
```

**1.2 Install Redis (for multi-pod HA):**

```bash
# Using Helm
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install redis bitnami/redis \
  --namespace streamspace-v2 \
  --set auth.enabled=false \
  --set master.persistence.enabled=false \
  --set master.resources.requests.memory=256Mi \
  --set master.resources.requests.cpu=100m

# Wait for Redis
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=redis -n streamspace-v2 --timeout=120s

# Verify Redis
kubectl exec -n streamspace-v2 redis-master-0 -- redis-cli ping
# Expected: PONG
```

**1.3 Create ConfigMap:**

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: streamspace-v2-config
  namespace: streamspace-v2
data:
  DB_HOST: "postgres.streamspace.svc.cluster.local"
  DB_PORT: "5432"
  DB_NAME: "streamspace"
  LOG_LEVEL: "info"
  AGENT_HEARTBEAT_TIMEOUT: "30s"
  VNC_PROXY_TIMEOUT: "5m"
  REDIS_URL: "redis://redis-master.streamspace-v2.svc.cluster.local:6379"
  AGENT_HUB_BACKEND: "redis"
EOF
```

**1.4 Create Secret:**

```bash
kubectl create secret generic streamspace-v2-secrets \
  --from-literal=DB_USER=streamspace \
  --from-literal=DB_PASSWORD=<your-db-password> \
  --from-literal=JWT_SECRET=<your-jwt-secret> \
  -n streamspace-v2
```

**1.5 Deploy Control Plane API (Multi-Pod HA):**

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-control-plane
  namespace: streamspace-v2
spec:
  replicas: 4  # HA: 2-10 replicas recommended
  selector:
    matchLabels:
      app: streamspace
      component: control-plane
  template:
    metadata:
      labels:
        app: streamspace
        component: control-plane
    spec:
      containers:
      - name: api
        image: streamspace/control-plane:v2.0-beta.1
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 8081
          name: websocket
        envFrom:
        - configMapRef:
            name: streamspace-v2-config
        - secretRef:
            name: streamspace-v2-secrets
        resources:
          requests:
            memory: "1Gi"
            cpu: "1000m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: streamspace-control-plane
  namespace: streamspace-v2
spec:
  selector:
    app: streamspace
    component: control-plane
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: websocket
    port: 8081
    targetPort: 8081
  type: ClusterIP
EOF
```

**1.6 Verify Control Plane:**

```bash
# Check pods (should see 4 replicas)
kubectl get pods -n streamspace-v2 -l component=control-plane

# Check logs
kubectl logs -n streamspace-v2 -l component=control-plane --tail=20

# Expected output:
# INFO: Server started on :8080
# INFO: Connected to database
# INFO: Redis AgentHub initialized (multi-pod mode)
```

---

### Step 2: Run Database Migration

**2.1 Download Migration:**

```bash
# Create migrations directory
mkdir -p migrations

# Download migration SQL
wget https://raw.githubusercontent.com/streamspace-dev/streamspace/main/migrations/v2.0-agents.sql \
  -O migrations/v2.0-agents.sql

# Download v2.0-beta.1 additions
wget https://raw.githubusercontent.com/streamspace-dev/streamspace/main/migrations/v2.0-beta1-additions.sql \
  -O migrations/v2.0-beta1-additions.sql
```

**2.2 Run Migration:**

```bash
# Run base v2.0 migration
psql -h <db-host> -U streamspace -d streamspace -f migrations/v2.0-agents.sql

# Run v2.0-beta.1 additions
psql -h <db-host> -U streamspace -d streamspace -f migrations/v2.0-beta1-additions.sql
```

**2.3 Verify Migration:**

```bash
# Check new tables created
psql -h <db-host> -U streamspace -d streamspace -c "\dt" | grep -E "agents|agent_commands"

# Check new columns added to sessions
psql -h <db-host> -U streamspace -d streamspace -c "\d sessions" | grep -E "agent_id|platform|cluster_id|tags"

# Check migration version
psql -h <db-host> -U streamspace -d streamspace -c \
  "SELECT version, applied_at FROM schema_migrations WHERE version LIKE 'v2.0%';"

# Expected:
#     version           |        applied_at
# ----------------------+---------------------------
#  v2.0.0-agents        | 2025-11-22 10:30:00
#  v2.0.0-beta1-ha      | 2025-11-22 10:30:05
```

---

### Step 3: Deploy K8s Agent (with High Availability)

**3.1 Create RBAC:**

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: streamspace-agent
  namespace: streamspace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: streamspace-agent
  namespace: streamspace
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log", "pods/status"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["pods/portforward"]  # NEW in v2.0-beta.1
  verbs: ["get", "list", "create"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "create", "delete"]
- apiGroups: ["stream.space"]
  resources: ["templates", "templates/status"]  # NEW in v2.0-beta.1
  verbs: ["get", "list", "watch"]
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]  # NEW in v2.0-beta.1 - for leader election
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: streamspace-agent
  namespace: streamspace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: streamspace-agent
subjects:
- kind: ServiceAccount
  name: streamspace-agent
  namespace: streamspace
EOF
```

**3.2 Deploy K8s Agent (HA with Leader Election):**

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
  labels:
    app: streamspace
    component: k8s-agent
spec:
  replicas: 5  # HA: 3-10 replicas recommended for production
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
        image: streamspace/k8s-agent:v2.0-beta.1
        env:
        - name: AGENT_ID
          value: "k8s-prod-cluster"
        - name: CONTROL_PLANE_URL
          value: "ws://streamspace-control-plane.streamspace-v2.svc.cluster.local:8081"
        - name: PLATFORM
          value: "kubernetes"
        - name: NAMESPACE
          value: "streamspace"
        - name: ENABLE_HA
          value: "true"  # NEW in v2.0-beta.1
        - name: LEASE_LOCK_NAME
          value: "k8s-agent-leader"  # NEW
        - name: LEASE_LOCK_NAMESPACE
          value: "streamspace"  # NEW
        - name: LEASE_DURATION
          value: "15s"  # NEW
        - name: RENEW_DEADLINE
          value: "10s"  # NEW
        - name: RETRY_PERIOD
          value: "2s"  # NEW
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8082
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8082
          initialDelaySeconds: 10
          periodSeconds: 5
EOF
```

**3.3 Verify K8s Agent (HA Setup):**

```bash
# Check agent pods (should see 5 replicas)
kubectl get pods -n streamspace -l component=k8s-agent

# Expected:
# NAME                                    READY   STATUS    RESTARTS   AGE
# streamspace-k8s-agent-7c8f9d6b5-abc12   1/1     Running   0          2m
# streamspace-k8s-agent-7c8f9d6b5-def34   1/1     Running   0          2m
# streamspace-k8s-agent-7c8f9d6b5-ghi56   1/1     Running   0          2m
# streamspace-k8s-agent-7c8f9d6b5-jkl78   1/1     Running   0          2m
# streamspace-k8s-agent-7c8f9d6b5-mno90   1/1     Running   0          2m

# Check leader election lease
kubectl get lease k8s-agent-leader -n streamspace

# Expected:
# NAME                HOLDER                                          AGE
# k8s-agent-leader    streamspace-k8s-agent-7c8f9d6b5-abc12_<uuid>    2m

# Check agent logs (leader will show election message)
kubectl logs -n streamspace -l component=k8s-agent --tail=30 | grep -E "leader|election"

# Expected from leader pod:
# INFO: Successfully acquired leader lease
# INFO: This agent is the LEADER
# INFO: Agent registered successfully with Control Plane

# Expected from follower pods:
# INFO: Attempting to acquire leader lease
# INFO: This agent is a FOLLOWER (leader: streamspace-k8s-agent-7c8f9d6b5-abc12)

# Verify agent in Control Plane
kubectl port-forward -n streamspace-v2 svc/streamspace-control-plane 8080:8080 &
JWT_TOKEN=$(curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r .token)

curl -H "Authorization: Bearer $JWT_TOKEN" \
  http://localhost:8080/api/v1/agents

# Expected:
# [
#   {
#     "agent_id": "k8s-prod-cluster",
#     "status": "online",
#     "platform": "kubernetes",
#     "region": null,
#     "capacity": {
#       "max_sessions": 100,
#       "current_sessions": 0
#     },
#     "last_heartbeat": "2025-11-22T10:35:00Z"
#   }
# ]
```

---

### Step 4: Deploy Docker Agent (Optional - Multi-Platform Support)

**4.1 Install Docker Agent (with HA):**

```bash
# Download Docker Agent binary
wget https://github.com/streamspace-dev/streamspace/releases/download/v2.0-beta.1/docker-agent-linux-amd64
chmod +x docker-agent-linux-amd64
sudo mv docker-agent-linux-amd64 /usr/local/bin/streamspace-docker-agent

# Create systemd service (Instance 1 - Leader Election Backend: Redis)
sudo tee /etc/systemd/system/streamspace-docker-agent.service > /dev/null <<EOF
[Unit]
Description=StreamSpace Docker Agent
After=docker.service redis.service
Requires=docker.service

[Service]
Type=simple
User=streamspace
Group=docker
Environment="AGENT_ID=docker-host-01"
Environment="CONTROL_PLANE_URL=wss://streamspace-v2.example.com"
Environment="PLATFORM=docker"
Environment="REGION=us-east-1"
Environment="ENABLE_HA=true"
Environment="HA_BACKEND=redis"
Environment="REDIS_URL=redis://localhost:6379"
Environment="LEASE_DURATION=15s"
ExecStart=/usr/local/bin/streamspace-docker-agent
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Start Docker Agent
sudo systemctl daemon-reload
sudo systemctl enable streamspace-docker-agent
sudo systemctl start streamspace-docker-agent

# Verify Docker Agent
sudo systemctl status streamspace-docker-agent

# Check logs
sudo journalctl -u streamspace-docker-agent -f

# Expected:
# INFO: Docker Agent starting (HA mode: redis)
# INFO: Connected to Docker daemon
# INFO: Successfully acquired leader lease via Redis
# INFO: This agent is the LEADER
# INFO: Agent registered successfully with Control Plane
# INFO: WebSocket connection established
# INFO: Agent ID: docker-host-01
```

**4.2 Deploy Additional Docker Agent Instances (HA):**

```bash
# On additional Docker hosts, deploy with different AGENT_ID
# Instance 2:
Environment="AGENT_ID=docker-host-02"
Environment="ENABLE_HA=true"
Environment="HA_BACKEND=redis"
Environment="REDIS_URL=redis://shared-redis.example.com:6379"

# Instance 3:
Environment="AGENT_ID=docker-host-03"
...

# Verify all instances
curl -H "Authorization: Bearer $JWT_TOKEN" \
  http://localhost:8080/api/v1/agents | jq '.[] | select(.platform=="docker")'

# Expected: Multiple Docker agents, one as leader
```

---

### Step 5: Migrate Existing Sessions

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

---

### Step 6: Update DNS/Load Balancer

**6.1 Test v2.0:**

Access v2.0 UI at https://streamspace-v2.example.com and verify:
- [ ] User login works
- [ ] Session creation works (both K8s and Docker platforms)
- [ ] VNC connection works
- [ ] Session list displays correctly
- [ ] Multi-pod API handles requests (check different pod logs)
- [ ] Agent failover works (kill leader pod, verify follower takes over)

**6.2 Switch Traffic:**

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

---

### Step 7: Decommission v1.x

**⚠️ Wait 1-2 weeks before decommissioning v1.x** (in case rollback needed)

**7.1 Stop v1.x Components:**

```bash
# Scale down v1.x API
kubectl scale deployment streamspace-api --replicas=0 -n streamspace

# Scale down v1.x Controller
kubectl scale deployment streamspace-controller --replicas=0 -n streamspace

# Delete Session CRDs (if not already done)
kubectl delete crd sessions.stream.space
kubectl delete crd templates.stream.space
```

**7.2 Clean Up Resources:**

```bash
# Uninstall v1.x Helm chart
helm uninstall streamspace -n streamspace

# Or delete v1.x deployments manually
kubectl delete deployment streamspace-api -n streamspace
kubectl delete deployment streamspace-controller -n streamspace

# Keep database! (v2.0 uses same database)
```

**7.3 Archive v1.x Configuration:**

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

### Migration SQL - Base v2.0 Tables

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
    retry_count INT DEFAULT 0,                     -- Number of retries attempted
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
SELECT 'v2.0 base migration completed successfully' AS status;
```

### Migration SQL - v2.0-beta.1 Additions (HA + Bug Fixes)

**File**: `migrations/v2.0-beta1-additions.sql`

```sql
-- StreamSpace v2.0-beta.1 Database Migration
-- Adds High Availability features and bug fixes from Waves 10-17
-- Run AFTER v2.0-agents.sql

-- 1. Add cluster_id column to agents (for multi-cluster HA)
ALTER TABLE agents
ADD COLUMN IF NOT EXISTS cluster_id VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_agents_cluster_id ON agents(cluster_id);

COMMENT ON COLUMN agents.cluster_id IS 'Cluster identifier for multi-cluster deployments';

-- 2. Add tags column to sessions (for filtering and organization)
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_sessions_tags ON sessions USING gin(tags);

COMMENT ON COLUMN sessions.tags IS 'Session tags for filtering and organization';

-- 3. Add active_sessions column to agents (for capacity tracking)
ALTER TABLE agents
ADD COLUMN IF NOT EXISTS active_sessions INT DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_agents_active_sessions ON agents(active_sessions);

COMMENT ON COLUMN agents.active_sessions IS 'Current number of active sessions on this agent';

-- 4. Update websocket_conn_id to allow NULL (agents can be offline)
-- Already nullable, but add comment for clarity
COMMENT ON COLUMN agents.websocket_conn_id IS 'Current WebSocket connection ID (NULL if offline)';

-- 5. Create redis_agent_connections table (for Redis-backed AgentHub)
CREATE TABLE IF NOT EXISTS redis_agent_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(255) NOT NULL,
    pod_id VARCHAR(255) NOT NULL,                 -- Control Plane pod handling this connection
    websocket_conn_id VARCHAR(255) NOT NULL,
    connected_at TIMESTAMP DEFAULT NOW(),
    last_heartbeat TIMESTAMP DEFAULT NOW(),
    UNIQUE(agent_id, websocket_conn_id)
);

CREATE INDEX IF NOT EXISTS idx_redis_agent_connections_agent_id ON redis_agent_connections(agent_id);
CREATE INDEX IF NOT EXISTS idx_redis_agent_connections_pod_id ON redis_agent_connections(pod_id);

COMMENT ON TABLE redis_agent_connections IS 'Tracks agent connections across multiple Control Plane pods (Redis backend)';
COMMENT ON COLUMN redis_agent_connections.pod_id IS 'Control Plane pod ID handling this agent connection';

-- 6. Create leader_election_leases table (for tracking leader elections)
CREATE TABLE IF NOT EXISTS leader_election_leases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lease_name VARCHAR(255) UNIQUE NOT NULL,      -- e.g., "k8s-agent-leader"
    holder_identity VARCHAR(255),                 -- Current lease holder (pod name)
    acquired_time TIMESTAMP,
    renew_time TIMESTAMP,
    lease_duration_seconds INT DEFAULT 15,
    platform VARCHAR(50),                         -- "kubernetes", "docker"
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_leader_election_leases_name ON leader_election_leases(lease_name);
CREATE INDEX IF NOT EXISTS idx_leader_election_leases_holder ON leader_election_leases(holder_identity);

COMMENT ON TABLE leader_election_leases IS 'Tracks leader election leases for HA agent deployments';
COMMENT ON COLUMN leader_election_leases.holder_identity IS 'Current lease holder (agent pod name or Docker agent instance ID)';

-- 7. Add state column to sessions (was missing, caused P0-WRONG-COLUMN bug)
-- Check if 'status' column exists and rename to 'state' if needed
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sessions' AND column_name = 'status'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sessions' AND column_name = 'state'
    ) THEN
        ALTER TABLE sessions RENAME COLUMN status TO state;
    END IF;
END $$;

-- Ensure state column exists
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS state VARCHAR(50) DEFAULT 'pending';

CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);

COMMENT ON COLUMN sessions.state IS 'Session state: pending, running, hibernated, terminating, terminated';

-- 8. Update agent_commands table to handle NULL values properly
-- Add default for retry_count (was causing NULL scan errors)
ALTER TABLE agent_commands
ALTER COLUMN retry_count SET DEFAULT 0;

-- Update existing NULL retry_count values
UPDATE agent_commands
SET retry_count = 0
WHERE retry_count IS NULL;

-- 9. Insert migration version
INSERT INTO schema_migrations (version) VALUES ('v2.0.0-beta1-ha')
ON CONFLICT (version) DO NOTHING;

-- Migration complete
SELECT 'v2.0-beta.1 additions migration completed successfully' AS status;
```

### Running the Migration

```bash
# Download migrations
wget https://raw.githubusercontent.com/streamspace-dev/streamspace/main/migrations/v2.0-agents.sql
wget https://raw.githubusercontent.com/streamspace-dev/streamspace/main/migrations/v2.0-beta1-additions.sql

# Backup database first!
pg_dump -h <db-host> -U streamspace -d streamspace \
  --format=custom --file=streamspace-pre-v2-backup.dump

# Run base v2.0 migration
psql -h <db-host> -U streamspace -d streamspace -f v2.0-agents.sql

# Run v2.0-beta.1 additions
psql -h <db-host> -U streamspace -d streamspace -f v2.0-beta1-additions.sql

# Verify migrations
psql -h <db-host> -U streamspace -d streamspace -c \
  "SELECT version, applied_at FROM schema_migrations ORDER BY applied_at;"

# Expected:
#        version         |        applied_at
# -----------------------+---------------------------
#  v2.0.0-agents         | 2025-11-22 10:30:00
#  v2.0.0-beta1-ha       | 2025-11-22 10:30:05
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

**v2.0-beta.1 (Control Plane):**
```bash
# Database (same as v1.x)
DB_HOST=postgres.example.com
DB_PORT=5432
DB_NAME=streamspace
DB_USER=streamspace
DB_PASSWORD=secret
JWT_SECRET=changeme

# Server
PORT=8080

# Agent Communication (NEW)
AGENT_HEARTBEAT_TIMEOUT=30s
VNC_PROXY_TIMEOUT=5m

# High Availability (NEW in v2.0-beta.1)
REDIS_URL=redis://redis-master:6379               # Required for multi-pod deployments
AGENT_HUB_BACKEND=redis                           # "memory" (single pod) or "redis" (multi-pod)

# Logging
LOG_LEVEL=info                                    # debug, info, warn, error
```

**v2.0-beta.1 (K8s Agent):**
```bash
# Agent Identity (REQUIRED)
AGENT_ID=k8s-prod-us-east-1
CONTROL_PLANE_URL=wss://streamspace.example.com

# Platform
PLATFORM=kubernetes
REGION=us-east-1
NAMESPACE=streamspace

# Capacity
MAX_CPU=100
MAX_MEMORY=256
MAX_SESSIONS=100

# High Availability (NEW in v2.0-beta.1)
ENABLE_HA=true                                    # Enable leader election
LEASE_LOCK_NAME=k8s-agent-leader                 # Lease name for leader election
LEASE_LOCK_NAMESPACE=streamspace                 # Namespace for lease resource
LEASE_DURATION=15s                               # How long lease is valid
RENEW_DEADLINE=10s                               # Time before lease expires to renew
RETRY_PERIOD=2s                                  # How often to retry acquiring lease
```

**v2.0-beta.1 (Docker Agent):**
```bash
# Agent Identity (REQUIRED)
AGENT_ID=docker-host-01
CONTROL_PLANE_URL=wss://streamspace.example.com

# Platform
PLATFORM=docker
REGION=us-east-1

# Docker Configuration
DOCKER_HOST=unix:///var/run/docker.sock          # Docker daemon socket
NETWORK_PREFIX=streamspace                        # Docker network prefix

# Capacity
MAX_SESSIONS=50

# High Availability (NEW in v2.0-beta.1)
ENABLE_HA=true
HA_BACKEND=redis                                  # "file" (single host), "redis" (multi-host), "swarm" (Docker Swarm)
REDIS_URL=redis://shared-redis:6379              # Required if HA_BACKEND=redis
LEASE_DURATION=15s
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

**v2.0-beta.1 Ingress:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: streamspace-v2
  annotations:
    # IMPORTANT: WebSocket support required for agent connections
    nginx.ingress.kubernetes.io/websocket-services: streamspace-control-plane
    # HA: Session affinity for multi-pod API deployments
    nginx.ingress.kubernetes.io/affinity: "cookie"
    nginx.ingress.kubernetes.io/session-cookie-name: "streamspace-affinity"
    nginx.ingress.kubernetes.io/session-cookie-hash: "sha1"
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

## High Availability Configuration

### Redis Deployment (for Multi-Pod API)

**Option 1: Helm (Recommended)**

```bash
# Install Redis
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install redis bitnami/redis \
  --namespace streamspace-v2 \
  --set auth.enabled=false \
  --set master.persistence.enabled=true \
  --set master.persistence.size=1Gi \
  --set master.resources.requests.memory=512Mi \
  --set master.resources.requests.cpu=250m

# Verify Redis
kubectl exec -n streamspace-v2 redis-master-0 -- redis-cli ping
# Expected: PONG
```

**Option 2: Standalone Redis**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: streamspace-v2
spec:
  serviceName: redis
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        volumeMounts:
        - name: data
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: streamspace-v2
spec:
  selector:
    app: redis
  ports:
  - port: 6379
```

### Multi-Pod API Configuration

**Horizontal Pod Autoscaler (Optional)**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: streamspace-control-plane
  namespace: streamspace-v2
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: streamspace-control-plane
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### K8s Agent Leader Election

**Verify Leader Election:**

```bash
# Check leader lease
kubectl get lease k8s-agent-leader -n streamspace -o yaml

# Check leader from agent logs
kubectl logs -n streamspace -l component=k8s-agent | grep -E "leader|LEADER|FOLLOWER"

# Test failover: Delete leader pod
LEADER_POD=$(kubectl get lease k8s-agent-leader -n streamspace -o jsonpath='{.spec.holderIdentity}' | cut -d'_' -f1)
kubectl delete pod $LEADER_POD -n streamspace

# Verify new leader elected (should take <5 seconds)
sleep 5
kubectl get lease k8s-agent-leader -n streamspace
kubectl logs -n streamspace -l component=k8s-agent --tail=50 | grep "acquired leader"
```

### Docker Agent High Availability

**Backend: Redis (Recommended for Multi-Host)**

```bash
# Instance 1 configuration
ENABLE_HA=true
HA_BACKEND=redis
REDIS_URL=redis://shared-redis.example.com:6379
LEASE_DURATION=15s

# Instance 2 configuration (same Redis)
ENABLE_HA=true
HA_BACKEND=redis
REDIS_URL=redis://shared-redis.example.com:6379
LEASE_DURATION=15s

# Verify leader election
redis-cli -h shared-redis.example.com GET "lease:docker-agent-leader"
# Expected: {"holder":"docker-host-01","acquired":"2025-11-22T10:30:00Z",...}
```

**Backend: Docker Swarm (Alternative)**

```bash
# For Docker Swarm environments
ENABLE_HA=true
HA_BACKEND=swarm
LEASE_DURATION=15s

# Swarm uses distributed Raft consensus for leader election
```

---

## Post-Migration

### Verification Checklist

**✅ Infrastructure:**
- [ ] Control Plane pods running (2+ replicas)
- [ ] Redis pod running (if multi-pod API)
- [ ] K8s Agent pods running (3+ replicas for HA)
- [ ] Docker Agent instances running (if using Docker platform)
- [ ] Agent status "online" in UI
- [ ] Database tables created (agents, agent_commands, redis_agent_connections, leader_election_leases)
- [ ] Ingress serving traffic with WebSocket support

**✅ High Availability:**
- [ ] Multiple API pods handling requests
- [ ] Agent connections distributed across API pods (check Redis)
- [ ] K8s Agent leader elected (check lease)
- [ ] Docker Agent leader elected (check Redis or Swarm)
- [ ] Failover working (kill leader, verify new leader elected <5s)

**✅ Functionality:**
- [ ] User login works
- [ ] Session creation works on K8s platform
- [ ] Session creation works on Docker platform (if deployed)
- [ ] VNC connection works (via proxy)
- [ ] Session list displays
- [ ] Session stop works
- [ ] Hibernate/wake works

**✅ Admin Features:**
- [ ] Agents page shows all agents (K8s + Docker)
- [ ] Audit logs recording events
- [ ] License enforcement working

**✅ Monitoring:**
- [ ] Prometheus metrics exposed
- [ ] Grafana dashboards updated
- [ ] Alerts configured

### Performance Testing

```bash
# Create 10 test sessions (mix of K8s and Docker)
for i in {1..5}; do
  # K8s sessions
  curl -X POST https://streamspace.example.com/api/v1/sessions \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"test${i}\",\"template\":\"firefox-browser\",\"platform\":\"kubernetes\",\"state\":\"running\"}"

  # Docker sessions
  curl -X POST https://streamspace.example.com/api/v1/sessions \
    -H "Authorization: Bearer $JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"user\":\"test$((i+5))\",\"template\":\"firefox-browser\",\"platform\":\"docker\",\"state\":\"running\"}"
done

# Wait for sessions to start
sleep 60

# Check session status
curl https://streamspace.example.com/api/v1/sessions \
  -H "Authorization: Bearer $JWT_TOKEN" | jq '.[] | {id, state, platform, agent_id}'

# Test VNC connections
# Manually open 3-5 session viewers and verify VNC works

# Test agent failover
# Kill K8s agent leader pod
kubectl delete pod $(kubectl get lease k8s-agent-leader -n streamspace -o jsonpath='{.spec.holderIdentity}' | cut -d'_' -f1) -n streamspace

# Wait 30 seconds
sleep 30

# Verify sessions still work (should have <23s disruption)
curl https://streamspace.example.com/api/v1/sessions \
  -H "Authorization: Bearer $JWT_TOKEN" | jq '.[] | {id, state, platform}'
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

  # NEW: HA-specific alerts
  - alert: NoAgentLeader
    expr: streamspace_agent_leader_status == 0
    for: 1m
    annotations:
      summary: "No leader elected for {{ $labels.platform }} agent"

  - alert: RedisConnectionFailure
    expr: streamspace_redis_connection_status == 0
    for: 2m
    annotations:
      summary: "Redis connection failed on Control Plane pod {{ $labels.pod }}"

  - alert: HighAPIReplicaFailure
    expr: (kube_deployment_status_replicas_ready{deployment="streamspace-control-plane"} / kube_deployment_spec_replicas{deployment="streamspace-control-plane"}) < 0.5
    for: 5m
    annotations:
      summary: "Less than 50% of API replicas are ready"
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

# Stop Docker Agents (on each Docker host)
sudo systemctl stop streamspace-docker-agent
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
- K8s Agent runs in session cluster (with HA support)
- Connects outbound to Control Plane
- No CRDs, command-based control
- Leader election for multi-pod agent deployments

**Impact**: Deployment model changes (agent deployment required).

**Migration**: Deploy K8s Agent (see deployment guide).

### 4. Database Schema Changes

**New Tables:**
- `agents`
- `agent_commands`
- `platform_controllers`
- `redis_agent_connections` (v2.0-beta.1)
- `leader_election_leases` (v2.0-beta.1)

**Modified Tables:**
- `sessions` (+6 columns: `agent_id`, `platform`, `platform_metadata`, `cluster_id`, `tags`, `state`)
- `agents` (+3 columns: `cluster_id`, `active_sessions`, improved indexes)

**Impact**: Custom database queries may need updates.

**Migration**: Update queries to include new columns.

### 5. High Availability Requirements (v2.0-beta.1)

**Before (v1.x):**
- Single API pod
- Single controller pod
- No Redis required

**After (v2.0-beta.1):**
- 2-10 API pods (recommended)
- Redis required for multi-pod deployments
- 3-10 agent pods per platform (recommended)
- Leader election for agents

**Impact**: Infrastructure requirements increased for HA deployments.

**Migration**: Deploy Redis, scale up replicas, configure leader election.

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

A: No. VNC proxying adds minimal latency (<100ms measured in v2.0-beta.1 testing). Quality remains the same.

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

**Q: Do I need Redis if I only run 1 API pod?**

A: No. Single-pod deployments can use in-memory AgentHub (`AGENT_HUB_BACKEND=memory`). Redis is only required for 2+ API pods.

**Q: How many agent replicas should I run for HA?**

A: Minimum 3 replicas (for quorum), recommended 5 replicas for production. More replicas improve failover resilience.

**Q: What happens if the agent leader crashes?**

A: A new leader is automatically elected within <5 seconds. All existing sessions survive (0% session loss, verified in testing).

**Q: Can I run Docker and Kubernetes agents simultaneously?**

A: Yes! v2.0-beta.1 supports multi-platform deployments. You can run sessions on both K8s and Docker platforms from the same Control Plane.

**Q: What's the difference between Docker Agent HA backends?**

A:
- **File Backend**: Single-host only, stores lease in local file (no HA across hosts)
- **Redis Backend**: Multi-host HA, uses shared Redis for distributed leader election (recommended)
- **Swarm Backend**: Docker Swarm native leader election using Raft consensus

---

## Support

**Migration Issues:**
- GitHub Issues: https://github.com/streamspace-dev/streamspace/issues
- Label: `migration`, `v2.0`

**Documentation:**
- Release Notes: [V2_BETA_RELEASE_NOTES.md](V2_BETA_RELEASE_NOTES.md)
- Deployment Guide: [V2_DEPLOYMENT_GUIDE.md](V2_DEPLOYMENT_GUIDE.md)
- Architecture: [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)
- Troubleshooting: [docs/TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md)

**Community:**
- Discord: https://discord.gg/streamspace
- Community Forum: https://community.streamspace.io

---

**Migration Guide Version**: 1.1
**Last Updated**: 2025-11-22
**StreamSpace Version**: v2.0.0-beta.1
**Changes from v1.0**: Added High Availability sections (Redis, Leader Election), Docker Agent deployment, v2.0-beta.1 database migrations, updated all version references
