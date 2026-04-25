# StreamSpace v2.0 API Reference

**Version**: 2.0.0-beta.1
**Date**: 2025-11-22
**Base URL**: `https://streamspace.example.com/api/v1`
**Status**: Production Ready

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Agent Management](#agent-management)
4. [Session Lifecycle](#session-lifecycle)
5. [Template Management](#template-management)
6. [User Management](#user-management)
7. [VNC Proxy](#vnc-proxy)
8. [WebSocket Protocol](#websocket-protocol)
9. [Error Handling](#error-handling)
10. [Rate Limiting](#rate-limiting)
11. [Examples](#examples)

---

## Overview

StreamSpace v2.0 Control Plane API provides RESTful HTTP endpoints and WebSocket connections for managing multi-platform container streaming infrastructure.

### Key Concepts

- **Control Plane**: Central API server coordinating agents and sessions
- **Agent**: Platform-specific executor (Kubernetes, Docker) managing sessions
- **Session**: User's containerized application instance
- **Template**: Application definition (image, resources, VNC config)
- **VNC Proxy**: WebSocket tunnel for VNC connections through Control Plane

### API Characteristics

- **Protocol**: HTTP/1.1, HTTPS, WebSocket (WSS)
- **Data Format**: JSON (request/response bodies)
- **Authentication**: JWT (JSON Web Tokens)
- **Versioning**: URI path versioning (`/api/v1`)
- **Character Encoding**: UTF-8

### Base URLs

| Environment | Base URL | WebSocket URL |
|-------------|----------|---------------|
| Production | `https://streamspace.example.com/api/v1` | `wss://streamspace.example.com` |
| Development | `http://localhost:8080/api/v1` | `ws://localhost:8080` |

---

## Authentication

StreamSpace uses **JWT (JSON Web Tokens)** for API authentication.

### Login

Obtain a JWT token by authenticating with username and password.

**Endpoint**: `POST /auth/login`

**Request Body**:
```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response** (200 OK):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "admin",
    "email": "admin@example.com",
    "role": "admin",
    "created_at": "2025-11-01T10:00:00Z"
  },
  "expires_at": "2025-11-23T10:00:00Z"
}
```

**Response** (401 Unauthorized):
```json
{
  "error": "Authentication failed",
  "message": "Invalid username or password",
  "code": "AUTH_INVALID_CREDENTIALS"
}
```

**Example**:
```bash
# Login and save token
curl -X POST https://streamspace.example.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}' \
  | jq -r .token > token.txt

# Use token in subsequent requests
TOKEN=$(cat token.txt)
```

### Logout

Invalidate the current JWT token.

**Endpoint**: `POST /auth/logout`

**Headers**:
- `Authorization: Bearer <token>`

**Response** (200 OK):
```json
{
  "message": "Logout successful"
}
```

### Using JWT Tokens

Include the JWT token in the `Authorization` header for all authenticated requests:

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Example**:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/sessions
```

### Token Expiration

- **Default TTL**: 24 hours
- **Refresh**: Re-login to obtain a new token
- **Validation**: Tokens are validated on every request
- **Revocation**: Logout endpoint invalidates the token server-side

---

## Agent Management

Agents are platform-specific executors (Kubernetes, Docker) that connect to the Control Plane via WebSocket and manage session lifecycle.

### List Agents

Get all registered agents.

**Endpoint**: `GET /agents`

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `platform` | string | Filter by platform (`kubernetes`, `docker`) | - |
| `status` | string | Filter by status (`online`, `offline`, `draining`) | - |
| `region` | string | Filter by region | - |
| `page` | integer | Page number (1-indexed) | 1 |
| `limit` | integer | Results per page (max 100) | 20 |

**Response** (200 OK):
```json
{
  "agents": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "agent_id": "k8s-prod-us-east-1",
      "platform": "kubernetes",
      "region": "us-east-1",
      "status": "online",
      "capacity": {
        "max_cpu": 100,
        "max_memory": 256,
        "max_sessions": 100,
        "current_sessions": 12
      },
      "metadata": {
        "cluster_name": "prod-k8s-cluster",
        "kubernetes_version": "v1.28.0",
        "agent_version": "v2.0-beta.1"
      },
      "websocket_conn_id": "conn-abc123",
      "last_heartbeat": "2025-11-22T10:35:00Z",
      "created_at": "2025-11-01T08:00:00Z",
      "updated_at": "2025-11-22T10:35:00Z"
    },
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "agent_id": "docker-host-01",
      "platform": "docker",
      "region": "us-east-1",
      "status": "online",
      "capacity": {
        "max_sessions": 50,
        "current_sessions": 5
      },
      "metadata": {
        "docker_version": "24.0.7",
        "agent_version": "v2.0-beta.1",
        "ha_backend": "redis",
        "is_leader": true
      },
      "websocket_conn_id": "conn-def456",
      "last_heartbeat": "2025-11-22T10:35:02Z",
      "created_at": "2025-11-15T10:00:00Z",
      "updated_at": "2025-11-22T10:35:02Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 2,
    "total_pages": 1
  }
}
```

**Example**:
```bash
# List all online Kubernetes agents
curl -H "Authorization: Bearer $TOKEN" \
  "https://streamspace.example.com/api/v1/agents?platform=kubernetes&status=online"
```

### Get Agent Details

Get detailed information about a specific agent.

**Endpoint**: `GET /agents/{agent_id}`

**Path Parameters**:
- `agent_id`: Agent identifier (e.g., `k8s-prod-us-east-1`)

**Response** (200 OK):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_id": "k8s-prod-us-east-1",
  "platform": "kubernetes",
  "region": "us-east-1",
  "status": "online",
  "capacity": {
    "max_cpu": 100,
    "max_memory": 256,
    "max_sessions": 100,
    "current_sessions": 12
  },
  "metadata": {
    "cluster_name": "prod-k8s-cluster",
    "kubernetes_version": "v1.28.0",
    "agent_version": "v2.0-beta.1",
    "ha_enabled": true,
    "is_leader": true,
    "lease_name": "k8s-agent-leader"
  },
  "websocket_conn_id": "conn-abc123",
  "last_heartbeat": "2025-11-22T10:35:00Z",
  "uptime_seconds": 2592000,
  "sessions": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "name": "admin-firefox-browser-abc123",
      "user": "admin",
      "template": "firefox-browser",
      "state": "running"
    }
  ],
  "created_at": "2025-11-01T08:00:00Z",
  "updated_at": "2025-11-22T10:35:00Z"
}
```

**Response** (404 Not Found):
```json
{
  "error": "Agent not found",
  "message": "No agent with ID 'invalid-agent-id' exists",
  "code": "AGENT_NOT_FOUND"
}
```

**Example**:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/agents/k8s-prod-us-east-1
```

### Update Agent Status

Update agent status (admin only, typically used for draining).

**Endpoint**: `PATCH /agents/{agent_id}`

**Path Parameters**:
- `agent_id`: Agent identifier

**Request Body**:
```json
{
  "status": "draining"
}
```

**Valid Status Values**:
- `online`: Agent accepting new sessions
- `draining`: Agent not accepting new sessions (existing sessions continue)
- `offline`: Agent disconnected (set automatically by Control Plane)

**Response** (200 OK):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_id": "k8s-prod-us-east-1",
  "status": "draining",
  "updated_at": "2025-11-22T10:40:00Z"
}
```

**Example**:
```bash
# Set agent to draining (prevent new sessions)
curl -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"draining"}' \
  https://streamspace.example.com/api/v1/agents/k8s-prod-us-east-1
```

### Agent Statistics

Get aggregated statistics across all agents.

**Endpoint**: `GET /agents/stats`

**Response** (200 OK):
```json
{
  "total_agents": 5,
  "online_agents": 4,
  "offline_agents": 0,
  "draining_agents": 1,
  "by_platform": {
    "kubernetes": 3,
    "docker": 2
  },
  "total_capacity": {
    "max_sessions": 300,
    "current_sessions": 45
  },
  "utilization": {
    "percentage": 15.0,
    "sessions_available": 255
  }
}
```

**Example**:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/agents/stats
```

---

## Session Lifecycle

Sessions represent user's containerized application instances managed by agents.

### Create Session

Create a new session for a user.

**Endpoint**: `POST /sessions`

**Request Body**:
```json
{
  "user": "john.doe",
  "template": "firefox-browser",
  "platform": "kubernetes",
  "state": "running",
  "resources": {
    "memory": "2Gi",
    "cpu": "1000m"
  },
  "persistent_home": true,
  "idle_timeout": "30m",
  "tags": ["project-alpha", "development"]
}
```

**Request Fields**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `user` | string | Yes | Username |
| `template` | string | Yes | Template name |
| `platform` | string | No | Platform (`kubernetes`, `docker`) - auto-selected if omitted |
| `state` | string | No | Initial state (`running`, `hibernated`) - default: `running` |
| `resources` | object | No | Resource overrides |
| `resources.memory` | string | No | Memory limit (e.g., `2Gi`) |
| `resources.cpu` | string | No | CPU limit (e.g., `1000m`) |
| `persistent_home` | boolean | No | Enable persistent home directory - default: `true` |
| `idle_timeout` | string | No | Auto-hibernate timeout (e.g., `30m`) - default: template value |
| `tags` | array | No | Session tags for filtering/organization |

**Response** (202 Accepted):
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "name": "john-doe-firefox-browser-abc123",
  "namespace": "streamspace",
  "user": "john.doe",
  "template": "firefox-browser",
  "platform": "kubernetes",
  "agent_id": "k8s-prod-us-east-1",
  "state": "pending",
  "resources": {
    "memory": "2Gi",
    "cpu": "1000m"
  },
  "persistent_home": true,
  "idle_timeout": "30m",
  "tags": ["project-alpha", "development"],
  "status": {
    "phase": "Pending",
    "message": "Session provisioning in progress...",
    "pod_ip": null,
    "vnc_url": null
  },
  "created_at": "2025-11-22T10:45:00Z",
  "updated_at": "2025-11-22T10:45:00Z"
}
```

**Response** (400 Bad Request):
```json
{
  "error": "Invalid request",
  "message": "Template 'invalid-template' does not exist",
  "code": "TEMPLATE_NOT_FOUND"
}
```

**Response** (503 Service Unavailable):
```json
{
  "error": "No agents available",
  "message": "No online agents available for platform 'kubernetes'",
  "code": "NO_AGENTS_AVAILABLE"
}
```

**Example**:
```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "john.doe",
    "template": "firefox-browser",
    "state": "running",
    "resources": {
      "memory": "2Gi"
    }
  }' \
  https://streamspace.example.com/api/v1/sessions
```

### List Sessions

Get all sessions (optionally filtered).

**Endpoint**: `GET /sessions`

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `user` | string | Filter by username | - |
| `template` | string | Filter by template | - |
| `platform` | string | Filter by platform | - |
| `agent_id` | string | Filter by agent | - |
| `state` | string | Filter by state (`pending`, `running`, `hibernated`, `terminating`, `terminated`) | - |
| `tags` | string | Filter by tags (comma-separated) | - |
| `page` | integer | Page number | 1 |
| `limit` | integer | Results per page (max 100) | 20 |
| `sort` | string | Sort field (`created_at`, `updated_at`, `user`, `state`) | `created_at` |
| `order` | string | Sort order (`asc`, `desc`) | `desc` |

**Response** (200 OK):
```json
{
  "sessions": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "name": "john-doe-firefox-browser-abc123",
      "user": "john.doe",
      "template": "firefox-browser",
      "platform": "kubernetes",
      "agent_id": "k8s-prod-us-east-1",
      "state": "running",
      "resources": {
        "memory": "2Gi",
        "cpu": "1000m"
      },
      "tags": ["project-alpha"],
      "status": {
        "phase": "Running",
        "message": "Session is running",
        "pod_ip": "10.42.1.5",
        "vnc_url": "/vnc-viewer/770e8400-e29b-41d4-a716-446655440002"
      },
      "created_at": "2025-11-22T10:45:00Z",
      "updated_at": "2025-11-22T10:45:12Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

**Example**:
```bash
# List all running sessions for user john.doe
curl -H "Authorization: Bearer $TOKEN" \
  "https://streamspace.example.com/api/v1/sessions?user=john.doe&state=running"

# List sessions with specific tags
curl -H "Authorization: Bearer $TOKEN" \
  "https://streamspace.example.com/api/v1/sessions?tags=project-alpha,development"
```

### Get Session Details

Get detailed information about a specific session.

**Endpoint**: `GET /sessions/{session_id}`

**Path Parameters**:
- `session_id`: Session UUID

**Response** (200 OK):
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "name": "john-doe-firefox-browser-abc123",
  "namespace": "streamspace",
  "user": "john.doe",
  "template": "firefox-browser",
  "platform": "kubernetes",
  "agent_id": "k8s-prod-us-east-1",
  "state": "running",
  "resources": {
    "memory": "2Gi",
    "cpu": "1000m"
  },
  "persistent_home": true,
  "idle_timeout": "30m",
  "tags": ["project-alpha"],
  "status": {
    "phase": "Running",
    "message": "Session is running",
    "pod_ip": "10.42.1.5",
    "pod_name": "john-doe-firefox-browser-abc123-7c8f9d6b5",
    "vnc_url": "/vnc-viewer/770e8400-e29b-41d4-a716-446655440002",
    "container_id": "abc123def456",
    "started_at": "2025-11-22T10:45:12Z"
  },
  "platform_metadata": {
    "namespace": "streamspace",
    "deployment_name": "john-doe-firefox-browser-abc123",
    "service_name": "john-doe-firefox-browser-abc123",
    "pvc_name": "john-doe-firefox-browser-abc123-home"
  },
  "created_at": "2025-11-22T10:45:00Z",
  "updated_at": "2025-11-22T10:45:12Z",
  "last_activity": "2025-11-22T11:00:00Z"
}
```

**Response** (404 Not Found):
```json
{
  "error": "Session not found",
  "message": "No session with ID '770e8400-e29b-41d4-a716-446655440002' exists",
  "code": "SESSION_NOT_FOUND"
}
```

**Example**:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/sessions/770e8400-e29b-41d4-a716-446655440002
```

### Update Session State

Update session state (hibernate, wake, terminate).

**Endpoint**: `PATCH /sessions/{session_id}`

**Path Parameters**:
- `session_id`: Session UUID

**Request Body**:
```json
{
  "state": "hibernated"
}
```

**Valid State Transitions**:
| From State | To State | Description |
|------------|----------|-------------|
| `running` | `hibernated` | Hibernate session (save state, scale to zero) |
| `hibernated` | `running` | Wake session (restore state) |
| `running` | `terminating` | Terminate session gracefully |
| `hibernated` | `terminating` | Terminate hibernated session |
| Any | `terminated` | Force terminate (admin only) |

**Response** (200 OK):
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "state": "hibernated",
  "status": {
    "phase": "Hibernated",
    "message": "Session hibernated successfully"
  },
  "updated_at": "2025-11-22T11:05:00Z"
}
```

**Response** (400 Bad Request):
```json
{
  "error": "Invalid state transition",
  "message": "Cannot transition from 'terminated' to 'running'",
  "code": "INVALID_STATE_TRANSITION"
}
```

**Example**:
```bash
# Hibernate session
curl -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"state":"hibernated"}' \
  https://streamspace.example.com/api/v1/sessions/770e8400-e29b-41d4-a716-446655440002

# Wake session
curl -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"state":"running"}' \
  https://streamspace.example.com/api/v1/sessions/770e8400-e29b-41d4-a716-446655440002

# Terminate session
curl -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"state":"terminating"}' \
  https://streamspace.example.com/api/v1/sessions/770e8400-e29b-41d4-a716-446655440002
```

### Delete Session

Delete a session (alias for state transition to `terminated`).

**Endpoint**: `DELETE /sessions/{session_id}`

**Path Parameters**:
- `session_id`: Session UUID

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `force` | boolean | Force delete without graceful shutdown | `false` |

**Response** (204 No Content)

**Response** (404 Not Found):
```json
{
  "error": "Session not found",
  "message": "No session with ID '770e8400-e29b-41d4-a716-446655440002' exists",
  "code": "SESSION_NOT_FOUND"
}
```

**Example**:
```bash
# Delete session gracefully
curl -X DELETE \
  -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/sessions/770e8400-e29b-41d4-a716-446655440002

# Force delete
curl -X DELETE \
  -H "Authorization: Bearer $TOKEN" \
  "https://streamspace.example.com/api/v1/sessions/770e8400-e29b-41d4-a716-446655440002?force=true"
```

### Session Logs

Get container logs for a session.

**Endpoint**: `GET /sessions/{session_id}/logs`

**Path Parameters**:
- `session_id`: Session UUID

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `tail` | integer | Number of lines from end | 100 |
| `follow` | boolean | Stream logs (WebSocket upgrade) | `false` |
| `timestamps` | boolean | Include timestamps | `true` |

**Response** (200 OK):
```json
{
  "logs": [
    "2025-11-22T10:45:15Z [INFO] VNC server started on port 5900",
    "2025-11-22T10:45:16Z [INFO] noVNC web server started on port 6080",
    "2025-11-22T10:45:18Z [INFO] Firefox initialized",
    "2025-11-22T10:46:00Z [INFO] User connected via VNC"
  ]
}
```

**Example**:
```bash
# Get last 50 log lines
curl -H "Authorization: Bearer $TOKEN" \
  "https://streamspace.example.com/api/v1/sessions/770e8400-e29b-41d4-a716-446655440002/logs?tail=50"
```

---

## Template Management

Templates define application configurations (image, resources, VNC settings).

### List Templates

Get all available templates.

**Endpoint**: `GET /templates`

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `category` | string | Filter by category | - |
| `page` | integer | Page number | 1 |
| `limit` | integer | Results per page (max 100) | 50 |

**Response** (200 OK):
```json
{
  "templates": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "name": "firefox-browser",
      "display_name": "Firefox Web Browser",
      "category": "Web Browsers",
      "description": "Mozilla Firefox web browser with privacy extensions",
      "base_image": "lscr.io/linuxserver/firefox:latest",
      "default_resources": {
        "memory": "2Gi",
        "cpu": "1000m"
      },
      "vnc": {
        "enabled": true,
        "port": 3000
      },
      "tags": ["browser", "web", "privacy"],
      "created_at": "2025-11-01T08:00:00Z",
      "updated_at": "2025-11-15T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 1,
    "total_pages": 1
  }
}
```

**Example**:
```bash
# List all browser templates
curl -H "Authorization: Bearer $TOKEN" \
  "https://streamspace.example.com/api/v1/templates?category=Web%20Browsers"
```

### Get Template Details

Get detailed information about a specific template.

**Endpoint**: `GET /templates/{template_name}`

**Path Parameters**:
- `template_name`: Template name (e.g., `firefox-browser`)

**Response** (200 OK):
```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "name": "firefox-browser",
  "display_name": "Firefox Web Browser",
  "category": "Web Browsers",
  "description": "Mozilla Firefox web browser with privacy extensions",
  "base_image": "lscr.io/linuxserver/firefox:latest",
  "default_resources": {
    "memory": "2Gi",
    "cpu": "1000m",
    "storage": "10Gi"
  },
  "vnc": {
    "enabled": true,
    "port": 3000,
    "resolution": "1920x1080",
    "color_depth": 24
  },
  "environment": {
    "PUID": "1000",
    "PGID": "1000",
    "TZ": "UTC"
  },
  "persistent_home": true,
  "idle_timeout": "30m",
  "tags": ["browser", "web", "privacy"],
  "icon_url": "https://cdn.streamspace.io/icons/firefox.svg",
  "created_at": "2025-11-01T08:00:00Z",
  "updated_at": "2025-11-15T10:00:00Z"
}
```

**Example**:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/templates/firefox-browser
```

---

## User Management

User CRUD operations (admin only).

### List Users

**Endpoint**: `GET /users`

**Query Parameters**:
| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `role` | string | Filter by role (`admin`, `user`) | - |
| `page` | integer | Page number | 1 |
| `limit` | integer | Results per page | 20 |

**Response** (200 OK):
```json
{
  "users": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "username": "admin",
      "email": "admin@example.com",
      "role": "admin",
      "active": true,
      "created_at": "2025-11-01T08:00:00Z",
      "last_login": "2025-11-22T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

### Create User

**Endpoint**: `POST /users`

**Request Body**:
```json
{
  "username": "john.doe",
  "email": "john.doe@example.com",
  "password": "secure-password",
  "role": "user"
}
```

**Response** (201 Created):
```json
{
  "id": "990e8400-e29b-41d4-a716-446655440004",
  "username": "john.doe",
  "email": "john.doe@example.com",
  "role": "user",
  "active": true,
  "created_at": "2025-11-22T11:10:00Z"
}
```

---

## VNC Proxy

VNC connections are proxied through the Control Plane via WebSocket.

### VNC WebSocket Connection

**Endpoint**: `GET /vnc-viewer/{session_id}`

**Path Parameters**:
- `session_id`: Session UUID

**Protocol**: WebSocket upgrade

**Query Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `token` | string | JWT authentication token | Yes |

**Connection Flow**:
1. Client initiates WebSocket connection with JWT token
2. Control Plane validates token and session ownership
3. Control Plane establishes tunnel to Agent
4. Agent port-forwards to session pod's VNC port
5. Bidirectional data relay: Client ↔ Control Plane ↔ Agent ↔ Pod

**Example (JavaScript)**:
```javascript
// Connect to VNC via WebSocket
const sessionId = '770e8400-e29b-41d4-a716-446655440002';
const token = 'eyJhbGciOiJIUzI1NiIs...';
const ws = new WebSocket(
  `wss://streamspace.example.com/vnc-viewer/${sessionId}?token=${token}`
);

ws.onopen = () => {
  console.log('VNC connection established');
};

ws.onmessage = (event) => {
  // VNC protocol data
  const data = event.data;
  // Pass to VNC client library (e.g., noVNC)
};

ws.onerror = (error) => {
  console.error('VNC connection error:', error);
};

ws.onclose = () => {
  console.log('VNC connection closed');
};
```

**Example (noVNC)**:
```html
<!DOCTYPE html>
<html>
<head>
  <script src="https://cdn.jsdelivr.net/npm/@novnc/novnc/core/rfb.js"></script>
</head>
<body>
  <div id="screen"></div>
  <script>
    const sessionId = '770e8400-e29b-41d4-a716-446655440002';
    const token = 'eyJhbGciOiJIUzI1NiIs...';
    const url = `wss://streamspace.example.com/vnc-viewer/${sessionId}?token=${token}`;

    const rfb = new RFB(document.getElementById('screen'), url);
    rfb.scaleViewport = true;
    rfb.resizeSession = true;
  </script>
</body>
</html>
```

---

## WebSocket Protocol

### Agent WebSocket Connection

Agents connect to the Control Plane via WebSocket for bidirectional communication.

**Endpoint**: `GET /agent/ws`

**Protocol**: WebSocket upgrade

**Query Parameters**:
| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `agent_id` | string | Agent identifier | Yes |
| `platform` | string | Platform type | Yes |
| `region` | string | Region | No |

**Message Format** (JSON):

#### Agent → Control Plane Messages

**1. Registration**:
```json
{
  "type": "register",
  "agent_id": "k8s-prod-us-east-1",
  "platform": "kubernetes",
  "region": "us-east-1",
  "capacity": {
    "max_cpu": 100,
    "max_memory": 256,
    "max_sessions": 100
  },
  "metadata": {
    "cluster_name": "prod-k8s-cluster",
    "kubernetes_version": "v1.28.0",
    "agent_version": "v2.0-beta.1",
    "ha_enabled": true,
    "is_leader": true
  }
}
```

**2. Heartbeat**:
```json
{
  "type": "heartbeat",
  "agent_id": "k8s-prod-us-east-1",
  "timestamp": "2025-11-22T10:35:00Z",
  "capacity": {
    "current_sessions": 12
  }
}
```

**3. Command Acknowledgement**:
```json
{
  "type": "command_ack",
  "command_id": "cmd-abc123",
  "agent_id": "k8s-prod-us-east-1",
  "status": "acknowledged"
}
```

**4. Command Result**:
```json
{
  "type": "command_result",
  "command_id": "cmd-abc123",
  "agent_id": "k8s-prod-us-east-1",
  "status": "completed",
  "result": {
    "session_id": "770e8400-e29b-41d4-a716-446655440002",
    "pod_ip": "10.42.1.5",
    "pod_name": "john-doe-firefox-browser-abc123-7c8f9d6b5",
    "vnc_port": 5900,
    "started_at": "2025-11-22T10:45:12Z"
  }
}
```

**5. Command Error**:
```json
{
  "type": "command_result",
  "command_id": "cmd-abc123",
  "agent_id": "k8s-prod-us-east-1",
  "status": "failed",
  "error": "Failed to create pod: insufficient resources",
  "error_code": "INSUFFICIENT_RESOURCES"
}
```

#### Control Plane → Agent Messages

**1. Registration Acknowledgement**:
```json
{
  "type": "register_ack",
  "agent_id": "k8s-prod-us-east-1",
  "status": "registered",
  "heartbeat_interval": "10s"
}
```

**2. Start Session Command**:
```json
{
  "type": "command",
  "command_id": "cmd-abc123",
  "command_type": "start_session",
  "session_id": "770e8400-e29b-41d4-a716-446655440002",
  "data": {
    "name": "john-doe-firefox-browser-abc123",
    "namespace": "streamspace",
    "user": "john.doe",
    "template": "firefox-browser",
    "image": "lscr.io/linuxserver/firefox:latest",
    "resources": {
      "memory": "2Gi",
      "cpu": "1000m"
    },
    "vnc": {
      "port": 3000
    },
    "persistent_home": true,
    "environment": {
      "PUID": "1000",
      "PGID": "1000"
    }
  }
}
```

**3. Hibernate Session Command**:
```json
{
  "type": "command",
  "command_id": "cmd-def456",
  "command_type": "hibernate_session",
  "session_id": "770e8400-e29b-41d4-a716-446655440002",
  "data": {
    "name": "john-doe-firefox-browser-abc123",
    "namespace": "streamspace"
  }
}
```

**4. Wake Session Command**:
```json
{
  "type": "command",
  "command_id": "cmd-ghi789",
  "command_type": "wake_session",
  "session_id": "770e8400-e29b-41d4-a716-446655440002",
  "data": {
    "name": "john-doe-firefox-browser-abc123",
    "namespace": "streamspace"
  }
}
```

**5. Terminate Session Command**:
```json
{
  "type": "command",
  "command_id": "cmd-jkl012",
  "command_type": "stop_session",
  "session_id": "770e8400-e29b-41d4-a716-446655440002",
  "data": {
    "name": "john-doe-firefox-browser-abc123",
    "namespace": "streamspace",
    "graceful": true,
    "timeout": "30s"
  }
}
```

**6. VNC Proxy Request**:
```json
{
  "type": "vnc_proxy_request",
  "session_id": "770e8400-e29b-41d4-a716-446655440002",
  "proxy_id": "proxy-abc123",
  "data": {
    "pod_name": "john-doe-firefox-browser-abc123-7c8f9d6b5",
    "namespace": "streamspace",
    "vnc_port": 5900
  }
}
```

### Connection Lifecycle

**1. Agent Connects**:
```
Agent → Control Plane: WebSocket upgrade request
Control Plane → Agent: 101 Switching Protocols
```

**2. Agent Registers**:
```
Agent → Control Plane: {"type": "register", ...}
Control Plane → Agent: {"type": "register_ack", ...}
```

**3. Heartbeat Loop**:
```
Agent → Control Plane: {"type": "heartbeat", ...} (every 10s)
```

**4. Command Execution**:
```
Control Plane → Agent: {"type": "command", "command_type": "start_session", ...}
Agent → Control Plane: {"type": "command_ack", ...}
Agent executes command...
Agent → Control Plane: {"type": "command_result", "status": "completed", ...}
```

**5. VNC Proxy**:
```
User → Control Plane: VNC WebSocket connection
Control Plane → Agent: {"type": "vnc_proxy_request", ...}
Agent establishes port-forward to pod
Control Plane relays VNC data bidirectionally
```

**6. Disconnection**:
```
Agent → Control Plane: WebSocket close
Control Plane marks agent as offline
Control Plane triggers agent failover (if HA enabled)
```

---

## Error Handling

### HTTP Status Codes

| Status Code | Description | Usage |
|-------------|-------------|-------|
| 200 OK | Request succeeded | GET, PATCH successful |
| 201 Created | Resource created | POST successful (user, template) |
| 202 Accepted | Request accepted (async) | POST session (provisioning) |
| 204 No Content | Request succeeded, no body | DELETE successful |
| 400 Bad Request | Invalid request | Missing fields, invalid data |
| 401 Unauthorized | Authentication failed | Invalid/missing JWT token |
| 403 Forbidden | Insufficient permissions | User lacks required role |
| 404 Not Found | Resource not found | Session, agent, template not found |
| 409 Conflict | Resource conflict | Duplicate username, session exists |
| 422 Unprocessable Entity | Validation failed | Invalid state transition |
| 429 Too Many Requests | Rate limit exceeded | Too many requests |
| 500 Internal Server Error | Server error | Unexpected error |
| 503 Service Unavailable | Service unavailable | No agents available |

### Error Response Format

All error responses follow this format:

```json
{
  "error": "Short error message",
  "message": "Detailed error description",
  "code": "ERROR_CODE",
  "details": {
    "field": "additional context"
  },
  "timestamp": "2025-11-22T10:50:00Z",
  "request_id": "req-abc123"
}
```

### Error Codes

#### Authentication Errors (AUTH_*)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `AUTH_INVALID_CREDENTIALS` | 401 | Invalid username or password |
| `AUTH_TOKEN_EXPIRED` | 401 | JWT token expired |
| `AUTH_TOKEN_INVALID` | 401 | JWT token invalid or malformed |
| `AUTH_TOKEN_MISSING` | 401 | Authorization header missing |
| `AUTH_INSUFFICIENT_PERMISSIONS` | 403 | User lacks required permissions |

#### Agent Errors (AGENT_*)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `AGENT_NOT_FOUND` | 404 | Agent does not exist |
| `AGENT_OFFLINE` | 503 | Agent is offline |
| `AGENT_DRAINING` | 503 | Agent is draining (not accepting new sessions) |
| `NO_AGENTS_AVAILABLE` | 503 | No online agents for platform |
| `AGENT_CAPACITY_EXCEEDED` | 503 | Agent at max capacity |

#### Session Errors (SESSION_*)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `SESSION_NOT_FOUND` | 404 | Session does not exist |
| `SESSION_ALREADY_EXISTS` | 409 | Session with same name exists |
| `INVALID_STATE_TRANSITION` | 400 | Invalid state change requested |
| `SESSION_PROVISIONING_FAILED` | 500 | Failed to provision session |
| `SESSION_TERMINATING` | 409 | Session is terminating |

#### Template Errors (TEMPLATE_*)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `TEMPLATE_NOT_FOUND` | 404 | Template does not exist |
| `TEMPLATE_INVALID` | 400 | Template configuration invalid |

#### Validation Errors (VALIDATION_*)

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `VALIDATION_FAILED` | 400 | Request validation failed |
| `INVALID_PARAMETER` | 400 | Invalid query/path parameter |
| `MISSING_REQUIRED_FIELD` | 400 | Required field missing |

---

## Rate Limiting

API requests are rate-limited to prevent abuse.

### Rate Limit Headers

All responses include rate limit information:

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1732272000
```

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Max requests per window |
| `X-RateLimit-Remaining` | Requests remaining in window |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |

### Rate Limits

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/auth/login` | 5 requests | 1 minute |
| `/sessions` (POST) | 10 requests | 1 minute |
| All other endpoints | 100 requests | 1 minute |

### Rate Limit Exceeded Response

**Status**: 429 Too Many Requests

```json
{
  "error": "Rate limit exceeded",
  "message": "Too many requests. Please try again in 45 seconds.",
  "code": "RATE_LIMIT_EXCEEDED",
  "retry_after": 45,
  "timestamp": "2025-11-22T10:55:00Z"
}
```

---

## Examples

### Complete Session Lifecycle

```bash
#!/bin/bash

# 1. Login and get token
TOKEN=$(curl -s -X POST https://streamspace.example.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}' \
  | jq -r .token)

echo "Logged in. Token: ${TOKEN:0:20}..."

# 2. List available templates
echo "\nAvailable templates:"
curl -s -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/templates \
  | jq '.templates[] | {name, display_name, category}'

# 3. Create a session
echo "\nCreating session..."
SESSION_ID=$(curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user": "john.doe",
    "template": "firefox-browser",
    "state": "running",
    "resources": {
      "memory": "2Gi",
      "cpu": "1000m"
    }
  }' \
  https://streamspace.example.com/api/v1/sessions \
  | jq -r .id)

echo "Session created: $SESSION_ID"

# 4. Wait for session to be running
echo "\nWaiting for session to start..."
while true; do
  STATE=$(curl -s -H "Authorization: Bearer $TOKEN" \
    https://streamspace.example.com/api/v1/sessions/$SESSION_ID \
    | jq -r .state)

  if [ "$STATE" = "running" ]; then
    echo "Session is running!"
    break
  fi

  echo "Current state: $STATE"
  sleep 2
done

# 5. Get VNC URL
VNC_URL=$(curl -s -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/sessions/$SESSION_ID \
  | jq -r .status.vnc_url)

echo "VNC URL: https://streamspace.example.com$VNC_URL"

# 6. Hibernate session
echo "\nHibernating session..."
curl -s -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"state":"hibernated"}' \
  https://streamspace.example.com/api/v1/sessions/$SESSION_ID \
  | jq '{state, status}'

# 7. Wake session
echo "\nWaking session..."
curl -s -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"state":"running"}' \
  https://streamspace.example.com/api/v1/sessions/$SESSION_ID \
  | jq '{state, status}'

# 8. Terminate session
echo "\nTerminating session..."
curl -s -X DELETE \
  -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/sessions/$SESSION_ID

echo "Session terminated"

# 9. Logout
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  https://streamspace.example.com/api/v1/auth/logout

echo "\nLogged out"
```

### Monitor Agent Health

```bash
#!/bin/bash

TOKEN="your-jwt-token"

while true; do
  clear
  echo "StreamSpace Agent Health Dashboard"
  echo "=================================="
  echo ""

  # Get agent stats
  curl -s -H "Authorization: Bearer $TOKEN" \
    https://streamspace.example.com/api/v1/agents/stats \
    | jq -r '"Total Agents: \(.total_agents)\nOnline: \(.online_agents)\nOffline: \(.offline_agents)\nDraining: \(.draining_agents)\n\nUtilization: \(.utilization.percentage)%\nSessions: \(.total_capacity.current_sessions) / \(.total_capacity.max_sessions)"'

  echo ""
  echo "Agents:"
  echo "-------"

  # List all agents
  curl -s -H "Authorization: Bearer $TOKEN" \
    https://streamspace.example.com/api/v1/agents \
    | jq -r '.agents[] | "\(.agent_id) (\(.platform)) - \(.status) - Sessions: \(.capacity.current_sessions)/\(.capacity.max_sessions)"'

  sleep 5
done
```

---

**API Version**: v1
**Last Updated**: 2025-11-22
**StreamSpace Version**: v2.0.0-beta.1

For more information, see:
- [Deployment Guide](V2_DEPLOYMENT_GUIDE.md)
- [Migration Guide](MIGRATION_V1_TO_V2.md)
- [Architecture Documentation](ARCHITECTURE.md)
