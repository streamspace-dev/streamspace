# P1 BUG REPORT: Command Payload Not Marshaled to JSON

**Bug ID**: P1-CMD-002
**Severity**: P1 (High - Session termination still broken)
**Status**: ❌ **DISCOVERED** during P1 fix validation
**Discovered**: 2025-11-21 22:51
**Component**: API - Agent Command Creation
**Affects**: Session termination (DeleteSession handler)
**Related**: P1-TERM-001 (follow-up bug discovered after partial fix)

---

## Executive Summary

Builder's P1 fixes for NULL handling and agent_id tracking are working correctly ✅, but session termination still fails due to a **different bug**: the command payload/parameters are being passed to SQL as a Go `map[string]interface{}` instead of being marshaled to JSON first.

**Previous P1 Issues (FIXED ✅)**:
1. NULL handling - FIXED with `sql.NullString`
2. Wrong column name (controller_id vs agent_id) - FIXED
3. Missing agent_id tracking - FIXED

**New P1 Issue (NEW BUG ❌)**:
4. Command payload not marshaled to JSON before database insertion

**Impact**: Session termination still completely broken - all DELETE requests fail with HTTP 500.

---

## Problem Statement

When testing the P1 fixes, the DELETE endpoint returns:

```json
{
  "error": "Failed to create stop command",
  "message": "Failed to create command in database: sql: converting argument $5 type: unsupported type map[string]interface {}, a map"
}
```

**HTTP Status**: 500 Internal Server Error

---

## Root Cause Analysis

### Good News: P1 Fixes Working ✅

**Database Query Before Termination**:
```sql
               id               |     agent_id     |  state
--------------------------------+------------------+---------
 admin-firefox-browser-52bfac7e | k8s-prod-cluster | pending
```

- ✅ agent_id is populated (was NULL before P1 fix)
- ✅ DeleteSession successfully queried the session
- ✅ No NULL scan errors (sql.NullString fix working)

**API Logs**:
No errors related to NULL handling or agent_id queries - those fixes are working!

### New Issue: JSON Marshaling Missing ❌

**Error Details**:
```
sql: converting argument $5 type: unsupported type map[string]interface {}, a map
```

This error occurs when:
1. DeleteSession creates a stop_session command
2. Command has a `payload` or `parameters` field containing Go map data
3. Code tries to INSERT the command into `agent_commands` table
4. SQL driver rejects the Go map because it expects JSON/JSONB or string type

**Expected**: Command payload should be marshaled to JSON before database insertion
**Actual**: Command payload is passed as raw Go `map[string]interface{}`

---

## Evidence

### 1. Test Results

**Test Date**: 2025-11-21 22:51

**Session Creation**: ✅ PASSED
```json
{
  "name": "admin-firefox-browser-52bfac7e",
  "state": "pending",
  "status": {
    "message": "Session provisioning in progress (agent: k8s-prod-cluster, command: cmd-859b4687)"
  }
}
```

**Database Verification**: ✅ PASSED
```sql
SELECT id, agent_id, state FROM sessions WHERE id = 'admin-firefox-browser-52bfac7e';

               id               |     agent_id     |  state
--------------------------------+------------------+---------
 admin-firefox-browser-52bfac7e | k8s-prod-cluster | pending
```

**Session Termination**: ❌ FAILED
```json
{
  "error": "Failed to create stop command",
  "message": "Failed to create command in database: sql: converting argument $5 type: unsupported type map[string]interface {}, a map"
}
```

### 2. Database Schema

The `agent_commands` table likely has:

```sql
CREATE TABLE agent_commands (
    command_id VARCHAR(255) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    action VARCHAR(50) NOT NULL,
    payload JSONB,  -- ⬅️ Expects JSON, not Go map
    status VARCHAR(50),
    error_message TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

**Key Point**: The `payload` column is likely JSONB or JSON type, which requires the data to be marshaled before insertion.

---

## Expected vs Actual Behavior

### Expected Flow (What Should Happen)

```go
// In DeleteSession handler
command := &models.AgentCommand{
    CommandID: fmt.Sprintf("cmd-%s", uuid.New().String()[:8]),
    AgentID:   agentID.String,
    SessionID: sessionID,
    Action:    "stop_session",
    Payload:   map[string]interface{}{  // Go map
        "session_id": sessionID,
        "namespace":  "streamspace",
    },
    Status:    "pending",
    CreatedAt: time.Now(),
}

// In CreateCommand function
payloadJSON, err := json.Marshal(command.Payload)  // ✅ Marshal to JSON
if err != nil {
    return fmt.Errorf("failed to marshal payload: %w", err)
}

_, err = db.ExecContext(ctx, `
    INSERT INTO agent_commands (
        command_id, agent_id, session_id, action, payload, status, created_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7)
`, command.CommandID, command.AgentID, command.SessionID, command.Action,
   payloadJSON,  // ✅ Pass JSON bytes, not Go map
   command.Status, command.CreatedAt)
```

### Actual Flow (What's Happening)

```go
// In DeleteSession handler
command := &models.AgentCommand{
    // ... same as above ...
    Payload: map[string]interface{}{
        "session_id": sessionID,
        "namespace":  "streamspace",
    },
}

// In CreateCommand function (MISSING JSON MARSHALING)
_, err = db.ExecContext(ctx, `
    INSERT INTO agent_commands (
        command_id, agent_id, session_id, action, payload, status, created_at
    ) VALUES ($1, $2, $3, $4, $5, $6, $7)
`, command.CommandID, command.AgentID, command.SessionID, command.Action,
   command.Payload,  // ❌ Passing Go map directly - SQL driver rejects this!
   command.Status, command.CreatedAt)
```

---

## Correct Implementation

### Option 1: Marshal in CreateCommand (Recommended)

**File**: `api/internal/db/commands.go` or similar

```go
func (s *Store) CreateCommand(ctx context.Context, command *models.AgentCommand) error {
    // Marshal payload to JSON if not already marshaled
    var payloadJSON []byte
    var err error

    if command.Payload != nil {
        payloadJSON, err = json.Marshal(command.Payload)
        if err != nil {
            return fmt.Errorf("failed to marshal command payload: %w", err)
        }
    }

    _, err = s.db.ExecContext(ctx, `
        INSERT INTO agent_commands (
            command_id, agent_id, session_id, action, payload,
            status, error_message, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `,
        command.CommandID,
        command.AgentID,
        nullString(command.SessionID),
        command.Action,
        payloadJSON,  // ✅ JSON bytes
        command.Status,
        nullString(command.ErrorMessage),
        command.CreatedAt,
        command.UpdatedAt,
    )

    return err
}
```

### Option 2: Use json.RawMessage in Model

**File**: `api/internal/models/command.go` or similar

```go
type AgentCommand struct {
    CommandID    string          `json:"command_id"`
    AgentID      string          `json:"agent_id"`
    SessionID    string          `json:"session_id,omitempty"`
    Action       string          `json:"action"`
    Payload      json.RawMessage `json:"payload,omitempty"`  // ✅ Already JSON
    Status       string          `json:"status"`
    ErrorMessage string          `json:"error_message,omitempty"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
}
```

Then when creating the command:

```go
// In DeleteSession handler
payloadJSON, _ := json.Marshal(map[string]interface{}{
    "session_id": sessionID,
    "namespace":  "streamspace",
})

command := &models.AgentCommand{
    CommandID: fmt.Sprintf("cmd-%s", uuid.New().String()[:8]),
    AgentID:   agentID.String,
    SessionID: sessionID,
    Action:    "stop_session",
    Payload:   payloadJSON,  // ✅ Already JSON
    Status:    "pending",
    CreatedAt: time.Now(),
}
```

---

## Testing Plan

### 1. Apply Fix

Builder should:
1. Add JSON marshaling to CreateCommand function
2. Or change Payload field type to json.RawMessage and marshal before creating command
3. Test with actual database insertion

### 2. Verify Command Creation

```bash
# Create session
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

SESSION=$(curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}' | jq -r '.name')

# Wait for running state
sleep 15

# Terminate session
curl -X DELETE "http://localhost:8000/api/v1/sessions/$SESSION" \
  -H "Authorization: Bearer $TOKEN" -v

# Expected: HTTP 202 with commandId

# Verify command in database
kubectl exec streamspace-postgres-0 -n streamspace -- psql -U streamspace -d streamspace \
  -c "SELECT command_id, agent_id, action, payload::text FROM agent_commands WHERE session_id = '$SESSION';"

# Expected:
#  command_id  |     agent_id     |    action     |                    payload
# -------------+------------------+---------------+-----------------------------------------------
#  cmd-abc123  | k8s-prod-cluster | stop_session  | {"session_id":"...","namespace":"streamspace"}
```

### 3. Test End-to-End Termination

```bash
# After fix applied:
1. Create session - should succeed ✅
2. Verify agent_id populated - should succeed ✅
3. DELETE session - should return HTTP 202 with commandId ✅
4. Verify agent receives stop_session command via WebSocket ✅
5. Verify pod and service are deleted ✅
6. Verify session CRD state updated ✅
```

---

## Impact Assessment

### Severity: P1 (High)

**Why P1**:
- Session termination still completely broken
- Blocks all P1 validation testing
- Prevents resource cleanup
- Same priority as previous P1 issues

**Partial Progress**:
- ✅ P1 NULL handling fix working
- ✅ P1 agent_id tracking fix working
- ❌ Session termination still broken (different reason)

**Full Fix Required**:
- This must be fixed before v2.0-beta can be released
- Without working termination, resources accumulate indefinitely

---

## Lessons Learned

### For Builder

1. **JSON Marshaling**: Always marshal Go maps/structs to JSON before SQL insertion
2. **Database Types**: Check column types (JSONB vs TEXT vs VARCHAR)
3. **Test Full Flow**: Test actual database insertion, not just SQL syntax
4. **Type Safety**: Consider using `json.RawMessage` for JSON columns to make intent clear

### For Validator

1. **Incremental Testing**: P1 fixes revealed next bug - good incremental approach
2. **Database Verification**: Checking database state confirmed P1 fixes working
3. **Error Message Analysis**: Clear error messages helped identify root cause quickly

---

## Status Summary

### P1 Issues Status

| Issue | Description | Status | Fix Commit |
|-------|-------------|--------|------------|
| P1-TERM-001a | NULL handling in DeleteSession | ✅ FIXED | 70c90e0 |
| P1-TERM-001b | Wrong column (controller_id vs agent_id) | ✅ FIXED | 70c90e0 |
| P1-TERM-001c | Missing agent_id tracking in CreateSession | ✅ FIXED | 70c90e0 |
| **P1-CMD-002** | **Command payload JSON marshaling** | ❌ **NEW BUG** | - |

### Recommended Action

Builder should fix the JSON marshaling issue and push updated commit. Validator will then re-test complete session lifecycle.

---

**Validator**: Claude Code
**Date**: 2025-11-21 22:51
**Branch**: `claude/v2-validator`
**Builder Commit Tested**: 70c90e0 (partial success)
**Status**: Testing blocked - new bug prevents validation

---

## Additional Notes

**Good Progress Made**:
- Agent connection is stable (no repeated disconnects)
- Session creation working smoothly
- Database agent_id tracking functional
- P1 fixes addressing their specific issues correctly

**Remaining Work**:
- Fix command payload JSON marshaling
- Complete session termination testing
- Verify agent receives and processes stop_session command
- Verify resource cleanup (pod, service, CRD)
