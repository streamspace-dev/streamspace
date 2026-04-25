# Bug Report: P0-RBAC-001 - Agent Cannot Read Template CRDs

**Priority**: P0 (Critical - Blocks Session Provisioning)
**Status**: 🔴 ACTIVE - Blocking E2E VNC Streaming Validation
**Component**: RBAC / K8s Agent / Template CRDs
**Discovered**: 2025-11-22 04:07:36 UTC
**Reporter**: Validator Agent
**Impact**: **CRITICAL** - No sessions can be provisioned

---

## Executive Summary

The K8s agent cannot create sessions because it lacks RBAC permissions to read Template Custom Resources. When the API sends a `start_session` command without including the template manifest in the payload, the agent attempts to fetch the template from Kubernetes and fails with a **403 Forbidden** error.

**Impact**: 🔴 **BLOCKS** all session creation and E2E VNC streaming validation.

---

## Error Details

### Agent Log Error

```
2025/11/22 04:07:36 [StartSessionHandler] Warning: No templateManifest in payload, falling back to K8s fetch: failed to parse template manifest: invalid template spec
2025/11/22 04:07:36 [K8sAgent] Command cmd-84c934b1 failed: failed to get template firefox-browser: failed to get template firefox-browser: templates.stream.space "firefox-browser" is forbidden: User "system:serviceaccount:streamspace:streamspace-agent" cannot get resource "templates" in API group "stream.space" in the namespace "streamspace"
```

### Full Error Breakdown

**Service Account**: `system:serviceaccount:streamspace:streamspace-agent`
**Resource**: `templates.stream.space`
**Action**: `get`
**Namespace**: `streamspace`
**Result**: **403 Forbidden**

### Affected Command

**Command ID**: `cmd-84c934b1`
**Action**: `start_session`
**Session**: `admin-firefox-browser-cbd582d7`
**Status**: `failed` (stuck in `pending` in database)

---

## Root Cause Analysis

### Flow of Execution

1. **User creates session via API**
   ```bash
   POST /api/v1/sessions
   {
     "user": "admin",
     "template": "firefox-browser",
     "resources": {"memory": "1Gi", "cpu": "500m"},
     "persistentHome": false
   }
   ```

2. **API creates session in database**
   - State: `pending`
   - agent_id: `k8s-prod-cluster`
   - Creates agent command: `cmd-84c934b1` (action: `start_session`)

3. **API sends WebSocket command to agent**
   - ✅ WebSocket connection working
   - ✅ Command delivered to agent
   - ❌ **Template manifest NOT included in payload**

4. **Agent receives command and processes**
   - Parses command payload
   - Looks for `templateManifest` field
   - **Field is missing** - triggers fallback to K8s API

5. **Agent attempts to fetch Template CRD**
   ```go
   // Agent code tries to fetch template from Kubernetes
   template, err := agent.GetTemplate(ctx, "firefox-browser")
   ```

6. **Kubernetes RBAC denies the request**
   - Service account: `streamspace:streamspace-agent`
   - Resource: `templates.stream.space/firefox-browser`
   - Permission required: `get`
   - **Permission NOT granted** → 403 Forbidden

7. **Session creation fails**
   - Command status: `failed`
   - Session state: stuck in `pending`
   - No pod created, no service created

---

## Impact Assessment

### Severity: P0 (Critical)

**Justification**:
- ❌ **ALL session provisioning blocked**
- ❌ **E2E VNC streaming validation blocked**
- ❌ **Integration testing cannot proceed**
- ❌ **Core product functionality broken**

### Affected Features

1. **Session Creation** (POST /api/v1/sessions) - 🔴 BROKEN
2. **Session Provisioning** - 🔴 BROKEN
3. **VNC Streaming** - 🔴 BLOCKED (no sessions can start)
4. **Multi-User Sessions** - 🔴 BLOCKED
5. **Template-Based Deployments** - 🔴 BROKEN

### Affected Users

- **All users**: Cannot create any sessions
- **Developers**: Cannot test session features
- **QA/Validation**: Integration testing blocked

---

## Contributing Factors

### Issue 1: Missing Template Manifest in API Command Payload

**Evidence**:
```
Warning: No templateManifest in payload, falling back to K8s fetch
```

**Analysis**:
- API should include full template manifest when sending `start_session` command
- Agent shouldn't need to fetch Template CRD from Kubernetes
- This would bypass the RBAC issue entirely

**Related Code** (likely in API):
- `api/internal/handlers/sessions.go` or similar
- WebSocket command construction for agent

### Issue 2: Agent Service Account RBAC Missing

**Current State**:
- Service account: `streamspace-agent` (namespace: `streamspace`)
- Permissions: Unknown (likely minimal)
- Missing permission: `get templates.stream.space`

**Required RBAC**:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: streamspace-agent-role
  namespace: streamspace
rules:
  - apiGroups: ["stream.space"]
    resources: ["templates"]
    verbs: ["get", "list", "watch"]
```

---

## Recommended Fixes

### Primary Fix (Preferred): Include Template Manifest in Command Payload

**Rationale**:
- Eliminates agent dependency on Kubernetes API for templates
- Reduces RBAC complexity
- Improves performance (no K8s API call needed)
- Matches design intent (agent receives all needed data via WebSocket)

**Implementation**:

**API Side** (`api/internal/handlers/sessions.go` or similar):
```go
// When creating start_session command
templateManifest, err := db.GetTemplate(ctx, templateName)
if err != nil {
    return fmt.Errorf("failed to get template: %w", err)
}

payload := map[string]interface{}{
    "sessionId": session.ID,
    "user": session.UserID,
    "template": templateName,
    "templateManifest": templateManifest, // ← ADD THIS
    "namespace": session.Namespace,
    "resources": session.Resources,
    "persistentHome": session.PersistentHome,
}
```

**Benefits**:
- ✅ Fixes issue immediately
- ✅ Eliminates RBAC dependency
- ✅ Improves reliability
- ✅ Reduces K8s API load

---

### Secondary Fix (Fallback): Add RBAC Permissions to Agent

**Rationale**:
- Provides fallback mechanism
- Allows agent to fetch templates if not in payload
- Defense in depth

**Implementation**:

**Kubernetes RBAC** (`manifests/rbac/agent-role.yaml`):
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: streamspace-agent
  namespace: streamspace
rules:
  # Existing permissions...

  # Add template CRD permissions
  - apiGroups: ["stream.space"]
    resources: ["templates"]
    verbs: ["get", "list", "watch"]

  # Also need sessions CRD permissions (if not already granted)
  - apiGroups: ["stream.space"]
    resources: ["sessions"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  # Also need to manage deployments/services for session pods
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  - apiGroups: [""]
    resources: ["services", "pods"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: streamspace-agent
  namespace: streamspace
subjects:
  - kind: ServiceAccount
    name: streamspace-agent
    namespace: streamspace
roleRef:
  kind: Role
  name: streamspace-agent
  apiGroup: rbac.authorization.k8s.io
```

**Benefits**:
- ✅ Provides fallback if template not in payload
- ✅ Enables agent to manage all session resources
- ✅ Aligns with agent's operational needs

---

### Recommended Approach: **BOTH FIXES**

**Rationale**:
1. **Primary fix** (template in payload) eliminates the immediate problem
2. **Secondary fix** (RBAC) provides safety net and enables other operations
3. Combined approach is most robust

**Priority**:
1. **Immediate**: Add RBAC permissions (quickest deployment fix)
2. **Medium-term**: Update API to include template manifest in payload
3. **Long-term**: Remove K8s template fetch from agent (no longer needed)

---

## Validation Plan

Once fixes are deployed, verify:

### Test 1: Session Creation with RBAC Fix

```bash
# Create session
curl -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {"memory": "1Gi", "cpu": "500m"},
    "persistentHome": false
  }'

# Expected: Session created, state transitions to "starting" then "running"
# Verify: Pod created, service created, agent logs show success
```

### Test 2: Agent Logs - No RBAC Errors

```bash
kubectl logs -n streamspace -l app=streamspace-k8s-agent | grep -E "(forbidden|RBAC|permission)"

# Expected: No "forbidden" or permission errors
```

### Test 3: Session Reaches "running" State

```bash
# Monitor session state
kubectl get sessions -n streamspace -w

# Expected: Session transitions pending → starting → running within 30s
```

### Test 4: Pod and Service Created

```bash
kubectl get pods -n streamspace | grep firefox-browser
kubectl get svc -n streamspace | grep firefox-browser

# Expected: Pod running (1/1 Ready), Service created
```

### Test 5: VNC Accessibility (if template manifest in payload)

```bash
# Port-forward to VNC
kubectl port-forward -n streamspace svc/admin-firefox-browser-... 3000:3000

# Access VNC
# Expected: VNC accessible at http://localhost:3000
```

---

## Technical Context

### Template CRD Structure

**API Group**: `stream.space`
**Resource**: `templates`
**Namespace**: `streamspace` (or cluster-wide if ClusterRole)

**Example Template CRD**:
```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
  namespace: streamspace
spec:
  displayName: "Firefox Browser"
  description: "Mozilla Firefox web browser"
  category: "browsers"
  appType: "desktop"
  container:
    image: "jlesage/firefox:latest"
    ports:
      - name: vnc
        containerPort: 5900
        protocol: TCP
  resources:
    requests:
      memory: "512Mi"
      cpu: "250m"
    limits:
      memory: "2Gi"
      cpu: "1000m"
```

### Current Agent Code Behavior

**Pseudocode** (agent logic):
```go
func (h *StartSessionHandler) Handle(cmd Command) error {
    // Parse command payload
    var payload struct {
        SessionID       string
        User            string
        Template        string
        TemplateManifest *TemplateSpec  // ← CURRENTLY NIL
        Namespace       string
        Resources       ResourceSpec
        PersistentHome  bool
    }

    json.Unmarshal(cmd.Payload, &payload)

    var templateSpec *TemplateSpec
    if payload.TemplateManifest != nil {
        // Use provided manifest (preferred path)
        templateSpec = payload.TemplateManifest
    } else {
        // Fallback: Fetch from Kubernetes (fails due to RBAC)
        templateSpec, err = h.getTemplateFromK8s(payload.Template)
        if err != nil {
            return fmt.Errorf("failed to get template: %w", err)
        }
    }

    // Create deployment, service, etc. using templateSpec
    return h.createSession(payload, templateSpec)
}
```

---

## Dependencies

**Blocks**:
- E2E VNC streaming validation
- Integration testing continuation
- Session provisioning for all users
- Multi-session concurrency testing

**Depends On**:
- ✅ P1-DATABASE-001 fix (validated)
- ✅ P1-SCHEMA-001 fix (validated)
- ✅ P1-SCHEMA-002 fix (validated)
- ✅ Agent WebSocket connection (working)

**Related Issues**:
- P0-AGENT-001 (WebSocket concurrent write) - ✅ FIXED
- P1-DATABASE-001 (TEXT[] arrays) - ✅ FIXED
- P1-SCHEMA-001 (cluster_id) - ✅ FIXED
- P1-SCHEMA-002 (tags column) - ✅ FIXED

---

## Additional Notes

### Why This Wasn't Caught Earlier

1. **P0/P1 fixes blocked testing**: Previous bugs prevented reaching session provisioning stage
2. **Agent was restarting**: During earlier tests, agent may have had stale permissions or different behavior
3. **Integration testing just started**: This is the first comprehensive E2E VNC streaming test

### Severity Assessment

**Why P0 (Critical)**:
- Blocks ALL session creation (not just some edge cases)
- No workaround available without code/config changes
- Impacts core product functionality
- Discovered during critical integration testing phase

**Why Not P1**:
- P1 issues allow partial functionality with workarounds
- This completely blocks session provisioning
- Cannot proceed with any E2E testing

---

## Evidence

### Test Execution

**Script**: `/tmp/test_e2e_vnc_streaming.sh`
**Session**: `admin-firefox-browser-cbd582d7`
**Command**: `cmd-84c934b1`
**Template**: `firefox-browser`

### Database State

```sql
SELECT command_id, agent_id, action, status FROM agent_commands
WHERE command_id = 'cmd-84c934b1';
```

**Result**:
```
 command_id   |     agent_id     |    action     | status
--------------+------------------+---------------+---------
 cmd-84c934b1 | k8s-prod-cluster | start_session | pending
```

**Analysis**: Command stuck in `pending` (should be `completed` or explicitly `failed`)

### Agent Logs Timeline

```
04:07:36 - Command received
04:07:36 - StartSessionHandler started
04:07:36 - Warning: No templateManifest in payload
04:07:36 - Attempted K8s template fetch
04:07:36 - RBAC 403 Forbidden error
04:07:36 - Command marked as failed
```

---

## Conclusion

**Summary**: K8s agent cannot create sessions due to missing RBAC permissions to read Template CRDs. The root cause is twofold: API doesn't include template manifest in command payload, and agent lacks fallback RBAC permissions.

**Immediate Action Required**:
1. **Quick fix**: Add RBAC permissions to agent service account
2. **Proper fix**: Update API to include template manifest in WebSocket command payload

**Severity**: P0 - Blocks all session provisioning and E2E testing

**Recommendation**: Deploy RBAC fix immediately, then implement template-in-payload fix for long-term reliability.

---

**Generated**: 2025-11-22 04:15:00 UTC
**Validator**: Claude (v2-validator branch)
**Next Step**: Builder to implement RBAC fix and/or template manifest inclusion
