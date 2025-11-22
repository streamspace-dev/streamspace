# CLAUDE.md - AI Assistant Guide for StreamSpace

**Last Updated**: 2025-11-21
**Project Version**: v2.0-beta (Integration Testing)
**Architecture**: Control Plane + Agent (Multi-Platform)

---

## 📋 Quick Reference

### Current Status (v2.0-beta)

**Progress**: Integration Testing Phase
**Architecture**: Control Plane (API/UI) + Execution Agents (K8s)

**✅ Completed:**

- **Control Plane**: Centralized API with WebSocket Hub
- **K8s Agent**: Fully functional agent with VNC tunneling
- **VNC Proxy**: Secure, firewall-friendly VNC streaming
- **UI**: Real-time agent monitoring & session management
- **Security**: Production-hardened (Auth, RBAC, Audit Logs)

**🔄 In Progress:**

- **Integration Testing**: Verifying E2E flows
- **Test Coverage**: Expanding to 80%

**📋 Next Priorities:**

1. **Integration Tests**: Validate VNC streaming and failover.
2. **Plugin Implementation**: Convert stubs to working plugins.
3. **Docker Agent**: Begin v2.1 development.

---

## 🎯 Project Overview

**StreamSpace** is a platform-agnostic container streaming platform that delivers GUI applications to web browsers.

**Key Features:**

- **Browser-based Access**: Stream any containerized app via VNC.
- **Multi-Platform**: Kubernetes (Ready), Docker (Planned).
- **Secure**: Centralized Control Plane with VNC Proxy.
- **Enterprise Ready**: SSO (SAML/OIDC), MFA, Audit Logs.

**v2.0 Architecture:**

- **Control Plane**: API + Web UI (Central Management).
- **Agents**: Lightweight executors running on target platforms.
- **Communication**: Secure WebSocket (Command & Control + VNC Tunnel).

---

## 📁 Repository Structure

```
streamspace/
├── api/                         # Control Plane API (Go/Gin)
│   ├── internal/handlers/      # REST & WebSocket handlers
│   ├── internal/websocket/     # Agent Hub & VNC Proxy
│   └── internal/db/            # Database models
├── agents/                      # Execution Agents
│   └── k8s-agent/               # Kubernetes Agent (Go)
├── ui/                         # Web UI (React/TypeScript)
├── manifests/                  # Kubernetes manifests
│   ├── crds/                   # Session & Template CRDs
│   └── config/                 # Deployment configs
├── chart/                      # Helm chart
└── docs/                       # Documentation
```

---

## 🤖 Development Workflow

### Key Technologies

- **Backend**: Go 1.21+ (Gin)
- **Frontend**: React 18+ (MUI, TypeScript)
- **Database**: PostgreSQL
- **Agent Protocol**: WebSocket (JSON commands + Binary VNC)

### Testing

- **Unit Tests**: `go test ./...` (API/Agent), `npm test` (UI)
- **Integration**: `tests/scripts/run-integration-tests.sh`

---

## 🚀 Key Commands

### Kubernetes Operations

```bash
# List sessions
kubectl get sessions -n streamspace

# Check agent logs
kubectl logs -n streamspace -l app=streamspace-k8s-agent

# Check API logs
kubectl logs -n streamspace -l app=streamspace-api
```

### Development

```bash
# Run K8s Agent locally
cd agents/k8s-agent
go run . --api-url=http://localhost:8000

# Run API locally
cd api
go run cmd/main.go
```

---

## 📂 Documentation Standards

**IMPORTANT**: All agents must follow these documentation standards:

### Report Location

**All bug reports, test reports, validation reports, and analysis documents MUST be placed in `.claude/reports/`**

- ✅ **Correct**: `.claude/reports/BUG_REPORT_P1_*.md`
- ✅ **Correct**: `.claude/reports/INTEGRATION_TEST_*.md`
- ✅ **Correct**: `.claude/reports/VALIDATION_RESULTS_*.md`
- ❌ **Wrong**: `BUG_REPORT_*.md` (in project root)
- ❌ **Wrong**: `TEST_REPORT_*.md` (in project root)

### Project Root Documentation

**Only essential, user-facing documentation belongs in the project root:**

- `README.md` - Project overview
- `FEATURES.md` - Feature status
- `CONTRIBUTING.md` - Contribution guidelines
- `CHANGELOG.md` - Version history
- `DEPLOYMENT.md` - Deployment instructions

### docs/ Directory

**Permanent, reference documentation:**

- `docs/ARCHITECTURE.md` - System design
- `docs/SCALABILITY.md` - Scaling guide
- `docs/TROUBLESHOOTING.md` - Common issues
- `docs/V2_DEPLOYMENT_GUIDE.md` - Deployment details
- `docs/V2_BETA_RELEASE_NOTES.md` - Release notes

### .claude/ Directory Structure

```
.claude/
├── multi-agent/              # Multi-agent coordination
│   ├── MULTI_AGENT_PLAN.md  # Agent coordination plan
│   ├── agent*-instructions.md
│   └── ...
└── reports/                  # All bug/test/validation reports
    ├── BUG_REPORT_*.md
    ├── INTEGRATION_TEST_*.md
    ├── VALIDATION_RESULTS_*.md
    └── ...
```

### Why This Matters

- **Clean Root**: Users see only essential docs when browsing repo
- **Organized Reports**: All agent work tracked in one location
- **Git History**: Cleaner commits without report noise
- **Discoverability**: Easier to find specific reports

---

## 📚 Documentation Map

- **[README.md](README.md)**: Project Overview
- **[FEATURES.md](FEATURES.md)**: Feature Status
- **[ROADMAP.md](ROADMAP.md)**: Future Plans
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**: System Design
- **[DEPLOYMENT.md](DEPLOYMENT.md)**: Installation Guide
- **[.claude/reports/](.claude/reports/)**: Bug Reports, Test Results, Validation Reports
