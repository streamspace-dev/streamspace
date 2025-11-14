# StreamSpace

> **Stream any app, anywhere** - 100% open source multi-user container streaming platform

StreamSpace is a Kubernetes-native platform that delivers browser-based access to containerized applications with on-demand auto-hibernation, persistent user storage, and enterprise-grade security. Built for self-hosting with complete independence from proprietary technologies, optimized for k3s and ARM64.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.19+-blue.svg)](https://kubernetes.io/)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/streamspace)](https://goreportcard.com/report/github.com/yourusername/streamspace)

## ✨ Features

- 🌐 **Browser-Based Access** - Access any application via web browser using open source VNC
- 👥 **Multi-User Support** - Isolated sessions with SSO (Authentik/Keycloak)
- 💾 **Persistent Home Directories** - User files persist across sessions (NFS)
- ⚡ **On-Demand Auto-Hibernation** - Idle workspaces automatically scale to zero
- 🚀 **200+ Pre-Built Templates** - Comprehensive application catalog
- 📊 **Resource Quotas** - Per-user memory, workspace, and storage limits
- 🔒 **Enterprise Security** - Network policies, SSO, audit logging, DLP
- 📈 **Comprehensive Monitoring** - Grafana dashboards and Prometheus metrics
- 🎯 **ARM64 Optimized** - Perfect for Orange Pi, Raspberry Pi, or any ARM cluster
- 🔓 **Fully Open Source** - No proprietary dependencies, complete self-hosting control

## 🎬 Quick Demo

```bash
# Install StreamSpace
helm install streamspace ./chart -n streamspace

# Launch Firefox workspace
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: my-firefox
spec:
  user: john
  template: firefox-browser
  resources:
    memory: 2Gi
EOF

# Access via browser
# https://my-firefox.streamspace.local
```

## 📋 Table of Contents

- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [Available Applications](#available-applications)
- [Configuration](#configuration)
- [Monitoring](#monitoring)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Web UI (React)                        │
│  User Dashboard • Catalog • Session Viewer • Admin      │
└────────────────────────┬────────────────────────────────┘
                         │ REST API + WebSocket
                         ↓
┌─────────────────────────────────────────────────────────┐
│              StreamSpace Controller (Go)                 │
│  Session Lifecycle • Auto-Hibernation • User Management │
└────────────────────────┬────────────────────────────────┘
                         │ Kubernetes API
                         ↓
┌─────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Session  │  │ Session  │  │ Session  │              │
│  │ Pod      │  │ Pod      │  │ Pod      │              │
│  │(VNC)     │  │(VNC)     │  │(VNC)     │              │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘              │
│       │             │             │                      │
│  /home/user1   /home/user2   /home/user3               │
│  (NFS PVC)     (NFS PVC)     (NFS PVC)                 │
└─────────────────────────────────────────────────────────┘
```

**Key Components**:
- **Controller**: Manages session lifecycle, hibernation, and provisioning
- **API Backend**: REST/WebSocket API for UI and integrations
- **Web UI**: User-facing dashboard and workspace catalog
- **Sessions**: Containerized applications with VNC streaming to your browser
- **User Storage**: Persistent NFS volumes mounted across all sessions

## 📦 Prerequisites

- Kubernetes 1.19+ (k3s recommended)
- Helm 3.0+
- PostgreSQL database
- NFS storage provisioner (or any ReadWriteMany storage)
- MetalLB or cloud LoadBalancer
- Authentik or Keycloak for SSO (optional but recommended)

**Minimum Cluster Resources**:
- 4 CPU cores
- 16GB RAM (32GB+ recommended for multiple concurrent sessions)
- 100GB storage

## 🚀 Installation

### Quick Start (Helm)

```bash
# 1. Add Helm repository
helm repo add streamspace https://streamspace.io/charts
helm repo update

# 2. Create namespace
kubectl create namespace streamspace

# 3. Install
helm install streamspace streamspace/streamspace -n streamspace

# 4. Get LoadBalancer IP
kubectl get svc -n streamspace streamspace-ui
```

### Manual Installation

```bash
# 1. Clone repository
git clone https://github.com/yourusername/streamspace.git
cd streamspace

# 2. Deploy CRDs
kubectl apply -f manifests/crds/

# 3. Deploy configuration
kubectl apply -f manifests/config/

# 4. Deploy application templates
kubectl apply -f manifests/templates/

# 5. Install via Helm
helm install streamspace ./chart -n streamspace
```

### Configuration

Edit `values.yaml` before installation:

```yaml
controller:
  config:
    postgres:
      host: postgres.default.svc.cluster.local
      password: YOUR_PASSWORD

    authentik:
      url: https://auth.example.com
      clientSecret: YOUR_CLIENT_SECRET

ingress:
  hostname: streamspace.example.com
  tls:
    enabled: true
```

Full configuration options: [docs/CONFIGURATION.md](docs/CONFIGURATION.md)

## 🎯 Usage

### For Users

1. **Login**: Navigate to your StreamSpace URL and login via SSO
2. **Browse Catalog**: Browse 200+ available applications by category
3. **Launch Session**: Click "Launch" on any application
4. **Access Workspace**: Browser opens streaming session in new tab
5. **Save Work**: All files in `/home` persist across sessions
6. **Auto-Hibernate**: Sessions automatically hibernate after 30m idle
7. **Resume**: Click session again to wake from hibernation (~20s)

### For Admins

```bash
# View all sessions
kubectl get sessions -n streamspace

# View session details
kubectl describe session my-firefox -n streamspace

# Force terminate session
kubectl delete session my-firefox -n streamspace

# Check resource usage
kubectl top pods -n streamspace

# View controller logs
kubectl logs -n streamspace deploy/streamspace-controller -f
```

Admin panel: `https://streamspace.example.com/admin`

## 📱 Available Applications

StreamSpace includes **200+ pre-configured templates** from LinuxServer.io:

### Web Browsers (5)
Firefox, Chromium, Chrome, Brave, LibreWolf

### Development (10+)
VS Code, GitHub Desktop, GitQlient, Gitea, JupyterLab

### Productivity (20+)
LibreOffice, Calligra, GIMP, Inkscape, Krita, Blender

### Media (15+)
Audacity, Kdenlive, Jellyfin, Plex, Radarr, Sonarr

### Design (10+)
GIMP, Krita, Inkscape, Blender, FreeCAD, KiCad

### Desktop Environments (16)
Ubuntu (XFCE, KDE, MATE), Alpine (i3), Fedora, Arch

### Gaming (8+)
Dolphin, DuckStation, MAME, GZDoom, Flycast

See full catalog: [docs/APPLICATIONS.md](docs/APPLICATIONS.md)

## ⚙️ Configuration

### Resource Quotas

Set per-user limits:

```yaml
apiVersion: stream.space/v1alpha1
kind: User
metadata:
  name: john
spec:
  tier: pro
  quotas:
    memory: 16Gi
    maxSessions: 5
    storage: 100Gi
    maxSessionDuration: 8h
```

### Custom Templates

Add your own applications:

```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: my-app
spec:
  displayName: My Custom App
  category: Custom
  image: myregistry/myapp:latest
  defaultResources:
    memory: 4Gi
    cpu: 2000m
  ports:
    - name: vnc
      containerPort: 3000
```

### Hibernation Settings

Configure auto-hibernation:

```yaml
controller:
  config:
    hibernation:
      enabled: true
      defaultIdleTimeout: 30m  # Hibernate after 30 min idle
      checkInterval: 60s       # Check every 60 seconds
```

## 📊 Monitoring

StreamSpace includes comprehensive monitoring:

### Grafana Dashboards

- **Session Overview**: Active/hibernated sessions, memory usage
- **User Activity**: Logins, launches, session duration
- **Cluster Capacity**: Resource utilization, queue depth
- **API Performance**: Request rates, error rates, latency

### Prometheus Metrics

```
streamspace_active_sessions_total
streamspace_hibernated_sessions_total
streamspace_session_starts_total
streamspace_hibernation_events_total
streamspace_resource_usage_bytes
streamspace_cluster_memory_usage_percent
```

### Alerts

11 pre-configured alerts including:
- High memory usage (>85%)
- Provisioning failures
- Controller/API downtime
- High API error rate

Access Grafana: `kubectl port-forward -n observability svc/grafana 3000:80`

## 🛠️ Development

### Build Controller

```bash
cd controller

# Initialize Go project
go mod init github.com/yourusername/streamspace

# Install Kubebuilder
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

# Initialize project
kubebuilder init --domain streamspace.io --repo github.com/yourusername/streamspace

# Create APIs
kubebuilder create api --group stream --version v1alpha1 --kind Session
kubebuilder create api --group stream --version v1alpha1 --kind Template

# Build
make docker-build docker-push IMG=yourregistry/streamspace-controller:latest
```

See full guide: [docs/CONTROLLER_GUIDE.md](docs/CONTROLLER_GUIDE.md)

### Build API Backend

```bash
cd api

# Go backend
go build -o streamspace-api

# Or Python backend
pip install -r requirements.txt
uvicorn main:app --reload
```

### Build Web UI

```bash
cd ui

# Install dependencies
npm install

# Development server
npm start

# Production build
npm run build
```

## 🧪 Testing

```bash
# Run controller tests
cd controller
make test

# Run API tests
cd api
go test ./... -v

# Run UI tests
cd ui
npm test

# Integration tests
cd tests
./run-integration-tests.sh
```

## 🤝 Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

### Development Setup

1. Fork the repository
2. Create feature branch: `git checkout -b feature/my-feature`
3. Make changes and test
4. Commit: `git commit -am 'Add new feature'`
5. Push: `git push origin feature/my-feature`
6. Submit Pull Request

## 📖 Documentation

- [Architecture Overview](docs/ARCHITECTURE.md)
- [Getting Started](docs/GETTING_STARTED.md)
- [User Guide](docs/USER_GUIDE.md)
- [Admin Guide](docs/ADMIN_GUIDE.md)
- [API Reference](docs/API_REFERENCE.md)
- [Controller Implementation](docs/CONTROLLER_GUIDE.md)
- [Security Hardening](docs/SECURITY.md)

## 🐛 Troubleshooting

### Sessions not starting

```bash
# Check controller logs
kubectl logs -n streamspace deploy/streamspace-controller

# Check session events
kubectl describe session -n streamspace <session-name>

# Check pod status
kubectl get pods -n streamspace
```

### Hibernation not working

```bash
# Check hibernation config
kubectl get cm -n streamspace streamspace-config -o yaml

# Check last activity timestamps
kubectl get sessions -n streamspace -o jsonpath='{.items[*].status.lastActivity}'
```

Common issues: [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)

## 📄 License

StreamSpace is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- Built for [k3s](https://k3s.io/) - Lightweight Kubernetes
- VNC technology: [TigerVNC](https://tigervnc.org/) (GPL-2.0) and [noVNC](https://github.com/novnc/noVNC) (MPL-2.0)
- Open source community providing the foundation for truly independent container streaming

## 🔗 Links

- **Website**: https://streamspace.io
- **Documentation**: https://docs.streamspace.io
- **GitHub**: https://github.com/yourusername/streamspace
- **Discord**: https://discord.gg/streamspace

## ⭐ Star History

If you find StreamSpace useful, please consider giving it a star! ⭐

---

**Made with ❤️ by the StreamSpace community**
