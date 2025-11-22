# StreamSpace v2.0 Agent Guide

> **Comprehensive guide for deploying and managing StreamSpace Agents**
> **Version:** v2.0-beta
> **Target Audience:** DevOps engineers, Platform administrators

---

## Table of Contents

1. [Overview](#overview)
2. [Agent Architecture](#agent-architecture)
3. [Prerequisites](#prerequisites)
4. [Installation](#installation)
   - [Option 1: Helm Chart](#option-1-helm-chart-recommended)
   - [Option 2: Kubernetes Manifests](#option-2-kubernetes-manifests)
   - [Option 3: From Source](#option-3-from-source)
5. [Configuration Reference](#configuration-reference)
6. [RBAC and Security](#rbac-and-security)
7. [Health Monitoring](#health-monitoring)
8. [Operational Tasks](#operational-tasks)
9. [Troubleshooting](#troubleshooting)
10. [Advanced Configuration](#advanced-configuration)
11. [Multi-Agent Deployment](#multi-agent-deployment)

---

## Overview

**StreamSpace Agents** are platform-specific components that execute session lifecycle operations on behalf of the Control Plane. In v2.0, agents connect TO the Control Plane via WebSocket (outbound only), enabling deployment behind firewalls, NAT, and corporate proxies.

### What is a StreamSpace Agent?

A StreamSpace Agent is a lightweight service that:
- Connects to the Control Plane via WebSocket
- Receives commands from the Control Plane (create session, delete session, etc.)
- Executes operations on the target platform (Kubernetes, Docker, VMs, etc.)
- Reports status and metrics back to the Control Plane
- Tunnels VNC traffic from sessions to the Control Plane

### v2.0-beta Agents

**Currently Available:**
- **K8s Agent** - Kubernetes platform agent (fully functional)

**Coming Soon:**
- Docker Agent (v2.1)
- VM Agent - Proxmox/VMware (v2.2)
- Cloud Agent - AWS/Azure/GCP (v2.3)

---

## Agent Architecture

### High-Level Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Control Plane                         │
│  - Agent Hub (WebSocket server)                          │
│  - Command Dispatcher                                    │
│  - VNC Proxy                                             │
└───────────────────┬─────────────────────────────────────┘
                    │ WebSocket (TLS)
                    │ wss://control-plane.example.com/api/v1/agent/connect
                    │
          ┌─────────┴──────────┐
          │                    │
          ↓                    ↓
┌─────────────────┐   ┌─────────────────┐
│  K8s Agent #1   │   │  K8s Agent #2   │
│  Region: US-E   │   │  Region: EU-W   │
│                 │   │                 │
│  - Session Mgr  │   │  - Session Mgr  │
│  - VNC Tunnel   │   │  - VNC Tunnel   │
│  - Health Check │   │  - Health Check │
└────────┬────────┘   └────────┬────────┘
         │                     │
         ↓                     ↓
┌─────────────────┐   ┌─────────────────┐
│  Kubernetes     │   │  Kubernetes     │
│  Cluster #1     │   │  Cluster #2     │
│  [Session Pods] │   │  [Session Pods] │
└─────────────────┘   └─────────────────┘
```

### Key Components

**1. WebSocket Client**
- Maintains persistent connection to Control Plane
- Automatic reconnection with exponential backoff
- Heartbeat every 30 seconds

**2. Command Handler**
- Processes commands from Control Plane
- Command lifecycle: pending → sent → ack → completed/failed
- Supports: create_session, delete_session, list_sessions, vnc_connect, vnc_data, vnc_disconnect

**3. Session Manager** (K8s Agent)
- CRUD operations for sessions (pods, services, PVCs)
- Resource allocation and labeling
- Environment variable injection
- Volume mounts for persistent home directories

**4. VNC Tunnel** (K8s Agent)
- Kubernetes port-forward to session pod VNC port (5900)
- Binary WebSocket streaming for VNC data
- Automatic tunnel cleanup on disconnect

**5. Health Monitor**
- Periodic heartbeat to Control Plane
- Capacity reporting (CPU, memory, max sessions)
- Agent status: online, offline, warning, error

---

## Prerequisites

### General Requirements

- **Control Plane**: Deployed and accessible (v2.0+)
- **Network**: Outbound access from agent to Control Plane (HTTPS/WSS)
- **TLS**: Valid TLS certificate on Control Plane (for wss://)

### K8s Agent Specific

- **Kubernetes**: 1.19+ (k3s, EKS, AKS, GKE supported)
- **kubectl**: Configured with cluster access
- **RBAC**: Permissions to create pods, services, PVCs in target namespace
- **Storage**: StorageClass with ReadWriteOnce support (RWX for shared home dirs)
- **Resources**: 1 CPU core, 2GB RAM minimum per agent

---

## Installation

### Option 1: Helm Chart (Recommended)

**Step 1: Add Helm Repository**
```bash
helm repo add streamspace https://streamspace.io/charts
helm repo update
```

**Step 2: Create Configuration**
```bash
cat > k8s-agent-values.yaml <<EOF
agent:
  # REQUIRED: Unique agent identifier
  id: k8s-prod-us-east-1

  # REQUIRED: Control Plane WebSocket URL
  controlPlaneUrl: wss://streamspace.example.com

  # Platform type (default: kubernetes)
  platform: kubernetes

  # Deployment region (optional, for UI display)
  region: us-east-1

  # Target namespace for sessions (default: streamspace)
  namespace: streamspace

  # Resource limits
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi

  # Replica count (1 recommended per cluster/region)
  replicaCount: 1

# RBAC configuration
rbac:
  create: true

serviceAccount:
  create: true
  name: streamspace-k8s-agent
EOF
```

**Step 3: Install Agent**
```bash
helm install streamspace-k8s-agent streamspace/k8s-agent \
  --namespace streamspace \
  --create-namespace \
  --values k8s-agent-values.yaml
```

**Step 4: Verify Installation**
```bash
# Check pod status
kubectl get pods -n streamspace -l app=streamspace,component=k8s-agent

# Check agent logs
kubectl logs -n streamspace -l app=streamspace,component=k8s-agent -f

# Verify agent registration in Control Plane
curl -H "Authorization: Bearer $JWT_TOKEN" \
  https://streamspace.example.com/api/v1/agents
```

---

### Option 2: Kubernetes Manifests

**Step 1: Create Namespace**
```bash
kubectl create namespace streamspace
```

**Step 2: Create RBAC Resources**
```yaml
# rbac.yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
rules:
# Pods
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "create", "delete", "patch"]
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods/portforward"]
  verbs: ["get", "list", "create"]
# Services
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch", "create", "delete", "patch"]
# PVCs
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "create", "delete", "patch"]
# ConfigMaps (for agent config)
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
# Secrets (for session credentials)
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: streamspace-k8s-agent
subjects:
- kind: ServiceAccount
  name: streamspace-k8s-agent
  namespace: streamspace
```

```bash
kubectl apply -f rbac.yaml
```

**Step 3: Create Agent Deployment**
```yaml
# deployment.yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
  labels:
    app: streamspace
    component: k8s-agent
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
      serviceAccountName: streamspace-k8s-agent
      containers:
      - name: agent
        image: streamspace/k8s-agent:v2.0
        env:
        # REQUIRED
        - name: AGENT_ID
          value: "k8s-prod-us-east-1"
        - name: CONTROL_PLANE_URL
          value: "wss://streamspace.example.com"

        # Platform configuration
        - name: PLATFORM
          value: "kubernetes"
        - name: REGION
          value: "us-east-1"
        - name: NAMESPACE
          value: "streamspace"

        # Agent behavior
        - name: HEARTBEAT_INTERVAL
          value: "30s"
        - name: RECONNECT_DELAY
          value: "5s"
        - name: MAX_RECONNECT_DELAY
          value: "5m"

        # VNC configuration
        - name: VNC_PORT
          value: "5900"
        - name: VNC_TUNNEL_TIMEOUT
          value: "1h"

        # Logging
        - name: LOG_LEVEL
          value: "info"

        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi

        # Health probes
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3

        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
```

```bash
kubectl apply -f deployment.yaml
```

**Step 4: Verify Deployment**
```bash
kubectl rollout status deployment/streamspace-k8s-agent -n streamspace
kubectl get pods -n streamspace -l component=k8s-agent
kubectl logs -n streamspace -l component=k8s-agent -f
```

---

### Option 3: From Source

**Step 1: Clone Repository**
```bash
git clone https://github.com/JoshuaAFerguson/streamspace.git
cd streamspace/agents/k8s-agent
```

**Step 2: Build Binary**
```bash
# Build for Linux (for Docker image)
GOOS=linux GOARCH=amd64 go build -o k8s-agent ./cmd/k8s-agent

# Or build for current platform (local testing)
go build -o k8s-agent ./cmd/k8s-agent
```

**Step 3: Build Docker Image**
```bash
docker build -t streamspace/k8s-agent:local .
```

**Step 4: Push to Registry** (if using remote cluster)
```bash
docker tag streamspace/k8s-agent:local your-registry.io/streamspace/k8s-agent:v2.0
docker push your-registry.io/streamspace/k8s-agent:v2.0
```

**Step 5: Update deployment.yaml with your image**
```yaml
spec:
  containers:
  - name: agent
    image: your-registry.io/streamspace/k8s-agent:v2.0
```

**Step 6: Deploy**
```bash
kubectl apply -f deployment.yaml
```

---

## Configuration Reference

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `AGENT_ID` | Unique agent identifier (must be unique across all agents) | `k8s-prod-us-east-1` |
| `CONTROL_PLANE_URL` | Control Plane WebSocket URL (wss://) | `wss://streamspace.example.com` |

### Platform Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `PLATFORM` | Platform type | `kubernetes` | `kubernetes` |
| `REGION` | Deployment region (optional, for UI display) | `""` | `us-east-1`, `eu-west-1` |
| `NAMESPACE` | Target namespace for sessions | `streamspace` | `streamspace`, `sessions` |
| `KUBECONFIG` | Path to kubeconfig file (if not using in-cluster config) | `""` | `/root/.kube/config` |

### Agent Behavior

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `HEARTBEAT_INTERVAL` | Heartbeat frequency to Control Plane | `30s` | `30s`, `1m` |
| `RECONNECT_DELAY` | Initial reconnect delay after disconnect | `5s` | `5s`, `10s` |
| `MAX_RECONNECT_DELAY` | Maximum reconnect delay (exponential backoff) | `5m` | `5m`, `10m` |
| `MAX_SESSIONS` | Maximum concurrent sessions (capacity limit) | `100` | `50`, `200` |

### Session Configuration (K8s Agent)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `SESSION_IMAGE_PULL_POLICY` | Image pull policy for session pods | `IfNotPresent` | `Always`, `Never` |
| `SESSION_DEFAULT_CPU` | Default CPU request for sessions | `1000m` | `500m`, `2000m` |
| `SESSION_DEFAULT_MEMORY` | Default memory request for sessions | `2Gi` | `1Gi`, `4Gi` |
| `SESSION_DEFAULT_STORAGE` | Default PVC size for home directories | `10Gi` | `5Gi`, `20Gi` |
| `SESSION_STORAGE_CLASS` | StorageClass for PVCs | `""` (cluster default) | `nfs`, `gp3` |
| `SESSION_SERVICE_TYPE` | Service type for session pods | `ClusterIP` | `ClusterIP`, `NodePort` |

### VNC Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `VNC_PORT` | VNC port on session pods | `5900` | `5900` |
| `VNC_TUNNEL_TIMEOUT` | VNC tunnel idle timeout | `1h` | `30m`, `2h` |
| `VNC_BUFFER_SIZE` | VNC data buffer size | `8192` | `4096`, `16384` |

### Logging and Monitoring

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `LOG_LEVEL` | Log level | `info` | `debug`, `warn`, `error` |
| `LOG_FORMAT` | Log format | `json` | `json`, `text` |
| `METRICS_ENABLED` | Enable Prometheus metrics | `true` | `true`, `false` |
| `METRICS_PORT` | Prometheus metrics port | `9090` | `9090`, `8081` |

### Advanced Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `COMMAND_TIMEOUT` | Command execution timeout | `5m` | `3m`, `10m` |
| `GRACEFUL_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout | `30s` | `30s`, `1m` |
| `MAX_CONCURRENT_OPERATIONS` | Max concurrent session operations | `10` | `5`, `20` |

---

## RBAC and Security

### Minimum RBAC Permissions (K8s Agent)

The K8s Agent requires the following permissions in the target namespace:

**Pods:**
- `get`, `list`, `watch` - View pod status
- `create`, `delete` - Session lifecycle
- `patch` - Update pod labels/annotations
- `pods/log` - Stream logs to Control Plane
- `pods/portforward` - VNC tunneling

**Services:**
- `get`, `list`, `watch` - View services
- `create`, `delete` - Session services
- `patch` - Update service labels

**PersistentVolumeClaims:**
- `get`, `list`, `watch` - View PVCs
- `create`, `delete` - Session home directories
- `patch` - Update PVC labels

**ConfigMaps and Secrets** (read-only):
- `get`, `list`, `watch` - Read configuration

### Security Best Practices

**1. Use Dedicated ServiceAccount**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
```

**2. Restrict to Target Namespace**
Use `Role` and `RoleBinding` instead of `ClusterRole` to limit scope:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace  # Only in this namespace
```

**3. Enable Pod Security Standards**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: streamspace
  labels:
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

**4. Use Network Policies**
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
spec:
  podSelector:
    matchLabels:
      component: k8s-agent
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: streamspace
    ports:
    - protocol: TCP
      port: 8080  # Health checks
  egress:
  - to:
    - namespaceSelector: {}  # Allow to API server
  - to:  # Allow to Control Plane
    ports:
    - protocol: TCP
      port: 443
```

**5. Use TLS for Control Plane Connection**
Always use `wss://` (WebSocket Secure) for Control Plane URL:
```bash
CONTROL_PLANE_URL=wss://streamspace.example.com  # ✅ Secure
CONTROL_PLANE_URL=ws://streamspace.example.com   # ❌ Insecure
```

**6. Rotate Agent Credentials**
If using authentication tokens (future):
```bash
# Generate new token
kubectl create secret generic streamspace-k8s-agent-token \
  --from-literal=token=$(openssl rand -base64 32) \
  -n streamspace

# Mount as environment variable
env:
- name: AGENT_TOKEN
  valueFrom:
    secretKeyRef:
      name: streamspace-k8s-agent-token
      key: token
```

---

## Health Monitoring

### Health Endpoints

**1. Liveness Probe: `/health`**
```bash
curl http://agent-pod:8080/health
```

Returns:
```json
{
  "status": "healthy",
  "agent_id": "k8s-prod-us-east-1",
  "uptime": "2h30m15s"
}
```

**2. Readiness Probe: `/ready`**
```bash
curl http://agent-pod:8080/ready
```

Returns:
```json
{
  "status": "ready",
  "control_plane_connected": true,
  "last_heartbeat": "2025-11-20T10:30:45Z"
}
```

**3. Metrics Endpoint: `/metrics`** (Prometheus)
```bash
curl http://agent-pod:9090/metrics
```

Returns Prometheus metrics:
```
# HELP streamspace_agent_sessions_active Active sessions managed by this agent
# TYPE streamspace_agent_sessions_active gauge
streamspace_agent_sessions_active 5

# HELP streamspace_agent_uptime_seconds Agent uptime in seconds
# TYPE streamspace_agent_uptime_seconds counter
streamspace_agent_uptime_seconds 9015

# HELP streamspace_agent_heartbeat_last_success_timestamp Last successful heartbeat timestamp
# TYPE streamspace_agent_heartbeat_last_success_timestamp gauge
streamspace_agent_heartbeat_last_success_timestamp 1732101045
```

### Monitoring Agent Status

**Check Agent Logs:**
```bash
kubectl logs -n streamspace -l component=k8s-agent -f --tail=100
```

**Check Agent Status in Control Plane:**
```bash
# List all agents
curl -H "Authorization: Bearer $JWT_TOKEN" \
  https://streamspace.example.com/api/v1/agents | jq

# Get specific agent
curl -H "Authorization: Bearer $JWT_TOKEN" \
  https://streamspace.example.com/api/v1/agents/k8s-prod-us-east-1 | jq
```

Response:
```json
{
  "id": "k8s-prod-us-east-1",
  "platform": "kubernetes",
  "region": "us-east-1",
  "status": "online",
  "last_heartbeat": "2025-11-20T10:30:45Z",
  "capacity": {
    "cpu": "8000m",
    "memory": "16Gi",
    "max_sessions": 100
  },
  "active_sessions": 5
}
```

### Prometheus Monitoring

**ServiceMonitor for Prometheus Operator:**
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: streamspace-k8s-agent
  namespace: streamspace
spec:
  selector:
    matchLabels:
      app: streamspace
      component: k8s-agent
  endpoints:
  - port: metrics
    interval: 30s
```

**Key Metrics to Monitor:**
- `streamspace_agent_sessions_active` - Active sessions
- `streamspace_agent_uptime_seconds` - Agent uptime
- `streamspace_agent_heartbeat_last_success_timestamp` - Heartbeat health
- `streamspace_agent_vnc_tunnels_active` - Active VNC tunnels
- `streamspace_agent_command_duration_seconds` - Command execution time

---

## Operational Tasks

### Upgrading an Agent

**Rolling Update (Zero Downtime):**
```bash
# Update image version
kubectl set image deployment/streamspace-k8s-agent \
  agent=streamspace/k8s-agent:v2.1 \
  -n streamspace

# Watch rollout
kubectl rollout status deployment/streamspace-k8s-agent -n streamspace
```

**Controlled Upgrade (Drain First):**
```bash
# Scale down to 0 (drains active sessions gracefully)
kubectl scale deployment/streamspace-k8s-agent --replicas=0 -n streamspace

# Wait for graceful shutdown (up to 30s)
kubectl wait --for=delete pod -l component=k8s-agent -n streamspace --timeout=60s

# Update image
kubectl set image deployment/streamspace-k8s-agent \
  agent=streamspace/k8s-agent:v2.1 \
  -n streamspace

# Scale back up
kubectl scale deployment/streamspace-k8s-agent --replicas=1 -n streamspace
```

### Restarting an Agent

**Graceful Restart:**
```bash
kubectl rollout restart deployment/streamspace-k8s-agent -n streamspace
```

**Force Restart (if hung):**
```bash
kubectl delete pod -l component=k8s-agent -n streamspace
```

### Scaling Agents

**Single Cluster (1 agent recommended):**
```bash
kubectl scale deployment/streamspace-k8s-agent --replicas=1 -n streamspace
```

**Multi-Region (deploy separate agents):**
```bash
# Deploy second agent with different AGENT_ID
helm install streamspace-k8s-agent-eu streamspace/k8s-agent \
  --set agent.id=k8s-prod-eu-west-1 \
  --set agent.region=eu-west-1 \
  -n streamspace
```

### Viewing Agent Logs

**Real-time logs:**
```bash
kubectl logs -n streamspace -l component=k8s-agent -f
```

**Logs with context (last 100 lines):**
```bash
kubectl logs -n streamspace -l component=k8s-agent -f --tail=100 --timestamps
```

**Logs for specific pod:**
```bash
kubectl logs -n streamspace streamspace-k8s-agent-<pod-id> -f
```

### Draining an Agent

**Graceful drain (wait for sessions to end):**
```bash
# Mark agent offline in Control Plane (prevents new sessions)
curl -X PATCH -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "offline"}' \
  https://streamspace.example.com/api/v1/agents/k8s-prod-us-east-1

# Wait for sessions to complete (monitor in UI)

# Scale down agent
kubectl scale deployment/streamspace-k8s-agent --replicas=0 -n streamspace
```

**Force drain (terminate active sessions):**
```bash
# Delete all sessions on this agent
kubectl delete pods -n streamspace -l agent=k8s-prod-us-east-1

# Scale down agent
kubectl scale deployment/streamspace-k8s-agent --replicas=0 -n streamspace
```

---

## Troubleshooting

### Agent Won't Connect to Control Plane

**Symptoms:**
- Agent pod running but status shows "offline"
- Logs show "connection refused" or "connection timeout"

**Diagnosis:**
```bash
# Check agent logs
kubectl logs -n streamspace -l component=k8s-agent -f

# Test connectivity from agent pod
kubectl exec -n streamspace -it streamspace-k8s-agent-<pod-id> -- \
  wget -O- https://streamspace.example.com/api/v1/health

# Check DNS resolution
kubectl exec -n streamspace -it streamspace-k8s-agent-<pod-id> -- \
  nslookup streamspace.example.com
```

**Solutions:**
1. **Verify Control Plane URL** - Check `CONTROL_PLANE_URL` environment variable
2. **Check TLS Certificate** - Ensure valid TLS cert on Control Plane
3. **Firewall Rules** - Allow outbound HTTPS (443) from agent
4. **Network Policies** - Allow egress to Control Plane
5. **Proxy Settings** - If behind proxy, configure `HTTP_PROXY`/`HTTPS_PROXY`

### Agent Crashes on Startup

**Symptoms:**
- Pod in CrashLoopBackOff
- Logs show panic or fatal error

**Diagnosis:**
```bash
# Check pod events
kubectl describe pod -n streamspace streamspace-k8s-agent-<pod-id>

# Check previous pod logs
kubectl logs -n streamspace streamspace-k8s-agent-<pod-id> --previous
```

**Common Causes:**
1. **Missing Required Env Vars** - Check `AGENT_ID` and `CONTROL_PLANE_URL`
2. **RBAC Issues** - Verify ServiceAccount has required permissions
3. **Invalid Kubeconfig** - If using external kubeconfig, check path
4. **Resource Limits** - Check if OOMKilled (increase memory)

**Solutions:**
```bash
# Check env vars
kubectl get deployment streamspace-k8s-agent -n streamspace -o yaml | grep -A 20 env:

# Test RBAC
kubectl auth can-i create pods --as=system:serviceaccount:streamspace:streamspace-k8s-agent -n streamspace

# Increase resources
kubectl set resources deployment/streamspace-k8s-agent \
  --limits=cpu=2000m,memory=2Gi \
  --requests=cpu=1000m,memory=1Gi \
  -n streamspace
```

### Sessions Won't Start

**Symptoms:**
- Session stuck in "pending" state
- Agent logs show errors creating pods

**Diagnosis:**
```bash
# Check agent logs
kubectl logs -n streamspace -l component=k8s-agent -f | grep -i error

# Check session pod events
kubectl get events -n streamspace --sort-by=.metadata.creationTimestamp | tail -20

# Check pod status
kubectl get pods -n streamspace -l app=session
```

**Common Causes:**
1. **RBAC Permissions** - Agent can't create pods
2. **Image Pull Errors** - Session image not accessible
3. **Resource Quotas** - Namespace quota exceeded
4. **Storage Issues** - PVC creation fails

**Solutions:**
```bash
# Fix RBAC
kubectl apply -f rbac.yaml

# Check image pull secret
kubectl get secrets -n streamspace

# Check resource quota
kubectl describe resourcequota -n streamspace

# Check storage class
kubectl get storageclass
```

### VNC Won't Connect

**Symptoms:**
- Session starts but VNC viewer shows "connecting..."
- VNC proxy returns 503 or timeout

**Diagnosis:**
```bash
# Check VNC tunnel logs
kubectl logs -n streamspace -l component=k8s-agent -f | grep -i vnc

# Check if pod VNC port is listening
kubectl exec -n streamspace <session-pod> -- netstat -ln | grep 5900

# Test port-forward manually
kubectl port-forward -n streamspace <session-pod> 5900:5900
```

**Common Causes:**
1. **VNC Server Not Started** - Session pod VNC server not running
2. **Port-Forward Fails** - Agent can't establish port-forward
3. **Tunnel Timeout** - VNC tunnel idle timeout too short
4. **Network Policy** - Agent can't reach session pods

**Solutions:**
```bash
# Check session pod logs
kubectl logs -n streamspace <session-pod>

# Test manual port-forward
kubectl port-forward -n streamspace <session-pod> 5900:5900

# Increase tunnel timeout
kubectl set env deployment/streamspace-k8s-agent \
  VNC_TUNNEL_TIMEOUT=2h \
  -n streamspace

# Allow agent-to-pod traffic
# (Check NetworkPolicies)
```

### High Memory Usage

**Symptoms:**
- Agent pod OOMKilled
- High memory usage in metrics

**Diagnosis:**
```bash
# Check resource usage
kubectl top pod -n streamspace -l component=k8s-agent

# Check memory limits
kubectl describe pod -n streamspace -l component=k8s-agent | grep -A 5 Limits
```

**Solutions:**
```bash
# Increase memory limit
kubectl set resources deployment/streamspace-k8s-agent \
  --limits=memory=2Gi \
  --requests=memory=1Gi \
  -n streamspace

# Reduce concurrent operations
kubectl set env deployment/streamspace-k8s-agent \
  MAX_CONCURRENT_OPERATIONS=5 \
  -n streamspace

# Reduce max sessions
kubectl set env deployment/streamspace-k8s-agent \
  MAX_SESSIONS=50 \
  -n streamspace
```

---

## Advanced Configuration

### Custom Session Pod Templates

Define custom pod templates for sessions:

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: streamspace-session-template
  namespace: streamspace
data:
  pod-template.yaml: |
    apiVersion: v1
    kind: Pod
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      tolerations:
      - key: streamspace
        operator: Equal
        value: sessions
        effect: NoSchedule
      nodeSelector:
        workload: streamspace
      containers:
      - name: session
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
```

Reference in agent:
```yaml
env:
- name: SESSION_POD_TEMPLATE
  value: /config/pod-template.yaml
volumeMounts:
- name: config
  mountPath: /config
volumes:
- name: config
  configMap:
    name: streamspace-session-template
```

### Resource Quotas per Agent

Limit resources consumed by agent's sessions:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: streamspace-agent-quota
  namespace: streamspace
spec:
  hard:
    pods: "100"
    requests.cpu: "50"
    requests.memory: "100Gi"
    limits.cpu: "100"
    limits.memory: "200Gi"
    persistentvolumeclaims: "100"
    requests.storage: "1Ti"
```

### Affinity and Anti-Affinity

**Keep agent on specific nodes:**
```yaml
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: streamspace
                operator: In
                values:
                - agent
```

**Anti-affinity for multi-agent:**
```yaml
spec:
  template:
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: component
                  operator: In
                  values:
                  - k8s-agent
              topologyKey: kubernetes.io/hostname
```

### Custom Logging Configuration

**JSON Logging:**
```yaml
env:
- name: LOG_FORMAT
  value: json
- name: LOG_LEVEL
  value: info
```

**Log to File (with sidecar):**
```yaml
spec:
  containers:
  - name: agent
    volumeMounts:
    - name: logs
      mountPath: /var/log/streamspace
  - name: log-forwarder
    image: fluent/fluent-bit:latest
    volumeMounts:
    - name: logs
      mountPath: /var/log/streamspace
  volumes:
  - name: logs
    emptyDir: {}
```

---

## Multi-Agent Deployment

### Use Cases

1. **Multi-Cluster**: One agent per Kubernetes cluster
2. **Multi-Region**: One agent per geographic region
3. **Multi-Tenant**: One agent per customer namespace
4. **High Availability**: Multiple agents for failover

### Deployment Strategies

**1. Multi-Cluster (Separate Clusters):**
```bash
# Cluster 1 (US-East)
helm install streamspace-agent-us-east streamspace/k8s-agent \
  --set agent.id=k8s-us-east \
  --set agent.region=us-east-1 \
  --kubeconfig ~/.kube/config-us-east \
  -n streamspace

# Cluster 2 (EU-West)
helm install streamspace-agent-eu-west streamspace/k8s-agent \
  --set agent.id=k8s-eu-west \
  --set agent.region=eu-west-1 \
  --kubeconfig ~/.kube/config-eu-west \
  -n streamspace
```

**2. Multi-Namespace (Same Cluster):**
```bash
# Tenant A
helm install streamspace-agent-tenant-a streamspace/k8s-agent \
  --set agent.id=k8s-tenant-a \
  --set agent.namespace=tenant-a \
  -n tenant-a

# Tenant B
helm install streamspace-agent-tenant-b streamspace/k8s-agent \
  --set agent.id=k8s-tenant-b \
  --set agent.namespace=tenant-b \
  -n tenant-b
```

**3. High Availability (Active-Standby):**
```bash
# Active agent
helm install streamspace-agent-primary streamspace/k8s-agent \
  --set agent.id=k8s-primary \
  --set agent.priority=high \
  -n streamspace

# Standby agent (same cluster, different node)
helm install streamspace-agent-standby streamspace/k8s-agent \
  --set agent.id=k8s-standby \
  --set agent.priority=low \
  --set affinity.podAntiAffinity.enabled=true \
  -n streamspace
```

### Load Balancing

Control Plane automatically distributes sessions across agents based on:
- Agent capacity (CPU, memory, max sessions)
- Agent region (prefer same region as user)
- Agent load (active sessions count)
- Agent health (only route to "online" agents)

---

## Appendix

### Environment Variable Quick Reference

```bash
# REQUIRED
AGENT_ID=k8s-prod-us-east-1
CONTROL_PLANE_URL=wss://streamspace.example.com

# Platform
PLATFORM=kubernetes
REGION=us-east-1
NAMESPACE=streamspace

# Behavior
HEARTBEAT_INTERVAL=30s
RECONNECT_DELAY=5s
MAX_RECONNECT_DELAY=5m
MAX_SESSIONS=100

# Session Defaults
SESSION_DEFAULT_CPU=1000m
SESSION_DEFAULT_MEMORY=2Gi
SESSION_DEFAULT_STORAGE=10Gi
SESSION_STORAGE_CLASS=nfs

# VNC
VNC_PORT=5900
VNC_TUNNEL_TIMEOUT=1h

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

### Troubleshooting Checklist

- [ ] Agent pod is running (`kubectl get pods`)
- [ ] Agent logs show no errors (`kubectl logs`)
- [ ] Agent connected to Control Plane (check status: online)
- [ ] RBAC permissions configured correctly
- [ ] Network connectivity to Control Plane works
- [ ] TLS certificate on Control Plane is valid
- [ ] StorageClass exists for PVCs
- [ ] Resource quotas not exceeded
- [ ] Session image is accessible
- [ ] VNC port (5900) is exposed in session pods

### Useful Commands

```bash
# Agent status
kubectl get pods -n streamspace -l component=k8s-agent
kubectl logs -n streamspace -l component=k8s-agent -f

# Sessions created by agent
kubectl get pods -n streamspace -l app=session

# Agent registration status
curl -H "Authorization: Bearer $JWT" \
  https://streamspace.example.com/api/v1/agents

# Test agent connectivity
kubectl exec -n streamspace -it streamspace-k8s-agent-<pod> -- \
  wget -O- https://streamspace.example.com/api/v1/health

# View agent metrics
kubectl port-forward -n streamspace svc/streamspace-k8s-agent 9090:9090
curl http://localhost:9090/metrics
```

---

**For more information:**
- **Deployment Guide**: `docs/V2_DEPLOYMENT_GUIDE.md`
- **Architecture Reference**: `docs/V2_ARCHITECTURE.md`
- **Migration Guide**: `docs/V2_MIGRATION_GUIDE.md`
- **API Reference**: `api/API_REFERENCE.md`

**Support**: https://github.com/JoshuaAFerguson/streamspace/issues

---

**StreamSpace v2.0 Agent Guide** - Comprehensive guide for agent deployment and management
Last Updated: 2025-11-21
