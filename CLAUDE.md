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

## 📂 Documentation Layout

### End-user-facing — sibling wiki repo

User-facing high-level documentation lives in the **streamspace.wiki** sibling repo (Getting Started, Architecture overview, Plugin/Template catalogs, Roadmap). Keep that repo as the entry point for people deploying and operating StreamSpace.

### Contributor-facing — `docs/`

Technical reference for people working ON the codebase:

- `docs/ARCHITECTURE.md` — system design
- `docs/API_REFERENCE.md` — REST + WebSocket API
- `docs/DEPLOYMENT.md` — deployment quick reference
- `docs/MIGRATION_V1_TO_V2.md` — migration guide
- `docs/design/architecture/` — ADRs
- `docs/historical/` — point-in-time architectural snapshots (don't edit; they're frozen records)

### Project root

Only top-level user-facing docs:
- `README.md`, `QUICKSTART.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `FEATURES.md`, `ROADMAP.md`

### Where ad-hoc agent work goes

Bug investigations, test runs, and validation reports should live in **GitHub issues / PR descriptions**, not as committed `.md` files. The previous practice of writing `.claude/reports/*.md` for every analysis cluttered the repo with ~150 stale files; the directory has been removed. If a finding has lasting architectural value, promote it to `docs/design/` or `docs/historical/` after review.

---

## 📚 Documentation Map

- [README.md](README.md) — project overview
- [QUICKSTART.md](QUICKSTART.md) — get running locally
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system design
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — deployment quick reference
- [docs/MIGRATION_V1_TO_V2.md](docs/MIGRATION_V1_TO_V2.md) — v1 → v2 migration
- [docs/design/architecture/](docs/design/architecture/) — ADRs
- [docs/historical/](docs/historical/) — frozen architectural snapshots
- streamspace.wiki sibling repo — end-user docs
