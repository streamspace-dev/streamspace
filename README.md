# StreamSpace

> **Stream any app, anywhere** - 100% open source multi-user container streaming platform

StreamSpace is a Kubernetes-native platform that delivers browser-based access to containerized applications with on-demand auto-hibernation, persistent user storage, and enterprise-grade security. Built for self-hosting with complete independence from proprietary technologies, optimized for k3s and ARM64.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.19+-blue.svg)](https://kubernetes.io/)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/streamspace)](https://goreportcard.com/report/github.com/yourusername/streamspace)
[![Production Ready](https://img.shields.io/badge/status-production_ready-success.svg)](ROADMAP.md)
[![Phase 5 Complete](https://img.shields.io/badge/phase-5_complete-brightgreen.svg)](ROADMAP.md)

## ✨ Features

### Core Features
- 🌐 **Browser-Based Access** - Access any application via web browser using open source VNC
- 👥 **Multi-User Support** - Isolated sessions with SSO (Authentik/Keycloak)
- 💾 **Persistent Home Directories** - User files persist across sessions (NFS)
- ⚡ **On-Demand Auto-Hibernation** - Idle workspaces automatically scale to zero
- 🚀 **200+ Pre-Built Templates** - Comprehensive application catalog
- 🔌 **Plugin System** - Extend functionality with extensions, webhooks, and integrations
- 📊 **Resource Quotas** - Per-user memory, workspace, and storage limits
- 📈 **Comprehensive Monitoring** - Grafana dashboards and Prometheus metrics
- 🎯 **ARM64 Optimized** - Perfect for Orange Pi, Raspberry Pi, or any ARM cluster
- 🔓 **Fully Open Source** - No proprietary dependencies, complete self-hosting control

### Enterprise Features
- 🔐 **Authentication**: Local, SAML 2.0 (Okta, Azure AD, Authentik, Keycloak, Auth0), OIDC OAuth2 (8 providers)
- 🛡️ **Multi-Factor Authentication** - TOTP authenticator apps with backup codes
- 🌐 **IP Whitelisting** - Restrict access to specific IP addresses or CIDR ranges
- 🔒 **Security**: CSRF protection, rate limiting, SSRF protection, session verification
- 📋 **Compliance**: SOC2, HIPAA, GDPR frameworks with policy enforcement and violation tracking
- 🛡️ **Data Loss Prevention** - DLP policies with real-time violation detection
- ⏰ **Scheduled Sessions** - Automate session start/stop times
- 🔗 **Webhooks & Integrations** - 16 event types, Slack, Teams, Discord, PagerDuty, email (SMTP)
- 📊 **Real-Time Dashboard** - Live WebSocket updates for all sessions
- 👨‍💼 **Admin Control Panel** - 12 admin pages for users, groups, quotas, plugins, compliance
- 🎯 **RBAC** - Fine-grained role-based access control with team permissions
- 📝 **Audit Logging** - Comprehensive audit trail with retention policies

### 🚀 Coming Soon: Managed SaaS
Skip the infrastructure setup! **StreamSpace Cloud** is launching soon - managed hosting with automatic updates, backups, and 24/7 support. [Sign up for early access](#)

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

## 🎯 Project Status

**Current Version**: v1.0.0 - Production Ready

StreamSpace has completed **Phase 5 (Production-Ready)** with all core and enterprise features fully implemented:

- ✅ **Phases 1-5 Complete**: 100% feature-complete platform
- ✅ **82+ Database Tables**: Full-featured PostgreSQL schema
- ✅ **70+ API Handlers**: Comprehensive REST/WebSocket API
- ✅ **50+ UI Components**: Complete user and admin interfaces
- ✅ **15+ Middleware Layers**: Production-grade security and observability
- ✅ **Enterprise Authentication**: Local, SAML 2.0, OIDC OAuth2, MFA
- ✅ **Compliance & Security**: DLP, audit logging, RBAC, IP whitelisting
- ✅ **Monitoring**: 40+ Prometheus metrics, Grafana dashboards

**For complete feature list**: See [FEATURES.md](FEATURES.md)
**For development roadmap**: See [ROADMAP.md](ROADMAP.md)

**Next Phase**: VNC Independence (v2.0.0) - Migration to TigerVNC + noVNC stack

## 📋 Table of Contents

- [Project Status](#project-status)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [Available Applications](#available-applications)
- [Plugin System](#plugin-system)
- [Security](#security)
- [Configuration](#configuration)
- [Monitoring](#monitoring)
- [Development](#development)
- [Contributing](#contributing)
- [Documentation](#documentation)
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

### 🔐 Production Secrets (IMPORTANT!)

**⚠️ CRITICAL**: Before deploying to production, you **MUST** change the default passwords and secrets!

#### PostgreSQL Password

The default manifests include an **INSECURE** placeholder password. Replace it before deployment:

```bash
# Generate a secure password
POSTGRES_PASSWORD=$(openssl rand -base64 32)

# Create the secret BEFORE applying manifests
kubectl create secret generic streamspace-secrets \
  --from-literal=postgres-password="$POSTGRES_PASSWORD" \
  -n streamspace

# Then deploy (skip the streamspace-postgres.yaml secret)
kubectl apply -f manifests/crds/
kubectl apply -f manifests/config/ --exclude=streamspace-postgres.yaml
```

#### Using Helm (Recommended)

```bash
# Generate secure password
POSTGRES_PASSWORD=$(openssl rand -base64 32)

# Install with custom password
helm install streamspace ./chart -n streamspace \
  --set postgresql.postgresPassword="$POSTGRES_PASSWORD"
```

#### Production Best Practices

For production deployments, use proper secret management:

- **Sealed Secrets**: `kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.18.0/controller.yaml`
- **External Secrets Operator**: Integrate with AWS Secrets Manager, Azure Key Vault, or HashiCorp Vault
- **SOPS**: Encrypt secrets in Git with `sops`

See [Security Best Practices](docs/SECURITY.md#secret-management) for more details.

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

StreamSpace provides **200+ pre-configured templates** via the [streamspace-templates](https://github.com/JoshuaAFerguson/streamspace-templates) repository:

### Web Browsers
Firefox, Chromium, Brave, LibreWolf

### Development
VS Code, GitHub Desktop, GitQlient, Sublime Text

### Productivity
LibreOffice, Calligra, OnlyOffice

### Design & Graphics
GIMP, Krita, Inkscape, Blender, FreeCAD, KiCad, darktable

### Media Production
Audacity, Kdenlive

### Gaming & Emulation
Dolphin (GameCube/Wii), DuckStation (PlayStation)

### Desktop Environments
Ubuntu (XFCE, KDE), Alpine (i3), Arch

**Full Template Catalog**: [streamspace-templates](https://github.com/JoshuaAFerguson/streamspace-templates)

### Automatic Template Sync

Templates are automatically synced from the external repository:

```yaml
# In chart/values.yaml
repositories:
  templates:
    enabled: true
    url: https://github.com/JoshuaAFerguson/streamspace-templates
    syncInterval: 1h
```

## 🔌 Plugin System

StreamSpace features a powerful plugin system that allows you to extend functionality without modifying core code. Plugins can add new features, integrate with external services, customize workflows, and more.

### Plugin Types

- **Extensions** - Add new features and UI components
- **Webhooks** - React to system events (session created, user logged in, etc.)
- **API Integrations** - Connect to external services
- **UI Themes** - Customize the web interface appearance
- **CLI Tools** - Add new command-line utilities

### User Guide

#### Browse & Install Plugins

1. Navigate to **Plugins → Plugin Catalog** in the web UI
2. Browse available plugins by category or search
3. Click on a plugin to view details, permissions, and reviews
4. Click **Install** to add the plugin to your account
5. Configure the plugin in **Plugins → My Plugins**

#### Manage Installed Plugins

- **Enable/Disable**: Toggle plugins on/off without uninstalling
- **Configure**: Use the built-in form editor or JSON editor
- **Uninstall**: Remove plugins you no longer need
- **Rate & Review**: Help others discover great plugins

### Admin Guide

Administrators can manage plugins system-wide from **Admin → Plugin Management**:

```bash
# View all installed plugins
kubectl get -n streamspace cm plugin-registry -o yaml

# Enable/disable plugins globally
# Use the admin UI at /admin/plugins
```

**Admin Features**:
- View all installed plugins across all users
- Enable/disable plugins globally
- Configure plugin settings system-wide
- View plugin usage statistics
- Manage plugin permissions

### Official Plugin Repository

Plugins are automatically synced from the [streamspace-plugins](https://github.com/JoshuaAFerguson/streamspace-plugins) repository:

```yaml
# In chart/values.yaml
repositories:
  plugins:
    enabled: true
    url: https://github.com/JoshuaAFerguson/streamspace-plugins
    syncInterval: 1h
```

**Browse Plugin Catalog**: [streamspace-plugins](https://github.com/JoshuaAFerguson/streamspace-plugins)

**Available Plugin Categories**:
- **Official** - Maintained by StreamSpace team
- **Community** - User-contributed plugins

### Custom Plugin Repositories

Add additional plugin repositories to access more plugins:

1. Go to **Repositories** in the web UI
2. Click **Add Repository**
3. Enter repository URL (must be a Git repository)
4. Set authentication if needed (private repositories)
5. Click **Sync** to load plugins from the repository

```yaml
# Example: Add custom repository via Helm values
repositories:
  plugins:
    enabled: true
    url: https://github.com/mycompany/streamspace-plugins
    branch: main
```

### Security & Permissions

Plugins declare required permissions in their manifest. Users see these permissions before installation:

- **Low Risk** (🟢): Read-only access, notifications, API access
- **Medium Risk** (🟠): Webhook access, network requests, user data
- **High Risk** (🔴): Write access, admin privileges, filesystem, execute commands

Review permissions carefully before installing plugins from third-party sources.

### Developer Guide

Want to create your own plugins? See our comprehensive guides:

- **[Plugin Development Guide](PLUGIN_DEVELOPMENT.md)** - Complete tutorial for building plugins
- **[Plugin API Reference](docs/PLUGIN_API.md)** - API documentation and examples
- **[Plugin Manifest Schema](docs/PLUGIN_MANIFEST.md)** - Manifest file format

**Quick Example** - Create a simple notification plugin:

```javascript
// manifest.json
{
  "name": "welcome-notifier",
  "version": "1.0.0",
  "displayName": "Welcome Notifier",
  "description": "Sends welcome notifications to new users",
  "type": "webhook",
  "author": "Your Name",
  "permissions": ["notifications", "read:users"],
  "entrypoints": {
    "webhook": "index.js"
  }
}

// index.js
module.exports = {
  async onUserCreated(user) {
    await streamspace.notify(user.id, {
      title: "Welcome to StreamSpace!",
      message: `Hi ${user.fullName}, welcome aboard!`
    });
  }
};
```

See [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md) for complete examples and best practices.

## 🔒 Security

StreamSpace is built with **enterprise-grade security** from the ground up. All critical vulnerabilities have been addressed and comprehensive security controls are in place.

### 🛡️ Security Features

**Authentication & Access Control:**
- Multi-factor authentication (MFA) with TOTP authenticator apps
- IP whitelisting for network-level access control
- SSO integration with Authentik/Keycloak
- Role-based access control (RBAC)

**Data Protection:**
- TLS/SSL encryption for all connections
- Secure secret management
- Comprehensive audit logging
- Data isolation between users

**Infrastructure Security:**
- Container security with Pod Security Standards
- Network policies and service mesh (mTLS)
- Automated vulnerability scanning
- Regular security updates

### ✅ Production-Ready

StreamSpace has completed comprehensive security hardening:
- ✅ Zero known critical vulnerabilities
- ✅ 30+ automated security tests
- ✅ Enterprise security controls deployed
- ✅ Regular third-party security audits

### 🚨 Reporting Security Issues

We take security seriously. If you discover a vulnerability:

1. **DO NOT** open a public GitHub issue
2. Email: **security@streamspace.io** or use [GitHub Security Advisories](https://github.com/JoshuaAFerguson/streamspace/security/advisories)
3. Expected response: **48 hours**

See [SECURITY.md](SECURITY.md) for our complete security policy and responsible disclosure process.

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

StreamSpace exposes **40+ production-grade metrics** including:

**Session Metrics**:
```
streamspace_active_sessions_total
streamspace_hibernated_sessions_total
streamspace_session_starts_total
streamspace_hibernation_events_total
streamspace_session_creation_duration_seconds
streamspace_session_errors_total
```

**Resource Metrics**:
```
streamspace_resource_usage_bytes
streamspace_cluster_memory_usage_percent
streamspace_cpu_usage_cores
streamspace_storage_usage_bytes
```

**API Metrics**:
```
streamspace_http_requests_total
streamspace_http_request_duration_seconds
streamspace_websocket_connections_total
streamspace_api_errors_total
```

**Controller Metrics**:
```
streamspace_reconciliation_duration_seconds
streamspace_reconciliation_errors_total
streamspace_queue_depth
```

See [FEATURES.md](FEATURES.md#observability-metrics) for complete metrics list.

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

### Essential Documentation
- **[FEATURES.md](FEATURES.md)** - Complete feature list and implementation status
- **[ROADMAP.md](ROADMAP.md)** - Development roadmap (Phases 1-5 complete, Phase 6 planned)
- **[CLAUDE.md](CLAUDE.md)** - AI assistant guide for working with the codebase

### Technical Guides
- [Architecture Overview](docs/ARCHITECTURE.md) - System architecture and data flows
- [Controller Implementation](docs/CONTROLLER_GUIDE.md) - Go controller development guide
- [Plugin Development Guide](PLUGIN_DEVELOPMENT.md) - Build custom plugins
- [Plugin API Reference](docs/PLUGIN_API.md) - Plugin API documentation

### Deployment & Operations
- [Quick Start Guide](QUICKSTART.md) - Get started quickly
- [Deployment Guide](DEPLOYMENT.md) - Production deployment instructions
- [SAML Configuration](docs/SAML_GUIDE.md) - SAML 2.0 SSO setup guide
- [AWS Deployment](docs/AWS_DEPLOYMENT.md) - AWS-specific deployment guide
- [Container Deployment](docs/CONTAINER_DEPLOYMENT.md) - Container-based deployment

### API & Development
- [API Reference](api/API_REFERENCE.md) - REST API documentation
- [User & Group Management](api/docs/USER_GROUP_MANAGEMENT.md) - User and group management API

### Security & Compliance
- [Security Policy](SECURITY.md) - Security policy and responsible disclosure
- [Security Implementation](docs/SECURITY_IMPL_GUIDE.md) - Security architecture and controls
- [Security Testing](docs/SECURITY_TESTING.md) - Security testing procedures
- [Security Audit Prep](docs/SECURITY_AUDIT_PREP.md) - Security audit preparation

### Additional Resources
- [SAAS Deployment](docs/SAAS_DEPLOYMENT.md) - SaaS architecture and scaling
- [Competitive Analysis](docs/COMPETITIVE_ANALYSIS.md) - Feature comparison
- [Changelog](CHANGELOG.md) - Version history and updates
- [Contributing Guide](CONTRIBUTING.md) - Contribution guidelines

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

For more troubleshooting help, check the controller logs and session events as shown above.

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
