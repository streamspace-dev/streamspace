# Multi-Controller Architecture Design

**Version**: 1.0
**Date**: 2025-11-17
**Status**: Proposed Design

---

## Executive Summary

This document outlines the architectural changes needed to evolve StreamSpace from a Kubernetes-only platform to a **multi-backend orchestration platform** that can manage sessions across different infrastructure types:

- **Kubernetes** (existing)
- **Docker** (standalone containers)
- **VMware ESXi** (virtual machines)
- **KVM/QEMU** (Linux virtual machines)
- **Proxmox VE** (containers and VMs)
- **Hyper-V / App-V** (Windows containers and VMs)
- **Future**: Cloud VMs (EC2, Azure VMs, GCE)

This design maintains **backward compatibility** with the existing Kubernetes controller while enabling extensibility for new controller types.

---

## Table of Contents

1. [Current Architecture](#current-architecture)
2. [Design Goals](#design-goals)
3. [Architecture Overview](#architecture-overview)
4. [Controller Interface Contract](#controller-interface-contract)
5. [Controller Registration System](#controller-registration-system)
6. [API Server Changes](#api-server-changes)
7. [Database Schema Changes](#database-schema-changes)
8. [Session Model Evolution](#session-model-evolution)
9. [Kubernetes Controller Refactoring](#kubernetes-controller-refactoring)
10. [Implementation Phases](#implementation-phases)
11. [Backward Compatibility](#backward-compatibility)
12. [Example: Docker Controller](#example-docker-controller)

---

## Current Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Web UI (React)                          │
└────────────────────────┬────────────────────────────────────┘
                         │ REST/WebSocket
                         ↓
┌─────────────────────────────────────────────────────────────┐
│                   API Backend (Go)                           │
│  - REST handlers                                             │
│  - Kubernetes client (k8s.Client)                           │
│  - Creates Session CRDs                                      │
└────────────────────────┬────────────────────────────────────┘
                         │ Kubernetes API
                         ↓
┌─────────────────────────────────────────────────────────────┐
│            Kubernetes Controller (Kubebuilder)               │
│  - Watches Session CRDs                                      │
│  - Creates Deployments, Services, Ingress, PVCs             │
│  - Manages hibernation (scale to 0)                          │
└────────────────────────┬────────────────────────────────────┘
                         │ Creates
                         ↓
┌─────────────────────────────────────────────────────────────┐
│                  Kubernetes Resources                        │
│  - Pods (containerized sessions)                            │
│  - Services (networking)                                     │
│  - Ingress (external access)                                │
│  - PVCs (persistent storage)                                │
└─────────────────────────────────────────────────────────────┘
```

### Limitations

- **Kubernetes-only**: Can only run sessions on Kubernetes clusters
- **Tight coupling**: API server directly uses Kubernetes client
- **CRD-based**: Session state stored in Kubernetes CRDs (not in PostgreSQL)
- **No abstraction**: Session creation assumes Kubernetes primitives

---

## Design Goals

### Primary Goals

1. **Multi-Backend Support**: Enable sessions on Docker, ESXi, KVM, Proxmox, Hyper-V
2. **Controller Abstraction**: Define a common interface for all controller types
3. **Backward Compatibility**: Existing Kubernetes deployments continue to work
4. **Centralized State**: API server becomes the source of truth (not Kubernetes)
5. **Controller Independence**: Controllers can be deployed separately, scaled independently

### Non-Goals (Out of Scope)

- Cross-controller session migration (Phase 1)
- Multi-controller load balancing (Phase 1)
- Automatic controller failover (Phase 1)

---

## Architecture Overview

### New Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                      Web UI (React)                             │
└───────────────────────────┬────────────────────────────────────┘
                            │ REST/WebSocket
                            ↓
┌────────────────────────────────────────────────────────────────┐
│                   API Backend (Go)                              │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │           Controller Router                              │  │
│  │  - Routes requests to appropriate controller             │  │
│  │  - Controller selection logic                            │  │
│  │  - Health checks and discovery                           │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │           PostgreSQL Database                            │  │
│  │  - Sessions table (source of truth)                      │  │
│  │  - Controllers table (registration)                      │  │
│  │  - Controller capabilities                               │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────┬────────────┬────────────┬────────────┬───────────────┘
         │            │            │            │
         │ gRPC       │ gRPC       │ gRPC       │ gRPC
         ↓            ↓            ↓            ↓
┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐
│ Kubernetes │ │   Docker   │ │    ESXi    │ │    KVM     │
│ Controller │ │ Controller │ │ Controller │ │ Controller │
└──────┬─────┘ └──────┬─────┘ └──────┬─────┘ └──────┬─────┘
       │              │              │              │
       ↓              ↓              ↓              ↓
┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐
│ K8s Pods   │ │ Containers │ │    VMs     │ │    VMs     │
└────────────┘ └────────────┘ └────────────┘ └────────────┘
```

### Key Changes

1. **API Server** becomes the orchestrator
2. **Controllers** are independent services (not just Kubernetes controllers)
3. **gRPC** protocol for API ↔ Controller communication
4. **PostgreSQL** stores session state (single source of truth)
5. **Controller registration** allows dynamic discovery

---

## Controller Interface Contract

### gRPC Service Definition

All controllers must implement this gRPC service:

```protobuf
// File: api/proto/controller.proto
syntax = "proto3";
package streamspace.controller.v1;

service ControllerService {
  // Lifecycle operations
  rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse);
  rpc GetSession(GetSessionRequest) returns (GetSessionResponse);
  rpc DeleteSession(DeleteSessionRequest) returns (DeleteSessionResponse);

  // State transitions
  rpc HibernateSession(HibernateSessionRequest) returns (HibernateSessionResponse);
  rpc WakeSession(WakeSessionRequest) returns (WakeSessionResponse);

  // Monitoring
  rpc GetSessionStatus(GetSessionStatusRequest) returns (SessionStatusResponse);
  rpc GetSessionMetrics(GetSessionMetricsRequest) returns (SessionMetricsResponse);
  rpc GetSessionLogs(GetSessionLogsRequest) returns (stream LogEntry);

  // Controller metadata
  rpc GetCapabilities(GetCapabilitiesRequest) returns (CapabilitiesResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message CreateSessionRequest {
  string session_id = 1;
  string user_id = 2;
  string template_id = 3;

  // Generic resource spec (controller interprets)
  SessionSpec spec = 4;

  // Controller-specific config (JSON)
  string controller_config = 5;
}

message SessionSpec {
  // Generic fields (all controllers support)
  string base_image = 1;
  ResourceRequirements resources = 2;
  repeated Port ports = 3;
  repeated EnvVar env = 4;
  repeated VolumeMount volume_mounts = 5;

  // VNC/webapp config
  VNCConfig vnc = 6;
  WebAppConfig webapp = 7;

  // Storage
  bool persistent_home = 8;
  string home_size = 9;
}

message ResourceRequirements {
  string memory = 1;  // e.g., "2Gi"
  string cpu = 2;     // e.g., "1000m" or "1"
  string storage = 3; // e.g., "50Gi"
}

message SessionStatusResponse {
  string session_id = 1;
  string phase = 2; // Pending, Running, Hibernated, Failed, Terminated
  string url = 3;   // Access URL
  string node_id = 4; // Controller-specific identifier (pod name, container ID, VM ID)
  google.protobuf.Timestamp last_activity = 5;
  ResourceUsage resource_usage = 6;
  repeated Condition conditions = 7;
}

message CapabilitiesResponse {
  string controller_type = 1; // "kubernetes", "docker", "esxi", etc.
  string version = 2;

  // Features
  bool supports_hibernation = 3;
  bool supports_gpu = 4;
  bool supports_persistent_storage = 5;
  bool supports_networking = 6;

  // Capacity
  ResourceCapacity capacity = 7;

  // Metadata
  map<string, string> labels = 8;
}
```

### Controller Responsibilities

Each controller must:

1. **Implement gRPC service**: All methods in `ControllerService`
2. **Register on startup**: Call API server registration endpoint
3. **Heartbeat**: Send periodic health checks (every 30s)
4. **State sync**: Report session status back to API server
5. **Resource management**: Allocate/deallocate resources on backend
6. **Lifecycle management**: Handle creation, hibernation, wake, deletion

---

## Controller Registration System

### Registration Flow

```
┌─────────────────┐                    ┌─────────────────┐
│   Controller    │                    │   API Server    │
│  (any type)     │                    │                 │
└────────┬────────┘                    └────────┬────────┘
         │                                      │
         │  1. Start controller                 │
         │     (read config)                    │
         │                                      │
         │  2. POST /api/v1/controllers/register│
         │  {                                   │
         │    "type": "docker",                 │
         │    "endpoint": "grpc://...:50051",   │
         │    "capabilities": {...}             │
         │  }                                   │
         │─────────────────────────────────────>│
         │                                      │
         │  3. Registration OK                  │
         │     (controller_id, token)           │
         │<─────────────────────────────────────│
         │                                      │
         │  4. Periodic heartbeat (30s)         │
         │     POST /api/v1/controllers/:id/heartbeat
         │─────────────────────────────────────>│
         │                                      │
         │  5. gRPC HealthCheck()               │
         │<─────────────────────────────────────│
         │                                      │
         │  6. Health OK                        │
         │─────────────────────────────────────>│
         │                                      │
```

### API Endpoints

**Register Controller**:
```
POST /api/v1/controllers/register
{
  "type": "docker",
  "name": "docker-controller-1",
  "endpoint": "grpc://docker-controller.local:50051",
  "capabilities": {
    "supports_hibernation": true,
    "supports_gpu": false,
    "capacity": {
      "max_sessions": 50,
      "memory": "128Gi",
      "cpu": "32"
    }
  },
  "labels": {
    "region": "us-west-2",
    "environment": "production"
  }
}

Response:
{
  "controller_id": "ctrl-abc123",
  "auth_token": "jwt-token-for-controller",
  "api_version": "v1alpha1"
}
```

**Heartbeat**:
```
POST /api/v1/controllers/:id/heartbeat
Authorization: Bearer <controller-token>
{
  "status": "healthy",
  "current_sessions": 15,
  "resource_usage": {
    "memory": "32Gi",
    "cpu": "8"
  }
}
```

**List Controllers** (Admin):
```
GET /api/v1/admin/controllers
Response:
[
  {
    "id": "ctrl-abc123",
    "type": "kubernetes",
    "name": "k8s-controller-1",
    "status": "healthy",
    "sessions": 25,
    "last_heartbeat": "2025-11-17T10:00:00Z"
  },
  {
    "id": "ctrl-def456",
    "type": "docker",
    "name": "docker-controller-1",
    "status": "healthy",
    "sessions": 10,
    "last_heartbeat": "2025-11-17T10:00:05Z"
  }
]
```

---

## API Server Changes

### 1. Controller Manager

New component in API server:

```go
// File: api/internal/controller/manager.go
package controller

import (
    "context"
    "sync"
    "time"

    pb "github.com/streamspace/streamspace/api/proto"
    "google.golang.org/grpc"
)

// ControllerManager manages connections to all registered controllers
type ControllerManager struct {
    mu          sync.RWMutex
    controllers map[string]*ControllerClient // controller_id -> client
    db          *db.Database
}

// ControllerClient wraps a gRPC connection to a controller
type ControllerClient struct {
    ID           string
    Type         string // "kubernetes", "docker", etc.
    Name         string
    Endpoint     string
    Conn         *grpc.ClientConn
    Client       pb.ControllerServiceClient
    Capabilities *pb.CapabilitiesResponse
    Status       string // "healthy", "unhealthy", "offline"
    LastHeartbeat time.Time
    SessionCount int
}

// RegisterController adds a new controller
func (m *ControllerManager) RegisterController(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
    // 1. Validate request
    // 2. Connect to controller gRPC endpoint
    // 3. Call GetCapabilities()
    // 4. Store in database
    // 5. Add to in-memory map
    // 6. Generate auth token
    // 7. Start health check goroutine
}

// GetController returns a controller client by ID
func (m *ControllerManager) GetController(controllerID string) (*ControllerClient, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    client, ok := m.controllers[controllerID]
    if !ok {
        return nil, ErrControllerNotFound
    }
    return client, nil
}

// SelectController chooses a controller for a new session
func (m *ControllerManager) SelectController(ctx context.Context, req *SessionCreateRequest) (*ControllerClient, error) {
    // Selection strategies:
    // 1. Explicit: User/admin specifies controller_id or controller_type
    // 2. Template-based: Template has preferred controller type
    // 3. Load balancing: Round-robin or least-loaded
    // 4. Capability-based: GPU requirements, storage type, etc.
    // 5. Label-based: Region, environment, etc.
}
```

### 2. Session Handler Refactoring

```go
// File: api/internal/handlers/sessions.go

// CreateSession now routes to appropriate controller
func (h *SessionHandler) CreateSession(c *gin.Context) {
    var req CreateSessionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 1. Select controller
    controller, err := h.controllerManager.SelectController(c.Request.Context(), &req)
    if err != nil {
        c.JSON(500, gin.H{"error": "No available controller"})
        return
    }

    // 2. Create session in database (pending state)
    session := &db.Session{
        ID:           generateSessionID(),
        UserID:       req.UserID,
        TemplateID:   req.TemplateID,
        ControllerID: controller.ID, // NEW FIELD
        State:        "pending",
        CreatedAt:    time.Now(),
    }
    if err := h.db.CreateSession(session); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // 3. Call controller via gRPC
    grpcReq := &pb.CreateSessionRequest{
        SessionId:  session.ID,
        UserId:     req.UserID,
        TemplateId: req.TemplateID,
        Spec:       convertToProtoSpec(req.Spec),
    }

    grpcResp, err := controller.Client.CreateSession(c.Request.Context(), grpcReq)
    if err != nil {
        // Update session state to failed
        h.db.UpdateSessionState(session.ID, "failed")
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // 4. Update session with controller response
    session.NodeID = grpcResp.NodeId // Pod name, container ID, VM ID, etc.
    session.State = grpcResp.Phase
    session.URL = grpcResp.Url
    h.db.UpdateSession(session)

    c.JSON(201, session)
}
```

### 3. Status Sync Worker

```go
// File: api/internal/controller/status_sync.go

// StatusSyncWorker periodically syncs session status from controllers
func (m *ControllerManager) StartStatusSyncWorker(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.syncAllSessionStatuses(ctx)
        }
    }
}

func (m *ControllerManager) syncAllSessionStatuses(ctx context.Context) {
    // 1. Get all active sessions from database
    sessions := m.db.GetActiveSessions()

    // 2. Group by controller
    sessionsByController := make(map[string][]string)
    for _, session := range sessions {
        sessionsByController[session.ControllerID] = append(
            sessionsByController[session.ControllerID],
            session.ID,
        )
    }

    // 3. Call each controller for status updates
    for controllerID, sessionIDs := range sessionsByController {
        controller, err := m.GetController(controllerID)
        if err != nil {
            continue
        }

        for _, sessionID := range sessionIDs {
            status, err := controller.Client.GetSessionStatus(ctx, &pb.GetSessionStatusRequest{
                SessionId: sessionID,
            })
            if err != nil {
                // Mark session as unhealthy
                continue
            }

            // Update database
            m.db.UpdateSessionStatus(sessionID, status)
        }
    }
}
```

---

## Database Schema Changes

### New Tables

```sql
-- Controllers table
CREATE TABLE controllers (
  id VARCHAR(255) PRIMARY KEY,
  type VARCHAR(50) NOT NULL, -- "kubernetes", "docker", "esxi", etc.
  name VARCHAR(255) NOT NULL,
  endpoint TEXT NOT NULL, -- gRPC endpoint (grpc://host:port)
  status VARCHAR(50) NOT NULL DEFAULT 'offline', -- "healthy", "unhealthy", "offline"
  capabilities JSONB, -- Full capabilities JSON
  labels JSONB, -- Custom labels for selection
  current_sessions INTEGER DEFAULT 0,
  last_heartbeat TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_controllers_type ON controllers(type);
CREATE INDEX idx_controllers_status ON controllers(status);

-- Controller authentication tokens
CREATE TABLE controller_tokens (
  controller_id VARCHAR(255) REFERENCES controllers(id) ON DELETE CASCADE,
  token_hash VARCHAR(255) NOT NULL,
  expires_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW(),
  PRIMARY KEY (controller_id)
);
```

### Modified Tables

```sql
-- Add controller_id and node_id to sessions table
ALTER TABLE sessions ADD COLUMN controller_id VARCHAR(255) REFERENCES controllers(id);
ALTER TABLE sessions ADD COLUMN controller_type VARCHAR(50); -- Denormalized for performance
ALTER TABLE sessions ADD COLUMN node_id VARCHAR(255); -- Pod name, container ID, VM ID, etc.

CREATE INDEX idx_sessions_controller ON sessions(controller_id);
CREATE INDEX idx_sessions_controller_type ON sessions(controller_type);
```

---

## Session Model Evolution

### Generic Session Spec

Sessions become infrastructure-agnostic:

```go
// File: api/internal/models/session.go

type Session struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    TemplateID  string    `json:"template_id"`

    // Controller information
    ControllerID   string `json:"controller_id"`
    ControllerType string `json:"controller_type"` // "kubernetes", "docker", etc.
    NodeID         string `json:"node_id"` // Backend-specific ID

    // Generic fields
    State       string    `json:"state"` // "pending", "running", "hibernated", "terminated"
    Phase       string    `json:"phase"` // "Pending", "Running", "Hibernated", "Failed", "Terminated"
    URL         string    `json:"url"`

    // Generic spec (all controllers support)
    Spec SessionSpec `json:"spec"`

    // Controller-specific config (optional)
    ControllerConfig map[string]interface{} `json:"controller_config,omitempty"`

    // Status
    Status SessionStatus `json:"status"`

    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type SessionSpec struct {
    BaseImage        string                 `json:"base_image"`
    Resources        ResourceRequirements   `json:"resources"`
    Ports            []Port                 `json:"ports"`
    Env              []EnvVar               `json:"env"`
    VolumeMounts     []VolumeMount          `json:"volume_mounts"`
    VNC              *VNCConfig             `json:"vnc,omitempty"`
    WebApp           *WebAppConfig          `json:"webapp,omitempty"`
    PersistentHome   bool                   `json:"persistent_home"`
    HomeSize         string                 `json:"home_size"`
    IdleTimeout      string                 `json:"idle_timeout"`
}
```

---

## Kubernetes Controller Refactoring

### Changes Required

1. **Rename**: `controller/` → `controller-kubernetes/` (or keep as is)
2. **Add gRPC server**: Implement `ControllerService` interface
3. **Remove CRD dependency** (optional): Use database as source of truth
4. **Registration**: Call API server on startup to register
5. **Heartbeat**: Send periodic health checks

### Option A: Keep CRDs (Recommended for Phase 1)

- Maintain existing CRD-based architecture
- Add gRPC server layer on top
- API server creates Sessions via gRPC, controller creates CRDs
- Minimal changes to existing reconciliation logic

```
API Server ──gRPC──> K8s Controller ──CRD──> Kubernetes API ──> Pods
```

### Option B: Remove CRDs (Future)

- Controller talks directly to Kubernetes API (no CRDs)
- API server database is source of truth
- More flexibility, but larger refactoring

```
API Server ──gRPC──> K8s Controller ──Deployments/Services──> Kubernetes API ──> Pods
```

**Recommendation**: Start with Option A, migrate to Option B later.

---

## Implementation Phases

### Phase 1: Foundation (Current Priority)

**Goal**: Prepare architecture for multi-controller support without breaking existing functionality.

**Tasks**:

1. ✅ **Design**: Create this document
2. **Database**:
   - Add `controllers` and `controller_tokens` tables
   - Add `controller_id`, `controller_type`, `node_id` to `sessions` table
   - Migration scripts
3. **API Server**:
   - Implement `ControllerManager`
   - Add controller registration endpoints
   - Add heartbeat handling
4. **Proto Definitions**:
   - Define gRPC service in `api/proto/controller.proto`
   - Generate Go and Python clients
5. **Kubernetes Controller**:
   - Add gRPC server
   - Implement `ControllerService` interface
   - Add registration logic on startup
   - Keep existing CRD reconciliation (no changes)
6. **Backward Compatibility**:
   - Default to Kubernetes controller if `controller_id` not specified
   - Auto-register Kubernetes controller on API server startup
7. **Testing**:
   - Integration tests for controller registration
   - Session creation via gRPC
   - Status sync worker

**Deliverables**:
- Working multi-controller infrastructure
- Kubernetes controller as first gRPC-based controller
- No breaking changes for existing users

### Phase 2: Docker Controller (Next)

**Goal**: Implement first alternative controller type.

**Tasks**:

1. **Docker Controller**:
   - New Go service: `controller-docker/`
   - Implements `ControllerService` gRPC
   - Uses Docker API to manage containers
   - Hibernation = `docker stop`, wake = `docker start`
   - Persistent storage via Docker volumes
2. **Templates**:
   - Mark templates as compatible with multiple controller types
   - Add `compatible_controllers: ["kubernetes", "docker"]` field
3. **UI**:
   - Show controller type in session list
   - Allow controller selection when creating session (optional)
4. **Documentation**:
   - Docker controller setup guide
   - Template compatibility guide

### Phase 3: ESXi/KVM Controllers (Future)

**Goal**: Add VM-based controller types.

**Tasks**:
- ESXi controller using vSphere API
- KVM controller using libvirt
- Proxmox controller using Proxmox VE API
- UI enhancements for controller selection
- Load balancing across controllers

### Phase 4: Advanced Features (Future)

**Goal**: Cross-controller capabilities.

**Tasks**:
- Session migration between controllers (same type)
- Multi-controller load balancing
- Controller auto-scaling
- Automatic failover

---

## Backward Compatibility

### Compatibility Guarantees

1. **Existing Deployments**: Continue to work without changes
2. **Kubernetes Controller**: Remains default if no controller specified
3. **CRDs**: Still supported (Option A approach)
4. **API Endpoints**: Existing endpoints unchanged
5. **Database**: Migrations are backward-compatible

### Migration Path for Existing Users

```bash
# 1. Upgrade API server (includes new tables)
kubectl apply -f manifests/config/api-deployment.yaml

# 2. Run database migration
kubectl exec -it api-pod -- /app/migrate

# 3. Upgrade Kubernetes controller (adds gRPC server)
kubectl apply -f manifests/config/controller-deployment.yaml

# 4. Verify auto-registration
curl http://api-server/api/v1/admin/controllers
# Should show Kubernetes controller registered

# 5. Existing sessions continue to work
kubectl get sessions
# All existing sessions now have controller_id populated
```

### Deprecation Policy

- **CRDs**: Not deprecated in Phase 1-2, may be optional in Phase 3
- **Direct Kubernetes API access**: No changes, still supported
- **API fields**: `controller_id` is optional (defaults to Kubernetes)

---

## Example: Docker Controller

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                 Docker Controller (Go)                        │
│  ┌────────────────────────────────────────────────────────┐  │
│  │           gRPC Server (:50052)                         │  │
│  │  - ControllerService implementation                    │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │           Docker API Client                            │  │
│  │  - Container lifecycle (create, start, stop, remove)   │  │
│  │  - Volume management                                   │  │
│  │  - Network configuration                               │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │           Session Manager                              │  │
│  │  - Tracks active containers                            │  │
│  │  - Status reporting                                    │  │
│  │  - Resource monitoring                                 │  │
│  └────────────────────────────────────────────────────────┘  │
└───────────────────────────┬──────────────────────────────────┘
                            │ Docker API
                            ↓
┌────────────────────────────────────────────────────────────────┐
│                    Docker Engine                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  Container   │  │  Container   │  │  Container   │         │
│  │ (Firefox)    │  │ (VS Code)    │  │ (Blender)    │         │
│  │              │  │              │  │              │         │
│  │ Volume:      │  │ Volume:      │  │ Volume:      │         │
│  │ user1-home   │  │ user2-home   │  │ user1-home   │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└────────────────────────────────────────────────────────────────┘
```

### Implementation Sketch

```go
// File: controller-docker/cmd/main.go
package main

import (
    "context"
    "fmt"
    "net"

    "github.com/docker/docker/client"
    pb "github.com/streamspace/streamspace/api/proto"
    "google.golang.org/grpc"
)

type DockerController struct {
    pb.UnimplementedControllerServiceServer
    dockerClient *client.Client
    apiServerURL string
}

func (c *DockerController) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
    // 1. Parse session spec
    spec := req.Spec

    // 2. Create Docker container
    containerName := fmt.Sprintf("ss-%s", req.SessionId)

    containerConfig := &container.Config{
        Image: spec.BaseImage,
        Env:   convertEnvVars(spec.Env),
        Labels: map[string]string{
            "streamspace.session_id": req.SessionId,
            "streamspace.user_id":    req.UserId,
        },
    }

    hostConfig := &container.HostConfig{
        Resources: container.Resources{
            Memory:   parseMemory(spec.Resources.Memory),
            CPUCount: parseCPU(spec.Resources.Cpu),
        },
        Binds: []string{
            fmt.Sprintf("user-%s:/config", req.UserId),
        },
    }

    resp, err := c.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
    if err != nil {
        return nil, err
    }

    // 3. Start container
    if err := c.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        return nil, err
    }

    // 4. Get container IP for URL
    inspect, _ := c.dockerClient.ContainerInspect(ctx, resp.ID)
    url := fmt.Sprintf("http://%s:%d", inspect.NetworkSettings.IPAddress, spec.Vnc.Port)

    return &pb.CreateSessionResponse{
        SessionId: req.SessionId,
        NodeId:    resp.ID, // Docker container ID
        Phase:     "Running",
        Url:       url,
    }, nil
}

func (c *DockerController) HibernateSession(ctx context.Context, req *pb.HibernateSessionRequest) (*pb.HibernateSessionResponse, error) {
    // Stop container (don't remove)
    timeout := 30
    if err := c.dockerClient.ContainerStop(ctx, req.NodeId, &timeout); err != nil {
        return nil, err
    }

    return &pb.HibernateSessionResponse{
        SessionId: req.SessionId,
        Phase:     "Hibernated",
    }, nil
}

func (c *DockerController) WakeSession(ctx context.Context, req *pb.WakeSessionRequest) (*pb.WakeSessionResponse, error) {
    // Start stopped container
    if err := c.dockerClient.ContainerStart(ctx, req.NodeId, types.ContainerStartOptions{}); err != nil {
        return nil, err
    }

    return &pb.WakeSessionResponse{
        SessionId: req.SessionId,
        Phase:     "Running",
    }, nil
}

func main() {
    // 1. Create Docker client
    dockerClient, _ := client.NewClientWithOpts(client.FromEnv)

    // 2. Create controller
    controller := &DockerController{
        dockerClient: dockerClient,
        apiServerURL: os.Getenv("API_SERVER_URL"),
    }

    // 3. Start gRPC server
    lis, _ := net.Listen("tcp", ":50052")
    grpcServer := grpc.NewServer()
    pb.RegisterControllerServiceServer(grpcServer, controller)

    // 4. Register with API server
    registerWithAPIServer(controller)

    // 5. Start heartbeat
    go startHeartbeat(controller)

    // 6. Serve
    grpcServer.Serve(lis)
}
```

---

## Conclusion

This multi-controller architecture provides:

1. **Flexibility**: Support for Kubernetes, Docker, ESXi, KVM, Proxmox, Hyper-V
2. **Backward Compatibility**: Existing Kubernetes deployments continue to work
3. **Scalability**: Controllers can be deployed and scaled independently
4. **Extensibility**: Easy to add new controller types via gRPC interface
5. **Centralized Management**: API server as single source of truth

### Next Steps

1. **Review & Approve**: Team review of this design
2. **Implement Phase 1**: Database changes, ControllerManager, gRPC definitions
3. **Refactor K8s Controller**: Add gRPC server, keep CRD reconciliation
4. **Test**: End-to-end testing with Kubernetes controller via gRPC
5. **Docker Controller**: Implement as proof of concept for multi-backend
6. **Documentation**: Update deployment guides, API docs

---

**Authors**: StreamSpace Team
**Last Updated**: 2025-11-17
