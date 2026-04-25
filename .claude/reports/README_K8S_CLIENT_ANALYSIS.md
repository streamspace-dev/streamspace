# K8sClient Refactoring Analysis - README

This directory contains three comprehensive documents analyzing k8sClient usage in the StreamSpace API and planning the migration to a controller-based architecture.

## Documents Overview

### 1. **K8S_CLIENT_REFACTORING_ANALYSIS.md** (Main Analysis - 21KB)
**Detailed technical analysis of all k8sClient usages**

Contains:
- Complete analysis of 12 files using k8sClient
- 50+ K8s operations catalogued by type and resource
- Per-handler breakdown with code examples
- Recommendations for each file (stay in API vs move to controller)
- Summary tables and reference information

**Best for:**
- Understanding current state
- Finding where specific operations are used
- Making refactoring decisions

**Key Findings:**
- 50+ K8s operations across 12 files
- 15+ should move to controller (state transitions, persistence)
- 20+ should stay in API (read-only, real-time)
- 3+ support operations (administrative triggers)

---

### 2. **K8S_CLIENT_REFACTORING_ROADMAP.md** (Timeline & Plan - 25KB)
**Phased refactoring plan with tasks, timeline, and risk mitigation**

Contains:
- 3-phase roadmap (16 weeks total)
- 15+ specific tasks with acceptance criteria
- File-by-file migration mapping
- Risk analysis and mitigation strategies
- Success metrics and rollback plans
- Communication and deployment strategy

**Phases:**
- **Phase 1 (Weeks 1-2):** Design controller reconcilers and webhooks
- **Phase 2 (Weeks 3-10):** Implement 4 new controllers
- **Phase 3 (Weeks 11-16):** Refactor API and migrate to production

**Best for:**
- Planning the refactoring work
- Estimating effort and timeline
- Understanding interdependencies
- Risk assessment

---

### 3. **K8S_CLIENT_OPERATIONS_CHECKLIST.md** (Execution Guide - 10KB)
**Operational checklist for moving specific K8s operations**

Contains:
- Operations to move to controller (with line numbers)
- Operations to keep in API (with reasons)
- File reduction summary
- New files to create
- Phased implementation order
- Testing strategy
- Verification checklists

**Best for:**
- Day-to-day execution
- Tracking which operations have been migrated
- Testing strategy
- Verification at each phase

---

## Quick Start

### For Managers/Leads
1. Read: **ROADMAP** (Executive Summary section)
2. Reference: **ANALYSIS** (Summary Table for priorities)
3. Plan: Use **ROADMAP** (Phases 1-3) for timeline

### For Developers
1. Start: **ANALYSIS** (Your specific file section)
2. Design: **ROADMAP** (Corresponding task description)
3. Execute: **CHECKLIST** (Specific operations to move)
4. Test: **CHECKLIST** (Testing strategy section)

### For Architects
1. Deep dive: **ANALYSIS** (Detailed File Analysis section)
2. Validate: **ROADMAP** (Task-specific designs)
3. Risk review: **ROADMAP** (Risk Mitigation section)
4. Approve: Use decision points in **CHECKLIST**

---

## Key Insights

### Current Problems
- **Scattered logic:** Session state transitions in API + activity tracker + connection tracker
- **Duplication:** Idle detection and auto-hibernation logic in two places
- **Implicit ordering:** API creates deployment, controller manages pod, no state coordination
- **Scalability:** In-process memory tracking (tracker.go) doesn't work at scale
- **Testing:** Hard to test K8s operations without full cluster

### Proposed Solution
- **Controller-driven:** All state transitions in controller (source of truth)
- **Event-driven:** API signals controller via CRD fields
- **Webhook validation:** Quota checks at admission time (no duplicated logic)
- **Async operations:** API returns 202 Accepted, client polls for status
- **Persistent state:** All state in CRD, survives controller restarts

### Expected Outcomes
- Session create from 200ms API call → 50ms webhook + async controller
- Idle detection from memory-based → CRD-based (survives restarts)
- Auto-start from in-process loop → event-driven (scales horizontally)
- Node ops from direct API calls → controller reconciliation
- Code size: API reduced 60% (state logic removed)

---

## File Analysis Summary

| File | Current State | Target State | Priority | Effort |
|------|----------------|--------------|----------|--------|
| **api/cmd/main.go** | k8s init | Stay same | - | 0h |
| **api/internal/api/handlers.go** | 50+ ops | 15 ops | HIGH | 40h |
| **api/internal/api/stubs.go** | 20+ ops | 10 ops | MEDIUM | 30h |
| **api/internal/handlers/applications.go** | 1 op | Stay same | - | 0h |
| **api/internal/handlers/nodes.go** | 9 ops | 2 ops | MEDIUM | 20h |
| **api/internal/handlers/dashboard.go** | 1 op | Stay same | - | 0h |
| **api/internal/handlers/activity.go** | 2 ops | 1 op | HIGH | 10h |
| **api/internal/activity/tracker.go** | 4 ops | 1 op | HIGH | 15h |
| **api/internal/tracker/tracker.go** | 2 ops | DELETE | HIGH | 5h |
| **api/internal/websocket/handlers.go** | 2 ops | Stay same | - | 0h |
| **NEW: controller/session_controller.go** | - | Create | HIGH | 50h |
| **NEW: controller/idle_reconciler.go** | - | Create | HIGH | 20h |
| **NEW: controller/autostart_reconciler.go** | - | Create | MEDIUM | 15h |
| **NEW: controller/nodeops_reconciler.go** | - | Create | MEDIUM | 30h |
| **NEW: controller/webhooks/session_validator.go** | - | Create | HIGH | 15h |

**Total Effort:** ~250 hours (15-20 developer weeks)

---

## Operations by Type

### CREATE Operations (8 total)
```
Sessions:     Create CRD (API keeps, controller creates pod)
Templates:    Create CRD (API keeps)
AppInstall:   Create CRD (API keeps - trigger)
ConfigMaps:   Create (move to controller)
Generic:      Create via dynamic client (move to webhook)
```

### READ Operations (35+ total)
```
List:         Sessions, Templates, Nodes, Pods, Deployments, Services, Namespaces
Get:          Sessions, Templates, Nodes, Pods, ConfigMaps
Logs:         Pod logs streaming (keep in API for real-time)
```

### UPDATE Operations (18 total)
```
Session State:    (Move to controller)
Node Labels:      (Move to controller)
Node Taints:      (Move to controller)
ConfigMaps:       (Move to controller)
Generic Resources:(Move to webhook)
```

### DELETE Operations (6 total)
```
Sessions:     (Move to controller)
Templates:    (API keeps for cleanup)
Nodes (drain):(Move to controller)
Generic:      (Move to webhook)
```

### SPECIAL Operations
```
Patch:        Node patches (labels, taints) - move to controller
Drain:        Pod eviction - move to controller
Heartbeat:    Activity tracking - keep in API (real-time)
```

---

## Architecture Changes

### Before (Current)
```
API Handler                              Controller (Kubebuilder)
├── CreateSession                        ├── Watch Session CRD
│   ├── Create Session CRD              └── Create Deployment/PVC
│   └── Wait (BLOCKING)
│
├── UpdateSessionState (DIRECT)          
│   └── Update Session.Spec.State
│
├── DeleteSession                        
│   └── Delete Session CRD (cascade)
│
├── Activity Tracker (background)        
│   └── Hibernation logic (IMPLICIT)
│
└── Connection Tracker (background)      
    └── Auto-start logic (IMPLICIT)
```

### After (Proposed)
```
API Handler (HTTP)              WebSocket                  Admission Controller
├── CreateSession               
│   ├── Create Session CRD (Pending)
│   └── Return 202 Accepted    
│       └── Client polls for status
│
├── ListSessions (read-only)    
│
├── RecordHeartbeat                                       ← Update lastActivity
│   └── Update Session.Status.LastActivity               
│
└── Connection Events           
    └── Webhook:Connected()      
                                                         Controller (Reconcilers)
                                                         ├── SessionReconciler
                                                         │   └── Pending→Running
                                                         │       Create Deployment/PVC
                                                         │
                                                         ├── IdleReconciler
                                                         │   └── Watch lastActivity
                                                         │       Hibernated (scale 0)
                                                         │
                                                         ├── AutoStartReconciler
                                                         │   └── Connection event
                                                         │       Running (scale 1)
                                                         │
                                                         └── NodeOpsReconciler
                                                             └── Cordon/Drain/Labels
                                                         
                                                         ValidatingWebhook
                                                         └── Quota validation
                                                            Session creation check
```

---

## Next Steps

### Immediate (This Week)
1. Review analysis documents with architecture team
2. Approve design approach
3. Schedule design review for Phase 1 tasks
4. Create tracking tickets

### Short Term (Next Month)
1. Complete Phase 1 design
2. Begin Phase 2a (SessionReconciler)
3. Set up test infrastructure
4. Create design documentation

### Medium Term (2-3 Months)
1. Complete Phase 2 (all 4 reconcilers)
2. Begin Phase 3 (API refactoring)
3. Deploy to staging
4. Load testing

### Long Term (3-4 Months)
1. Production rollout
2. Monitor metrics
3. Gather feedback
4. Plan next iteration

---

## Key Decision Points

| Question | Analysis Answer | Next Action |
|----------|-----------------|-------------|
| Should session state move to controller? | YES - state consistency | Implement SessionReconciler |
| Keep API heartbeat endpoint? | YES - must be low-latency | Keep activity.UpdateSessionActivity() |
| When to move quota checks? | AFTER webhook design | Plan SessionValidator |
| Should tracker.go be deleted? | YES - logic in controller | Plan deletion in Phase 3a |
| Can node ops stay in API? | NO - infrastructure logic | Plan NodeOpsReconciler |

---

## Documents Checklist

- [x] K8S_CLIENT_REFACTORING_ANALYSIS.md - Complete technical analysis
- [x] K8S_CLIENT_REFACTORING_ROADMAP.md - Phased implementation plan
- [x] K8S_CLIENT_OPERATIONS_CHECKLIST.md - Day-to-day execution guide
- [x] README_K8S_CLIENT_ANALYSIS.md - This overview document

## Related Documents to Update

After using this analysis, update:
- [ ] CLAUDE.md - Add controller reconciler patterns
- [ ] ROADMAP.md - Phase 6 plan references
- [ ] docs/ARCHITECTURE.md - Add controller architecture diagrams
- [ ] docs/CONTROLLER_GUIDE.md - Add reconciler patterns

---

## Support & Questions

For questions about:
- **Specific operations**: See K8S_CLIENT_REFACTORING_ANALYSIS.md
- **Timeline/Planning**: See K8S_CLIENT_REFACTORING_ROADMAP.md
- **Execution**: See K8S_CLIENT_OPERATIONS_CHECKLIST.md
- **Architecture decisions**: Review all three documents and discussion in ROADMAP.md Risk Mitigation section

---

**Analysis Completed:** 2025-11-19  
**Status:** Ready for team review and planning  
**Estimated Effort:** 250 hours / 15-20 developer weeks  
**Risk Level:** Medium (requires careful state machine design)

