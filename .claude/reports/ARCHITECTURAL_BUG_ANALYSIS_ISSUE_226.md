# Architectural Bug Analysis - Issue #226

**Date:** 2025-11-28
**Issue:** #226 - K8s Agent Cannot Self-Register (Chicken-and-Egg Authentication)
**Severity:** P0 - Blocks v2.0-beta.1 Release
**Discovered By:** Validator (Agent 3)
**Analysis By:** Architect (Agent 1)

---

## Executive Summary

**Problem:** K8s agents cannot self-register because authentication middleware requires agents to exist in database before registration endpoint can be called.

**Impact:** **RELEASE BLOCKER** - Agents cannot be deployed in v2.0

**Root Cause:** Architectural oversight introduced during security hardening (Issue #220, Wave 28)

**Recommendation:** Implement **Option 1: Shared Bootstrap Key** - Lowest risk, maintains security, minimal code changes

---

## Problem Statement

### Current Authentication Flow (Broken)

```
1. K8s Agent starts up
2. Agent calls POST /api/v1/agents/register
3. AgentAuth middleware intercepts request
4. Middleware queries: SELECT api_key_hash FROM agents WHERE agent_id = ?
5. Agent doesn't exist in database → sql.ErrNoRows
6. Middleware returns 404: "Agent must be pre-registered with an API key before connecting"
7. ❌ Registration fails - chicken-and-egg problem
```

### Expected Flow (Desired)

```
1. K8s Agent starts up with AGENT_API_KEY environment variable
2. Agent calls POST /api/v1/agents/register with API key
3. Middleware validates API key (via bootstrap key or other mechanism)
4. Registration handler creates agent record in database
5. ✅ Agent is registered and can connect
```

---

## Root Cause Analysis

### Timeline of Introduction

**Wave 28 (Issue #220) - Security Hardening:**
- Added `api_key_hash` column to `agents` table
- Added `AgentAuth` middleware to validate API keys
- Applied middleware to `/agents/register` endpoint
- **Oversight:** Didn't account for first-time registration

### Code Locations

**1. AgentAuth Middleware** (`api/internal/middleware/agent_auth.go:121-138`)
```go
// Look up agent in database
err := a.database.DB().QueryRow(`
    SELECT agent_id, api_key_hash
    FROM agents
    WHERE agent_id = $1
`, agentID).Scan(&agentIDFromDB, &apiKeyHash)

if err == sql.ErrNoRows {
    c.JSON(http.StatusNotFound, gin.H{
        "error":   "Agent not found",
        "details": "Agent must be pre-registered with an API key before connecting",
        "agentId": agentID,
    })
    c.Abort()
    return
}
```

**Problem:** Rejects requests from non-existent agents

**2. RegisterAgent Handler** (`api/internal/handlers/agents.go:124-166`)
```go
// Check if agent already exists
var existingID string
err := h.database.DB().QueryRow(
    "SELECT id FROM agents WHERE agent_id = $1",
    req.AgentID,
).Scan(&existingID)

if err == sql.ErrNoRows {
    // Agent doesn't exist - create new
    err = h.database.DB().QueryRow(`
        INSERT INTO agents (...)
        VALUES (...)
    `, ...).Scan(...)
}
```

**Problem:** Handler can create agents, but middleware blocks access

**3. Route Registration** (`api/cmd/main.go:1045-1050`)
```go
agentRoutes := v1.Group("/agents")
agentRoutes.Use(middleware.AgentAuth(database)) // ❌ Blocks registration
agentHandler.RegisterRoutes(agentRoutes)
```

**Problem:** Middleware applied to all `/agents/*` routes including `/register`

---

## Impact Assessment

### Severity: P0 - Release Blocker

**Why P0:**
1. **Cannot deploy agents** - Core functionality broken
2. **No workaround** - Manual pre-registration requires DB access
3. **Security regression** - Added in security hardening (Wave 28)
4. **Discovered late** - After Wave 29 "GO FOR RELEASE" decision

### Affected Components

- ✅ **API Backend:** Code change required
- ❌ **K8s Agent:** No change required (already sends API key)
- ❌ **Database:** No schema change required
- ❌ **UI:** No change required
- ❌ **Documentation:** Minor update needed

### Deployment Impact

**Current Deployment Flow (Broken):**
```bash
# 1. Deploy API
kubectl apply -f manifests/api-deployment.yaml

# 2. Deploy K8s Agent (with AGENT_API_KEY set)
kubectl apply -f manifests/k8s-agent-deployment.yaml

# 3. ❌ Agent fails to register (404 error)
# 4. Agent cannot connect to WebSocket
# 5. No sessions can be created
```

**Workaround (Not Viable):**
```sql
-- Manually pre-register agent via SQL
INSERT INTO agents (agent_id, api_key_hash, ...)
VALUES ('k8s-agent-1', '$2a$10$...', ...);
```

**Problem:** Requires database access, defeats self-service deployment

---

## Proposed Solutions

### Option 1: Shared Bootstrap Key (RECOMMENDED) ⭐

**Approach:**
- Add `AGENT_BOOTSTRAP_KEY` environment variable to API
- In `AgentAuth` middleware, if agent doesn't exist, check request API key against bootstrap key
- If bootstrap key matches, allow request to proceed to registration handler
- Registration handler creates agent and stores the provided API key hash

**Implementation:**

**1. Update agent_auth.go:**
```go
if err == sql.ErrNoRows {
    // Agent doesn't exist - check if using bootstrap key for first-time registration
    bootstrapKey := os.Getenv("AGENT_BOOTSTRAP_KEY")
    if bootstrapKey != "" && providedKey == bootstrapKey {
        // Allow first-time registration with bootstrap key
        c.Set("isBootstrapAuth", true)
        c.Next()
        return
    }

    c.JSON(http.StatusNotFound, gin.H{
        "error":   "Agent not found",
        "details": "Agent must be pre-registered with an API key before connecting",
        "agentId": agentID,
    })
    c.Abort()
    return
}
```

**2. Update RegisterAgent handler:**
```go
func (h *AgentHandler) RegisterAgent(c *gin.Context) {
    var req models.AgentRegistrationRequest
    if !validator.BindAndValidate(c, &req) {
        return
    }

    // Get provided API key from context (set by middleware)
    providedKey, _ := c.Get("agentAPIKey")
    apiKey := providedKey.(string)

    // Check if this is bootstrap auth
    isBootstrap, _ := c.Get("isBootstrapAuth")

    // Hash the API key for storage
    apiKeyHash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to hash API key"})
        return
    }

    // Check if agent already exists
    var existingID string
    err := h.database.DB().QueryRow(
        "SELECT id FROM agents WHERE agent_id = $1",
        req.AgentID,
    ).Scan(&existingID)

    if err == sql.ErrNoRows {
        // Agent doesn't exist - create with hashed API key
        err = h.database.DB().QueryRow(`
            INSERT INTO agents (agent_id, platform, region, status, capacity,
                               last_heartbeat, metadata, api_key_hash, created_at, updated_at)
            VALUES ($1, $2, $3, 'online', $4, $5, $6, $7, $8, $8)
            RETURNING ...
        `, req.AgentID, req.Platform, req.Region, req.Capacity,
           now, req.Metadata, string(apiKeyHash), now).Scan(...)
    }
    // ...
}
```

**Pros:**
- ✅ Minimal code changes (~20 lines)
- ✅ Maintains security (bootstrap key is secret)
- ✅ No schema changes required
- ✅ Backward compatible (existing agents unaffected)
- ✅ Standard industry pattern (similar to Kubernetes bootstrap tokens)
- ✅ Easy to deploy (single environment variable)

**Cons:**
- ⚠️ Requires bootstrap key rotation if compromised
- ⚠️ All agents must use same bootstrap key initially

**Security Considerations:**
- Bootstrap key should be strong (32+ characters)
- Bootstrap key should be different from individual agent API keys
- After registration, agents use their own unique API keys
- Bootstrap key only used for initial registration

---

### Option 2: Bypass Auth for /register

**Approach:**
- Remove `AgentAuth` middleware from `/register` endpoint only
- Move API key validation into `RegisterAgent` handler
- Handler validates and stores API key hash during registration

**Implementation:**

**1. Update route registration (main.go):**
```go
// Agent self-registration (NO middleware - validates internally)
v1.POST("/agents/register", agentHandler.RegisterAgent)

// Other agent routes (with middleware)
agentRoutes := v1.Group("/agents")
agentRoutes.Use(middleware.AgentAuth(database))
agentHandler.RegisterOtherRoutes(agentRoutes) // heartbeat, etc.
```

**2. Update RegisterAgent handler:**
```go
func (h *AgentHandler) RegisterAgent(c *gin.Context) {
    // Manually extract and validate API key (since no middleware)
    apiKey := c.GetHeader("X-Agent-API-Key")
    if apiKey == "" {
        c.JSON(401, gin.H{"error": "API key required"})
        return
    }

    // Check expected API key from environment
    expectedKey := os.Getenv("AGENT_API_KEY")
    if apiKey != expectedKey {
        c.JSON(401, gin.H{"error": "Invalid API key"})
        return
    }

    // Hash and store API key
    apiKeyHash, _ := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)

    // Create agent with api_key_hash
    // ...
}
```

**Pros:**
- ✅ Simpler logic (no bootstrap key concept)
- ✅ Clear separation (registration vs. other endpoints)
- ✅ Easy to understand

**Cons:**
- ⚠️ Requires refactoring route registration
- ⚠️ Duplicates API key validation logic
- ⚠️ Less flexible (harder to support multiple registration methods)
- ⚠️ All agents must share same initial API key

---

### Option 3: Admin Pre-Provisioning (NOT RECOMMENDED)

**Approach:**
- Require admins to create agent records via UI/API before deploying agents
- Agents must be pre-registered with API keys
- Current workflow, just formalized

**Implementation:**

**1. Add UI page for agent pre-provisioning**
**2. Admin workflow:**
```
1. Admin logs into UI
2. Admin navigates to Agents page
3. Admin clicks "Add Agent"
4. Admin enters agent_id, generates API key
5. Admin copies API key
6. Admin deploys agent with API key in environment
7. Agent registers successfully
```

**Pros:**
- ✅ No code changes to middleware/handlers
- ✅ Explicit control over agent deployment
- ✅ Audit trail of who created agents

**Cons:**
- ❌ **Operationally burdensome** - Manual step for every agent
- ❌ **Breaks Helm deployment** - Can't deploy agents automatically
- ❌ **Not self-service** - Requires admin intervention
- ❌ **Scalability issues** - Manual process for 100s of agents
- ❌ **Poor UX** - Extra steps for common operation

---

## Recommendation

### ✅ **Implement Option 1: Shared Bootstrap Key**

**Rationale:**

1. **Lowest Risk:**
   - Minimal code changes (~20-30 lines)
   - No schema changes
   - No route refactoring
   - Backward compatible

2. **Industry Standard:**
   - Kubernetes uses bootstrap tokens for node registration
   - Docker Swarm uses join tokens
   - Consul uses bootstrap ACL tokens
   - Proven pattern for agent enrollment

3. **Security:**
   - Bootstrap key is secret (not in codebase)
   - Each agent gets unique API key after registration
   - Bootstrap key only used once per agent
   - Can be rotated if needed

4. **Operational Excellence:**
   - Self-service deployment
   - Helm chart compatibility
   - No manual provisioning required
   - Scalable to 100s of agents

5. **Implementation Speed:**
   - Can be completed in 2-3 hours
   - Easy to test
   - Low regression risk

---

## Implementation Plan (Option 1)

### Phase 1: Code Changes (2 hours)

**1. Update AgentAuth Middleware** (`api/internal/middleware/agent_auth.go`)
- Add bootstrap key check when agent doesn't exist
- Set `isBootstrapAuth` flag in context
- Allow request to proceed if bootstrap key matches

**2. Update RegisterAgent Handler** (`api/internal/handlers/agents.go`)
- Extract API key from context
- Hash API key for storage
- Store `api_key_hash` during agent creation

**3. Add Environment Variable** (`.env.example`, `manifests/*.yaml`)
- Add `AGENT_BOOTSTRAP_KEY` documentation
- Update Helm chart values
- Update deployment manifests

### Phase 2: Testing (1 hour)

**1. Unit Tests:**
- Test bootstrap key validation
- Test API key hashing and storage
- Test existing agent re-registration

**2. Integration Tests:**
- Deploy API with bootstrap key
- Deploy agent with API key
- Verify agent registers successfully
- Verify agent can connect to WebSocket

### Phase 3: Documentation (30 min)

**1. Update Deployment Guide:**
- Document `AGENT_BOOTSTRAP_KEY` requirement
- Explain bootstrap vs. agent API keys
- Security best practices

**2. Update CHANGELOG:**
- Document fix for Issue #226
- Breaking change notice (requires bootstrap key)

### Phase 4: Review and Merge (30 min)

**1. Code Review:**
- Builder reviews changes
- Validator tests deployment

**2. Merge:**
- Create hotfix branch from feature branch
- Apply fix
- Merge back to feature branch
- Update v2.0-beta.1 milestone

---

## Security Considerations

### Bootstrap Key Management

**Generation:**
```bash
# Generate strong bootstrap key (32 characters)
openssl rand -base64 32
```

**Storage:**
- Store in Kubernetes secrets
- Never commit to git
- Rotate periodically (every 90 days)

**Helm Chart Values:**
```yaml
api:
  env:
    - name: AGENT_BOOTSTRAP_KEY
      valueFrom:
        secretKeyRef:
          name: streamspace-secrets
          key: agent-bootstrap-key

agents:
  k8s:
    env:
      - name: AGENT_API_KEY
        valueFrom:
          secretKeyRef:
            name: streamspace-secrets
            key: agent-api-key
```

### Agent API Key Lifecycle

**First Registration:**
1. Agent uses bootstrap key to register
2. API stores hash of agent's unique API key
3. Future requests use agent's unique key (not bootstrap)

**Key Rotation:**
1. Generate new agent API key
2. Update agent deployment
3. Agent re-registers with new key
4. API updates `api_key_hash` in database

---

## Alternative: Quick Hotfix (Option 2 Simplified)

**If Option 1 is deemed too complex for immediate release:**

**Quick Fix (5 lines of code):**

Update `api/cmd/main.go`:
```go
// Agent self-registration (bypass auth for registration only)
v1.POST("/agents/register", agentHandler.RegisterAgent)

// Other agent routes (with auth)
agentRoutes := v1.Group("/agents")
agentRoutes.Use(middleware.AgentAuth(database))
agentHandler.RegisterOtherRoutes(agentRoutes)
```

Update `RegisterAgent` handler to validate API key directly.

**Pros:**
- ✅ Fastest fix (< 1 hour)
- ✅ Unblocks release immediately

**Cons:**
- ⚠️ Less elegant
- ⚠️ Requires all agents to share same API key initially
- ⚠️ May need refactoring later

---

## Impact on v2.0-beta.1 Release

### If Fixed Today (2025-11-28)

**Timeline:**
- Implementation: 2-3 hours
- Testing: 1 hour
- Documentation: 30 min
- Review: 30 min
- **Total: 4-5 hours**

**Release Impact:**
- Delay v2.0-beta.1 release by 1 day
- New target: 2025-11-29 EOD
- Add Issue #226 to milestone
- Update CHANGELOG with fix

### If NOT Fixed

**Impact:**
- ❌ Cannot deploy K8s agents
- ❌ Platform is non-functional
- ❌ Cannot release v2.0-beta.1
- ❌ Major regression from v1.x

**Conclusion:** **MUST FIX BEFORE RELEASE**

---

## Recommendation Summary

**Action:** Implement **Option 1: Shared Bootstrap Key**

**Assignee:** Builder (Agent 2)

**Timeline:** 4-5 hours (today, 2025-11-28)

**Deliverables:**
1. Updated `agent_auth.go` (bootstrap key check)
2. Updated `agents.go` (API key hashing/storage)
3. Updated environment variables/Helm chart
4. Unit tests for bootstrap auth
5. Integration test (deploy agent end-to-end)
6. Documentation updates

**Release Impact:**
- Delay v2.0-beta.1 by 1 day (2025-11-29)
- Add Issue #226 to milestone
- Re-run integration tests
- Update CHANGELOG

**Risk Assessment:** LOW
- Minimal code changes
- Well-understood pattern
- Easy to test
- Easy to rollback (remove bootstrap key check)

---

## Conclusion

**Issue #226 is a P0 release blocker but can be fixed quickly with Option 1.**

The chicken-and-egg problem was introduced during security hardening (Wave 28) and represents a common architectural pattern challenge. The recommended solution (shared bootstrap key) is an industry-standard approach used by Kubernetes, Docker Swarm, and other distributed systems.

**Recommended Next Steps:**
1. ✅ Approve Option 1 approach
2. Assign to Builder (Agent 2)
3. Implement fix (4-5 hours)
4. Re-run integration tests
5. Update v2.0-beta.1 release date to 2025-11-29
6. Proceed with release

---

**Report Complete:** 2025-11-28
**Severity:** P0 - Release Blocker
**Status:** Awaiting approval for Option 1 implementation
**ETA for Fix:** 4-5 hours
**New Release Target:** 2025-11-29 EOD
