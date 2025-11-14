# StreamSpace Architecture

Complete architecture documentation for the StreamSpace container streaming platform.

## Overview

StreamSpace is a Kubernetes-native multi-user platform that streams containerized applications to web browsers using KasmVNC. Built for k3s and optimized for ARM64, it provides on-demand provisioning with auto-hibernation for resource efficiency.

**Based on**: Original implementation plan in `ai-infra-k3s/docs/KASM_ALTERNATIVE_PLAN.md`

## System Architecture

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        Users                                  │
│              (Web Browsers - Any Device)                      │
└────────────────────────┬─────────────────────────────────────┘
                         │ HTTPS
                         ↓
┌──────────────────────────────────────────────────────────────┐
│                   Ingress (Traefik)                           │
│  - TLS termination                                            │
│  - ForwardAuth (Authentik SSO)                               │
│  - Dynamic routing per session                                │
└────────────────────────┬─────────────────────────────────────┘
                         │
          ┌──────────────┴─────────────┐
          ↓                            ↓
┌─────────────────────┐      ┌──────────────────────┐
│   Web UI (React)    │      │   API Backend (Go)   │
│  - Dashboard        │      │   - REST API         │
│  - Catalog          │      │   - WebSocket        │
│  - Session viewer   │      │   - Auth middleware  │
│  - Admin panel      │      │   - K8s client       │
└─────────────────────┘      └──────────┬───────────┘
                                        │
                         ┌──────────────┴──────────────┐
                         ↓                             ↓
┌──────────────────────────────────────┐   ┌─────────────────┐
│    StreamSpace Controller (Go)        │   │  PostgreSQL     │
│  ┌────────────────────────────────┐  │   │  - Sessions     │
│  │  Session Reconciler            │  │   │  - Users        │
│  │  - Create/Update/Delete pods   │  │   │  - Templates    │
│  │  - Status tracking             │  │   │  - Audit logs   │
│  └────────────────────────────────┘  │   └─────────────────┘
│  ┌────────────────────────────────┐  │
│  │  Hibernation Controller        │  │
│  │  - Idle detection              │  │
│  │  - Scale to zero               │  │
│  │  - Wake on access              │  │
│  └────────────────────────────────┘  │
│  ┌────────────────────────────────┐  │
│  │  User Manager                  │  │
│  │  - PVC provisioning            │  │
│  │  - Quota enforcement           │  │
│  └────────────────────────────────┘  │
└────────────────┬─────────────────────┘
                 │ Kubernetes API
                 ↓
┌──────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ Session Pod  │  │ Session Pod  │  │ Session Pod  │       │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │       │
│  │ │Container │ │  │ │Container │ │  │ │Container │ │       │
│  │ │(Firefox) │ │  │ │(VS Code) │ │  │ │(Blender) │ │       │
│  │ │+ KasmVNC │ │  │ │+ KasmVNC │ │  │ │+ KasmVNC │ │       │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │       │
│  │      ↓       │  │      ↓       │  │      ↓       │       │
│  │ /home/user1  │  │ /home/user2  │  │ /home/user1  │       │
│  │  (NFS PVC)   │  │  (NFS PVC)   │  │  (NFS PVC)   │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└──────────────────────────────────────────────────────────────┘
                         ↑
                         │ NFS Protocol
                         ↓
┌──────────────────────────────────────────────────────────────┐
│              NFS Server (Persistent User Homes)               │
│  /export/home/user1, /export/home/user2, /export/home/user3  │
└──────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. StreamSpace Controller

**Language**: Go with Kubebuilder framework
**Purpose**: Manages session lifecycle and resource provisioning

**Responsibilities**:
- Watch for Session CRD changes
- Provision pods, services, PVCs based on templates
- Update session status (phase, URL, resource usage)
- Enforce user quotas
- Handle state transitions (running → hibernated → terminated)

**Key Reconcilers**:
- `SessionReconciler`: Main reconciliation loop
- `HibernationReconciler`: Idle detection and scale-to-zero
- `UserReconciler`: User management and PVC provisioning

**Metrics Exposed**:
- `streamspace_active_sessions_total`
- `streamspace_hibernated_sessions_total`
- `streamspace_session_starts_total`
- `streamspace_hibernation_duration_seconds`
- `streamspace_resource_usage_bytes`

### 2. API Backend

**Language**: Go (Gin framework) or Python (FastAPI)
**Purpose**: REST/WebSocket API for UI and integrations

**Endpoints**:
- `GET /api/v1/sessions` - List user sessions
- `POST /api/v1/sessions` - Create session
- `GET /api/v1/sessions/{id}` - Get session details
- `DELETE /api/v1/sessions/{id}` - Terminate session
- `POST /api/v1/sessions/{id}/wake` - Wake hibernated session
- `GET /api/v1/templates` - List templates
- `GET /api/v1/users/me` - Get current user info
- `WS /api/v1/sessions/{id}/connect` - WebSocket for KasmVNC proxy

**Authentication**:
- OIDC via Authentik
- JWT tokens (1-hour expiration)
- Refresh token flow

**Authorization**:
- Users: Own sessions only
- Admins: All sessions + config

### 3. Web UI

**Framework**: React + TypeScript + Material-UI
**Purpose**: User-facing dashboard and admin panel

**Pages**:
- `/login` - Authentik SSO login
- `/dashboard` - My sessions (running, hibernated)
- `/catalog` - Browse templates by category
- `/session/{id}` - View/connect to session (iframe or new tab)
- `/admin/users` - User management
- `/admin/templates` - Template management
- `/admin/analytics` - Usage analytics

**State Management**: React Context API or Redux
**Routing**: React Router
**API Client**: Axios with JWT interceptors

### 4. Session Pods

**Structure**: Single-container pod with user-specific labels

**Pod Specification**:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ss-user1-firefox-abc123
  labels:
    app: streamspace-session
    user: user1
    template: firefox-browser
    session: user1-firefox
spec:
  containers:
  - name: workspace
    image: lscr.io/linuxserver/firefox:latest
    ports:
    - containerPort: 3000
      name: vnc
    env:
    - name: PUID
      value: "1000"
    - name: PGID
      value: "1000"
    volumeMounts:
    - name: user-home
      mountPath: /config
    resources:
      requests:
        memory: 2Gi
        cpu: 1000m
      limits:
        memory: 2Gi
        cpu: 1000m
  volumes:
  - name: user-home
    persistentVolumeClaim:
      claimName: home-user1
```

**Networking**:
- Service per session: `ss-user1-firefox-svc`
- Ingress rule: `user1-firefox.streamspace.local` → Service
- KasmVNC port: 3000 (default)

### 5. User Storage

**Backend**: NFS with ReadWriteMany support

**PVC per User**:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: home-user1
  namespace: streamspace
spec:
  accessModes: [ReadWriteMany]
  storageClassName: nfs-client
  resources:
    requests:
      storage: 50Gi
```

**Mount Path**: `/config` (LinuxServer.io convention) or `/home/kasm-user`

**Benefits**:
- Files persist across sessions
- Shared across all user's workspaces
- Backed up independently

### 6. Plugin System

**Purpose**: Extensible architecture for adding custom functionality without modifying core code

**Plugin Types**:
- **Extension**: Add new features and UI components
- **Webhook**: React to system events (session created, user login, etc.)
- **API Integration**: Connect to external services (Slack, GitHub, Jira)
- **UI Theme**: Customize web interface appearance
- **CLI**: Add custom command-line tools

**Database Schema**:
```sql
-- Plugin repositories (GitHub, GitLab, custom)
CREATE TABLE repositories (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  url TEXT NOT NULL,
  branch VARCHAR(255) DEFAULT 'main',
  auth_type VARCHAR(50),
  enabled BOOLEAN DEFAULT true
);

-- Catalog of available plugins
CREATE TABLE catalog_plugins (
  id SERIAL PRIMARY KEY,
  repository_id INTEGER REFERENCES repositories(id),
  name VARCHAR(255) NOT NULL UNIQUE,
  version VARCHAR(50),
  display_name VARCHAR(255),
  description TEXT,
  category VARCHAR(100),
  plugin_type VARCHAR(50),
  icon_url TEXT,
  manifest JSONB,
  tags TEXT[]
);

-- User-installed plugins
CREATE TABLE installed_plugins (
  id SERIAL PRIMARY KEY,
  catalog_plugin_id INTEGER REFERENCES catalog_plugins(id),
  name VARCHAR(255) NOT NULL UNIQUE,
  version VARCHAR(50),
  enabled BOOLEAN DEFAULT false,
  config JSONB,
  installed_by VARCHAR(255),
  installed_at TIMESTAMP DEFAULT NOW()
);
```

**API Endpoints**:
- `GET /api/v1/plugins/catalog` - Browse available plugins
- `POST /api/v1/plugins/install` - Install plugin
- `GET /api/v1/plugins/installed` - List installed plugins
- `POST /api/v1/plugins/{id}/enable` - Enable plugin
- `POST /api/v1/plugins/{id}/disable` - Disable plugin
- `PUT /api/v1/plugins/{id}/config` - Update plugin configuration
- `DELETE /api/v1/plugins/{id}` - Uninstall plugin

**UI Components**:
- **PluginCatalog** (`/plugins/catalog`) - Browse and install plugins with search, filters, ratings
- **InstalledPlugins** (`/plugins/installed`) - Manage installed plugins with config editor
- **Admin PluginManagement** (`/admin/plugins`) - System-wide plugin administration
- **PluginCard** - Display plugin with type-based color coding
- **PluginDetailModal** - Full details, reviews, permissions with risk indicators
- **PluginConfigForm** - Schema-based form generator for plugin configuration

**Security Features**:
- Permission system with risk levels (low/medium/high)
- Sandbox execution environment
- Configuration validation
- Manifest schema enforcement
- User/admin approval workflows

**Event System**:
```javascript
// Plugins can register handlers for these events:
- session.created
- session.started
- session.stopped
- session.hibernated
- session.woken
- session.deleted
- user.created
- user.updated
- user.deleted
- user.login
- user.logout
```

**Documentation**:
- `PLUGIN_DEVELOPMENT.md` - Complete developer guide with examples
- `docs/PLUGIN_API.md` - Comprehensive API reference

## Data Flow

### Session Creation Flow

1. **User clicks "Launch" in UI**
   ```
   POST /api/v1/sessions
   {
     "template": "firefox-browser",
     "resources": {"memory": "2Gi"}
   }
   ```

2. **API validates request**
   - Check user quota (max sessions, memory limit)
   - Verify template exists
   - Generate unique session name

3. **API creates Session CR**
   ```yaml
   apiVersion: stream.space/v1alpha1
   kind: Session
   metadata:
     name: user1-firefox
   spec:
     user: user1
     template: firefox-browser
     state: running
     resources:
       memory: 2Gi
   ```

4. **Controller watches Session CR**
   - Reconcile loop triggered

5. **Controller provisions resources**
   - Create/ensure user PVC exists
   - Create Deployment (with pod template)
   - Create Service
   - Create Ingress rule
   - Update Session status with URL

6. **Pod starts**
   - Container pulls image
   - KasmVNC starts on port 3000
   - User home directory mounted

7. **Status update**
   ```yaml
   status:
     phase: Running
     url: https://user1-firefox.streamspace.local
     podName: ss-user1-firefox-abc123
     lastActivity: "2025-01-15T10:00:00Z"
   ```

8. **UI polls for ready status**
   - Opens session URL in iframe or new tab

### Hibernation Flow

1. **Hibernation controller checks sessions every 60s**

2. **Detects idle session**
   - `time.Now() - lastActivity > idleTimeout` (default 30m)

3. **Updates Session state**
   ```yaml
   spec:
     state: hibernated
   ```

4. **Session reconciler scales down**
   - Set Deployment replicas to 0
   - Pod terminates (PVC persists)
   - Update Session phase to "Hibernated"

5. **User returns and clicks session**

6. **API wake endpoint**
   ```
   POST /api/v1/sessions/{id}/wake
   ```

7. **Updates Session state**
   ```yaml
   spec:
     state: running
   ```

8. **Session reconciler scales up**
   - Set Deployment replicas to 1
   - Pod starts (mounts same PVC)
   - Wait for readiness

9. **UI redirects to session URL**

## Custom Resource Definitions

### Session CRD

```yaml
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: user1-firefox
  namespace: streamspace
spec:
  user: user1
  template: firefox-browser
  state: running  # running, hibernated, terminated
  resources:
    memory: 2Gi
    cpu: 1000m
  persistentHome: true
  idleTimeout: 30m
  maxSessionDuration: 8h
status:
  phase: Running  # Pending, Running, Hibernated, Failed, Terminated
  podName: ss-user1-firefox-abc123
  url: https://user1-firefox.streamspace.local
  lastActivity: "2025-01-15T10:30:00Z"
  resourceUsage:
    memory: 1.2Gi
    cpu: 450m
```

### Template CRD

```yaml
apiVersion: stream.space/v1alpha1
kind: Template
metadata:
  name: firefox-browser
  namespace: streamspace
spec:
  displayName: Firefox Web Browser
  description: Modern web browser with privacy features
  category: Web Browsers
  icon: https://example.com/firefox-icon.png
  baseImage: lscr.io/linuxserver/firefox:latest
  defaultResources:
    memory: 2Gi
    cpu: 1000m
  ports:
    - name: vnc
      containerPort: 3000
  env:
    - name: PUID
      value: "1000"
  volumeMounts:
    - name: user-home
      mountPath: /config
  kasmvnc:
    enabled: true
    port: 3000
  capabilities: [Network, Audio, Clipboard]
  tags: [browser, web, firefox]
```

## Security Architecture

### Authentication

**SSO via Authentik**:
- OIDC provider
- JWT tokens (access + refresh)
- MFA support
- Social logins

### Authorization

**RBAC**:
- Users can only access their own sessions
- Admins can access all sessions
- Service accounts for automation

**Network Policies**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: session-isolation
spec:
  podSelector:
    matchLabels:
      app: streamspace-session
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: streamspace-ingress
  egress:
  - to:
    - podSelector: {}  # Allow DNS
    ports:
    - port: 53
  - to:
    - namespaceSelector: {}  # Internet access
```

### Data Protection

- User PVCs isolated by RBAC
- Audit logs for all actions
- Optional session recording (Phase 5)
- Secrets in Kubernetes Secrets (not ConfigMaps)

## Resource Management

### Memory Allocation

**Cluster**: 64GB total (4 × 16GB nodes)
- System overhead: 8GB
- StreamSpace platform: 4GB
- **Available for sessions**: 52GB

**Per-Session Estimates**:
- Browsers: 2GB
- IDEs: 4GB
- 3D/Video: 6-8GB

**Capacity**: ~26 lightweight or ~13 medium or ~6 heavy concurrent sessions

**With Hibernation**: Support 50+ users (20% active concurrency)

### Quota Enforcement

```go
func (r *SessionReconciler) enforceQuota(user string) error {
    // Count active sessions
    activeSessions := r.countActiveSessions(user)
    if activeSessions >= user.Quota.MaxSessions {
        return errors.New("max sessions exceeded")
    }

    // Check memory usage
    totalMemory := r.calculateTotalMemory(user)
    if totalMemory >= user.Quota.Memory {
        return errors.New("memory quota exceeded")
    }

    return nil
}
```

## Monitoring & Observability

### Metrics

**Controller Metrics**:
- Active sessions count
- Hibernated sessions count
- Session start/end events
- Hibernation events
- Resource usage (memory, CPU)
- Cluster capacity %

**API Metrics**:
- Request rate
- Error rate
- Response time (p50, p95, p99)
- Concurrent connections

### Dashboards

**Grafana "Session Overview"**:
- Active vs hibernated sessions
- Memory usage (per session, total)
- Session lifecycle events
- User activity
- API performance

### Alerts

- High memory usage (>85%)
- Provisioning failures
- Controller/API downtime
- Hibernation not working
- Long-running sessions (>12h)

## Deployment Architecture

### Helm Chart Structure

```
chart/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── controller-deployment.yaml
│   ├── api-deployment.yaml
│   ├── ui-deployment.yaml
│   ├── ingress.yaml
│   ├── servicemonitor.yaml
│   └── prometheusrule.yaml
└── crds/
    ├── session-crd.yaml
    └── template-crd.yaml
```

### High Availability (Phase 5)

**Controller HA**:
- 2+ replicas with leader election
- Kubernetes lease for coordination

**API HA**:
- 3+ replicas behind Service
- Horizontal Pod Autoscaler

**Database HA**:
- PostgreSQL with replication
- Or cloud-managed (RDS, Cloud SQL)

## Performance Considerations

### Session Provisioning

**Target**: < 30 seconds from request to accessible
- Pod scheduling: 5-10s
- Image pull (cached): 2-5s
- Container start: 10-15s
- KasmVNC ready: 5s

### Hibernation/Wake

**Hibernation**: < 5 seconds (scale to 0)
**Wake**: < 20 seconds (scale to 1, wait for ready)

### Optimization

- Pre-pull images on all nodes
- Use smaller base images (Alpine vs Ubuntu)
- Optimize readiness probes
- CRIU for instant wake (Phase 5, advanced)

## Future Enhancements

- **GPU Support**: PassThrough for 3D/gaming workspaces
- **CRIU Hibernation**: Checkpoint/restore for instant resume
- **Multi-Cluster**: Federate sessions across clusters
- **Marketplace**: Public template registry
- **Analytics**: Advanced usage insights
- **WebRTC**: Alternative to KasmVNC for lower latency

---

For implementation details, see:
- Controller: `docs/CONTROLLER_GUIDE.md`
- API: `docs/API_REFERENCE.md`
- Deployment: `docs/GETTING_STARTED.md`
