# Bug Report: P1-SCHEMA-002 - Missing updated_at Column in agent_commands Table

**Bug ID**: P1-SCHEMA-002
**Severity**: P1 - HIGH (Blocks accurate command status tracking)
**Component**: Database Schema (agent_commands table)
**Discovered During**: P1-COMMAND-SCAN-001 fix validation
**Status**: 🔴 ACTIVE
**Reporter**: Claude (v2-validator)
**Date**: 2025-11-22 07:09:00 UTC

---

## Executive Summary

The `agent_commands` table is missing the `updated_at` column that is referenced in the CommandDispatcher code. When the CommandDispatcher attempts to update command status (e.g., marking commands as "failed"), the update fails with a "column does not exist" error.

**Impact**: **MODERATE** - Does not block command processing, but prevents accurate command status tracking when commands fail.

---

## Symptoms

### Error Message

**API Logs**:
```
[CommandDispatcher] Failed to update command cmd-xxx status to failed: pq: column "updated_at" of relation "agent_commands" does not exist
```

**Frequency**: Every time CommandDispatcher tries to update a command to "failed" status

---

### Observed Behavior

**Scenario**: CommandDispatcher attempts to mark a command as "failed" when agent is not connected

**Timeline**:
```
07:09:21 [CommandDispatcher] Worker 5 processing command cmd-7ff211f7 for agent k8s-prod-cluster
07:09:21 [CommandDispatcher] Agent k8s-prod-cluster is not connected, marking command cmd-7ff211f7 as failed
07:09:21 [CommandDispatcher] Failed to update command cmd-7ff211f7 status to failed: pq: column "updated_at" of relation "agent_commands" does not exist
```

**Result**:
- ❌ Command status not updated in database
- ❌ Command remains in "pending" status
- ⚠️ Error logged but processing continues

---

## Root Cause Analysis

### Database Schema Issue

**Table**: `agent_commands`

**Current Schema** (Missing column):
```sql
CREATE TABLE agent_commands (
    command_id VARCHAR(255) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    action VARCHAR(50) NOT NULL,
    payload JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP,
    acknowledged_at TIMESTAMP,
    completed_at TIMESTAMP
);
-- Missing: updated_at TIMESTAMP
```

**Expected Schema** (With missing column):
```sql
CREATE TABLE agent_commands (
    command_id VARCHAR(255) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    action VARCHAR(50) NOT NULL,
    payload JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,  -- ← MISSING
    sent_at TIMESTAMP,
    acknowledged_at TIMESTAMP,
    completed_at TIMESTAMP
);
```

---

### Code Expectation

**File**: `api/internal/websocket/command_dispatcher.go` (or similar)

**Code** (Expects `updated_at` column):
```go
func (d *CommandDispatcher) markCommandFailed(commandID, errorMsg string) error {
    query := `
        UPDATE agent_commands
        SET status = 'failed',
            error_message = $1,
            updated_at = NOW()  -- ← Expects this column to exist
        WHERE command_id = $2
    `
    _, err := d.db.Exec(query, errorMsg, commandID)
    return err
}
```

**Error**: PostgreSQL returns `column "updated_at" of relation "agent_commands" does not exist`

---

## Evidence

### API Logs (During Test 3.2)

**Sample Errors** (37+ occurrences):
```
2025/11/22 07:09:21 [CommandDispatcher] Failed to update command cmd-7ff211f7 status to failed: pq: column "updated_at" of relation "agent_commands" does not exist
2025/11/22 07:09:21 [CommandDispatcher] Failed to update command cmd-fdd72a0f status to failed: pq: column "updated_at" of relation "agent_commands" does not exist
2025/11/22 07:09:21 [CommandDispatcher] Failed to update command cmd-6bbcdcae status to failed: pq: column "updated_at" of relation "agent_commands" does not exist
2025/11/22 07:09:21 [CommandDispatcher] Failed to update command cmd-512d3d3f status to failed: pq: column "updated_at" of relation "agent_commands" does not exist
...
```

**Total**: 37+ commands affected during testing

---

### Database Schema Verification

**Query**:
```bash
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "\d agent_commands"
```

**Result**:
```
                Table "public.agent_commands"
     Column      |            Type             | Nullable | Default
-----------------+-----------------------------+----------+---------
 command_id      | character varying(255)      | not null |
 agent_id        | character varying(255)      | not null |
 session_id      | character varying(255)      |          |
 action          | character varying(50)       | not null |
 payload         | jsonb                       |          |
 status          | character varying(50)       |          | 'pending'
 error_message   | text                        |          |
 created_at      | timestamp without time zone |          | CURRENT_TIMESTAMP
 sent_at         | timestamp without time zone |          |
 acknowledged_at | timestamp without time zone |          |
 completed_at    | timestamp without time zone |          |

-- Notice: updated_at column is MISSING
```

---

## Impact Assessment

### Severity: P1 - HIGH

**Why P1**:
- **Blocks accurate status tracking** - Failed commands not marked correctly
- **Affects audit logging** - Cannot track when commands were updated
- **Affects debugging** - Harder to diagnose command processing issues
- **High error volume** - 37+ errors during testing

**Why Not P0**:
- Does not block command processing (successful commands still work)
- Does not prevent session creation
- Does not cause data loss
- Has workaround (ignore failed status updates)

---

### Affected Functionality

**Working**:
- ✅ Command creation (INSERT does not use updated_at)
- ✅ Command queuing
- ✅ Successful command processing
- ✅ Command completion (when agent processes successfully)

**Broken**:
- ❌ Marking commands as "failed"
- ❌ Tracking command update timestamps
- ❌ Accurate command status after failures
- ❌ Audit trail for command state changes

---

### Observed Failure Scenarios

All scenarios where CommandDispatcher marks commands as "failed":

1. **Agent Not Connected** (Most common):
   - Command dispatched but agent not available
   - CommandDispatcher tries to mark as "failed"
   - Update fails silently
   - Command remains in "pending" status

2. **Command Timeout**:
   - Command sent but not acknowledged
   - Timeout handler tries to mark as "failed"
   - Update fails
   - Command remains in previous status

3. **Agent Error Response**:
   - Agent returns error during processing
   - CommandDispatcher tries to update status
   - Update may fail if using `updated_at`

---

## Recommended Fix

### Solution 1: Add updated_at Column (Recommended)

**Approach**: Add the missing `updated_at` column to the `agent_commands` table

**Migration SQL**:
```sql
-- Add updated_at column with default value
ALTER TABLE agent_commands
ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Backfill existing rows with created_at value
UPDATE agent_commands
SET updated_at = created_at
WHERE updated_at IS NULL;

-- Add trigger to auto-update on row changes
CREATE OR REPLACE FUNCTION update_agent_commands_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER agent_commands_updated_at_trigger
BEFORE UPDATE ON agent_commands
FOR EACH ROW
EXECUTE FUNCTION update_agent_commands_updated_at();
```

**Benefits**:
- ✅ Fixes the immediate error
- ✅ Enables accurate timestamp tracking
- ✅ Adds automatic update trigger
- ✅ Minimal code changes required
- ✅ Backward compatible (existing code continues working)

**Estimated Implementation Time**: 15 minutes

---

### Solution 2: Remove updated_at from Code (Alternative)

**Approach**: Remove all references to `updated_at` from CommandDispatcher code

**Code Changes**:
```go
// BEFORE:
func (d *CommandDispatcher) markCommandFailed(commandID, errorMsg string) error {
    query := `
        UPDATE agent_commands
        SET status = 'failed',
            error_message = $1,
            updated_at = NOW()  -- ← Remove this line
        WHERE command_id = $2
    `
    _, err := d.db.Exec(query, errorMsg, commandID)
    return err
}

// AFTER:
func (d *CommandDispatcher) markCommandFailed(commandID, errorMsg string) error {
    query := `
        UPDATE agent_commands
        SET status = 'failed',
            error_message = $1
        WHERE command_id = $2
    `
    _, err := d.db.Exec(query, errorMsg, commandID)
    return err
}
```

**Drawbacks**:
- ❌ Loses timestamp tracking capability
- ❌ Harder to audit when commands were updated
- ❌ Cannot distinguish between create and update times

**Recommendation**: **Do NOT use** - Keep timestamp tracking capability

---

### Solution 3: Use completed_at for All Updates (Workaround)

**Approach**: Use existing `completed_at` column for all status updates

**Code Changes**:
```go
func (d *CommandDispatcher) markCommandFailed(commandID, errorMsg string) error {
    query := `
        UPDATE agent_commands
        SET status = 'failed',
            error_message = $1,
            completed_at = NOW()  -- Use completed_at instead of updated_at
        WHERE command_id = $2
    `
    _, err := d.db.Exec(query, errorMsg, commandID)
    return err
}
```

**Drawbacks**:
- ❌ Semantically incorrect (failed ≠ completed)
- ❌ Confusing for developers
- ❌ Cannot distinguish between successful completion and failure

**Recommendation**: **Temporary workaround only**

---

## Reproduction Steps

### Prerequisites
- StreamSpace v2.0-beta deployed
- API with P1-COMMAND-SCAN-001 fix
- K8s agent running

### Steps

1. Stop the agent (simulate downtime):
   ```bash
   kubectl scale deployment/streamspace-k8s-agent -n streamspace --replicas=0
   ```

2. Create a session (will fail due to no agent):
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
   # Will return error: No agents available
   ```

3. Check API logs for the error:
   ```bash
   kubectl logs -n streamspace -l app.kubernetes.io/component=api | grep "updated_at"
   ```

**Expected Result**: Error logged:
```
[CommandDispatcher] Failed to update command cmd-xxx status to failed: pq: column "updated_at" of relation "agent_commands" does not exist
```

4. Check command status in database:
   ```bash
   kubectl exec -n streamspace streamspace-postgres-0 -- \
     psql -U streamspace -d streamspace \
     -c "SELECT command_id, status FROM agent_commands ORDER BY created_at DESC LIMIT 5;"
   ```

**Expected Result**: Commands remain in "pending" status (not "failed")

---

## Validation Testing

### After Fix Applied

**Test 1: Verify Column Exists**

```bash
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "\d agent_commands" | grep updated_at
```

**Expected**: Column listed with type TIMESTAMP

---

**Test 2: Verify Failed Status Updates Work**

```bash
# Stop agent
kubectl scale deployment/streamspace-k8s-agent -n streamspace --replicas=0

# Create command (will fail)
curl -X POST http://localhost:8000/api/v1/sessions ... (as above)

# Wait a few seconds
sleep 5

# Check API logs (should be no errors)
kubectl logs -n streamspace -l app.kubernetes.io/component=api --tail=50 | grep "updated_at"
```

**Expected**: No "column does not exist" errors

---

**Test 3: Verify Status Updates**

```bash
# Check command status in database
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "SELECT command_id, status, updated_at FROM agent_commands WHERE status = 'failed' ORDER BY created_at DESC LIMIT 5;"
```

**Expected**:
- Commands marked as "failed" ✅
- updated_at timestamp populated ✅

---

**Test 4: Verify Trigger Works**

```bash
# Manually update a command
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "UPDATE agent_commands SET status = 'completed' WHERE command_id = 'cmd-xxx';"

# Check updated_at changed
kubectl exec -n streamspace streamspace-postgres-0 -- \
  psql -U streamspace -d streamspace \
  -c "SELECT command_id, status, created_at, updated_at FROM agent_commands WHERE command_id = 'cmd-xxx';"
```

**Expected**: updated_at ≠ created_at (trigger updated it)

---

## Related Issues

### Discovered During
- P1-COMMAND-SCAN-001 fix validation

### Dependencies
- This bug BLOCKS accurate command status tracking
- This bug AFFECTS audit logging
- This bug AFFECTS debugging failed commands

### Related Bugs
- P1-COMMAND-SCAN-001 (AgentCommand NULL scan) - RESOLVED
- P1-MULTI-POD-001 (AgentHub not shared) - ACTIVE
- P1-AGENT-STATUS-001 (Agent status sync) - RESOLVED

---

## Workarounds

### Current Workaround: Ignore Failed Status Updates

**Approach**: Accept that failed commands remain in "pending" status

**Effectiveness**: ⚠️ **PARTIAL** - System continues functioning but loses status accuracy

**Limitations**:
- Cannot distinguish between truly pending vs failed commands
- Audit trail incomplete
- Debugging harder

**Temporary**: Until migration applied

---

## Priority Justification

### Why P1 (Not P0)

- **P0** bugs prevent deployment or cause complete system failure
- **P1** bugs block critical functionality but system remains partially functional

**This is P1 because**:
- ❌ Blocks accurate status tracking (important for operations)
- ❌ Blocks audit logging (important for compliance)
- ✅ Has workaround (ignore errors)
- ✅ System functional (successful commands work)
- ✅ Does not cause data loss

**Could be elevated to P0 if**:
- Compliance requirements mandate audit trail
- Status tracking becomes critical for operations
- No workaround existed

---

## Next Steps

1. **Builder**: Create database migration script
   - Add `updated_at` column to `agent_commands` table
   - Backfill existing rows
   - Add auto-update trigger

2. **Builder**: Add migration to deployment manifests
   - Include in Helm chart
   - Add to init container

3. **Builder**: Commit migration to `claude/v2-builder` branch

4. **Validator**: Merge migration and redeploy

5. **Validator**: Run validation tests (Test 1-4 above)

6. **Validator**: Document validation results

---

## Additional Context

### Impact on Production

**Affected Operations**:
- Command status auditing
- Failed command debugging
- Command lifecycle tracking
- Compliance reporting

**Expected Behavior**: All command status updates tracked with timestamps

**Actual Behavior**: Failed command updates fail silently, no timestamp tracking

**Risk**: **MEDIUM** - Affects operations and compliance, but not critical functionality

---

## Conclusion

**Bug Summary**: agent_commands table missing `updated_at` column expected by CommandDispatcher

**Impact**: Blocks accurate command status tracking and audit logging

**Fix Complexity**: Low - Simple database migration

**Testing**: 4 validation tests required

**Priority**: P1 - HIGH (affects operations and compliance)

**Recommended Solution**: Add updated_at column with auto-update trigger (Solution 1)

---

**Generated**: 2025-11-22 07:17:00 UTC
**Validator**: Claude (v2-validator)
**Branch**: claude/v2-validator
**Status**: 🔴 ACTIVE - Awaiting Builder Fix
**Priority**: P1 - HIGH
**Blocks**: Command Status Tracking, Audit Logging, Operations Debugging
