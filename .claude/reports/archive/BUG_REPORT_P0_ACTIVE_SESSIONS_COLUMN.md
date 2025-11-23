# P0 BUG REPORT: Session Creation Fails Due to Non-Existent Column

**Bug ID**: P0-005
**Severity**: P0 (Critical - Breaks Core Functionality)
**Status**: Open
**Discovered**: 2025-11-21
**Component**: API - CreateSession Handler
**Affects**: All session creation attempts via API
**Related**: Builder's commit 3284bdf ("fix(api): Implement v2.0-beta session creation architecture")

---

## Executive Summary

The CreateSession handler (api/internal/api/handlers.go:690-695) contains a SQL query that references a non-existent `active_sessions` column in the `agents` table. This causes the query to fail silently, returning no results, which triggers a "No agents available" error even when agents are online and connected.

**Impact**: Session creation is completely broken. No sessions can be created via the API.

---

## Problem Statement

When attempting to create a session via POST /api/v1/sessions, the request fails with:

```json
{
  "error": "No agents available",
  "message": "No online agents are currently available to handle this session. Please try again later."
}
```

This occurs even when:
1. Agents are online and connected via WebSocket
2. Agents are sending heartbeats successfully
3. Agents are marked as `status='online'` in the database
4. The CSRF protection is working correctly (JWT authentication succeeds)

---

## Root Cause

### Invalid SQL Query

**File**: `api/internal/api/handlers.go`
**Lines**: 690-695

```go
err = h.db.DB().QueryRowContext(ctx, `
    SELECT agent_id FROM agents
    WHERE status = 'online' AND platform = $1
    ORDER BY active_sessions ASC
    LIMIT 1
`, h.platform).Scan(&agentID)
```

The query attempts to `ORDER BY active_sessions ASC`, but the `agents` table has **no `active_sessions` column**.

### Agents Table Schema

```sql
Table "public.agents"
     Column     |            Type
----------------+-----------------------------
 id             | uuid
 agent_id       | character varying(255)
 platform       | character varying(50)
 region         | character varying(100)
 status         | character varying(50)
 capacity       | jsonb
 last_heartbeat | timestamp without time zone
 websocket_id   | character varying(255)
 metadata       | jsonb
 created_at     | timestamp without time zone
 updated_at     | timestamp without time zone
```

**Missing Column**: `active_sessions`

### Error Flow

1. User calls POST /api/v1/sessions with valid JWT token
2. API creates Session CRD successfully
3. API attempts to select an online agent with the invalid SQL query
4. PostgreSQL returns an error: "column active_sessions does not exist"
5. Go's `sql.QueryRowContext` returns `sql.ErrNoRows`
6. Handler treats this as "no agents available" (line 697-708)
7. API returns HTTP 503 Service Unavailable

---

## Evidence

### 1. Agent is Online in Database

```bash
$ kubectl exec -n streamspace streamspace-postgres-0 -- psql -U streamspace -d streamspace -c \
  "SELECT agent_id, status, platform, last_heartbeat FROM agents;"

     agent_id     | status |  platform  |       last_heartbeat
------------------+--------+------------+----------------------------
 k8s-prod-cluster | online | kubernetes | 2025-11-21 20:14:10.671964
```

### 2. Agent Connected via WebSocket

```bash
$ kubectl logs -n streamspace deploy/streamspace-api | grep k8s-prod-cluster | tail -5
2025/11/21 20:12:10 [AgentWebSocket] Agent k8s-prod-cluster connected (platform: kubernetes)
2025/11/21 20:12:10 [AgentHub] Registered agent: k8s-prod-cluster (platform: kubernetes), total connections: 1
2025/11/21 20:12:40 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/21 20:13:10 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
2025/11/21 20:13:40 [AgentWebSocket] Heartbeat from agent k8s-prod-cluster (status: online, activeSessions: 0)
```

### 3. Session Creation Request Fails

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

### 4. API Logs Show Error

```bash
$ kubectl logs -n streamspace deploy/streamspace-api | grep -i error | tail -2
2025/11/21 20:12:13 ERROR map[client_ip:127.0.0.1 duration:27.051216ms method:POST path:/api/v1/sessions status:503 user_id:admin]
2025/11/21 20:13:42 ERROR map[client_ip:127.0.0.1 duration:19.924227ms method:POST path:/api/v1/sessions status:503 user_id:admin]
```

### 5. Missing Column Confirmed

```bash
$ kubectl exec -n streamspace streamspace-postgres-0 -- psql -U streamspace -d streamspace -c \
  "SELECT column_name FROM information_schema.columns WHERE table_name = 'agents';"

 column_name
----------------
 id
 agent_id
 platform
 region
 status
 capacity
 last_heartbeat
 websocket_id
 metadata
 created_at
 updated_at
(11 rows)
```

**No `active_sessions` column exists.**

---

## Impact Assessment

### Severity: P0 (Critical)

**Why P0**:
- Session creation is a **core feature** - the primary purpose of the platform
- **100% failure rate** - no sessions can be created via API
- Affects all users attempting to create sessions
- Breaks the entire v2.0-beta workflow
- Discovered during integration testing after CSRF fix was applied

**Affected Use Cases**:
- ❌ All session creation attempts via POST /api/v1/sessions
- ❌ Web UI session creation (depends on API)
- ❌ CLI/script-based session creation
- ❌ Integration tests
- ❌ Production usage

**Not Affected**:
- ✅ Agent registration and connectivity
- ✅ Authentication and authorization
- ✅ Session CRD creation (succeeds before query fails)
- ✅ Template management
- ✅ Other API endpoints

---

## Recommended Solution

### Option 1: Calculate Active Sessions with Subquery (Recommended)

Modify the query to calculate active sessions from the `sessions` table:

```go
err = h.db.DB().QueryRowContext(ctx, `
    SELECT a.agent_id
    FROM agents a
    LEFT JOIN (
        SELECT agent_id, COUNT(*) as active_sessions
        FROM sessions
        WHERE status IN ('running', 'starting')
        GROUP BY agent_id
    ) s ON a.agent_id = s.agent_id
    WHERE a.status = 'online' AND a.platform = $1
    ORDER BY COALESCE(s.active_sessions, 0) ASC
    LIMIT 1
`, h.platform).Scan(&agentID)
```

**Pros**:
- No schema changes required
- Dynamically calculates active sessions
- Accurate load balancing

**Cons**:
- Slightly more complex query
- Requires JOIN on every session creation

### Option 2: Add active_sessions Column (Alternative)

Add an `active_sessions` column to the `agents` table and update it when sessions start/stop:

```sql
ALTER TABLE agents ADD COLUMN active_sessions INTEGER DEFAULT 0;
```

Then update the column when:
- Agent provisions a pod (increment)
- Session terminates (decrement)
- Agent heartbeat (sync from actual pod count)

**Pros**:
- Simple query (keeps existing code)
- Fast lookup (no JOIN)

**Cons**:
- Requires migration
- Requires additional update logic
- Risk of desync if updates fail

### Option 3: Remove ORDER BY Clause (Quick Fix)

Remove the `ORDER BY` clause entirely for now:

```go
err = h.db.DB().QueryRowContext(ctx, `
    SELECT agent_id FROM agents
    WHERE status = 'online' AND platform = $1
    LIMIT 1
`, h.platform).Scan(&agentID)
```

**Pros**:
- Immediate fix
- Unblocks testing

**Cons**:
- No load balancing (random agent selection)
- Not a proper solution

---

## Testing Plan

Once fixed:

### 1. Verify Query Succeeds

```bash
# Test the fixed query directly in PostgreSQL
kubectl exec -n streamspace streamspace-postgres-0 -- psql -U streamspace -d streamspace -c \
  "SELECT agent_id FROM agents WHERE status = 'online' AND platform = 'kubernetes' LIMIT 1;"
```

**Expected**: Returns `k8s-prod-cluster`

### 2. Create Session via API

```bash
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<password>"}' | jq -r '.token')

curl -s -X POST http://localhost:8000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}' | jq .
```

**Expected**: HTTP 202 Accepted with session details:
```json
{
  "name": "admin-firefox-browser-<uuid>",
  "namespace": "streamspace",
  "user": "admin",
  "template": "firefox-browser",
  "state": "pending",
  "status": {
    "phase": "Pending",
    "message": "Session provisioning in progress (agent: k8s-prod-cluster, command: cmd-<uuid>)"
  }
}
```

### 3. Verify Agent Receives Command

```bash
kubectl logs -n streamspace deploy/streamspace-api | grep "Selected agent"
```

**Expected**: Log shows agent selection succeeded.

### 4. Verify Pod is Provisioned

```bash
kubectl get pods -n streamspace | grep admin-firefox
```

**Expected**: Pod exists and is Running or ContainerCreating.

---

## Related Bugs

- **P2-004**: CSRF Protection (FIXED by commit a9238a3)
- **P0-003**: Missing Controller (INVALID - controller intentionally removed)
- **P0-001**: K8s Agent Crash (FIXED by commit 22a39d8)
- **P1-002**: Admin Authentication (FIXED by commit 6c22c96)

---

## Timeline

- **2025-11-21 17:00**: Builder commits session creation fix (3284bdf)
- **2025-11-21 18:00**: Validator reviews code (looked correct)
- **2025-11-21 19:00**: Validator discovers P2 CSRF bug (blocks testing)
- **2025-11-21 20:00**: Builder commits CSRF fix (a9238a3)
- **2025-11-21 20:13**: Validator tests session creation, discovers P0 bug
- **2025-11-21 20:15**: Validator confirms `active_sessions` column missing

---

## Recommendation

**Priority**: P0 (Critical - Fix Immediately)

**Recommended Solution**: Option 1 (subquery)

**Estimated Fix Time**: 30 minutes

**Impact After Fix**: Session creation via API will work end-to-end

---

**Reporter**: Claude Code (Validator)
**Date**: 2025-11-21
**Branch**: `claude/v2-validator`
