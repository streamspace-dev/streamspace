# Bug Report: P1-MULTI-POD-001 - AgentHub Not Shared Across API Replicas

**Bug ID**: P1-MULTI-POD-001
**Severity**: P1 - HIGH (Blocks horizontal scaling of API)
**Component**: Control Plane AgentHub
**Discovered During**: P1-COMMAND-SCAN-001 fix validation (Test 3.2 re-run)
**Status**: 🔴 ACTIVE
**Reporter**: Claude (v2-validator)
**Date**: 2025-11-22 07:11:00 UTC

---

## Executive Summary

When the StreamSpace API is deployed with multiple replicas (pods), agent WebSocket connections are stored in-memory within each pod's AgentHub. This causes session creation requests to fail with "No agents available" errors when the request is load-balanced to a different API pod than the one the agent is connected to.

**Impact**: **CRITICAL** - Multi-replica API deployments are completely broken for agent connectivity. Horizontal scaling of the API is not possible.

---

## Symptoms

### User-Facing Error

**Error Message**:
```json
{
  "error": "No agents available",
  "message": "No online agents are currently available: no agents match selection criteria"
}
```

**HTTP Status**: 503 Service Unavailable

---

### API Logs

```
2025/11/22 07:11:48 [AgentSelector] Found 1 online agents
2025/11/22 07:11:48 [AgentSelector] Skipping agent k8s-prod-cluster (not connected via WebSocket)
2025/11/22 07:11:48 No agents available for session admin-firefox-browser-3befe1ad: no agents match selection criteria
```

**Observation**:
- AgentSelector finds the agent in the database (status: "online")
- AgentSelector skips the agent because it's "not connected via WebSocket"
- Session creation fails with "No agents available"

---

## Root Cause Analysis

### Architecture Issue

**Component**: AgentHub (WebSocket connection manager)
**Location**: `api/internal/websocket/hub.go` (or similar)

**Problem**: AgentHub maintains WebSocket connections in-memory within each API pod

**Current Architecture** (Broken with multiple replicas):

```
┌─────────────────────────────────────────┐
│  Kubernetes Service (Load Balancer)     │
│     streamspace-api:8000                │
└────────┬─────────────────┬──────────────┘
         │                 │
         ▼                 ▼
   ┌─────────┐       ┌─────────┐
   │ API Pod 1│       │ API Pod 2│
   │          │       │          │
   │ AgentHub │       │ AgentHub │
   │  (empty) │       │  (empty) │
   └─────────┘       └─────────┘
         │
         │ WebSocket
         │
    ┌────▼──────┐
    │K8s Agent  │
    └───────────┘

Flow:
1. Agent connects to Pod 2 via WebSocket → AgentHub in Pod 2 registers agent
2. User sends session creation request → Load balancer routes to Pod 1
3. Pod 1's AgentHub has no agent connections → "No agents available"
```

**Expected Architecture** (Needs implementation):

```
┌─────────────────────────────────────────┐
│  Kubernetes Service (Load Balancer)     │
│     streamspace-api:8000                │
└────────┬─────────────────┬──────────────┘
         │                 │
         ▼                 ▼
   ┌─────────┐       ┌─────────┐
   │ API Pod 1│       │ API Pod 2│
   │          │       │          │
   │ AgentHub │       │ AgentHub │
   │          │       │          │
   └────┬────┘       └────┬─────┘
        │                 │
        └────────┬────────┘
                 │
                 ▼
          ┌──────────┐
          │  Redis   │ ← Shared state for agent connections
          │          │
          └──────────┘
```

---

### Database State vs In-Memory State

**Database** (agents table):
```sql
agent_id: k8s-prod-cluster
status: online
last_heartbeat: 2025-11-22 07:11:49
```

**In-Memory State** (AgentHub in Pod 1):
```
Connections: {} (empty)
```

**In-Memory State** (AgentHub in Pod 2):
```
Connections: {
  "k8s-prod-cluster": <WebSocket connection>
}
```

**AgentSelector Logic**:
1. Query database for online agents → Finds k8s-prod-cluster ✅
2. Check if agent connected via WebSocket in THIS pod's AgentHub → Not found ❌
3. Skip agent → "No agents available"

---

## Evidence

### Test Scenario

**Setup**:
- API deployment scaled to 2 replicas
- K8s agent running and connected

**Steps**:
1. Deploy API with 2 replicas:
   ```bash
   kubectl get pods -n streamspace -l app.kubernetes.io/component=api
   # NAME                              READY   STATUS    RESTARTS   AGE
   # streamspace-api-86d989cc5-7cwx2   1/1     Running   0          3m26s
   # streamspace-api-86d989cc5-c6hq7   1/1     Running   0          3m44s
   ```

2. Agent connects to one pod:
   ```
   07:10:19 [AgentHub] Registered agent: k8s-prod-cluster (platform: kubernetes), total connections: 1
   ```

3. Create session (request routed to different pod):
   ```bash
   curl -X POST http://localhost:8000/api/v1/sessions ...
   ```

**Result**:
```json
{
  "error": "No agents available",
  "message": "No online agents are currently available: no agents match selection criteria"
}
```

---

### API Logs Evidence

**Pod 2** (agent connected to this pod):
```
07:10:19 [AgentHub] Registered agent: k8s-prod-cluster (platform: kubernetes), total connections: 1
07:10:49 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
```

**Pod 1** (session creation request routed here):
```
07:11:48 [AgentSelector] Found 1 online agents
07:11:48 [AgentSelector] Skipping agent k8s-prod-cluster (not connected via WebSocket)
07:11:48 No agents available for session admin-firefox-browser-3befe1ad: no agents match selection criteria
```

---

### Database Verification

**Query**:
```sql
SELECT agent_id, status, last_heartbeat FROM agents WHERE agent_id = 'k8s-prod-cluster';
```

**Result**:
```
agent_id     | status |       last_heartbeat
-------------|--------|---------------------------
k8s-prod-cluster | online | 2025-11-22 07:11:49.131286
```

**Analysis**: Agent is "online" in database, but not accessible from all API pods.

---

## Impact Assessment

### Severity: P1 - HIGH

**Why P1**:
- **Blocks horizontal scaling** - Cannot run multiple API replicas
- **Affects production readiness** - Single API pod is single point of failure
- **Affects high availability** - Cannot achieve HA deployment
- **Affects load capacity** - Single pod limits throughput

**Why Not P0**:
- Has workaround (scale to 1 replica)
- System functional with single replica
- Does not affect existing single-replica deployments

---

### Affected Scenarios

All scenarios requiring multiple API pods:

1. **High Availability Deployments**:
   - ❌ Cannot run 2+ API pods for redundancy
   - ❌ Single pod failure = complete API outage

2. **Load Balancing**:
   - ❌ Cannot distribute load across multiple API pods
   - ❌ Single pod becomes bottleneck

3. **Rolling Updates**:
   - ⚠️ Brief downtime during pod replacement
   - ⚠️ Agent disconnections during rollout

4. **Auto-Scaling**:
   - ❌ Cannot auto-scale API based on load
   - ❌ HPA (Horizontal Pod Autoscaler) not usable

---

### Production Readiness Impact

| Component | Single Replica | Multi-Replica | Status |
|-----------|----------------|---------------|--------|
| **Session Creation** | ✅ Working | ❌ Broken | Not Production Ready |
| **Agent Connectivity** | ✅ Working | ❌ Broken | Not Production Ready |
| **High Availability** | ❌ Not Available | ❌ Broken | Not Production Ready |
| **Load Distribution** | ❌ Not Available | ❌ Broken | Not Production Ready |
| **Horizontal Scaling** | ❌ Not Available | ❌ Broken | Not Production Ready |

**Overall**: ⚠️ **LIMITED PRODUCTION READINESS** - Works only with single API replica

---

## Recommended Fix

### Solution 1: Shared State with Redis (Recommended)

**Approach**: Use Redis to store agent connection state instead of in-memory maps

**Benefits**:
- ✅ Supports multiple API replicas
- ✅ Fast lookups (< 1ms)
- ✅ Standard pattern for distributed systems
- ✅ Minimal code changes

**Changes Required**:

**1. Add Redis to deployment**:
```yaml
# manifests/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: streamspace-redis
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
```

**2. Update AgentHub to use Redis**:

```go
// api/internal/websocket/hub.go

type AgentHub struct {
    redisClient *redis.Client
    // Remove: connections map[string]*AgentConnection
}

func (h *AgentHub) RegisterAgent(agentID string, conn *websocket.Conn) {
    // Store connection metadata in Redis
    h.redisClient.Set(ctx, fmt.Sprintf("agent:%s:connected", agentID), "true", 5*time.Minute)
    h.redisClient.Set(ctx, fmt.Sprintf("agent:%s:pod", agentID), os.Getenv("POD_NAME"), 5*time.Minute)

    // Store actual WebSocket connection locally (can't serialize)
    h.localConnections[agentID] = conn
}

func (h *AgentHub) IsAgentConnected(agentID string) bool {
    // Check Redis for agent connection state across all pods
    connected, err := h.redisClient.Get(ctx, fmt.Sprintf("agent:%s:connected", agentID)).Result()
    return err == nil && connected == "true"
}

func (h *AgentHub) SendCommandToAgent(agentID string, command *AgentCommand) error {
    // Check if agent connected to THIS pod
    if conn, ok := h.localConnections[agentID]; ok {
        return conn.WriteJSON(command)
    }

    // Agent connected to different pod - use Redis pub/sub
    podName, err := h.redisClient.Get(ctx, fmt.Sprintf("agent:%s:pod", agentID)).Result()
    if err != nil {
        return fmt.Errorf("agent not connected")
    }

    // Publish command to pod-specific channel
    commandJSON, _ := json.Marshal(command)
    h.redisClient.Publish(ctx, fmt.Sprintf("pod:%s:commands", podName), commandJSON)
    return nil
}
```

**3. Add Redis pub/sub listener in each pod**:

```go
func (h *AgentHub) ListenForCommands() {
    pubsub := h.redisClient.Subscribe(ctx, fmt.Sprintf("pod:%s:commands", os.Getenv("POD_NAME")))

    for msg := range pubsub.Channel() {
        var command AgentCommand
        json.Unmarshal([]byte(msg.Payload), &command)

        // Send to local WebSocket connection
        if conn, ok := h.localConnections[command.AgentID]; ok {
            conn.WriteJSON(command)
        }
    }
}
```

**Estimated Implementation Time**: 2-4 hours

---

### Solution 2: WebSocket Service Affinity (Alternative)

**Approach**: Use Kubernetes service session affinity to route all requests from an agent to the same pod

**Benefits**:
- ✅ No code changes required
- ✅ Simple Kubernetes configuration
- ✅ Works immediately

**Drawbacks**:
- ❌ Load imbalance (agents sticky to pods)
- ❌ Agent reconnects if pod restarts
- ❌ Uneven distribution of agents

**Changes Required**:

```yaml
# manifests/api-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: streamspace-api
spec:
  type: ClusterIP
  sessionAffinity: ClientIP
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 10800  # 3 hours
  ports:
  - port: 8000
    targetPort: 8000
  selector:
    app: streamspace-api
```

**Limitation**: Does not solve the fundamental problem - AgentHub still not shared

**Estimated Implementation Time**: 5 minutes

**Recommendation**: Use as temporary workaround only

---

### Solution 3: Single API Pod (Current Workaround)

**Approach**: Scale API deployment to 1 replica

**Command**:
```bash
kubectl scale deployment/streamspace-api -n streamspace --replicas=1
```

**Benefits**:
- ✅ Works immediately
- ✅ No code changes
- ✅ No additional infrastructure

**Drawbacks**:
- ❌ No high availability
- ❌ Single point of failure
- ❌ Limited throughput
- ❌ Not production ready

**Recommendation**: Testing/development only

---

## Reproduction Steps

### Prerequisites
- StreamSpace v2.0-beta deployed
- K8s agent connected
- API deployment with 2+ replicas

### Steps

1. Deploy API with 2 replicas:
   ```bash
   kubectl scale deployment/streamspace-api -n streamspace --replicas=2
   kubectl rollout status deployment/streamspace-api -n streamspace
   ```

2. Verify 2 API pods running:
   ```bash
   kubectl get pods -n streamspace -l app.kubernetes.io/component=api
   # Should show 2 pods
   ```

3. Check agent connection logs:
   ```bash
   kubectl logs -n streamspace -l app.kubernetes.io/component=api | grep "Registered agent"
   # Agent will be registered in ONE pod only
   ```

4. Attempt to create session:
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

   curl -X POST http://localhost:8000/api/v1/sessions \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "user": "admin",
       "template": "firefox-browser",
       "resources": {"memory": "512Mi", "cpu": "250m"},
       "persistentHome": false
     }'
   ```

5. Observe error (50% chance based on load balancing):
   ```json
   {
     "error": "No agents available",
     "message": "No online agents are currently available: no agents match selection criteria"
   }
   ```

6. Check API logs:
   ```bash
   kubectl logs -n streamspace -l app.kubernetes.io/component=api | grep -i "AgentSelector\|no agents"
   ```

**Expected Result** (with bug): "No agents available" on some requests

**Expected Result** (after fix): Session created successfully on all requests

---

## Validation Testing

### After Fix Applied

**Test 1: Verify Multi-Pod Agent Connectivity**

```bash
# Deploy API with 2 replicas
kubectl scale deployment/streamspace-api -n streamspace --replicas=2
kubectl rollout status deployment/streamspace-api -n streamspace

# Wait for agent to connect
sleep 10

# Create 10 sessions (should all succeed with load balancing)
for i in {1..10}; do
  echo "Creating session $i..."
  curl -s -X POST http://localhost:8000/api/v1/sessions \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "user": "admin",
      "template": "firefox-browser",
      "resources": {"memory": "512Mi", "cpu": "250m"},
      "persistentHome": false
    }' | jq -r '.name'
done
```

**Expected**: All 10 sessions created successfully

---

**Test 2: Verify Agent Connection Visible Across Pods**

```bash
# Check agent status from each pod
for pod in $(kubectl get pods -n streamspace -l app.kubernetes.io/component=api -o name); do
  echo "Pod: $pod"
  kubectl exec -n streamspace $pod -- curl -s http://localhost:8000/api/v1/agents
done
```

**Expected**: All pods return same agent list

---

**Test 3: Verify Commands Routed to Correct Pod**

```bash
# Create session via Pod 1
# Send termination command via Pod 2
# Verify command processed successfully
```

**Expected**: Command routed correctly regardless of which pod receives the request

---

## Related Issues

### Discovered During
- P1-COMMAND-SCAN-001 fix validation (Test 3.2 re-run)

### Dependencies
- This bug BLOCKS horizontal scaling of API
- This bug BLOCKS high availability deployments
- This bug BLOCKS production readiness assessment

### Related Bugs
- P1-COMMAND-SCAN-001 (AgentCommand NULL scan) - RESOLVED
- P1-SCHEMA-002 (missing updated_at column) - ACTIVE
- P1-AGENT-STATUS-001 (Agent status sync) - RESOLVED

---

## Workarounds

### Current Workaround: Scale to 1 Replica

**Command**:
```bash
kubectl scale deployment/streamspace-api -n streamspace --replicas=1
```

**Effectiveness**: ✅ **WORKS** - All agent connectivity issues resolved

**Limitations**:
- No high availability
- Single point of failure
- Limited throughput
- Not suitable for production

---

## Priority Justification

### Why P1 (Not P0)

- **P0** bugs prevent deployment or cause complete system failure
- **P1** bugs block critical functionality but system remains partially functional

**This is P1 because**:
- ❌ Blocks horizontal scaling (critical for production)
- ❌ Blocks high availability
- ✅ Has workaround (single replica)
- ✅ System functional with workaround
- ✅ Does not affect single-replica deployments

**Could be elevated to P0 if**:
- Single replica becomes insufficient for production load
- No workaround existed
- Caused data loss or corruption

---

## Next Steps

1. **Builder**: Implement Solution 1 (Shared State with Redis)
   - Add Redis deployment to manifests
   - Update AgentHub to use Redis for connection state
   - Add Redis pub/sub for cross-pod command routing
   - Update Helm chart to include Redis dependency

2. **Builder**: Commit fix to `claude/v2-builder` branch

3. **Validator**: Merge fix and redeploy with 2 replicas

4. **Validator**: Run validation tests (Test 1, 2, 3 above)

5. **Validator**: Document validation results

6. **Validator**: Continue integration testing

---

## Additional Context

### Impact on Production

**Deployment Scenarios** (ALL affected):
- High availability deployments (2+ API pods)
- Auto-scaling deployments (HPA-based scaling)
- Load-balanced deployments (multiple regions)
- Rolling update deployments (brief multi-pod state)

**Expected Behavior**: Agent connections accessible across all API pods

**Actual Behavior**: Agent connections isolated to one pod

**Risk**: **HIGH** - Cannot achieve production-grade high availability

---

## Conclusion

**Bug Summary**: AgentHub maintains WebSocket connections in-memory per pod, preventing multi-replica deployments

**Impact**: Blocks horizontal scaling and high availability

**Fix Complexity**: Medium - Requires Redis integration and pub/sub implementation

**Testing**: Multi-pod validation tests required

**Priority**: P1 - HIGH (blocks production readiness)

**Recommended Solution**: Shared state with Redis (Solution 1)

---

**Generated**: 2025-11-22 07:16:00 UTC
**Validator**: Claude (v2-validator)
**Branch**: claude/v2-validator
**Status**: 🔴 ACTIVE - Awaiting Builder Fix
**Priority**: P1 - HIGH
**Blocks**: Horizontal Scaling, High Availability, Production Readiness
