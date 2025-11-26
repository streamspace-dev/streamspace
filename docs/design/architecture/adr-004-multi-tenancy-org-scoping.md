# ADR-004: Multi-Tenancy via Organization-Scoped RBAC
- **Status**: In Progress
- **Date**: 2025-11-26
- **Owners**: Agent 2 (Builder)
- **Implementation**: Issues #211, #212 (Wave 27)

## Context

StreamSpace v2.0-beta currently operates as a single-tenant system where all users share the hardcoded "streamspace" Kubernetes namespace. This architecture has critical security implications:

1. **Cross-Tenant Data Leakage**: WebSocket broadcasts (`api/internal/websocket/handlers.go`) use `ListSessions(ctx, "streamspace")` which returns ALL sessions to ANY connected client without organization filtering.

2. **Missing Org Context**: JWT claims lack `org_id` field, and auth middleware does not populate organization context, preventing handlers from enforcing org-scoped access controls.

3. **Hardcoded Namespace**: Namespace `"streamspace"` is hardcoded throughout API handlers and WebSocket subscriptions, making true multi-tenancy impossible.

4. **No Authorization Guards**: WebSocket handlers lack authorization checks before subscription, allowing users to potentially access other organizations' data.

**Risk Level**: P0 CRITICAL - Blocks v2.0-beta.1 production release

**Discovery**: Identified via design & governance review (2025-11-26)

## Decision

Implement organization-level isolation throughout the system to enable true multi-tenancy:

### 1. JWT Claims Enhancement
- Add `org_id` (string, required) to JWT claims
- Add `org_name` (string, optional) for display purposes
- Maintain backward compatibility during migration period
- Update token generation to include org_id from user's organization record

```go
type CustomClaims struct {
    UserID   string `json:"user_id"`
    OrgID    string `json:"org_id"`     // NEW
    OrgName  string `json:"org_name"`   // NEW (optional)
    Role     string `json:"role"`
    jwt.RegisteredClaims
}
```

### 2. Auth Middleware Enhancement
- Extract `org_id` from validated JWT claims
- Populate request context: `ctx = context.WithValue(ctx, "org_id", orgID)`
- Populate `user_id` in context (if not already done)
- Return 401 Unauthorized if `org_id` missing from valid token
- Log org_id in all audit logs and request logs

```go
// api/internal/middleware/auth.go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // ... validate JWT ...

        orgID := claims.OrgID
        if orgID == "" {
            c.JSON(401, gin.H{"error": "Missing org_id in token"})
            c.Abort()
            return
        }

        ctx := context.WithValue(c.Request.Context(), "org_id", orgID)
        ctx = context.WithValue(ctx, "user_id", claims.UserID)
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

### 3. Database Query Scoping

**All database queries MUST include org_id filter:**

- **Sessions**:
  - List: `WHERE org_id = $1 ORDER BY created_at DESC`
  - Get: `WHERE session_id = $1 AND org_id = $2`
  - Create: `INSERT ... VALUES (..., $org_id)`
  - Update: `WHERE session_id = $1 AND org_id = $2`
  - Delete: `WHERE session_id = $1 AND org_id = $2`

- **Templates**:
  - List: `WHERE org_id = $1 OR is_public = true`
  - Get: `WHERE template_id = $1 AND (org_id = $2 OR is_public = true)`
  - Create: `INSERT ... VALUES (..., $org_id)`
  - Update: `WHERE template_id = $1 AND org_id = $2`
  - Delete: `WHERE template_id = $1 AND org_id = $2`

- **Other Resources**:
  - Webhooks: `WHERE org_id = $1`
  - API Keys: `WHERE org_id = $1 AND user_id = $2`
  - Audit Logs: `WHERE org_id = $1` (admins) or `WHERE org_id = $1 AND user_id = $2` (users)
  - Quotas: `WHERE org_id = $1`
  - Agents: Filter by org's assigned clusters/agents

### 4. WebSocket Broadcast Scoping

**WebSocket handlers MUST filter broadcasts by org_id:**

```go
// api/internal/websocket/handlers.go

// BEFORE (v2.0-beta - INSECURE):
sessions, err := h.sessionService.ListSessions(ctx, "streamspace")
// Broadcasts ALL sessions to ANY client

// AFTER (v2.0-beta.1 - SECURE):
orgID := ctx.Value("org_id").(string)
namespace := h.getNamespaceForOrg(orgID) // org-specific namespace
sessions, err := h.sessionService.ListSessions(ctx, namespace)
// Only sessions for subscriber's org
```

**Authorization guard before subscription:**
```go
func HandleSessionsWebSocket(w http.ResponseWriter, r *http.Request, db *db.Database) {
    // Extract org_id from request context (set by auth middleware)
    orgID := r.Context().Value("org_id").(string)
    if orgID == "" {
        http.Error(w, "Unauthorized", 401)
        return
    }

    // Upgrade WebSocket with org context
    // ... subscribe only to org's data ...
}
```

### 5. Namespace Mapping

**Replace hardcoded `"streamspace"` with org-aware namespace:**

**Option A: Derived namespace** (recommended for v2.0)
```go
func getNamespaceForOrg(orgID string) string {
    return fmt.Sprintf("org-%s", orgID)
}
```

**Option B: Database mapping** (future enhancement)
```sql
SELECT namespace FROM organizations WHERE org_id = $1
```

**Fail-closed behavior:**
- If namespace unknown/unmapped → return error (don't default to "streamspace")
- Log all namespace lookups for audit trail

## Alternatives Considered

### Alternative A: Single-Tenant (Current State) ❌
- **Pros**: Simple, no multi-tenancy complexity
- **Cons**: Not scalable, no isolation, security risk in shared deployments
- **Verdict**: Rejected - Blocks enterprise adoption

### Alternative B: Org-Scoped RBAC (Chosen) ✅
- **Pros**: True multi-tenancy, strong isolation, scalable
- **Cons**: Breaking change (JWT format), requires migration
- **Verdict**: Accepted - Essential for production readiness

### Alternative C: Fine-Grained Resource ACLs ❌
- **Pros**: Maximum flexibility (per-resource permissions)
- **Cons**: Too complex for v2.0, performance overhead, hard to audit
- **Verdict**: Deferred - Consider for v2.1+ if needed

### Alternative D: Separate Deployments per Org ❌
- **Pros**: Complete isolation (infrastructure-level)
- **Cons**: High operational cost, resource waste, complex management
- **Verdict**: Rejected - SaaS model requires multi-tenancy

## Rationale

### Why Organization-Level Scoping?
1. **Security**: Prevents cross-tenant data leakage (critical for compliance)
2. **Scalability**: Single deployment serves multiple organizations
3. **Enterprise-Ready**: Required for SaaS and enterprise deployments
4. **Compliance**: Meets SOC2, GDPR, HIPAA data isolation requirements
5. **Cost-Effective**: Shared infrastructure (vs separate deployments)

### Why JWT Claims?
- **Stateless**: No database lookup on every request
- **Performance**: Context available immediately after auth
- **Auditability**: org_id in JWT logs = complete audit trail
- **Standard**: Follows OAuth2/OIDC best practices

### Why Database Filtering?
- **Defense in Depth**: Even if context lost, query fails safely
- **Explicit**: Intent clear in SQL queries
- **Testable**: Easy to validate with integration tests
- **Performant**: Database indexes on org_id

## Consequences

### Positive Consequences ✅
1. **True Multi-Tenancy**: Multiple organizations can use same deployment
2. **Data Isolation**: Cross-org access impossible (by design)
3. **Scalability**: Horizontal scaling without per-org infrastructure
4. **Compliance**: Meets regulatory data isolation requirements
5. **Cost Reduction**: Shared infrastructure reduces operational costs
6. **Future-Proof**: Enables org-level features (quotas, billing, etc.)

### Negative Consequences ⚠️
1. **Breaking Change**: JWT format changes (migration required)
2. **Migration Complexity**: Existing users need org assignment
3. **Query Complexity**: Every query needs org_id filter
4. **Performance**: Additional JOIN in some queries
5. **Testing**: Requires org context in all integration tests

### Migration Plan

**Phase 1: Org Infrastructure (Pre-v2.0-beta.1)**
1. Create `organizations` table (if not exists)
2. Add `org_id` column to all resource tables (sessions, templates, etc.)
3. Default org: Assign all existing users to default "streamspace" org
4. Backfill: `UPDATE sessions SET org_id = 'default' WHERE org_id IS NULL`

**Phase 2: JWT Changes (v2.0-beta.1 - Wave 27)**
1. Update JWT generation to include org_id
2. Update auth middleware to extract org_id
3. Maintain backward compatibility: Accept tokens without org_id temporarily
4. Issue new tokens with org_id on next login

**Phase 3: Handler Updates (v2.0-beta.1 - Wave 27)**
1. Update all API handlers to filter by org_id
2. Update WebSocket handlers to check org authorization
3. Replace hardcoded "streamspace" with namespace lookup
4. Add integration tests for org isolation

**Phase 4: Enforcement (v2.0-beta.1)**
1. Make org_id required in JWT (reject missing org_id)
2. Remove backward compatibility code
3. Full enforcement of org-scoping across system

### Rollback Plan
- If critical issues found: Temporarily allow tokens without org_id
- If data corruption: Database has org_id = 'default' fallback
- If performance issues: Add database indexes on org_id columns

## Security Considerations

### Threat Model
1. **Threat**: User A accesses User B's sessions (different orgs)
   - **Mitigation**: Database queries filter by org_id
   - **Validation**: Integration tests verify 403 Forbidden

2. **Threat**: WebSocket broadcast leaks org A data to org B
   - **Mitigation**: Broadcast filtering by subscriber's org_id
   - **Validation**: WebSocket integration tests

3. **Threat**: JWT token stolen, used by different org
   - **Mitigation**: org_id bound to user, cannot be changed
   - **Validation**: Token validation includes org membership check

4. **Threat**: SQL injection bypasses org_id filter
   - **Mitigation**: Parameterized queries, input validation (Issue #164)
   - **Validation**: Security testing, code review

### Audit Trail
- All API requests log org_id from JWT
- All database changes log org_id
- Audit log queries filterable by org_id
- Admin actions log source org and target org (if cross-org)

## Performance Considerations

### Database Indexes
```sql
-- Required indexes for performance
CREATE INDEX idx_sessions_org_id ON sessions(org_id);
CREATE INDEX idx_templates_org_id ON templates(org_id);
CREATE INDEX idx_agent_commands_org_id ON agent_commands(org_id);
CREATE INDEX idx_audit_logs_org_id ON audit_logs(org_id);
```

### Query Performance
- Org-scoped queries use indexes: Fast (< 10ms typical)
- List queries: `WHERE org_id = $1` = index scan (not full scan)
- Single-record queries: Add org_id to unique index for optimal performance

### Caching Strategy (Future - Issue #214)
- Cache keys include org_id: `cache:org:<org_id>:sessions:list`
- Invalidation per org (not global)
- TTL per resource type (templates: 60s, sessions: 15s)

## Testing Strategy

### Unit Tests
- JWT generation includes org_id
- Auth middleware extracts org_id into context
- Missing org_id returns 401 Unauthorized

### Integration Tests
1. **Org Isolation**:
   - User A (org-A) creates session
   - User B (org-B) tries to GET session → 403 Forbidden
   - User B tries to DELETE session → 403 Forbidden
   - User B lists sessions → sees only org-B sessions

2. **WebSocket Scoping**:
   - User A connects WebSocket
   - User B connects WebSocket
   - Create session in org-A → Only User A receives broadcast
   - Create session in org-B → Only User B receives broadcast

3. **Template Visibility**:
   - Org-specific template in org-A → Visible to org-A only
   - Public template (is_public=true) → Visible to all orgs
   - Org-B cannot access org-A private templates

4. **Namespace Mapping**:
   - Session created with org-A → Uses namespace `org-<org-a>`
   - Session created with org-B → Uses namespace `org-<org-b>`
   - No hardcoded "streamspace" namespace in queries

### Security Tests
- Penetration testing: Attempt cross-org access via API
- Fuzzing: Invalid org_id values in JWT
- Token tampering: Modified org_id in JWT (should fail signature validation)

## Implementation Timeline

**Wave 27 (2025-11-26 → 2025-11-28):**

**Builder (Agent 2):**
- Day 1: Issue #212 - Org context & RBAC plumbing (1-2 days)
  - Update JWT claims, middleware, all handlers
- Day 2: Issue #211 - WebSocket org scoping (4-8 hours)
  - Filter broadcasts, namespace mapping, auth guards

**Validator (Agent 3):**
- Day 2-3: Validate org isolation (4-6 hours)
  - Integration tests for cross-org access denial
  - WebSocket broadcast filtering tests

**Target**: v2.0-beta.1 release (2025-11-28 or 2025-11-29)

## Open Questions

1. **Namespace Allocation**: How to allocate K8s namespaces per org?
   - **Proposal**: Derive from org_id: `org-<org-id>`
   - **Alternative**: Database mapping table (future enhancement)

2. **Cross-Org Admin Access**: Should super-admins access all orgs?
   - **Proposal**: Super-admin role with cross-org visibility (v2.1)
   - **Current**: Admins scoped to their org only (v2.0)

3. **Org Creation**: How are new orgs provisioned?
   - **Current**: Manual (admin creates org via database)
   - **Future**: Self-service org creation API (v2.1)

4. **Billing Integration**: How does org-scoping relate to billing?
   - **Deferred**: Billing is future feature (v2.x)
   - **Note**: org_id enables per-org usage tracking

## References

- **Issues**:
  - #211: WebSocket org scoping and auth guard (P0)
  - #212: Org context and RBAC plumbing for API and WebSockets (P0)
- **Implementation**:
  - api/internal/auth/jwt.go (JWT claims)
  - api/internal/middleware/auth.go (auth middleware)
  - api/internal/handlers/ (all API handlers)
  - api/internal/websocket/handlers.go (WebSocket broadcasts)
- **Design Docs**:
  - 03-system-design/authz-and-rbac.md (RBAC design)
  - 03-system-design/websocket-hardening.md (WebSocket security)
  - 09-risk-and-governance/code-observations.md (security audit)
- **Security Review**:
  - .claude/reports/DESIGN_GOVERNANCE_REVIEW_2025-11-26.md
- **Task Assignments**:
  - .claude/reports/WAVE_27_TASK_ASSIGNMENTS.md

## Approval

- **Status**: In Progress (implementation underway)
- **Approved By**: Agent 1 (Architect)
- **Implementation**: Agent 2 (Builder)
- **Validation**: Agent 3 (Validator)
- **Target Release**: v2.0-beta.1
