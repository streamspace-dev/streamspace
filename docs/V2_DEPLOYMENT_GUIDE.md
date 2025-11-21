# StreamSpace v2.0 Deployment Guide

**Version**: 2.0.0-beta
**Date**: 2025-11-21
**Status**: Production Ready (K8s Agent)

---

## Overview

This guide covers deploying StreamSpace v2.0 with the new Control Plane + Agent architecture. The v2.0 architecture enables multi-platform support, with the first platform being Kubernetes.

**What's New in v2.0:**
- Control Plane + Agent architecture (replacing direct Kubernetes controller)
- VNC proxy/tunneling through Control Plane (firewall-friendly)
- Multi-cluster support (agents can be in different clusters)
- Multi-platform ready (Docker Agent coming in v2.1)

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Architecture Overview](#architecture-overview)
3. [Control Plane Deployment](#control-plane-deployment)
4. [Kubernetes Agent Deployment](#kubernetes-agent-deployment)
5. [Database Migration](#database-migration)
6. [Configuration Reference](#configuration-reference)
7. [Verification & Testing](#verification--testing)
8. [Troubleshooting](#troubleshooting)
9. [Production Considerations](#production-considerations)

---

## Prerequisites

### System Requirements

**Control Plane:**
- Kubernetes cluster (1.19+) OR Docker host OR VM
- PostgreSQL 12+ database
- 2 CPU cores, 4GB RAM minimum
- Persistent storage for database
- External HTTPS endpoint (for agent connections)

**Kubernetes Agent:**
- Kubernetes cluster (1.19+) for agent deployment
- Kubernetes cluster (any version) for sessions
- Outbound HTTPS/WSS access to Control Plane
- 500m CPU, 512Mi RAM minimum per agent
- RBAC permissions to create Deployments, Services, PVCs

### Network Requirements

**Control Plane:**
- Inbound: HTTPS (443) for UI and API
- Inbound: WSS (443) for Agent WebSocket connections
- Inbound: WSS (443) for VNC proxy connections

**Agents:**
- Outbound: HTTPS/WSS to Control Plane (firewall-friendly!)
- Inbound: None required (agents initiate all connections)

**Session Pods:**
- Inbound: VNC port 5900 (from agent only, not exposed externally)

### Software Requirements

- kubectl (for K8s deployments)
- **Helm 3.12.0 - 3.18.x** (recommended for Control Plane)
  - ⚠️ **NOT SUPPORTED**: Helm v3.19.x (has chart loading bugs)
  - ⚠️ **NOT SUPPORTED**: Helm v4.0.x (broken chart loading - upstream regression)
  - ✅ **Recommended**: Helm v3.18.0 (stable, tested)
  - To downgrade if needed: `brew uninstall helm && brew install helm@3.18.0`
- Docker (for building custom images)
- PostgreSQL client (for database setup)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ Control Plane (Centralized)                                     │
│                                                                  │
│  ┌──────────┐      ┌─────────────────────────────────┐         │
│  │ Web UI   │─────▶│ Control Plane API               │         │
│  └──────────┘      │                                 │         │
│       │            │ - Agent Registration            │         │
│       │            │ - WebSocket Hub (Agent Comms)   │         │
│       │            │ - Command Dispatcher            │         │
│       │            │ - VNC Proxy/Tunnel              │         │
│       │            │ - Session State Manager         │         │
│       │            └─────────────────────────────────┘         │
│       │                          │                              │
│       │                          │ WebSocket (Outbound)         │
│       │                          ▼                              │
│       │            ┌──────────────────────────────┐             │
│       │            │ VNC Proxy Endpoint           │             │
│       │            │ /vnc/{session_id}            │             │
│       │            └──────────────────────────────┘             │
│       └──────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────────────┘
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        │                          │                          │
        ▼                          ▼                          ▼
┌────────────────┐      ┌────────────────┐       ┌────────────────┐
│ K8s Agent      │      │ Docker Agent   │       │ Future Agents  │
│ (Cluster 1)    │      │ (v2.1)         │       │ (VM, Cloud)    │
│                │      │                │       │                │
│ - Connects OUT │      │ - Connects OUT │       │ - Connects OUT │
│ - Creates Pods │      │ - Runs Contnrs │       │ - Platform API │
│ - VNC Tunnel   │      │ - VNC Tunnel   │       │ - VNC Tunnel   │
└────────────────┘      └────────────────┘       └────────────────┘
        │                       │                         │
        ▼                       ▼                         ▼
┌────────────────┐      ┌────────────────┐       ┌────────────────┐
│ Session Pod    │      │ Session Contnr │       │ Session VM     │
└────────────────┘      └────────────────┘       └────────────────┘
```

**Key Components:**

1. **Control Plane**: Central management, agent coordination, VNC proxying
2. **Agents**: Platform-specific executors (K8s, Docker, etc.)
3. **Sessions**: User containers/VMs running applications

---

## Control Plane Deployment

The Control Plane is the centralized management component that coordinates all agents.

### Option 1: Helm Chart Deployment (Recommended)

```bash
# Add StreamSpace Helm repository
helm repo add streamspace https://charts.streamspace.io
helm repo update

# Create namespace
kubectl create namespace streamspace

# Deploy Control Plane
helm install streamspace-control-plane streamspace/control-plane \
  --namespace streamspace \
  --set database.host=postgres.example.com \
  --set database.port=5432 \
  --set database.name=streamspace \
  --set database.user=streamspace \
  --set database.password=changeme \
  --set ingress.enabled=true \
  --set ingress.host=streamspace.example.com
```

### Option 2: Manual Kubernetes Deployment

**1. Create namespace and secrets:**

```bash
# Create namespace
kubectl create namespace streamspace

# Create database secret
kubectl create secret generic streamspace-db \
  --namespace streamspace \
  --from-literal=host=postgres.example.com \
  --from-literal=port=5432 \
  --from-literal=database=streamspace \
  --from-literal=username=streamspace \
  --from-literal=password=changeme

# Create JWT secret
kubectl create secret generic streamspace-jwt \
  --namespace streamspace \
  --from-literal=secret=$(openssl rand -base64 32)
```

**2. Deploy Control Plane:**

```yaml
# control-plane-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-control-plane
  namespace: streamspace
spec:
  replicas: 2  # High availability
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
        image: streamspace/control-plane:v2.0
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: streamspace-db
              key: host
        - name: DB_PORT
          valueFrom:
            secretKeyRef:
              name: streamspace-db
              key: port
        - name: DB_NAME
          valueFrom:
            secretKeyRef:
              name: streamspace-db
              key: database
        - name: DB_USER
          valueFrom:
            secretKeyRef:
              name: streamspace-db
              key: username
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: streamspace-db
              key: password
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: streamspace-jwt
              key: secret
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
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
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: streamspace-control-plane
  namespace: streamspace
spec:
  selector:
    app: streamspace
    component: control-plane
  ports:
  - port: 8080
    targetPort: 8080
    name: http
  type: LoadBalancer  # Or ClusterIP with Ingress
```

**3. Apply deployment:**

```bash
kubectl apply -f control-plane-deployment.yaml
```

**4. Create Ingress (for external access):**

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: streamspace
  namespace: streamspace
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/websocket-services: streamspace-control-plane
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - streamspace.example.com
    secretName: streamspace-tls
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

```bash
kubectl apply -f ingress.yaml
```

### Option 3: Docker Deployment

```bash
# Run PostgreSQL
docker run -d \
  --name streamspace-db \
  -e POSTGRES_DB=streamspace \
  -e POSTGRES_USER=streamspace \
  -e POSTGRES_PASSWORD=changeme \
  -v streamspace-db-data:/var/lib/postgresql/data \
  postgres:14

# Run Control Plane
docker run -d \
  --name streamspace-control-plane \
  -p 8080:8080 \
  -e DB_HOST=streamspace-db \
  -e DB_PORT=5432 \
  -e DB_NAME=streamspace \
  -e DB_USER=streamspace \
  -e DB_PASSWORD=changeme \
  -e JWT_SECRET=$(openssl rand -base64 32) \
  --link streamspace-db \
  streamspace/control-plane:v2.0
```

---

## Kubernetes Agent Deployment

The K8s Agent connects to the Control Plane and manages sessions in a Kubernetes cluster.

### Prerequisites

**1. Create namespace for agent:**

```bash
kubectl create namespace streamspace
```

**2. Apply RBAC permissions:**

```bash
# Download and apply RBAC manifests
kubectl apply -f https://raw.githubusercontent.com/JoshuaAFerguson/streamspace/main/agents/k8s-agent/k8s/rbac.yaml
```

Or create manually:

```yaml
# rbac.yaml
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
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["services", "pods", "persistentvolumeclaims", "configmaps", "secrets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get", "list"]
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
```

### Deploy Agent

**1. Create agent deployment:**

```yaml
# agent-deployment.yaml
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
        imagePullPolicy: IfNotPresent
        env:
        # Required: Agent identifier (must be unique)
        - name: AGENT_ID
          value: "k8s-prod-us-east-1"

        # Required: Control Plane WebSocket URL
        - name: CONTROL_PLANE_URL
          value: "wss://streamspace.example.com"

        # Optional: Platform type (default: kubernetes)
        - name: PLATFORM
          value: "kubernetes"

        # Optional: Deployment region
        - name: REGION
          value: "us-east-1"

        # Optional: Session namespace (default: streamspace)
        - name: NAMESPACE
          value: "streamspace"

        # Optional: Capacity limits
        - name: MAX_CPU
          value: "100"  # 100 cores

        - name: MAX_MEMORY
          value: "256"  # 256 GB

        - name: MAX_SESSIONS
          value: "100"  # 100 concurrent sessions

        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"

        livenessProbe:
          exec:
            command:
            - sh
            - -c
            - pgrep -x k8s-agent
          initialDelaySeconds: 30
          periodSeconds: 30

        readinessProbe:
          exec:
            command:
            - sh
            - -c
            - pgrep -x k8s-agent
          initialDelaySeconds: 5
          periodSeconds: 10
```

**2. Apply deployment:**

```bash
kubectl apply -f agent-deployment.yaml
```

**3. Verify agent is running:**

```bash
# Check agent pod
kubectl get pods -n streamspace -l component=k8s-agent

# Check agent logs
kubectl logs -n streamspace -l component=k8s-agent --tail=50

# Expected output:
# Agent registered successfully with Control Plane
# WebSocket connection established
# Agent ID: k8s-prod-us-east-1
# Heartbeat sent every 10 seconds
```

---

## Database Migration

If upgrading from v1.x, run database migrations to add agent-related tables.

### Migration SQL

```sql
-- 1. Create agents table
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(255) UNIQUE NOT NULL,
    platform VARCHAR(50) NOT NULL,
    region VARCHAR(100),
    status VARCHAR(50) DEFAULT 'offline',
    capacity JSONB,
    metadata JSONB,
    websocket_conn_id VARCHAR(255),
    last_heartbeat TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_agents_agent_id ON agents(agent_id);
CREATE INDEX idx_agents_platform ON agents(platform);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_last_heartbeat ON agents(last_heartbeat);

-- 2. Create agent_commands table
CREATE TABLE IF NOT EXISTS agent_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    session_id UUID REFERENCES sessions(id) ON DELETE CASCADE,
    command_type VARCHAR(50) NOT NULL,
    command_data JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    result JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    sent_at TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX idx_agent_commands_agent_id ON agent_commands(agent_id);
CREATE INDEX idx_agent_commands_session_id ON agent_commands(session_id);
CREATE INDEX idx_agent_commands_status ON agent_commands(status);

-- 3. Alter sessions table (add agent columns)
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS platform VARCHAR(50),
ADD COLUMN IF NOT EXISTS platform_metadata JSONB;

CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_sessions_platform ON sessions(platform);
```

### Run Migration

```bash
# Using psql
psql -h postgres.example.com -U streamspace -d streamspace -f migrations/v2.0-agents.sql

# Or using kubectl exec (if database is in cluster)
kubectl exec -n streamspace deployment/postgres -- \
  psql -U streamspace -d streamspace -f /migrations/v2.0-agents.sql
```

---

## Configuration Reference

### Control Plane Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DB_HOST` | Yes | - | PostgreSQL host |
| `DB_PORT` | Yes | 5432 | PostgreSQL port |
| `DB_NAME` | Yes | streamspace | Database name |
| `DB_USER` | Yes | - | Database username |
| `DB_PASSWORD` | Yes | - | Database password |
| `JWT_SECRET` | Yes | - | JWT signing secret (32+ chars) |
| `PORT` | No | 8080 | API server port |
| `LOG_LEVEL` | No | info | Log level (debug, info, warn, error) |
| `AGENT_HEARTBEAT_TIMEOUT` | No | 30s | Heartbeat timeout before marking agent offline |
| `VNC_PROXY_TIMEOUT` | No | 5m | VNC connection idle timeout |

### Kubernetes Agent Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AGENT_ID` | Yes | - | Unique agent identifier |
| `CONTROL_PLANE_URL` | Yes | - | Control Plane WebSocket URL (wss://) |
| `PLATFORM` | No | kubernetes | Platform type |
| `REGION` | No | - | Deployment region |
| `NAMESPACE` | No | streamspace | Namespace for session pods |
| `MAX_CPU` | No | 0 (unlimited) | Max CPU cores for sessions |
| `MAX_MEMORY` | No | 0 (unlimited) | Max memory (GB) for sessions |
| `MAX_SESSIONS` | No | 0 (unlimited) | Max concurrent sessions |

---

## Verification & Testing

### 1. Verify Control Plane

```bash
# Check Control Plane health
curl https://streamspace.example.com/health

# Expected: {"status":"healthy"}

# List agents (should show registered agents)
curl -H "Authorization: Bearer $JWT_TOKEN" \
  https://streamspace.example.com/api/v1/agents

# Expected:
# [
#   {
#     "agent_id": "k8s-prod-us-east-1",
#     "platform": "kubernetes",
#     "status": "online",
#     "region": "us-east-1",
#     "last_heartbeat": "2025-11-21T12:34:56Z"
#   }
# ]
```

### 2. Verify Agent Registration

```bash
# Check agent logs
kubectl logs -n streamspace -l component=k8s-agent --tail=20

# Expected output:
# INFO: Registering agent with Control Plane
# INFO: Agent registered successfully: k8s-prod-us-east-1
# INFO: WebSocket connection established
# INFO: Sending heartbeat (capacity: 100 cores, 256GB RAM, 0/100 sessions)
```

### 3. Test Session Creation

```bash
# Create a test session via UI or API
curl -X POST https://streamspace.example.com/api/v1/sessions \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "testuser",
    "template": "firefox-browser",
    "state": "running"
  }'

# Watch session creation in agent logs
kubectl logs -n streamspace -l component=k8s-agent --follow

# Expected:
# INFO: Received start_session command for session sess-123
# INFO: Creating deployment for session sess-123
# INFO: Creating service for session sess-123
# INFO: Waiting for pod to be ready...
# INFO: Session sess-123 started successfully (pod IP: 10.42.1.5)
# INFO: VNC tunnel initialized for session sess-123
```

### 4. Test VNC Connection

1. Open StreamSpace UI: https://streamspace.example.com
2. Navigate to session viewer for test session
3. Verify VNC connection establishes (you should see the desktop)
4. Test keyboard and mouse input

**Check VNC proxy logs:**

```bash
# Control Plane logs
kubectl logs -n streamspace -l component=control-plane | grep vnc

# Expected:
# INFO: VNC proxy connection established for session sess-123
# INFO: VNC traffic flowing: UI <-> Control Plane <-> Agent <-> Pod
```

---

## Troubleshooting

### Agent Not Connecting

**Symptoms:**
- Agent status shows "offline" in UI
- Agent logs show connection errors

**Solutions:**

```bash
# 1. Check agent logs
kubectl logs -n streamspace -l component=k8s-agent --tail=50

# 2. Verify Control Plane URL is accessible
kubectl exec -n streamspace deployment/streamspace-k8s-agent -- \
  wget -O- https://streamspace.example.com/health

# 3. Check WebSocket connectivity
# WebSocket must use wss:// (not https://) and port 443

# 4. Verify JWT authentication
# If using authentication, agent needs valid credentials

# 5. Check firewall rules
# Agent needs outbound HTTPS/WSS (port 443) access
```

### VNC Connection Fails

**Symptoms:**
- VNC viewer shows "Connecting..." indefinitely
- Error: "Failed to connect to VNC proxy"

**Solutions:**

```bash
# 1. Check session status
curl -H "Authorization: Bearer $JWT_TOKEN" \
  https://streamspace.example.com/api/v1/sessions/sess-123

# Verify: state should be "running", agent_id should be set

# 2. Check VNC tunnel in agent
kubectl logs -n streamspace -l component=k8s-agent | grep "VNC tunnel"

# Expected: "VNC tunnel initialized for session sess-123"

# 3. Check Control Plane VNC proxy
kubectl logs -n streamspace -l component=control-plane | grep vnc_proxy

# 4. Verify session pod is running
kubectl get pods -n streamspace -l session=sess-123

# 5. Test VNC server in pod
kubectl exec -n streamspace <session-pod> -- nc -zv localhost 5900
# Expected: Connection to localhost 5900 port [tcp/*] succeeded!
```

### Sessions Not Starting

**Symptoms:**
- Session stuck in "pending" state
- No pods created

**Solutions:**

```bash
# 1. Check agent logs
kubectl logs -n streamspace -l component=k8s-agent --tail=100

# 2. Verify RBAC permissions
kubectl auth can-i create deployments --namespace streamspace \
  --as system:serviceaccount:streamspace:streamspace-agent

# Expected: yes

# 3. Check resource quotas
kubectl describe resourcequota -n streamspace

# 4. Check PVC creation (if using persistent storage)
kubectl get pvc -n streamspace

# 5. Check image pull secrets
kubectl get pods -n streamspace -l session=sess-123 -o yaml | grep -A5 ImagePullBackOff
```

### Database Connection Issues

**Symptoms:**
- Control Plane pod crashes
- Logs show "connection refused" or "authentication failed"

**Solutions:**

```bash
# 1. Check database secret
kubectl get secret streamspace-db -n streamspace -o yaml

# 2. Test database connection from pod
kubectl run -it --rm debug --image=postgres:14 --restart=Never -n streamspace -- \
  psql -h postgres.example.com -U streamspace -d streamspace

# 3. Check database migrations
# Run migration SQL if not already applied

# 4. Verify database is accessible
# Database should allow connections from Control Plane pods
```

---

## Production Considerations

### High Availability

**Control Plane:**
- Deploy 2+ replicas with load balancing
- Use external PostgreSQL (RDS, Cloud SQL) with replicas
- Enable session persistence for WebSocket connections
- Use Redis for distributed session storage (optional)

```yaml
spec:
  replicas: 3  # Minimum for HA
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
```

**Agents:**
- Deploy multiple agents for redundancy
- Use different agent IDs per instance
- Agents automatically reconnect on failure
- Control Plane redistributes sessions on agent failure

### Security

**TLS/SSL:**
- Always use HTTPS/WSS in production
- Use cert-manager for automatic certificate renewal
- Enable HSTS headers

**Authentication:**
- Rotate JWT secrets regularly
- Use strong secrets (32+ characters, random)
- Enable MFA for admin users
- Use SAML/OIDC for SSO

**Network Policies:**
- Restrict agent ingress (only outbound connections needed)
- Restrict session pod access (only agent can connect to VNC port)
- Use NetworkPolicies in Kubernetes

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: streamspace-agent-policy
  namespace: streamspace
spec:
  podSelector:
    matchLabels:
      component: k8s-agent
  policyTypes:
  - Egress
  egress:
  - to:
    - podSelector:
        matchLabels:
          component: control-plane
    ports:
    - protocol: TCP
      port: 8080
```

### Monitoring

**Metrics to Monitor:**
- Agent status (online/offline)
- Agent heartbeat latency
- Session creation success rate
- VNC connection success rate
- Database connection pool usage
- WebSocket connection count

**Prometheus Integration:**

```yaml
# ServiceMonitor for Control Plane
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: streamspace-control-plane
  namespace: streamspace
spec:
  selector:
    matchLabels:
      component: control-plane
  endpoints:
  - port: metrics
    interval: 30s
```

### Backup & Recovery

**Database Backups:**
- Daily automated backups
- Point-in-time recovery enabled
- Test restore procedure regularly

**Configuration Backups:**
- Store Kubernetes manifests in Git
- Backup secrets securely (Vault, Sealed Secrets)
- Document deployment procedures

### Scaling

**Horizontal Scaling:**
- Scale Control Plane pods based on CPU/memory
- Scale agents based on session load
- Add agents in new regions as needed

**Vertical Scaling:**
- Increase agent resources for larger sessions
- Increase Control Plane resources for more agents

```bash
# Scale Control Plane
kubectl scale deployment streamspace-control-plane \
  --replicas=5 -n streamspace

# Add new agent in different region
kubectl apply -f agent-deployment-eu-west-1.yaml
```

---

## Next Steps

- **Architecture Documentation**: See [V2_ARCHITECTURE.md](V2_ARCHITECTURE.md) for detailed architecture
- **Migration Guide**: See [V2_MIGRATION_GUIDE.md](V2_MIGRATION_GUIDE.md) for v1.x → v2.0 migration
- **Troubleshooting**: See [TROUBLESHOOTING.md](../TROUBLESHOOTING.md) for common issues
- **API Reference**: See [API_REFERENCE.md](../api/API_REFERENCE.md) for API documentation

---

## Support

- GitHub Issues: https://github.com/JoshuaAFerguson/streamspace/issues
- Documentation: https://docs.streamspace.io
- Community Discord: https://discord.gg/streamspace

---

**Deployment Guide Version**: 1.0
**Last Updated**: 2025-11-21
**StreamSpace Version**: v2.0.0-beta
