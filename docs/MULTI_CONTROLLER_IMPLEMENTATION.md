# Multi-Controller Implementation Guide

**Date**: 2025-11-17
**Status**: Implementation Roadmap
**Related**: See `MULTI_CONTROLLER_ARCHITECTURE.md` for full design

---

## Quick Summary

This document provides a practical, step-by-step implementation guide for adding multi-controller support to StreamSpace. Changes are designed to be **backward compatible** and **incremental**.

---

## Phase 1: Core Infrastructure (Weeks 1-3)

### Week 1: Database & Proto Definitions

#### 1.1 Database Schema Changes

**Location**: `api/internal/db/database.go`

**Add new tables**:

```sql
-- controllers table
CREATE TABLE IF NOT EXISTS controllers (
  id VARCHAR(255) PRIMARY KEY,
  type VARCHAR(50) NOT NULL, -- "kubernetes", "docker", "esxi", "kvm", etc.
  name VARCHAR(255) NOT NULL,
  endpoint TEXT NOT NULL, -- gRPC endpoint: "grpc://host:port"
  status VARCHAR(50) NOT NULL DEFAULT 'offline', -- "healthy", "unhealthy", "offline"
  capabilities JSONB, -- Full capabilities: {supports_hibernation, supports_gpu, ...}
  labels JSONB, -- Custom labels: {region: "us-west", env: "prod", ...}
  current_sessions INTEGER DEFAULT 0,
  capacity JSONB, -- {max_sessions, memory, cpu, storage}
  last_heartbeat TIMESTAMP,
  registered_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_controllers_type ON controllers(type);
CREATE INDEX idx_controllers_status ON controllers(status);

-- controller_tokens table
CREATE TABLE IF NOT EXISTS controller_tokens (
  controller_id VARCHAR(255) PRIMARY KEY REFERENCES controllers(id) ON DELETE CASCADE,
  token_hash VARCHAR(255) NOT NULL,
  expires_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Modify sessions table
ALTER TABLE sessions ADD COLUMN controller_id VARCHAR(255) REFERENCES controllers(id);
ALTER TABLE sessions ADD COLUMN controller_type VARCHAR(50); -- Denormalized for queries
ALTER TABLE sessions ADD COLUMN node_id VARCHAR(255); -- Backend-specific ID (pod name, container ID, etc.)

CREATE INDEX idx_sessions_controller_id ON sessions(controller_id);
CREATE INDEX idx_sessions_controller_type ON sessions(controller_type);
CREATE INDEX idx_sessions_node_id ON sessions(node_id);
```

**Migration Script**: Create `api/migrations/007_multi_controller.sql`

**Backward compatibility**: Existing sessions will have `controller_id = NULL`, which will be handled as "default Kubernetes controller" in the code.

#### 1.2 Protocol Buffers Definitions

**Location**: Create new directory `api/proto/`

**File**: `api/proto/controller.proto`

```protobuf
syntax = "proto3";
package streamspace.controller.v1;
option go_package = "github.com/streamspace/streamspace/api/proto/v1";

import "google/protobuf/timestamp.proto";

// ControllerService defines the interface all controllers must implement
service ControllerService {
  // Lifecycle operations
  rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse);
  rpc DeleteSession(DeleteSessionRequest) returns (DeleteSessionResponse);

  // State transitions
  rpc HibernateSession(HibernateSessionRequest) returns (HibernateSessionResponse);
  rpc WakeSession(WakeSessionRequest) returns (WakeSessionResponse);

  // Monitoring
  rpc GetSessionStatus(GetSessionStatusRequest) returns (SessionStatusResponse);
  rpc GetSessionMetrics(GetSessionMetricsRequest) returns (SessionMetricsResponse);
  rpc StreamLogs(StreamLogsRequest) returns (stream LogEntry);

  // Controller metadata
  rpc GetCapabilities(GetCapabilitiesRequest) returns (CapabilitiesResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

// CreateSession messages
message CreateSessionRequest {
  string session_id = 1;
  string user_id = 2;
  string template_id = 3;
  SessionSpec spec = 4;
  map<string, string> controller_config = 5; // Controller-specific config
}

message SessionSpec {
  string base_image = 1;
  ResourceRequirements resources = 2;
  repeated Port ports = 3;
  repeated EnvVar env = 4;
  repeated VolumeMount volume_mounts = 5;
  VNCConfig vnc = 6;
  bool persistent_home = 7;
  string home_size = 8;
  string idle_timeout = 9;
}

message ResourceRequirements {
  string memory = 1;  // e.g., "2Gi"
  string cpu = 2;     // e.g., "1000m"
  string storage = 3; // e.g., "50Gi"
}

message Port {
  string name = 1;
  int32 container_port = 2;
  string protocol = 3;
}

message EnvVar {
  string name = 1;
  string value = 2;
}

message VolumeMount {
  string name = 1;
  string mount_path = 2;
}

message VNCConfig {
  bool enabled = 1;
  int32 port = 2;
  string protocol = 3;
}

message CreateSessionResponse {
  string session_id = 1;
  string node_id = 2; // Backend-specific ID
  string phase = 3;   // "Pending", "Running", etc.
  string url = 4;     // Access URL
  string message = 5; // Optional status message
}

// Hibernate/Wake messages
message HibernateSessionRequest {
  string session_id = 1;
  string node_id = 2;
}

message HibernateSessionResponse {
  string session_id = 1;
  string phase = 2;
  string message = 3;
}

message WakeSessionRequest {
  string session_id = 1;
  string node_id = 2;
}

message WakeSessionResponse {
  string session_id = 1;
  string phase = 2;
  string url = 3;
  string message = 4;
}

// Delete session
message DeleteSessionRequest {
  string session_id = 1;
  string node_id = 2;
}

message DeleteSessionResponse {
  string session_id = 1;
  bool success = 2;
  string message = 3;
}

// Status messages
message GetSessionStatusRequest {
  string session_id = 1;
  string node_id = 2;
}

message SessionStatusResponse {
  string session_id = 1;
  string phase = 2;
  string url = 3;
  string node_id = 4;
  google.protobuf.Timestamp last_activity = 5;
  ResourceUsage resource_usage = 6;
  repeated Condition conditions = 7;
}

message ResourceUsage {
  string memory = 1;
  string cpu = 2;
  float memory_percent = 3;
  float cpu_percent = 4;
}

message Condition {
  string type = 1;
  string status = 2;
  string reason = 3;
  string message = 4;
  google.protobuf.Timestamp last_transition_time = 5;
}

// Metrics
message GetSessionMetricsRequest {
  string session_id = 1;
  string node_id = 2;
}

message SessionMetricsResponse {
  string session_id = 1;
  ResourceUsage current_usage = 2;
  map<string, string> custom_metrics = 3;
}

// Logs
message StreamLogsRequest {
  string session_id = 1;
  string node_id = 2;
  bool follow = 3;
  int32 tail_lines = 4;
}

message LogEntry {
  google.protobuf.Timestamp timestamp = 1;
  string message = 2;
  string stream = 3; // "stdout" or "stderr"
}

// Capabilities
message GetCapabilitiesRequest {}

message CapabilitiesResponse {
  string controller_type = 1;
  string version = 2;
  bool supports_hibernation = 3;
  bool supports_gpu = 4;
  bool supports_persistent_storage = 5;
  bool supports_networking = 6;
  ResourceCapacity capacity = 7;
  map<string, string> labels = 8;
}

message ResourceCapacity {
  int32 max_sessions = 1;
  string total_memory = 2;
  string total_cpu = 3;
  string total_storage = 4;
}

// Health check
message HealthCheckRequest {}

message HealthCheckResponse {
  string status = 1; // "healthy", "unhealthy"
  string message = 2;
  int32 current_sessions = 3;
  ResourceUsage resource_usage = 4;
}
```

**Generate Go code**:
```bash
# Install protoc and plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  api/proto/controller.proto
```

### Week 2: API Server Changes

#### 2.1 Controller Manager

**Location**: Create `api/internal/controller/manager.go`

```go
package controller

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "sync"
    "time"

    "github.com/streamspace/streamspace/api/internal/db"
    pb "github.com/streamspace/streamspace/api/proto/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

// ControllerManager manages all registered controllers
type ControllerManager struct {
    mu          sync.RWMutex
    controllers map[string]*ControllerClient // controller_id -> client
    db          *db.Database
    defaultKubernetesControllerID string // For backward compatibility
}

// ControllerClient wraps a gRPC connection to a controller
type ControllerClient struct {
    ID           string
    Type         string
    Name         string
    Endpoint     string
    Conn         *grpc.ClientConn
    Client       pb.ControllerServiceClient
    Capabilities *pb.CapabilitiesResponse
    Status       string
    LastHeartbeat time.Time
    SessionCount int
}

// NewControllerManager creates a new controller manager
func NewControllerManager(database *db.Database) *ControllerManager {
    return &ControllerManager{
        controllers: make(map[string]*ControllerClient),
        db:          database,
    }
}

// Initialize loads existing controllers from database
func (m *ControllerManager) Initialize(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Load all controllers from database
    controllers, err := m.db.ListControllers(ctx)
    if err != nil {
        return err
    }

    // Connect to each controller
    for _, ctrl := range controllers {
        if err := m.connectToController(ctx, ctrl); err != nil {
            // Log error but continue
            fmt.Printf("Failed to connect to controller %s: %v\n", ctrl.ID, err)
            continue
        }
    }

    return nil
}

// RegisterController registers a new controller
func (m *ControllerManager) RegisterController(ctx context.Context, req *RegisterControllerRequest) (*RegisterControllerResponse, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Generate controller ID
    controllerID := generateControllerID(req.Type, req.Name)

    // Connect to controller gRPC endpoint
    conn, err := grpc.Dial(req.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return nil, fmt.Errorf("failed to connect to controller: %w", err)
    }

    client := pb.NewControllerServiceClient(conn)

    // Get capabilities
    capResp, err := client.GetCapabilities(ctx, &pb.GetCapabilitiesRequest{})
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("failed to get capabilities: %w", err)
    }

    // Generate auth token
    token := generateAuthToken()
    tokenHash := hashToken(token)

    // Store in database
    controller := &db.Controller{
        ID:           controllerID,
        Type:         req.Type,
        Name:         req.Name,
        Endpoint:     req.Endpoint,
        Status:       "healthy",
        Capabilities: capResp,
        Labels:       req.Labels,
        RegisteredAt: time.Now(),
    }

    if err := m.db.CreateController(ctx, controller); err != nil {
        conn.Close()
        return nil, err
    }

    if err := m.db.CreateControllerToken(ctx, controllerID, tokenHash); err != nil {
        conn.Close()
        return nil, err
    }

    // Add to in-memory map
    m.controllers[controllerID] = &ControllerClient{
        ID:           controllerID,
        Type:         req.Type,
        Name:         req.Name,
        Endpoint:     req.Endpoint,
        Conn:         conn,
        Client:       client,
        Capabilities: capResp,
        Status:       "healthy",
        LastHeartbeat: time.Now(),
    }

    // Start health check for this controller
    go m.healthCheckLoop(controllerID)

    return &RegisterControllerResponse{
        ControllerID: controllerID,
        AuthToken:    token,
        APIVersion:   "v1alpha1",
    }, nil
}

// GetController returns a controller by ID
func (m *ControllerManager) GetController(controllerID string) (*ControllerClient, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    ctrl, ok := m.controllers[controllerID]
    if !ok {
        return nil, fmt.Errorf("controller not found: %s", controllerID)
    }

    return ctrl, nil
}

// SelectController chooses a controller for a new session
func (m *ControllerManager) SelectController(ctx context.Context, req *SelectControllerRequest) (*ControllerClient, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Strategy 1: Explicit controller ID
    if req.ControllerID != "" {
        return m.controllers[req.ControllerID], nil
    }

    // Strategy 2: Explicit controller type
    if req.ControllerType != "" {
        return m.selectByType(req.ControllerType)
    }

    // Strategy 3: Template preference
    if req.Template != nil && req.Template.PreferredControllerType != "" {
        return m.selectByType(req.Template.PreferredControllerType)
    }

    // Strategy 4: Default to Kubernetes
    if m.defaultKubernetesControllerID != "" {
        return m.controllers[m.defaultKubernetesControllerID], nil
    }

    // Strategy 5: Least loaded
    return m.selectLeastLoaded()
}

func (m *ControllerManager) selectByType(controllerType string) (*ControllerClient, error) {
    candidates := []*ControllerClient{}

    for _, ctrl := range m.controllers {
        if ctrl.Type == controllerType && ctrl.Status == "healthy" {
            candidates = append(candidates, ctrl)
        }
    }

    if len(candidates) == 0 {
        return nil, fmt.Errorf("no healthy controller of type %s", controllerType)
    }

    // Return least loaded
    leastLoaded := candidates[0]
    for _, ctrl := range candidates {
        if ctrl.SessionCount < leastLoaded.SessionCount {
            leastLoaded = ctrl
        }
    }

    return leastLoaded, nil
}

func (m *ControllerManager) selectLeastLoaded() (*ControllerClient, error) {
    var leastLoaded *ControllerClient

    for _, ctrl := range m.controllers {
        if ctrl.Status != "healthy" {
            continue
        }

        if leastLoaded == nil || ctrl.SessionCount < leastLoaded.SessionCount {
            leastLoaded = ctrl
        }
    }

    if leastLoaded == nil {
        return nil, fmt.Errorf("no healthy controllers available")
    }

    return leastLoaded, nil
}

// Health check loop for a controller
func (m *ControllerManager) healthCheckLoop(controllerID string) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        m.checkControllerHealth(controllerID)
    }
}

func (m *ControllerManager) checkControllerHealth(controllerID string) {
    m.mu.RLock()
    ctrl, ok := m.controllers[controllerID]
    m.mu.RUnlock()

    if !ok {
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := ctrl.Client.HealthCheck(ctx, &pb.HealthCheckRequest{})
    if err != nil {
        // Mark as unhealthy
        m.mu.Lock()
        ctrl.Status = "unhealthy"
        m.mu.Unlock()
        m.db.UpdateControllerStatus(context.Background(), controllerID, "unhealthy")
        return
    }

    // Update status
    m.mu.Lock()
    ctrl.Status = resp.Status
    ctrl.LastHeartbeat = time.Now()
    ctrl.SessionCount = int(resp.CurrentSessions)
    m.mu.Unlock()

    m.db.UpdateControllerStatus(context.Background(), controllerID, resp.Status)
    m.db.UpdateControllerSessionCount(context.Background(), controllerID, int(resp.CurrentSessions))
}

// Helper functions
func generateControllerID(ctrlType, name string) string {
    timestamp := time.Now().UnixNano()
    raw := fmt.Sprintf("%s-%s-%d", ctrlType, name, timestamp)
    hash := sha256.Sum256([]byte(raw))
    return fmt.Sprintf("ctrl-%s", hex.EncodeToString(hash[:8]))
}

func generateAuthToken() string {
    // Generate secure random token
    // Implementation: use crypto/rand
    return "jwt-token-here" // TODO: Implement JWT generation
}

func hashToken(token string) string {
    hash := sha256.Sum256([]byte(token))
    return hex.EncodeToString(hash[:])
}

// Request/Response types
type RegisterControllerRequest struct {
    Type     string
    Name     string
    Endpoint string
    Labels   map[string]string
}

type RegisterControllerResponse struct {
    ControllerID string
    AuthToken    string
    APIVersion   string
}

type SelectControllerRequest struct {
    ControllerID   string
    ControllerType string
    Template       *Template
}

type Template struct {
    PreferredControllerType string
}
```

#### 2.2 Controller API Endpoints

**Location**: Create `api/internal/handlers/controllers.go`

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/streamspace/streamspace/api/internal/controller"
)

type ControllerHandler struct {
    controllerManager *controller.ControllerManager
}

func NewControllerHandler(cm *controller.ControllerManager) *ControllerHandler {
    return &ControllerHandler{
        controllerManager: cm,
    }
}

// RegisterController handles controller registration
// POST /api/v1/controllers/register
func (h *ControllerHandler) RegisterController(c *gin.Context) {
    var req controller.RegisterControllerRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    resp, err := h.controllerManager.RegisterController(c.Request.Context(), &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, resp)
}

// ListControllers lists all registered controllers (admin only)
// GET /api/v1/admin/controllers
func (h *ControllerHandler) ListControllers(c *gin.Context) {
    // TODO: Implement list controllers from database
    c.JSON(http.StatusOK, gin.H{"controllers": []string{}})
}

// GetController gets a specific controller (admin only)
// GET /api/v1/admin/controllers/:id
func (h *ControllerHandler) GetController(c *gin.Context) {
    controllerID := c.Param("id")
    ctrl, err := h.controllerManager.GetController(controllerID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Controller not found"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "id":              ctrl.ID,
        "type":            ctrl.Type,
        "name":            ctrl.Name,
        "status":          ctrl.Status,
        "session_count":   ctrl.SessionCount,
        "last_heartbeat":  ctrl.LastHeartbeat,
        "capabilities":    ctrl.Capabilities,
    })
}

// Heartbeat handles controller heartbeat
// POST /api/v1/controllers/:id/heartbeat
func (h *ControllerHandler) Heartbeat(c *gin.Context) {
    // TODO: Verify controller auth token
    // TODO: Update last_heartbeat in database
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

**Register routes** in `api/internal/api/handlers.go`:

```go
// Controller routes
controllerHandler := handlers.NewControllerHandler(controllerManager)
api.POST("/controllers/register", controllerHandler.RegisterController)
api.POST("/controllers/:id/heartbeat", controllerHandler.Heartbeat)

admin := api.Group("/admin")
admin.Use(middleware.RequireAdmin())
{
    admin.GET("/controllers", controllerHandler.ListControllers)
    admin.GET("/controllers/:id", controllerHandler.GetController)
}
```

### Week 3: Session Handler Refactoring

**Location**: Modify `api/internal/handlers/sessiontemplates.go` (or create new session handler)

**Key changes**:

1. Add controller selection logic
2. Call controller via gRPC instead of creating Kubernetes CRDs directly
3. Store `controller_id` and `node_id` in database

```go
// CreateSession (modified)
func (h *SessionHandler) CreateSession(c *gin.Context) {
    var req CreateSessionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Get user from context
    userID := c.GetString("user_id")

    // Get template
    template, err := h.db.GetTemplate(c.Request.Context(), req.TemplateID)
    if err != nil {
        c.JSON(404, gin.H{"error": "Template not found"})
        return
    }

    // SELECT CONTROLLER
    selectReq := &controller.SelectControllerRequest{
        ControllerID:   req.ControllerID,   // Optional explicit selection
        ControllerType: req.ControllerType, // Optional type selection
        Template:       template,            // For template preferences
    }

    ctrl, err := h.controllerManager.SelectController(c.Request.Context(), selectReq)
    if err != nil {
        c.JSON(500, gin.H{"error": "No available controller"})
        return
    }

    // Generate session ID
    sessionID := generateSessionID(userID, template.Name)

    // CREATE SESSION IN DATABASE (pending state)
    session := &db.Session{
        ID:             sessionID,
        UserID:         userID,
        TemplateID:     req.TemplateID,
        ControllerID:   ctrl.ID,
        ControllerType: ctrl.Type,
        State:          "pending",
        Spec:           req.Spec,
        CreatedAt:      time.Now(),
    }

    if err := h.db.CreateSession(c.Request.Context(), session); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // CALL CONTROLLER VIA GRPC
    grpcReq := &pb.CreateSessionRequest{
        SessionId:  sessionID,
        UserId:     userID,
        TemplateId: req.TemplateID,
        Spec:       convertToProtoSpec(req.Spec),
    }

    grpcResp, err := ctrl.Client.CreateSession(c.Request.Context(), grpcReq)
    if err != nil {
        // Update session to failed
        h.db.UpdateSessionState(c.Request.Context(), sessionID, "failed")
        c.JSON(500, gin.H{"error": fmt.Sprintf("Controller error: %v", err)})
        return
    }

    // UPDATE SESSION WITH CONTROLLER RESPONSE
    session.NodeID = grpcResp.NodeId
    session.State = grpcResp.Phase
    session.URL = grpcResp.Url
    session.UpdatedAt = time.Now()

    if err := h.db.UpdateSession(c.Request.Context(), session); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(201, session)
}
```

---

## Phase 2: Kubernetes Controller Refactoring (Weeks 4-5)

### 4.1 Add gRPC Server to Kubernetes Controller

**Location**: Create `controller/internal/grpc/server.go`

```go
package grpc

import (
    "context"
    "fmt"

    pb "github.com/streamspace/streamspace/api/proto/v1"
    "github.com/streamspace/streamspace/controller/internal/session"
)

type ControllerServer struct {
    pb.UnimplementedControllerServiceServer
    sessionManager *session.Manager
}

func NewControllerServer(sessionMgr *session.Manager) *ControllerServer {
    return &ControllerServer{
        sessionManager: sessionMgr,
    }
}

func (s *ControllerServer) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
    // Call existing session creation logic (which creates CRD)
    result, err := s.sessionManager.CreateSession(ctx, &session.CreateRequest{
        SessionID:  req.SessionId,
        UserID:     req.UserId,
        TemplateID: req.TemplateId,
        Spec:       convertFromProtoSpec(req.Spec),
    })

    if err != nil {
        return nil, err
    }

    return &pb.CreateSessionResponse{
        SessionId: result.SessionID,
        NodeId:    result.PodName, // Pod name is the "node ID"
        Phase:     result.Phase,
        Url:       result.URL,
    }, nil
}

func (s *ControllerServer) HibernateSession(ctx context.Context, req *pb.HibernateSessionRequest) (*pb.HibernateSessionResponse, error) {
    err := s.sessionManager.HibernateSession(ctx, req.SessionId, req.NodeId)
    if err != nil {
        return nil, err
    }

    return &pb.HibernateSessionResponse{
        SessionId: req.SessionId,
        Phase:     "Hibernated",
    }, nil
}

// Implement other methods...

func (s *ControllerServer) GetCapabilities(ctx context.Context, req *pb.GetCapabilitiesRequest) (*pb.CapabilitiesResponse, error) {
    return &pb.CapabilitiesResponse{
        ControllerType:            "kubernetes",
        Version:                   "v1.0.0",
        SupportsHibernation:       true,
        SupportsGpu:              true,
        SupportsPersistentStorage: true,
        SupportsNetworking:        true,
        Capacity: &pb.ResourceCapacity{
            MaxSessions:  100,
            TotalMemory:  "256Gi",
            TotalCpu:     "64",
            TotalStorage: "1Ti",
        },
    }, nil
}

func (s *ControllerServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
    // Get current session count from Kubernetes
    count, usage := s.sessionManager.GetStats()

    return &pb.HealthCheckResponse{
        Status:          "healthy",
        CurrentSessions: int32(count),
        ResourceUsage: &pb.ResourceUsage{
            Memory:        usage.Memory,
            Cpu:           usage.CPU,
            MemoryPercent: usage.MemoryPercent,
            CpuPercent:    usage.CPUPercent,
        },
    }, nil
}
```

### 4.2 Add Registration Logic

**Location**: Modify `controller/cmd/main.go`

```go
func main() {
    // ... existing setup ...

    // Start gRPC server
    grpcServer := startGRPCServer(sessionManager)

    // Register with API server
    if err := registerWithAPIServer(); err != nil {
        log.Fatalf("Failed to register with API server: %v", err)
    }

    // Start heartbeat
    go startHeartbeat()

    // ... existing controller manager run ...
}

func startGRPCServer(sessionMgr *session.Manager) *grpc.Server {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()
    pb.RegisterControllerServiceServer(grpcServer, grpc.NewControllerServer(sessionMgr))

    go func() {
        if err := grpcServer.Serve(lis); err != nil {
            log.Fatalf("Failed to serve: %v", err)
        }
    }()

    return grpcServer
}

func registerWithAPIServer() error {
    apiServerURL := os.Getenv("API_SERVER_URL")
    if apiServerURL == "" {
        apiServerURL = "http://streamspace-api:8080"
    }

    req := map[string]interface{}{
        "type":     "kubernetes",
        "name":     os.Getenv("CONTROLLER_NAME"),
        "endpoint": fmt.Sprintf("grpc://%s:50051", os.Getenv("POD_IP")),
        "labels": map[string]string{
            "cluster": os.Getenv("CLUSTER_NAME"),
        },
    }

    // POST to /api/v1/controllers/register
    // Store returned auth token for heartbeats
    // ...
}

func startHeartbeat() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        // POST to /api/v1/controllers/:id/heartbeat
        // Include current session count and resource usage
    }
}
```

---

## Testing Strategy

### Unit Tests

1. **Controller Manager**: Registration, selection logic
2. **gRPC handlers**: Session create/delete/hibernate/wake
3. **Database**: New tables and queries

### Integration Tests

1. **End-to-end session creation**: API → Controller → Kubernetes
2. **Controller registration**: Controller → API Server
3. **Health checks**: API Server → Controller
4. **Session lifecycle**: Create → Hibernate → Wake → Delete

### Manual Testing

```bash
# 1. Start API server with controller manager
make run-api

# 2. Start Kubernetes controller with gRPC server
make run-controller-kubernetes

# 3. Verify controller registered
curl http://localhost:8080/api/v1/admin/controllers

# 4. Create session (should use Kubernetes controller)
curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "template_id": "firefox-browser",
    "resources": {"memory": "2Gi", "cpu": "1000m"}
  }'

# 5. Verify session created in Kubernetes
kubectl get sessions -n streamspace

# 6. Check database
psql -c "SELECT id, controller_id, controller_type, node_id FROM sessions;"
```

---

## Rollback Plan

If issues arise, the rollback is simple:

1. **Database**: Controllers and sessions work without new fields (NULL is OK)
2. **API Server**: Can run without ControllerManager (falls back to direct Kubernetes client)
3. **Controller**: Can run without gRPC server (CRD reconciliation continues)

**To rollback**:
```bash
# Redeploy previous versions
kubectl rollout undo deployment/streamspace-api
kubectl rollout undo deployment/streamspace-controller
```

---

## Next Steps

1. **Review this document** with the team
2. **Create JIRA tickets** for each week's tasks
3. **Implement Week 1** (database + proto)
4. **Code review** after each week
5. **Document as you go** (update API docs, deployment guides)

---

## Summary

This phased approach allows incremental implementation with backward compatibility at every step. The Kubernetes controller remains functional while we add the multi-controller infrastructure around it.

**Key Benefits**:
- ✅ No breaking changes for existing deployments
- ✅ Kubernetes controller works during entire migration
- ✅ New controllers can be added without API changes
- ✅ Database migrations are additive (no data loss)
- ✅ Rollback is straightforward

**Estimated Timeline**: 5 weeks for Phase 1 (core infrastructure + Kubernetes refactoring)

