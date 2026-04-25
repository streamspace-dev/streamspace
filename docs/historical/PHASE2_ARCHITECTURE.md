# StreamSpace Phase 2: Full Platform Architecture

**Version**: 2.0
**Status**: Planning → Implementation
**Target**: Enterprise self-hosted application platform

---

## Executive Summary

Phase 2 transforms StreamSpace from a VNC streaming controller into a **complete enterprise platform** with:

- **Admin Dashboard**: Full Kubernetes cluster management UI
- **User Portal**: Application marketplace and launcher
- **Hybrid Apps**: VNC workspaces + native web applications
- **Multi-user Sessions**: Connection tracking and on-demand scaling
- **Template Marketplace**: GitHub-based app catalog with user repositories
- **Configuration Management**: Web-based settings and policy management

Think: **Rancher + Portainer + Kasm Workspaces** - but 100% open source.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Users (Browser)                          │
└────────────┬──────────────────────────────────┬─────────────────┘
             │                                  │
             │                                  │
    ┌────────▼────────┐                ┌───────▼──────────┐
    │   Admin UI      │                │   User Portal     │
    │  (React SPA)    │                │   (React SPA)     │
    │                 │                │                   │
    │ - Cluster Mgmt  │                │ - App Launcher    │
    │ - Config        │                │ - My Sessions     │
    │ - Templates     │                │ - App Catalog     │
    │ - Users         │                │ - Favorites       │
    └────────┬────────┘                └───────┬───────────┘
             │                                  │
             └──────────────┬───────────────────┘
                            │
                   ┌────────▼─────────┐
                   │   API Backend    │
                   │   (Go/Gin)       │
                   │                  │
                   │ REST API         │
                   │ WebSocket        │
                   │ SSE Streaming    │
                   │ Auth (JWT/OIDC)  │
                   └────┬──────┬──────┘
                        │      │
         ┌──────────────┘      └──────────────┐
         │                                    │
    ┌────▼─────────┐                  ┌──────▼────────┐
    │  PostgreSQL  │                  │  Kubernetes   │
    │              │                  │   API Server  │
    │ - Users      │                  │               │
    │ - Sessions   │                  │ - Sessions    │
    │ - Connections│                  │ - Templates   │
    │ - Templates  │                  │ - Deployments │
    │ - Config     │                  │ - Services    │
    │ - Audit Log  │                  │ - Ingress     │
    └──────────────┘                  └───────────────┘
```

---

## Component Architecture

### 1. API Backend (Go + Gin)

**Location**: `/api/`

**Responsibilities**:
- REST API for all operations
- WebSocket for real-time updates
- Kubernetes client integration
- PostgreSQL database management
- Authentication & authorization
- Session/connection tracking
- Template catalog management

**Endpoints**:

#### Sessions API
```
GET    /api/v1/sessions              # List all sessions
POST   /api/v1/sessions              # Create session
GET    /api/v1/sessions/:id          # Get session details
PATCH  /api/v1/sessions/:id          # Update session (hibernate/wake)
DELETE /api/v1/sessions/:id          # Delete session
GET    /api/v1/sessions/:id/connect  # Get connection URL + track
POST   /api/v1/sessions/:id/disconnect # Untrack connection
```

#### Templates API
```
GET    /api/v1/templates             # List templates
POST   /api/v1/templates             # Create template (admin)
GET    /api/v1/templates/:id         # Get template
PATCH  /api/v1/templates/:id         # Update template
DELETE /api/v1/templates/:id         # Delete template
GET    /api/v1/templates/catalog     # Get catalog from repos
POST   /api/v1/templates/install     # Install from catalog
```

#### Cluster Management API
```
GET    /api/v1/cluster/nodes         # List nodes
GET    /api/v1/cluster/pods          # List all pods
GET    /api/v1/cluster/deployments   # List deployments
GET    /api/v1/cluster/services      # List services
GET    /api/v1/cluster/namespaces    # List namespaces
POST   /api/v1/cluster/resources     # Create any resource
PATCH  /api/v1/cluster/resources     # Update any resource
DELETE /api/v1/cluster/resources     # Delete any resource
```

#### Configuration API
```
GET    /api/v1/config                # Get all config
PATCH  /api/v1/config                # Update config
GET    /api/v1/config/ingress        # Ingress settings
GET    /api/v1/config/storage        # Storage settings
GET    /api/v1/config/resources      # Resource defaults
```

#### Catalog API
```
GET    /api/v1/catalog/repositories  # List template repos
POST   /api/v1/catalog/repositories  # Add repository
DELETE /api/v1/catalog/repositories/:id # Remove repo
POST   /api/v1/catalog/sync          # Sync all repos
GET    /api/v1/catalog/templates     # Browse all templates
```

#### Users API
```
GET    /api/v1/users                 # List users (admin)
POST   /api/v1/users                 # Create user (admin)
GET    /api/v1/users/:id             # Get user
PATCH  /api/v1/users/:id             # Update user
GET    /api/v1/users/me              # Get current user
GET    /api/v1/users/:id/sessions    # Get user's sessions
```

#### Metrics & Health
```
GET    /api/v1/metrics               # Platform metrics
GET    /api/v1/health                # Health check
GET    /api/v1/version               # Version info
```

#### WebSocket
```
WS     /api/v1/ws/sessions           # Real-time session updates
WS     /api/v1/ws/cluster            # Real-time cluster events
WS     /api/v1/ws/logs/:pod          # Stream pod logs
```

**Technology Stack**:
- **Framework**: Gin (fast, lightweight)
- **Database**: PostgreSQL (users, sessions, config)
- **K8s Client**: client-go
- **Auth**: JWT tokens + OIDC integration
- **WebSocket**: gorilla/websocket
- **Metrics**: Prometheus client

### 2. Web UI (React + TypeScript)

**Location**: `/ui/`

**Structure**:
```
ui/
├── public/
├── src/
│   ├── components/
│   │   ├── admin/          # Admin dashboard components
│   │   ├── user/           # User portal components
│   │   ├── shared/         # Shared components
│   │   └── cluster/        # K8s resource components
│   ├── pages/
│   │   ├── AdminDashboard.tsx
│   │   ├── UserPortal.tsx
│   │   ├── SessionManager.tsx
│   │   ├── ClusterView.tsx
│   │   ├── TemplateMarketplace.tsx
│   │   └── ConfigurationPage.tsx
│   ├── api/                # API client
│   ├── hooks/              # React hooks
│   ├── store/              # State management (Redux/Zustand)
│   ├── types/              # TypeScript types
│   └── utils/              # Utilities
├── package.json
└── tsconfig.json
```

#### Admin Dashboard Features

**Navigation**:
- Dashboard (overview, metrics)
- Cluster Management
  - Nodes
  - Pods
  - Deployments
  - Services
  - ConfigMaps
  - Secrets
  - Namespaces
  - Custom Resources
- Sessions
  - All sessions
  - Active connections
  - Hibernated sessions
- Templates
  - Template library
  - Template catalog
  - Repository management
- Users & Access
  - User management
  - Roles & permissions
  - Quotas
- Configuration
  - Ingress settings
  - Storage settings
  - Resource defaults
  - Feature flags
- Monitoring
  - Metrics & graphs
  - Logs viewer
  - Alerts

#### User Portal Features

**Main View**:
- **App Launcher**: Grid/list of available apps
- **My Sessions**: Active and hibernated sessions
- **Favorites**: Pinned applications
- **Recent**: Recently used apps
- **Categories**: Browse by category (Browsers, Development, etc.)

**Session View**:
- Embedded iframe (for web apps)
- VNC viewer (for desktop apps)
- Connection status
- Session controls (hibernate, terminate)
- Share session (future)

**Technology Stack**:
- **Framework**: React 18 + TypeScript
- **UI Library**: Material-UI (MUI)
- **State**: Redux Toolkit or Zustand
- **Routing**: React Router v6
- **API Client**: Axios + React Query
- **WebSocket**: socket.io-client or native WebSocket
- **Charts**: Recharts or Chart.js

### 3. Enhanced CRDs

#### Session CRD Updates

**Add fields for non-VNC apps and connection tracking**:

```yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: Session
metadata:
  name: user1-grafana
spec:
  user: user1
  template: grafana
  state: running

  # NEW: Application type
  appType: webapp  # webapp | desktop | hybrid

  # NEW: Connection tracking
  minConnections: 0       # Auto-hibernate when 0
  maxConnections: 10      # Limit concurrent users
  idleTimeout: 5m         # Hibernate after last disconnect

  # NEW: Auto-start configuration
  autoStart: true         # Start on first connection request
  startupTimeout: 60s     # Max time to wait for startup

  # Existing fields
  resources: {...}
  persistentHome: true

status:
  phase: Running
  url: https://user1-grafana.streamspace.local

  # NEW: Connection tracking
  activeConnections: 3
  lastConnection: "2025-01-15T10:30:00Z"
  lastDisconnect: "2025-01-15T11:00:00Z"
  totalConnections: 156   # Lifetime counter

  # Existing fields
  podName: ss-user1-grafana-abc
  resourceUsage: {...}
```

#### Template CRD Updates

**Add webapp support**:

```yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: Template
metadata:
  name: grafana
spec:
  displayName: Grafana
  description: Metrics and monitoring dashboard
  category: Monitoring

  # NEW: Application type
  appType: webapp  # webapp | desktop | hybrid

  # NEW: For webapp - native HTTP port
  webapp:
    port: 3000
    protocol: http  # or https
    path: /          # Base path
    healthCheck: /api/health

  # VNC config (for desktop apps)
  vnc:
    enabled: false  # Not a VNC app

  # NEW: Multi-user support
  multiUser: true
  maxConcurrentUsers: 50

  # NEW: Resource scaling based on connections
  autoScaling:
    enabled: true
    minReplicas: 0    # Can scale to 0
    maxReplicas: 3
    targetConnections: 10  # Scale up when > 10 connections

  # Existing fields
  baseImage: grafana/grafana:latest
  env: [...]
  defaultResources: {...}
```

#### NEW: TemplateRepository CRD

**For managing template catalogs**:

```yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: TemplateRepository
metadata:
  name: streamspace-official
  namespace: streamspace
spec:
  url: https://github.com/streamspace/templates.git
  branch: main
  path: templates/

  # Authentication (optional)
  auth:
    type: none  # none | basic | ssh | token
    secretRef:
      name: repo-credentials

  # Sync configuration
  syncInterval: 1h
  autoSync: true

  # Access control
  public: true  # All users can see
  allowedUsers: []  # Empty = all

status:
  phase: Synced
  lastSync: "2025-01-15T10:00:00Z"
  templateCount: 150
  syncError: ""
```

#### NEW: Connection CRD

**Track active connections to sessions**:

```yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: Connection
metadata:
  name: conn-abc123
  namespace: streamspace
spec:
  sessionName: user1-grafana
  user: user1
  clientIP: 192.168.1.100
  userAgent: "Mozilla/5.0..."

status:
  connectedAt: "2025-01-15T10:30:00Z"
  lastActivity: "2025-01-15T10:35:00Z"
  active: true
```

### 4. Connection Tracking System

**Component**: Connection Tracker (in API backend)

**Responsibilities**:
- Track active connections per session
- Auto-start hibernated sessions on connect request
- Auto-hibernate sessions when all users disconnect
- Enforce max connection limits
- Update Connection CRDs

**Flow**:

```
1. User clicks "Launch App" in UI
   ↓
2. API checks Session status
   - If Running: Return URL + create Connection
   - If Hibernated: Wake + wait + return URL
   - If Not Exists: Create + wait + return URL
   ↓
3. User connects (iframe loads)
   - API creates Connection CRD
   - Increments activeConnections counter
   - Updates lastConnection timestamp
   ↓
4. User closes tab/logs out
   - Frontend sends disconnect event
   - API deletes Connection CRD
   - Decrements activeConnections counter
   - If count == 0 && idleTimeout: schedule hibernation
   ↓
5. Hibernation Controller (separate)
   - Watches for activeConnections == 0
   - Waits idleTimeout duration
   - Sets state=hibernated if still 0
```

**Heartbeat System**:
- Fronten sends heartbeat every 30s
- API updates Connection.lastActivity
- Stale connections (>60s) auto-cleaned
- Prevents connection leaks

### 5. Template Catalog/Marketplace

**Architecture**:

```
GitHub Repositories
├── streamspace/templates (official)
│   └── templates/
│       ├── browsers/
│       ├── development/
│       ├── productivity/
│       └── monitoring/
│
└── user-org/custom-templates (user repo)
    └── templates/
        └── my-app/
            ├── template.yaml
            ├── icon.png
            └── README.md
```

**Catalog Service** (in API backend):

```go
type CatalogService struct {
    repos []TemplateRepository
}

func (s *CatalogService) SyncRepositories() {
    for repo in repos {
        // Git clone/pull
        // Parse template YAMLs
        // Store in database
        // Update TemplateRepository status
    }
}

func (s *CatalogService) BrowseTemplates(category, search string) []Template {
    // Query database
    // Filter by category/search
    // Return templates from all repos
}

func (s *CatalogService) InstallTemplate(repoName, templateName string) {
    // Get template YAML from repo
    // Create Template CRD in cluster
    // Track installation
}
```

**UI Marketplace View**:
- Grid/list of all templates from all repos
- Filter by: category, repo, tags
- Search by name/description
- Template details modal
- One-click install
- User ratings/reviews (future)
- Usage statistics

### 6. Cluster Management UI

**Resource Views** (admin only):

**Nodes View**:
- List all nodes
- CPU/Memory usage graphs
- Taints/labels display
- Drain/cordon actions

**Pods View**:
- List pods across all namespaces
- Status indicators
- Resource usage
- Logs viewer
- Shell access (exec)
- Delete/restart actions

**Deployments View**:
- List deployments
- Replica counts
- Update strategy
- Scale actions
- Rollout history
- YAML editor

**Services View**:
- List services
- Endpoints
- Type (ClusterIP, NodePort, LoadBalancer)
- Port mappings

**Generic Resource View**:
- List any CRD/resource type
- YAML viewer/editor
- Create from YAML
- Delete resources

**Technology**:
- Use Kubernetes JavaScript client
- Real-time updates via WebSocket
- YAML editor: Monaco Editor (VSCode editor)
- Terminal: xterm.js for pod shell access

### 7. Configuration Management

**ConfigMap-based** (existing) + **Database-backed** (new):

**Database Tables**:

```sql
-- Platform configuration
CREATE TABLE configuration (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT,
    type VARCHAR(50),  -- string, int, bool, json
    category VARCHAR(100),  -- ingress, storage, etc.
    description TEXT,
    updated_at TIMESTAMP,
    updated_by VARCHAR(255)
);

-- Audit log
CREATE TABLE audit_log (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255),
    action VARCHAR(100),  -- create_session, update_config, etc.
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    changes JSONB,
    timestamp TIMESTAMP,
    ip_address VARCHAR(45)
);
```

**Configuration UI**:
- Form-based editing
- Validation
- Test changes before apply
- Rollback to previous version
- Audit trail

---

## Database Schema

### Tables

```sql
-- Users (or use OIDC exclusively)
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    name VARCHAR(255),
    role VARCHAR(50),  -- admin, user
    created_at TIMESTAMP,
    last_login TIMESTAMP
);

-- Sessions cache (mirror of K8s Sessions)
CREATE TABLE sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) REFERENCES users(id),
    template_name VARCHAR(255),
    state VARCHAR(50),
    app_type VARCHAR(50),
    active_connections INT DEFAULT 0,
    url TEXT,
    created_at TIMESTAMP,
    last_connection TIMESTAMP,
    last_disconnect TIMESTAMP
);

-- Active connections
CREATE TABLE connections (
    id VARCHAR(255) PRIMARY KEY,
    session_id VARCHAR(255) REFERENCES sessions(id),
    user_id VARCHAR(255) REFERENCES users(id),
    client_ip VARCHAR(45),
    user_agent TEXT,
    connected_at TIMESTAMP,
    last_heartbeat TIMESTAMP
);

-- Template catalog cache
CREATE TABLE catalog_templates (
    id SERIAL PRIMARY KEY,
    repository VARCHAR(255),
    name VARCHAR(255),
    display_name VARCHAR(255),
    description TEXT,
    category VARCHAR(100),
    app_type VARCHAR(50),
    icon_url TEXT,
    manifest JSONB,  -- Full template YAML as JSON
    tags TEXT[],
    install_count INT DEFAULT 0,
    created_at TIMESTAMP,
    UNIQUE(repository, name)
);

-- Template repositories
CREATE TABLE repositories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE,
    url TEXT,
    branch VARCHAR(100),
    last_sync TIMESTAMP,
    template_count INT,
    status VARCHAR(50)  -- synced, error, syncing
);

-- Configuration (as above)
CREATE TABLE configuration (...);

-- Audit log (as above)
CREATE TABLE audit_log (...);
```

---

## Implementation Phases

### Phase 2.1: API Foundation (Week 1-2)
- ✅ Create API backend structure
- ✅ REST API for sessions
- ✅ Kubernetes client integration
- ✅ PostgreSQL setup
- ✅ Basic authentication

### Phase 2.2: Connection Tracking (Week 2-3)
- ✅ Connection CRD
- ✅ Connection tracker service
- ✅ Auto-start/hibernate logic
- ✅ Heartbeat system
- ✅ Multi-user session support

### Phase 2.3: Template Catalog (Week 3-4)
- ✅ TemplateRepository CRD
- ✅ Catalog service (Git sync)
- ✅ Template browsing API
- ✅ Install from catalog

### Phase 2.4: Web UI Foundation (Week 4-5)
- ✅ React project setup
- ✅ User portal layout
- ✅ Session launcher
- ✅ My Sessions view
- ✅ Template marketplace

### Phase 2.5: Admin Dashboard (Week 5-7)
- ✅ Admin layout
- ✅ Cluster management views
- ✅ Template management
- ✅ User management
- ✅ Configuration UI

### Phase 2.6: Advanced Features (Week 7-8)
- ✅ WebSocket real-time updates
- ✅ Pod logs viewer
- ✅ Terminal (exec into pods)
- ✅ YAML editor
- ✅ Metrics dashboards

---

## Technology Decisions

### Why Gin (vs FastAPI, Express)?
- ✅ Native Go - same language as controller
- ✅ Excellent performance (fastest web framework)
- ✅ Built-in middleware
- ✅ Smaller binary size
- ✅ Better K8s client-go integration

### Why React (vs Vue, Angular)?
- ✅ Largest ecosystem
- ✅ MUI component library
- ✅ TypeScript support
- ✅ React Query for server state
- ✅ Industry standard

### Why PostgreSQL (vs MongoDB, Redis)?
- ✅ ACID compliance
- ✅ JSONB for flexibility
- ✅ Excellent Go support
- ✅ Proven reliability
- ✅ SQL for complex queries

---

## Security Considerations

### Authentication
- **OIDC Integration**: Authentik, Keycloak, Auth0
- **JWT Tokens**: Short-lived access tokens
- **Refresh Tokens**: Long-lived for renewal
- **RBAC**: Role-based access control

### Authorization
- **Session Isolation**: Users can only see their sessions
- **Admin Privileges**: Full cluster access for admins
- **Resource Quotas**: Per-user limits
- **Namespace Isolation**: Optional multi-tenancy

### Network Security
- **TLS Everywhere**: HTTPS + WSS
- **Ingress with TLS**: cert-manager integration
- **Secret Management**: Kubernetes secrets for credentials
- **CORS**: Proper CORS configuration

---

## Deployment Architecture

```
┌─────────────────────────────────────────────┐
│            Ingress Controller                │
│         (Traefik/Nginx + cert-manager)       │
└──┬────────────────┬─────────────────┬────────┘
   │                │                 │
   │                │                 │
┌──▼──────┐  ┌──────▼──────┐  ┌──────▼─────────┐
│ Web UI  │  │ API Backend │  │ User Sessions  │
│ (nginx) │  │ (Gin)       │  │ (Dynamic)      │
└─────────┘  └──────┬──────┘  └────────────────┘
                    │
          ┌─────────┴──────────┐
          │                    │
    ┌─────▼─────┐      ┌───────▼────────┐
    │PostgreSQL │      │  Controller    │
    │           │      │  (Existing)    │
    └───────────┘      └────────────────┘
```

---

## Next Steps

1. **Create API backend structure** ✅ Starting now
2. **Update CRDs** for webapp support
3. **Build connection tracker**
4. **Create UI foundation**
5. **Implement template catalog**
6. **Build admin dashboard**

Ready to transform StreamSpace into a complete platform! 🚀
