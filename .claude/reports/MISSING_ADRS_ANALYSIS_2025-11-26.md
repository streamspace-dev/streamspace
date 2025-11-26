# Missing Architecture Decision Records (ADRs) Analysis

**Date:** 2025-11-26
**Analyst:** Agent 1 (Architect)
**Status:** Comprehensive analysis complete

---

## Executive Summary

After analyzing the StreamSpace v2.0-beta codebase and design documentation, I've identified **11 architectural decisions** that have been implemented or proposed but **lack formal ADR documentation**. These decisions represent significant architectural choices that should be documented for future reference.

**Current ADR Status:**
- ✅ **3 ADRs exist** (all marked "Proposed", need status updates)
- ⚠️ **11 missing ADRs identified** (high-impact decisions undocumented)
- 🔴 **Priority:** 6 high-priority ADRs for v2.0-beta.1
- 🟡 **Priority:** 5 medium-priority ADRs for v2.1+

---

## Current ADRs (Status Update Needed)

### ADR-001: VNC Token Authentication ✅ Implemented

**Current Status:** Proposed
**Actual Status:** ✅ **ACCEPTED** (implemented in v2.0-beta)

**Evidence:**
- File: `api/internal/handlers/vnc_proxy.go`
- VNC token validation implemented
- Token format: JWT with session_id claim
- Expiry: Configurable (default: 1 hour)

**Action Required:**
- Update ADR-001 status: Proposed → **Accepted**
- Add implementation date: 2025-11-21
- Update owner: Agent 2 (Builder)

---

### ADR-002: Cache Layer for Control Plane ✅ Partially Implemented

**Current Status:** Proposed
**Actual Status:** ✅ **ACCEPTED** (Redis cache infrastructure exists, needs strategy implementation)

**Evidence:**
- File: `api/internal/cache/cache.go`
- Redis cache implemented with fail-open behavior
- Cache enabled via `CACHE_ENABLED` env var
- Missing: Standardized keys/TTLs, invalidation hooks (Issue #214)

**Action Required:**
- Update ADR-002 status: Proposed → **Accepted**
- Add implementation date: 2025-11-20
- Add note: Full strategy implementation in Issue #214 (v2.0-beta.2)
- Update owner: Agent 2 (Builder)

---

### ADR-003: Agent Heartbeat Contract 🟡 In Progress

**Current Status:** Proposed
**Actual Status:** 🟡 **IN PROGRESS** (basic heartbeat exists, needs formalization)

**Evidence:**
- File: `api/internal/websocket/agent_hub.go`
- Heartbeat mechanism implemented (30s interval)
- Missing: Formal schema, protocol_version, capacity reporting, status transitions

**Action Required:**
- Update ADR-003 status: Proposed → **In Progress**
- Add implementation timeline: Issue #215 (v2.0-beta.2)
- Update owner: Agent 2 (Builder) + Agent 3 (Validator)

---

## Missing ADRs - High Priority (v2.0-beta.1)

These decisions have been implemented or are critical for v2.0-beta.1 release but lack formal ADR documentation.

### ADR-004: Multi-Tenancy via Org-Scoped RBAC 🚨 CRITICAL

**Status:** ⚠️ **URGENT - Being Implemented (Issue #212, #211)**

**Decision Required:** How to enforce organization-level isolation and access control

**Context:**
- v2.0-beta is single-tenant (all users share "streamspace" namespace)
- WebSocket broadcasts leak data across orgs (hardcoded namespace)
- JWT claims lack org_id field
- Handlers cannot enforce org-scoped access

**Proposed Decision:**
1. **JWT Claims:** Add org_id to JWT claims (required field)
2. **Middleware:** Extract org_id into request context
3. **Database Queries:** All queries include org_id filter: `WHERE org_id = $1`
4. **WebSocket Scoping:** Broadcasts filtered by subscriber's org_id
5. **Namespace Mapping:** Org-specific K8s namespace (org-{org_id} or custom mapping)

**Alternatives Considered:**
- **Option A:** Single-tenant (current state) - ❌ Not scalable, no isolation
- **Option B:** Org-scoped RBAC (proposed) - ✅ Recommended
- **Option C:** Fine-grained resource-level ACLs - ❌ Too complex for v2.0

**Consequences:**
- ✅ Pro: Enables true multi-tenancy
- ✅ Pro: Prevents cross-org data leakage
- ✅ Pro: Scales to enterprise deployments
- ⚠️ Con: Breaking change (JWT format change)
- ⚠️ Con: Migration required for existing users

**Implementation:**
- Issue #212 (P0): Org context & RBAC plumbing
- Issue #211 (P0): WebSocket org scoping
- Timeline: Wave 27 (2025-11-26 → 2025-11-28)

**References:**
- Design doc: `03-system-design/authz-and-rbac.md`
- Code: `api/internal/auth/jwt.go`, `api/internal/middleware/auth.go`
- Security risk: `09-risk-and-governance/code-observations.md`

**Action Required:**
- ✅ Create ADR-004 with above content
- Link to issues #211, #212
- Status: **In Progress** (implementation underway)
- Owner: Agent 2 (Builder)
- Target: v2.0-beta.1

---

### ADR-005: WebSocket Command Dispatch vs NATS Event Bus 🔴 IMPLEMENTED

**Status:** ✅ **IMPLEMENTED** (needs formal ADR)

**Decision:** Replace NATS message broker with direct WebSocket command dispatch

**Context:**
- v1.x used NATS for agent communication (pub/sub model)
- v2.0-beta replaced NATS with direct WebSocket connections
- Agents maintain persistent WebSocket connection to Control Plane
- Commands sent via WebSocket, not NATS topics

**Decision:**
- **Agent Communication:** Direct WebSocket connection (agent → control plane)
- **Command Dispatch:** Control Plane sends commands via WebSocket (CommandDispatcher)
- **No Message Broker:** NATS removed entirely (event publisher is now stub)
- **Command Queue:** Database-backed command queue (agent_commands table)
- **Retry Logic:** Control Plane retries commands if agent offline

**Evidence:**
- File: `api/internal/events/stub.go` - "NATS removed - event publishing is now a no-op"
- File: `api/internal/services/command_dispatcher.go` - WebSocket command dispatch
- File: `agents/k8s-agent/main.go` - Outbound WebSocket connection
- File: `agents/docker-agent/main.go` - Outbound WebSocket connection

**Alternatives Considered:**
- **Option A:** Keep NATS (v1.x) - ❌ Added complexity, extra infrastructure
- **Option B:** WebSocket + CommandDispatcher (v2.0) - ✅ Chosen
- **Option C:** gRPC streaming - ❌ More complex than WebSocket
- **Option D:** HTTP long-polling - ❌ Less efficient than WebSocket

**Rationale:**
- ✅ Simplicity: No external message broker to manage
- ✅ Firewall-friendly: Outbound WebSocket from agent (agents behind NAT work)
- ✅ Real-time: Persistent connection enables instant command delivery
- ✅ Resilience: Database-backed command queue survives agent restarts
- ✅ Observability: Centralized command tracking in agent_commands table
- ⚠️ Con: Control Plane must track agent connections (AgentHub)
- ⚠️ Con: Multi-pod API requires Redis for agent routing (Issue #211)

**Consequences:**
- **Deployment:** No NATS cluster required (reduced ops complexity)
- **Agent Architecture:** Agents are stateless, reconnect on restart
- **Scalability:** Control Plane must scale to handle agent WebSocket connections
- **Multi-Pod API:** Requires Redis-backed AgentHub for pod-to-pod routing
- **Command Reliability:** Database ensures commands survive agent downtime

**Implementation Timeline:**
- v2.0-alpha: NATS removed, WebSocket implemented
- v2.0-beta: CommandDispatcher + agent_commands table
- v2.0-beta.1: Multi-pod support via Redis AgentHub (Wave 17)

**References:**
- File: `api/internal/services/command_dispatcher.go`
- File: `api/internal/websocket/agent_hub.go`
- Design doc: `03-system-design/control-plane.md`

**Action Required:**
- ✅ Create ADR-005 documenting this decision
- Status: **Accepted** (already implemented)
- Date: 2025-11-20
- Owner: Agent 2 (Builder)

---

### ADR-006: Database as Source of Truth (No K8s CRD Reconciliation) 🔴 IMPLEMENTED

**Status:** ✅ **IMPLEMENTED** (needs formal ADR)

**Decision:** Use PostgreSQL as source of truth; minimize K8s client usage in API

**Context:**
- v1.x had tight coupling between API and K8s (direct CRD manipulation)
- v2.0-beta uses database as source of truth
- K8s CRDs exist but API rarely reads from K8s
- Agents create/manage K8s resources, sync status back to DB

**Decision:**
- **Database:** PostgreSQL is canonical source of truth
- **K8s CRDs:** Created by agents, not API (except initial template sync)
- **API Reads:** Database-only (no `kubectl get` in hot path)
- **Status Updates:** Agents update database via WebSocket commands
- **K8s Client:** Optional in API (can run without K8s access)

**Evidence:**
- File: `api/cmd/main.go:105` - Comment: "k8sClient is OPTIONAL (last parameter) - can be nil for standalone API"
- File: `api/internal/api/handlers.go` - All reads from database, not K8s
- File: `agents/k8s-agent/main.go` - Agent creates K8s resources (Sessions, CRDs)
- Database schema: `sessions`, `templates`, `agents` tables

**Alternatives Considered:**
- **Option A:** K8s as source of truth (v1.x) - ❌ Tight coupling, hard to multi-platform
- **Option B:** Database as source of truth (v2.0) - ✅ Chosen
- **Option C:** Dual source of truth (DB + K8s) - ❌ Eventual consistency issues
- **Option D:** Event sourcing - ❌ Over-engineered for v2.0

**Rationale:**
- ✅ Multi-Platform: Database works for K8s and Docker agents
- ✅ Decoupling: API doesn't need K8s RBAC (simpler deployment)
- ✅ Performance: Database reads faster than K8s API calls
- ✅ Reliability: Database handles more concurrent reads than K8s API
- ✅ Observability: Centralized audit log and query capabilities
- ⚠️ Con: Agents must sync status back to DB (eventual consistency)
- ⚠️ Con: K8s CRDs become "projections" of DB state (not canonical)

**Consequences:**
- **API Deployment:** Can run without K8s client (Docker, bare metal)
- **Template Sync:** Initial template import from K8s CRDs (one-time)
- **Session Management:** Database tracks state, agents execute
- **Testing:** Easier to test API without K8s cluster
- **Migration Path:** Easier to support non-K8s platforms

**Open Questions:**
- Should we remove K8s client from API entirely? (Future ADR)
- How to handle CRD schema changes? (Migration strategy)

**References:**
- File: `api/cmd/main.go`
- Design doc: `03-system-design/control-plane.md`
- Code comments: "v2.0-beta: agentHub enables multi-agent routing, k8sClient is OPTIONAL"

**Action Required:**
- ✅ Create ADR-006 documenting this decision
- Status: **Accepted** (already implemented)
- Date: 2025-11-20
- Owner: Agent 2 (Builder)

---

### ADR-007: Agent Outbound WebSocket (Firewall-Friendly) 🔴 IMPLEMENTED

**Status:** ✅ **IMPLEMENTED** (needs formal ADR)

**Decision:** Agents initiate outbound WebSocket connections to Control Plane (not inbound)

**Context:**
- v1.x agents required inbound connectivity (K8s Service, LoadBalancer)
- Enterprise deployments often block inbound connections to agents
- Agents behind NAT/firewalls couldn't connect

**Decision:**
- **Connection Direction:** Agent → Control Plane (outbound from agent)
- **Authentication:** Agents authenticate via shared secret or mTLS
- **Persistent Connection:** Agent maintains persistent WebSocket
- **Reconnection:** Agents automatically reconnect on disconnect
- **Command Delivery:** Control Plane pushes commands via WebSocket

**Evidence:**
- File: `agents/k8s-agent/main.go:120` - `websocket.DefaultDialer.Dial(wsURL, nil)`
- File: `agents/docker-agent/main.go:150` - `websocket.DefaultDialer.Dial(wsURL, nil)`
- File: `api/internal/websocket/agent_hub.go` - Accepts incoming WebSocket connections
- Config: `CONTROL_PLANE_URL` env var (agents connect to API, not vice versa)

**Alternatives Considered:**
- **Option A:** Inbound to agents (v1.x) - ❌ NAT/firewall issues
- **Option B:** Outbound from agents (v2.0) - ✅ Chosen
- **Option C:** Bidirectional (mesh) - ❌ Complex topology
- **Option D:** Polling (agents poll API) - ❌ High latency, inefficient

**Rationale:**
- ✅ Firewall-Friendly: Outbound connections work through NAT/firewalls
- ✅ Enterprise-Ready: Agents behind corporate firewall can connect
- ✅ Edge Deployment: Agents in edge locations (VPC, on-prem) can connect
- ✅ Security: Control Plane only exposes HTTPS/WSS (no agent-specific ports)
- ✅ Simplicity: Single ingress point for all agents (no per-agent LoadBalancer)
- ⚠️ Con: Control Plane must accept many WebSocket connections (scalability)

**Consequences:**
- **Deployment:** Agents only need outbound HTTPS/WSS (port 443) access
- **Security:** Agents authenticate to Control Plane (not vice versa)
- **Load Balancing:** Control Plane horizontally scalable (stateless API)
- **Reconnection:** Agents handle reconnection logic (exponential backoff)
- **Multi-Pod API:** Requires Redis AgentHub for agent→pod mapping

**Security Considerations:**
- Agent authentication: Shared secret or mTLS
- WebSocket origin validation
- Rate limiting on WebSocket connections
- Connection timeout and idle detection

**References:**
- File: `agents/k8s-agent/main.go`
- File: `agents/docker-agent/main.go`
- File: `api/internal/websocket/agent_hub.go`
- Design doc: `03-system-design/agents.md`

**Action Required:**
- ✅ Create ADR-007 documenting this decision
- Status: **Accepted** (already implemented)
- Date: 2025-11-18
- Owner: Agent 2 (Builder)

---

### ADR-008: VNC Proxy via Control Plane (No Direct Agent Access) 🔴 IMPLEMENTED

**Status:** ✅ **IMPLEMENTED** (needs formal ADR)

**Decision:** VNC connections proxy through Control Plane, not directly to agents

**Context:**
- v1.x users connected directly to session VNC ports (K8s Service per session)
- Direct access required exposing agent network to users
- Enterprise deployments want centralized access control

**Decision:**
- **VNC Proxy:** Control Plane acts as VNC WebSocket proxy
- **User Flow:** User → Control Plane VNC endpoint → Agent VNC tunnel → Session Pod
- **Authentication:** VNC tokens issued by API, validated by proxy
- **Agent Tunnel:** Agent creates K8s port-forward tunnel to session pod
- **Binary Proxy:** Control Plane proxies binary VNC stream (no parsing)

**Evidence:**
- File: `api/internal/handlers/vnc_proxy.go` - VNC WebSocket proxy handler
- File: `api/internal/websocket/agent_hub.go` - VNC tunnel routing
- File: `agents/k8s-agent/agent_vnc_tunnel.go` - K8s port-forward to pod
- Architecture: User → API VNC proxy → Agent VNC tunnel → Pod :5900

**Alternatives Considered:**
- **Option A:** Direct to agent (v1.x) - ❌ Security issues, network exposure
- **Option B:** Proxy via Control Plane (v2.0) - ✅ Chosen
- **Option C:** Dedicated VNC gateway - ❌ Additional infrastructure
- **Option D:** Agent-to-agent mesh - ❌ Complex, hard to secure

**Rationale:**
- ✅ Security: Centralized auth/authz at Control Plane
- ✅ Firewall-Friendly: Single ingress point for users (no agent exposure)
- ✅ Auditability: All VNC connections logged at Control Plane
- ✅ Multi-Platform: Works for K8s and Docker agents
- ✅ Token Expiry: VNC tokens expire (limited session lifetime)
- ⚠️ Con: Control Plane must proxy VNC bandwidth (scalability concern)
- ⚠️ Con: Extra hop adds latency (~10-20ms)

**Consequences:**
- **Architecture:** 3-hop VNC path: User → Control Plane → Agent → Pod
- **Performance:** Acceptable latency (<50ms typically)
- **Scalability:** Control Plane must handle VNC bandwidth (plan capacity)
- **Security:** VNC tokens prevent unauthorized access (JWT-based)
- **Observability:** VNC connection metrics at Control Plane

**Security:**
- VNC token: JWT with `session_id`, `user_id`, `exp` (1 hour default)
- Token validation: Control Plane validates before proxying
- Per-session tokens: Each session gets unique VNC endpoint
- Token revocation: Expires automatically (no explicit revoke needed)

**References:**
- File: `api/internal/handlers/vnc_proxy.go`
- File: `agents/k8s-agent/agent_vnc_tunnel.go`
- ADR-001: VNC Token Auth (related)
- Design doc: `03-system-design/control-plane.md`

**Action Required:**
- ✅ Create ADR-008 documenting this decision
- Status: **Accepted** (already implemented)
- Date: 2025-11-18
- Owner: Agent 2 (Builder)

---

### ADR-009: Helm Chart Deployment (No Kubernetes Operator) 🟡 PROPOSED

**Status:** 🟡 **PROPOSED** (needs formal ADR)

**Decision:** Deploy via Helm chart; no custom Kubernetes Operator (yet)

**Context:**
- StreamSpace uses K8s CRDs (Session, Template, TemplateRepository, Connection)
- Custom resources typically require custom controllers (Operators)
- v2.0-beta has CRDs but no Operator

**Current State:**
- **CRDs Exist:** `chart/crds/stream.space_*.yaml`
- **No Operator:** No controller watching CRDs
- **Agent Creates CRDs:** K8s agent creates Session CRDs when provisioning
- **API Doesn't Watch CRDs:** API reads from database, not K8s

**Decision (Implicit):**
- **Deployment:** Helm chart only (no Operator)
- **CRD Management:** CRDs are created by agents, not reconciled
- **Why No Operator:**
  - Database is source of truth (not K8s)
  - Agents handle CRD lifecycle
  - No reconciliation loop needed
  - Simpler deployment (fewer moving parts)

**Alternatives Considered:**
- **Option A:** Helm chart + Operator (v1.x approach) - ❌ Extra complexity
- **Option B:** Helm chart only (v2.0) - ✅ Current (implicit)
- **Option C:** Operator-only (no Helm) - ❌ Harder for users

**Open Questions:**
- Should we formalize "no Operator" decision? (ADR needed)
- Future: Operator for advanced reconciliation? (v3.0?)
- CRD lifecycle: Who deletes orphaned CRDs?

**Consequences:**
- ✅ Simpler deployment (Helm chart only)
- ✅ Fewer RBAC permissions needed
- ✅ Easier to understand for users
- ⚠️ Con: CRDs may become stale (no reconciliation)
- ⚠️ Con: Manual cleanup required if agent crashes

**Action Required:**
- ✅ Create ADR-009 documenting decision (no Operator for v2.0)
- Status: **Proposed** (needs review and acceptance)
- Target: v2.0-beta.1 documentation
- Owner: Agent 1 (Architect)

---

## Missing ADRs - Medium Priority (v2.1+)

These decisions can be documented post-v2.0-beta.1 release.

### ADR-010: Plugin System Architecture (Runtime V2) 🟡 PROPOSED

**Status:** 🟡 **IMPLEMENTED** (needs formal ADR)

**Decision:** Plugin system with auto-discovery, database-driven loading, and event bus

**Context:**
- StreamSpace has extensive plugin system (`api/internal/plugins/`)
- Plugins can extend API, UI, scheduler, and events
- RuntimeV2 provides auto-discovery and auto-loading

**Key Design Elements:**
- **Discovery:** Scans filesystem for `.so` plugins + built-in registry
- **Database-Driven:** Loads only enabled plugins from `installed_plugins` table
- **Auto-Start:** Plugins load on API startup (if enabled)
- **Event Bus:** Inter-plugin communication via event broker
- **Registries:** API, UI, Events, Scheduler registries for extensions
- **Lifecycle Hooks:** OnLoad, OnUnload, OnSessionCreated, etc.

**Evidence:**
- File: `api/internal/plugins/runtime_v2.go` (1,000+ lines of plugin orchestration)
- File: `api/internal/plugins/discovery.go` - Plugin discovery
- File: `api/internal/plugins/event_bus.go` - Event-driven architecture
- Database: `installed_plugins`, `catalog_plugins` tables

**Action Required:**
- Create ADR-010 documenting plugin architecture
- Status: **Proposed** (needs review)
- Priority: P1 (for plugin developers)
- Target: v2.1 documentation
- Owner: Agent 2 (Builder) or Architect

---

### ADR-011: API Pagination Strategy 🟡 PROPOSED

**Status:** 🟡 **PROPOSED** (Issue #213)

**Decision:** Standardize pagination across all list endpoints

**Context:**
- Current API returns inconsistent pagination (some use page/size, some use cursors, some return raw arrays)
- Design doc proposes standard envelope: `{items: [...], pagination: {page, page_size, total, cursors}}`

**Proposed Decision:**
- **Envelope:** All list endpoints return `{items, pagination}`
- **Pagination:** Support both offset-based (page/size) and cursor-based
- **Defaults:** page=1, page_size=20, max_page_size=100
- **Cursors:** Optional for efficient pagination of large datasets

**Action Required:**
- Create ADR-011 after implementing Issue #213
- Status: **Proposed** (needs implementation)
- Priority: P1
- Target: v2.0-beta.2
- Owner: Agent 2 (Builder)

---

### ADR-012: Webhook Delivery System 🟡 PROPOSED

**Status:** 🟡 **PROPOSED** (Issue #216)

**Decision:** Webhook delivery with HMAC signing, retries, and idempotency

**Context:**
- Design doc proposes webhook system for lifecycle events
- Events: `session.started`, `session.stopped`, `session.failed`, etc.
- No implementation exists yet

**Proposed Decision:**
- **Delivery:** POST to user-configured URL
- **Security:** HMAC signature (sha256) with shared secret
- **Retries:** Exponential backoff (1s, 5s, 30s, 2m, 10m)
- **Idempotency:** `delivery_id` UUID for duplicate detection
- **Timestamp:** Prevent replay attacks (5-minute window)

**Action Required:**
- Create ADR-012 when implementing Issue #216
- Status: **Proposed** (needs implementation)
- Priority: P1
- Target: v2.0-beta.2 or v2.1
- Owner: Agent 2 (Builder)

---

### ADR-013: Error Handling & Standard Error Envelopes 🟡 PROPOSED

**Status:** 🟡 **PROPOSED** (Issue #213)

**Decision:** Standardize error responses across all API endpoints

**Context:**
- Current API returns various error formats
- Design doc proposes standard envelope: `{code, message, correlation_id}`

**Proposed Decision:**
- **Envelope:** `{code: "INVALID_INPUT", message: "...", correlation_id: "req-123"}`
- **HTTP Status:** Map error codes to HTTP status (400, 403, 404, 409, 500)
- **Codes:** Predefined error codes (INVALID_INPUT, NOT_FOUND, UNAUTHORIZED, etc.)
- **Correlation ID:** Unique ID for request tracing

**Action Required:**
- Create ADR-013 after implementing Issue #213
- Status: **Proposed** (needs implementation)
- Priority: P1
- Target: v2.0-beta.2
- Owner: Agent 2 (Builder)

---

### ADR-014: Session State Machine 🟡 PROPOSED

**Status:** 🟡 **PROPOSED** (needs formalization)

**Decision:** Formalize session state transitions and lifecycle

**Context:**
- Sessions have states: pending, scheduling, running, hibernated, stopping, stopped, failed
- State transitions implicit in code but not formally documented

**Proposed Decision:**
- **States:** Define all valid session states
- **Transitions:** Define valid state transitions (FSM)
- **Triggers:** Define what triggers each transition
- **Validations:** Define invalid transitions (error conditions)

**State Machine:**
```
requested → scheduling → running ⇄ hibernated
                        ↓           ↓
                      stopping → stopped
                        ↓
                      failed
```

**Action Required:**
- Create ADR-014 documenting session state machine
- Status: **Proposed** (needs review)
- Priority: P2
- Target: v2.1 documentation
- Owner: Agent 1 (Architect)

---

## Summary & Recommendations

### Immediate Actions (v2.0-beta.1)

**Priority 1: Update Existing ADRs**
1. ✅ ADR-001: Update status to **Accepted** (VNC token auth implemented)
2. ✅ ADR-002: Update status to **Accepted** (cache infrastructure exists)
3. ✅ ADR-003: Update status to **In Progress** (Issue #215)

**Priority 2: Create Critical ADRs**
4. 🚨 ADR-004: Multi-Tenancy via Org-Scoped RBAC (URGENT - Issue #211, #212)
5. ✅ ADR-005: WebSocket Command Dispatch vs NATS (document v1→v2 change)
6. ✅ ADR-006: Database as Source of Truth (document architecture decision)
7. ✅ ADR-007: Agent Outbound WebSocket (firewall-friendly design)
8. ✅ ADR-008: VNC Proxy via Control Plane (centralized access)
9. 🟡 ADR-009: Helm Chart Deployment (no Operator)

**Estimated Effort:**
- Update 3 existing ADRs: **1 hour** (Architect)
- Create 6 new ADRs: **6-8 hours** (Architect + Builder)
- **Total: 7-9 hours** (can be done in parallel with Wave 27)

### Post-Release (v2.1+)

**Priority 3: Document Implemented Features**
10. ADR-010: Plugin System Architecture (RuntimeV2)
11. ADR-014: Session State Machine

**Priority 4: Document Future Features**
12. ADR-011: API Pagination Strategy (Issue #213)
13. ADR-012: Webhook Delivery System (Issue #216)
14. ADR-013: Error Handling & Envelopes (Issue #213)

---

## Proposed Timeline

### Week of 2025-11-26 (v2.0-beta.1 Sprint)

**Architect (Agent 1):**
- **Day 1:** Create ADR-004 (Multi-Tenancy) - 2 hours
- **Day 1:** Update ADR-001, 002, 003 status - 1 hour
- **Day 2:** Create ADR-005, 006, 007 - 3 hours
- **Day 3:** Create ADR-008, 009 - 2 hours

**Total: 8 hours** (parallelizable with Builder/Validator work)

### Week of 2025-12-02 (v2.0-beta.2 Planning)

**Architect + Builder:**
- Create ADR-010 (Plugin System) - 3 hours
- Create ADR-014 (Session State Machine) - 2 hours
- Defer ADR-011, 012, 013 until features implemented

---

## ADR Template Usage

All ADRs should follow the template in `02-architecture/adr-template.md`:

```markdown
# ADR-NNN: Title
- **Status**: Proposed | Accepted | Rejected | Superseded by ADR-XXX
- **Date**: YYYY-MM-DD
- **Owners**: Name(s)

## Context
[Problem statement and background]

## Decision
[What we decided to do]

## Alternatives Considered
[Other options and why we didn't choose them]

## Consequences
[Impact of this decision - pros and cons]

## References
[Links to code, docs, issues, etc.]
```

---

## Conclusion

**11 architectural decisions** have been identified that need formal ADR documentation:
- **6 high-priority** (v2.0-beta.1) - Critical for understanding v2.0 architecture
- **5 medium-priority** (v2.1+) - Can be documented post-release

**Most Critical:**
- 🚨 **ADR-004** (Multi-Tenancy) - Being implemented NOW (Issue #211, #212)
- ✅ **ADR-005-008** - Already implemented, need documentation for historical record

**Recommendation:** Architect (Agent 1) should create these ADRs during Wave 27 (in parallel with Builder/Validator work) to ensure v2.0-beta.1 has comprehensive architectural documentation.

---

**Status:** ✅ COMPLETE
**Next Action:** Architect to create ADRs (8-hour effort, parallelizable)
