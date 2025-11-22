# Bug Report: Missing cluster_id Column in Database Schema

**Bug ID**: P1-SCHEMA-001 (Wave 14 Regression)
**Severity**: P1 (High - Still Blocks Integration Testing)
**Component**: API - Database Schema (agents & sessions tables)
**Status**: 🔴 **DISCOVERED - NEEDS BUILDER FIX**
**Discovered By**: Claude Code (Agent 3 - Validator)
**Date**: 2025-11-22
**Discovery Context**: P1 database fix validation testing

---

## Executive Summary

**NEW BLOCKER**: Session creation fails with missing database column error after P1 TEXT[] array fix was validated. The code is attempting to query a `cluster_id` column that doesn't exist in the database schema.

**Impact**: Integration testing still blocked (session creation fails)
**Root Cause**: Wave 14 code changes reference cluster_id column, but database migration wasn't applied
**Urgency**: High - blocks all v2.0-beta integration testing

---

## Bug Details

### Error Messages

**Primary Error** (Session Creation):
```json
{
  "error": "No agents available",
  "message": "No online agents are currently available: failed to get online agents: failed to query agents: pq: column \"cluster_id\" does not exist"
}
```

**Secondary Error** (Quota Check):
```
2025/11/22 03:03:24 Failed to get sessions for quota check: failed to list sessions for user admin: pq: column "cluster_id" does not exist
```

### When Does It Occur?

**Trigger**: Creating a session via POST /api/v1/sessions

**Flow**:
1. ✅ User authenticates (token obtained)
2. ✅ Template fetched from database (P1 fix working!)
3. ❌ **FAILS HERE**: Agent assignment query attempts to use cluster_id column
4. ❌ **ALSO FAILS**: User quota check attempts to use cluster_id column

### Affected Operations

**Agent Operations**:
- Querying online agents for session assignment
- Agent selection for new sessions
- Potentially agent registration/heartbeat

**Session Operations**:
- Listing sessions for user quota checks
- Creating new sessions
- Potentially session queries/filters

---

## Technical Analysis

### Missing Column: `cluster_id`

**Affected Tables** (suspected):
1. `agents` table - definitely missing cluster_id
2. `sessions` table - likely missing cluster_id (based on quota check error)

**Column Purpose** (inferred from context):
- Appears to be part of multi-cluster architecture
- Used to identify which cluster an agent belongs to
- Used to filter sessions by cluster

### Database Schema Investigation Needed

Builder needs to check:
1. What is the correct schema for `cluster_id`?
   - Data type? (likely TEXT or INTEGER)
   - Nullable? (likely NOT NULL with default)
   - Foreign key? (possibly references a clusters table)
2. Where should cluster_id be added?
   - `agents` table (confirmed)
   - `sessions` table (suspected)
   - Any other tables?
3. Was there a migration file that wasn't run?
4. Was this part of Wave 14 changes that needs a migration?

---

## Reproduction Steps

1. Deploy v2.0-beta API with P1 TEXT[] fix (commit 1aab1a5)
2. Ensure K8s agent is connected and online
3. Attempt to create a session:
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"83nXgy87RL2QBoApPHmJagsfKJ4jc467"}' | jq -r '.token')

   curl -s -X POST http://localhost:8000/api/v1/sessions \
     -H "Authorization: Bearer $TOKEN" \
     -H 'Content-Type: application/json' \
     -d '{"user":"admin","template":"firefox-browser","resources":{"memory":"1Gi","cpu":"500m"},"persistentHome":false}'
   ```
4. Observe error: `"pq: column \"cluster_id\" does not exist"`

**Reproducibility**: 100% - happens every time

---

## Environment

- **Platform**: Docker Desktop Kubernetes (macOS)
- **Namespace**: streamspace
- **Commit**: 1aab1a5 (includes P1 TEXT[] fix)
- **PostgreSQL**: Running in streamspace namespace
- **API Version**: local build (v2.0-beta)

---

## API Logs

```
2025/11/22 03:00:36 Found 0 templates in repository 1
2025/11/22 03:00:36 Successfully synced repository 1 with 0 templates and 19 plugins
2025/11/22 03:00:36 Cloning repository https://github.com/JoshuaAFerguson/streamspace-templates to /tmp/streamspace-repos/repo-2
2025/11/22 03:00:37 Found 195 templates in repository 2
2025/11/22 03:00:38 Updated catalog with 195 templates for repository 2
2025/11/22 03:00:38 Successfully synced repository 2 with 195 templates and 0 plugins
2025/11/22 03:03:24 Fetched template firefox-browser from database (ID: 6628)  ← ✅ P1 fix working
2025/11/22 03:03:24 Failed to get sessions for quota check: failed to list sessions for user admin: pq: column "cluster_id" does not exist  ← ❌ NEW ERROR
2025/11/22 03:03:24 No agents available for session admin-firefox-browser-8069cc63: failed to get online agents: failed to query agents: pq: column "cluster_id" does not exist  ← ❌ NEW ERROR
```

---

## Impact Assessment

### Blocking Operations
- ❌ Session creation (100% failure rate)
- ❌ Integration testing (cannot test VNC streaming)
- ❌ Agent assignment validation
- ❌ User quota checks

### Working Operations
- ✅ Authentication
- ✅ Template fetching (P1 fix validated!)
- ✅ Template repository sync
- ✅ API health checks
- ✅ Agent WebSocket connection (P0 fix validated!)

### Integration Testing Status
- **P0-AGENT-001**: ✅ VALIDATED (agent stability working)
- **P1-DATABASE-001**: ✅ VALIDATED (TEXT[] arrays working)
- **P1-SCHEMA-001**: ❌ **BLOCKING** (missing cluster_id column)

---

## Recommended Fix

### Option 1: Add Database Migration (Recommended)

Create migration to add cluster_id column to affected tables:

**For agents table**:
```sql
ALTER TABLE agents
ADD COLUMN cluster_id TEXT NOT NULL DEFAULT 'default-cluster';

-- Optional: Add index for performance
CREATE INDEX idx_agents_cluster_id ON agents(cluster_id);
```

**For sessions table** (if needed):
```sql
ALTER TABLE sessions
ADD COLUMN cluster_id TEXT;

-- Optional: Add foreign key if clusters table exists
-- ALTER TABLE sessions
-- ADD CONSTRAINT fk_sessions_cluster
-- FOREIGN KEY (cluster_id) REFERENCES clusters(id);
```

### Option 2: Remove cluster_id Usage (Not Recommended)

Remove cluster_id references from code if multi-cluster isn't ready for v2.0-beta. This would:
- Defer multi-cluster support to v2.1+
- Simplify v2.0-beta release
- But loses multi-cluster functionality

**Recommendation**: Use Option 1 - add the migration. Multi-cluster appears to be part of Wave 14's architecture.

---

## Testing Requirements

After Builder provides fix:

1. **Schema Validation**:
   - Verify cluster_id column exists in agents table
   - Verify cluster_id column exists in sessions table (if needed)
   - Verify column data types match code expectations

2. **Functional Testing**:
   - Create session successfully
   - Verify agent assignment works
   - Verify user quota checks work
   - Verify multi-agent scenarios (if applicable)

3. **Regression Testing**:
   - Ensure P1 TEXT[] fix still works
   - Ensure P0 agent WebSocket fix still works
   - Verify no new errors introduced

---

## Related Issues

**Fixed Issues** (not related):
- P0-AGENT-001: WebSocket concurrent write panic ✅ FIXED (commit 215e3e9)
- P1-DATABASE-001: TEXT[] array scanning error ✅ FIXED (commit 1249904)

**Potentially Related**:
- Wave 14 multi-agent architecture changes
- Multi-cluster support implementation
- Database schema versioning/migrations

---

## Timeline

- **2025-11-22 03:00**: P1 TEXT[] fix deployed and validated
- **2025-11-22 03:03**: First session creation test attempted
- **2025-11-22 03:03**: cluster_id error discovered in API logs
- **2025-11-22 03:04**: Bug report created for Builder

---

## Builder Action Items

1. **Immediate**:
   - Investigate cluster_id column requirements
   - Determine correct schema for affected tables
   - Create database migration script
   - Test migration in local environment

2. **Before Merge**:
   - Verify migration works with existing data
   - Test session creation end-to-end
   - Verify agent assignment logic
   - Document cluster_id purpose and usage

3. **Documentation**:
   - Update database schema docs
   - Document migration process
   - Add cluster_id to architecture docs

---

## Workaround

**None Available** - This is a schema-level issue that requires a code/migration fix. Cannot be worked around by configuration or deployment changes.

---

## Priority Justification

**P1 (High)** because:
- Blocks ALL integration testing
- Prevents session creation (core functionality)
- Affects v2.0-beta release timeline
- Multiple operations broken (agent assignment, quota checks)

Not P0 because:
- System doesn't crash
- API remains responsive
- Agent connections still work
- Can be fixed with database migration

---

**Reported By**: Claude Code (Agent 3 - Validator)
**Date**: 2025-11-22
**Branch**: claude/v2-validator
**Commit**: 1aab1a5
**Status**: Awaiting Builder fix

**Next Steps**: Builder to provide cluster_id schema migration for validation testing.
