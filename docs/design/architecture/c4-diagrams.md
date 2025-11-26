# C4 Model Architecture Diagrams

**Version**: v2.0-beta
**Last Updated**: 2025-11-26
**Owner**: Agent 1 (Architect)
**Status**: Living Document

---

## Introduction

This document provides C4 model architecture diagrams for StreamSpace using Mermaid notation. The C4 model (Context, Containers, Components, Code) provides a hierarchical way to visualize software architecture at different levels of abstraction.

**Reference**: [C4 Model](https://c4model.com/) by Simon Brown

---

## Level 1: System Context Diagram

Shows StreamSpace in the context of its users and external systems.

```mermaid
graph TB
    subgraph External Users
        DevUser[Developer/End User<br/>Uses containerized apps via browser]
        OrgAdmin[Organization Admin<br/>Manages users, templates, policies]
        SysAdmin[System Admin<br/>Deploys and monitors platform]
    end

    subgraph StreamSpace Platform
        StreamSpace[StreamSpace<br/>Container streaming platform<br/>Delivers GUI apps to browsers]
    end

    subgraph External Systems
        SSO[SSO Provider<br/>SAML/OIDC/OAuth2<br/>Okta, Auth0, Keycloak]
        Registry[Container Registry<br/>Docker Hub, ECR, GCR<br/>Application images]
        Storage[Object Storage<br/>S3, NFS, CSI<br/>User home directories]
        Monitoring[Monitoring<br/>Prometheus, Grafana<br/>Metrics & alerts]
    end

    DevUser -->|Access sessions via browser| StreamSpace
    OrgAdmin -->|Manage org, users, templates| StreamSpace
    SysAdmin -->|Deploy, monitor, configure| StreamSpace

    StreamSpace -->|Authenticate users| SSO
    StreamSpace -->|Pull container images| Registry
    StreamSpace -->|Store user data| Storage
    StreamSpace -->|Export metrics| Monitoring

    style StreamSpace fill:#4a90e2,stroke:#2e5c8a,stroke-width:3px,color:#fff
    style SSO fill:#e8f4f8,stroke:#4a90e2
    style Registry fill:#e8f4f8,stroke:#4a90e2
    style Storage fill:#e8f4f8,stroke:#4a90e2
    style Monitoring fill:#e8f4f8,stroke:#4a90e2
```

### Key Relationships

1. **Users → StreamSpace**:
   - Developers access containerized applications via web browser (VNC over WebSocket)
   - Org Admins manage organizational resources, users, and policies via Web UI
   - System Admins deploy platform, monitor health, configure settings

2. **StreamSpace → External Systems**:
   - **SSO Integration**: Delegates authentication to enterprise identity providers
   - **Container Registry**: Pulls application images for session provisioning
   - **Object Storage**: Persists user home directories and session data
   - **Monitoring**: Exports metrics and logs for observability

---

## Level 2: Container Diagram

Shows the major containers (applications/services) within StreamSpace and their interactions.

```mermaid
graph TB
    subgraph Users
        Browser[Web Browser<br/>React SPA]
        CLI[CLI/API Client<br/>REST/WebSocket]
    end

    subgraph Control Plane
        UI[Web UI<br/>React + TypeScript<br/>Port 3000]
        API[API Server<br/>Go + Gin<br/>Port 8000]
        Database[(PostgreSQL<br/>Sessions, Users, Orgs<br/>Templates, Audit Logs)]
        Cache[(Redis<br/>Session cache<br/>Agent routing<br/>Optional)]
    end

    subgraph Execution Layer
        K8sAgent[K8s Agent<br/>Go<br/>Manages K8s sessions]
        DockerAgent[Docker Agent<br/>Go<br/>Manages Docker sessions<br/>Future]
    end

    subgraph Runtime
        K8sPod[Session Pod<br/>User container + VNC server<br/>Kubernetes]
        DockerContainer[Session Container<br/>User container + VNC server<br/>Docker]
    end

    subgraph External
        SSO[SSO Provider]
        Registry[Container Registry]
        Storage[Object Storage]
    end

    Browser -->|HTTPS/WSS| UI
    Browser -->|HTTPS/WSS| API
    CLI -->|HTTPS/WSS| API

    UI -->|REST API| API
    API -->|SQL| Database
    API -->|Cache queries| Cache
    API -->|WebSocket commands| K8sAgent
    API -->|WebSocket commands| DockerAgent
    API -->|Authenticate| SSO

    K8sAgent -->|Create/manage| K8sPod
    K8sAgent -->|Status updates| API
    K8sAgent -->|VNC tunnel| API

    DockerAgent -->|Create/manage| DockerContainer
    DockerAgent -->|Status updates| API
    DockerAgent -->|VNC tunnel| API

    K8sPod -->|Pull images| Registry
    K8sPod -->|Mount volumes| Storage

    DockerContainer -->|Pull images| Registry
    DockerContainer -->|Mount volumes| Storage

    style UI fill:#61dafb,stroke:#2e5c8a,color:#000
    style API fill:#00add8,stroke:#2e5c8a,color:#fff
    style Database fill:#336791,stroke:#2e5c8a,color:#fff
    style Cache fill:#dc382d,stroke:#2e5c8a,color:#fff
    style K8sAgent fill:#326ce5,stroke:#2e5c8a,color:#fff
    style DockerAgent fill:#2496ed,stroke:#2e5c8a,color:#fff
```

### Container Descriptions

#### Control Plane Containers

1. **Web UI** (React + TypeScript)
   - **Technology**: React 18, Material-UI, TypeScript
   - **Port**: 3000 (development), served via API in production
   - **Purpose**: User interface for session management, org admin, system settings
   - **Communication**: REST API (sessions, templates, users), WebSocket (real-time updates)

2. **API Server** (Go + Gin)
   - **Technology**: Go 1.21+, Gin web framework
   - **Port**: 8000
   - **Purpose**: Central control plane - authentication, authorization, session lifecycle, VNC proxy
   - **Communication**:
     - Inbound: HTTPS/REST (UI, CLI), WebSocket (agents, VNC clients)
     - Outbound: PostgreSQL (state), Redis (cache), SSO (auth), agents (commands)

3. **PostgreSQL Database**
   - **Technology**: PostgreSQL 14+
   - **Purpose**: Canonical source of truth (see ADR-006)
   - **Schema**: Sessions, Users, Organizations, Templates, APIKeys, AuditLogs, AgentCommands
   - **Backup**: Daily snapshots, WAL archiving

4. **Redis Cache** (Optional)
   - **Technology**: Redis 7+
   - **Purpose**:
     - Session data cache (reduce DB load)
     - Agent routing (multi-pod API, see ADR-005)
     - Rate limiting counters
   - **Persistence**: Optional (cache can fail open)

#### Execution Layer Containers

5. **Kubernetes Agent** (Go)
   - **Technology**: Go 1.21+, Kubernetes client-go
   - **Purpose**: Provisions and manages sessions on Kubernetes clusters
   - **Communication**:
     - Outbound WebSocket to API (commands, status updates)
     - K8s API (create CRDs, pods, services)
     - Port-forward to session pods (VNC tunnel)

6. **Docker Agent** (Go) - Future (v2.1+)
   - **Technology**: Go 1.21+, Docker SDK
   - **Purpose**: Provisions and manages sessions on Docker hosts
   - **Communication**:
     - Outbound WebSocket to API
     - Docker daemon (create containers, networks)

#### Runtime Containers

7. **Session Pod/Container**
   - **Technology**: User-defined application + VNC server (TigerVNC, x11vnc)
   - **Purpose**: Runs user's containerized application with GUI access
   - **Networking**: VNC server on port 5900 (internal), tunneled via agent → API → browser

---

## Level 3: Component Diagram (API Server)

Shows the internal components of the API Server container.

```mermaid
graph TB
    subgraph API Server
        subgraph HTTP Layer
            Router[Gin Router<br/>Route handlers]
            Middleware[Middleware<br/>Auth, CORS, Rate Limit]
            WSUpgrade[WebSocket Upgrader<br/>Protocol switch]
        end

        subgraph Handlers
            SessionHandler[Session Handler<br/>CRUD operations]
            TemplateHandler[Template Handler<br/>Catalog management]
            UserHandler[User/Org Handler<br/>Identity management]
            VNCHandler[VNC Proxy Handler<br/>VNC token & tunnel]
            AdminHandler[Admin Handler<br/>API keys, audit, settings]
        end

        subgraph Services
            CommandDispatcher[Command Dispatcher<br/>Send commands to agents]
            EventPublisher[Event Publisher<br/>Audit events stub]
            SyncService[Sync Service<br/>Template sync K8s↔DB]
            QuotaEnforcer[Quota Enforcer<br/>Resource limits]
        end

        subgraph WebSocket Layer
            AgentHub[Agent Hub<br/>Track agent connections<br/>Route commands]
            VNCProxy[VNC Proxy<br/>Tunnel VNC streams]
            WSManager[WebSocket Manager<br/>Real-time UI updates]
        end

        subgraph Data Access
            Database[Database Client<br/>PostgreSQL queries]
            Cache[Cache Client<br/>Redis operations]
            K8sClient[K8s Client<br/>Optional, template sync]
        end
    end

    Router --> Middleware
    Middleware --> SessionHandler
    Middleware --> TemplateHandler
    Middleware --> UserHandler
    Middleware --> VNCHandler
    Middleware --> AdminHandler

    SessionHandler --> CommandDispatcher
    SessionHandler --> Database
    SessionHandler --> QuotaEnforcer

    TemplateHandler --> SyncService
    TemplateHandler --> Database

    UserHandler --> Database
    AdminHandler --> Database

    VNCHandler --> VNCProxy
    VNCHandler --> Database

    CommandDispatcher --> AgentHub
    CommandDispatcher --> Database

    SyncService --> K8sClient
    SyncService --> Database

    AgentHub --> Database
    AgentHub --> Cache

    WSManager --> Cache

    EventPublisher -.->|Stub| Database

    style Router fill:#00add8,stroke:#2e5c8a,color:#fff
    style AgentHub fill:#4a90e2,stroke:#2e5c8a,color:#fff
    style VNCProxy fill:#e85d75,stroke:#2e5c8a,color:#fff
    style CommandDispatcher fill:#50c878,stroke:#2e5c8a,color:#fff
    style Database fill:#336791,stroke:#2e5c8a,color:#fff
```

### Component Descriptions

#### HTTP Layer

1. **Gin Router**
   - Routes: `/api/v1/sessions`, `/api/v1/templates`, `/api/v1/users`, `/api/v1/admin`
   - WebSocket routes: `/ws/agent`, `/ws/vnc`, `/ws/ui`

2. **Middleware**
   - **Auth Middleware**: JWT validation, org context extraction (see ADR-004)
   - **CORS**: Cross-origin configuration for UI
   - **Rate Limiting**: Per-user, per-org, per-IP limits
   - **Logging**: Structured logging with request ID correlation

3. **WebSocket Upgrader**
   - HTTP → WebSocket protocol upgrade
   - Connection validation, origin checks

#### Handlers (REST API)

4. **Session Handler** (`api/internal/handlers/sessions.go`)
   - `POST /api/v1/sessions` - Create session (validate quota → dispatch command)
   - `GET /api/v1/sessions` - List sessions (org-scoped, see ADR-004)
   - `GET /api/v1/sessions/:id` - Get session details
   - `DELETE /api/v1/sessions/:id` - Stop session (dispatch stop command)
   - `POST /api/v1/sessions/:id/hibernate` - Hibernate session
   - `POST /api/v1/sessions/:id/resume` - Resume hibernated session

5. **Template Handler** (`api/internal/handlers/sessiontemplates.go`)
   - `GET /api/v1/templates` - List templates (org-scoped)
   - `POST /api/v1/templates` - Create template
   - `PUT /api/v1/templates/:id` - Update template
   - `DELETE /api/v1/templates/:id` - Delete template

6. **User/Org Handler** (`api/internal/handlers/users.go`, `organizations.go`)
   - User CRUD, org management, RBAC assignment

7. **VNC Handler** (`api/internal/handlers/vnc_proxy.go`)
   - `GET /api/v1/sessions/:id/vnc` - Generate VNC token (JWT)
   - `WebSocket /ws/vnc` - VNC proxy endpoint (see ADR-008)

8. **Admin Handler** (`api/internal/handlers/apikeys.go`, `audit.go`, `configuration.go`)
   - API key management, audit log queries, system settings

#### Services

9. **Command Dispatcher** (`api/internal/services/command_dispatcher.go`)
   - Creates commands in `agent_commands` table
   - Sends commands to agents via AgentHub
   - Handles command retry on agent reconnect
   - See ADR-005 (WebSocket Command Dispatch)

10. **Event Publisher** (`api/internal/events/stub.go`)
    - Stub implementation (NATS removed, see ADR-005)
    - Audit events written directly to database

11. **Sync Service** (`api/internal/services/sync_service.go`)
    - Syncs templates from K8s CRDs to database (one-time import)
    - Optional reconciliation loop (future)

12. **Quota Enforcer** (`api/internal/services/quota_enforcer.go`)
    - Validates session creation against org quotas
    - Resource limits (max sessions, CPU, memory)

#### WebSocket Layer

13. **Agent Hub** (`api/internal/websocket/agent_hub.go`)
    - Tracks active agent WebSocket connections (`agent_id → WebSocket`)
    - Routes commands to specific agents
    - Handles agent registration, heartbeat, disconnection
    - Multi-pod support via Redis (Issue #211)

14. **VNC Proxy** (`api/internal/handlers/vnc_proxy.go`)
    - Validates VNC tokens (JWT)
    - Proxies VNC stream: User ↔ API ↔ Agent ↔ Session
    - See ADR-008 (VNC Proxy via Control Plane)

15. **WebSocket Manager** (`api/internal/websocket/manager.go`)
    - Real-time updates to UI clients
    - Session state changes, metrics updates
    - Org-scoped broadcasts (see ADR-004 multi-tenancy fix)

#### Data Access

16. **Database Client** (`api/internal/db/`)
    - PostgreSQL queries via pgx driver
    - Org-scoped queries (WHERE org_id = $1)
    - Connection pooling, prepared statements

17. **Cache Client** (`api/internal/cache/`)
    - Redis operations (GET, SET, HGETALL, PUBLISH/SUBSCRIBE)
    - Agent routing, session cache, rate limiting

18. **K8s Client** (optional)
    - Used for template sync only
    - Can be nil (see ADR-006: Database as Source of Truth)

---

## Level 4: Code Diagram (Session Creation Flow)

Detailed sequence diagram for session creation (most critical flow).

```mermaid
sequenceDiagram
    participant User as User (Browser)
    participant UI as Web UI
    participant API as API Server
    participant Auth as Auth Middleware
    participant SessionHandler as Session Handler
    participant QuotaEnforcer as Quota Enforcer
    participant DB as PostgreSQL
    participant CommandDispatcher as Command Dispatcher
    participant AgentHub as Agent Hub
    participant Agent as K8s Agent
    participant K8s as Kubernetes API
    participant Pod as Session Pod

    User->>UI: Click "Create Session"
    UI->>API: POST /api/v1/sessions<br/>{template_id, resources}

    API->>Auth: Validate JWT token
    Auth->>Auth: Extract user_id, org_id
    Auth->>API: Context{user_id, org_id, role}

    API->>SessionHandler: CreateSession(context, request)
    SessionHandler->>QuotaEnforcer: CheckQuota(org_id, user_id)
    QuotaEnforcer->>DB: SELECT COUNT(*) FROM sessions<br/>WHERE org_id=$1 AND status='running'
    DB-->>QuotaEnforcer: count
    QuotaEnforcer-->>SessionHandler: ✓ Under quota

    SessionHandler->>DB: INSERT INTO sessions<br/>(session_id, user_id, org_id, template_id, status='pending')
    DB-->>SessionHandler: session_id

    SessionHandler->>CommandDispatcher: DispatchCommand('start_session', session_id, template_id)
    CommandDispatcher->>DB: INSERT INTO agent_commands<br/>(command_type='start_session', status='pending')
    CommandDispatcher->>AgentHub: SendCommand(agent_id, command)
    AgentHub->>Agent: WebSocket: {type: 'start_session', session_id, template}

    SessionHandler-->>API: {session_id, status: 'pending'}
    API-->>UI: 201 Created {session_id}
    UI-->>User: "Session creating..."

    Agent->>K8s: Create Session CRD
    K8s-->>Agent: CRD created
    Agent->>K8s: Create Pod<br/>(image, resources, VNC server)
    K8s-->>Agent: Pod scheduled
    Agent->>K8s: Watch Pod status
    K8s-->>Agent: Pod running

    Agent->>API: WebSocket: StatusUpdate<br/>{session_id, status='running', pod_name}
    API->>DB: UPDATE sessions SET status='running'
    API->>UI: WebSocket: SessionUpdate<br/>{session_id, status='running'}
    UI-->>User: "Session ready! [Connect]"

    User->>UI: Click "Connect"
    UI->>API: GET /api/v1/sessions/:id/vnc
    API->>API: Generate VNC token (JWT)<br/>{session_id, user_id, exp: 1h}
    API-->>UI: {vnc_url: "wss://api/ws/vnc?token=..."}
    UI->>API: WebSocket /ws/vnc?token=...
    API->>Agent: WebSocket: CreateVNCTunnel<br/>{session_id}
    Agent->>Pod: Port-forward :5900
    Agent-->>API: VNC stream (binary)
    API-->>UI: VNC stream (binary)
    UI-->>User: Display VNC session
```

### Key Observations

1. **Asynchronous Flow**: Session creation returns immediately (201 Created), actual provisioning happens asynchronously
2. **Org-Scoped Security**: Auth middleware extracts `org_id` from JWT, enforced in all DB queries
3. **Command Persistence**: Commands stored in database for retry on agent reconnect
4. **Real-Time Updates**: WebSocket pushes session status changes to UI
5. **VNC Token Security**: Short-lived JWT (1 hour expiry) for VNC access

---

## Component Diagram (Kubernetes Agent)

```mermaid
graph TB
    subgraph K8s Agent
        subgraph Connection Layer
            WSClient[WebSocket Client<br/>Connect to API]
            Heartbeat[Heartbeat Manager<br/>10s interval]
            Reconnect[Reconnect Handler<br/>Exponential backoff]
        end

        subgraph Command Handlers
            StartSession[Start Session Handler<br/>Provision pod]
            StopSession[Stop Session Handler<br/>Delete resources]
            Hibernate[Hibernate Handler<br/>Pause container]
            Resume[Resume Handler<br/>Unpause container]
            VNCTunnel[VNC Tunnel Handler<br/>Port-forward :5900]
        end

        subgraph K8s Operations
            CRDManager[CRD Manager<br/>Create Session CRDs]
            PodManager[Pod Manager<br/>Create/delete pods]
            ServiceManager[Service Manager<br/>Create K8s services]
            VolumeManager[Volume Manager<br/>PVC for home dirs]
        end

        subgraph Monitoring
            StatusWatcher[Status Watcher<br/>Watch pod events]
            ResourceMonitor[Resource Monitor<br/>Track CPU/memory]
        end
    end

    WSClient --> StartSession
    WSClient --> StopSession
    WSClient --> Hibernate
    WSClient --> Resume
    WSClient --> VNCTunnel

    StartSession --> CRDManager
    StartSession --> PodManager
    StartSession --> ServiceManager
    StartSession --> VolumeManager

    StopSession --> CRDManager
    StopSession --> PodManager

    Hibernate --> PodManager
    Resume --> PodManager

    VNCTunnel --> PodManager

    StatusWatcher --> WSClient
    ResourceMonitor --> WSClient

    Heartbeat --> WSClient

    style WSClient fill:#326ce5,stroke:#2e5c8a,color:#fff
    style StartSession fill:#50c878,stroke:#2e5c8a,color:#fff
    style CRDManager fill:#4a90e2,stroke:#2e5c8a,color:#fff
```

---

## Deployment View

Shows physical deployment topology for production.

```mermaid
graph TB
    subgraph Internet
        Users[Users<br/>HTTPS/WSS]
    end

    subgraph Load Balancer
        LB[AWS ALB / GCP LB<br/>TLS termination<br/>Sticky sessions]
    end

    subgraph Kubernetes Cluster
        subgraph Control Plane Namespace
            API1[API Pod 1<br/>8000]
            API2[API Pod 2<br/>8000]
            API3[API Pod 3<br/>8000]

            UI1[UI Pod 1<br/>3000]
            UI2[UI Pod 2<br/>3000]
        end

        subgraph Data Namespace
            PG[(PostgreSQL<br/>Replicated<br/>Primary + Standby)]
            Redis[(Redis Cluster<br/>3 masters + 3 replicas)]
        end

        subgraph Agent Namespace
            K8sAgent1[K8s Agent Pod 1]
            K8sAgent2[K8s Agent Pod 2]
        end

        subgraph Sessions Namespace
            SessionPod1[Session Pod 1<br/>User: alice]
            SessionPod2[Session Pod 2<br/>User: bob]
            SessionPodN[Session Pod N<br/>User: ...]
        end
    end

    subgraph External Services
        S3[S3 / NFS<br/>Home directories]
        SSO[Okta / Auth0<br/>SSO]
        Prometheus[Prometheus<br/>Metrics]
    end

    Users --> LB
    LB --> API1
    LB --> API2
    LB --> API3
    LB --> UI1
    LB --> UI2

    API1 --> PG
    API2 --> PG
    API3 --> PG

    API1 --> Redis
    API2 --> Redis
    API3 --> Redis

    API1 --> SSO

    K8sAgent1 -.Outbound WebSocket.-> API1
    K8sAgent2 -.Outbound WebSocket.-> API2

    K8sAgent1 --> SessionPod1
    K8sAgent1 --> SessionPod2
    K8sAgent2 --> SessionPodN

    SessionPod1 --> S3
    SessionPod2 --> S3

    API1 --> Prometheus

    style LB fill:#ff9900,stroke:#2e5c8a,color:#fff
    style PG fill:#336791,stroke:#2e5c8a,color:#fff
    style Redis fill:#dc382d,stroke:#2e5c8a,color:#fff
    style K8sAgent1 fill:#326ce5,stroke:#2e5c8a,color:#fff
```

### Deployment Characteristics

1. **High Availability**:
   - API: 3+ pods with horizontal autoscaling
   - PostgreSQL: Primary + synchronous standby
   - Redis: Cluster mode (3 masters, 3 replicas)

2. **Network Isolation**:
   - Control Plane namespace: Public-facing services
   - Sessions namespace: Isolated user workloads
   - Agent namespace: Management plane

3. **Persistence**:
   - Database: Persistent volumes (SSD, replicated)
   - Session storage: NFS/S3 (shared across pods)

4. **Scalability**:
   - Agents connect to any API pod (sticky sessions for VNC)
   - Redis-backed AgentHub routes commands across pods

---

## Diagram Maintenance

### Update Triggers

Update these diagrams when:
1. New major component added (e.g., Docker Agent, Plugin System)
2. Communication patterns change (e.g., new WebSocket protocol)
3. External integrations added (e.g., Vault for secrets)
4. Deployment topology changes (e.g., multi-cluster support)

### Ownership

- **Level 1 (Context)**: Architect (Agent 1)
- **Level 2 (Containers)**: Architect + Builder (Agent 2)
- **Level 3 (Components)**: Builder (Agent 2)
- **Level 4 (Code)**: Builder (Agent 2)
- **Deployment View**: Architect + SRE

### Review Cadence

- **Major releases** (v2.0, v3.0): Full review
- **Minor releases** (v2.1, v2.2): Update as needed
- **Quarterly**: Validate accuracy against implementation

---

## References

- **C4 Model**: https://c4model.com/
- **Mermaid Syntax**: https://mermaid.js.org/
- **Related ADRs**:
  - ADR-005: WebSocket Command Dispatch
  - ADR-006: Database as Source of Truth
  - ADR-007: Agent Outbound WebSocket
  - ADR-008: VNC Proxy via Control Plane
- **Implementation**:
  - `api/internal/` - API server components
  - `agents/k8s-agent/` - Kubernetes agent
  - `ui/src/` - Web UI

---

**Version History**:
- **v1.0** (2025-11-26): Initial C4 diagrams for v2.0-beta
- **Next Review**: v2.1 release (Q1 2026)
