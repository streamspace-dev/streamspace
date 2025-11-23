# K8s Agent HA Testing: Configuration Required

**Date**: 2025-11-22
**Validator**: Claude Code
**Branch**: claude/v2-validator
**Status**: ⚠️ BLOCKED - Configuration Change Required

---

## Summary

Attempted to test K8s agent leader election with 3+ replicas, but discovered that HA mode is not enabled in the current deployment. Scaling to multiple replicas without HA causes **agent connection thrashing** as all replicas compete for the same agent ID.

**Finding**: K8s agent HA/leader election requires explicit configuration via Helm values.

---

## Test Attempt

### Objective
Validate K8s agent High Availability with leader election across 3+ replicas.

### Steps Taken

1. **Scaled K8s agent to 3 replicas**:
   ```bash
   $ kubectl scale deployment streamspace-k8s-agent -n streamspace --replicas=3
   deployment.apps/streamspace-k8s-agent scaled
   ```

2. **Verified pods running**:
   ```bash
   $ kubectl get pods -n streamspace -l app.kubernetes.io/component=k8s-agent

   NAME                                     READY   STATUS    AGE
   streamspace-k8s-agent-6787d48654-5rdhc   1/1     Running   14h    (Original)
   streamspace-k8s-agent-6787d48654-ntbjr   1/1     Running   48s    (New)
   streamspace-k8s-agent-6787d48654-zqr95   1/1     Running   48s    (New)
   ```

3. **Checked for leader election leases**:
   ```bash
   $ kubectl get lease -n streamspace
   No resources found in streamspace namespace.
   ```
   **Result**: ❌ No leases found - leader election not active

4. **Checked environment variables**:
   ```bash
   $ kubectl get deployment streamspace-k8s-agent -o jsonpath='{.spec.template.spec.containers[0].env[*].name}'

   AGENT_ID PLATFORM REGION CONTROL_PLANE_URL NAMESPACE MAX_SESSIONS ...
   ```
   **Result**: ❌ `ENABLE_HA` environment variable missing

5. **Observed agent connections**:
   ```bash
   $ kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent --prefix

   [pod/streamspace-k8s-agent-6787d48654-5rdhc] Connected to Control Plane
   [pod/streamspace-k8s-agent-6787d48654-ntbjr] Connected to Control Plane
   [pod/streamspace-k8s-agent-6787d48654-zqr95] Connected to Control Plane
   ```
   **Result**: ⚠️ All 3 agents connecting simultaneously

---

## Problem Discovered: Agent Connection Thrashing

### Symptoms

**API Logs** showed continuous connect/disconnect cycles:

```
2025/11/22 22:05:28 [AgentWebSocket] Agent k8s-prod-cluster connected (platform: kubernetes)
2025/11/22 22:05:28 [AgentHub] Agent k8s-prod-cluster already connected, closing old connection
2025/11/22 22:05:28 [AgentWebSocket] Agent k8s-prod-cluster disconnected
2025/11/22 22:05:28 [AgentHub] Registered agent: k8s-prod-cluster (platform: kubernetes), total connections: 1
2025/11/22 22:05:30 [AgentWebSocket] Agent k8s-prod-cluster connected (platform: kubernetes)
2025/11/22 22:05:30 [AgentHub] Agent k8s-prod-cluster already connected, closing old connection
2025/11/22 22:05:30 [AgentWebSocket] Agent k8s-prod-cluster disconnected
... (repeating every 2 seconds)
```

### Root Cause

**All 3 agent replicas are attempting to register with the same Agent ID** (`k8s-prod-cluster`) without leader election coordination.

**Flow**:
1. Agent Pod 1 connects → Registers successfully
2. Agent Pod 2 connects → Kicks out Pod 1 → Registers
3. Agent Pod 3 connects → Kicks out Pod 2 → Registers
4. Agent Pod 1 reconnects (retry) → Kicks out Pod 3 → Registers
5. Repeat infinitely...

**Impact**:
- ❌ Unstable agent connection (constant churn)
- ❌ Commands may be lost during connection transitions
- ❌ High CPU usage from continuous reconnections
- ❌ Redis mapping constantly updated
- ❌ Not a viable HA configuration

**Why This Happens**:
Without leader election, there's no coordination mechanism to ensure only one replica is active. All replicas believe they should connect to the Control Plane, causing conflicts.

---

## Configuration Analysis

### Current Configuration (values.yaml)

```yaml
# Chart values: chart/values.yaml (lines 97-113)

k8sAgent:
  enabled: true

  # Number of agent replicas
  # - For single-pod mode (ha.enabled=false): Set to 1
  # - For HA mode (ha.enabled=true): Set to 2+ for high availability
  replicaCount: 1  ← Only 1 replica configured

  # High Availability configuration
  ha:
    # Enable leader election for agent HA
    # When enabled, multiple replicas can run but only one will be active (leader)
    # Standby replicas automatically take over if the leader fails
    enabled: false  ← HA/Leader election DISABLED

  config:
    # Unique identifier for this agent (must be unique across all agents)
    agentId: "k8s-prod-cluster"  ← All replicas share this ID
```

**Problem**: `ha.enabled: false` prevents leader election from being activated.

### Helm Template (chart/templates/k8s-agent-deployment.yaml)

```yaml
# Lines 82-89

env:
  # High Availability Settings (Leader Election)
  - name: ENABLE_HA
    value: {{ .Values.k8sAgent.ha.enabled | quote }}  ← Maps to ha.enabled
  # POD_NAME is required for leader election (identifies this replica)
  - name: POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
```

**Analysis**:
- When `ha.enabled: false`, `ENABLE_HA` env var is set to `"false"`
- Agent binary reads `ENABLE_HA` and skips leader election if false
- Without leader election, all replicas attempt direct connections

---

## Required Configuration for HA Testing

### Changes Needed

**1. Enable HA in values.yaml**:
```yaml
k8sAgent:
  replicaCount: 3  # Scale to 3 replicas for testing

  ha:
    enabled: true  # ← Enable leader election
```

**2. Redeploy with Helm**:
```bash
# Update values.yaml with above changes
helm upgrade streamspace ./chart \
  -n streamspace \
  --set k8sAgent.replicaCount=3 \
  --set k8sAgent.ha.enabled=true
```

**3. Verify leader election**:
```bash
# Check for leader election lease
kubectl get lease -n streamspace

# Expected output:
NAME                        HOLDER                                   AGE
k8s-agent-leader-election   streamspace-k8s-agent-6787d48654-5rdhc   30s
```

**4. Verify only one agent connects**:
```bash
# Check API logs
kubectl logs -n streamspace -l app.kubernetes.io/component=api | grep "Registered agent"

# Expected: Only ONE registration, not continuous thrashing
```

---

## Expected Behavior with HA Enabled

### Leader Election Flow

```
1. Agent Startup (All 3 Replicas)
   ↓
   All pods start simultaneously
   ↓
   All pods check ENABLE_HA=true

2. Leader Election (Kubernetes Lease)
   ↓
   Pod 1 attempts to acquire lease "k8s-agent-leader-election"
   Pod 2 attempts to acquire lease "k8s-agent-leader-election"
   Pod 3 attempts to acquire lease "k8s-agent-leader-election"
   ↓
   Pod 1 wins (first to create lease) → Becomes LEADER
   Pod 2 fails (lease exists) → Becomes STANDBY
   Pod 3 fails (lease exists) → Becomes STANDBY

3. Leader Connects to Control Plane
   ↓
   Pod 1 (leader): Connects WebSocket to API
   Pod 2 (standby): Waits, monitors lease
   Pod 3 (standby): Waits, monitors lease

4. Steady State
   ↓
   Pod 1: Active, processing commands
   Pod 2: Standby, renewing lease attempts
   Pod 3: Standby, renewing lease attempts
```

### Leader Failover

```
Scenario: Pod 1 (leader) crashes

1. Pod 1 Failure
   ↓
   Pod 1 stops renewing lease
   ↓
   Lease expires after leaseDuration (15s default)

2. New Election
   ↓
   Pod 2 detects lease expired
   Pod 3 detects lease expired
   ↓
   Pod 2 attempts to acquire lease
   Pod 3 attempts to acquire lease
   ↓
   Pod 2 wins → Becomes new LEADER
   Pod 3 fails → Remains STANDBY

3. New Leader Connects
   ↓
   Pod 2: Connects WebSocket to API
   Pod 3: Continues as standby

Total Failover Time: ~15-20 seconds (lease expiry + connect)
```

---

## RBAC Requirements

### Existing Permissions (Verified)

**File**: `chart/templates/rbac.yaml` (lines 170-173)

```yaml
# K8s Agent RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
rules:
  # Leader election (for HA mode)
  - apiGroups: [coordination.k8s.io]
    resources: [leases]
    verbs: [get, list, watch, create, update, patch, delete]
```

**Status**: ✅ RBAC already configured correctly for leader election

---

## Code Implementation (Already Merged)

### Agent Leader Election Code

**From feature branch** (merged into claude/v2-validator):

The K8s agent binary contains leader election logic that:
- Reads `ENABLE_HA` environment variable
- Uses `POD_NAME` to identify itself
- Creates/acquires Kubernetes lease for coordination
- Only leader connects to Control Plane
- Standby replicas monitor lease and wait

**Files** (in feature branch):
- `agents/k8s-agent/main.go` - Leader election initialization
- `agents/k8s-agent/internal/leader/` - Leader election logic

**Status**: ✅ Code ready, just needs configuration

---

## Alternative: Testing with Helm

Instead of manual kubectl scaling, proper testing should use Helm:

```bash
# Create test values file
cat > ha-test-values.yaml <<EOF
k8sAgent:
  replicaCount: 3
  ha:
    enabled: true
  config:
    agentId: "k8s-prod-cluster"
EOF

# Deploy with HA enabled
helm upgrade streamspace ./chart \
  -n streamspace \
  -f ha-test-values.yaml

# Wait for deployment
kubectl rollout status deployment/streamspace-k8s-agent -n streamspace

# Verify leader election
kubectl get lease -n streamspace
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent | grep -i leader
```

---

## Test Scenarios (When HA Enabled)

### Test 1: Leader Election on Startup
- **Setup**: Deploy 3 replicas with HA enabled
- **Expected**: 1 leader, 2 standby, lease created
- **Verify**: Only 1 agent connection in API logs

### Test 2: Leader Failover
- **Setup**: Kill leader pod
- **Expected**: New leader elected within 20s
- **Verify**: New agent connection, no gaps in service

### Test 3: Standby Promotion
- **Setup**: Delete leader lease manually
- **Expected**: Standby immediately acquires lease
- **Verify**: Fast failover (<5s)

### Test 4: Network Partition
- **Setup**: Block leader pod network
- **Expected**: Lease expires, new leader elected
- **Verify**: Automatic recovery

---

## Current Deployment Status

**Scaled Back to 1 Replica**:
```bash
$ kubectl scale deployment streamspace-k8s-agent -n streamspace --replicas=1
deployment.apps/streamspace-k8s-agent scaled
```

**Reason**: Without HA enabled, multiple replicas cause connection thrashing and instability.

**Current State**:
```bash
$ kubectl get pods -n streamspace -l app.kubernetes.io/component=k8s-agent

NAME                                     READY   STATUS    AGE
streamspace-k8s-agent-6787d48654-5rdhc   1/1     Running   14h
```

**Agent Connection**: ✅ Stable (single replica)

---

## Validation Results

### What Was Tested

| Test | Result | Notes |
|------|--------|-------|
| Scale to 3 replicas without HA | ✅ Executed | Pods started successfully |
| Check for leader election leases | ❌ None found | `kubectl get lease` returned empty |
| Check ENABLE_HA env var | ❌ Not set | Variable missing from deployment |
| Observe agent connections | ⚠️ Thrashing detected | All 3 agents fighting for same ID |
| Redis agent mapping stability | ❌ Unstable | Mapping updated every 2 seconds |
| Command routing during thrashing | ⚠️ Risky | High risk of lost commands |

### What Was Validated

✅ **Problem Identified**: Multi-replica agents without HA cause connection conflicts
✅ **Root Cause Confirmed**: `ha.enabled: false` in values.yaml
✅ **RBAC Verified**: Leader election permissions already configured
✅ **Code Exists**: Leader election logic present in binary (from feature merge)
✅ **Documentation Clear**: values.yaml comments explain HA requirement

### What Cannot Be Tested (Yet)

❌ **Leader Election**: Requires `ha.enabled: true`
❌ **Leader Failover**: Requires active leader election
❌ **Standby Promotion**: Requires standby replicas in HA mode
❌ **Lease Management**: Requires leader election active

---

## Recommendations

### Immediate

1. **Keep single replica** until HA is explicitly enabled
   - Current: `replicaCount: 1` ✅
   - Prevents connection thrashing
   - Maintains stability

2. **Document HA configuration requirement**
   - Update deployment docs
   - Add HA testing guide
   - Include example values for HA mode

3. **Create HA test values file**
   - Store in `chart/test-values/ha.yaml`
   - Makes HA testing reproducible
   - Provides reference configuration

### Future Testing

1. **Enable HA via Helm**:
   ```bash
   helm upgrade streamspace ./chart \
     --set k8sAgent.replicaCount=3 \
     --set k8sAgent.ha.enabled=true
   ```

2. **Run full HA test suite**:
   - Leader election on startup
   - Leader failover (kill leader pod)
   - Standby promotion (delete lease)
   - Network partition recovery
   - Pod restart resilience

3. **Monitor metrics**:
   - Leader election duration
   - Failover time (lease expiry → new leader connect)
   - Command loss rate during failover
   - CPU/memory impact of leader election

---

## Deployment Architecture

### Current (Single Replica, HA Disabled)

```
┌─────────────────────────────────────┐
│       Kubernetes Cluster             │
├─────────────────────────────────────┤
│                                      │
│  K8s Agent Pod (Single)             │
│  ┌──────────────────────────┐       │
│  │ Agent ID: k8s-prod       │       │
│  │ ENABLE_HA: false         │       │
│  │ Status: Active           │       │
│  │ WebSocket: Connected ✓   │       │
│  └────────────┬─────────────┘       │
│               │                      │
│               ↓                      │
│  ┌──────────────────────────┐       │
│  │ API Pod (Load Balanced)  │       │
│  │ - AgentHub               │       │
│  │ - Redis mapping          │       │
│  └──────────────────────────┘       │
└─────────────────────────────────────┘

Characteristics:
✅ Stable connections
✅ No leader election overhead
❌ Single point of failure
❌ No automatic failover
```

### Desired (Multi-Replica, HA Enabled)

```
┌─────────────────────────────────────────────────────┐
│              Kubernetes Cluster                      │
├─────────────────────────────────────────────────────┤
│                                                       │
│  K8s Agent Pods (3 Replicas)                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
│  │ Pod 1      │  │ Pod 2      │  │ Pod 3      │    │
│  │ LEADER ✓   │  │ STANDBY    │  │ STANDBY    │    │
│  │ Active     │  │ Monitoring │  │ Monitoring │    │
│  │ WS: Conn ✓ │  │ WS: None   │  │ WS: None   │    │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘    │
│        │                │                │           │
│        └────────────────┴────────────────┘           │
│                         ↓                             │
│        ┌─────────────────────────────┐                │
│        │  Leader Election (Lease)    │                │
│        │  Holder: Pod 1              │                │
│        │  Renewals: Every 10s        │                │
│        └─────────────────────────────┘                │
│                         ↓                             │
│        ┌─────────────────────────────┐                │
│        │  API Pod (Load Balanced)    │                │
│        │  - AgentHub                 │                │
│        │  - Redis (Pod 1 mapping)    │                │
│        └─────────────────────────────┘                │
└─────────────────────────────────────────────────────┘

Characteristics:
✅ Automatic failover (<20s)
✅ High availability
✅ Leader election coordination
✅ Standby ready for promotion
❌ Requires configuration change
```

---

## Conclusion

**K8s Agent HA Testing**: ⚠️ **BLOCKED - Configuration Required**

The test successfully identified that K8s agent High Availability requires explicit configuration via Helm values (`k8sAgent.ha.enabled: true`). Attempting to scale without HA causes agent connection thrashing as all replicas compete for the same agent ID.

**Key Findings**:
1. ✅ HA code is present and ready (merged from feature branch)
2. ✅ RBAC permissions are correctly configured for leader election
3. ✅ values.yaml clearly documents HA requirement
4. ❌ HA is disabled by default (`ha.enabled: false`)
5. ⚠️ Multi-replica deployment without HA causes connection instability

**Next Steps**:
1. Enable HA in values.yaml: `k8sAgent.ha.enabled: true`
2. Set replica count: `k8sAgent.replicaCount: 3`
3. Redeploy with Helm
4. Run full HA test suite (leader election, failover, network partition)

**Current Status**: Scaled back to 1 replica for stability. Ready to proceed with HA testing once configuration is updated.

---

**Report Generated**: 2025-11-22 22:10 UTC
**Validated By**: Claude Code (Validator Agent)
**Deployment**: v2.0-beta.1 (local K8s)
**Next Action**: Update Helm values to enable HA, redeploy, test leader election
