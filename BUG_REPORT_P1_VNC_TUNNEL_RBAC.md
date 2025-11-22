# Bug Report: P1-VNC-RBAC-001 - Agent Needs pods/portforward Permission for VNC Tunneling

**Priority**: P1 (High - VNC Streaming Impacted)
**Status**: 🟡 ACTIVE - Sessions Working, VNC Tunnel Failing
**Component**: RBAC / K8s Agent / VNC Proxy
**Discovered**: 2025-11-22 04:49:28 UTC
**Reporter**: Validator Agent
**Impact**: VNC streaming through agent tunnel fails, direct pod access works

---

## Executive Summary

After P0-MANIFEST-001 was fixed, sessions are now provisioning correctly with pods running successfully. However, the agent's VNC tunnel creation fails due to missing RBAC permissions. The agent cannot create port-forwards to session pods, preventing VNC streaming through the control plane's VNC proxy.

**Impact**: 🟡 **MEDIUM** - Sessions functional, VNC tunneling through agent blocked

**Workaround**: Direct pod access via service works for VNC connectivity

---

## Error Details

### Agent Log Error

```
2025/11/22 04:49:28 [VNCTunnel] Port-forward error for admin-firefox-browser-d40f9190: error upgrading connection: pods "admin-firefox-browser-d40f9190-584bc6576f-5b9z9" is forbidden: User "system:serviceaccount:streamspace:streamspace-agent" cannot create resource "pods/portforward" in API group "" in the namespace "streamspace"
2025/11/22 04:49:58 [VNCHandler] Failed to create VNC tunnel for session admin-firefox-browser-d40f9190: timeout waiting for port-forward
```

### Full Error Breakdown

**Service Account**: `system:serviceaccount:streamspace:streamspace-agent`
**Resource**: `pods/portforward`
**Action**: `create`
**Namespace**: `streamspace`
**Result**: **403 Forbidden**

### Affected Session

**Session ID**: `admin-firefox-browser-d40f9190`
**Pod**: `admin-firefox-browser-d40f9190-584bc6576f-5b9z9` (1/1 Running)
**Service**: `admin-firefox-browser-d40f9190` (ClusterIP: 10.110.232.135)
**Status**: Pod running successfully, VNC tunnel creation failed

---

## Root Cause Analysis

### VNC Tunnel Architecture (v2.0-beta)

StreamSpace v2.0-beta uses a **centralized VNC proxy** architecture:

1. **Session Pod**: Runs containerized application with VNC server (port 3000)
2. **Agent VNC Tunnel**: Creates port-forward from agent to session pod VNC port
3. **Control Plane VNC Proxy**: Proxies VNC traffic from users to agent tunnel
4. **User Browser**: Connects to control plane VNC proxy URL

**Flow**:
```
User Browser → Control Plane VNC Proxy → Agent VNC Tunnel → Session Pod VNC Server
```

### Current RBAC Permissions

**File**: `agents/k8s-agent/deployments/rbac.yaml`

**Current Permissions**:
```yaml
rules:
# StreamSpace CRDs
- apiGroups: ["stream.space"]
  resources: ["templates", "sessions"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Pods - for monitoring
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]

# Pod logs - for debugging
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get", "list"]
```

**Missing Permission**:
```yaml
- apiGroups: [""]
  resources: ["pods/portforward"]
  verbs: ["create", "get"]
```

---

## Impact Assessment

### Severity: P1 (High)

**Justification**:
- ✅ Sessions provision successfully (P0 fixed)
- ✅ Pods running and healthy
- ✅ Services created
- ❌ VNC streaming through control plane blocked
- ✅ Workaround available (direct pod access)

**Why Not P0**:
- Core session provisioning works
- Pods are functional
- Direct VNC access possible via service
- This is a VNC proxy feature issue, not a core provisioning issue

**Why P1**:
- VNC proxy is a key v2.0-beta feature
- Centralized VNC streaming is the designed architecture
- Users cannot access sessions through the control plane UI
- Production deployment requires this working

---

## Affected Features

1. **VNC Streaming via Control Plane** - 🔴 BROKEN
2. **Session Provisioning** - ✅ WORKING
3. **Direct Pod VNC Access** - ✅ WORKING (workaround)
4. **Control Plane VNC Proxy** - 🔴 BLOCKED (no tunnel to pods)

---

## Current Behavior vs Expected Behavior

### Current Behavior

1. ✅ User creates session via API
2. ✅ Session created in database (state: pending)
3. ✅ Agent receives WebSocket command
4. ✅ Agent parses template manifest
5. ✅ Agent creates deployment and service
6. ✅ Pod starts and becomes ready (6 seconds)
7. ✅ Agent marks session as "started successfully"
8. ❌ **Agent attempts to create VNC tunnel → RBAC error**
9. ❌ **VNC tunnel creation fails**
10. ❌ User cannot access VNC via control plane

### Expected Behavior

1. ✅ User creates session via API
2. ✅ Session created in database
3. ✅ Agent provisions pod and service
4. ✅ Agent creates VNC tunnel to pod
5. ✅ Control plane VNC proxy connects to agent tunnel
6. ✅ User accesses VNC via control plane URL (e.g., `https://streamspace.local/sessions/{id}/vnc`)

---

## Recommended Fix

### Add pods/portforward Permission to Agent RBAC

**File**: `agents/k8s-agent/deployments/rbac.yaml`

**Add to `rules` section**:
```yaml
# Port-forward - for VNC tunneling
- apiGroups: [""]
  resources: ["pods/portforward"]
  verbs: ["create", "get"]
```

**Complete Updated RBAC**:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: streamspace-agent
  namespace: streamspace
  labels:
    app: streamspace
    component: k8s-agent
rules:
# StreamSpace CRDs - Templates and Sessions
- apiGroups: ["stream.space"]
  resources: ["templates"]
  verbs: ["get", "list", "watch"]

- apiGroups: ["stream.space"]
  resources: ["sessions"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

- apiGroups: ["stream.space"]
  resources: ["sessions/status"]
  verbs: ["get", "update", "patch"]

# Deployments - for session containers
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Services - for session networking
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Pods - for monitoring session status
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]

# Pod logs - for debugging
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get", "list"]

# Port-forward - for VNC tunneling  ← ADD THIS
- apiGroups: [""]
  resources: ["pods/portforward"]
  verbs: ["create", "get"]

# PersistentVolumeClaims - for persistent user storage
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# ConfigMaps - for session configuration
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Secrets - for session credentials
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**Helm Chart** (`chart/templates/rbac.yaml`): Apply same change

---

## Deployment Steps

### 1. Update RBAC Manifest

```bash
kubectl apply -f agents/k8s-agent/deployments/rbac.yaml
```

### 2. Restart Agent Pod (Pick Up New Permissions)

```bash
kubectl delete pods -n streamspace -l app.kubernetes.io/component=k8s-agent
kubectl rollout status deployment/streamspace-k8s-agent -n streamspace
```

### 3. Test VNC Tunnel Creation

Create a new session and verify VNC tunnel succeeds:

```bash
# Create session
curl -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}'

# Check agent logs for VNC tunnel success
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent | grep VNCTunnel
```

**Expected Log**:
```
[VNCTunnel] Creating tunnel for session: admin-firefox-browser-...
[VNCTunnel] Found pod ... with VNC port 3000
[VNCTunnel] Port-forward established for session ...
[VNCHandler] VNC tunnel ready for session ...
```

---

## Validation Plan

### Test 1: VNC Tunnel Creation

**Steps**:
1. Apply RBAC update
2. Restart agent pod
3. Create new session
4. Check agent logs for VNC tunnel success

**Expected**: VNC tunnel created without RBAC errors

---

### Test 2: Control Plane VNC Proxy Access

**Steps**:
1. Create session
2. Wait for pod to be ready
3. Access VNC via control plane URL
4. Verify VNC stream displays

**Expected**: VNC accessible via control plane proxy

---

### Test 3: Multi-Session VNC Tunnels

**Steps**:
1. Create 3 concurrent sessions
2. Verify all VNC tunnels created
3. Access each session's VNC via control plane

**Expected**: All tunnels working concurrently

---

## Security Considerations

### Permission Scope

**Resource**: `pods/portforward`
**Verbs**: `create`, `get`
**Namespace**: `streamspace` (scoped by Role, not ClusterRole)

**Why Safe**:
- Agent already has `pods` `get` permission (can list pods)
- Port-forward is a standard Kubernetes debugging/access mechanism
- Limited to streamspace namespace (not cluster-wide)
- Agent creates port-forwards only for sessions it manages
- No data modification (read-only access to pod traffic)

**Security Best Practice**:
- Use Role (not ClusterRole) to limit to streamspace namespace
- Agent uses least-privilege service account
- Port-forwards are temporary (tied to agent connection lifetime)

---

## Alternative Approaches (Not Recommended)

### Alternative 1: Direct Pod Access via Service (Current Workaround)

**Pros**:
- No RBAC changes needed
- Works immediately

**Cons**:
- ❌ Bypasses control plane VNC proxy
- ❌ Users must access pods directly (not via UI)
- ❌ No centralized VNC streaming
- ❌ Defeats v2.0-beta architecture design
- ❌ No VNC traffic routing through control plane

---

### Alternative 2: Service-Based VNC Proxy (Architectural Change)

**Approach**: Control plane proxies to session service instead of agent port-forward

**Pros**:
- No agent port-forward needed
- Direct service-to-service routing

**Cons**:
- ❌ Requires significant architectural changes
- ❌ Agent VNC handler redesign needed
- ❌ Less flexible for cross-cluster scenarios
- ❌ High implementation cost

**Recommendation**: Not worth the effort, RBAC fix is simpler

---

## Technical Context

### Kubernetes Port-Forward

**What It Does**: Creates a tunnel from client to pod, forwarding traffic to a specific port

**Agent Use Case**:
```go
// Agent creates port-forward from itself to session pod VNC port
portForward := clientset.CoreV1().RESTClient().Post().
    Resource("pods").
    Namespace(namespace).
    Name(podName).
    SubResource("portforward")
```

**Control Plane Use Case**:
- Control plane VNC proxy connects to agent's port-forward tunnel
- Streams VNC traffic from user browser to session pod

---

### VNC Proxy Architecture (v2.0-beta)

**Components**:
1. **User Browser**: Connects to control plane VNC proxy endpoint
2. **Control Plane VNC Proxy**: Receives VNC requests, routes to agent tunnel
3. **Agent VNC Tunnel**: Port-forward from agent to session pod
4. **Session Pod**: Runs VNC server (e.g., port 3000)

**Why This Design**:
- Centralized access control (all traffic through control plane)
- Works across clusters (agents in different clusters)
- Single entry point for users (control plane URL)
- Firewall-friendly (outbound agent connections only)

---

## Dependencies

**Blocks**:
- VNC streaming through control plane UI
- E2E VNC accessibility testing (via control plane)
- Full integration testing completion

**Depends On**:
- ✅ P0-MANIFEST-001 (session provisioning) - FIXED
- ✅ P0-RBAC-001 (agent RBAC + API manifest) - FIXED

**Related Issues**:
- P0-RBAC-001 (added template/session CRD permissions) - ✅ FIXED
- P0-MANIFEST-001 (template manifest case mismatch) - ✅ FIXED

---

## Additional Notes

### Why Not Discovered Earlier

1. **P0 issues blocked testing**: Session provisioning was broken, never reached VNC tunnel stage
2. **Multi-step issue chain**: Required P0-RBAC-001 + P0-MANIFEST-001 fixes first
3. **VNC tunnel is late-stage operation**: Only attempted after pod is ready

### Priority Justification

**Why P1 (not P2)**:
- VNC proxy is a core v2.0-beta feature
- Production deployments require centralized VNC access
- Affects user experience significantly

**Why Not P0**:
- Session provisioning works (pods running)
- Workaround available (direct pod access)
- Not blocking core functionality

---

## Evidence

### Test Execution

**Session**: `admin-firefox-browser-d40f9190`
**Pod**: Running successfully (1/1 Ready)
**Service**: Created (ClusterIP: 10.110.232.135)
**VNC Tunnel**: Failed with RBAC error

### Agent Logs

```
2025/11/22 04:49:26 [StartSessionHandler] Session admin-firefox-browser-d40f9190 started successfully (pod: admin-firefox-browser-d40f9190-584bc6576f-5b9z9, IP: 10.1.2.176)
2025/11/22 04:49:26 [VNCHandler] Initializing VNC tunnel for session admin-firefox-browser-d40f9190
2025/11/22 04:49:28 [VNCTunnel] Creating tunnel for session: admin-firefox-browser-d40f9190
2025/11/22 04:49:28 [VNCTunnel] Found pod admin-firefox-browser-d40f9190-584bc6576f-5b9z9 with VNC port 3000
2025/11/22 04:49:28 [VNCTunnel] Port-forward error for admin-firefox-browser-d40f9190: error upgrading connection: pods "admin-firefox-browser-d40f9190-584bc6576f-5b9z9" is forbidden: User "system:serviceaccount:streamspace:streamspace-agent" cannot create resource "pods/portforward" in API group "" in the namespace "streamspace"
2025/11/22 04:49:58 [VNCHandler] Failed to create VNC tunnel for session admin-firefox-browser-d40f9190: timeout waiting for port-forward
```

---

## Conclusion

**Summary**: Agent needs `pods/portforward` RBAC permission to create VNC tunnels to session pods. Sessions are provisioning successfully, but VNC streaming through the control plane VNC proxy is blocked.

**Immediate Action Required**: Add `pods/portforward` permission to agent Role

**Fix Complexity**: Low (single RBAC permission addition)

**Risk**: Very Low (standard Kubernetes permission, scoped to namespace)

**Recommendation**: Deploy RBAC fix to unblock VNC streaming feature

---

**Generated**: 2025-11-22 04:55:00 UTC
**Validator**: Claude (v2-validator branch)
**Next Step**: Builder to add pods/portforward permission to agent RBAC
