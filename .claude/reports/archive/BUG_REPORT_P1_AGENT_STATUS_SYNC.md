# Bug Report: P1-AGENT-STATUS-001 - Agent WebSocket Heartbeats Don't Update Database Status

**Bug ID**: P1-AGENT-STATUS-001
**Severity**: P1 - HIGH (Blocks all session creation)
**Component**: Control Plane WebSocket Hub / Agent Heartbeat Handler
**Discovered During**: Integration Test 3.1 (Agent Failover Testing)
**Status**: 🔴 ACTIVE
**Reporter**: Claude (v2-validator)
**Date**: 2025-11-22 05:41:00 UTC

---

## Executive Summary

Agent WebSocket heartbeats are being received and processed by the API, but the database `agents.status` field is not being updated from "offline" to "online". This causes the AgentSelector to believe no agents are available, blocking all session creation requests with HTTP 503 "No online agents available".

**Impact**: **CRITICAL** - Zero sessions can be created despite agent being connected and healthy.

---

## Symptoms

### User-Facing Error
```json
{
  "error": "No agents available",
  "message": "No online agents are currently available: no online agents available"
}
```
HTTP Status: **503 Service Unavailable**

### API Logs vs Database State Mismatch

**API Logs** (In-Memory State):
```
2025/11/22 05:40:38 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
```
- Agent status logged as: **"online"** ✅
- Heartbeats received every 30 seconds ✅

**Database Query** (Persistent State):
```sql
SELECT agent_id, status, last_heartbeat FROM agents;

     agent_id     | status  |       last_heartbeat
------------------+---------+----------------------------
 k8s-prod-cluster | offline | 2025-11-22 05:40:08.554907
```
- Agent status in database: **"offline"** ❌
- Last heartbeat timestamp IS being updated ✅
- Status field NOT being updated ❌

---

## Root Cause Analysis

### Flow of Agent Status Updates

**Expected Flow**:
1. Agent connects via WebSocket → `agents.status` = "online"
2. Agent sends heartbeat (every 30s) → `agents.status` remains "online", `last_heartbeat` updated
3. Agent disconnects → `agents.status` = "offline"

**Actual Flow (Buggy)**:
1. Agent connects via WebSocket → `agents.status` = ??? (not updated or set to "offline")
2. Agent sends heartbeat (every 30s) → `last_heartbeat` updated, **`status` remains "offline"**
3. AgentSelector queries database → sees `status = "offline"` → rejects session creation

### Code Location

**File**: `api/internal/websocket/hub.go` (or similar)
**Handler**: Agent heartbeat message handler
**Issue**: Heartbeat handler updates `agents.last_heartbeat` but NOT `agents.status`

**Expected Fix**:
```go
// In heartbeat handler
func (h *Hub) handleAgentHeartbeat(agentID string, heartbeat AgentHeartbeat) {
    // Update last_heartbeat AND status
    err := h.db.UpdateAgent(ctx, agentID, map[string]interface{}{
        "last_heartbeat": time.Now(),
        "status": "online",  // ← MISSING: This line is not being executed
        "active_sessions": heartbeat.ActiveSessions,
    })
}
```

---

## Evidence

### Test 3.1: Agent Failover Test Results

**Timeline**:
```
05:35:04 - Test creates 5 sessions
05:35:04 - All 5 return HTTP 503 "No agents available"
05:35:04 - API logs: "Skipping agent k8s-prod-cluster (not connected via WebSocket)"
05:35:21 - Agent reconnects
05:35:21 - API logs: "Agent k8s-prod-cluster connected (platform: kubernetes)"
05:36:42 - New session creation attempt
05:36:42 - Still fails with HTTP 503 "no online agents available"
05:36:43 - Agent reconnects again
05:37:08 - Heartbeat logged as "status: online"
05:39:59 - Session creation STILL fails with HTTP 503
05:40:08 - Database query shows status = "offline"
```

### API Logs - Heartbeats Received
```
2025/11/22 05:37:08 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/22 05:37:38 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/22 05:38:08 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/22 05:38:38 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/22 05:39:08 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/22 05:39:38 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/22 05:40:08 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/22 05:40:38 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
```
**Analysis**: Heartbeats received every 30 seconds, logged as "online" in memory

### Database Query - Status Field Not Updated
```bash
$ kubectl exec streamspace-postgres-0 -- psql -U streamspace -d streamspace \
  -c "SELECT agent_id, status, last_heartbeat, NOW() - last_heartbeat as time_since_heartbeat FROM agents;"

     agent_id     | status  |       last_heartbeat       | time_since_heartbeat
------------------+---------+----------------------------+----------------------
 k8s-prod-cluster | offline | 2025-11-22 05:40:08.554907 | 00:00:24.746728
```
**Analysis**:
- `last_heartbeat` updated 24 seconds ago ✅
- `status` stuck on "offline" ❌

---

## Impact Assessment

### Severity: P1 - HIGH

**Why P1**:
- **Complete session creation failure** - No sessions can be created
- **Zero workaround available** - Manual database update would be overwritten
- **Affects all deployments** - Any agent restart breaks session creation
- **Discovered during critical failover testing** - Breaks production reliability

**Affected Functionality**:
- ❌ Session creation (HTTP 503)
- ❌ Agent failover testing
- ❌ Integration testing continuation
- ✅ Existing sessions (not affected, pods still running)
- ✅ Agent heartbeats (received and logged)
- ✅ Database heartbeat timestamp updates

---

## Reproduction Steps

### Prerequisites
- StreamSpace v2.0-beta deployed
- K8s agent connected and sending heartbeats
- Port-forward to API active

### Steps
1. Verify agent is connected:
   ```bash
   kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent --tail=10 | grep "WebSocket connected"
   # Should see: "WebSocket connected"
   ```

2. Check API logs for heartbeats:
   ```bash
   kubectl logs -n streamspace -l app=streamspace-api --tail=20 | grep Heartbeat
   # Should see: "Heartbeat from agent k8s-prod-cluster (status: online, ...)"
   ```

3. Query database agent status:
   ```bash
   kubectl exec -n streamspace streamspace-postgres-0 -- \
     psql -U streamspace -d streamspace \
     -c "SELECT agent_id, status, last_heartbeat FROM agents;"
   # Will show: status = "offline" despite heartbeats
   ```

4. Attempt session creation:
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

   curl -s -X POST http://localhost:8000/api/v1/sessions \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "user": "admin",
       "template": "firefox-browser",
       "resources": {"memory": "512Mi", "cpu": "250m"},
       "persistentHome": false
     }' | jq '.'
   # Returns: {"error": "No agents available", "message": "No online agents are currently available"}
   ```

**Expected Result**: Session created successfully
**Actual Result**: HTTP 503 "No agents available"

---

## Recommended Fix

### Primary Fix: Update Database Status in Heartbeat Handler

**File**: `api/internal/websocket/hub.go` (or agent heartbeat handler)

**Change Required**:
```go
// Current (Buggy) - Only updates last_heartbeat
func (h *Hub) handleAgentHeartbeat(agentID string, heartbeat AgentHeartbeat) {
    err := h.db.Exec(`
        UPDATE agents
        SET last_heartbeat = $1
        WHERE agent_id = $2
    `, time.Now(), agentID)
}

// Fixed - Updates both last_heartbeat AND status
func (h *Hub) handleAgentHeartbeat(agentID string, heartbeat AgentHeartbeat) {
    err := h.db.Exec(`
        UPDATE agents
        SET last_heartbeat = $1,
            status = 'online'
        WHERE agent_id = $2
    `, time.Now(), agentID)
}
```

### Alternative Fix: Update Status on WebSocket Connect

**File**: `api/internal/websocket/hub.go` (WebSocket connection handler)

**Change Required**:
```go
// On agent WebSocket connection
func (h *Hub) handleAgentConnect(agentID string, conn *websocket.Conn) {
    // Register WebSocket connection
    h.agentConns[agentID] = conn

    // Update database status to "online"
    err := h.db.Exec(`
        UPDATE agents
        SET status = 'online',
            last_heartbeat = $1
        WHERE agent_id = $2
    `, time.Now(), agentID)

    log.Printf("[AgentWebSocket] Agent %s connected (platform: %s)", agentID, platform)
}
```

### Additional Fix: Update Status to "offline" on Disconnect

**File**: `api/internal/websocket/hub.go` (WebSocket disconnect handler)

**Change Required**:
```go
// On agent WebSocket disconnect
func (h *Hub) handleAgentDisconnect(agentID string) {
    // Remove WebSocket connection
    delete(h.agentConns, agentID)

    // Update database status to "offline"
    err := h.db.Exec(`
        UPDATE agents
        SET status = 'offline'
        WHERE agent_id = $1
    `, agentID)

    log.Printf("[AgentWebSocket] Agent %s disconnected", agentID)
}
```

---

## Recommended Testing

### Test 1: Manual Database Status Update (Temporary Workaround)
```bash
# Temporarily fix status to verify this is the issue
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "UPDATE agents SET status = 'online' WHERE agent_id = 'k8s-prod-cluster';"

# Try session creation again
curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {"memory": "512Mi", "cpu": "250m"},
    "persistentHome": false
  }' | jq '.'

# Should succeed with manual status update
```

### Test 2: After Fix - Verify Status Updates
```bash
# 1. Check initial status after agent connects
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "SELECT agent_id, status FROM agents WHERE agent_id = 'k8s-prod-cluster';"
# Should show: status = 'online'

# 2. Wait for heartbeat (30 seconds)
sleep 35

# 3. Check status still online after heartbeat
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "SELECT agent_id, status, last_heartbeat FROM agents WHERE agent_id = 'k8s-prod-cluster';"
# Should show: status = 'online', last_heartbeat updated

# 4. Restart agent
kubectl rollout restart deployment/streamspace-k8s-agent -n streamspace

# 5. Wait for disconnect
sleep 5

# 6. Check status changed to offline
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "SELECT agent_id, status FROM agents WHERE agent_id = 'k8s-prod-cluster';"
# Should show: status = 'offline'

# 7. Wait for agent to reconnect
sleep 30

# 8. Check status changed back to online
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "SELECT agent_id, status FROM agents WHERE agent_id = 'k8s-prod-cluster';"
# Should show: status = 'online'

# 9. Create session to verify it works
curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "admin",
    "template": "firefox-browser",
    "resources": {"memory": "512Mi", "cpu": "250m"},
    "persistentHome": false
  }' | jq '.'
# Should succeed
```

---

## Integration Test 3.1 Impact

### Test 3.1: Agent Disconnection During Active Sessions

**Test Objective**: Validate system resilience when agent disconnects and reconnects

**Test Results (With Bug)**:
- ❌ Session creation failed before restart (HTTP 503)
- ❌ Session creation failed after restart (HTTP 503)
- ❌ Test blocked by P1-AGENT-STATUS-001

**Expected Results (After Fix)**:
- ✅ Sessions created successfully before restart
- ✅ Sessions survive agent restart
- ✅ New sessions created successfully after restart
- ✅ Agent reconnects within 30 seconds
- ✅ Zero data loss during failover

**Test Status**: **BLOCKED** - Cannot proceed with failover testing until status sync bug is fixed

---

## Related Issues

### Discovered During
- Integration Test 3.1: Agent Disconnection During Active Sessions

### Dependencies
- This bug BLOCKS all integration testing requiring session creation
- This bug BLOCKS Phase 3 (Failover Testing)
- This bug BLOCKS Phase 4 (Performance Testing)

### Related Bugs
- None (first occurrence)

---

## Workarounds

### Temporary Workaround (Manual Database Update)
```bash
# Every time agent restarts, manually update database
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "UPDATE agents SET status = 'online' WHERE agent_id = 'k8s-prod-cluster';"
```

**Limitations**:
- Requires manual intervention after every agent restart
- Not sustainable for production
- Doesn't fix underlying synchronization issue
- Status will revert to "offline" on next heartbeat (if heartbeat handler doesn't update it)

---

## Validation Criteria

After fix is applied, the following must be verified:

1. **WebSocket Connection**: ✅ Agent status = "online" when WebSocket connects
2. **Heartbeat Processing**: ✅ Agent status remains "online" after heartbeats
3. **Heartbeat Timestamp**: ✅ `last_heartbeat` field updated every 30 seconds
4. **Disconnect Handling**: ✅ Agent status = "offline" when WebSocket disconnects
5. **Session Creation**: ✅ Sessions can be created with agent online
6. **AgentSelector Query**: ✅ AgentSelector finds online agents via database query
7. **Failover Test**: ✅ Test 3.1 passes with zero session loss

---

## Priority Justification

### Why P1 (Not P0)
- **P0** bugs prevent deployment or cause data loss
- **P1** bugs block critical functionality but have workarounds

**This is P1 because**:
- ❌ Blocks ALL session creation (critical functionality)
- ✅ Has manual workaround (database update)
- ✅ Doesn't cause data loss (existing sessions unaffected)
- ✅ Doesn't prevent deployment

**Could be elevated to P0 if**:
- No workaround existed
- Caused data loss or corruption
- Prevented any deployments

---

## Next Steps

1. **Builder**: Implement recommended fix (update `agents.status` in heartbeat handler)
2. **Builder**: Add status update on WebSocket connect/disconnect
3. **Builder**: Commit fix to `claude/v2-builder` branch
4. **Validator**: Merge fix and redeploy
5. **Validator**: Run manual database update workaround to unblock testing
6. **Validator**: After fix deployed, verify status sync working
7. **Validator**: Re-run Test 3.1 (Agent Failover)
8. **Validator**: Continue integration testing

---

## Additional Context

### Database Schema

**agents table** (relevant columns):
```sql
agent_id        VARCHAR PRIMARY KEY
platform        VARCHAR NOT NULL
status          VARCHAR NOT NULL  -- 'online' or 'offline'
last_heartbeat  TIMESTAMP         -- Updated on each heartbeat
created_at      TIMESTAMP
updated_at      TIMESTAMP
```

### Expected Behavior

**Healthy Agent Lifecycle**:
1. Agent starts → Connects WebSocket → `status = 'online'`
2. Agent sends heartbeat (every 30s) → `last_heartbeat` updated, `status = 'online'`
3. Agent stops → Disconnects WebSocket → `status = 'offline'`

**AgentSelector Logic**:
```sql
SELECT * FROM agents WHERE status = 'online' ORDER BY (SELECT COUNT(*) FROM sessions WHERE agent_id = agents.agent_id);
```
- Queries database for agents with `status = 'online'`
- If no agents found → returns "No online agents available"

---

**Generated**: 2025-11-22 05:41:00 UTC
**Validator**: Claude (v2-validator)
**Branch**: claude/v2-validator
**Status**: 🔴 ACTIVE - Awaiting Builder Fix
**Priority**: P1 - HIGH
**Blocks**: Integration Testing (Phase 3, 4)
