# k8sClient Refactoring Roadmap

## Executive Summary

**Current State:** k8sClient scattered across 12 files performing 50+ K8s operations

**Goal:** Consolidate K8s management logic into controller, keep API for:
- Read-only queries
- Real-time operations (heartbeats, WebSocket)
- Administrative triggers (application installation)

**Timeline:** Phased approach over 3 phases
**Effort:** 15-20 developer weeks
**Risk Level:** Medium (requires careful state machine design)

---

## Phase 1: Preparation & Design (Weeks 1-2)

### Task 1.1: Design Controller Reconcilers
**File:** `controller/internal/controllers/session_reconciler.go`
**Work:**
- Design Session state machine (Pending → Running/Hibernated → Terminated)
- Design IdleDetection reconciler
- Design ConnectionTracking reconciler
- Define CRD status fields for controller feedback

**Acceptance Criteria:**
- [ ] State machine diagram in docs
- [ ] CRD spec updated with new status fields
- [ ] Controller interfaces documented

### Task 1.2: Design Admission Webhooks
**File:** `controller/internal/webhooks/session_validator.go`
**Work:**
- Design ValidatingWebhook for Session creation (quota validation)
- Design MutatingWebhook for Session defaults
- Plan certificate management

**Acceptance Criteria:**
- [ ] Webhook manifest examples
- [ ] Quota validation logic in webhook code
- [ ] Error handling documented

### Task 1.3: API-Controller Communication Protocol
**Work:**
- Define how API signals controller for operations (CRD fields)
- Plan connection event propagation (webhook or annotation)
- Document async operation patterns

**Acceptance Criteria:**
- [ ] Communication protocol document
- [ ] Example payload flows

---

## Phase 2: Controller Implementation (Weeks 3-10)

### Task 2.1: Session Lifecycle Controller
**Priority:** HIGH
**File:** `controller/internal/controllers/session_controller.go`
**Work:**
- Implement Session state machine
- Handle transitions: Pending → Running/Hibernated → Terminated
- Implement Deployment/PVC creation logic (from API)
- Implement session cleanup

**Changes to API:**
- `CreateSession()` → Creates Session CRD only (state: Pending)
- Controller creates Deployment
- API watches for Running status

**K8s Operations Moved:**
- `h.k8sClient.CreateSession()` → Controller (creates Pod/Deployment)
- `h.k8sClient.UpdateSessionState()` → Controller (state transitions)
- `h.k8sClient.DeleteSession()` → Controller (cleanup)

**Acceptance Criteria:**
- [ ] Session state transitions working
- [ ] Deployment/PVC created automatically
- [ ] Status fields updated correctly
- [ ] E2E test: Create session → pods appear

### Task 2.2: Idle Detection Controller
**Priority:** HIGH
**File:** `controller/internal/controllers/idle_reconciler.go`
**Work:**
- Watch Session.Status.LastActivity
- Calculate idle duration
- Auto-hibernate after threshold + grace period
- Update Session.Spec.State to "hibernated"

**Changes to API:**
- Remove `activity/tracker.go` background loop
- Keep `activity.UpdateSessionActivity()` for heartbeat endpoint
- API heartbeat endpoint only updates lastActivity timestamp

**K8s Operations Moved:**
- `tracker.ListSessions()` for idle check → Controller
- `session.UpdateSession()` for hibernation → Controller

**Acceptance Criteria:**
- [ ] Heartbeat updates lastActivity
- [ ] Controller detects idle sessions
- [ ] Auto-hibernation works
- [ ] E2E test: Session idle after 30m → hibernated

### Task 2.3: Connection-Based Auto-Start
**Priority:** MEDIUM
**File:** `controller/internal/controllers/autostart_reconciler.go`
**Work:**
- Implement connection event webhook
- API sends connection events
- Controller auto-starts hibernated sessions
- Update Session.Spec.State to "running"

**Changes to API:**
- Remove `tracker.autoStartHibernatedSession()`
- API tracks connections (DB only)
- API sends webhook when connection arrives
- Controller receives webhook and starts session

**K8s Operations Moved:**
- `ct.k8sClient.UpdateSessionState()` for auto-start → Controller

**Acceptance Criteria:**
- [ ] Connection events logged
- [ ] Webhook integration working
- [ ] Auto-start on connection works
- [ ] E2E test: Connection to hibernated session → auto-start

### Task 2.4: Node Management Controller
**Priority:** MEDIUM
**File:** `controller/internal/controllers/nodeops_reconciler.go`
**Work:**
- Create NodeOperation CRD for maintenance requests
- Implement cordon/drain/uncordon logic
- Update node labels/taints via controller

**Changes to API:**
- API creates NodeOperation CR (not direct node operations)
- Keep read-only node endpoints (`ListNodes()`, `GetNode()`)
- API: `AddNodeLabel()` → Create NodeOperation CR
- Controller: watches NodeOperation and applies changes

**K8s Operations Moved:**
- `h.k8sClient.PatchNode()` → Controller
- `h.k8sClient.CordonNode()` → Controller
- `h.k8sClient.DrainNode()` → Controller

**Acceptance Criteria:**
- [ ] NodeOperation CRD defined
- [ ] Cordon logic working
- [ ] Drain logic working
- [ ] E2E test: Cordon node via API → node unschedulable

### Task 2.5: Integration & Testing
**File:** `controller/tests/integration_test.go`
**Work:**
- Test all 4 reconcilers together
- Test failure scenarios
- Test state persistence
- Performance testing

**Acceptance Criteria:**
- [ ] 100+ integration tests passing
- [ ] All reconcilers tested
- [ ] Failure scenarios handled
- [ ] Performance acceptable

---

## Phase 3: API Refactoring & Migration (Weeks 11-16)

### Task 3.1: Remove Session Lifecycle Logic from API
**Files Affected:**
- `api/internal/api/handlers.go` (CreateSession, UpdateSession, DeleteSession)
- `api/internal/tracker/tracker.go` (remove entirely)
- `api/internal/activity/tracker.go` (remove background loop)

**Changes:**
```go
// BEFORE
func (h *Handler) CreateSession(c *gin.Context) {
    session := &k8s.Session{...}
    created, err := h.k8sClient.CreateSession(ctx, session)  // ❌ Removed
}

// AFTER
func (h *Handler) CreateSession(c *gin.Context) {
    session := &k8s.Session{...}
    // Controller will create Deployment
    created, err := h.k8sClient.CreateSession(ctx, session)  // Still creates CRD
    
    // Wait for controller to set Status.Running
    // Or return 202 Accepted (async)
}
```

**K8s Operations Removed from API:**
- CreateSession (Deployment creation)
- UpdateSessionState (state transitions)
- DeleteSession (pod eviction)
- ListSessionsForIdleCheck
- UpdateSessionActivity (partial - keep heartbeat endpoint)

**Acceptance Criteria:**
- [ ] API handlers simplified
- [ ] No session state transitions in API
- [ ] No pod creation in API
- [ ] All logic moved to controller

### Task 3.2: Keep Read-Only & Real-Time APIs
**Files to Keep:**
- Dashboard queries (ListTemplates, etc.)
- WebSocket broadcasters
- Activity heartbeat endpoint
- Connection tracking

**Changes:**
- `GetSession()` - KEEP (read-only)
- `ListSessions()` - KEEP (read-only)
- `RecordHeartbeat()` - KEEP (real-time)
- `ListNodes()` - KEEP (read-only monitoring)

**Acceptance Criteria:**
- [ ] All read-only endpoints working
- [ ] Real-time endpoints low-latency
- [ ] WebSocket broadcasting working

### Task 3.3: Quota Enforcement Migration
**File:** `controller/internal/webhooks/session_validator.go`
**Work:**
- Move quota validation to ValidatingWebhook
- Webhook blocks Session creation if quota exceeded
- API removes quota checks from handler

**Changes:**
```go
// BEFORE
func (h *Handler) CreateSession(c *gin.Context) {
    // Check quota
    err := h.quotaEnforcer.CheckSessionCreation(...)  // ❌ Removed

// AFTER (in webhook)
func (v *SessionValidator) ValidateCreate(session *k8s.Session) error {
    // Check quota
    return v.quotaEnforcer.CheckSessionCreation(...)
}
```

**Acceptance Criteria:**
- [ ] Webhook quota validation working
- [ ] API quota checks removed
- [ ] 403 returned for quota violations
- [ ] E2E test: Over-quota session rejected

### Task 3.4: Documentation & Migration Guide
**Files to Create:**
- `docs/CONTROLLER_RECONCILERS.md` - Controller architecture
- `docs/API_CONTROLLER_SPLIT.md` - Responsibility boundaries
- `MIGRATION_GUIDE_API_TO_CONTROLLER.md` - Deployment instructions

**Work:**
- Document all controller reconcilers
- Update API documentation
- Create user-facing migration guide
- Update CLAUDE.md with new patterns

**Acceptance Criteria:**
- [ ] All reconcilers documented
- [ ] API/controller split clear
- [ ] Migration guide complete
- [ ] Examples for all patterns

### Task 3.5: Deployment & Rollout
**Work:**
- Update Helm chart for new controller
- Update CI/CD pipelines
- Gradual rollout strategy
- Rollback plan

**Acceptance Criteria:**
- [ ] Helm chart updated
- [ ] CI/CD working
- [ ] Rollout checklist complete
- [ ] Rollback tested

---

## Detailed File Mapping

### FILES TO MODIFY

**api/internal/api/handlers.go** (-80% operations)
```
REMOVE:                              ADD:
- CreateSession (full)     →        - Wait for controller status
- UpdateSessionState (all) →        - Return 202 Accepted for async ops
- DeleteSession (full)     →        - Error handling for webhook rejections
- GetPods (quota)          →        - Check CRD status
- enrichSessionWithDBInfo  →        
```

**api/internal/activity/tracker.go** (-70% operations)
```
REMOVE:                              KEEP:
- StartIdleMonitor loop    →        - UpdateSessionActivity (heartbeat)
- hibernateIdleSessions    →        - GetActivityStatus (read-only)
- Check idle logic         →        
- Update state to hibernated→
```

**api/internal/tracker/tracker.go** (Remove entirely)
```
DELETE ENTIRE FILE:
- All logic moved to controller
- Connection tracking to DB only (no state changes)
```

**api/internal/handlers/nodes.go** (-50% operations)
```
REMOVE:                              KEEP:
- AddNodeLabel             →        - ListNodes
- RemoveNodeLabel          →        - GetNode
- AddNodeTaint             →        - GetClusterStats
- RemoveNodeTaint          →
- CordonNode               →
- UncordonNode             →
- DrainNode                →
```

### FILES TO CREATE

**controller/internal/controllers/session_controller.go**
```go
// New: Session lifecycle reconciliation
- Reconcile(Session) error
- createDeployment()
- createPVC()
- handleStateTransitions()
- cleanup()
```

**controller/internal/controllers/idle_reconciler.go**
```go
// New: Idle detection & hibernation
- Reconcile(Session) error
- detectIdleSessions()
- hibernateSession()
```

**controller/internal/controllers/autostart_reconciler.go**
```go
// New: Connection-based auto-start
- HandleConnectionEvent(connectionID, sessionID)
- startSession()
```

**controller/internal/controllers/nodeops_reconciler.go**
```go
// New: Node maintenance operations
- Reconcile(NodeOperation) error
- applyNodePatch()
- cordonNode()
- drainNode()
```

**controller/internal/webhooks/session_validator.go**
```go
// New: Quota validation at admission time
- ValidateCreate(Session) error
- ValidateUpdate(old, new Session) error
- checkQuota()
```

---

## Risk Mitigation

### Risk 1: Quota Enforcement
**Risk:** Webhook validation takes longer than API check
**Mitigation:**
- Webhook should be fast (cache quota limits)
- Fall-back: API maintains quota check temporarily
- Gradual migration with feature flag

### Risk 2: Stale Controller Status
**Risk:** API returns wrong session status if controller lags
**Mitigation:**
- API checks CRD status (not DB cache)
- Expose reconciliation timestamp to clients
- Health check: controller uptime metric

### Risk 3: Lost Session State
**Risk:** Session state inconsistency during migration
**Mitigation:**
- Backup all sessions before migration
- Run controller and API in parallel temporarily
- Verify CRD status matches expected state

### Risk 4: Connection Event Loss
**Risk:** Missed connection events if webhook fails
**Mitigation:**
- API fallback: mark session as "wake_requested"
- Controller polls for wake requests periodically
- Webhook retry policy

---

## Success Metrics

| Metric | Target | Current |
|--------|--------|---------|
| K8s operations in API | < 20 | 50+ |
| Controller reconcilers | 4+ | 0 |
| Session state transitions in controller | 100% | 0% |
| API heartbeat latency | < 100ms | Varies |
| Test coverage (controller) | > 85% | N/A |
| Deployment rollout time | < 10 min | N/A |

---

## Dependencies

### External
- Kubernetes 1.19+ (webhook support)
- cert-manager (webhook cert management)
- etcd persistence (CRD state)

### Internal
- `k8s.Client` - both API and controller
- `db.Database` - connection tracking, DB records
- `quota.Enforcer` - moved to webhook

---

## Communication Plan

### Developers
- Sync meetings: 2x/week during Phase 2-3
- Slack channel: #streamspace-refactoring
- Decision log in `/docs/REFACTORING_DECISIONS.md`

### Operators
- Deployment guide in `/docs/DEPLOYMENT_GUIDE.md`
- Backward compatibility for 2 releases
- Gradual rollout (staging → production)

### Users
- Blog post: "Controller-Driven Architecture"
- No user-facing changes (transparent migration)
- Beta feature flag for early adopters

---

## Rollback Plan

### If Phase 1 (Design) Fails
- Continue with current architecture
- Loss: 2 weeks planning

### If Phase 2 (Controller) Fails
- Disable controller, use API fallback
- Keep code in separate branch
- Restart with simplified design

### If Phase 3 (Migration) Fails
- Keep old API handlers in place
- Use feature flag to toggle between old/new
- Gradual migration per resource type

---

## Next Steps

1. **Week 1:** Schedule design review with team
2. **Week 1:** Create CRD updates PR
3. **Week 2:** Approve controller design
4. **Week 3:** Start Task 2.1 implementation
5. **Month 2:** Begin API refactoring
6. **Month 3:** Deploy to staging
7. **Month 4:** Production rollout

