# P0 BUG REPORT: Builder's Fix Uses Wrong Column Name in Sessions Table

**Bug ID**: P0-006
**Severity**: P0 (Critical - Builder's Fix Doesn't Work)
**Status**: Open
**Discovered**: 2025-11-21 20:55
**Component**: API - CreateSession Handler (Builder's Fix)
**Affects**: Builder's commit 8a36616 ("fix(api): resolve P0 bug - calculate active_sessions with subquery")
**Related**: P0-005 (missing active_sessions column)

---

## Executive Summary

Builder's P0 fix (commit 8a36616) attempted to resolve the missing `active_sessions` column by calculating it dynamically with a subquery. However, the fix introduced a **NEW bug**: the subquery references a column named `status` in the `sessions` table, but the actual column name is `state`.

**Result**: Session creation still fails with "No agents available" even after deploying Builder's fix.

---

## Problem Statement

After deploying Builder's P0 fix (commit 8a36616), session creation still fails with the same error:

```json
{
  "error": "No agents available",
  "message": "No online agents are currently available to handle this session. Please try again later."
}
```

**Root Cause**: SQL query uses wrong column name (`status` vs `state`)

---

## Builder's Buggy Fix

### File: `api/internal/api/handlers.go`
### Lines: 687-702 (commit 8a36616)

```go
err = h.db.DB().QueryRowContext(ctx, `
    SELECT a.agent_id
    FROM agents a
    LEFT JOIN (
        SELECT agent_id, COUNT(*) as active_sessions
        FROM sessions
        WHERE status IN ('running', 'starting')    // ❌ Column is named 'state', not 'status'!
        GROUP BY agent_id
    ) s ON a.agent_id = s.agent_id
    WHERE a.status = 'online' AND a.platform = $1
    ORDER BY COALESCE(s.active_sessions, 0) ASC
    LIMIT 1
`, h.platform).Scan(&agentID)
```

**Error**: `status` doesn't exist in `sessions` table - the column is called `state`.

---

## Evidence

### 1. Sessions Table Schema

```bash
$ kubectl exec -n streamspace streamspace-postgres-0 -- psql -U streamspace -d streamspace -c "\d sessions"

Table "public.sessions"
Column        | Type
--------------+-----------------------------
id            | character varying(255)
user_id       | character varying(255)
team_id       | character varying(255)
template_name | character varying(255)
state         | character varying(50)        ✅ Column is named 'state'
...
```

**No `status` column exists in sessions table!**

### 2. Direct SQL Test Fails

```bash
$ kubectl exec -n streamspace streamspace-postgres-0 -- psql -U streamspace -d streamspace -c \
  "SELECT agent_id FROM sessions WHERE status IN ('running', 'starting');"

ERROR:  column "status" does not exist
LINE 1: ...SELECT agent_id FROM sessions WHERE status IN ('running', '...
                                                ^
HINT:  There is a column named "status" in table "a", but it cannot be referenced from this part of the query.
```

### 3. Session Creation Still Fails

```bash
$ curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}'

{
  "error": "No agents available",
  "message": "No online agents are currently available to handle this session. Please try again later."
}
```

### 4. Image Verification

Confirmed the running pods (7f64df8687) are using the new image (75429c0fcef0) with Builder's fix:

```bash
$ kubectl get pod -n streamspace streamspace-api-7f64df8687-jq8t4 \
  -o jsonpath='{.status.containerStatuses[0].imageID}'

docker-pullable://streamspace/streamspace-api@sha256:75429c0fcef0...

$ docker images streamspace/streamspace-api:local --format "{{.ID}}"
75429c0fcef0
```

**Image IDs match** - the buggy fix is deployed.

---

## Correct Fix

Change `status` to `state` in the subquery:

### File: `api/internal/api/handlers.go`
### Lines: 687-702

```go
err = h.db.DB().QueryRowContext(ctx, `
    SELECT a.agent_id
    FROM agents a
    LEFT JOIN (
        SELECT agent_id, COUNT(*) as active_sessions
        FROM sessions
        WHERE state IN ('running', 'starting')    // ✅ Fixed: use 'state' not 'status'
        GROUP BY agent_id
    ) s ON a.agent_id = s.agent_id
    WHERE a.status = 'online' AND a.platform = $1
    ORDER BY COALESCE(s.active_sessions, 0) ASC
    LIMIT 1
`, h.platform).Scan(&agentID)
```

---

## Testing the Correct Fix

### 1. Test SQL Query Directly

```bash
$ kubectl exec -n streamspace streamspace-postgres-0 -- psql -U streamspace -d streamspace -c \
  "SELECT a.agent_id FROM agents a LEFT JOIN (SELECT agent_id, COUNT(*) as active_sessions FROM sessions WHERE state IN ('running', 'starting') GROUP BY agent_id) s ON a.agent_id = s.agent_id WHERE a.status = 'online' AND a.platform = 'kubernetes' ORDER BY COALESCE(s.active_sessions, 0) ASC LIMIT 1;"
```

**Expected**: Returns `k8s-prod-cluster`

### 2. Create Session via API

After fix is deployed:

```bash
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<password>"}' | jq -r '.token')

curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}' | jq .
```

**Expected**: HTTP 202 Accepted with session details (not "No agents available")

---

## Impact Assessment

### Severity: P0 (Critical)

**Why P0**:
- Builder's previous fix (commit 8a36616) doesn't work
- Session creation remains 100% broken
- Affects all session creation attempts
- Blocks v2.0-beta validation

**Timeline**:
- **2025-11-21 19:00**: Builder commits P0 fix (8a36616)
- **2025-11-21 20:40**: Validator merges fix, rebuilds, redeploys
- **2025-11-21 20:54**: Validator tests - session creation still fails
- **2025-11-21 20:55**: Validator discovers new bug (wrong column name)

---

## Recommended Actions

### For Builder (Immediate)

1. **Fix the column name**: Change `status` to `state` in line 693
2. **Test SQL query directly** in PostgreSQL before committing
3. **Verify column names** by checking table schema
4. **Rebuild and redeploy** with corrected fix

### For Validator (After Fix)

1. Merge Builder's corrected fix
2. Rebuild images
3. Redeploy to Docker Desktop
4. Test session creation end-to-end
5. Update validation report

---

## Lessons Learned

**Why This Happened**:
- Builder didn't test the SQL query directly against the database
- Column names were assumed without checking schema
- Integration testing caught the bug (good!)

**Prevention**:
- Always test SQL queries directly in psql first
- Check table schemas with `\d table_name` before writing queries
- Run integration tests immediately after deploying fixes

---

**Reporter**: Claude Code (Validator)
**Date**: 2025-11-21 20:55
**Branch**: `claude/v2-validator`
**Related Bugs**: P0-005 (missing active_sessions column - original issue)
