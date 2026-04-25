# Wave 27 Task Assignments - Multi-Tenancy Security & Test Fixes

**Wave:** 27
**Start Date:** 2025-11-26
**Target Completion:** 2025-11-28 EOD
**Status:** 🔴 IN PROGRESS - P0 Critical Security Work

---

## Wave 27 Overview

**Critical Priority Shift:** Design & governance review identified P0 multi-tenancy security vulnerabilities that must be fixed before v2.0-beta.1 release.

**Wave Goals:**
1. ✅ Fix P0 multi-tenancy security vulnerabilities (#211, #212)
2. ✅ Complete broken test suite fixes (#200)
3. ✅ Add backup/DR documentation (#217)
4. ✅ Create observability dashboards (#218)
5. ✅ Unblock v2.0-beta.1 release

**Timeline Impact:** v2.0-beta.1 release delayed 2-3 days to 2025-11-28 or 2025-11-29

---

## 🔨 Builder (Agent 2) - P0 CRITICAL SECURITY

**Branch:** `claude/v2-builder`
**Timeline:** 2 days (2025-11-26 → 2025-11-28)
**Status:** Active - Security implementation
**Priority:** P0 - HIGHEST (blocking release)

### Task 1: Issue #212 - Org Context & RBAC Plumbing (P0)

**Timeline:** 1-2 days
**Priority:** P0 - CRITICAL
**Milestone:** v2.0-beta.1
**Dependencies:** None (start immediately)

**Description:**
Implement organization-scoped RBAC to prevent cross-tenant data access. Currently, JWT claims and auth middleware do not surface org context, so handlers cannot enforce org-scoped access controls.

**Implementation Steps:**

1. **Update JWT Claims Structure** (2-4 hours)
   - File: `api/internal/auth/jwt.go`
   - Add `org_id` field to JWT claims struct
   - Add `org_name` field (optional, for display)
   - Ensure backward compatibility with existing tokens
   - Update token generation to include org_id from user record

2. **Update Auth Middleware** (2-4 hours)
   - File: `api/internal/middleware/auth.go`
   - Extract `org_id` from JWT claims
   - Populate org_id in request context: `ctx = context.WithValue(ctx, "org_id", orgID)`
   - Populate user_id in request context (if not already done)
   - Return 401 Unauthorized if org_id missing from valid token

3. **Update Database Queries - Sessions** (4-6 hours)
   - Files: `api/internal/handlers/sessions.go`, `api/internal/services/session_service.go`
   - Add org_id to all session queries (list, get, create, update, delete)
   - ListSessions: `WHERE org_id = $1` (from context)
   - GetSession: `WHERE session_id = $1 AND org_id = $2`
   - CreateSession: Insert with org_id from context
   - UpdateSession: `WHERE session_id = $1 AND org_id = $2`
   - DeleteSession: `WHERE session_id = $1 AND org_id = $2`

4. **Update Database Queries - Templates** (2-4 hours)
   - Files: `api/internal/handlers/sessiontemplates.go`, `api/internal/db/templates.go`
   - Add org_id to template queries (list, get, create, update, delete)
   - Templates may be org-specific or global (is_public flag)
   - ListTemplates: `WHERE org_id = $1 OR is_public = true`
   - GetTemplate: `WHERE template_id = $1 AND (org_id = $2 OR is_public = true)`

5. **Update Database Queries - Other Resources** (4-6 hours)
   - Files: Various handlers (agents, webhooks, audit logs, etc.)
   - Agents: List/view agents scoped to org's clusters
   - Webhooks: `WHERE org_id = $1`
   - Audit Logs: `WHERE org_id = $1` (admins can view org logs, users view own)
   - API Keys: `WHERE org_id = $1 AND user_id = $2` (user's keys in org)
   - Quotas: `WHERE org_id = $1`

6. **Update WebSocket Handlers** (covered in Task 2)

7. **Add Tests** (4-6 hours)
   - Test org isolation: User A cannot access User B's sessions (different orgs)
   - Test within org: User A can access User B's sessions (same org, if admin)
   - Test 403 Forbidden when accessing other org's resources
   - Test JWT claims include org_id
   - Test middleware populates org_id in context

**Deliverable:**
- `.claude/reports/P0_ORG_CONTEXT_IMPLEMENTATION.md` - Implementation report
- All API handlers enforce org-scoping
- All database queries include org_id filters
- Tests validate org isolation

**Reference Documents:**
- `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/03-system-design/authz-and-rbac.md`
- `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/09-risk-and-governance/code-observations.md`

---

### Task 2: Issue #211 - WebSocket Org Scoping (P0)

**Timeline:** 4-8 hours
**Priority:** P0 - CRITICAL
**Milestone:** v2.0-beta.1
**Dependencies:** Task 1 (#212) must be complete first

**Description:**
Fix WebSocket broadcast cross-tenant data leakage. Currently, session/metrics broadcasts use hardcoded namespace "streamspace" and broadcast all sessions to any connected client without org filtering.

**Implementation Steps:**

1. **Add Auth Guard to WebSocket Handlers** (2-3 hours)
   - File: `api/internal/websocket/handlers.go`
   - Extract org_id from request context before WebSocket upgrade
   - Verify user has permission to subscribe (RBAC check)
   - Return 403 Forbidden if org_id missing or unauthorized
   - Pass org_id to broadcast subscription/filtering logic

2. **Filter Session Broadcasts by Org** (2-3 hours)
   - File: `api/internal/websocket/handlers.go` - HandleSessionsWebSocket
   - Replace: `sessions, err := h.sessionService.ListSessions(ctx, "streamspace")`
   - With: `sessions, err := h.sessionService.ListSessions(ctx, namespace)` where namespace = org's K8s namespace
   - Filter broadcast messages: Only send sessions for subscriber's org_id
   - Query: `SELECT * FROM sessions WHERE org_id = $1`

3. **Filter Metrics Broadcasts by Org** (1-2 hours)
   - File: `api/internal/websocket/handlers.go` - HandleMetricsWebSocket
   - Aggregate metrics per org: `COUNT(*) FROM sessions WHERE org_id = $1 GROUP BY status`
   - Broadcast only org-scoped metrics to subscriber

4. **Replace Hardcoded Namespace** (2-3 hours)
   - Current: `ListSessions(ctx, "streamspace")` uses hardcoded namespace
   - New: Derive namespace from org_id
   - Options:
     - Map org_id → K8s namespace (e.g., org-<org_id>, or custom mapping)
     - Store namespace in org table: `SELECT namespace FROM orgs WHERE org_id = $1`
   - Fail closed: Return error if namespace unknown

5. **Use Cancellable Contexts** (1-2 hours)
   - Replace `context.Background()` with request-scoped context
   - Cancel WebSocket goroutines when client disconnects
   - Cancel K8s log streams when client drops
   - Add context deadline for long-running operations

6. **Add Tests** (2-3 hours)
   - Test WebSocket session broadcasts filtered by org (no leakage)
   - Test metrics broadcasts scoped to org
   - Test unauthorized org subscription blocked (403)
   - Test namespace selection per org (no hardcoded "streamspace")
   - Test context cancellation on client disconnect

**Deliverable:**
- `.claude/reports/P0_WEBSOCKET_ORG_SCOPING.md` - Implementation report
- WebSocket broadcasts org-scoped and filtered
- No hardcoded "streamspace" namespace
- Cancellable contexts for WebSocket goroutines
- Tests validate org isolation

**Reference Documents:**
- `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/03-system-design/websocket-hardening.md`
- `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/03-system-design/websocket-hardening-checklist.md`

---

### Task 3: Issue #218 - Observability Dashboards (P1)

**Timeline:** 6-8 hours
**Priority:** P1 - HIGH
**Milestone:** v2.0-beta.1
**Dependencies:** None (can be done in parallel)

**Description:**
Create starter Grafana dashboards and alert rules aligned to SLOs for production monitoring.

**Implementation Steps:**

1. **Control Plane Dashboard** (2-3 hours)
   - Panels:
     - API request rate (requests/sec)
     - API error rate (5xx, 4xx %)
     - API latency (p50, p95, p99)
     - Active WebSocket connections
     - Database connection pool usage
   - Metrics source: Prometheus/OpenTelemetry
   - File: `manifests/observability/dashboards/control-plane.json`

2. **Session Lifecycle Dashboard** (2-3 hours)
   - Panels:
     - Session creation rate (sessions/minute)
     - Session start latency (p50, p95, p99)
     - Active sessions by status (running/pending/failed)
     - Session failure rate
     - Session termination rate
   - File: `manifests/observability/dashboards/sessions.json`

3. **Agent Health Dashboard** (1-2 hours)
   - Panels:
     - Agent count by status (online/degraded/offline)
     - Agent heartbeat freshness (last heartbeat age)
     - Agent capacity (sessions per agent)
     - Agent distribution by platform/region
   - File: `manifests/observability/dashboards/agents.json`

4. **Alert Rules** (2-3 hours)
   - API 5xx error rate > 1% for 5 minutes
   - API p99 latency > 500ms for 10 minutes
   - Session start p99 > 15s for 15 minutes
   - Agent heartbeat stale (>60s) for any agent
   - No online agents available
   - File: `manifests/observability/alerts/critical.yaml`

**Deliverable:**
- Grafana dashboard JSON configs (3 dashboards)
- Prometheus alert rules YAML
- Documentation in `docs/OBSERVABILITY.md` (how to deploy/customize)

**Reference Documents:**
- `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/06-operations-and-sre/observability-dashboards.md`
- `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/06-operations-and-sre/slo.md`

---

## 🧪 Validator (Agent 3) - P0 CRITICAL TESTING

**Branch:** `claude/v2-validator`
**Timeline:** 2 days (2025-11-26 → 2025-11-28)
**Status:** Active - Testing & validation
**Priority:** P0 - HIGHEST (blocking release)

### Task 1: Issue #200 - Fix Broken Test Suites (P0)

**Timeline:** 4-8 hours
**Priority:** P0 - CRITICAL
**Milestone:** v2.0-beta.1
**Dependencies:** None (start immediately)

**Description:**
Fix broken test suites in API handlers, K8s agent, and UI components. Many tests are currently failing due to recent refactoring and validation framework changes.

**Implementation Steps:**

1. **Fix API Handler Tests** (2-4 hours)
   - Run: `cd api && go test ./internal/handlers/... -v`
   - Identify failing tests (likely related to validation framework changes)
   - Update test mocks to include validation context
   - Update expected error messages (validation framework standardized errors)
   - Files: `api/internal/handlers/*_test.go`

2. **Fix K8s Agent Tests** (1-2 hours)
   - Run: `cd agents/k8s-agent && go test ./... -v`
   - Fix any failing tests
   - File: `agents/k8s-agent/agent_test.go`

3. **Fix UI Component Tests** (1-2 hours)
   - Run: `cd ui && npm test`
   - Fix failing component tests
   - Update mocks for API validation responses
   - Files: `ui/src/**/*.test.tsx`

**Deliverable:**
- `.claude/reports/P0_TEST_SUITE_FIXES.md` - Test fix report
- All test suites passing: API (100%), K8s Agent (100%), Docker Agent (100%), UI (100%)
- CI/CD green

---

### Task 2: Validate Issue #212 - Org Context (P0)

**Timeline:** 4-6 hours
**Priority:** P0 - CRITICAL
**Milestone:** v2.0-beta.1
**Dependencies:** Builder Task 1 (#212) must be complete

**Description:**
Validate that org-scoping is correctly implemented and enforced across all API endpoints and WebSocket handlers.

**Validation Steps:**

1. **Setup Test Environment** (1 hour)
   - Create 2 test orgs: org-A, org-B
   - Create 2 test users: user-A (org-A), user-B (org-B)
   - Create JWT tokens with org_id for each user

2. **Test Org Isolation - Sessions** (1-2 hours)
   - User A creates session in org-A
   - User B creates session in org-B
   - Test: User A lists sessions → sees only org-A sessions
   - Test: User B lists sessions → sees only org-B sessions
   - Test: User A tries to GET user B's session → 403 Forbidden
   - Test: User A tries to DELETE user B's session → 403 Forbidden

3. **Test Org Isolation - Templates** (1 hour)
   - Create org-specific template in org-A
   - Create public template (is_public=true)
   - Test: User A sees org-A templates + public templates
   - Test: User B sees org-B templates + public templates (NOT org-A private)

4. **Test Org Isolation - Other Resources** (1-2 hours)
   - Test webhooks scoped to org
   - Test audit logs scoped to org
   - Test API keys scoped to org + user
   - Test quotas scoped to org

5. **Test JWT Claims** (30 minutes)
   - Verify JWT tokens include org_id
   - Verify middleware extracts org_id into context
   - Verify missing org_id returns 401 Unauthorized

**Deliverable:**
- `.claude/reports/P0_ORG_CONTEXT_VALIDATION.md` - Validation report
- All org isolation tests passing
- No cross-org data leakage

---

### Task 3: Validate Issue #211 - WebSocket Scoping (P0)

**Timeline:** 4-6 hours
**Priority:** P0 - CRITICAL
**Milestone:** v2.0-beta.1
**Dependencies:** Builder Task 2 (#211) must be complete

**Description:**
Validate that WebSocket broadcasts are org-scoped and filtered correctly.

**Validation Steps:**

1. **Test Session Broadcast Filtering** (2-3 hours)
   - Connect user-A WebSocket (org-A)
   - Connect user-B WebSocket (org-B)
   - Create session in org-A
   - Verify: User A receives broadcast for org-A session
   - Verify: User B does NOT receive broadcast for org-A session
   - Create session in org-B
   - Verify: User B receives broadcast for org-B session
   - Verify: User A does NOT receive broadcast for org-B session

2. **Test Metrics Broadcast Scoping** (1-2 hours)
   - Connect user-A to metrics WebSocket
   - Verify metrics show only org-A counts (not global)
   - Connect user-B to metrics WebSocket
   - Verify metrics show only org-B counts

3. **Test Unauthorized Access** (1 hour)
   - Try to subscribe to WebSocket without JWT → 401
   - Try to subscribe with org_id missing from JWT → 401
   - Try to subscribe to other org's namespace → 403

4. **Test Namespace Selection** (1 hour)
   - Verify sessions created in correct namespace (not hardcoded "streamspace")
   - Verify namespace derived from org_id
   - Verify error if namespace unknown/unmapped

5. **Test Context Cancellation** (1 hour)
   - Connect WebSocket, start session log stream
   - Disconnect WebSocket client
   - Verify K8s log stream cancelled (no resource leak)

**Deliverable:**
- `.claude/reports/P0_WEBSOCKET_VALIDATION.md` - Validation report
- All WebSocket org isolation tests passing
- No cross-org broadcast leakage

---

## 📝 Scribe (Agent 4) - P1 DOCUMENTATION

**Branch:** `claude/v2-scribe`
**Timeline:** 1 day (2025-11-26 → 2025-11-27)
**Status:** Active - Documentation
**Priority:** P1 - HIGH (required for release)

### Task 1: Issue #217 - Backup & DR Guide (P1)

**Timeline:** 4-6 hours
**Priority:** P1 - HIGH
**Milestone:** v2.0-beta.1
**Dependencies:** None (start immediately)

**Description:**
Create comprehensive backup and disaster recovery guide for production deployments.

**Content Outline:**

1. **Overview** (30 minutes)
   - RPO/RTO targets: RPO 1 hour, RTO 4 hours
   - Backup scope: Database, Redis, persistent storage, secrets
   - Disaster scenarios covered

2. **PostgreSQL Backup** (1-2 hours)
   - Automated backup schedule (daily full + hourly incremental)
   - Backup retention policy (30 days daily, 12 months monthly)
   - Managed DB backups (AWS RDS, GCP Cloud SQL, Azure Database)
   - Self-hosted backups (pg_dump, WAL archiving)
   - Restore procedures with examples
   - Validation: Test restores monthly

3. **Redis Backup** (1 hour)
   - RDB snapshots vs AOF persistence
   - Backup schedule (hourly snapshots)
   - Managed Redis backups (ElastiCache, MemoryStore)
   - Self-hosted backups (BGSAVE, redis-cli --rdb)
   - Restore procedures

4. **Persistent Storage Backup** (1 hour)
   - Session home directories (NFS/CSI volumes)
   - Snapshot schedule (daily)
   - CSI snapshot examples (AWS EBS, GCP PD, Azure Disk)
   - NFS backup strategies
   - Restore procedures

5. **Secrets & Config Backup** (30 minutes)
   - Kubernetes secrets backup (via etcd backup or Velero)
   - ConfigMaps backup
   - Restore procedures

6. **Disaster Recovery Runbook** (1-2 hours)
   - DR scenario: Total cluster loss
   - DR scenario: Database corruption
   - DR scenario: Storage failure
   - Step-by-step recovery procedures
   - Validation checklist

7. **Backup Monitoring & Alerts** (30 minutes)
   - Backup success/failure alerts
   - Backup age monitoring
   - Restore drill schedule (quarterly)

**Deliverable:**
- `docs/BACKUP_AND_DR_GUIDE.md` - Complete backup/DR guide
- Add backup validation to release checklist

**Reference Documents:**
- `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/06-operations-and-sre/backup-and-dr.md`

---

### Task 2: Document Design Docs Strategy (P2)

**Timeline:** 2-3 hours
**Priority:** P2 - MEDIUM
**Milestone:** v2.0-beta.1 (nice to have)

**Description:**
Document the strategy for maintaining design & governance documentation in separate private GitHub repo.

**Content:**

1. **Overview**
   - Design docs location: `/Users/s0v3r1gn/streamspace/streamspace-design-and-governance/`
   - Private GitHub repo: `streamspace-dev/streamspace-design-and-governance` (to be created)
   - Main repo links to design docs for reference

2. **Repository Structure**
   - Design docs repo structure (00-product-vision through 09-risk-and-governance)
   - Main repo minimal docs (ARCHITECTURE.md, DEPLOYMENT.md, etc.)
   - How to contribute to design docs

3. **Synchronization Strategy**
   - Design docs updated via direct editing in private repo
   - Main repo references design docs via links
   - ADRs copied to main repo `docs/design/architecture/` for visibility

4. **Access Control**
   - Private repo for design docs (team access only)
   - Main repo docs are public (deployment guides, API docs)

**Deliverable:**
- `docs/DESIGN_DOCS_STRATEGY.md` - Design docs strategy
- Update `README.md` to link to design docs repo

---

### Task 3: Update MULTI_AGENT_PLAN (Post-Wave 27)

**Timeline:** 2-4 hours
**Priority:** P1 - HIGH
**Dependencies:** Wave 27 complete

**Description:**
Document Wave 27 integration in MULTI_AGENT_PLAN.md after completion.

**Content:**
- Wave 27 integration summary
- Files changed, lines added/removed
- Issues resolved (#211, #212, #200, #217, #218)
- Impact on v2.0-beta.1 release

**Deliverable:**
- Updated `MULTI_AGENT_PLAN.md` with Wave 27 summary

---

## 🏗️ Architect (Agent 1) - COORDINATION

**Branch:** `feature/streamspace-v2-agent-refactor`
**Timeline:** Daily (ongoing)
**Status:** Active - Coordination & integration

### Tasks:

1. ✅ **Design & Governance Review** - COMPLETE
   - Reviewed 63 design documents
   - Identified P0 security vulnerabilities
   - Created comprehensive review report

2. ✅ **Issue Reassignment** - COMPLETE
   - Assigned #211, #212, #217, #218 to v2.0-beta.1 milestone
   - Assigned #213-#216, #219 to v2.0-beta.2 milestone

3. ✅ **MULTI_AGENT_PLAN Update** - COMPLETE
   - Added Wave 27 planning
   - Updated release timeline (2025-11-28/29)
   - Created detailed task assignments

4. ⏳ **Daily Coordination** - ONGOING
   - Monitor Builder progress on #212/#211
   - Monitor Validator progress on #200 and validations
   - Monitor Scribe progress on #217
   - Daily check-ins with agents

5. ⏳ **Wave 27 Integration** - TARGET: 2025-11-28 EOD
   - Integrate Builder branch (security fixes)
   - Integrate Validator branch (test fixes, validations)
   - Integrate Scribe branch (documentation)
   - Resolve conflicts
   - Update MULTI_AGENT_PLAN with Wave 27 summary

6. ⏳ **Release Coordination**
   - Update release checklist with org-scoping validation
   - Final release readiness review
   - Coordinate v2.0-beta.1 release (2025-11-28/29)

---

## Wave 27 Success Criteria

**Must Complete Before Integration:**

**Builder:**
- ✅ Issue #212 implemented and tested
- ✅ Issue #211 implemented and tested
- ✅ Issue #218 dashboards created
- ✅ All code committed to `claude/v2-builder` branch
- ✅ Implementation reports in `.claude/reports/`

**Validator:**
- ✅ Issue #200 test fixes complete (all tests passing)
- ✅ Issue #212 validated (org isolation confirmed)
- ✅ Issue #211 validated (WebSocket org-scoping confirmed)
- ✅ Validation reports in `.claude/reports/`

**Scribe:**
- ✅ Issue #217 backup/DR guide complete
- ✅ Design docs strategy documented
- ✅ Documentation committed to `claude/v2-scribe` branch

**Architect:**
- ✅ All agent branches integrated into `feature/streamspace-v2-agent-refactor`
- ✅ No merge conflicts
- ✅ All tests passing in integrated branch
- ✅ MULTI_AGENT_PLAN updated with Wave 27 summary

---

## Critical Path

**Day 1 (2025-11-26):**
- Builder: Start #212 (org context)
- Validator: Fix #200 (broken tests)
- Scribe: Start #217 (backup/DR guide)

**Day 2 (2025-11-27):**
- Builder: Complete #212, start #211 (WebSocket)
- Validator: Validate #212, start #211 validation
- Scribe: Complete #217, start design docs strategy

**Day 3 (2025-11-28):**
- Builder: Complete #211, start #218 (dashboards)
- Validator: Complete #211 validation, final testing
- Scribe: Complete design docs strategy
- Architect: Wave 27 integration

**Day 4 (2025-11-29):**
- All: Final validation and release prep
- v2.0-beta.1 release!

---

**Report Status:** ✅ COMPLETE
**Distribution:** All agents (Builder, Validator, Scribe)
**Next Action:** Agents begin Wave 27 work immediately
