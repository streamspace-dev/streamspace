# P1 BUG REPORT: Session Termination Fix Incomplete - Multiple Issues

**Bug ID**: P1-TERM-001
**Severity**: P1 (High - Core functionality incomplete)
**Status**: ❌ **DISCOVERED** during testing
**Discovered**: 2025-11-21 22:30
**Component**: API - DeleteSession Handler
**Affects**: Session termination (commit ff5cd46)
**Related**: P0-007 (NULL handling), EXPANDED_TESTING_REPORT.md

---

## Executive Summary

Builder's session termination fix (commit ff5cd46) has **three critical issues** that prevent it from working:

1. **NULL Handling Bug**: Same issue as P0-007 - tries to scan NULL `controller_id` into `string` type
2. **Wrong Column Name**: Queries `controller_id` (legacy) instead of `agent_id` (v2.0-beta)
3. **Missing NULL Check**: Doesn't use `sql.NullString` or `COALESCE` for nullable column

**Impact**: Session termination completely broken - all DELETE requests fail with HTTP 500.

---

## Problem Statement

When testing the session termination fix, the DELETE endpoint returns:

```json
{
  "error": "Failed to query session",
  "message": "Database error: sql: Scan error on column index 0, name \"controller_id\": converting NULL to string is unsupported"
}
```

**HTTP Status**: 500 Internal Server Error

---

## Root Cause Analysis

### Issue 1: NULL Handling (Same as P0-007)

Builder's code tries to scan a nullable column into a `string` type:

```go
// ❌ WRONG: controller_id can be NULL
var controllerID string
var currentState string
err := h.db.DB().QueryRowContext(ctx, `
    SELECT controller_id, state FROM sessions WHERE id = $1
`, sessionID).Scan(&controllerID, &currentState)
```

When `controller_id` is NULL, this causes a scan error.

### Issue 2: Wrong Column Name

The sessions table has **two** columns:
- `controller_id` (legacy v1.x, can be NULL)
- `agent_id` (v2.0-beta, can be NULL, has foreign key to agents table)

Builder's fix queries `controller_id` but v2.0-beta uses `agent_id` for agent assignment.

### Issue 3: All Sessions Have NULL Values

```sql
streamspace=# SELECT id, agent_id, controller_id, state FROM sessions LIMIT 5;
               id                | agent_id | controller_id |  state
---------------------------------+----------+---------------+---------
 admin-firefox-browser-7e367bc3  |          |               | pending
 admin-firefox-browser-0b02f38b  |          |               | running
 admin-firefox-browser-35a9a603  |          |               | running
```

**ALL sessions** have NULL `agent_id` AND NULL `controller_id`. This means:
- Sessions table schema has both columns
- Neither column is being populated during session creation
- The termination fix will fail for ALL sessions

---

## Evidence

### 1. API Logs

```
2025/11/21 22:31:02 Failed to query session: sql: Scan error on column index 0, name "controller_id": converting NULL to string is unsupported
2025/11/21 22:31:02 ERROR map[... method:DELETE path:/api/v1/sessions/admin-firefox-browser-7e367bc3 ... status:500 ...]
```

### 2. Database Schema

```sql
\d sessions

Column            | Type                        | Nullable
------------------+-----------------------------+----------
controller_id     | character varying(255)      | YES
agent_id          | character varying(255)      | YES

Foreign-key constraints:
    "sessions_agent_id_fkey" FOREIGN KEY (agent_id) REFERENCES agents(agent_id)
```

### 3. Agent Status

```sql
SELECT agent_id, status FROM agents WHERE platform = 'kubernetes';

    agent_id      | status
------------------+--------
 k8s-prod-cluster | online
```

Agent is online and healthy - the issue is purely in the DeleteSession handler.

---

## Correct Implementation

Builder needs to fix all three issues:

### Option 1: Use agent_id with sql.NullString (Recommended)

```go
// ✅ CORRECT: Use agent_id (v2.0-beta) and handle NULL
var agentID sql.NullString
var currentState string
err := h.db.DB().QueryRowContext(ctx, `
    SELECT agent_id, state FROM sessions WHERE id = $1
`, sessionID).Scan(&agentID, &currentState)

if err == sql.ErrNoRows {
    c.JSON(http.StatusNotFound, gin.H{
        "error":   "Session not found",
        "message": "The specified session does not exist",
    })
    return
}

if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{
        "error": fmt.Sprintf("Failed to query session: %v", err),
    })
    return
}

// Check if session has an agent assigned
if !agentID.Valid || agentID.String == "" {
    c.JSON(http.StatusConflict, gin.H{
        "error":   "Session not fully started",
        "message": "Session has no agent assigned - cannot terminate",
    })
    return
}

// Use agentID.String for the rest of the logic
```

### Option 2: Use COALESCE (Quick Fix)

```go
// ✅ CORRECT: Use COALESCE to handle NULL
var agentID string
var currentState string
err := h.db.DB().QueryRowContext(ctx, `
    SELECT COALESCE(agent_id, '') as agent_id, state
    FROM sessions
    WHERE id = $1
`, sessionID).Scan(&agentID, &currentState)

// Then check if agentID is empty
if agentID == "" {
    c.JSON(http.StatusConflict, gin.H{
        "error":   "Session not fully started",
        "message": "Session has no agent assigned - cannot terminate",
    })
    return
}
```

---

## Additional Issues Discovered

### Agent Connection Instability

After API restart, agent repeatedly disconnects/reconnects:

```
[AgentHub] Detected stale connection for agent k8s-prod-cluster (no heartbeat for >30s)
[AgentHub] Unregistered agent: k8s-prod-cluster, remaining connections: 0
[AgentHub] Registered agent: k8s-prod-cluster (platform: kubernetes), total connections: 1
```

This causes intermittent "No agents available" errors during session creation.

### Sessions Not Populating agent_id

Even successful session creations from P0-007 testing left `agent_id` NULL. This suggests:
- Session creation doesn't update the sessions table with agent assignment
- Or the UPDATE query is failing silently
- Or we're relying on the CRD as source of truth (not the database)

**Question for Builder**: Should the database sessions table track agent assignments, or is the CRD the source of truth?

---

## Testing Plan

### 1. Apply Fixes

Builder should:
1. Change `controller_id` to `agent_id` in the query
2. Use `sql.NullString` for `agent_id`
3. Add validation for NULL/empty `agent_id`
4. Verify session creation populates `agent_id` in database

### 2. Test Session Termination

```bash
# Create a session
curl -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"}}'

# Verify agent_id is set in database
kubectl exec streamspace-postgres-0 -- psql -U streamspace -d streamspace \
  -c "SELECT id, agent_id, state FROM sessions WHERE id = '<session-id>';"

# Terminate session
curl -X DELETE "http://localhost:8000/api/v1/sessions/<session-id>" \
  -H "Authorization: Bearer $TOKEN"

# Expected response
{
  "name": "<session-id>",
  "commandId": "cmd-<uuid>",
  "message": "Session termination requested, agent will delete resources"
}

# Verify agent receives stop_session command
kubectl logs deploy/streamspace-k8s-agent --tail=20 | grep "stop_session"

# Verify pod is deleted
kubectl get pods -n streamspace | grep "<session-id>"  # Should not exist
```

### 3. Verify End-to-End

- [ ] Session creation populates agent_id
- [ ] DELETE returns HTTP 202 with commandId
- [ ] Agent receives stop_session command
- [ ] Agent deletes Deployment and Service
- [ ] Session CRD state updated to "terminated"
- [ ] Database session state updated

---

## Impact Assessment

### Severity: P1 (High)

**Why P1**:
- Session termination completely broken
- Affects all users
- Blocks cleanup of session resources
- Resource leaks (pods, services remain allocated)
- Same class of error as P0-007 (NULL handling)

**Partial Mitigation**:
- Sessions can be manually deleted via kubectl
- Sessions eventually hibernate after idle timeout (if configured)

**Full Fix Required**:
- This needs to be fixed before v2.0-beta can be released
- Without working termination, resources accumulate indefinitely

---

## Lessons Learned

### For Builder

1. **Test NULL scenarios**: Always test with NULL database values
2. **Check table schema**: Verify column names in actual database before coding
3. **Use sql.NullString**: For ANY nullable column - no exceptions
4. **Test end-to-end**: Don't just test that code compiles - actually run DELETE requests

### For Architecture

1. **Source of truth clarity**: Is the CRD or database the source of truth for agent assignment?
2. **Column naming consistency**: Should we deprecate `controller_id` in favor of `agent_id`?
3. **Database population**: Session creation should populate `agent_id` in database for API queries

---

## Recommended Actions

### Immediate (Builder)

1. Fix the three issues in DeleteSession handler:
   - Change to `agent_id`
   - Use `sql.NullString`
   - Add NULL validation
2. Test with actual database NULL values
3. Verify session creation populates `agent_id`

### Short-term (Builder)

1. Review all handlers for similar NULL handling issues
2. Add integration tests for DELETE endpoint
3. Document agent assignment flow (CRD vs database)

### Medium-term (Architect)

1. Decide: Should we remove `controller_id` column entirely?
2. Ensure database is source of truth OR document CRD-first architecture
3. Add database constraints to prevent NULL agent_id for "running" sessions

---

**Validator**: Claude Code
**Date**: 2025-11-21 22:33
**Branch**: `claude/v2-validator`
**Builder Commit Tested**: ff5cd46
**Status**: Testing blocked - multiple bugs prevent validation

