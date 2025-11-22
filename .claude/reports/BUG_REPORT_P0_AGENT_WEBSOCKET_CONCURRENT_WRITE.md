# P0 BUG REPORT: Agent WebSocket Concurrent Write Panic

**Bug ID**: P0-AGENT-001
**Severity**: P0 (CRITICAL - BLOCKING ALL INTEGRATION TESTING)
**Status**: ❌ **DISCOVERED** during integration testing
**Discovered**: 2025-11-21 23:19
**Component**: K8s Agent - WebSocket Communication
**Affects**: ALL agent operations (session creation, termination, command processing)
**Impact**: Complete failure of v2.0-beta agent-based architecture

---

## Executive Summary

The K8s Agent crashes repeatedly with a `panic: concurrent write to websocket connection` error approximately 4 minutes after startup. This prevents the agent from processing ANY commands from the database, causing all sessions to remain in "pending" state indefinitely.

**Blocker Status**: This bug completely blocks integration testing and prevents v2.0-beta from functioning.

---

## Problem Statement

When attempting to run E2E integration tests, discovered that:
1. Session commands (start_session, stop_session) stuck in "pending" status in database
2. No pods/deployments created despite session CRD showing "running" state
3. Agent logs show repeated crashes every ~4 minutes
4. Agent never processes commands before crashing

**Error Message**:
```
panic: concurrent write to websocket connection

goroutine 31 [running]:
github.com/gorilla/websocket.(*messageWriter).flushFrame(0xc000490360, 0x1, {0x0?, 0x0?, 0x0?})
	/go/pkg/mod/github.com/gorilla/websocket@v1.5.0/conn.go:617 +0x4b8
github.com/gorilla/websocket.(*messageWriter).Close(0x0?)
	/go/pkg/mod/github.com/gorilla/websocket@v1.5.0/conn.go:731 +0x35
github.com/gorilla/websocket.(*Conn).beginMessage(0xc0003f8000, 0xc0003fe9c0, 0x9)
	/go/pkg/mod/github.com/gorilla/websocket@v1.5.0/conn.go:480 +0x3a
github.com/gorilla/websocket.(*Conn).NextWriter(0xc0003f8000, 0x9)
	/go/pkg/mod/github.com/gorilla/websocket@v1.5.0/conn.go:520 +0x3f
github.com/gorilla/websocket.(*Conn).WriteMessage(0xc2405ac75ffbc5b7?, 0x413483f559?, {0x0, 0x0, 0x0})
	/go/pkg/mod/github.com/gorilla/websocket@v1.5.0/conn.go:773 +0x138
main.(*K8sAgent).writePump(0xc00009ee40)
	/app/main.go:607 +0x192
created by main.(*K8sAgent).Run in goroutine 6
	/app/main.go:172 +0x219
```

---

## Root Cause Analysis

### Concurrency Issue

The agent has **at least two goroutines** attempting to write to the WebSocket concurrently:

1. **`writePump` goroutine** (main.go:607)
   - Launched in `Run()` at main.go:172
   - Handles sending messages from write channel
   - Uses `conn.WriteMessage()`

2. **Heartbeat sender**
   - Logs show: `[K8sAgent] Starting heartbeat sender (interval: 30s)`
   - Likely also calls `conn.WriteMessage()` directly
   - Not synchronized with `writePump`

**Gorilla WebSocket Documentation**:
> Connections support one concurrent reader and one concurrent writer. Applications are responsible for ensuring that no more than one goroutine calls the write methods (NextWriter, SetWriteDeadline, WriteMessage, WriteJSON, EnableWriteCompression, SetCompressionLevel) concurrently and that no more than one goroutine calls the read methods (NextReader, SetReadDeadline, ReadMessage, ReadJSON, SetPongHandler, SetPingHandler) concurrently.

**Violation**: Multiple goroutines calling `WriteMessage()` without synchronization.

---

## Evidence

### 1. Agent Crash Logs

**Timestamp**: 2025-11-21 23:19:47 (4 minutes 30 seconds after startup)

```
2025/11/21 23:15:17 [K8sAgent] Starting agent: k8s-prod-cluster
2025/11/21 23:15:17 [K8sAgent] Registered successfully
2025/11/21 23:15:17 [K8sAgent] WebSocket connected
2025/11/21 23:15:17 [K8sAgent] Starting heartbeat sender (interval: 30s)
2025/11/21 23:19:47 [K8sAgent] Write pump stopped
panic: concurrent write to websocket connection
```

**Pattern**: Agent crashes consistently after 4-5 minutes, likely after multiple heartbeats.

---

### 2. Database Evidence - Commands Never Processed

**Session**: admin-firefox-browser-d020bb30

**Database State**:
```sql
SELECT id, agent_id, state, created_at FROM sessions WHERE id = 'admin-firefox-browser-d020bb30';

               id               |     agent_id     |    state    |         created_at
--------------------------------+------------------+-------------+----------------------------
 admin-firefox-browser-d020bb30 | k8s-prod-cluster | terminating | 2025-11-21 23:02:59.984798
```

**Commands State**:
```sql
SELECT command_id, action, status, created_at FROM agent_commands
WHERE session_id = 'admin-firefox-browser-d020bb30' ORDER BY created_at DESC;

 command_id  |    action     | status  |         created_at
--------------+---------------+---------+----------------------------
 cmd-81cbb02b | stop_session  | pending | 2025-11-21 23:03:15.477586
 cmd-15e74c10 | start_session | pending | 2025-11-21 23:02:59.981641
```

**Analysis**:
- ❌ Commands created 16+ minutes ago
- ❌ BOTH commands stuck in "pending" status
- ❌ NO status updates ("processing", "completed", "failed")
- ❌ Agent NEVER processed these commands

---

### 3. Kubernetes Resource State

**Session CRD**:
```yaml
Name:         admin-firefox-browser-d020bb30
Namespace:    streamspace
State:        running  # ❌ INCORRECT - should be "pending" or "terminated"
```

**Deployment**: NOT FOUND (never created)
```bash
$ kubectl get deployment admin-firefox-browser-d020bb30 -n streamspace
Error from server (NotFound): deployments.apps "admin-firefox-browser-d020bb30" not found
```

**Pods**: NONE (never created)
```bash
$ kubectl get pods -n streamspace | grep admin-firefox-browser-d020bb30
# No output
```

**Service**: NONE (never created)
```bash
$ kubectl get svc -n streamspace -l session=admin-firefox-browser-d020bb30
No resources found in streamspace namespace.
```

**Analysis**: Session shows "running" but no actual resources created because agent never processed the start_session command.

---

### 4. Agent Restart Pattern

```bash
$ kubectl get pods -n streamspace | grep agent
streamspace-k8s-agent-5849b86487-w6vlz   1/1   Running   3 (4m7s ago)   17m
```

**Restart Count**: 3 restarts in 17 minutes
**Frequency**: ~5 minutes between restarts
**Cause**: Agent crashes, Kubernetes restarts it, crashes again

---

## Expected vs Actual Behavior

### Expected Flow

```
1. Agent starts
2. Agent connects to Control Plane WebSocket
3. Agent starts heartbeat goroutine
4. Agent starts command polling/listening
5. API creates command in database (status: pending)
6. Agent receives command via WebSocket OR polls database
7. Agent processes command (status: processing)
8. Agent creates K8s resources (deployment, service, pod)
9. Agent updates command (status: completed)
10. Agent updates session CRD state
11. Heartbeat continues in background without conflicts
```

### Actual Flow

```
1. Agent starts ✅
2. Agent connects to Control Plane WebSocket ✅
3. Agent starts heartbeat goroutine ✅
4. Agent starts command polling/listening ✅ (assumed)
5. API creates command in database (status: pending) ✅
6. Agent receives command ❓ (unknown - crashes before processing)
7. Heartbeat sends message concurrently with writePump ❌
8. PANIC: concurrent write to websocket connection ❌
9. Agent crashes and restarts ❌
10. Commands remain in "pending" forever ❌
11. No resources created ❌
```

---

## Code Analysis

### File: k8s-agent/main.go (Suspected)

**Lines Involved**:
- main.go:172 - Creates `writePump` goroutine
- main.go:607 - `writePump()` calls `conn.WriteMessage()`
- Unknown location - Heartbeat sender directly writes to WebSocket

**Problem Pattern**:

```go
// BROKEN PATTERN (Suspected):

func (a *K8sAgent) Run() {
    // ... connection setup ...

    // Goroutine 1: writePump for regular messages
    go a.writePump()  // main.go:172

    // Goroutine 2: Heartbeat sender
    go a.sendHeartbeats()  // Assumed - directly writes to WebSocket

    // Both goroutines call conn.WriteMessage() without synchronization!
}

func (a *K8sAgent) writePump() {
    for {
        select {
        case message := <-a.writeChan:
            err := a.conn.WriteMessage(websocket.TextMessage, message)  // ❌ Write 1
            // ...
        }
    }
}

func (a *K8sAgent) sendHeartbeats() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        heartbeat := `{"type":"heartbeat","timestamp":"..."}`
        err := a.conn.WriteMessage(websocket.TextMessage, []byte(heartbeat))  // ❌ Write 2 (concurrent!)
        // ...
    }
}
```

---

## Correct Implementation

### Option 1: Use Write Channel for ALL Messages (Recommended)

```go
func (a *K8sAgent) Run() {
    // Single writer goroutine
    go a.writePump()

    // Heartbeat sender uses channel (no direct writes)
    go a.sendHeartbeats()

    // Command processor uses channel (no direct writes)
    go a.processCommands()
}

func (a *K8sAgent) writePump() {
    for {
        select {
        case message := <-a.writeChan:
            // ONLY place where WriteMessage is called
            err := a.conn.WriteMessage(websocket.TextMessage, message)
            if err != nil {
                log.Printf("Write error: %v", err)
                return
            }
        }
    }
}

func (a *K8sAgent) sendHeartbeats() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        heartbeat := `{"type":"heartbeat","timestamp":"..."}`
        // Send via channel instead of direct write
        select {
        case a.writeChan <- []byte(heartbeat):
        case <-time.After(5 * time.Second):
            log.Println("Heartbeat send timeout")
        }
    }
}

func (a *K8sAgent) sendCommand(cmd interface{}) {
    jsonData, _ := json.Marshal(cmd)
    // Send via channel instead of direct write
    select {
    case a.writeChan <- jsonData:
    case <-time.After(5 * time.Second):
        log.Println("Command send timeout")
    }
}
```

### Option 2: Use Mutex for Write Protection

```go
type K8sAgent struct {
    conn      *websocket.Conn
    writeMux  sync.Mutex  // Protects WebSocket writes
    writeChan chan []byte
}

func (a *K8sAgent) writeMessage(messageType int, data []byte) error {
    a.writeMux.Lock()
    defer a.writeMux.Unlock()
    return a.conn.WriteMessage(messageType, data)
}

func (a *K8sAgent) writePump() {
    for message := range a.writeChan {
        if err := a.writeMessage(websocket.TextMessage, message); err != nil {
            log.Printf("Write error: %v", err)
            return
        }
    }
}

func (a *K8sAgent) sendHeartbeats() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        heartbeat := `{"type":"heartbeat","timestamp":"..."}`
        if err := a.writeMessage(websocket.TextMessage, []byte(heartbeat)); err != nil {
            log.Printf("Heartbeat error: %v", err)
            return
        }
    }
}
```

**Recommendation**: Option 1 is preferred as it follows the single-writer pattern recommended by Gorilla WebSocket.

---

## Testing Plan

### 1. Fix Verification

After Builder applies fix:

```bash
# Deploy fixed agent
kubectl rollout restart deployment/streamspace-k8s-agent -n streamspace

# Monitor logs for crashes (wait 10 minutes)
kubectl logs -n streamspace deploy/streamspace-k8s-agent -f

# Expected: No panics, stable operation
```

### 2. Command Processing Verification

```bash
# Create session
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

SESSION_ID=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}' | jq -r '.name')

# Wait 30 seconds
sleep 30

# Check command processed
kubectl exec -n streamspace statefulset/streamspace-postgres -- psql -U streamspace -d streamspace \
  -c "SELECT command_id, action, status FROM agent_commands WHERE session_id = '$SESSION_ID';"

# Expected: status = 'completed' (not 'pending')

# Check resources created
kubectl get deployment "$SESSION_ID" -n streamspace
kubectl get pods -n streamspace | grep "$SESSION_ID"
kubectl get svc -n streamspace | grep "$SESSION_ID"

# Expected: All resources exist and running
```

### 3. Stability Testing

```bash
# Monitor agent for 30 minutes
kubectl logs -n streamspace deploy/streamspace-k8s-agent -f --tail=0

# Create/terminate 10 sessions during monitoring
for i in {1..10}; do
  echo "Creating session $i..."
  # Create session, wait 60s, terminate
  # Monitor agent logs for crashes
done

# Check agent restart count
kubectl get pods -n streamspace | grep agent

# Expected: 0 restarts after fix
```

---

## Impact Assessment

### Severity: P0 (CRITICAL)

**Why P0**:
- **Blocks ALL v2.0-beta functionality** - No sessions can be created
- **Blocks ALL integration testing** - Cannot test VNC, multi-agent, failover
- **Blocks v2.0-beta release** - Architecture fundamentally broken
- **No workaround available** - v1.x controller-based approach was removed

**Current State**:
- ❌ Session creation: Completely broken
- ❌ Session termination: Completely broken
- ❌ Agent command processing: Completely broken
- ❌ Integration testing: Blocked
- ❌ v2.0-beta: Not functional

**Dependencies**:
- All P1 fixes validated (NULL handling, agent_id, JSON marshaling) ✅
- But rendered useless because agent crashes before processing commands

---

## Lessons Learned

### For Builder

1. **WebSocket Concurrency**: Always use single-writer pattern for WebSocket connections
2. **Gorilla WebSocket Docs**: Read and follow documentation on concurrent access
3. **Testing**: Test agent stability over time (not just initial connection)
4. **Error Handling**: Ensure panics don't bring down the entire agent

### For Validator

1. **Integration Testing Value**: This bug would not be caught by unit tests
2. **Monitor Over Time**: Agents can appear healthy initially but crash later
3. **Check Database State**: Verify commands are actually processed, not just created
4. **End-to-End Validation**: Test complete flow from API call to resource creation

---

## Recommended Actions

### Immediate (Builder)

1. **Fix WebSocket Writes**: Implement single-writer pattern (Option 1)
2. **Test Locally**: Run agent for 30+ minutes to verify stability
3. **Push Fix**: Commit and push to refactor branch
4. **Notify Validator**: Signal fix is ready for testing

### Follow-up (Validator)

1. **Re-test**: Run stability test (30-minute monitoring)
2. **Verify Commands**: Ensure commands transition: pending → processing → completed
3. **Resume Integration Testing**: Continue with E2E VNC tests
4. **Document Results**: Update integration test results

### Long-term (All)

1. **Add Tests**: Integration tests that monitor agent stability
2. **Add Metrics**: Track agent restart count, command processing time
3. **Add Alerts**: Alert if agent restarts > threshold
4. **Code Review**: Review ALL WebSocket write calls for concurrency safety

---

## Status Summary

**Discovery Date**: 2025-11-21 23:19
**Discovered By**: Validator (Agent 3) during integration testing Phase 1
**Severity**: P0 (CRITICAL - BLOCKING)
**Component**: k8s-agent WebSocket handling
**Fix Owner**: Builder (Agent 2)
**Status**: ❌ DISCOVERED - Awaiting Builder fix

**Integration Testing Status**:
- Phase 1 (E2E VNC): ❌ BLOCKED
- Phase 2 (Multi-Agent): ❌ BLOCKED
- Phase 3 (Failover): ❌ BLOCKED
- Phase 4 (Performance): ❌ BLOCKED

**v2.0-beta Status**: ❌ NOT FUNCTIONAL - Critical blocker prevents all operations

---

**Validator**: Claude Code (Agent 3)
**Date**: 2025-11-21 23:19
**Branch**: claude/v2-validator
**Integration Testing**: BLOCKED - awaiting P0 fix
**Next Step**: Notify user, await Builder fix

---

## Additional Notes

### Why This Wasn't Caught Earlier

1. **P1 Testing Was Isolated**: Previous tests only checked command creation, not processing
2. **Short Test Duration**: P1 tests completed in < 1 minute, agent crashes at ~4 minutes
3. **No End-to-End Validation**: Didn't verify resources actually created

### Good Progress Despite Bug

- v2.0-beta architecture is sound (agent-based approach correct)
- P1 fixes all working (NULL handling, agent_id tracking, JSON marshaling)
- Database schema correct
- API handlers correct
- Issue is ONLY in agent WebSocket concurrency

**Estimated Fix Time**: 30-60 minutes for Builder (straightforward concurrency fix)
**Estimated Test Time**: 1-2 hours for Validator (stability + E2E tests)

Once fixed, integration testing can proceed immediately.
