# CLAUDE.md - AI Assistant Guide for StreamSpace

**Last Updated**: 2025-11-20
**Project Version**: v1.0.0-beta (45% complete)
**Development Model**: Multi-agent workflow (Architect, Builder, Validator, Scribe)

---

## 📋 Quick Reference

### Current Status (v1.0.0 Development)

**Progress**: Week 2-3 of 10-12 weeks (45% complete)

**✅ Completed:**

- Comprehensive codebase audit (Architect)
- All 3 P0 admin features (Builder):
  - Audit Logs Viewer (1,131 lines)
  - System Configuration (938 lines)
  - License Management (1,814 lines)
- API Keys Management - P1 (Builder, 1,217 lines)
- Controller test coverage expansion (Validator, 1,565 lines, +32 test cases)
- Comprehensive documentation (Scribe, 3,600+ lines)

**🔄 In Progress:**

- API handler test coverage (Validator, next P0 task)
- Remaining P1 admin features (Builder)

**📋 Next Priorities:**

1. Complete test coverage: API handlers (P0), UI components (P1)
2. Complete P1 admin features: Alert Management, Controller Management, Session Recordings
3. Implement top 10 plugins by extracting handler logic
4. Verify template repository sync

---

## 🎯 Project Overview

**StreamSpace** is a Kubernetes-native container streaming platform that delivers GUI applications to web browsers.

**Key Features:**

- Browser-based access to containerized applications
- Multi-user support with enterprise SSO (SAML, OIDC, MFA)
- Auto-hibernation for resource efficiency
- Comprehensive admin UI with audit logs and license management
- 200+ application templates (external repository)
- Plugin system for extensibility

**Current Architecture:**

- **Kubernetes-native**: Production-ready K8s controller with CRDs
- **API Backend**: Go/Gin with 70+ handlers, 87 database tables
- **Web UI**: React/TypeScript with 50+ components
- **VNC Stack**: Currently LinuxServer.io images (migration to TigerVNC + noVNC planned for v2.0)

---

## 📁 Repository Structure

```
streamspace/
├── .claude/multi-agent/         # Multi-agent coordination files
│   └── MULTI_AGENT_PLAN.md     # Central coordination document
├── api/                         # Go API backend (REST + WebSocket)
│   ├── internal/handlers/      # 70+ API handlers
│   ├── internal/middleware/    # 15+ middleware layers
│   └── internal/db/            # Database (87 tables)
├── agents/                      # Platform agents
│   └── k8s-agent/               # Kubernetes agent (WebSocket-based)
├── ui/                         # React web UI
│   ├── src/pages/admin/        # Admin portal (12+ pages)
│   └── src/components/         # Reusable components
├── manifests/                  # Kubernetes manifests
│   ├── crds/                   # Session and Template CRDs
│   └── config/                 # Deployment configurations
├── docs/                       # Technical documentation
│   ├── CODEBASE_AUDIT_REPORT.md
│   ├── ADMIN_UI_GAP_ANALYSIS.md
│   ├── TESTING_GUIDE.md
│   └── ADMIN_UI_IMPLEMENTATION.md
└── chart/                      # Helm chart
```

---

## 🤖 Multi-Agent Development Workflow

StreamSpace uses a **4-agent development model** for parallel work:

### Agent Roles

**Agent 1: Architect** (Research & Planning)

- Codebase exploration and analysis
- Feature gap identification
- Architecture planning and design decisions
- **Branch**: `claude/audit-streamspace-codebase-*`

**Agent 2: Builder** (Feature Implementation)

- New features (admin UI, API handlers, controllers)
- Bug fixes and refactoring
- **Does NOT write tests** (that's Validator's job)
- **Branch**: `claude/setup-agent2-builder-*`

**Agent 3: Validator** (Testing & QA)

- Unit tests, integration tests, E2E tests
- Test coverage expansion (goal: 70%+)
- Quality assurance and bug discovery
- **Branch**: `claude/setup-agent3-validator-*`

**Agent 4: Scribe** (Documentation)

- Technical documentation
- API references, deployment guides
- Testing guides and implementation guides
- **Branch**: `claude/setup-agent4-scribe-*`

### Coordination

**Central Document**: `.claude/multi-agent/MULTI_AGENT_PLAN.md`

- All agents update this file with progress
- Task assignments and status tracking
- Integration notes and blockers

**Integration Process**:

1. Agents work independently on their branches
2. Architect periodically pulls and merges all agent work
3. Conflicts resolved, progress documented
4. Changes pushed to Architect's branch

---

## 🎯 Key Technologies

**Backend:**

- Go 1.21+ with Gin framework
- PostgreSQL (87 tables)
- Kubernetes controller (Kubebuilder 3.x)

**Frontend:**

- React 18+ with TypeScript
- Material-UI (MUI)
- React Router, Axios

**Infrastructure:**

- Kubernetes 1.19+ (k3s recommended)
- NFS storage (ReadWriteMany)
- Traefik ingress

**Testing:**

- Controller: Ginkgo/Gomega + envtest
- API: Go standard testing + sqlmock
- UI: Vitest (configured for 80% threshold)

---

## 🔑 Custom Resource Definitions (CRDs)

### Session CRD (`stream.space/v1alpha1`)

Represents a user's containerized session.

```yaml
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: user1-firefox
  namespace: streamspace
spec:
  user: user1
  template: firefox-browser
  state: running  # running | hibernated | terminated
  resources:
    memory: 2Gi
    cpu: 1000m
  persistentHome: true
  idleTimeout: 30m
```

**Short names**: `ss`, `sessions`

### Template CRD (`stream.space/v1alpha1`)

Defines an application template.

```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
  namespace: streamspace
spec:
  displayName: Firefox Web Browser
  category: Web Browsers
  baseImage: lscr.io/linuxserver/firefox:latest
  defaultResources:
    memory: 2Gi
    cpu: 1000m
  vnc:
    enabled: true
    port: 3000
```

**Short names**: `tpl`, `templates`

---

## 📝 Git Conventions

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

**Examples:**

```bash
feat(admin-ui): implement audit logs viewer with CSV export
fix(controller): handle session deletion during reconciliation
test(controller): add concurrent operation tests for session controller
docs(architecture): update data flow diagrams for v1.0.0
```

### Branch Naming

**Multi-agent branches**:

- `claude/audit-streamspace-codebase-*` - Architect
- `claude/setup-agent2-builder-*` - Builder
- `claude/setup-agent3-validator-*` - Validator
- `claude/setup-agent4-scribe-*` - Scribe

**Important**: Always push to branches starting with `claude/` and ending with session ID, otherwise push will fail with 403.

---

## 🧪 Testing

### Run Tests

```bash
# K8s Agent tests
cd agents/k8s-agent
go test ./... -v

# Check coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

### Run API Tests

```bash
cd api
go test ./... -v
```

### Run UI Tests

```bash
cd ui
npm test
npm test -- --coverage
```

### Current Test Coverage

- **Controller**: 65-70% (32 new test cases added)
- **API**: 10-20% (expansion in progress)
- **UI**: 5% (expansion planned)
- **Target**: 70%+ for all

---

## 🚀 Key Commands

### Kubernetes Operations

```bash
# List sessions
kubectl get sessions -n streamspace
kubectl get ss -n streamspace  # short name

# List templates
kubectl get templates -n streamspace
kubectl get tpl -n streamspace  # short name

# Create a session
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: test-firefox
  namespace: streamspace
spec:
  user: testuser
  template: firefox-browser
  state: running
EOF

# Check session status
kubectl describe session test-firefox -n streamspace

# Delete session
kubectl delete session test-firefox -n streamspace
```

### Development

```bash
# Install CRDs
kubectl apply -f manifests/crds/

# Run K8s agent locally (for testing)
cd agents/k8s-agent
go run . --api-url=http://localhost:8000

# Deploy full platform
helm install streamspace ./chart -n streamspace --create-namespace
```

---

## 📚 Important Documentation

### For Current Development

- **Multi-Agent Plan**: `.claude/multi-agent/MULTI_AGENT_PLAN.md` - Central coordination
- **Codebase Audit**: `docs/CODEBASE_AUDIT_REPORT.md` - Comprehensive audit results
- **Admin UI Gaps**: `docs/ADMIN_UI_GAP_ANALYSIS.md` - Missing features identified
- **Testing Guide**: `docs/TESTING_GUIDE.md` - For Validator (1,186 lines)
- **Implementation Guide**: `docs/ADMIN_UI_IMPLEMENTATION.md` - For Builder (1,446 lines)

### For Architecture

- **Architecture**: `docs/ARCHITECTURE.md` - Complete system architecture
- **Controller Guide**: `docs/CONTROLLER_GUIDE.md` - Kubebuilder implementation
- **Plugin API**: `docs/PLUGIN_API.md` - Plugin system reference

### Changelogs

- **CHANGELOG.md**: User-facing changes and milestones
- **FEATURES.md**: Complete feature list with implementation status
- **ROADMAP.md**: Development roadmap (v1.0.0 → v2.0.0)

---

## 🎯 Current v1.0.0 Priorities

### Completed ✅

1. **Codebase Audit**: Verified documentation accuracy (87 tables, 66K+ lines API code)
2. **P0 Admin Features** (100%):
   - Audit Logs Viewer (SOC2/HIPAA/GDPR compliance)
   - System Configuration (7 categories, full config UI)
   - License Management (Community/Pro/Enterprise tiers)
3. **P1 Admin Features** (25%):
   - API Keys Management (scope-based access, rate limiting)
4. **Controller Tests**: 65-70% coverage (+32 test cases, 1,565 lines)

### In Progress 🔄

1. **API Handler Tests** (Validator, P0, 3-4 weeks)
2. **UI Component Tests** (Validator, P1, 2-3 weeks)
3. **Remaining P1 Admin Features** (Builder):
   - Alert Management (2-3 days)
   - Controller Management (3-4 days)
   - Session Recordings Viewer (4-5 days)

### Next Up 📋

1. **Plugin Implementation** (Builder, P1, 4-6 weeks):
   - Extract logic from 10 handlers into plugins
   - Calendar, Slack, Teams, Discord, PagerDuty, Multi-Monitor, Snapshots, Recording, Compliance, DLP
2. **Template Repository Verification** (P1, 1-2 weeks)
3. **Bug Fixes** (Ongoing, discovered during testing)

---

## 🚨 Critical Context for AI Assistants

### What's Actually Implemented

**Database**: 87 tables (verified)
**API Backend**: 66,988 lines (70+ handlers)
**Kubernetes Controller**: 6,562 lines (production-ready)
**Web UI**: 54 components/pages (complete)
**Authentication**: SAML, OIDC, MFA, JWT (all implemented)
**Admin UI**: 4/8 features complete (all P0 done, 1 P1 done)

### What's NOT Implemented

**Plugins**: Framework complete (8,580 lines), but 28 plugin implementations are stubs
**Docker Controller**: 718 lines (not functional, deferred to v1.1)
**Test Coverage**: 15-20% overall (being expanded to 70%+)
**Templates**: 1 local template, external repo dependency (needs verification)

### Production Readiness

✅ **CAN Deploy**: Full admin UI, config management, audit logs
✅ **CAN Commercialize**: License enforcement with 3 tiers
✅ **CAN Pass Audits**: SOC2, HIPAA, GDPR, ISO 27001 support
⚠️ **Test Coverage**: Needs expansion before production (in progress)

---

## 🔧 Common Issues & Solutions

### Controller Issues

**Problem**: Session stuck in Pending

```bash
# Check controller logs
kubectl logs -n streamspace deploy/streamspace-controller -f

# Check pod events
kubectl get events -n streamspace --sort-by=.metadata.creationTimestamp
```

### CRD Issues

**Problem**: CRD not found

```bash
# Install CRDs
kubectl apply -f manifests/crds/
```

### Storage Issues

**Problem**: PVC stuck in Pending

```bash
# Check storage class exists
kubectl get storageclass

# Verify NFS provisioner is running
kubectl get pods -n kube-system | grep nfs
```

---

## 📞 Key References

- **GitHub**: <https://github.com/streamspace-dev/streamspace>
- **Website**: <https://streamspace.dev>
- **Kubebuilder**: <https://book.kubebuilder.io/>
- **Kubernetes CRDs**: <https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/>
- **LinuxServer.io**: <https://docs.linuxserver.io/> (temporary, migrating away)

---

## 🎉 v1.0.0 Milestones Achieved

- ✅ Production deployment enabled (full config UI)
- ✅ Commercialization enabled (license enforcement)
- ✅ Compliance ready (audit logs for SOC2/HIPAA/GDPR)
- ✅ API automation enabled (API keys with scopes)
- ✅ Test quality improved (controller coverage doubled)

**Next Milestone**: Complete all test coverage expansion (API + UI tests) for stable release.
