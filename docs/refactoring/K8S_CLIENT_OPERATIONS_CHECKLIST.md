# K8sClient Operations Migration Checklist

## Operations to Move to Controller

### Session Operations (HIGH PRIORITY)
| Operation | Current File | Method | Action | Target |
|-----------|--------------|--------|--------|--------|
| Create Session | handlers.go:464 | `CreateSession()` | MOVE | SessionReconciler |
| Update State | handlers.go:500 | `UpdateSessionState()` | MOVE | SessionReconciler |
| Delete Session | handlers.go:528 | `DeleteSession()` | MOVE | SessionReconciler |
| List for idle check | activity.go:196 | `ListSessions()` | MOVE | IdleReconciler |
| Update to hibernated | activity.go:232 | `UpdateSession()` | MOVE | IdleReconciler |
| Auto-start session | tracker.go:463 | `UpdateSessionState()` | MOVE | AutoStartReconciler |
| Auto-hibernate session | tracker.go:512 | `UpdateSessionState()` | MOVE | IdleReconciler |

### Node Operations (MEDIUM PRIORITY)
| Operation | Current File | Method | Action | Target |
|-----------|--------------|--------|--------|--------|
| Patch labels | nodes.go:241,267 | `PatchNode()` | MOVE | NodeOpsReconciler |
| Patch taints | nodes.go:313 | `PatchNode()` | MOVE | NodeOpsReconciler |
| Cordon node | nodes.go:383 | `CordonNode()` | MOVE | NodeOpsReconciler |
| Uncordon node | nodes.go:405 | `UncordonNode()` | MOVE | NodeOpsReconciler |
| Drain node | nodes.go:435 | `DrainNode()` | MOVE | NodeOpsReconciler |
| Update taints | nodes.go:361 | `UpdateNodeTaints()` | MOVE | NodeOpsReconciler |

### Other Operations to Move
| Operation | Current File | Method | Action | Target |
|-----------|--------------|--------|--------|--------|
| Quota validation | handlers.go:423 | `CheckSessionCreation()` | MOVE | SessionValidator Webhook |
| Pod eviction | nodes.go:435 | `DrainNode()` | MOVE | NodeOpsReconciler |
| ConfigMap updates | stubs.go:624,636 | `ConfigMaps().Create/Update()` | MOVE | ConfigReconciler |
| Dynamic resource create | stubs.go:340 | `GetDynamicClient().Create()` | MOVE | ResourceWebhook |
| Dynamic resource update | stubs.go:414 | `GetDynamicClient().Update()` | MOVE | ResourceWebhook |
| Dynamic resource delete | stubs.go:459 | `GetDynamicClient().Delete()` | MOVE | ResourceWebhook |

---

## Operations to Keep in API

### Read-Only Monitoring
| Operation | Current File | Method | Reason | Keep |
|-----------|--------------|--------|--------|------|
| List Sessions | handlers.go:259-261 | `ListSessions()` | Read-only query | ✅ |
| Get Session | handlers.go:284 | `GetSession()` | Read-only query | ✅ |
| List Templates | handlers.go:762-764 | `ListTemplates()` | Catalog lookup | ✅ |
| Get Template | handlers.go:884 | `GetTemplate()` | Template validation | ✅ |
| List Nodes | nodes.go:156 | `GetNodes()` | Monitoring | ✅ |
| Get Node | nodes.go:187 | `GetNode()` | Node status | ✅ |
| List Pods | stubs.go:239 | `GetPods()` | Monitoring | ✅ |
| List Deployments | stubs.go:254 | Clientset.Deployments() | Monitoring | ✅ |
| List Services | stubs.go:269 | `GetServices()` | Monitoring | ✅ |
| List Namespaces | stubs.go:279 | `GetNamespaces()` | Monitoring | ✅ |
| Get cluster stats | nodes.go:214 | `calculateClusterStats()` | Dashboard | ✅ |

### Real-Time Operations
| Operation | Current File | Method | Reason | Keep |
|-----------|--------------|--------|--------|------|
| Heartbeat update | activity.go:121 | `UpdateSessionActivity()` | Low-latency | ✅ |
| Activity status | handlers.go (implied) | `GetActivityStatus()` | Real-time | ✅ |
| Broadcast sessions | websocket.go:227 | `ListSessions()` | WebSocket stream | ✅ |
| Stream pod logs | websocket.go:181 | `GetLogs()` | Real-time logs | ✅ |

### Administrative Triggers
| Operation | Current File | Method | Reason | Keep |
|-----------|--------------|--------|--------|------|
| Install application | applications.go:221 | `CreateApplicationInstall()` | Request trigger | ✅ |
| Create Template | handlers.go:906 | `CreateTemplate()` | One-time setup | ✅ |
| Delete Template | handlers.go:921 | `DeleteTemplate()` | Admin operation | ✅ |
| Get ConfigMap | stubs.go:573,608 | `ConfigMaps().Get()` | Read config | ✅ |
| Get template config | applications.go | N/A | Admin query | ✅ |

---

## Files Summary

### Files to SIGNIFICANTLY REDUCE
```
api/internal/api/handlers.go
  Current: ~2000 LOC, 50+ k8s operations
  After:   ~800 LOC, 15+ k8s operations (all read-only)
  Removed: Session CRUD, state transitions, quota checks, pod queries
  
api/internal/activity/tracker.go
  Current: ~300 LOC, 4 k8s operations
  After:   ~100 LOC, 1 k8s operation (heartbeat endpoint only)
  Removed: IdleMonitor loop, hibernation logic
  
api/internal/handlers/nodes.go
  Current: ~600 LOC, 9 k8s operations
  After:   ~200 LOC, 2 k8s operations (list, get)
  Removed: Patch, cordon, uncordon, drain operations
```

### Files to DELETE
```
api/internal/tracker/tracker.go
  Entire file: ~500 LOC, 2 k8s operations
  Reason: All auto-start/hibernate logic moves to controller
  Keep: Connection DB tracking (no k8s operations)
```

### Files to CREATE
```
controller/internal/controllers/session_controller.go
  New: ~800 LOC, 5+ k8s operations
  Reconciles: Session CRD → Deployment, PVC, state machine
  
controller/internal/controllers/idle_reconciler.go
  New: ~300 LOC, 2 k8s operations
  Reconciles: Idle sessions → hibernation
  
controller/internal/controllers/autostart_reconciler.go
  New: ~300 LOC, 1 k8s operation
  Reconciles: Connection events → auto-start
  
controller/internal/controllers/nodeops_reconciler.go
  New: ~600 LOC, 6 k8s operations
  Reconciles: NodeOperation CR → node patches, cordon, drain
  
controller/internal/webhooks/session_validator.go
  New: ~200 LOC, 0 k8s operations (quota check only)
  Validates: Session creation against quota
```

---

## Migration Order (Phased Approach)

### Phase 1: Design (Weeks 1-2)
- [ ] Finalize Session state machine
- [ ] Design IdleDetection reconciler
- [ ] Design ConnectionTracking webhook
- [ ] Design quota ValidatingWebhook
- [ ] Design NodeOperation CRD

### Phase 2a: Session Lifecycle (Weeks 3-5)
- [ ] Implement SessionReconciler
  - [ ] Handle Pending → Running transition
  - [ ] Create Deployment
  - [ ] Create PVC
  - [ ] Handle Terminated cleanup
- [ ] Add comprehensive tests
- [ ] E2E test: Create session → Running

### Phase 2b: Idle Detection (Weeks 5-7)
- [ ] Implement IdleReconciler
  - [ ] Watch lastActivity timestamp
  - [ ] Detect idle sessions
  - [ ] Hibernate sessions (update state)
- [ ] Remove activity tracker background loop
- [ ] Keep heartbeat endpoint (update lastActivity)
- [ ] E2E test: Idle after 30m → hibernated

### Phase 2c: Auto-Start (Weeks 7-8)
- [ ] Design connection event webhook
- [ ] Implement AutoStartReconciler
  - [ ] Listen for connection events
  - [ ] Auto-start hibernated sessions
- [ ] Remove tracker auto-start logic
- [ ] E2E test: Connect to hibernated → running

### Phase 2d: Node Operations (Weeks 8-9)
- [ ] Create NodeOperation CRD
- [ ] Implement NodeOpsReconciler
  - [ ] Handle cordon/uncordon
  - [ ] Handle drain
  - [ ] Handle label/taint patches
- [ ] Remove node operation methods from API
- [ ] E2E test: Cordon via API → node unschedulable

### Phase 2e: Testing (Weeks 9-10)
- [ ] Integration tests for all reconcilers
- [ ] Failure scenario testing
- [ ] Performance testing
- [ ] State consistency verification

### Phase 3a: API Refactoring (Weeks 11-13)
- [ ] Remove CreateSession implementation
- [ ] Remove UpdateSessionState logic
- [ ] Remove DeleteSession logic
- [ ] Remove node state-change operations
- [ ] Remove tracker.go entirely
- [ ] Keep read-only endpoints

### Phase 3b: Quota Migration (Weeks 13-14)
- [ ] Implement SessionValidator webhook
- [ ] Remove quota checks from CreateSession
- [ ] Verify webhook rejects over-quota
- [ ] Add feature flag for fallback

### Phase 3c: Testing & Documentation (Weeks 14-16)
- [ ] Integration tests: API + controller
- [ ] Update documentation
- [ ] Create migration guide
- [ ] Prepare rollout plan

---

## Testing Strategy

### Unit Tests
| Component | Test File | Target Coverage |
|-----------|-----------|-----------------|
| SessionReconciler | controller/controllers/session_reconciler_test.go | 85%+ |
| IdleReconciler | controller/controllers/idle_reconciler_test.go | 85%+ |
| AutoStartReconciler | controller/controllers/autostart_reconciler_test.go | 85%+ |
| NodeOpsReconciler | controller/controllers/nodeops_reconciler_test.go | 85%+ |
| SessionValidator | controller/webhooks/session_validator_test.go | 90%+ |

### Integration Tests
| Scenario | Test File | Expected Result |
|----------|-----------|-----------------|
| Create session → Running | integration_test.go | Session in Running state, Deployment exists |
| Idle detection → Hibernated | integration_test.go | Session hibernated after idle timeout |
| Connection → Auto-start | integration_test.go | Session transitions Hibernated → Running |
| Node operations | integration_test.go | Node cordon/drain/label applied |
| Quota validation | integration_test.go | Over-quota session rejected by webhook |

### E2E Tests
| Scenario | Expected | Success Criteria |
|----------|----------|-----------------|
| User creates session | Session Running in <5s | Deployment, PVC, Service created |
| Session idle for 30m | Session Hibernated | State changed, pods scaled to 0 |
| User connects to idle session | Session auto-starts | State Running, pods scaled to 1 |
| Admin drains node | Node drained | Sessions moved, node unschedulable |
| User over quota | Session rejected | Webhook returns 403 |

---

## Verification Checklist

### Before Phase 2 Starts
- [ ] All 12 files analyzed and documented
- [ ] k8s operations categorized (move vs keep)
- [ ] Controller design approved by team
- [ ] CRD updates planned
- [ ] Risk mitigation strategies agreed

### Before Phase 3 Starts
- [ ] All controller reconcilers working
- [ ] 100+ integration tests passing
- [ ] API heartbeat endpoint latency acceptable
- [ ] No regressions vs current behavior
- [ ] Rollback plan tested

### Before Production Rollout
- [ ] All tests passing on staging
- [ ] Load testing completed
- [ ] Operator runbook prepared
- [ ] Rollback procedure tested
- [ ] Communication sent to users

---

## Rollback Decision Points

| Phase | Decision | Go/No-Go |
|-------|----------|----------|
| Design | Controller design viable | |
| Phase 2a | SessionReconciler working | |
| Phase 2b | IdleReconciler working | |
| Phase 2c | AutoStartReconciler working | |
| Phase 2d | NodeOpsReconciler working | |
| Phase 2e | All integration tests pass | |
| Phase 3a | API refactoring complete | |
| Phase 3b | Webhook quota validation works | |
| Phase 3c | Staging deployment successful | |
| Rollout | Production deployment successful | |

---

## Success Indicators

✅ Session creation moves from 200ms (API) to 50ms (webhook) + async controller
✅ Idle detection moves from memory-based to CRD-based (persistent)
✅ Auto-start moves from in-process to event-driven (scalable)
✅ Node operations move from API to controller (proper separation)
✅ All tests passing (100+ integration, 500+ unit)
✅ Code duplication reduced (tracker.go deleted)
✅ Controller responsibility clear (state machine)
✅ API responsibility clear (query + trigger)

