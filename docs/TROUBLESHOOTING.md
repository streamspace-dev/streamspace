# StreamSpace v2.0-beta Troubleshooting Guide

**Version**: 2.0.0-beta
**Date**: 2025-11-21
**Last Updated**: Integration Testing Wave 9

---

## Overview

This guide documents common issues encountered during v2.0-beta development and deployment, along with their solutions. All issues listed here have been fixed in the current release, but this guide helps you verify fixes and troubleshoot similar problems.

**Integration Testing Status:**
- **Phase**: 10 - v2.0-beta Integration Testing
- **Bugs Fixed**: 4 (3 P0, 1 P1)
- **Scenarios Complete**: 1/8

---

## Table of Contents

1. [Deployment Issues](#deployment-issues)
2. [K8s Agent Issues](#k8s-agent-issues)
3. [Authentication Issues](#authentication-issues)
4. [Session Management Issues](#session-management-issues)
5. [VNC Connection Issues](#vnc-connection-issues)
6. [Database Issues](#database-issues)
7. [Helm Chart Issues](#helm-chart-issues)
8. [Network and Connectivity](#network-and-connectivity)

---

## Deployment Issues

### Helm Chart Not Compatible with v2.0-beta

**Status**: ✅ FIXED (Integration Wave 7-8)
**Severity**: P0 - CRITICAL BLOCKER

**Symptoms:**
- Helm install fails with "Chart.yaml file is missing" or template errors
- Deployment script tries to set `k8sAgent.*` values but chart doesn't recognize them
- Chart deploys v1.x `controller` component instead of v2.0 `k8sAgent`
- NATS pods deploy (v1.x event system, deprecated in v2.0)

**Root Cause:**
Helm chart was not updated for v2.0-beta architecture. Chart still defined v1.x components (kubernetes-controller, NATS) but deployment scripts expected v2.0 components (k8sAgent, WebSocket communication).

**Solution:**

The Helm chart has been updated with the following changes:

1. **Removed v1.x Components:**
   - `chart/templates/nats.yaml` (122 lines) - v1.x event system
   - `controller` now disabled by default

2. **Added v2.0 Components:**
   - `chart/templates/k8s-agent-deployment.yaml` (118 lines)
   - `chart/templates/k8s-agent-serviceaccount.yaml` (17 lines)
   - Updated `chart/templates/rbac.yaml` (62 lines for agent)
   - Updated `chart/values.yaml` (125+ lines for k8sAgent config)

**Verification:**
```bash
# Validate chart structure
helm lint ./chart

# Check for k8sAgent section
grep "^k8sAgent:" chart/values.yaml
# Expected: k8sAgent configuration block

# Verify NATS is removed
ls chart/templates/nats.yaml
# Expected: No such file or directory

# Deploy with k8sAgent enabled
helm install streamspace ./chart \
  --namespace streamspace \
  --create-namespace \
  --set k8sAgent.enabled=true \
  --dry-run --debug
# Expected: No errors, k8s-agent deployment rendered
```

**Reference:**
- Bug Report: `BUG_REPORT_P0_HELM_CHART_v2.md`
- Commits: f611b65, 4ab1cbc

---

## K8s Agent Issues

### K8s Agent Crashes on Startup with Nil Pointer

**Status**: ✅ FIXED (Integration Wave 7)
**Severity**: P0 - CRITICAL BLOCKER

**Symptoms:**
- Agent pod crashes immediately after startup
- Pod status shows `CrashLoopBackOff`
- Agent logs show:
  ```
  panic: runtime error: invalid memory address or nil pointer dereference
  [signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x...]
  goroutine 1 [running]:
  main.startHeartbeat(...)
  ```
- Error occurs when agent tries to access `config.HeartbeatInterval`

**Root Cause:**
The agent's `HeartbeatInterval` configuration field was not being initialized from the environment variable `HEARTBEAT_INTERVAL`. The field remained `nil`, causing a panic when the heartbeat goroutine tried to use it.

**Affected Code:**
```go
// agents/k8s-agent/main.go (BEFORE FIX):
config := &Config{
    ControlPlaneURL: os.Getenv("CONTROL_PLANE_URL"),
    AgentID:         os.Getenv("AGENT_ID"),
    // HeartbeatInterval NOT loaded - causes nil pointer!
}

// Later in code:
time.Sleep(config.HeartbeatInterval) // PANIC! nil pointer
```

**Solution (Applied):**

Fixed in `agents/k8s-agent/main.go`:
```go
// Load HeartbeatInterval from env var with 30s default
heartbeatInterval := 30 * time.Second
if envInterval := os.Getenv("HEARTBEAT_INTERVAL"); envInterval != "" {
    if d, err := time.ParseDuration(envInterval); err == nil {
        heartbeatInterval = d
    }
}
config.HeartbeatInterval = heartbeatInterval
```

**Verification:**
```bash
# Check agent pod is running (not crashing)
kubectl get pods -n streamspace -l app.kubernetes.io/component=k8s-agent
# Expected: STATUS = Running, RESTARTS = 0

# Check agent logs for successful startup
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent --tail=30
# Expected: No panic errors, "Agent started" message

# Verify heartbeat is working
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent | grep heartbeat
# Expected: Periodic heartbeat messages every 30s
```

**Prevention:**
- Always initialize config fields with defaults before reading env vars
- Add validation checks for required config fields at startup
- Use `viper` or similar library for config management with defaults

**Reference:**
- Bug Report: `BUG_REPORT_P0_K8S_AGENT_CRASH.md`
- Commit: 4ab1cbc (Integration Wave 7)

---

### Agent Shows "Offline" in UI

**Symptoms:**
- Agent pod is running but UI shows agent status as "offline"
- Agent logs show WebSocket connection errors
- Control Plane logs show no agent heartbeats received

**Possible Causes:**

#### 1. WebSocket Connection Failure

**Check:**
```bash
# Check agent logs for connection errors
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent | grep -i "websocket\|connection"

# Common errors:
# "Failed to connect to Control Plane"
# "WebSocket handshake failed"
# "Connection refused"
```

**Solution:**
```bash
# Verify Control Plane URL is correct
kubectl get deployment streamspace-k8s-agent -n streamspace -o yaml | grep CONTROL_PLANE_URL

# Should be: ws://streamspace-api:8000/agent/ws (internal)
# Or: wss://streamspace.example.com/agent/ws (external)

# Update if incorrect:
kubectl set env deployment/streamspace-k8s-agent \
  CONTROL_PLANE_URL=ws://streamspace-api:8000/agent/ws \
  -n streamspace
```

#### 2. Agent HeartbeatInterval Too Long

**Check:**
```bash
# Check heartbeat interval setting
kubectl get deployment streamspace-k8s-agent -n streamspace -o yaml | grep HEARTBEAT_INTERVAL

# Default: 30s
# If > 60s, Control Plane may timeout and mark agent offline
```

**Solution:**
```bash
# Set to 30s (recommended)
kubectl set env deployment/streamspace-k8s-agent \
  HEARTBEAT_INTERVAL=30s \
  -n streamspace
```

#### 3. Network Policy Blocking Traffic

**Check:**
```bash
# Check if NetworkPolicy exists
kubectl get networkpolicy -n streamspace

# Test connectivity from agent to API
kubectl exec -n streamspace deployment/streamspace-k8s-agent -- \
  wget -O- http://streamspace-api:8000/health
```

**Solution:**
If NetworkPolicy is blocking, ensure it allows agent → API traffic:
```yaml
# Allow k8s-agent → api traffic
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-agent-to-api
  namespace: streamspace
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: api
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app.kubernetes.io/component: k8s-agent
    ports:
    - protocol: TCP
      port: 8000
```

---

## Authentication Issues

### Admin Login Fails with Correct Credentials

**Status**: ✅ FIXED (Integration Wave 8)
**Severity**: P1 - HIGH

**Symptoms:**
- UI shows "Invalid username or password" error
- Admin user exists in database (verified via psql)
- Correct credentials from `streamspace-admin-credentials` secret don't work
- API logs show password mismatch

**Root Cause:**
Admin password was passed as a plain environment variable in the API deployment (`ADMIN_PASSWORD={{ .Values.adminPassword }}`), but the authentication code expected the password from a Kubernetes secret. This caused a mismatch between the password stored in the database (from secret) and the password the API was checking (from values.yaml).

**Affected Configuration:**
```yaml
# chart/templates/api-deployment.yaml (BEFORE FIX):
env:
  - name: ADMIN_PASSWORD
    value: {{ .Values.adminPassword | quote }}  # WRONG: from values.yaml

# Authentication checked:
# secretPassword != valuesPassword → Login fails!
```

**Solution (Applied):**

Fixed in `chart/templates/api-deployment.yaml`:
```yaml
# chart/templates/api-deployment.yaml (AFTER FIX):
env:
  - name: ADMIN_PASSWORD
    valueFrom:
      secretKeyRef:
        name: {{ include "streamspace.fullname" . }}-admin-credentials
        key: password  # CORRECT: from secret
```

**Verification:**
```bash
# 1. Get admin credentials from secret
USERNAME=$(kubectl get secret streamspace-admin-credentials -n streamspace -o jsonpath='{.data.username}' | base64 -d)
PASSWORD=$(kubectl get secret streamspace-admin-credentials -n streamspace -o jsonpath='{.data.password}' | base64 -d)

echo "Username: $USERNAME"
echo "Password: $PASSWORD"

# 2. Verify API pod has correct env var
kubectl exec -n streamspace deployment/streamspace-api -- env | grep ADMIN_PASSWORD
# Expected: Should NOT show password (it's from secret)

# 3. Test login via API
curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}"
# Expected: {"token":"...", "user":{...}}

# 4. Test login via UI
# Open http://localhost:8080
# Login with username/password from step 1
# Expected: Successful login, redirected to dashboard
```

**Prevention:**
- Always use Kubernetes secrets for sensitive data (passwords, tokens)
- Never pass secrets via `values.yaml` (values are often committed to Git)
- Use `secretKeyRef` or `secretRef` in Helm templates
- Test authentication after any Helm chart changes

**Reference:**
- Bug Report: `BUG_REPORT_P1_ADMIN_AUTH.md`
- Commit: 617d16e (Integration Wave 8)

---

### JWT Token Expires Immediately

**Symptoms:**
- Login succeeds but subsequent API calls return 401 Unauthorized
- UI shows "Session expired" immediately after login
- JWT token appears to be expired before it's even used

**Possible Causes:**

#### 1. JWT_SECRET Not Set

**Check:**
```bash
# Verify JWT_SECRET exists in API pod
kubectl exec -n streamspace deployment/streamspace-api -- env | grep JWT_SECRET
# Should show: JWT_SECRET=<base64-encoded-secret>
```

**Solution:**
```bash
# If missing, check secret exists
kubectl get secret streamspace-secrets -n streamspace -o yaml | grep jwt-secret

# If secret exists, restart API pods to reload env vars
kubectl rollout restart deployment/streamspace-api -n streamspace
```

#### 2. System Clock Skew

**Check:**
```bash
# Check if pod time matches host time
kubectl exec -n streamspace deployment/streamspace-api -- date
date
# Times should match within a few seconds
```

**Solution:**
If times are significantly different, check node clock synchronization (NTP).

---

## Session Management Issues

### Sessions Stuck in "Pending" State

**Status**: ✅ FIXED (Integration Wave 8)
**Severity**: P0 - CRITICAL BLOCKER

**Symptoms:**
- Create session via UI or API
- Session remains in "pending" state indefinitely
- No session pods are created in namespace
- API logs show: "controller not available" or "session provisioner unavailable"
- Sessions table in database shows state = "pending"

**Root Cause:**
API session creation handler was calling v1.x controller code (CRD-based workflow) instead of v2.0 agent-based workflow. The handler expected a Kubernetes controller to exist and watch Session CRDs, but v2.0 architecture uses agents that connect via WebSocket to receive commands.

**Affected Code:**
```go
// api/internal/handlers/sessions.go (BEFORE FIX - v1.x):
func CreateSession(c *gin.Context) {
    // 1. Create Session CRD in Kubernetes
    sessionCRD := &v1alpha1.Session{...}
    k8sClient.Create(ctx, sessionCRD)

    // 2. Wait for controller to update status
    // BUG: No controller exists in v2.0!
    // Session stuck in pending forever
}
```

**Solution (Applied):**

Rewrote session creation handler for v2.0 agent-based workflow:
```go
// api/internal/handlers/sessions.go (AFTER FIX - v2.0):
func CreateSession(c *gin.Context) {
    // 1. Create session record in database
    session := &models.Session{
        UserID: user.ID,
        TemplateID: template.ID,
        State: "pending",
    }
    db.Create(session)

    // 2. Find available agent
    agent := findAvailableAgent(template.Platform)
    if agent == nil {
        return errors.New("no available agents")
    }

    // 3. Send start_session command to agent via WebSocket
    command := &AgentCommand{
        Type: "start_session",
        SessionID: session.ID,
        Template: template,
    }
    agentHub.SendCommand(agent.ID, command)

    // 4. Agent provisions pod and reports back via WebSocket
    // Session state updated asynchronously
}
```

**Verification:**
```bash
# 1. Verify agent is registered and online
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/agents
# Expected: At least one agent with status="online"

# 2. Create test session
curl -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "testuser",
    "template": "firefox-browser",
    "state": "running"
  }'
# Expected: {"id":"sess-123",...,"state":"pending"}

# 3. Check session moves to "running" (within 30s)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/sessions/sess-123
# Expected: "state":"running"

# 4. Verify session pod was created
kubectl get pods -n streamspace -l app=session,session-id=sess-123
# Expected: Pod running with 1/1 ready

# 5. Check agent logs for session creation
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent | grep sess-123
# Expected:
# "Received start_session command for sess-123"
# "Creating deployment for sess-123"
# "Session sess-123 started successfully"
```

**Prevention:**
- Update all API handlers when changing architecture (v1.x → v2.0)
- Add integration tests that verify end-to-end workflows
- Document architecture changes in MIGRATION_GUIDE.md
- Use feature flags to support both architectures during transition

**Reference:**
- Bug Report: `BUG_REPORT_P0_MISSING_CONTROLLER.md`
- Commit: 617d16e (Integration Wave 8)

---

### Session Pod Fails to Start

**Symptoms:**
- Session moves from "pending" to "starting" but never reaches "running"
- Session pod exists but shows status `ImagePullBackOff`, `CrashLoopBackOff`, or `Pending`
- Agent logs show "Failed to start session" error

**Possible Causes:**

#### 1. Image Pull Failure

**Check:**
```bash
# Check pod events
kubectl describe pod <session-pod-name> -n streamspace | grep -A5 Events

# Common errors:
# "Failed to pull image: rpc error: code = Unknown desc = Error response from daemon: pull access denied"
# "ErrImagePull"
# "ImagePullBackOff"
```

**Solution:**
```bash
# Verify template image is accessible
kubectl run test-pull --image=<template-image> --restart=Never -n streamspace
kubectl logs test-pull -n streamspace
kubectl delete pod test-pull -n streamspace

# If using private registry, create image pull secret:
kubectl create secret docker-registry regcred \
  --docker-server=<registry> \
  --docker-username=<username> \
  --docker-password=<password> \
  -n streamspace

# Update template to use image pull secret
```

#### 2. Insufficient Resources

**Check:**
```bash
# Check node resources
kubectl describe nodes | grep -A5 "Allocated resources"

# Check if pod is pending due to resources
kubectl describe pod <session-pod-name> -n streamspace | grep "FailedScheduling"
```

**Solution:**
```bash
# Reduce session resource requests
# Or add more nodes to cluster
# Or scale down other workloads
```

#### 3. PVC Not Binding

**Check:**
```bash
# Check PVC status
kubectl get pvc -n streamspace | grep <session-id>

# If status is "Pending":
kubectl describe pvc <pvc-name> -n streamspace
```

**Solution:**
```bash
# Check storage class exists
kubectl get storageclass

# If no storage class, create one:
# For local development (Docker Desktop):
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: local-storage
provisioner: docker.io/hostpath
volumeBindingMode: Immediate
EOF
```

---

## VNC Connection Issues

### VNC Viewer Shows "Connecting..." Indefinitely

**Symptoms:**
- Session state is "running" and pod is ready
- VNC viewer in UI shows "Connecting..." but never displays desktop
- Browser console may show WebSocket errors
- No VNC traffic visible in Network tab

**Possible Causes:**

#### 1. VNC Tunnel Not Initialized

**Check:**
```bash
# Check agent logs for VNC tunnel messages
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent | grep -i "vnc tunnel"

# Expected to see:
# "VNC tunnel initialized for session sess-123"
# "VNC connection established: UI -> Control Plane -> Agent -> Pod"
```

**Solution:**
If no VNC tunnel messages, check:
```bash
# 1. Verify session has agent_id set
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/sessions/sess-123 | jq '.agent_id'
# Expected: "k8s-agent-1" (not null)

# 2. Restart agent to reinitialize tunnels
kubectl rollout restart deployment/streamspace-k8s-agent -n streamspace
```

#### 2. Session Pod VNC Server Not Running

**Check:**
```bash
# Check if VNC server is listening in session pod
kubectl exec -n streamspace <session-pod> -- netstat -ln | grep 5900
# Expected: tcp        0      0 0.0.0.0:5900            0.0.0.0:*               LISTEN

# Test VNC connection from agent
kubectl exec -n streamspace deployment/streamspace-k8s-agent -- \
  nc -zv <session-pod-ip> 5900
# Expected: Connection to <pod-ip> 5900 port [tcp/*] succeeded!
```

**Solution:**
If VNC server not running, check session pod logs:
```bash
kubectl logs <session-pod> -n streamspace

# Common issues:
# - X server failed to start
# - Display :1 already in use
# - VNC password not set
```

#### 3. WebSocket Proxy Error

**Check:**
```bash
# Check Control Plane logs for VNC proxy errors
kubectl logs -n streamspace -l app.kubernetes.io/component=control-plane | grep vnc_proxy

# Common errors:
# "VNC proxy: failed to connect to agent"
# "VNC proxy: session not found"
# "VNC proxy: agent not online"
```

**Solution:**
```bash
# Verify VNC proxy endpoint is reachable
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: test" \
  ws://localhost:8000/vnc/sess-123
# Expected: 101 Switching Protocols
```

---

## Database Issues

### Database Connection Refused

**Symptoms:**
- API pods crash with "connection refused" errors
- Logs show: `dial tcp <db-host>:<db-port>: connect: connection refused`
- API deployment shows `CrashLoopBackOff`

**Check:**
```bash
# 1. Verify database is running
kubectl get pods -n streamspace -l app=postgres
# Expected: STATUS = Running

# 2. Verify database service exists
kubectl get svc -n streamspace | grep postgres
# Expected: streamspace-postgres ClusterIP <IP> <port>/TCP

# 3. Test database connectivity
kubectl run -it --rm debug --image=postgres:14 --restart=Never -n streamspace -- \
  psql -h streamspace-postgres -U streamspace -d streamspace
# Should connect successfully
```

**Solution:**
```bash
# If database pod is not running:
kubectl logs -n streamspace -l app=postgres

# If database service is missing:
kubectl apply -f chart/templates/postgres-service.yaml

# If connection works from debug pod but not from API:
# Check API database configuration:
kubectl get secret streamspace-secrets -n streamspace -o yaml | grep -A5 database
```

---

### Database Migrations Not Applied

**Symptoms:**
- API starts but crashes when trying to access database
- Logs show: `table "agents" does not exist` or similar
- Database tables for v2.0 (agents, agent_commands) missing

**Check:**
```bash
# Connect to database and list tables
kubectl exec -n streamspace -it streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace -c "\\dt"

# Check if v2.0 tables exist:
# - agents
# - agent_commands
# - (87 total tables expected)
```

**Solution:**
```bash
# Run v2.0 migrations
kubectl exec -n streamspace -it streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace <<EOF
-- Add agents table
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

-- Add agent_commands table
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

-- Update sessions table for v2.0
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS platform VARCHAR(50),
ADD COLUMN IF NOT EXISTS platform_metadata JSONB;
EOF

# Restart API pods to reconnect
kubectl rollout restart deployment/streamspace-api -n streamspace
```

---

## Helm Chart Issues

### Helm Template Rendering Fails

**Symptoms:**
- `helm install` or `helm upgrade` fails with template errors
- Error messages like:
  - "nil pointer evaluating interface {}.enabled"
  - "template rendering error"
  - "Chart.yaml file is missing" (Helm v4.0.0 confusing error)

**Check:**
```bash
# Validate chart structure
helm lint ./chart
# Should show: "1 chart(s) linted, 0 chart(s) failed"

# Dry-run to see rendered templates
helm install streamspace ./chart \
  --namespace streamspace \
  --dry-run --debug \
  --set k8sAgent.enabled=true
# Should render without errors
```

**Solution:**
Common fixes:
1. **Missing values section**: Ensure all referenced values exist in `values.yaml`
2. **Typos in template**: Check `.Values.<section>.<key>` matches `values.yaml`
3. **Conditional rendering**: Use `{{- if .Values.x.enabled }}` before accessing `.Values.x.something`

Example fix:
```yaml
# WRONG (causes nil pointer if k8sAgent not defined):
{{- if .Values.k8sAgent.enabled }}

# RIGHT (checks if section exists first):
{{- if and .Values.k8sAgent .Values.k8sAgent.enabled }}
```

---

## Network and Connectivity

### Agent Can't Reach Control Plane URL

**Symptoms:**
- Agent logs show: "Failed to connect to Control Plane at <URL>"
- WebSocket connection times out or connection refused

**Check:**
```bash
# Test connectivity from agent pod
kubectl exec -n streamspace deployment/streamspace-k8s-agent -- \
  wget -O- http://streamspace-api:8000/health
# Expected: {"service":"streamspace-api","status":"healthy"}

# Check DNS resolution
kubectl exec -n streamspace deployment/streamspace-k8s-agent -- \
  nslookup streamspace-api
# Expected: Resolves to ClusterIP
```

**Solution:**
If connection fails:
```bash
# 1. Verify API service exists
kubectl get svc streamspace-api -n streamspace
# Expected: ClusterIP service on port 8000

# 2. Check API pods are running
kubectl get pods -n streamspace -l app.kubernetes.io/component=control-plane

# 3. Update agent CONTROL_PLANE_URL
kubectl set env deployment/streamspace-k8s-agent \
  CONTROL_PLANE_URL=ws://streamspace-api:8000/agent/ws \
  -n streamspace
```

---

## General Debugging Commands

### Essential kubectl Commands

```bash
# Get all resources in streamspace namespace
kubectl get all -n streamspace

# Check pod logs
kubectl logs -n streamspace <pod-name> --tail=100 -f

# Check pod events
kubectl describe pod <pod-name> -n streamspace | grep -A10 Events

# Get pod YAML
kubectl get pod <pod-name> -n streamspace -o yaml

# Exec into pod
kubectl exec -it -n streamspace <pod-name> -- /bin/sh

# Port forward to service
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000
kubectl port-forward -n streamspace svc/streamspace-ui 8080:8080

# Check secrets
kubectl get secrets -n streamspace
kubectl describe secret <secret-name> -n streamspace

# Restart deployment
kubectl rollout restart deployment/<deployment-name> -n streamspace
```

### Helm Debugging Commands

```bash
# List Helm releases
helm list -n streamspace

# Get release values
helm get values streamspace -n streamspace

# Get release manifest
helm get manifest streamspace -n streamspace

# Rollback to previous version
helm rollback streamspace -n streamspace

# Upgrade with --wait and --debug
helm upgrade streamspace ./chart \
  --namespace streamspace \
  --wait --debug \
  --set k8sAgent.enabled=true
```

---

## Support and Resources

If you encounter an issue not covered in this guide:

1. **Check Integration Test Report**: `INTEGRATION_TEST_REPORT_V2_BETA.md`
2. **Check Bug Reports**: `BUG_REPORT_*.md` files in repository root
3. **Check Deployment Summary**: `DEPLOYMENT_SUMMARY_V2_BETA.md`
4. **GitHub Issues**: https://github.com/streamspace-dev/streamspace/issues
5. **Documentation**: https://docs.streamspace.io

### Useful Log Grep Patterns

```bash
# Find errors in logs
kubectl logs -n streamspace <pod-name> | grep -i error

# Find panics/crashes
kubectl logs -n streamspace <pod-name> | grep -i "panic\|fatal"

# Find WebSocket issues
kubectl logs -n streamspace <pod-name> | grep -i "websocket\|ws:"

# Find database issues
kubectl logs -n streamspace <pod-name> | grep -i "database\|postgres\|sql"

# Find authentication issues
kubectl logs -n streamspace <pod-name> | grep -i "auth\|jwt\|token"
```

---

**Troubleshooting Guide Version**: 1.0
**Last Updated**: 2025-11-21 (Integration Testing Wave 9)
**StreamSpace Version**: v2.0.0-beta
