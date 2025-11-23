# Bug Report: P1-COMMAND-SCAN-001 - CommandDispatcher Fails to Scan Pending Commands with NULL error_message

**Bug ID**: P1-COMMAND-SCAN-001
**Severity**: P1 - HIGH (Blocks command retry during agent downtime)
**Component**: Control Plane Command Dispatcher
**Discovered During**: Integration Test 3.2 (Command Retry During Agent Downtime)
**Status**: 🔴 ACTIVE
**Reporter**: Claude (v2-validator)
**Date**: 2025-11-22 06:17:00 UTC

---

## Executive Summary

The CommandDispatcher fails to scan pending commands from the `agent_commands` table when the `error_message` column contains NULL values. This prevents the CommandDispatcher from processing any pending commands, causing commands sent during agent downtime to never be processed even after the agent reconnects.

**Impact**: **CRITICAL** - Command retry functionality completely broken. Commands queued during agent downtime are never processed.

---

## Symptoms

### API Logs (Repeated Error)

```
[CommandDispatcher] Failed to scan pending command: sql: Scan error on column index 7, name "error_message": converting NULL to string is unsupported
```

**Frequency**: Every time CommandDispatcher tries to load pending commands
**Result**: Pending commands are not loaded, therefore not processed

---

### User-Facing Impact

**Scenario**: Agent goes down → User sends session termination command → Agent reconnects

**Expected Behavior**:
1. API accepts termination command (HTTP 202) ✅
2. Command stored in `agent_commands` table with status "pending" ✅
3. CommandDispatcher loads pending commands ❌ **FAILS HERE**
4. CommandDispatcher sends command to agent after reconnection ❌ Never happens
5. Agent processes command and terminates session ❌ Never happens

**Actual Behavior**:
- Command stuck in "pending" status forever
- Session pod never terminated
- No error visible to user (command appears "accepted")

---

## Root Cause Analysis

### Database Schema

**Table**: `agent_commands`

```sql
Column: error_message
Type: text
Nullable: YES (can be NULL)
Default: NULL
```

**Commands in "pending" status** have `error_message = NULL` (no error yet)

---

### Go Code Issue

**File**: `api/internal/websocket/command_dispatcher.go` (or similar)

**Problematic Code** (suspected):
```go
type AgentCommand struct {
    CommandID      string
    AgentID        string
    SessionID      string
    Action         string
    Payload        json.RawMessage
    Status         string
    ErrorMessage   string    // ← PROBLEM: Should be *string or sql.NullString
    CreatedAt      time.Time
    SentAt         *time.Time
    AcknowledgedAt *time.Time
    CompletedAt    *time.Time
}

func (d *CommandDispatcher) loadPendingCommands() ([]*AgentCommand, error) {
    rows, err := d.db.Query(`
        SELECT command_id, agent_id, session_id, action, payload,
               status, error_message, created_at
        FROM agent_commands
        WHERE status = 'pending'
        ORDER BY created_at ASC
    `)

    for rows.Next() {
        cmd := &AgentCommand{}
        err := rows.Scan(
            &cmd.CommandID,
            &cmd.AgentID,
            &cmd.SessionID,
            &cmd.Action,
            &cmd.Payload,
            &cmd.Status,
            &cmd.ErrorMessage,  // ← FAILS when NULL (string cannot be NULL)
            &cmd.CreatedAt,
        )
        // Error logged but command skipped, loop continues
    }
}
```

**Fix Required**:
```go
type AgentCommand struct {
    CommandID      string
    AgentID        string
    SessionID      string
    Action         string
    Payload        json.RawMessage
    Status         string
    ErrorMessage   *string   // ← FIX: Use pointer to string (or sql.NullString)
    CreatedAt      time.Time
    SentAt         *time.Time
    AcknowledgedAt *time.Time
    CompletedAt    *time.Time
}
```

---

## Evidence

### Test 3.2: Command Retry During Agent Downtime

**Test Flow**:
1. ✅ Session created: `admin-firefox-browser-1edf5ee9`
2. ✅ Session pod running: `admin-firefox-browser-1edf5ee9-5fff477c55-bnwg4`
3. ✅ Agent pod killed: `streamspace-k8s-agent-69748cbdfc-s4bbq`
4. ✅ Termination command sent while agent down (HTTP 202)
5. ✅ Command stored in database:
   ```
   command_id: cmd-26acdfcf
   session_id: admin-firefox-browser-1edf5ee9
   action: stop_session
   status: pending
   error_message: NULL
   ```
6. ✅ Agent reconnected in 3 seconds
7. ❌ Command NOT processed (stuck in "pending" after 30+ seconds)
8. ❌ Session pod still running

---

### API Logs Analysis

**Timeline**:
```
06:10:36 - API pods started after restart
06:10:36 - CommandDispatcher workers started
06:10:36 - CommandDispatcher tried to load pending commands
06:10:36 - Scan errors repeated (21+ times)
06:16:00 - Test 3.2 started
06:16:33 - New command created (cmd-26acdfcf)
06:16:38 - Agent reconnected
06:17:00+ - Command still "pending" (never processed)
```

**Evidence**: CommandDispatcher has been broken since API restart

---

### Database Query

**Check pending commands**:
```sql
SELECT command_id, session_id, action, status, error_message, created_at
FROM agent_commands
WHERE status = 'pending'
ORDER BY created_at DESC;
```

**Result**: Commands exist but are never scanned successfully by CommandDispatcher

---

## Impact Assessment

### Severity: P1 - HIGH

**Why P1**:
- **Complete command retry failure** - Commands queued during downtime never processed
- **Affects agent failover** - Primary use case for command queuing
- **Silent failure** - Users get HTTP 202 but command never executes
- **Data accumulation** - Pending commands accumulate in database forever

**Affected Functionality**:
- ❌ Command retry during agent downtime (Test 3.2)
- ❌ Graceful agent restart scenarios
- ❌ Network disruption recovery
- ❌ Agent maintenance windows
- ✅ Real-time commands (when agent connected) - still work
- ✅ Session creation - still works
- ✅ Agent heartbeats - still work

**Why Not P0**:
- Real-time commands still work (when agent is connected)
- System remains functional for live operations
- Has workaround (manual command retry or database fix)

---

## Reproduction Steps

### Prerequisites
- StreamSpace v2.0-beta deployed
- K8s agent connected
- Port-forward to API active

### Steps

1. Create a test session:
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

   SESSION_ID=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "user": "admin",
       "template": "firefox-browser",
       "resources": {"memory": "512Mi", "cpu": "250m"},
       "persistentHome": false
     }' | jq -r '.name')

   echo "Session created: $SESSION_ID"
   ```

2. Wait for session pod to be running:
   ```bash
   kubectl wait --for=condition=ready pod -l "session=${SESSION_ID}" -n streamspace --timeout=60s
   ```

3. Kill the agent pod:
   ```bash
   kubectl delete pod -n streamspace -l app.kubernetes.io/component=k8s-agent
   ```

4. Immediately send termination command:
   ```bash
   curl -X DELETE "http://localhost:8000/api/v1/sessions/${SESSION_ID}" \
     -H "Authorization: Bearer $TOKEN"
   # Should return HTTP 202
   ```

5. Verify command queued:
   ```bash
   kubectl exec -n streamspace streamspace-postgres-0 -- \
     psql -U streamspace -d streamspace \
     -c "SELECT command_id, action, status, error_message FROM agent_commands WHERE session_id = '${SESSION_ID}';"
   # Should show: status = 'pending', error_message = NULL
   ```

6. Wait for agent to reconnect (30 seconds):
   ```bash
   sleep 30
   kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=k8s-agent -n streamspace --timeout=60s
   ```

7. Check command status again:
   ```bash
   kubectl exec -n streamspace streamspace-postgres-0 -- \
     psql -U streamspace -d streamspace \
     -c "SELECT command_id, action, status FROM agent_commands WHERE session_id = '${SESSION_ID}';"
   # Still shows: status = 'pending' (NOT processed)
   ```

8. Check if session pod still running:
   ```bash
   kubectl get pod -n streamspace -l "session=${SESSION_ID}"
   # Pod still exists (command was never processed)
   ```

9. Check API logs for scan errors:
   ```bash
   kubectl logs -n streamspace -l app.kubernetes.io/component=api --tail=50 | grep CommandDispatcher
   # Shows repeated: "Failed to scan pending command: sql: Scan error on column index 7"
   ```

**Expected Result**: Command processed, session terminated
**Actual Result**: Command stuck in "pending", session still running

---

## Recommended Fix

### Primary Fix: Change ErrorMessage to Nullable Type

**File**: `api/internal/websocket/command_dispatcher.go` (or wherever AgentCommand struct is defined)

**Change Required**:
```go
// Before (Buggy)
type AgentCommand struct {
    CommandID      string
    AgentID        string
    SessionID      string
    Action         string
    Payload        json.RawMessage
    Status         string
    ErrorMessage   string    // ← Cannot handle NULL
    CreatedAt      time.Time
    SentAt         *time.Time
    AcknowledgedAt *time.Time
    CompletedAt    *time.Time
}

// After (Fixed - Option 1: Use pointer)
type AgentCommand struct {
    CommandID      string
    AgentID        string
    SessionID      string
    Action         string
    Payload        json.RawMessage
    Status         string
    ErrorMessage   *string   // ← Can handle NULL
    CreatedAt      time.Time
    SentAt         *time.Time
    AcknowledgedAt *time.Time
    CompletedAt    *time.Time
}

// After (Fixed - Option 2: Use sql.NullString)
type AgentCommand struct {
    CommandID      string
    AgentID        string
    SessionID      string
    Action         string
    Action         string
    Payload        json.RawMessage
    Status         string
    ErrorMessage   sql.NullString   // ← Can handle NULL
    CreatedAt      time.Time
    SentAt         *time.Time
    AcknowledgedAt *time.Time
    CompletedAt    *time.Time
}
```

**Recommendation**: Use `*string` (pointer) for cleaner code and better JSON marshaling

---

### Code Locations to Update

**Scan Operation** (`loadPendingCommands()`):
```go
func (d *CommandDispatcher) loadPendingCommands() ([]*AgentCommand, error) {
    // ... query ...

    for rows.Next() {
        cmd := &AgentCommand{}
        err := rows.Scan(
            &cmd.CommandID,
            &cmd.AgentID,
            &cmd.SessionID,
            &cmd.Action,
            &cmd.Payload,
            &cmd.Status,
            &cmd.ErrorMessage,  // Now *string, handles NULL correctly
            &cmd.CreatedAt,
        )
        if err != nil {
            log.Printf("[CommandDispatcher] Failed to scan pending command: %v", err)
            continue  // Still logged, but now should work
        }
        commands = append(commands, cmd)
    }
    return commands, nil
}
```

**Update Command Status**:
```go
func (d *CommandDispatcher) markCommandFailed(commandID, errorMsg string) error {
    _, err := d.db.Exec(`
        UPDATE agent_commands
        SET status = 'failed', error_message = $1
        WHERE command_id = $2
    `, errorMsg, commandID)  // errorMsg is string, not pointer
    return err
}
```

**JSON Marshaling** (automatic with `*string`):
```go
// With *string, JSON marshaling handles NULL automatically
// NULL → null (in JSON)
// "error" → "error" (in JSON)
```

---

## Validation Testing

### After Fix Applied

**Test 1: Verify Scan Works**
```bash
# Check API logs after restart
kubectl logs -n streamspace -l app.kubernetes.io/component=api --tail=50 | grep CommandDispatcher
# Should NOT show: "Failed to scan pending command"
```

**Test 2: Verify Pending Commands Loaded**
```bash
# Create some pending commands (run Test 3.2)
# Check API logs
kubectl logs -n streamspace -l app.kubernetes.io/component=api | grep "Loaded.*pending commands"
# Should show: "Loaded X pending commands"
```

**Test 3: Run Test 3.2 (Command Retry)**
```bash
/Users/s0v3r1gn/streamspace/streamspace-validator/tests/scripts/test_command_retry_agent_downtime.sh
# Should PASS: Command processed after agent reconnection
```

**Test 4: Verify Command Processing**
```bash
# After Test 3.2 completes
# Command should be status = 'completed', not 'pending'
# Session pod should be deleted
```

---

## Integration Test 3.2 Impact

### Test 3.2: Command Retry During Agent Downtime

**Test Objective**: Validate commands queued during agent downtime are processed after reconnection

**Test Results (With Bug)**:
- ✅ Command queuing works (HTTP 202, command stored in database)
- ❌ Command processing BLOCKED (scan error prevents loading)
- ❌ Agent reconnection doesn't help (commands never loaded)
- ❌ Commands accumulate in database forever

**Expected Results (After Fix)**:
- ✅ Command queued during downtime
- ✅ Command loaded by CommandDispatcher
- ✅ Command sent to agent after reconnection
- ✅ Agent processes command
- ✅ Session terminated successfully

**Test Status**: **BLOCKED** - Cannot proceed with Test 3.2 until fix applied

---

## Related Issues

### Discovered During
- Integration Test 3.2: Command Retry During Agent Downtime

### Dependencies
- This bug BLOCKS Test 3.2 (Command Retry)
- This bug affects agent failover reliability
- This bug affects Test 3.1 command processing during failover

### Related Bugs
- P1-AGENT-STATUS-001 (Agent status sync) - RESOLVED
- P0-MANIFEST-001 (Template manifest parsing) - RESOLVED
- P1-VNC-RBAC-001 (VNC tunnel RBAC) - RESOLVED

---

## Workarounds

### Temporary Workaround 1: Update error_message to empty string

**WARNING**: This only fixes EXISTING commands, new commands will still fail

```bash
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "UPDATE agent_commands SET error_message = '' WHERE error_message IS NULL AND status = 'pending';"
```

**Limitations**:
- Only fixes existing commands
- New commands will still have NULL error_message and fail to scan
- Need to run after every command creation
- Not sustainable

---

### Temporary Workaround 2: Manual Command Processing

**Process pending commands manually**:

1. Get pending commands:
   ```bash
   kubectl exec -n streamspace streamspace-postgres-0 -- \
     psql -U streamspace -d streamspace \
     -c "SELECT command_id, session_id, action FROM agent_commands WHERE status = 'pending';"
   ```

2. For each command, manually execute via API or kubectl

3. Update command status:
   ```bash
   kubectl exec -n streamspace streamspace-postgres-0 -- \
     psql -U streamspace -d streamspace \
     -c "UPDATE agent_commands SET status = 'completed' WHERE command_id = 'cmd-xxx';"
   ```

**Limitations**:
- Manual intervention required
- Not scalable
- Defeats purpose of command retry
- Not sustainable for production

---

## Priority Justification

### Why P1 (Not P0)

- **P0** bugs prevent deployment or cause complete system failure
- **P1** bugs block critical functionality but system remains partially functional

**This is P1 because**:
- ❌ Blocks command retry (critical feature)
- ❌ Breaks agent failover scenarios
- ✅ Real-time commands still work (when agent connected)
- ✅ Has workarounds (manual processing)
- ✅ Doesn't prevent deployment
- ✅ Doesn't cause data loss

**Could be elevated to P0 if**:
- Real-time commands also broken
- No workaround existed
- Caused data corruption
- Prevented any deployments

---

## Next Steps

1. **Builder**: Implement recommended fix (change ErrorMessage to *string)
2. **Builder**: Update all code that sets error_message
3. **Builder**: Commit fix to `claude/v2-builder` branch
4. **Validator**: Merge fix and redeploy
5. **Validator**: Run Test 3.2 to validate fix
6. **Validator**: Document validation results
7. **Validator**: Continue integration testing

---

## Additional Context

### Impact on Production

**Agent Downtime Scenarios** (ALL affected):
- Planned agent maintenance
- Agent pod restarts (k8s rollout)
- Network disruptions
- Agent crashes
- Kubernetes node failures

**Expected Behavior**: Commands queued, processed after reconnection
**Actual Behavior**: Commands queued, NEVER processed

**Risk**: High - Any agent downtime results in stuck commands

---

## Conclusion

**Bug Summary**: CommandDispatcher cannot scan pending commands with NULL error_message field

**Impact**: Command retry completely broken, affecting all agent failover scenarios

**Fix Complexity**: Low - Change ErrorMessage type from `string` to `*string`

**Testing**: Test 3.2 validates fix

**Priority**: P1 - HIGH (blocks critical functionality, has workaround)

---

**Generated**: 2025-11-22 06:18:00 UTC
**Validator**: Claude (v2-validator)
**Branch**: claude/v2-validator
**Status**: 🔴 ACTIVE - Awaiting Builder Fix
**Priority**: P1 - HIGH
**Blocks**: Integration Test 3.2, Agent Failover Reliability

