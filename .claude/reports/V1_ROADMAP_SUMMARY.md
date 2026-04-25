# StreamSpace v1.0 → v1.1 Roadmap Summary

**Last Updated:** 2025-11-20
**Status:** v1.0.0-beta → v1.0.0 stable in progress

---

## 📍 Current Status

**Version:** v1.0.0-beta
**Release Status:** Production-ready core, needs testing and plugin completion
**Architecture:** Kubernetes-native (CRD-based controller)

**Audit Verdict (2025-11-20):** ✅ Documentation is remarkably accurate
- Core platform is solid (K8s controller, API, UI, database all verified)
- 87 database tables implemented
- 66,988 lines of API code (higher than claimed)
- Full authentication stack (SAML, OIDC, MFA)
- Plugin framework complete (8,580 lines)

**Audit Report:** See `/docs/CODEBASE_AUDIT_REPORT.md`

---

## 🎯 v1.0.0 Stable Release (Current Focus)

**Target:** 10-12 weeks
**Goal:** Stabilize and complete existing Kubernetes-native platform

### Critical Tasks (P0)

**1. Test Coverage: Controller Tests (2-3 weeks)**
- Expand 4 existing test files in `k8s-controller/controllers/`
- Target: 30-40% → 70%+
- Focus: Error handling, edge cases, hibernation cycles, session lifecycle

**2. Test Coverage: API Handler Tests (3-4 weeks)**
- Add tests for 63 untested handler files in `api/internal/handlers/`
- Target: 10-20% → 70%+
- Focus: Critical paths (sessions, users, auth, quotas)
- Fix existing test build errors

**3. Critical Bug Fixes (Ongoing)**
- Fix bugs discovered during test implementation
- Priority: session lifecycle, authentication, authorization, data integrity

### High Priority Tasks (P1)

**4. Test Coverage: UI Component Tests (2-3 weeks)**
- Add tests for 48 untested components in `ui/src/components/`
- Target: 5% → 70%+
- Focus: Critical user flows
- Vitest already configured with 80% threshold

**5. Plugin Implementation: Top 10 Plugins (4-6 weeks)**
Extract existing handler logic into plugin modules:
1. `streamspace-calendar` (from scheduling.go)
2. `streamspace-slack` (from integrations.go)
3. `streamspace-teams` (from integrations.go)
4. `streamspace-discord` (from integrations.go)
5. `streamspace-pagerduty` (from integrations.go)
6. `streamspace-multi-monitor` (from handlers)
7. `streamspace-snapshots` (extract logic)
8. `streamspace-recording` (extract logic)
9. `streamspace-compliance` (extract logic)
10. `streamspace-dlp` (extract logic)

**6. Template Repository Verification (1-2 weeks)**
- Verify external `streamspace-templates` repository
- Test catalog sync functionality
- Document template repository setup

### v1.0.0 Success Criteria

- [ ] Test coverage reaches 70%+ (controller, API, UI)
- [ ] Top 10 plugins implemented and working
- [ ] Template repository sync verified and documented
- [ ] All critical bugs fixed
- [ ] Documentation updated to reflect reality
- [ ] Security audit complete
- [ ] Performance benchmarks established

**Release Target:** 10-12 weeks from 2025-11-20

---

## 🚀 v1.1.0 Multi-Platform (Deferred)

**Target:** 13-19 weeks after v1.0.0 stable
**Goal:** Platform-agnostic architecture supporting Kubernetes, Docker, and future platforms

**Status:** DEFERRED until v1.0.0 stable release
**Reason:** Current K8s architecture is production-ready. Complete testing and plugins first.

### Phase 1: Control Plane Decoupling (4-6 weeks)

**Goal:** Move from CRD-based to database-backed resource management

- Create `Session` and `Template` database tables (replace CRD dependency)
- Implement `Controller` registration API (WebSocket/gRPC)
- Refactor API to use database instead of K8s client
- Maintain backward compatibility with existing K8s controller

**Benefits:**
- Support non-Kubernetes platforms (Docker, Hyper-V, bare metal)
- Simplified API without K8s client dependency
- Centralized resource management

### Phase 2: K8s Agent Adaptation (3-4 weeks)

**Goal:** Convert K8s controller from CRD reconciler to API agent

- Fork `k8s-controller` to `controllers/k8s`
- Implement Agent loop (connect to Control Plane API, listen for commands)
- Replace CRD status updates with API status reporting
- Test dual-mode operation (CRD + API for migration)

**Benefits:**
- Consistent architecture across all platforms
- Easier to add new platform controllers
- Simplified controller logic (no CRD reconciliation)

### Phase 3: Docker Controller Completion (4-6 weeks)

**Goal:** Functional Docker controller with parity to K8s controller

**Current:** 718 lines, ~10% complete (skeleton only)

- Complete Docker container lifecycle management
- Implement volume management for user storage
- Add network configuration (port mapping, isolation)
- Implement status reporting back to Control Plane API
- Create integration tests
- Support Docker Compose deployment option

**Benefits:**
- Run StreamSpace without Kubernetes
- Support edge/IoT deployments
- Simpler local development setup

### Phase 4: UI Updates for Multi-Platform (2-3 weeks)

**Goal:** Platform-agnostic UI terminology and controls

- Rename "Pod" to "Instance" (platform-agnostic terminology)
- Update "Nodes" view to "Controllers"
- Add platform selector UI (Kubernetes, Docker, etc.)
- Ensure status fields map correctly for all platforms
- Update documentation for multi-platform deployment

**Benefits:**
- Consistent user experience across platforms
- Clear platform selection during session creation

### v1.1.0 Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        Users                                  │
│              (Web Browsers - Any Device)                      │
└────────────────────────┬─────────────────────────────────────┘
                         │ HTTPS
                         ↓
┌──────────────────────────────────────────────────────────────┐
│                   Ingress / Load Balancer                     │
└────────────────────────┬─────────────────────────────────────┘
                         │
          ┌──────────────┴─────────────┐
          ↓                            ↓
┌─────────────────────┐      ┌──────────────────────┐
│   Web UI (React)    │      │   Control Plane (API)│
│  - Dashboard        │      │   - REST API         │
│  - Catalog          │      │   - WebSocket        │
│  - Session viewer   │      │   - PostgreSQL       │
│  - Admin panel      │      │   - Controller Mgmt  │
└─────────────────────┘      └──────────┬───────────┘
                                        │ Secure Protocol (gRPC/WS)
                         ┌──────────────┴──────────────┐
                         ↓                             ↓
┌──────────────────────────────────────┐   ┌──────────────────────────────────────┐
│    Kubernetes Controller (Agent)      │   │      Docker Controller (Agent)       │
│  - Runs on K8s Cluster               │   │  - Runs on Docker Host               │
│  - Manages Pods/PVCs                 │   │  - Manages Containers/Volumes        │
│  - Reports Status via API            │   │  - Reports Status via API            │
└────────────────┬─────────────────────┘   └────────────────┬─────────────────────┘
                 │                                          │
                 ↓                                          ↓
┌──────────────────────────────────────┐   ┌──────────────────────────────────────┐
│         Kubernetes Cluster           │   │            Docker Host               │
│  [Session Pods]                      │   │  [Session Containers]                │
└──────────────────────────────────────┘   └──────────────────────────────────────┘
```

### v1.1.0 Success Criteria

- [ ] API backend uses database instead of K8s CRDs
- [ ] Kubernetes controller operates as Agent (connects to API)
- [ ] Docker controller fully functional (parity with K8s controller)
- [ ] UI supports multiple controller platforms
- [ ] Backward compatibility maintained with v1.0.0 deployments
- [ ] Documentation updated for multi-platform deployment
- [ ] Integration tests pass for both K8s and Docker platforms

**Release Target:** 13-19 weeks after v1.0.0 stable

---

## 🔮 v2.0.0 VNC Independence (Future)

**Target:** 4-6 months after v1.1.0
**Goal:** 100% open-source VNC stack, self-hosted container images

**Status:** Planned, not yet started

### Key Changes

**1. VNC Stack Migration**
- **Current:** LinuxServer.io images with KasmVNC (external dependency)
- **Target:** StreamSpace-native images with TigerVNC + noVNC (100% open source)

**2. Container Image Strategy**
- Build 200+ StreamSpace-native container images
- Set up image build pipeline (GitHub Actions)
- Security scanning with Trivy
- Image signing with Cosign
- Host on ghcr.io/streamspace

**3. Base Image Tiers**
- Tier 1: Core bases (Ubuntu, Alpine, Debian with TigerVNC)
- Tier 2: Applications (browsers, IDEs, design tools - 100+ images)
- Tier 3: Specialized (gaming, scientific, CAD - 50+ images)

### v2.0.0 Success Criteria

- [ ] All base images built with TigerVNC + noVNC
- [ ] 200+ application templates migrated to StreamSpace images
- [ ] Image build pipeline operational
- [ ] Security scanning and signing automated
- [ ] No external image dependencies (except OS base images)
- [ ] Migration guide for v1.x users
- [ ] Performance parity or better than LinuxServer.io images

**Release Target:** 4-6 months after v1.1.0 stable

---

## 📊 Release Timeline

```
2025-11-20: v1.0.0-beta (Current)
    │
    ├─ Test Coverage (6-8 weeks)
    ├─ Plugin Implementation (4-6 weeks)
    ├─ Template Verification (1-2 weeks)
    │
2026-02-03: v1.0.0 Stable Target (10-12 weeks)
    │
    ├─ Control Plane Decoupling (4-6 weeks)
    ├─ K8s Agent Adaptation (3-4 weeks)
    ├─ Docker Controller Completion (4-6 weeks)
    ├─ UI Multi-Platform Updates (2-3 weeks)
    │
2026-05-26: v1.1.0 Multi-Platform Target (13-19 weeks)
    │
    ├─ VNC Stack Migration (8-12 weeks)
    ├─ Image Build Pipeline (4-6 weeks)
    ├─ Template Migration (8-12 weeks)
    │
2026-11-16: v2.0.0 VNC Independence Target (4-6 months)
```

**Total Time to v2.0.0:** ~12 months from 2025-11-20

---

## 🎯 Decision Rationale

### Why v1.0.0 First?

**Architect's Recommendation (2025-11-20):**

1. **Current Architecture Works Well**
   - Kubernetes controller is production-ready (6,562 lines)
   - All reconcilers functioning (Session, Hibernation, Template, ApplicationInstall)
   - Well-tested architecture pattern (Kubebuilder)

2. **Build on Solid Foundation**
   - Fix what's incomplete (tests, plugins) before redesigning
   - Validate current architecture works at scale
   - Gather user feedback on K8s-native deployment

3. **Risk Management**
   - Architecture redesign is high-risk, high-effort
   - Complete Docker controller BEFORE abstracting architecture
   - Ensure v1.0.0 is stable before major changes

4. **User Value**
   - Users need working platform NOW (K8s is most common)
   - Tests and plugins deliver immediate value
   - Multi-platform support can wait for v1.1

### Why Defer Multi-Platform?

**Don't fix what isn't broken.**

The Kubernetes-native architecture is:
- ✅ Production-ready and working
- ✅ Well-documented and maintainable
- ✅ Using proven patterns (Kubebuilder, CRDs)
- ✅ Sufficient for majority of users (K8s is standard)

Complete Docker controller FIRST, then abstract if patterns emerge.

---

## 📚 Related Documentation

- **Codebase Audit Report:** `/docs/CODEBASE_AUDIT_REPORT.md`
- **Multi-Agent Plan:** `.claude/multi-agent/MULTI_AGENT_PLAN.md`
- **Feature Status:** `FEATURES.md`
- **Current Roadmap:** `ROADMAP.md`
- **Architecture Details:** `docs/ARCHITECTURE.md`
- **Contributing Guide:** `CONTRIBUTING.md`

---

**Document Maintained By:** Agent 1 (Architect)
**Next Review:** After v1.0.0 stable release
