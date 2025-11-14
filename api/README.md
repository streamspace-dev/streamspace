# StreamSpace API Backend

Go-based REST API backend for StreamSpace, providing session management, template catalog, cluster administration, and connection tracking.

## Overview

The API backend serves as the central control plane for StreamSpace, interfacing between:
- **PostgreSQL database** - User data, session cache, connection tracking, audit logs
- **Kubernetes API** - Session/Template CRDs, cluster resources
- **Web UI** - REST endpoints + WebSocket for real-time updates

## Architecture

```
┌─────────────┐      REST/WS      ┌──────────────┐
│   Web UI    │ ◄────────────────► │  API Backend │
│  (React)    │                    │   (Go/Gin)   │
└─────────────┘                    └──────┬───────┘
                                          │
                        ┌─────────────────┼─────────────────┐
                        │                 │                 │
                        ▼                 ▼                 ▼
                 ┌─────────────┐   ┌─────────┐     ┌──────────────┐
                 │ PostgreSQL  │   │   K8s   │     │  Connection  │
                 │  Database   │   │   API   │     │   Tracker    │
                 └─────────────┘   └─────────┘     └──────────────┘
```

## Features

### ✅ Phase 2.1 (Completed)

- **Database Layer** (`internal/db/`)
  - PostgreSQL connection management
  - Complete schema with 7 tables
  - Migration system
  - User accounts, session cache, connections, repositories, catalog, config, audit logs

- **Kubernetes Client** (`internal/k8s/`)
  - Session CRUD operations
  - Template CRUD operations
  - Cluster resource queries (nodes, pods, services, PVCs, namespaces)
  - Watch streams for real-time updates

- **Connection Tracker** (`internal/tracker/`)
  - Active connection monitoring
  - Auto-start hibernated sessions on connect
  - Auto-hibernate sessions when all users disconnect
  - Heartbeat-based connection staleness detection

- **API Handlers** (`internal/api/`)
  - **Session Endpoints**: List, Get, Create, Update, Delete
  - **Connection Endpoints**: Connect, Disconnect, Heartbeat, Get Connections
  - **Template Endpoints**: List, Get, Create, Delete
  - **Catalog Endpoints**: List marketplace templates, Install from catalog
  - **Repository Endpoints**: List, Add, Sync, Delete template repositories

- **Enhanced CRDs**
  - Template CRD: Added `appType` field and `webapp` configuration
  - New Connection CRD: Track active user connections
  - New TemplateRepository CRD: Manage Git-based template repositories

### ✅ Phase 2.2 - Repository Sync (Completed)

- **Sync Service** (`internal/sync/`)
  - Git repository cloning and pulling
  - Template YAML parsing and validation
  - Catalog database population
  - Scheduled sync jobs (configurable interval)
  - Manual sync triggers via API
  - Webhook support for auto-sync on Git push

- **Git Client** (`internal/sync/git.go`)
  - Clone repositories with depth=1 for faster operations
  - Pull latest changes (fetch + reset --hard)
  - Authentication support: SSH keys, tokens, basic auth
  - GitHub and GitLab URL formatting
  - Git version validation

- **Template Parser** (`internal/sync/parser.go`)
  - YAML parsing with gopkg.in/yaml.v3
  - Template manifest validation
  - App type inference (desktop vs webapp)
  - JSON conversion for database storage
  - Recursive repository scanning

- **Enhanced API Endpoints**
  - `POST /api/v1/catalog/sync` - Trigger sync for all repositories
  - `POST /api/v1/catalog/repositories/:id/sync` - Trigger sync for specific repository
  - `POST /webhooks/repository/sync` - Webhook endpoint for Git providers

- **Scheduled Sync**
  - Automatic background sync (default: 1 hour interval)
  - Configurable via `SYNC_INTERVAL` env var
  - Initial sync on startup
  - Repository status tracking (pending, syncing, synced, failed)

### ✅ Phase 2.4 - WebSocket & Real-Time (Completed)

- **WebSocket Hub Infrastructure** (`internal/websocket/hub.go`)
  - Hub/Client pattern for managing WebSocket connections
  - Thread-safe client registration/unregistration
  - Broadcast messaging to all connected clients
  - Read/write pumps for bidirectional communication
  - Automatic cleanup on disconnect
  - Client connection counting

- **WebSocket Handlers** (`internal/websocket/handlers.go`)
  - Manager orchestrates multiple WebSocket hubs (sessions, metrics)
  - Real-time session updates (broadcasts every 3 seconds)
  - Real-time cluster metrics (broadcasts every 5 seconds)
  - Pod logs streaming (tail -f style with timestamps)
  - Enriched session data with active connections from database
  - JSON message format with type field for different update types

- **WebSocket Endpoints**
  - `WS /api/v1/ws/sessions` - Real-time session updates with active connections
  - `WS /api/v1/ws/cluster` - Real-time cluster metrics (sessions, connections, repositories)
  - `WS /api/v1/ws/logs/:namespace/:pod` - Streaming pod logs with timestamps

- **Message Formats**
  ```json
  // Sessions update
  {
    "type": "sessions_update",
    "sessions": [...],
    "count": 5,
    "timestamp": "2025-01-15T10:30:00Z"
  }

  // Metrics update
  {
    "type": "metrics_update",
    "metrics": {
      "sessions": {"running": 3, "hibernated": 2, "total": 5},
      "activeConnections": 7,
      "repositories": 2,
      "templates": 150
    },
    "timestamp": "2025-01-15T10:30:05Z"
  }

  // Pod logs (raw text messages)
  "2025-01-15T10:30:10Z Starting application..."
  ```

- **Frontend WebSocket Integration**
  - Custom React hook (`useWebSocket`) with automatic reconnection
  - Exponential backoff reconnection strategy (3s, 4.5s, 6.75s, ..., max 30s)
  - Max 10 reconnection attempts
  - Real-time Dashboard updates (no polling)
  - Real-time Sessions page updates (no polling)
  - Connection status indicators in UI
  - Reconnection attempt tracking

## Directory Structure

```
api/
├── cmd/
│   └── main.go                    # Server entry point (270 lines)
├── internal/
│   ├── db/
│   │   └── database.go            # Database layer (200 lines)
│   ├── k8s/
│   │   └── client.go              # K8s client wrapper (700+ lines)
│   ├── tracker/
│   │   └── tracker.go             # Connection tracker (450+ lines)
│   ├── sync/
│   │   ├── sync.go                # Sync service (220+ lines)
│   │   ├── git.go                 # Git client (150+ lines)
│   │   └── parser.go              # Template parser (250+ lines)
│   ├── websocket/
│   │   ├── hub.go                 # WebSocket hub infrastructure (160+ lines)
│   │   └── handlers.go            # WebSocket handlers (280+ lines)
│   └── api/
│       ├── handlers.go            # API handlers (700+ lines)
│       └── stubs.go               # Stub handlers and webhooks (265+ lines)
├── go.mod                         # Go module definition
└── README.md                      # This file
```

## Database Schema

### Tables

1. **users** - User accounts (or OIDC integration)
   - id, email, name, role, created_at, last_login

2. **sessions** - Cache of Kubernetes Sessions
   - id, user_id, template_name, state, app_type, active_connections, url, namespace, timestamps

3. **connections** - Track active user connections per session
   - id, session_id, user_id, client_ip, user_agent, connected_at, last_heartbeat

4. **repositories** - Git repositories for template catalog
   - id, name, url, branch, auth_type, auth_secret, last_sync, template_count, status

5. **catalog_templates** - Template marketplace cache
   - id, repository_id, name, display_name, description, category, app_type, manifest, tags, install_count

6. **configuration** - Platform configuration settings
   - key, value, type, category, description, updated_at, updated_by

7. **audit_log** - Audit trail of all actions
   - id, user_id, action, resource_type, resource_id, changes (JSONB), timestamp, ip_address

## API Endpoints

### Session Management

```
GET    /api/v1/sessions             # List sessions (filter by ?user=username)
GET    /api/v1/sessions/:id         # Get session details
POST   /api/v1/sessions             # Create new session
PATCH  /api/v1/sessions/:id         # Update session (state change)
DELETE /api/v1/sessions/:id         # Delete session

GET    /api/v1/sessions/:id/connect      # Connect to session (returns connectionId)
DELETE /api/v1/sessions/:id/disconnect   # Disconnect from session
POST   /api/v1/sessions/:id/heartbeat    # Send heartbeat for connection
GET    /api/v1/sessions/:id/connections  # Get active connections
```

### Template Management

```
GET    /api/v1/templates            # List templates (filter by ?category=...)
GET    /api/v1/templates/:id        # Get template details
POST   /api/v1/templates            # Create template (admin)
DELETE /api/v1/templates/:id        # Delete template (admin)
```

### Template Catalog (Marketplace)

```
GET    /api/v1/catalog/templates    # List catalog templates (filter by ?category=... or ?tag=...)
POST   /api/v1/catalog/templates/:id/install  # Install template from catalog
```

### Repository Management

```
GET    /api/v1/repositories         # List template repositories
POST   /api/v1/repositories         # Add new repository
POST   /api/v1/repositories/:id/sync   # Trigger repository sync
DELETE /api/v1/repositories/:id     # Delete repository
```

### WebSocket Endpoints

```
WS     /api/v1/ws/sessions                  # Real-time session updates (broadcast every 3s)
WS     /api/v1/ws/cluster                   # Real-time cluster metrics (broadcast every 5s)
WS     /api/v1/ws/logs/:namespace/:pod      # Pod logs streaming (tail -f with timestamps)
```

**WebSocket Message Formats:**

Sessions Update (every 3 seconds):
```json
{
  "type": "sessions_update",
  "sessions": [
    {
      "name": "user1-firefox",
      "user": "user1",
      "template": "firefox-browser",
      "state": "running",
      "status": "Running",
      "activeConnections": 2,
      "resources": {"memory": "2Gi", "cpu": "1000m"},
      "createdAt": "2025-01-15T10:00:00Z"
    }
  ],
  "count": 1,
  "timestamp": "2025-01-15T10:30:00Z"
}
```

Metrics Update (every 5 seconds):
```json
{
  "type": "metrics_update",
  "metrics": {
    "sessions": {
      "running": 3,
      "hibernated": 2,
      "total": 5
    },
    "activeConnections": 7,
    "repositories": 2,
    "templates": 150
  },
  "timestamp": "2025-01-15T10:30:05Z"
}
```

Pod Logs (streaming):
```
2025-01-15T10:30:10Z Starting application...
2025-01-15T10:30:11Z Server listening on port 3000
```

## Configuration

### Environment Variables

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=streamspace
DB_PASSWORD=changeme
DB_NAME=streamspace
DB_SSLMODE=disable

# Kubernetes
KUBECONFIG=/path/to/kubeconfig  # Optional, uses in-cluster config if not set
NAMESPACE=streamspace            # Default namespace

# Repository Sync
SYNC_WORK_DIR=/tmp/streamspace-repos  # Directory for cloned repos
SYNC_INTERVAL=1h                      # Scheduled sync interval

# Server
PORT=8080
GIN_MODE=release                # release or debug
```

## Running Locally

### Prerequisites

- Go 1.21+
- PostgreSQL 13+
- Kubernetes cluster with StreamSpace CRDs installed
- kubectl configured

### Steps

1. **Start PostgreSQL**:
```bash
docker run -d \
  --name postgres \
  -e POSTGRES_USER=streamspace \
  -e POSTGRES_PASSWORD=changeme \
  -e POSTGRES_DB=streamspace \
  -p 5432:5432 \
  postgres:15
```

2. **Set environment variables**:
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=streamspace
export DB_PASSWORD=changeme
export DB_NAME=streamspace
export NAMESPACE=streamspace
```

3. **Run the server**:
```bash
cd api
go run cmd/main.go
```

The API will start on `http://localhost:8080`.

### Testing Endpoints

```bash
# List sessions
curl http://localhost:8080/api/v1/sessions

# Create session
curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "user": "alice",
    "template": "firefox-browser",
    "resources": {
      "memory": "2Gi",
      "cpu": "1000m"
    }
  }'

# Get session
curl http://localhost:8080/api/v1/sessions/alice-firefox-abc123

# Connect to session
curl http://localhost:8080/api/v1/sessions/alice-firefox-abc123/connect?user=alice

# Send heartbeat
curl -X POST "http://localhost:8080/api/v1/sessions/alice-firefox-abc123/heartbeat?connectionId=conn-xyz"

# List templates
curl http://localhost:8080/api/v1/templates

# List catalog templates
curl http://localhost:8080/api/v1/catalog/templates?category=Browsers

# Add template repository
curl -X POST http://localhost:8080/api/v1/catalog/repositories \
  -H "Content-Type: application/json" \
  -d '{
    "name": "community-templates",
    "url": "https://github.com/streamspace/templates",
    "branch": "main",
    "authType": "none"
  }'

# List repositories
curl http://localhost:8080/api/v1/catalog/repositories

# Trigger sync for specific repository
curl -X POST http://localhost:8080/api/v1/catalog/repositories/1/sync

# Trigger sync for all repositories
curl -X POST http://localhost:8080/api/v1/catalog/sync

# Webhook (Git provider pushes to this)
curl -X POST http://localhost:8080/webhooks/repository/sync \
  -H "Content-Type: application/json" \
  -d '{
    "repository_url": "https://github.com/streamspace/templates",
    "branch": "main",
    "ref": "refs/heads/main"
  }'
```

## Connection Tracking & Auto-Hibernation

### How It Works

1. **User connects** → API creates Connection record
2. **Connection Tracker** monitors heartbeats every 30 seconds
3. **Auto-start**: If session is hibernated, tracker updates state to "running"
4. **Heartbeats**: Client sends heartbeat every 30s to keep connection alive
5. **Stale Detection**: Connections without heartbeat for 60s are marked stale and removed
6. **Auto-hibernate**: When last connection closes, tracker waits for idle timeout (default 30m), then hibernates session

### Connection Flow

```
┌──────┐                 ┌─────────┐                 ┌───────────┐
│ User │                 │   API   │                 │  Tracker  │
└──┬───┘                 └────┬────┘                 └─────┬─────┘
   │                          │                            │
   │ GET /sessions/:id/connect│                            │
   ├─────────────────────────►│                            │
   │                          │ Create Connection          │
   │                          ├───────────────────────────►│
   │                          │                            │ Auto-start
   │                          │                            │ if hibernated
   │◄─────────────────────────┤                            │
   │ {connectionId, url}      │                            │
   │                          │                            │
   │ POST /sessions/:id/heartbeat (every 30s)              │
   ├─────────────────────────►│                            │
   │                          │ Update heartbeat           │
   │                          ├───────────────────────────►│
   │                          │                            │
   │                          │                            │ Check connections
   │                          │                            │ every 30s
   │                          │                            │
   │ DELETE /sessions/:id/disconnect                       │
   ├─────────────────────────►│                            │
   │                          │ Remove connection          │
   │                          ├───────────────────────────►│
   │                          │                            │ Auto-hibernate
   │                          │                            │ after idle timeout
```

## Deployment

### Docker Build

```bash
cd api

# Build image
docker build -t streamspace-api:latest .

# Run container
docker run -d \
  --name streamspace-api \
  -p 8080:8080 \
  -e DB_HOST=postgres \
  -e DB_PASSWORD=changeme \
  -e KUBECONFIG=/config/kubeconfig \
  -v ~/.kube/config:/config/kubeconfig:ro \
  streamspace-api:latest
```

### Kubernetes Deployment

See `manifests/config/api-deployment.yaml` for Kubernetes deployment manifest.

```bash
kubectl apply -f manifests/config/api-deployment.yaml
```

## Development Roadmap

### ✅ Phase 2.1 - API Foundation (Completed)
- Database layer with complete schema
- K8s client wrapper
- Connection tracker with auto-start/hibernate
- REST API handlers
- Enhanced CRDs (webapp support, Connection, TemplateRepository)

### ✅ Phase 2.2 - Repository Sync (Completed)
- Git repository cloning and syncing
- Template manifest parsing (YAML)
- Catalog database population
- Scheduled sync jobs
- Webhook support for auto-sync

### 📋 Phase 2.3 - Authentication & Authorization (Next)
- JWT token validation
- OIDC integration (Authentik/Keycloak)
- Role-based access control (RBAC)
- User quotas and limits
- API key support

### 📋 Phase 2.4 - WebSocket & Real-Time
- Session status updates via WebSocket
- Pod logs streaming
- Terminal (exec) support
- Metrics streaming

### 📋 Phase 2.5 - Cluster Management
- Node management endpoints
- Deployment management
- Service management
- PVC management
- YAML editor support

### 📋 Phase 2.6 - Advanced Features
- Session templates (user-defined configurations)
- Scheduled sessions (auto-start at specific times)
- Session sharing (multiple users, permissions)
- Resource usage analytics
- Cost tracking

## Contributing

See the main [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.

## License

MIT License - See [LICENSE](../LICENSE)

---

**Built with**:
- [Go](https://golang.org/) - Programming language
- [Gin](https://github.com/gin-gonic/gin) - Web framework
- [client-go](https://github.com/kubernetes/client-go) - Kubernetes client
- [lib/pq](https://github.com/lib/pq) - PostgreSQL driver
- [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) - YAML parsing
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket support
