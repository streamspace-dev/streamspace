# Kubernetes Removal Testing Plan - v2.0-beta Architecture

**Created**: 2025-11-21
**Assigned To**: Validator (Agent 3)
**Priority**: P0 - CRITICAL
**Status**: PENDING - Ready for execution

---

## Executive Summary

Builder has completed a **major architectural refactoring** to fully decouple the API from Kubernetes, implementing pure v2.0-beta architecture where:

- **API**: Database-only operations (no Kubernetes client)
- **Agents**: All Kubernetes/Docker operations
- **Communication**: WebSocket commands from API to agents

**Scope of Changes**: 15 files, 1,925 insertions, 525 deletions
**Impact**: ALL session lifecycle operations affected
**Risk Level**: HIGH - Core functionality completely refactored

---

## Changes Summary

### 1. Kubernetes Code Removal from API (13 commits)

**Key Changes**:
- ✅ Removed K8s client calls from CreateSession
- ✅ Removed K8s fallback from ListSessions and GetSession
- ✅ Removed Session CRD creation from API
- ✅ Removed Template CRD fetching from API
- ✅ Quota enforcement now uses database instead of K8s API
- ✅ Implemented hibernate and wake session endpoints (database-only)

**Files Modified**:
- `api/internal/api/handlers.go`: 950 lines changed
- `api/internal/api/stubs.go`: 185 lines added
- `api/cmd/main.go`: K8s client now optional

### 2. New Agent Selection Service

**New File**: `api/internal/services/agent_selector.go` (313 lines)

**Features**:
- Multi-agent load balancing
- Cluster affinity routing
- Region preference
- Capacity-based selection
- Health filtering (online agents only)
- WebSocket connection verification

**Selection Criteria**:
- ClusterID (optional)
- Region (optional)
- Platform (kubernetes, docker, etc.)
- PreferLowLoad (default: true)
- RequireConnected (default: true)

### 3. Database Template Layer

**New File**: `api/internal/db/templates.go` (230 lines)

**Purpose**: Templates now managed in database instead of querying Kubernetes

**Features**:
- CreateTemplate, GetTemplate, ListTemplates
- UpdateTemplate, DeleteTemplate
- Template categories and tags
- Default resource specifications

### 4. Database Migrations (3 new migrations)

**Migration 001**: Add tags to sessions
- `tags` JSONB column for session metadata
- Index on tags for filtering

**Migration 002**: Add agent and cluster tracking
- `agent_id` VARCHAR(255) - which agent owns session
- `cluster_id` VARCHAR(255) - which cluster session runs on
- Foreign key to agents table
- Indexes for efficient queries

**Migration 003**: Add cluster fields to agents
- `cluster_id` VARCHAR(255) - cluster identifier
- `cluster_name` VARCHAR(255) - human-readable name
- `region` VARCHAR(100) - geographic region

### 5. Agent Enhancements

**Files Modified**:
- `agents/k8s-agent/agent_handlers.go` (74 lines changed)
- `agents/k8s-agent/agent_k8s_operations.go` (429 lines added)
- `agents/k8s-agent/main.go` (23 lines changed)

**New Agent Responsibilities**:
- Fetch Template CRDs from Kubernetes
- Create Session CRDs after pod becomes ready
- Use templateManifest from command payload
- Handle ALL Kubernetes operations (API does none)

### 6. Session Lifecycle Completeness

**New Endpoints**:
- `PUT /api/v1/sessions/:id/hibernate` - Scale to 0 replicas
- `PUT /api/v1/sessions/:id/wake` - Scale to 1 replica

**Complete Lifecycle**:
- ✅ Create (start_session command)
- ✅ Terminate (stop_session command)
- ✅ Hibernate (hibernate_session command)
- ✅ Wake (wake_session command)

---

## Testing Strategy

### Phase 1: Database Migration Testing (P0)

**Objective**: Verify database schema changes are applied correctly

**Test Cases**:

1. **Migration 001 - Tags**:
   ```sql
   -- Verify tags column exists
   SELECT column_name, data_type FROM information_schema.columns
   WHERE table_name = 'sessions' AND column_name = 'tags';

   -- Verify index exists
   SELECT indexname FROM pg_indexes
   WHERE tablename = 'sessions' AND indexname = 'idx_sessions_tags';
   ```

2. **Migration 002 - Agent Tracking**:
   ```sql
   -- Verify agent_id and cluster_id columns
   SELECT column_name FROM information_schema.columns
   WHERE table_name = 'sessions'
   AND column_name IN ('agent_id', 'cluster_id');

   -- Verify foreign key constraint
   SELECT constraint_name FROM information_schema.table_constraints
   WHERE table_name = 'sessions'
   AND constraint_name = 'fk_sessions_agent_id';
   ```

3. **Migration 003 - Cluster Fields**:
   ```sql
   -- Verify cluster fields in agents table
   SELECT column_name FROM information_schema.columns
   WHERE table_name = 'agents'
   AND column_name IN ('cluster_id', 'cluster_name', 'region');
   ```

**Acceptance Criteria**:
- [ ] All migrations apply without errors
- [ ] All columns exist with correct data types
- [ ] All indexes created successfully
- [ ] Foreign key constraints working
- [ ] Rollback migrations work correctly

---

### Phase 2: Session Creation Testing (P0)

**Objective**: Verify session creation works without API accessing Kubernetes

**Prerequisites**:
- K8s agent running and connected
- Database migrations applied
- At least one template in database

**Test Cases**:

1. **Basic Session Creation**:
   ```bash
   POST /api/v1/sessions
   {
     "user": "admin",
     "template": "firefox-browser",
     "resources": {"memory": "1Gi", "cpu": "500m"}
   }
   ```

   **Expected**:
   - HTTP 202 Accepted
   - Session created in database with state='pending'
   - agent_id populated correctly
   - start_session command created
   - Command dispatched to agent via WebSocket
   - **API never calls Kubernetes API**

2. **Verify Agent Receives Command**:
   ```bash
   # Check agent logs
   kubectl logs -n streamspace deploy/streamspace-k8s-agent | grep start_session
   ```

   **Expected**:
   - Agent receives command via WebSocket
   - Agent fetches Template CRD from Kubernetes
   - Agent creates Deployment
   - Agent creates Service
   - Agent creates Session CRD
   - Agent updates database session state

3. **Verify Database State**:
   ```sql
   SELECT id, agent_id, cluster_id, state FROM sessions
   WHERE user_id = 'admin' ORDER BY created_at DESC LIMIT 1;
   ```

   **Expected**:
   - agent_id is NOT NULL
   - cluster_id is populated (if agent has cluster)
   - state transitions: pending → starting → running

4. **Multi-Agent Load Balancing**:
   - Start 2+ K8s agents with different agent_ids
   - Create 10 sessions
   - Verify sessions distributed evenly across agents

   **SQL Verification**:
   ```sql
   SELECT agent_id, COUNT(*) as session_count
   FROM sessions WHERE state IN ('running', 'starting')
   GROUP BY agent_id;
   ```

**Acceptance Criteria**:
- [ ] Session creation succeeds without API K8s access
- [ ] agent_id tracking works correctly
- [ ] cluster_id populated when available
- [ ] Load balancing distributes sessions evenly
- [ ] Agent receives all command fields correctly
- [ ] Pod creation successful
- [ ] Database state updated by agent

---

### Phase 3: Session Termination Testing (P0)

**Objective**: Verify termination works with new architecture

**Test Cases**:

1. **Basic Termination**:
   ```bash
   DELETE /api/v1/sessions/{session_id}
   ```

   **Expected**:
   - HTTP 202 Accepted
   - stop_session command created
   - Command routed to correct agent (based on agent_id)
   - Database state updated to 'terminating'
   - **API never calls Kubernetes API**

2. **Verify Agent Cleanup**:
   ```bash
   kubectl logs -n streamspace deploy/streamspace-k8s-agent | grep stop_session
   ```

   **Expected**:
   - Agent receives stop_session command
   - Agent deletes Deployment
   - Agent deletes Service
   - Agent deletes Session CRD (if exists)
   - Agent updates database state to 'terminated'

3. **Orphan Session Handling**:
   - Create session on agent A
   - Stop agent A
   - Attempt to terminate session

   **Expected Behavior**:
   - API returns error (agent offline)
   - OR session marked for cleanup when agent reconnects

**Acceptance Criteria**:
- [ ] Termination succeeds without API K8s access
- [ ] Command routed to correct agent
- [ ] Cleanup completes successfully
- [ ] Database state transitions correctly
- [ ] Orphaned sessions handled gracefully

---

### Phase 4: Session Hibernation & Wake Testing (NEW - P1)

**Objective**: Test new hibernate and wake endpoints

**Test Cases**:

1. **Hibernate Running Session**:
   ```bash
   PUT /api/v1/sessions/{session_id}/hibernate
   ```

   **Expected**:
   - HTTP 202 Accepted
   - hibernate_session command created
   - State: running → hibernating
   - Agent scales Deployment to 0 replicas
   - State: hibernating → hibernated
   - PVC preserved (if persistentHome=true)

2. **Wake Hibernated Session**:
   ```bash
   PUT /api/v1/sessions/{session_id}/wake
   ```

   **Expected**:
   - HTTP 202 Accepted
   - wake_session command created
   - State: hibernated → waking
   - Agent scales Deployment to 1 replica
   - State: waking → running
   - Pod mounts existing PVC (data persists)

3. **State Validation**:
   - Attempt to hibernate already hibernated session → 409 Conflict
   - Attempt to wake already running session → 409 Conflict
   - Attempt to wake terminated session → 404 or 409

**Acceptance Criteria**:
- [ ] Hibernate endpoint works correctly
- [ ] Wake endpoint works correctly
- [ ] State transitions are valid
- [ ] PVC data persists across hibernate/wake
- [ ] Invalid state transitions rejected

---

### Phase 5: Quota Enforcement Testing (P0)

**Objective**: Verify quota enforcement uses database instead of Kubernetes

**Test Cases**:

1. **User Quota Calculation**:
   - Create user with resource quota (2 CPU, 4Gi memory)
   - Create session (1 CPU, 2Gi) → Success
   - Create session (1 CPU, 2Gi) → Success (at limit)
   - Create session (1 CPU, 2Gi) → 403 Forbidden (over quota)

2. **Database-Based Calculation**:
   ```sql
   -- API should use this query, NOT Kubernetes API
   SELECT SUM(CAST(cpu AS NUMERIC)) as total_cpu,
          SUM(CAST(memory AS NUMERIC)) as total_memory
   FROM sessions
   WHERE user_id = 'test_user'
   AND state IN ('running', 'starting', 'hibernated', 'waking');
   ```

3. **Verify No K8s API Calls**:
   - Monitor API logs during session creation
   - Should see NO calls to `client-go` or Kubernetes API
   - All quota checks via database queries

**Acceptance Criteria**:
- [ ] Quota enforcement works correctly
- [ ] Uses database for usage calculation
- [ ] No Kubernetes API calls for quotas
- [ ] Quota errors return 403 with clear messages

---

### Phase 6: Template Management Testing (P1)

**Objective**: Verify templates work from database

**Test Cases**:

1. **List Templates**:
   ```bash
   GET /api/v1/templates
   ```

   **Expected**:
   - Returns templates from database
   - No Kubernetes CRD listing
   - Includes all template metadata

2. **Get Template**:
   ```bash
   GET /api/v1/templates/firefox-browser
   ```

   **Expected**:
   - Returns template from database
   - No Kubernetes CRD fetch

3. **Template Sync** (if implemented):
   - Verify agent can sync Template CRDs to database
   - OR verify admin can populate templates via API

**Acceptance Criteria**:
- [ ] Template listing works from database
- [ ] Template retrieval works from database
- [ ] No K8s API calls for template operations

---

### Phase 7: Agent Selector Testing (P1)

**Objective**: Test multi-agent routing logic

**Test Cases**:

1. **Load Balancing**:
   - Deploy 3 agents
   - Create 30 sessions
   - Verify distribution is roughly even (±2 sessions)

2. **Cluster Affinity**:
   - Set agent cluster_id='prod-us-east-1'
   - Create session with clusterID='prod-us-east-1'
   - Verify session routed to correct cluster

3. **Region Preference**:
   - Set agent region='us-west-2'
   - Create session with region='us-west-2'
   - Verify session routed to preferred region

4. **Health Filtering**:
   - Stop one agent (disconnect WebSocket)
   - Create sessions
   - Verify no sessions routed to offline agent

5. **Platform Filtering**:
   - Deploy K8s agent (platform='kubernetes')
   - Deploy Docker agent (platform='docker')
   - Create session with platform='docker'
   - Verify routed to Docker agent

**Acceptance Criteria**:
- [ ] Load balancing distributes evenly
- [ ] Cluster affinity works correctly
- [ ] Region preference works correctly
- [ ] Only online agents selected
- [ ] Platform filtering works correctly

---

### Phase 8: Error Handling Testing (P0)

**Test Cases**:

1. **No Agents Available**:
   - Stop all agents
   - Create session
   - Expected: HTTP 503 "No agents available"

2. **Agent Disconnects Mid-Session**:
   - Create session on agent A
   - Kill agent A
   - Verify session marked with stale agent
   - Restart agent A
   - Verify agent re-registers and resumes management

3. **Database Unavailable**:
   - Simulate database connection failure
   - Expected: API returns 500 errors (fail-closed)
   - No silent fallback to Kubernetes

4. **Invalid Session State**:
   - Attempt to hibernate terminated session
   - Expected: 404 or 409 error

**Acceptance Criteria**:
- [ ] Clear error messages for all failure scenarios
- [ ] No silent fallbacks to Kubernetes
- [ ] Proper HTTP status codes
- [ ] Graceful degradation where possible

---

### Phase 9: Backward Compatibility Testing (P1)

**Test Cases**:

1. **Existing Sessions**:
   - Sessions created before refactor (NULL agent_id)
   - Verify ListSessions includes them
   - Verify GetSession works
   - Verify termination fails gracefully (no agent assigned)

2. **Migration Path**:
   - Test upgrade from previous version
   - Verify migrations apply cleanly
   - Verify existing data preserved

**Acceptance Criteria**:
- [ ] Old sessions visible in listings
- [ ] Graceful handling of NULL agent_id
- [ ] Clean migration path documented

---

### Phase 10: Performance & Scalability Testing (P2)

**Test Cases**:

1. **API Response Times**:
   - Measure CreateSession latency (should be faster without K8s calls)
   - Target: < 100ms for API response (excluding agent provisioning)

2. **Concurrent Session Creation**:
   - Create 100 sessions concurrently
   - Verify all succeed
   - Verify even distribution across agents

3. **Database Query Performance**:
   - Monitor query times for agent selection
   - Verify indexes are used (EXPLAIN ANALYZE)

**Acceptance Criteria**:
- [ ] API responses faster than before
- [ ] Concurrent operations succeed
- [ ] Database queries optimized

---

## Test Execution Order

1. **Phase 1**: Database Migrations (prerequisite for all)
2. **Phase 2**: Session Creation (core functionality)
3. **Phase 3**: Session Termination (existing feature)
4. **Phase 4**: Hibernate & Wake (new features)
5. **Phase 5**: Quota Enforcement (critical path)
6. **Phase 8**: Error Handling (safety)
7. **Phase 7**: Agent Selector (advanced features)
8. **Phase 6**: Template Management (less critical)
9. **Phase 9**: Backward Compatibility (edge cases)
10. **Phase 10**: Performance (optimization)

---

## Success Criteria

**Must Pass (P0)**:
- All Phase 1-5 tests passing
- Phase 8 error handling tests passing
- No Kubernetes API calls from API process
- All session lifecycle operations working
- Agent selection and routing working

**Should Pass (P1)**:
- Phase 4 hibernate/wake tests passing
- Phase 6 template management working
- Phase 7 advanced agent selection features
- Phase 9 backward compatibility

**Nice to Have (P2)**:
- Phase 10 performance improvements verified

---

## Risk Assessment

**HIGH RISK AREAS**:
1. Session creation (complete refactor)
2. Agent selection (new service)
3. Database migrations (schema changes)
4. Quota enforcement (different data source)

**MEDIUM RISK AREAS**:
1. Hibernate/wake (new endpoints)
2. Template management (new layer)
3. Multi-agent routing (complex logic)

**LOW RISK AREAS**:
1. Session listing (minimal changes)
2. Authentication (unchanged)
3. Error handling (improved)

---

## Rollback Plan

If critical bugs discovered:

1. **Database Rollback**:
   ```bash
   psql -U streamspace -d streamspace < api/migrations/003_*_rollback.sql
   psql -U streamspace -d streamspace < api/migrations/002_*_rollback.sql
   psql -U streamspace -d streamspace < api/migrations/001_*_rollback.sql
   ```

2. **Code Rollback**:
   ```bash
   git revert <commit_hash>
   ```

3. **Deployment Rollback**:
   ```bash
   helm rollback streamspace <revision>
   ```

---

## Notes for Validator

1. **Testing Environment**: Use k3s cluster with at least 2 K8s agents
2. **Database Access**: Direct PostgreSQL access required for verification
3. **Log Monitoring**: Watch both API and agent logs simultaneously
4. **Network Inspection**: Verify no K8s API traffic from API pods
5. **Documentation**: Create comprehensive test report with evidence

**Estimated Testing Time**: 2-3 days for thorough validation

---

**Created By**: Architect (Agent 1)
**Date**: 2025-11-21
**Version**: v2.0-beta Kubernetes Removal Validation
