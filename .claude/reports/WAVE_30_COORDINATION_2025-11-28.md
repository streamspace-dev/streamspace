# Wave 30 Coordination - P0 Release Blocker

**Date:** 2025-11-28
**Wave:** 30 (Critical Bug Fix)
**Status:** 🔴 **ACTIVE** - Agent assignments complete
**Priority:** P0 - RELEASE BLOCKER

---

## Executive Summary

**Critical Issue Discovered:** Issue #226 - Agent registration chicken-and-egg authentication bug

**Status:**
- ✅ Issue identified and analyzed
- ✅ Solution designed (shared bootstrap key)
- ✅ Detailed implementation plan created
- ✅ Builder assigned with comprehensive instructions
- 🔄 Implementation in progress

**Release Impact:**
- v2.0-beta.1 delayed by 1 day
- New release target: **2025-11-29 EOD**
- Issue #226 added to v2.0-beta.1 milestone

---

## Issue Overview

### Problem Statement

**Issue #226: K8s Agent Cannot Self-Register**

K8s agents cannot self-register because the AgentAuth middleware requires agents to exist in the database before the registration endpoint can be called.

**Authentication Flow (Broken):**
```
1. K8s Agent starts → Calls POST /api/v1/agents/register
2. AgentAuth middleware intercepts request
3. Middleware queries: SELECT api_key_hash FROM agents WHERE agent_id = ?
4. Agent doesn't exist → sql.ErrNoRows
5. Middleware returns 404: "Agent must be pre-registered"
6. ❌ Registration fails - chicken-and-egg problem
```

**Root Cause:**
- Introduced in Wave 28 (Issue #220 - Security hardening)
- Auth middleware applied to `/agents/register` endpoint
- Oversight: Didn't account for first-time registration

**Impact:**
- ❌ Cannot deploy K8s agents in v2.0
- ❌ Core functionality broken
- ❌ **BLOCKS v2.0-beta.1 RELEASE**

---

## Solution: Shared Bootstrap Key

### Approved Approach

**Option 1: Shared Bootstrap Key Pattern** (Industry Standard)

**How it Works:**
1. API has `AGENT_BOOTSTRAP_KEY` environment variable
2. Agent provides API key in registration request
3. Middleware checks if agent exists in database
4. If agent doesn't exist, middleware checks if provided key matches bootstrap key
5. If bootstrap key matches, allow registration to proceed
6. Registration handler creates agent and stores API key hash
7. Future requests use agent's unique API key (not bootstrap)

**Why This Approach:**
- ✅ Industry standard (Kubernetes, Docker, Consul use this)
- ✅ Minimal code changes (~130 lines total)
- ✅ Maintains security
- ✅ Self-service deployment
- ✅ Scalable
- ✅ Low regression risk

---

## Agent Assignments

### Builder (Agent 2) - P0 CRITICAL 🚨

**Branch:** `claude/v2-builder`
**Timeline:** 4-5 hours (2025-11-28)
**Status:** 🔴 ASSIGNED - Ready to start immediately

**Task:** Fix Issue #226 - Agent Registration Bug

**Implementation Steps:**

**1. Update AgentAuth Middleware** (`api/internal/middleware/agent_auth.go`)
```go
// When agent doesn't exist in database
if err == sql.ErrNoRows {
    // Check if using bootstrap key for first-time registration
    bootstrapKey := os.Getenv("AGENT_BOOTSTRAP_KEY")
    if bootstrapKey != "" && providedKey == bootstrapKey {
        // Allow first-time registration
        c.Set("isBootstrapAuth", true)
        c.Set("agentAPIKey", providedKey)
        c.Next()
        return
    }

    // Otherwise reject
    c.JSON(http.StatusNotFound, gin.H{
        "error": "Agent not found",
        "details": "Agent must be pre-registered with an API key before connecting",
    })
    c.Abort()
    return
}
```
**Lines:** ~15 added

**2. Update RegisterAgent Handler** (`api/internal/handlers/agents.go`)
```go
func (h *AgentHandler) RegisterAgent(c *gin.Context) {
    var req models.AgentRegistrationRequest
    if !validator.BindAndValidate(c, &req) {
        return
    }

    // Get API key from context (set by middleware)
    providedKeyRaw, exists := c.Get("agentAPIKey")
    if !exists {
        c.JSON(401, gin.H{"error": "API key required"})
        return
    }
    providedKey := providedKeyRaw.(string)

    // Hash API key for storage
    apiKeyHash, err := bcrypt.GenerateFromPassword([]byte(providedKey), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to hash API key"})
        return
    }

    // Check if agent exists
    var existingID string
    err = h.database.DB().QueryRow(
        "SELECT id FROM agents WHERE agent_id = $1",
        req.AgentID,
    ).Scan(&existingID)

    if err == sql.ErrNoRows {
        // Create agent with hashed API key
        err = h.database.DB().QueryRow(`
            INSERT INTO agents (agent_id, platform, region, status, capacity,
                               last_heartbeat, metadata, api_key_hash, created_at, updated_at)
            VALUES ($1, $2, $3, 'online', $4, $5, $6, $7, $8, $8)
            RETURNING ...
        `, req.AgentID, req.Platform, req.Region, req.Capacity,
           now, req.Metadata, string(apiKeyHash), now).Scan(...)
    }
    // ... rest of handler
}
```
**Lines:** ~25 modified

**3. Add Environment Variables**

`.env.example`:
```bash
# Agent Bootstrap Key (for first-time agent registration)
# Generate with: openssl rand -base64 32
AGENT_BOOTSTRAP_KEY=your-secure-bootstrap-key-here
```

`chart/values.yaml`:
```yaml
api:
  env:
    agentBootstrapKey: ""  # Override via --set or secrets
```

`chart/templates/api-deployment.yaml`:
```yaml
- name: AGENT_BOOTSTRAP_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "streamspace.fullname" . }}-secrets
      key: agent-bootstrap-key
```

**Lines:** ~10 added

**4. Add Unit Tests** (`api/internal/middleware/agent_auth_test.go`)
- Test: Bootstrap key allows registration for non-existent agent
- Test: Invalid bootstrap key is rejected
- Test: Existing agent uses its own API key (not bootstrap)
**Lines:** ~50 added

**5. Update Documentation**

`docs/V2_DEPLOYMENT_GUIDE.md`:
- Bootstrap key setup instructions
- Security best practices
- Key rotation procedures

`CHANGELOG.md`:
- Document fix for Issue #226
- Breaking change notice (requires bootstrap key)

**Lines:** ~25 added

**Total Changes:** ~130 lines across 9 files

**Deliverables:**
- ✅ Updated middleware with bootstrap key check
- ✅ Updated handler with API key hashing
- ✅ Environment variable configuration
- ✅ Unit tests (3+ test cases)
- ✅ Integration test validation
- ✅ Documentation updates
- ✅ Report: `.claude/reports/ISSUE_226_FIX_COMPLETE.md`

**Acceptance Criteria:**
- ✅ Agent can register with bootstrap key
- ✅ API key hash stored in database
- ✅ Subsequent requests use agent's unique API key
- ✅ All unit tests passing
- ✅ Integration test: Deploy agent end-to-end successfully
- ✅ Documentation complete

---

### Validator (Agent 3) - STANDBY

**Branch:** `claude/v2-validator`
**Status:** ⏸️ STANDBY - Ready to validate fix
**Timeline:** 1 hour after Builder completes

**Tasks:**
1. Wait for Builder to complete Issue #226
2. Re-run integration tests with fixed agent registration
3. Verify agents can deploy and register automatically
4. Verify `api_key_hash` stored correctly in database
5. Update integration test report
6. Provide final GO/NO-GO recommendation

**Deliverable:**
- Updated integration test report with agent registration validation

---

### Scribe (Agent 4) - STANDBY

**Branch:** `claude/v2-scribe`
**Status:** ⏸️ STANDBY - Available if needed
**Priority:** Low

**Potential Tasks:**
- Review and enhance deployment documentation
- Update release notes with critical fix
- Clarify bootstrap key security best practices

**Note:** Builder has documentation covered, Scribe only needed if additional polish required

---

### Architect (Agent 1) - Coordination

**Status:** 🟢 ACTIVE - Wave 30 coordination

**Tasks Completed:**
1. ✅ Identified P0 release blocker (Issue #226)
2. ✅ Created architectural analysis (600+ lines)
   - `.claude/reports/ARCHITECTURAL_BUG_ANALYSIS_ISSUE_226.md`
3. ✅ Evaluated 3 solution options
4. ✅ Recommended Option 1 (Shared Bootstrap Key)
5. ✅ Created detailed implementation plan
6. ✅ Assigned Issue #226 to Builder with comprehensive instructions
7. ✅ Updated MULTI_AGENT_PLAN with Wave 30
8. ✅ Labeled and milestoned Issue #226

**Tasks Pending:**
- ⏳ Monitor Builder progress
- ⏳ Integrate Builder's fix when ready
- ⏳ Wait for Validator's final GO recommendation
- ⏳ Merge to main branch
- ⏳ Tag v2.0.0-beta.1 release

---

## Timeline

### Wave 30 Schedule

**Day 1 (2025-11-28):**
- 14:00 - Wave 30 coordination complete (Architect)
- 14:00 - Builder starts implementation
- 14:00-16:00 - Code changes (middleware + handler)
- 16:00-17:00 - Unit tests
- 17:00-17:30 - Documentation
- 17:30-19:00 - Integration testing + review
- **19:00 EOD** - Builder pushes fix

**Day 2 (2025-11-29):**
- 09:00 - Validator re-runs integration tests
- 10:00 - Validator provides GO/NO-GO
- 11:00 - Architect merges to main
- 12:00 - Tag v2.0.0-beta.1
- 13:00 - Deploy to staging
- **14:00** - v2.0-beta.1 RELEASED 🚀

**Total Delay:** 1 day (acceptable for critical fix)

---

## Risk Assessment

### Implementation Risk: LOW

**Mitigations:**
- ✅ Minimal code changes (~30 lines in core logic)
- ✅ Well-understood pattern (Kubernetes bootstrap tokens)
- ✅ Easy to test (unit + integration)
- ✅ Easy to rollback (remove bootstrap key check)
- ✅ No schema changes required
- ✅ Backward compatible (existing agents unaffected)

### Security Risk: LOW

**Bootstrap Key Security:**
- Must be strong (32+ characters via `openssl rand -base64 32`)
- Stored in Kubernetes secrets (never in git)
- Different from individual agent API keys
- Rotated periodically (every 90 days)
- Only used for initial registration

**Agent API Keys:**
- Each agent gets unique API key after registration
- API key hash stored in database (bcrypt)
- Bootstrap key only used once per agent
- Future requests use agent's unique key

---

## Release Impact

### v2.0-beta.1 Milestone

**Before Issue #226:**
- Open issues: 0
- Status: Ready for release
- Target date: 2025-11-28

**After Issue #226:**
- Open issues: 1 (#226)
- Status: Blocked
- Target date: 2025-11-29 (+1 day)

**Milestone Update:**
- Added Issue #226 to v2.0-beta.1
- Total milestone issues: 31 (30 closed + 1 open)
- Completion: 97% → 100% after fix

### CHANGELOG Update

**v2.0.0-beta.1 (2025-11-29):**

**Fixed:**
- **[CRITICAL]** Fixed agent registration chicken-and-egg problem (Issue #226)
  - Added `AGENT_BOOTSTRAP_KEY` for first-time agent registration
  - Agents can now self-register without manual database provisioning
  - Introduced in Wave 28 security hardening, fixed in Wave 30

---

## Success Criteria

### Wave 30 Success

**Builder Deliverables:**
- ✅ Issue #226 fix implemented
- ✅ All unit tests passing
- ✅ Integration test successful
- ✅ Documentation complete
- ✅ Report delivered

**Validator Deliverables:**
- ✅ Integration tests re-run successfully
- ✅ Agent deployment validated end-to-end
- ✅ GO recommendation provided

**Release Criteria:**
- ✅ Issue #226 closed
- ✅ All 31 milestone issues closed
- ✅ Integration tests passing
- ✅ Agents can deploy automatically
- ✅ Ready for v2.0-beta.1 tag

---

## Documentation Updates

### Files to Update

**Code:**
1. `api/internal/middleware/agent_auth.go` - Bootstrap key check
2. `api/internal/handlers/agents.go` - API key hashing
3. `.env.example` - Bootstrap key documentation
4. `chart/values.yaml` - Helm chart values
5. `chart/templates/api-deployment.yaml` - Environment variables
6. `chart/templates/secrets.yaml` - Bootstrap key secret
7. `api/internal/middleware/agent_auth_test.go` - Unit tests

**Documentation:**
8. `docs/V2_DEPLOYMENT_GUIDE.md` - Bootstrap key setup
9. `CHANGELOG.md` - Fix documentation

**Reports:**
10. `.claude/reports/ISSUE_226_FIX_COMPLETE.md` - Builder's completion report

---

## Communication

### GitHub Issue

**Issue #226:**
- Status: OPEN → IN PROGRESS → CLOSED
- Labels: P0, bug, security, blocking, agent:builder
- Milestone: v2.0-beta.1
- Assignee: Builder (Agent 2)
- Detailed implementation instructions added
- Progress tracked via comments

### MULTI_AGENT_PLAN

**Updated Sections:**
- Current Status: Wave 30 active
- Wave 30 section added with agent assignments
- Wave 29 marked complete
- Release target updated to 2025-11-29

---

## Lessons Learned

### What Went Well

1. **Early Detection:** Validator caught bug during integration testing
2. **Rapid Analysis:** Architect identified root cause and solution within hours
3. **Clear Assignment:** Builder has comprehensive implementation instructions
4. **Structured Process:** Wave-based coordination enabled quick response

### What Could Improve

1. **Security Review:** Future security changes need integration test validation
2. **Regression Testing:** Add agent registration to automated test suite
3. **Architecture Review:** Multi-agent auth flows need design review

### Preventive Measures

**For Future Releases:**
1. Add agent registration to integration test checklist
2. Review all auth middleware changes for first-time flows
3. Validate self-service patterns before merging
4. Include "fresh deployment" tests in CI/CD

---

## Conclusion

**Wave 30 Status:** 🔴 **ACTIVE** - Agent assignments complete

**Issue #226:** P0 Release Blocker identified and assigned

**Solution:** Shared bootstrap key pattern (industry standard)

**Builder Assignment:** Comprehensive 130-line implementation with detailed instructions

**Timeline:** 4-5 hours implementation + 1 hour validation = 1 day delay

**Release Impact:** v2.0-beta.1 delayed to 2025-11-29 (acceptable)

**Risk:** LOW - Minimal code changes, well-understood pattern, easy to test

**Confidence:** HIGH - Clear solution, experienced agent, comprehensive plan

**Next Action:** Builder implements fix, Validator validates, Architect merges and releases

---

**Report Complete:** 2025-11-28
**Wave Status:** Active
**Agent Assignments:** Complete
**Builder Status:** Ready to start
**Release Target:** 2025-11-29 EOD

**LET'S FIX THIS AND SHIP v2.0-beta.1! 🚀**
