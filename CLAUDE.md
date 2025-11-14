# CLAUDE.md - AI Assistant Guide for StreamSpace

This document provides comprehensive guidance for AI assistants working with the StreamSpace codebase.

**Last Updated**: 2025-11-14
**Project Version**: v0.1.0 (Phase 1 - Planning Complete)

---

## 📋 Table of Contents

- [Project Overview](#project-overview)
- [Strategic Vision: Independence from Proprietary Technologies](#strategic-vision-independence-from-proprietary-technologies)
- [Repository Structure](#repository-structure)
- [Key Technologies](#key-technologies)
- [Custom Resource Definitions (CRDs)](#custom-resource-definitions-crds)
- [Development Workflows](#development-workflows)
- [Git Conventions](#git-conventions)
- [Testing Guidelines](#testing-guidelines)
- [Deployment Instructions](#deployment-instructions)
- [Code Style & Conventions](#code-style--conventions)
- [Common Tasks & Commands](#common-tasks--commands)
- [Important Context for AI Assistants](#important-context-for-ai-assistants)
- [Troubleshooting](#troubleshooting)

---

## 📖 Project Overview

**StreamSpace** is a Kubernetes-native multi-user platform that streams containerized applications to web browsers using open source VNC technology. It provides on-demand provisioning with auto-hibernation for resource efficiency.

**Strategic Goal**: Build a 100% open source alternative to commercial container streaming platforms with complete independence from proprietary technologies.

### Key Features
- Browser-based access to any containerized application
- Multi-user support with SSO (Authentik/Keycloak)
- Persistent home directories (NFS)
- On-demand auto-hibernation for resource efficiency
- 200+ pre-built application templates (LinuxServer.io catalog)
- Resource quotas and limits per user
- Comprehensive monitoring with Grafana and Prometheus
- Optimized for k3s and ARM64 architectures

### Project Status
- **Current Phase**: Phase 1 (Planning & Architecture Complete)
- **Next Steps**: Controller implementation using Kubebuilder
- **Migration**: Recently migrated from `ai-infra-k3s/workspaces/` to standalone repository
- **Branding**: Rebranded from "Workspace Streaming Platform" to "StreamSpace"

### API Changes from Migration
- **Old API Group**: `workspaces.aiinfra.io/v1alpha1`
- **New API Group**: `stream.space/v1alpha1`
- **Old Resources**: WorkspaceSession, WorkspaceTemplate
- **New Resources**: Session (short: `ss`), Template (short: `tpl`)

---

## 🎯 Strategic Vision: Independence from Proprietary Technologies

**CRITICAL**: StreamSpace is being built as a **100% open source, fully independent** container streaming platform. All proprietary dependencies must be eliminated by v1.0.

### Mission Statement

StreamSpace will become the leading open source alternative to commercial container streaming platforms, offering complete independence from proprietary technologies while providing enterprise-grade features.

### Independence Roadmap

#### Current Dependencies to Eliminate

1. **KasmVNC** (Proprietary VNC Implementation)
   - **Current Status**: Used in all GUI application templates
   - **Target Replacement**: TigerVNC + noVNC (100% open source)
   - **Timeline**: Phase 3 (Months 7-9)
   - **Impact**: ~50 file references, complete architecture change
   - **Alternative Options**:
     - **Primary**: TigerVNC server + noVNC web client
     - **Secondary**: Apache Guacamole (clientless remote desktop)
     - **Research**: WebRTC-based streaming (lower latency)

2. **LinuxServer.io Container Images** (External Dependency)
   - **Current Status**: All 22 templates use LinuxServer.io images
   - **Target Replacement**: StreamSpace-native container images
   - **Timeline**: Phase 3 (Months 7-9)
   - **Impact**: 100+ container images to build and maintain
   - **Benefits**:
     - Full control over VNC stack
     - Optimized for StreamSpace
     - Faster security patches
     - ARM64 optimization

3. **Kasm Brand References** (Marketing/Documentation)
   - **Current Status**: Multiple references in docs and code
   - **Target Replacement**: StreamSpace brand only
   - **Timeline**: Ongoing, complete by Phase 3
   - **Impact**: Documentation, comments, examples

### Technical Migration Strategy

#### Phase 3: VNC Independence (CRITICAL)

**Recommended VNC Stack**:
```
┌─────────────────────────────────────┐
│  Web Browser (User)                 │
└──────────────┬──────────────────────┘
               │ HTTPS + WebSocket
               ↓
┌─────────────────────────────────────┐
│  noVNC Web Client (JavaScript)      │
│  - Canvas rendering                 │
│  - WebSocket transport              │
│  - Input handling                   │
└──────────────┬──────────────────────┘
               │ RFB Protocol
               ↓
┌─────────────────────────────────────┐
│  WebSocket Proxy (Go)               │
│  - TLS termination                  │
│  - Authentication                   │
│  - Connection routing               │
└──────────────┬──────────────────────┘
               │ TCP
               ↓
┌─────────────────────────────────────┐
│  TigerVNC Server (Container)        │
│  - Xvfb (Virtual framebuffer)       │
│  - Window manager (XFCE/i3)         │
│  - Application                      │
└─────────────────────────────────────┘
```

**Component Details**:

1. **TigerVNC Server**
   - License: GPL-2.0 (100% open source)
   - Features: High performance, clipboard support, resize
   - Platform: Linux, works with Xvfb
   - Integration: Drop-in replacement for KasmVNC server

2. **noVNC Client**
   - License: MPL-2.0 (100% open source)
   - Features: HTML5 canvas, touch support, mobile-friendly
   - Customization: Full UI control, branding
   - Integration: Direct WebSocket to VNC server

3. **WebSocket Proxy**
   - Implementation: Go-based custom proxy in API backend
   - Features: Authentication, rate limiting, monitoring
   - Integration: Part of StreamSpace API (Phase 2)

#### Container Image Strategy

**Base Image Tiers**:

1. **Tier 1: Core Bases** (Build First)
   - `streamspace/base-ubuntu-vnc:22.04` - Ubuntu with TigerVNC + XFCE
   - `streamspace/base-alpine-vnc:3.18` - Alpine with TigerVNC + i3
   - `streamspace/base-debian-vnc:12` - Debian with TigerVNC + MATE

2. **Tier 2: Application Categories** (100+ images)
   - Web Browsers: Firefox, Chromium, Brave (priority: high)
   - Development: VS Code, IntelliJ, Eclipse
   - Design: GIMP, Inkscape, Blender, Krita
   - Productivity: LibreOffice, Calligra
   - Media: Audacity, Kdenlive, OBS Studio

3. **Tier 3: Specialized** (50+ images)
   - Gaming: DuckStation, Dolphin, RetroArch
   - Scientific: Jupyter, R Studio, Octave
   - CAD/Engineering: FreeCAD, KiCad, OpenSCAD

**Image Build Infrastructure**:
```yaml
# GitHub Actions workflow
name: Build Container Images
on:
  schedule: [cron: '0 0 * * 0']  # Weekly
  push: {branches: [main]}

jobs:
  build-matrix:
    strategy:
      matrix:
        app: [firefox, chromium, vscode, gimp, ...]
        arch: [amd64, arm64]
    steps:
      - Build with TigerVNC + noVNC
      - Security scan (Trivy)
      - Sign image (Cosign)
      - Push to ghcr.io/streamspace
```

### Development Guidelines for AI Assistants

**IMPORTANT RULES**:

1. **Never introduce new Kasm dependencies**
   - Don't reference KasmVNC in new code
   - Don't use Kasm-specific features
   - Don't add Kasm to documentation

2. **Use generic VNC terminology**
   - Say "VNC server" not "KasmVNC server"
   - Say "VNC client" not "Kasm client"
   - Say "streaming" not "Kasm streaming"

3. **Prepare for VNC migration**
   - Write VNC-agnostic code
   - Use configuration for VNC endpoints
   - Abstract VNC details behind interfaces

4. **Reference alternatives in docs**
   - Mention noVNC as the target
   - Link to TigerVNC documentation
   - Explain migration path

5. **Track dependencies**
   - Document any external dependencies
   - Prefer open source alternatives
   - Plan for self-hosting

### Code Patterns for VNC Abstraction

**Good Pattern** (VNC-agnostic):
```go
type VNCConfig struct {
    Port        int    `json:"port"`
    Protocol    string `json:"protocol"`  // "vnc", "rfb", "websocket"
    Encryption  bool   `json:"encryption"`
}

func (t *Template) GetVNCPort() int {
    if t.Spec.VNC.Port != 0 {
        return t.Spec.VNC.Port
    }
    return 5900  // Standard VNC port
}
```

**Bad Pattern** (Kasm-specific):
```go
// ❌ DON'T DO THIS
type KasmVNCConfig struct {
    KasmPort int `json:"kasmPort"`
}
```

**Good Template Definition**:
```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
spec:
  vnc:  # Generic VNC config
    enabled: true
    port: 5900
    protocol: rfb
    websocket: true
```

**Bad Template Definition**:
```yaml
# ❌ DON'T DO THIS
spec:
  kasmvnc:  # Kasm-specific
    enabled: true
    kasmPort: 3000
```

### Migration Checklist

Track progress toward full independence:

**Phase 3 Tasks**:
- [ ] Research and select VNC stack (TigerVNC + noVNC)
- [ ] Build proof-of-concept with open source VNC
- [ ] Create base container images with TigerVNC
- [ ] Implement WebSocket proxy in API backend
- [ ] Rebuild all 22 templates with new VNC stack
- [ ] Update all documentation
- [ ] Remove all KasmVNC references from code
- [ ] Remove all KasmVNC references from docs
- [ ] Update CRD field names (kasmvnc → vnc)
- [ ] Migration guide for existing deployments
- [ ] Performance testing and optimization
- [ ] Security audit of new VNC stack

**Completion Criteria**:
- Zero mentions of "Kasm" or "kasmvnc" in codebase
- All container images built by StreamSpace
- No external dependencies on proprietary software
- Documentation explains open source stack
- Migration path documented for users

### Reference Documentation

For detailed migration plan, see:
- `ROADMAP.md` - Complete development roadmap
- Phase 3 section for VNC migration details
- Phase 6 for production readiness

For technical architecture, see:
- `docs/ARCHITECTURE.md` - Current architecture
- Future: `docs/VNC_MIGRATION.md` - VNC migration guide

---

## 📁 Repository Structure

```
streamspace/
├── .git/                    # Git repository
├── .gitignore              # Comprehensive ignore rules
├── README.md               # User-facing documentation
├── LICENSE                 # MIT License
├── CONTRIBUTING.md         # Contribution guidelines
├── MIGRATION_SUMMARY.md    # Migration details and history
├── CLAUDE.md              # This file - AI assistant guide
│
├── manifests/              # Kubernetes manifests
│   ├── crds/              # Custom Resource Definitions
│   │   ├── session.yaml           # Session CRD (main resource)
│   │   ├── template.yaml          # Template CRD (application definitions)
│   │   ├── workspacesession.yaml  # Legacy CRD (for backwards compatibility)
│   │   └── workspacetemplate.yaml # Legacy CRD (for backwards compatibility)
│   │
│   ├── config/            # Core platform configuration
│   │   ├── namespace.yaml         # streamspace namespace
│   │   ├── rbac.yaml             # RBAC roles and bindings
│   │   ├── controller-deployment.yaml   # Controller deployment spec
│   │   ├── controller-configmap.yaml    # Controller configuration
│   │   ├── api-deployment.yaml          # API backend deployment
│   │   ├── ui-deployment.yaml           # Web UI deployment
│   │   ├── database-init.yaml           # PostgreSQL initialization
│   │   └── ingress.yaml                 # Traefik ingress configuration
│   │
│   ├── templates/         # Application template manifests (22 pre-built)
│   │   ├── browsers/      # Firefox, Chromium, Brave, LibreWolf (4)
│   │   ├── development/   # VS Code, GitHub Desktop, etc. (3)
│   │   ├── productivity/  # LibreOffice, Calligra, etc. (3)
│   │   ├── design/        # GIMP, Krita, Inkscape, Blender, etc. (5)
│   │   ├── media/         # Audacity, Kdenlive, etc. (2)
│   │   ├── gaming/        # DuckStation, Dolphin, etc. (2)
│   │   └── webtop/        # Desktop environments (3)
│   │
│   └── monitoring/        # Observability stack
│       ├── servicemonitor.yaml              # Prometheus ServiceMonitor
│       ├── prometheusrule.yaml             # Alert rules
│       └── grafana-dashboard-workspace-overview.yaml  # Grafana dashboard
│
├── chart/                 # Helm chart for deployment
│   ├── Chart.yaml        # Chart metadata
│   ├── values.yaml       # Default configuration values
│   ├── README.md         # Helm installation guide
│   └── templates/        # Helm templates (to be created)
│
├── docs/                  # Technical documentation
│   ├── ARCHITECTURE.md        # Complete system architecture (17.8KB)
│   └── CONTROLLER_GUIDE.md    # Go controller implementation guide (19.2KB)
│
├── scripts/               # Utility scripts
│   └── generate-templates.py  # Generate 200+ LinuxServer.io templates
│
└── controller/            # To be created - Go controller using Kubebuilder
    └── (Phase 1 implementation)
```

### Directory Purposes

- **`manifests/`**: All Kubernetes YAML manifests, organized by purpose
  - `crds/`: Custom Resource Definitions for Sessions and Templates
  - `config/`: Platform deployment configurations
  - `templates/`: Pre-built application templates
  - `monitoring/`: Prometheus and Grafana configurations

- **`chart/`**: Helm chart for easy deployment and configuration management

- **`docs/`**: Comprehensive technical documentation
  - Architecture diagrams and data flows
  - Implementation guides for each component

- **`scripts/`**: Automation scripts for template generation and utilities

- **`controller/`**: (To be implemented) Go-based Kubernetes controller
  - Will use Kubebuilder framework
  - Manages Session lifecycle and hibernation

---

## 🛠 Key Technologies

### Core Stack
- **Kubernetes**: 1.19+ (k3s recommended for ARM64)
- **Container Runtime**: Docker/containerd
- **Storage**: NFS with ReadWriteMany support
- **Ingress**: Traefik (default) or any Kubernetes ingress controller
- **Authentication**: Authentik or Keycloak (OIDC/SSO)
- **Database**: PostgreSQL (for user data, sessions, audit logs)

### Controller (To Be Implemented)
- **Language**: Go 1.21+
- **Framework**: Kubebuilder 3.x
- **Client**: controller-runtime
- **Metrics**: Prometheus client_golang

### API Backend (To Be Implemented - Phase 2)
- **Option 1**: Go with Gin framework
- **Option 2**: Python with FastAPI
- **Authentication**: JWT tokens via OIDC
- **WebSocket**: For KasmVNC proxy connections

### Web UI (To Be Implemented - Phase 2)
- **Framework**: React 18+ with TypeScript
- **UI Library**: Material-UI (MUI)
- **State Management**: React Context API or Redux
- **Routing**: React Router
- **HTTP Client**: Axios with JWT interceptors

### Application Streaming
- **VNC Server**: Currently KasmVNC (⚠️ TEMPORARY - will be replaced with TigerVNC + noVNC in Phase 3)
- **Base Images**: Currently LinuxServer.io containers (⚠️ TEMPORARY - will be replaced with StreamSpace-native images in Phase 3)
- **VNC Port**: 5900 (standard VNC) or 3000 (current LinuxServer.io convention)
- **Target Stack**: TigerVNC server + noVNC client + WebSocket proxy (100% open source)

### Monitoring
- **Metrics**: Prometheus
- **Dashboards**: Grafana
- **Alerts**: PrometheusRule CRDs
- **Service Discovery**: ServiceMonitor CRDs

---

## 🎯 Custom Resource Definitions (CRDs)

### Session CRD (`stream.space/v1alpha1`)

**Purpose**: Represents a user's containerized workspace session.

**Location**: `manifests/crds/session.yaml`

**Short Names**: `ss`, `sessions`

**Key Fields**:
```yaml
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: user1-firefox           # Unique session identifier
  namespace: streamspace
spec:
  user: user1                   # Username (required)
  template: firefox-browser     # Template name (required)
  state: running                # running | hibernated | terminated (required)
  resources:                    # Resource limits
    memory: 2Gi
    cpu: 1000m
  persistentHome: true          # Mount user's persistent home directory
  idleTimeout: 30m              # Auto-hibernate after inactivity
  maxSessionDuration: 8h        # Maximum session lifetime
status:
  phase: Running                # Pending | Running | Hibernated | Failed | Terminated
  podName: ss-user1-firefox-abc123
  url: https://user1-firefox.streamspace.local
  lastActivity: "2025-01-15T10:30:00Z"
  resourceUsage:
    memory: 1.2Gi
    cpu: 450m
  conditions: []                # Standard Kubernetes conditions
```

**kubectl Examples**:
```bash
# List all sessions
kubectl get sessions -n streamspace
kubectl get ss -n streamspace  # Using short name

# Get session details
kubectl describe session user1-firefox -n streamspace

# Watch session status
kubectl get ss -n streamspace -w

# Delete a session
kubectl delete session user1-firefox -n streamspace
```

### Template CRD (`stream.space/v1alpha1`)

**Purpose**: Defines an application template that can be launched as a Session.

**Location**: `manifests/crds/template.yaml`

**Short Names**: `tpl`, `templates`

**Key Fields**:
```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
  namespace: streamspace
spec:
  displayName: Firefox Web Browser
  description: Modern, privacy-focused web browser
  category: Web Browsers        # Categorization for UI
  icon: https://example.com/firefox-icon.png
  baseImage: lscr.io/linuxserver/firefox:latest
  defaultResources:
    memory: 2Gi
    cpu: 1000m
  ports:
    - name: vnc
      containerPort: 3000
      protocol: TCP
  env:
    - name: PUID
      value: "1000"
    - name: PGID
      value: "1000"
  volumeMounts:
    - name: user-home
      mountPath: /config
  kasmvnc:
    enabled: true
    port: 3000
  capabilities:
    - Network
    - Audio
    - Clipboard
  tags:
    - browser
    - web
    - privacy
```

**kubectl Examples**:
```bash
# List all templates
kubectl get templates -n streamspace
kubectl get tpl -n streamspace  # Using short name

# View template details
kubectl describe template firefox-browser -n streamspace

# Get templates by category
kubectl get tpl -n streamspace -l category="Web Browsers"
```

### Legacy CRDs (Backwards Compatibility)

- `workspacesession.yaml`: Old WorkspaceSession CRD (deprecated, use Session)
- `workspacetemplate.yaml`: Old WorkspaceTemplate CRD (deprecated, use Template)

These exist for migration compatibility but should not be used in new code.

---

## 🔄 Development Workflows

### Phase 1: Controller Implementation (Current Phase)

**Goal**: Build the Go-based Kubernetes controller using Kubebuilder.

**Prerequisites**:
- Go 1.21+
- Kubebuilder 3.x
- Docker
- kubectl with cluster access
- Make

**Implementation Steps**:

1. **Initialize Kubebuilder Project**:
```bash
mkdir -p controller
cd controller

# Initialize Go module
go mod init github.com/yourusername/streamspace

# Initialize Kubebuilder
kubebuilder init --domain streamspace.io --repo github.com/yourusername/streamspace

# Create APIs
kubebuilder create api --group stream --version v1alpha1 --kind Session
kubebuilder create api --group stream --version v1alpha1 --kind Template
```

2. **Define CRD Types**:
- Edit `api/v1alpha1/session_types.go`
- Edit `api/v1alpha1/template_types.go`
- Reference: `docs/CONTROLLER_GUIDE.md` for detailed examples

3. **Implement Reconcilers**:
- `controllers/session_controller.go`: Main reconciliation logic
- `controllers/hibernation_controller.go`: Auto-hibernation logic
- `controllers/user_controller.go`: User PVC management

4. **Add Prometheus Metrics**:
- Active sessions gauge
- Hibernation events counter
- Resource usage metrics

5. **Build and Test**:
```bash
# Generate CRDs and code
make manifests generate

# Install CRDs to cluster
make install

# Run controller locally
make run

# Run tests
make test

# Build Docker image
make docker-build IMG=your-registry/streamspace-controller:v0.1.0
```

6. **Deploy to Cluster**:
```bash
# Push image
make docker-push IMG=your-registry/streamspace-controller:v0.1.0

# Deploy controller
make deploy IMG=your-registry/streamspace-controller:v0.1.0
```

### Phase 2: API & UI Implementation (Future)

**API Backend** (Go with Gin or Python with FastAPI):
- REST endpoints for session management
- WebSocket proxy for KasmVNC connections
- JWT authentication with OIDC
- Kubernetes client for CRD operations

**Web UI** (React + TypeScript):
- User dashboard (my sessions, catalog)
- Admin panel (all sessions, users, templates)
- Session viewer (iframe or new tab)
- Real-time status updates via WebSocket

### Phase 3: Monitoring & Observability (Future)

- Grafana dashboards
- Prometheus alert rules
- Audit logging
- Usage analytics

---

## 📝 Git Conventions

### Branch Strategy

**Main Branch**: `main` (protected)

**Feature Branches**:
- Format: `claude/claude-md-<session-id>`
- Example: `claude/claude-md-mhy5zeq2njvrp3yh-01MfcP2sWxBRw6sTTyEGW5gg`
- Always develop on feature branches, not main

### Commit Messages

Follow conventional commit format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Test additions or changes
- `chore`: Build/tooling changes
- `ci`: CI/CD changes

**Examples**:
```bash
feat(controller): implement session hibernation reconciler
fix(crd): correct validation for resource limits
docs(architecture): update data flow diagrams
refactor(api): extract authentication middleware
test(controller): add session lifecycle integration tests
```

### Commit Guidelines

1. **Clear and Concise**: Summarize what changed and why
2. **Present Tense**: Use "add" not "added", "fix" not "fixed"
3. **Focus on Why**: Explain the reason for the change
4. **Reference Issues**: Include issue numbers when applicable

**Good Examples**:
```bash
git commit -m "Add hibernation controller for auto-scaling sessions

Implements idle timeout detection and automatic scale-to-zero for
sessions that have been inactive beyond the configured threshold.

Closes #42"
```

**Bad Examples** (avoid):
```bash
git commit -m "updates"
git commit -m "fixed stuff"
git commit -m "WIP"
```

### Git Operations

**Pushing Changes**:
```bash
# Always push to feature branch with -u flag
git push -u origin claude/claude-md-<session-id>

# CRITICAL: Branch must start with 'claude/' and end with session ID
# Otherwise push will fail with 403 error
```

**Network Retry Strategy**:
- If `git push` or `git fetch` fails due to network errors
- Retry up to 4 times with exponential backoff (2s, 4s, 8s, 16s)

**Pull Requests**:
- Create PRs from feature branch to main
- Use PR template (see `CONTRIBUTING.md`)
- Request review from maintainers
- Ensure CI passes before merging

---

## 🧪 Testing Guidelines

### Unit Tests

**Controller Tests**:
```bash
cd controller
make test
```

**Test Structure**:
- Place tests in `*_test.go` files next to source
- Use `ginkgo` and `gomega` for BDD-style tests
- Mock Kubernetes client with `envtest`

**Example Test**:
```go
var _ = Describe("Session Controller", func() {
    Context("When creating a new Session", func() {
        It("Should create a Deployment", func() {
            // Test implementation
        })
    })
})
```

### Integration Tests

**Location**: `tests/` directory (to be created)

**Run Integration Tests**:
```bash
./scripts/run-integration-tests.sh
```

**Test Scenarios**:
- Session creation and lifecycle
- Hibernation and wake flows
- Resource quota enforcement
- User PVC provisioning

### Manual Testing

**Deploy to Test Cluster**:
```bash
# Create test namespace
kubectl create namespace streamspace-dev

# Deploy CRDs
kubectl apply -f manifests/crds/

# Deploy templates
kubectl apply -f manifests/templates/

# Create test session
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: test-firefox
  namespace: streamspace-dev
spec:
  user: testuser
  template: firefox-browser
  state: running
  resources:
    memory: 2Gi
    cpu: 1000m
  persistentHome: true
  idleTimeout: 30m
EOF

# Verify session status
kubectl get sessions -n streamspace-dev
kubectl describe session test-firefox -n streamspace-dev

# Check created resources
kubectl get pods,svc,pvc -n streamspace-dev -l workspace=test-firefox

# Cleanup
kubectl delete session test-firefox -n streamspace-dev
```

---

## 🚀 Deployment Instructions

### Deploy CRDs Only

```bash
# Deploy Session and Template CRDs
kubectl apply -f manifests/crds/session.yaml
kubectl apply -f manifests/crds/template.yaml

# Verify CRDs installed
kubectl get crds | grep stream.space
```

### Deploy Application Templates

```bash
# Deploy all templates
kubectl apply -f manifests/templates/

# Or deploy specific category
kubectl apply -f manifests/templates/browsers/
kubectl apply -f manifests/templates/development/

# Verify templates
kubectl get templates -n streamspace
```

### Deploy Platform (Full Installation)

**Option 1: Manual Deployment**:
```bash
# 1. Create namespace
kubectl apply -f manifests/config/namespace.yaml

# 2. Deploy RBAC
kubectl apply -f manifests/config/rbac.yaml

# 3. Deploy database
kubectl apply -f manifests/config/database-init.yaml

# 4. Deploy controller (after building image)
kubectl apply -f manifests/config/controller-deployment.yaml
kubectl apply -f manifests/config/controller-configmap.yaml

# 5. Deploy API and UI (Phase 2)
kubectl apply -f manifests/config/api-deployment.yaml
kubectl apply -f manifests/config/ui-deployment.yaml

# 6. Deploy ingress
kubectl apply -f manifests/config/ingress.yaml

# 7. Deploy monitoring
kubectl apply -f manifests/monitoring/
```

**Option 2: Helm Deployment** (Recommended):
```bash
# Install from local chart
helm install streamspace ./chart -n streamspace --create-namespace

# Or with custom values
helm install streamspace ./chart -n streamspace \
  --values custom-values.yaml

# Upgrade
helm upgrade streamspace ./chart -n streamspace

# Uninstall
helm uninstall streamspace -n streamspace
```

### Configuration

**Key Configuration Files**:
- `chart/values.yaml`: Helm chart defaults
- `manifests/config/controller-configmap.yaml`: Controller settings

**Important Settings**:
```yaml
# Hibernation
hibernation:
  enabled: true
  defaultIdleTimeout: 30m
  checkInterval: 60s

# Resources
resources:
  defaultMemory: 2Gi
  defaultCPU: 1000m
  maxMemory: 8Gi
  maxCPU: 4000m

# Storage
storage:
  className: nfs-client
  defaultHomeSize: 50Gi

# Networking
networking:
  ingressDomain: streamspace.local
  ingressClass: traefik
```

---

## 📐 Code Style & Conventions

### Go (Controller)

**Style Guide**: Follow [Effective Go](https://golang.org/doc/effective_go.html)

**Formatting**:
```bash
# Format code
gofmt -w .

# Run linter
golangci-lint run
```

**Naming Conventions**:
- Types: PascalCase (`SessionReconciler`, `UserManager`)
- Functions: camelCase (`reconcileSession`, `ensureUserPVC`)
- Constants: UPPER_SNAKE_CASE or PascalCase for exported
- Packages: lowercase, single word (`controllers`, `metrics`)

**Error Handling**:
```go
// Always handle errors explicitly
if err := r.Create(ctx, deployment); err != nil {
    log.Error(err, "Failed to create Deployment")
    return ctrl.Result{}, err
}

// Use wrapped errors for context
return fmt.Errorf("failed to get template %s: %w", templateName, err)
```

**Comments**:
```go
// SessionReconciler reconciles a Session object and manages
// the lifecycle of workspace pods, services, and PVCs.
type SessionReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// Reconcile implements the main reconciliation logic for Sessions.
// It handles state transitions (running, hibernated, terminated) and
// ensures the actual state matches the desired state.
func (r *SessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Implementation
}
```

### YAML (Kubernetes Manifests)

**Formatting**:
- Indent: 2 spaces
- Use `---` separator between resources in same file
- Order fields: apiVersion, kind, metadata, spec, status

**Labels**:
```yaml
metadata:
  labels:
    app: streamspace-session
    user: username
    template: firefox-browser
    session: user1-firefox
    app.kubernetes.io/name: streamspace
    app.kubernetes.io/component: session-pod
    app.kubernetes.io/managed-by: streamspace-controller
```

**Annotations**:
```yaml
metadata:
  annotations:
    description: "User session for firefox-browser"
    streamspace.io/created-by: "user1"
    streamspace.io/last-activity: "2025-01-15T10:30:00Z"
```

**Resource Naming**:
- Sessions: `{username}-{template}` (e.g., `user1-firefox`)
- Pods: `ss-{username}-{template}-{hash}` (e.g., `ss-user1-firefox-abc123`)
- Services: `ss-{username}-{template}-svc`
- PVCs: `home-{username}` (e.g., `home-user1`)

### Documentation

**Code Comments**:
- Public APIs must have godoc comments
- Complex logic should have inline comments explaining "why"
- Use TODO/FIXME/NOTE markers with issue references

**Markdown Files**:
- Use ATX-style headers (`#` not `===`)
- Include table of contents for long documents
- Use code blocks with language tags
- Keep line length reasonable (80-120 chars)

---

## 🔧 Common Tasks & Commands

### Working with CRDs

**Install CRDs**:
```bash
kubectl apply -f manifests/crds/session.yaml
kubectl apply -f manifests/crds/template.yaml
```

**Update CRDs** (after modifying in controller):
```bash
cd controller
make manifests  # Generate updated CRDs
kubectl apply -f config/crd/bases/
```

**View CRD Definition**:
```bash
kubectl get crd sessions.stream.space -o yaml
kubectl explain session.spec
kubectl explain session.status
```

### Working with Sessions

**Create a Session**:
```bash
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: user1-firefox
  namespace: streamspace
spec:
  user: user1
  template: firefox-browser
  state: running
  resources:
    memory: 2Gi
    cpu: 1000m
  persistentHome: true
  idleTimeout: 30m
EOF
```

**List Sessions**:
```bash
# All sessions
kubectl get sessions -n streamspace

# User's sessions
kubectl get sessions -n streamspace -l user=user1

# Running sessions only
kubectl get sessions -n streamspace --field-selector spec.state=running
```

**Hibernate a Session**:
```bash
kubectl patch session user1-firefox -n streamspace \
  --type merge -p '{"spec":{"state":"hibernated"}}'
```

**Wake a Session**:
```bash
kubectl patch session user1-firefox -n streamspace \
  --type merge -p '{"spec":{"state":"running"}}'
```

**Delete a Session**:
```bash
kubectl delete session user1-firefox -n streamspace
```

### Working with Templates

**Create a Template**:
```bash
kubectl apply -f manifests/templates/browsers/firefox.yaml
```

**Generate More Templates**:
```bash
cd scripts

# Generate all 200+ LinuxServer.io templates
python3 generate-templates.py

# List available categories
python3 generate-templates.py --list-categories

# Generate specific category
python3 generate-templates.py --category "Web Browsers"
```

**View Template Details**:
```bash
kubectl get template firefox-browser -n streamspace -o yaml
```

### Controller Development

**Run Controller Locally**:
```bash
cd controller
make run ENABLE_WEBHOOKS=false
```

**View Controller Logs**:
```bash
# In cluster
kubectl logs -n streamspace deploy/streamspace-controller -f

# Locally
make run 2>&1 | tee controller.log
```

**Debug Controller**:
```bash
# Enable debug logging
export LOG_LEVEL=debug
make run

# Or use delve debugger
dlv debug ./cmd/main.go
```

### Monitoring

**View Prometheus Metrics**:
```bash
# Port forward to controller
kubectl port-forward -n streamspace deploy/streamspace-controller 8080:8080

# Query metrics
curl http://localhost:8080/metrics | grep streamspace
```

**Access Grafana**:
```bash
kubectl port-forward -n observability svc/grafana 3000:80

# Open http://localhost:3000
# Default credentials: admin/admin
```

**View Alerts**:
```bash
kubectl get prometheusrules -n streamspace
kubectl describe prometheusrule streamspace-alerts -n streamspace
```

---

## 🤖 Important Context for AI Assistants

### Project History

1. **Original Project**: Part of `ai-infra-k3s` repository as `workspaces/` subdirectory
2. **Migration**: Moved to standalone `streamspace` repository (Nov 2024)
3. **Rebranding**: Changed from "Workspace Streaming Platform" to "StreamSpace"
4. **API Evolution**: `workspaces.aiinfra.io` → `stream.space`
5. **Resource Renaming**: WorkspaceSession → Session, WorkspaceTemplate → Template

### Current State

**What Exists**:
- ✅ Complete architecture documentation (`docs/ARCHITECTURE.md`)
- ✅ Controller implementation guide (`docs/CONTROLLER_GUIDE.md`)
- ✅ CRD definitions (Session, Template)
- ✅ 22 pre-built application templates
- ✅ Kubernetes manifests for deployment
- ✅ Helm chart structure with values
- ✅ Monitoring configuration (Prometheus, Grafana)
- ✅ Template generator script (for 200+ apps)
- ✅ Comprehensive README and CONTRIBUTING guides

**Implementation Status**:
- ✅ Go controller using Kubebuilder (Phase 1 - Complete)
- ✅ API backend with REST/WebSocket (Phase 2 - Complete)
- ✅ React web UI with admin panel (Phase 4 - Complete)
- ✅ Hibernation controller logic (Phase 1 - Complete)
- ✅ User management and quotas (Phase 2/4 - Complete)
- ✅ CI/CD pipelines (Phase 3 - Complete)
- ✅ Container image builds and registry (Phase 3 - Complete)
- ✅ Comprehensive testing suite (Phase 5 - Complete)
- ✅ Helm chart for deployment (Phase 5 - Complete)

**What Remains** (Future Enhancements):
- ⏳ WebSocket real-time session updates
- ⏳ Session sharing between users
- ⏳ Advanced resource quotas and policies
- ⏳ VNC migration from LinuxServer.io to native stack
- ⏳ Multi-cluster federation

### When Assisting with Code

**⚠️ CRITICAL RULES** (See "Strategic Vision" section above for details):

1. **NEVER introduce new Kasm/KasmVNC dependencies** - Use generic VNC terminology
2. **NEVER reference Kasm in new code or documentation** - StreamSpace is fully independent
3. **Always use VNC-agnostic patterns** - Abstract VNC implementation details

**Standard Guidelines**:

4. **CRD API Group**: Always use `stream.space/v1alpha1`, not `workspaces.aiinfra.io`
5. **Resource Names**: Use `Session` and `Template`, not the old Workspace* names
6. **Short Names**: Prefer `ss` and `tpl` in kubectl examples
7. **Namespace**: Default namespace is `streamspace`, not `workspaces`
8. **Kubebuilder**: When implementing controller, use domain `streamspace.io`
9. **Images**: Currently LinuxServer.io (`lscr.io/linuxserver/...`), migrating to StreamSpace-native
10. **VNC Port**: Use 5900 (standard VNC), currently 3000 for LinuxServer.io compatibility
11. **Storage**: Assume NFS with ReadWriteMany access mode
12. **Ingress Domain**: Default is `streamspace.local` (configurable)
13. **VNC Fields**: Use `vnc:` not `kasmvnc:` in template specs

### Key Design Decisions

1. **Single Container Per Pod**: Each session runs one application container (no sidecars in Phase 1)
2. **Shared User PVC**: All sessions for a user mount the same PVC at `/config`
3. **Deployment Pattern**: Use Deployments (not StatefulSets) with replicas 0/1 for hibernation
4. **Template-Based**: Sessions are instantiated from Template CRDs
5. **State-Driven**: Session state (`running`/`hibernated`/`terminated`) drives reconciliation
6. **Activity Tracking**: `lastActivity` timestamp updated externally (API/sidecar)
7. **Hibernation Model**: Scale Deployment to 0 replicas, not delete pod
8. **URL Pattern**: `{session-name}.{ingress-domain}` (e.g., `user1-firefox.streamspace.local`)

### Common Misconceptions to Avoid

**⚠️ Critical - Independence Strategy**:
- ❌ **Don't** introduce new KasmVNC references - use generic VNC
- ❌ **Don't** hardcode Kasm-specific features - keep VNC-agnostic
- ❌ **Don't** use `kasmvnc:` field name - use `vnc:` instead
- ❌ **Don't** assume KasmVNC will remain - code for TigerVNC migration

**Architecture Patterns**:
- ❌ **Don't** use StatefulSets - use Deployments with replicas field
- ❌ **Don't** delete pods for hibernation - scale Deployment to 0
- ❌ **Don't** create per-session PVCs - use shared user PVC
- ❌ **Don't** use `workspaces.aiinfra.io` API group - use `stream.space`
- ❌ **Don't** hardcode namespace - support configurable namespace
- ❌ **Don't** implement WebSocket proxy in controller - that's for API backend
- ❌ **Don't** build UI components in Phase 1 - focus on controller only

### Files to Reference

When helping with specific tasks, reference these files:

- **Strategic roadmap**: `ROADMAP.md` - Complete development roadmap and Kasm independence plan
- **Architecture questions**: `docs/ARCHITECTURE.md`
- **Controller implementation**: `docs/CONTROLLER_GUIDE.md`
- **CRD structure**: `manifests/crds/session.yaml`, `manifests/crds/template.yaml`
- **Template examples**: `manifests/templates/browsers/firefox.yaml`
- **Deployment config**: `chart/values.yaml`
- **Migration context**: `MIGRATION_SUMMARY.md`
- **Contribution workflow**: `CONTRIBUTING.md`

### Code Generation vs Manual Writing

- **CRDs**: Should be generated by Kubebuilder (`make manifests`)
- **Reconciler scaffolding**: Generated by Kubebuilder
- **Reconciler logic**: Manual implementation following `docs/CONTROLLER_GUIDE.md`
- **RBAC markers**: Use kubebuilder annotations, generate with `make manifests`
- **Template manifests**: Can be generated by `scripts/generate-templates.py`
- **Helm templates**: Manual creation based on `manifests/config/` examples

---

## 🔍 Troubleshooting

### CRD Issues

**Problem**: CRD not found
```bash
# Solution: Install CRDs
kubectl apply -f manifests/crds/session.yaml
kubectl apply -f manifests/crds/template.yaml

# Verify
kubectl get crds | grep stream.space
```

**Problem**: CRD validation errors
```bash
# Solution: Check CRD schema
kubectl explain session.spec
kubectl get crd sessions.stream.space -o yaml | grep -A 50 openAPIV3Schema

# Re-apply updated CRD
kubectl apply -f manifests/crds/session.yaml
```

### Session Issues

**Problem**: Session stuck in Pending phase
```bash
# Check session status
kubectl describe session <name> -n streamspace

# Check controller logs
kubectl logs -n streamspace deploy/streamspace-controller -f

# Check pod status
kubectl get pods -n streamspace -l session=<name>

# Check events
kubectl get events -n streamspace --sort-by=.metadata.creationTimestamp
```

**Problem**: Session pod not starting
```bash
# Check pod details
kubectl describe pod <pod-name> -n streamspace

# Check pod logs
kubectl logs <pod-name> -n streamspace

# Common issues:
# - Image pull errors: Check image name and registry access
# - PVC mount errors: Verify NFS provisioner is working
# - Resource limits: Check node capacity
```

**Problem**: Hibernation not working
```bash
# Verify hibernation is enabled
kubectl get cm -n streamspace streamspace-config -o yaml | grep hibernation

# Check lastActivity timestamp
kubectl get session <name> -n streamspace -o jsonpath='{.status.lastActivity}'

# Check hibernation controller logs
kubectl logs -n streamspace deploy/streamspace-controller -f | grep -i hibernation
```

### Template Issues

**Problem**: Template not found
```bash
# List available templates
kubectl get templates -n streamspace

# Create template
kubectl apply -f manifests/templates/browsers/firefox.yaml

# Verify
kubectl get template firefox-browser -n streamspace
```

**Problem**: Template image pull failures
```bash
# Test image manually
docker pull lscr.io/linuxserver/firefox:latest

# Check LinuxServer.io status
curl -I https://lscr.io/v2/

# Use alternative tag if latest fails
kubectl edit template firefox-browser -n streamspace
# Change tag to specific version
```

### Controller Issues

**Problem**: Controller not starting
```bash
# Check controller deployment
kubectl get deploy -n streamspace streamspace-controller

# Check controller logs
kubectl logs -n streamspace deploy/streamspace-controller

# Common issues:
# - CRDs not installed: kubectl apply -f manifests/crds/
# - RBAC permissions: kubectl apply -f manifests/config/rbac.yaml
# - Invalid config: kubectl get cm streamspace-config -n streamspace
```

**Problem**: Controller errors in logs
```bash
# Enable debug logging
kubectl set env -n streamspace deploy/streamspace-controller LOG_LEVEL=debug

# Watch logs
kubectl logs -n streamspace deploy/streamspace-controller -f

# Check for common errors:
# - "Failed to get Template": Template CRD missing
# - "Failed to create PVC": Storage class issues
# - "Failed to create Deployment": Resource quota exceeded
```

### Storage Issues

**Problem**: PVC stuck in Pending
```bash
# Check PVC status
kubectl describe pvc home-<username> -n streamspace

# Check storage class
kubectl get storageclass

# Verify NFS provisioner
kubectl get pods -n kube-system | grep nfs

# Common fixes:
# - Install NFS provisioner
# - Verify NFS server is accessible
# - Check storage class exists
```

### Network Issues

**Problem**: Cannot access session URL
```bash
# Check ingress
kubectl get ingress -n streamspace

# Check ingress controller
kubectl get pods -n kube-system -l app.kubernetes.io/name=traefik

# Check service
kubectl get svc -n streamspace -l session=<name>

# Test connectivity
kubectl port-forward -n streamspace svc/<service-name> 3000:3000
# Access http://localhost:3000
```

### Build Issues

**Problem**: `make` commands fail in controller
```bash
# Install Kubebuilder
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

# Install controller-gen
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Verify installation
kubebuilder version
controller-gen --version

# Re-run make
make manifests generate
```

**Problem**: Docker build fails
```bash
# Check Dockerfile exists
ls -la Dockerfile

# Build with verbose output
docker build --progress=plain -t streamspace-controller:latest .

# Check disk space
df -h

# Clean up old images
docker system prune -a
```

---

## 📚 Additional Resources

### External Documentation
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [LinuxServer.io Documentation](https://docs.linuxserver.io/)
- [KasmVNC Project](https://github.com/kasmtech/KasmVNC)
- [Traefik Documentation](https://doc.traefik.io/traefik/)

### Internal Documentation
- `README.md`: User-facing project overview
- `CONTRIBUTING.md`: Contribution guidelines and coding standards
- `MIGRATION_SUMMARY.md`: Migration history and context
- `docs/ARCHITECTURE.md`: Complete system architecture (17KB)
- `docs/CONTROLLER_GUIDE.md`: Go controller implementation guide (19KB)
- `chart/README.md`: Helm installation instructions

### Community & Support
- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and community support
- **Discord**: Real-time chat (link in README)
- **Documentation Site**: https://docs.streamspace.io (future)

---

## 📅 Version History

- **v0.1.0** (2025-11-14): Initial CLAUDE.md creation
  - Comprehensive guide for AI assistants
  - Repository structure documentation
  - Development workflows and conventions
  - Phase 1 (Controller) implementation guidance

---

**For Questions**: Refer to `docs/ARCHITECTURE.md` for technical details, or `CONTRIBUTING.md` for contribution workflow.

**Next Steps**: Follow `docs/CONTROLLER_GUIDE.md` to implement the Kubernetes controller using Kubebuilder.
