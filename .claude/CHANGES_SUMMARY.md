# StreamSpace v2.0 Architecture Refactor - Changes Summary

**Last Updated:** 2025-11-21
**Status:** v1.0.0 REFACTOR-READY → v2.0 Architecture Refactor In Progress

---

## What Changed

StreamSpace is undergoing a major architecture refactor from a **Kubernetes-native single-cluster platform** to a **multi-platform Control Plane + Agent architecture** that supports Kubernetes, Docker, VMs, and cloud platforms.

This document summarizes the key changes between v1.0.0 and v2.0.

---

## v1.0.0 Achievements (REFACTOR-READY Status)

Before starting the v2.0 refactor, StreamSpace achieved production-ready status:

### Core Platform
- ✅ **82%+ completion rate** across all features
- ✅ **87 database tables** (verified, production-ready schema)
- ✅ **70+ API handlers** (66,988 lines of Go code)
- ✅ **Kubernetes controller** (6,562 lines, Kubebuilder-based)
- ✅ **54 UI components/pages** (React 18+, Material-UI)

### Admin Features (100% of P0, 25% of P1 Complete)
- ✅ **Audit Logs Viewer** (1,131 lines) - SOC2/HIPAA/GDPR compliance
- ✅ **System Configuration** (938 lines) - 7 categories, full config UI
- ✅ **License Management** (1,814 lines) - Community/Pro/Enterprise tiers
- ✅ **API Keys Management** (1,217 lines) - Scope-based access control

### Quality & Testing
- ✅ **11,131 lines of tests** (464 test cases)
- ✅ **65-70% controller coverage** (+32 test cases added)
- ✅ **6,700+ lines of documentation** (comprehensive technical docs)

### Enterprise Readiness
- ✅ **Authentication**: SAML, OIDC, MFA, JWT (all implemented)
- ✅ **Audit Compliance**: SOC2, HIPAA, GDPR, ISO 27001 support
- ✅ **License Enforcement**: 3-tier licensing with feature gating
- ✅ **API Automation**: API keys with rate limiting and scopes

**Conclusion:** v1.0.0 is production-ready and can be deployed, but the architecture is limited to single Kubernetes clusters.

---

## Why v2.0 Refactor?

### Current Architecture Limitations (v1.0.0)

**Kubernetes-Native Architecture:**
```
User → Web UI → Go API → K8s Controller → K8s Pods
                                            ↓
                                      VNC (direct from pods)
```

**Problems:**
1. **Single-Cluster Only**: Can only deploy to one Kubernetes cluster
2. **Platform Locked**: Cannot support Docker hosts, VMs, or cloud platforms
3. **Network Constraints**: VNC streaming requires direct pod access
4. **Scaling Limits**: All sessions must be in the same cluster as the API
5. **No Multi-Region**: Cannot distribute sessions across regions/clouds

### Target Architecture (v2.0)

**Multi-Platform Control Plane + Agents:**
```
User → Web UI → Control Plane API (Centralized)
                      ↓
          ┌───────────┼───────────┐
          ↓           ↓           ↓
    K8s Agent    Docker Agent   VM Agent
    (Cluster 1)   (Host 1)     (Cloud 1)
          ↓           ↓           ↓
     K8s Pods    Containers   Virtual Machines
```

**Benefits:**
1. ✅ **Multi-Platform**: Kubernetes, Docker, VMs, Cloud (AWS, Azure, GCP)
2. ✅ **Multi-Region**: Deploy agents anywhere, sessions routed optimally
3. ✅ **Network Flexibility**: VNC tunneled through Control Plane WebSocket
4. ✅ **Independent Scaling**: Scale Control Plane and Agents separately
5. ✅ **Firewall-Friendly**: Agents connect TO Control Plane (outbound only)
6. ✅ **Platform Abstraction**: Generic "Session" concept, agents translate

---

## Major Architecture Changes

### 1. Control Plane (Centralized Management)

**What Changed:**
- **v1.0:** Kubernetes controller directly manages pods
- **v2.0:** Control Plane API manages all platforms through agents

**New Components:**
- Agent Registration API (POST /api/v1/agents/register)
- WebSocket Hub (maintains agent connections)
- Command Dispatcher (queues commands to agents)
- VNC Proxy/Tunnel (proxies VNC through WebSocket)
- Session State Manager (platform-agnostic tracking)

**Files:**
- `api/internal/handlers/agents.go` (NEW) - Agent management API
- `api/internal/models/agent.go` (NEW) - Agent data models
- `api/internal/db/database.go` (MODIFIED) - New tables: agents, agent_commands

### 2. Platform-Specific Agents

**What Changed:**
- **v1.0:** Single Kubernetes controller
- **v2.0:** Multiple platform-specific agents

**Agent Types:**
- **K8s Agent**: Manages Kubernetes sessions (converted from v1.0 controller)
- **Docker Agent**: Manages Docker container sessions
- **VM Agent**: Manages virtual machine sessions (future)
- **Cloud Agent**: Manages cloud provider sessions (future)

**Agent Responsibilities:**
- Connect to Control Plane via WebSocket (outbound connection)
- Receive commands (start_session, stop_session, hibernate_session, wake_session)
- Translate generic session spec to platform-specific resources
- Tunnel VNC traffic back to Control Plane
- Report session status and health

### 3. WebSocket-Based Communication

**What Changed:**
- **v1.0:** Direct Kubernetes API communication
- **v2.0:** WebSocket-based command and VNC tunneling

**Protocol:**
```
Agent → Control Plane WebSocket Connection (persistent)
  ↓
Control Plane sends commands as JSON messages
  ↓
Agent acknowledges and executes
  ↓
Agent tunnels VNC traffic through same WebSocket
```

**Benefits:**
- Works through firewalls (agents initiate connection)
- Bidirectional real-time communication
- Single connection for commands + VNC tunneling
- Automatic reconnection and heartbeats

### 4. VNC Tunneling Architecture

**What Changed:**
- **v1.0:** UI connects directly to pod IP (VNC on port 5900/3000)
- **v2.0:** UI connects to Control Plane proxy, tunneled to agents

**Old VNC Flow (v1.0):**
```
UI → Direct WebSocket → Pod IP:5900
```

**New VNC Flow (v2.0):**
```
UI → Control Plane (/vnc/{sessionId})
      ↓
Control Plane WebSocket Hub
      ↓
Agent WebSocket Connection
      ↓
Agent Port-Forward to Local Pod/Container
      ↓
VNC Server (port 5900)
```

**Benefits:**
- Works across networks (no direct pod access required)
- Works through NAT/firewalls
- Supports sessions on any platform (K8s, Docker, VM, Cloud)
- Centralized access control and audit logging

### 5. Database Schema Changes

**New Tables:**

**agents table** (platform-specific execution agents)
```sql
- id (UUID, primary key)
- agent_id (VARCHAR, unique) - User-defined ID like "k8s-prod-us-east-1"
- platform (VARCHAR) - kubernetes, docker, vm, cloud
- region (VARCHAR) - Geographical/logical region
- status (VARCHAR) - online, offline, draining
- capacity (JSONB) - Resource limits
- last_heartbeat (TIMESTAMP)
- websocket_id (VARCHAR) - Active WebSocket connection ID
- metadata (JSONB) - Platform-specific data
- created_at, updated_at
```

**agent_commands table** (command queue)
```sql
- id (UUID, primary key)
- command_id (VARCHAR, unique)
- agent_id (VARCHAR, foreign key to agents)
- session_id (VARCHAR) - Affected session
- action (VARCHAR) - start_session, stop_session, hibernate_session, wake_session
- payload (JSONB) - Command-specific data
- status (VARCHAR) - pending, sent, ack, completed, failed
- error_message (TEXT)
- created_at, sent_at, acknowledged_at, completed_at
```

**sessions table alterations:**
```sql
- agent_id (VARCHAR) - Which agent manages this session
- platform (VARCHAR) - kubernetes, docker, vm, cloud
- platform_metadata (JSONB) - Platform-specific details (pod name, container ID, etc.)
```

**12 new indexes** for performance optimization.

### 6. UI Changes

**Admin UI - New Agents Management Page:**
- View all registered agents
- Filter by platform, status, region
- See agent capacity and active sessions
- Monitor agent health (last heartbeat)
- Deregister offline agents
- View agent-specific metadata

**Session List Updates:**
- Display agent ID and platform for each session
- Filter sessions by agent/platform
- Show platform-specific metadata

**Session Creation Updates:**
- Select target platform (if multiple available)
- Optional region preference
- Platform-specific resource options

**VNC Viewer Critical Update:**
```javascript
// Old (v1.0)
const vncUrl = `ws://${podIP}:5900`;

// New (v2.0)
const vncUrl = `/vnc/${sessionId}`;  // Proxied through Control Plane
```

**Admin Dashboard Updates:**
- Agent count by platform
- Agent health status (online/offline/draining)
- Sessions by platform breakdown
- Multi-platform system health

---

## Implementation Phases (10 Total)

### Phase 1: Design & Documentation ✅ COMPLETE
**Duration:** 2 days
**Deliverables:**
- ✅ `docs/REFACTOR_ARCHITECTURE_V2.md` (727 lines)
- ✅ Complete architecture specification
- ✅ WebSocket protocol design
- ✅ Database schema design
- ✅ Migration path documented

### Phase 2: Agent Registration API 🔄 IN PROGRESS
**Duration:** 3-5 days
**Assigned To:** Builder
**Deliverables:**
- 5 HTTP endpoints for agent management
- Unit tests (>70% coverage)
- Input validation and error handling

### Phase 3: WebSocket Command Channel ⏳ PENDING
**Duration:** 5-7 days
**Deliverables:**
- WebSocket hub implementation
- Command dispatcher
- Heartbeat monitoring
- Reconnection logic

### Phase 4: VNC Proxy/Tunnel ⏳ PENDING
**Duration:** 4-6 days
**Deliverables:**
- VNC proxy endpoint (/vnc/{sessionId})
- Binary WebSocket tunneling
- Connection routing to agents
- Error handling and timeouts

### Phase 5: K8s Agent Conversion ⏳ PENDING
**Duration:** 7-10 days
**Deliverables:**
- Convert existing controller to K8s Agent
- WebSocket client connection to Control Plane
- Command handling (start, stop, hibernate, wake)
- Backward compatibility with v1.0 sessions

### Phase 6: K8s Agent VNC Tunneling ⏳ PENDING
**Duration:** 3-5 days
**Deliverables:**
- Port-forward to local pods
- VNC tunnel through WebSocket
- Integration with Control Plane proxy

### Phase 7: Docker Agent ⏳ PENDING
**Duration:** 7-10 days
**Deliverables:**
- Docker Agent implementation (new)
- Docker container lifecycle management
- VNC tunneling for Docker containers
- Agent registration and heartbeats

### Phase 8: UI Updates ⏳ PENDING
**Duration:** 5-7 days
**Deliverables:**
- Admin Agents Management page (new)
- Session list/details updates
- Session creation form updates
- VNC Viewer proxy connection update (CRITICAL)
- Admin dashboard updates

### Phase 9: Database Schema ✅ COMPLETE
**Duration:** 1 day
**Deliverables:**
- ✅ `agents` table created
- ✅ `agent_commands` table created
- ✅ `sessions` table alterations (agent_id, platform, platform_metadata)
- ✅ 12 indexes for performance

### Phase 10: Testing & Migration ⏳ PENDING
**Duration:** 7-10 days
**Deliverables:**
- Integration tests (Control Plane + K8s Agent)
- E2E tests (session creation across platforms)
- Migration guide (v1.0 → v2.0)
- Backward compatibility testing

**Total Estimated Duration:** 6-8 weeks

---

## Breaking Changes

### API Changes

**Session Creation:**
```javascript
// Old (v1.0)
POST /api/v1/sessions
{
  "user": "alice",
  "template": "firefox-browser"
}

// New (v2.0) - Optional platform/region
POST /api/v1/sessions
{
  "user": "alice",
  "template": "firefox-browser",
  "platform": "kubernetes",  // Optional: auto-select if omitted
  "region": "us-east-1"      // Optional: prefer region
}
```

**Session Response:**
```javascript
// Old (v1.0)
{
  "id": "sess-123",
  "user": "alice",
  "template": "firefox-browser",
  "state": "running"
}

// New (v2.0) - Includes platform info
{
  "id": "sess-123",
  "user": "alice",
  "template": "firefox-browser",
  "state": "running",
  "agentId": "k8s-prod-us-east-1",
  "platform": "kubernetes",
  "platformMetadata": {
    "podName": "sess-123-abc",
    "nodeName": "worker-1"
  }
}
```

### VNC Connection

**Critical Change:**
```javascript
// Old (v1.0) - Direct pod connection
const vncUrl = `ws://${session.podIP}:5900`;
rfb.connect(vncUrl);

// New (v2.0) - Proxied through Control Plane
const vncUrl = `/vnc/${sessionId}`;  // Relative URL, proxied by Control Plane
rfb.connect(vncUrl);
```

**Why This Matters:**
- Old approach requires direct network access to pods
- New approach works across networks, through firewalls
- Enables sessions on Docker hosts, VMs, cloud platforms

### Kubernetes Controller Deployment

**Old (v1.0):**
```bash
# Single controller, manages local cluster only
kubectl apply -f manifests/controller.yaml
```

**New (v2.0):**
```bash
# 1. Deploy Control Plane (centralized)
kubectl apply -f manifests/control-plane.yaml

# 2. Deploy K8s Agent to each cluster (connects to Control Plane)
kubectl apply -f manifests/k8s-agent.yaml

# 3. Deploy Docker Agent to each Docker host
docker run streamspace/docker-agent --control-plane-url https://control.example.com
```

---

## Migration Path (v1.0 → v2.0)

### Option 1: In-Place Migration (Recommended for Small Deployments)

1. **Backup existing sessions** (export session data)
2. **Deploy v2.0 Control Plane** (new API with agent support)
3. **Convert K8s controller to K8s Agent** (connects to Control Plane)
4. **Update UI** (VNC proxy connection)
5. **Migrate sessions** (update session records with agent_id, platform)
6. **Test VNC connectivity** (ensure proxy works)
7. **Remove v1.0 controller** (replaced by K8s Agent)

**Downtime:** 15-30 minutes (during controller conversion)

### Option 2: Blue-Green Deployment (Recommended for Production)

1. **Deploy v2.0 Control Plane** (parallel to v1.0)
2. **Deploy K8s Agent** (connects to v2.0 Control Plane)
3. **Create new sessions on v2.0** (test platform)
4. **Gradually migrate users** (session by session)
5. **Keep v1.0 running** (until all sessions migrated)
6. **Decommission v1.0** (when migration complete)

**Downtime:** Zero (gradual migration)

### Backward Compatibility

**v2.0 K8s Agent maintains compatibility with:**
- Existing Session CRDs (no schema changes)
- Existing Template CRDs (no schema changes)
- Existing PVCs for persistent home directories
- Existing VNC image format (LinuxServer.io)

**What Changes:**
- Session records include `agent_id`, `platform`, `platform_metadata`
- VNC connections proxied through Control Plane
- Session creation can specify platform/region preferences

---

## Current Status (2025-11-21)

### Completed ✅
- Phase 1: Design & Documentation (727 lines)
- Phase 9: Database Schema (agents, agent_commands tables)
- All .claude coordination files updated
- Multi-agent workflow coordinated

### In Progress 🔄
- Phase 2: Agent Registration API (Builder assigned, 3-5 days)

### Next Up ⏳
- Phase 3: WebSocket Command Channel (5-7 days)
- Phase 4: VNC Proxy/Tunnel (4-6 days)
- Phase 5: K8s Agent Conversion (7-10 days)

### Remaining Work
- 7 more phases (6-7 weeks estimated)
- Integration testing (1-2 weeks)
- Migration testing (1 week)
- Documentation updates (ongoing)

---

## Success Criteria

### Phase Completion Criteria
- All 10 phases complete with acceptance criteria met
- Unit tests >70% coverage for all new code
- Integration tests passing (Control Plane + K8s Agent)
- E2E tests passing (session creation, VNC connection)

### v2.0 Release Criteria
- ✅ K8s Agent fully functional (backward compatible with v1.0)
- ✅ Docker Agent fully functional (new platform)
- ✅ VNC tunneling working across networks
- ✅ Admin UI for agent management complete
- ✅ Migration guide tested and documented
- ✅ Test coverage >70% for all components

### Future Enhancements (Post-v2.0)
- VM Agent implementation
- Cloud Agent implementations (AWS, Azure, GCP)
- Multi-region session routing optimization
- Agent auto-scaling based on capacity
- Advanced session placement algorithms

---

## Files Updated for v2.0 Refactor

### Documentation
- ✅ `docs/REFACTOR_ARCHITECTURE_V2.md` (NEW, 727 lines)
- ✅ `.claude/README.md` (UPDATED)
- ✅ `.claude/QUICK_REFERENCE.md` (UPDATED)
- ✅ `.claude/CHANGES_SUMMARY.md` (UPDATED, this file)
- ✅ `.claude/multi-agent/MULTI_AGENT_PLAN.md` (UPDATED, Phase 2-8 added)

### Backend Code
- ✅ `api/internal/models/agent.go` (NEW, 468 lines)
- ✅ `api/internal/db/database.go` (MODIFIED, +79 lines for v2.0 schema)
- ⏳ `api/internal/handlers/agents.go` (PENDING, Builder assigned)

### Multi-Agent Coordination
- ✅ `.claude/multi-agent/agent1-architect-instructions.md` (UPDATED)
- ✅ `.claude/multi-agent/agent2-builder-instructions.md` (UPDATED)
- ✅ `.claude/multi-agent/agent3-validator-instructions.md` (UPDATED)
- ✅ `.claude/multi-agent/agent4-scribe-instructions.md` (UPDATED)

---

## Key Takeaways

1. **v1.0.0 is Production-Ready**: 82%+ complete, admin features done, can deploy now
2. **v2.0 is Architecture Evolution**: Multi-platform support, not a rewrite
3. **Backward Compatible**: K8s Agent maintains v1.0 functionality
4. **Bottom-Up Approach**: Database → K8s Agent → Docker Agent → UI
5. **Estimated Timeline**: 6-8 weeks for full v2.0 implementation
6. **Current Focus**: Phase 2 (Agent Registration API) - Builder working
7. **Multi-Agent Coordination**: 4 agents working in parallel on different phases

---

**Next Milestone:** Phase 2 completion (Agent Registration API with 5 endpoints + tests)

**Questions?** See `.claude/multi-agent/MULTI_AGENT_PLAN.md` for detailed phase specifications and current task assignments.
