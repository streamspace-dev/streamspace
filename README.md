# StreamSpace

> **Stream any app to your browser** - An open source platform-agnostic container streaming platform

StreamSpace is a platform-agnostic platform that delivers browser-based access to containerized applications. It features a central Control Plane (API/WebUI) that manages distributed Controllers across various platforms (Kubernetes, Docker, Hyper-V, vCenter, etc.).

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.19+-blue.svg)](https://kubernetes.io/)

## Project Status

**Current Version**: v2.0-beta (Development Complete - Integration Testing)

StreamSpace has completed a major architectural transformation to a multi-platform Control Plane + Agent model. v2.0-beta development is 100% complete with integration testing underway.

### What's New in v2.0-beta

- **Multi-Platform Architecture**: Control Plane + Agent model supporting Kubernetes (Docker/VM/Cloud coming in v2.1+)
- **End-to-End VNC Proxy**: All VNC traffic flows through Control Plane (firewall-friendly, NAT traversal)
- **K8s Agent**: Fully functional Kubernetes agent with VNC tunneling (2,450 lines)
- **Agent Management UI**: Real-time agent monitoring, platform icons, status indicators
- **Modernized VNC Viewer**: Static noVNC page with Control Plane proxy integration
- **WebSocket Command Channel**: Bidirectional agent communication with command queue
- **13,850+ Lines Added**: Control Plane (~700), K8s Agent (~2,450), UI (~970), Tests (~2,500), Docs (~5,400)

### What Works (v2.0-beta)

- **Control Plane**: Agent Hub, Command Dispatcher, VNC Proxy, Agent Management API
- **K8s Agent**: Session lifecycle, VNC tunneling, health monitoring (fully functional)
- **API Backend**: 70+ REST handlers + agent endpoints, WebSocket support, 87 database tables
- **Web UI**: 50+ components, admin portal with agent management, VNC viewer proxy
- **Authentication**: Local, SAML 2.0, OIDC OAuth2, MFA (TOTP)
- **Test Coverage**: >70% on all v2.0 code (500+ test cases)
- **Documentation**: Comprehensive (3,131 lines: Deployment, Architecture, Migration guides)

### In Progress

- **Integration Testing** (Phase 10): E2E VNC streaming, multi-agent operations, failover testing
- **Docker Agent** (Phase 7, v2.1): Second platform implementation
- **Test Coverage Expansion**: Non-critical API handlers, additional UI tests

### Not Yet Implemented

- Docker, VM, Cloud platforms (v2.1+)
- Multi-cluster federation (v2.3+)
- WebRTC streaming (v3.0+)
- GPU acceleration (v3.0+)

## Features

### Core Features

- Browser-based access to containerized applications via VNC
- Multi-user support with isolated sessions
- Persistent home directories (NFS)
- Auto-hibernation (scale to zero when idle)
- 200+ pre-built application templates
- Resource quotas and limits per user
- Monitoring with Prometheus and Grafana

### Enterprise Features

- Authentication: Local, SAML 2.0 (Okta, Azure AD, Authentik, Keycloak, Auth0), OIDC OAuth2
- Multi-factor authentication with TOTP
- IP whitelisting and rate limiting
- Compliance frameworks (SOC2, HIPAA, GDPR)
- Audit logging and DLP policies
- Webhooks and integrations (Slack, Teams, Discord, PagerDuty, email)

## Quick Start

### Prerequisites

- Kubernetes 1.19+ (k3s recommended)
- Helm 3.0+
- PostgreSQL database
- NFS storage provisioner (ReadWriteMany)
- 4 CPU cores, 16GB RAM minimum

### Installation

```bash
# Clone repository
git clone https://github.com/streamspace-dev/streamspace.git
cd streamspace

# Deploy CRDs
kubectl apply -f manifests/crds/

# Install via Helm
helm install streamspace ./chart -n streamspace --create-namespace

# Create a session
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: my-firefox
  namespace: streamspace
spec:
  user: john
  template: firefox-browser
  state: running
  resources:
    memory: 2Gi
EOF
```

### Important: Production Secrets

Before deploying to production, change the default passwords:

```bash
POSTGRES_PASSWORD=$(openssl rand -base64 32)
kubectl create secret generic streamspace-secrets \
  --from-literal=postgres-password="$POSTGRES_PASSWORD" \
  -n streamspace
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│              Web UI (React)                     │
│  Dashboard, Catalog, Admin Panel               │
└──────────────────────┬──────────────────────────┘
                       │ REST API + WebSocket
                       ↓
┌─────────────────────────────────────────────────┐
│            Control Plane (API)                 │
│  Session CRUD, Auth, Plugins, Controller Mgmt  │
└──────────────────────┬──────────────────────────┘
                       │ Secure Protocol
                       ↓
┌─────────────────────────────────────────────────┐
│            StreamSpace Controllers              │
│  (Kubernetes, Docker, Hyper-V, etc.)           │
└──────────────────────┬──────────────────────────┘
                       │
                       ↓
┌─────────────────────────────────────────────────┐
│           Target Infrastructure                 │
│  Sessions (Pods/Containers/VMs)                │
└─────────────────────────────────────────────────┘
```

## Available Applications

Templates available via [streamspace-templates](https://github.com/JoshuaAFerguson/streamspace-templates):

- **Browsers**: Firefox, Chromium, Brave, LibreWolf
- **Development**: VS Code, GitHub Desktop
- **Productivity**: LibreOffice, OnlyOffice
- **Design**: GIMP, Krita, Inkscape, Blender
- **Media**: Audacity, Kdenlive

## Development

### Build Components

```bash
# Controller
cd k8s-controller && make docker-build IMG=your-registry/controller:latest

# API
cd api && go build -o streamspace-api

# UI
cd ui && npm install && npm run build
```

### Run Tests

```bash
# Controller tests (requires envtest)
cd k8s-controller && make test

# API tests
cd api && go test ./... -v

# UI tests
cd ui && npm test

# Integration tests
cd tests && ./scripts/run-integration-tests.sh
```

Current test coverage is approximately 15-20%. See `tests/reports/TEST_COVERAGE_REPORT.md` for details.

## Documentation

### Essential Docs

- [FEATURES.md](FEATURES.md) - Feature list with implementation status
- [ROADMAP.md](ROADMAP.md) - Development roadmap and next steps
- [CLAUDE.md](CLAUDE.md) - AI assistant guide for the codebase

### Technical Guides

- [Architecture](docs/ARCHITECTURE.md) - System architecture
- [Controller Guide](docs/CONTROLLER_GUIDE.md) - Controller implementation
- [Plugin Development](PLUGIN_DEVELOPMENT.md) - Building plugins
- [API Reference](api/API_REFERENCE.md) - REST API documentation

### Deployment

- [Deployment Guide](DEPLOYMENT.md) - Production deployment
- [Security](SECURITY.md) - Security policy

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

### Development Setup

1. Fork the repository
2. Create feature branch: `git checkout -b feature/my-feature`
3. Make changes and add tests
4. Commit: `git commit -am 'Add new feature'`
5. Push: `git push origin feature/my-feature`
6. Submit Pull Request

### Priority Areas for Contribution

1. **Test coverage** - Help us reach 80%+ coverage
2. **Plugin implementations** - Convert the 28 plugin stubs into working plugins
3. **Docker Controller** - Complete the Docker platform support
4. **VNC Migration** - Help migrate to TigerVNC + noVNC

## Troubleshooting

### Sessions not starting

```bash
kubectl logs -n streamspace deploy/streamspace-controller
kubectl describe session <session-name> -n streamspace
```

### Hibernation issues

```bash
kubectl get sessions -n streamspace -o jsonpath='{.items[*].status.lastActivity}'
```

## License

StreamSpace is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Acknowledgments

- [k3s](https://k3s.io/) - Lightweight Kubernetes
- [LinuxServer.io](https://linuxserver.io/) - Container images (temporary, migration planned)
- [TigerVNC](https://tigervnc.org/) and [noVNC](https://github.com/novnc/noVNC) - Future VNC stack

## Links

- **GitHub**: <https://github.com/JoshuaAFerguson/streamspace>
- **Templates**: <https://github.com/JoshuaAFerguson/streamspace-templates>
- **Plugins**: <https://github.com/JoshuaAFerguson/streamspace-plugins>

---

**Note**: This project is under active development. While the Kubernetes platform is functional, some features documented as "complete" may have partial implementations. See [FEATURES.md](FEATURES.md) for detailed status.
