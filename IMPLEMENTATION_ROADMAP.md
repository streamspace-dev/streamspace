# StreamSpace Implementation Roadmap

**Last Updated**: 2025-11-15
**Goal**: Complete all functionality - no simplified implementations, no "future development" markers
**Status**: In Progress

---

## 📊 Progress Overview

| Category | Total | Completed | In Progress | Not Started |
|----------|-------|-----------|-------------|-------------|
| **Critical (P0)** | 1 | 0 | 0 | 1 |
| **High (P1)** | 2 | 0 | 0 | 2 |
| **Medium (P2)** | 3 | 0 | 0 | 3 |
| **Low (P3)** | 3 | 0 | 0 | 3 |
| **TOTAL** | 9 | 0 | 0 | 9 |

**Overall Completion**: 0% (0/9 tasks)

---

## 🔴 P0 - CRITICAL (Production Blockers)

### ✅ 1. Fix Mock Replica Count in Auto-Scaling ⚠️ BLOCKER
**Status**: ❌ Not Started
**File**: `api/internal/handlers/loadbalancing.go:876-877`
**Effort**: 1 hour
**Impact**: HIGH - Auto-scaling completely broken without this

**Current Code**:
```go
// Get current replica count (mock - would query Kubernetes in production)
currentReplicas := 1
```

**Required Implementation**:
- Query actual deployment from Kubernetes API
- Get current replica count from deployment spec
- Use real value for scaling calculations
- Add error handling for deployment not found

**Implementation Notes**:
- We already have `getKubernetesConfig()` helper
- We already have Kubernetes clientset creation pattern
- Can reuse code from `scaleKubernetesDeployment()` function
- Function: `TriggerScaling()` in `loadbalancing.go`

**Acceptance Criteria**:
- [ ] Query real deployment replica count from Kubernetes
- [ ] Handle deployment not found errors gracefully
- [ ] Log current vs. target replica counts
- [ ] Remove mock value and comment
- [ ] Test with actual Kubernetes cluster

**Dependencies**: None (all infrastructure already in place)

---

## 🟠 P1 - HIGH PRIORITY (Core Features)

### 2. Implement Snapshot Creation
**Status**: ❌ Not Started
**File**: `api/internal/handlers/snapshots.go:587-598`
**Effort**: 8-12 hours
**Impact**: HIGH - Snapshot feature completely non-functional

**Current Implementation**: Simulated with `time.Sleep(2 * time.Second)`

**Required Implementation**:
1. **Filesystem Snapshotting**:
   - Option A: Use `rsync` to copy session files
   - Option B: Use `tar` to create compressed archives
   - Option C: Use volume snapshots (CSI integration)
   - Recommended: Start with tar, add CSI later

2. **Process Flow**:
   - Stop/pause the session (optional, can snapshot while running)
   - Create snapshot directory structure
   - Copy or archive session filesystem
   - Calculate actual snapshot size
   - Store snapshot metadata
   - Update status to 'available'
   - Resume session if paused

3. **Storage Integration**:
   - Define snapshot storage location (NFS, S3, PVC)
   - Implement retention policies
   - Add cleanup for old snapshots

**Implementation Notes**:
- Sessions use persistent volumes at `/config`
- Need to determine snapshot storage backend
- Should support incremental snapshots later
- Compression recommended for disk space

**Acceptance Criteria**:
- [ ] Actual filesystem snapshot created (tar or rsync)
- [ ] Real size calculation (not mock 100MB)
- [ ] Snapshot stored in persistent location
- [ ] Metadata includes file count, size, creation time
- [ ] Error handling for disk space, permissions
- [ ] Background worker to avoid blocking API
- [ ] Test with actual session data

**Dependencies**: Storage backend decision needed

---

### 3. Implement Snapshot Restore
**Status**: ❌ Not Started
**File**: `api/internal/handlers/snapshots.go:622-636`
**Effort**: 6-8 hours
**Impact**: HIGH - Restore feature completely non-functional

**Current Implementation**: Simulated with `time.Sleep(3 * time.Second)`

**Required Implementation**:
1. **Restore Process**:
   - Stop target session completely
   - Backup current session state (safety)
   - Extract/copy snapshot to session volume
   - Verify file integrity
   - Start session with restored state
   - Update restore job status

2. **Safety Mechanisms**:
   - Backup before restore (rollback capability)
   - Validate snapshot integrity before restore
   - Handle mid-restore failures
   - Lock session during restore

3. **User Experience**:
   - Progress tracking for large restores
   - Estimated time remaining
   - Notification on completion

**Implementation Notes**:
- Must coordinate with session controller
- Session should be in 'stopped' state during restore
- Need rollback mechanism for failed restores
- Should support partial restore options

**Acceptance Criteria**:
- [ ] Actual file restoration from snapshot
- [ ] Session stopped before restore starts
- [ ] Backup created before restoration
- [ ] File integrity verification
- [ ] Rollback on failure
- [ ] Progress updates during restore
- [ ] Session restarted after successful restore
- [ ] Test with various snapshot sizes

**Dependencies**: Requires Snapshot Creation (#2) to be complete

---

## 🟡 P2 - MEDIUM PRIORITY (Important Features)

### 4. Implement Batch Tag Operations
**Status**: ❌ Not Started
**File**: `api/internal/handlers/batch.go:627-631`
**Effort**: 3-4 hours
**Impact**: MEDIUM - Batch tag management incomplete

**Current Implementation**: Only updates timestamp, doesn't modify tags

**Required Implementation**:
1. **Add Tags to Sessions**:
   - Parse tag array from request
   - Use PostgreSQL JSONB array operations
   - Append new tags to existing tags
   - Handle duplicate tags

2. **Remove Tags from Sessions**:
   - Parse tag array from request
   - Use JSONB array removal operations
   - Remove specified tags
   - Handle tags that don't exist

3. **Replace Tags**:
   - Clear existing tags
   - Set new tag array

**SQL Patterns**:
```sql
-- Add tags
UPDATE sessions
SET tags = tags || '["new-tag"]'::jsonb
WHERE id = $1;

-- Remove tags
UPDATE sessions
SET tags = tags - 'tag-to-remove'
WHERE id = $1;

-- Replace tags
UPDATE sessions
SET tags = '["tag1", "tag2"]'::jsonb
WHERE id = $1;
```

**Implementation Notes**:
- Tags stored as JSONB array in sessions table
- Need to handle JSONB array operations properly
- Should prevent duplicate tags
- Validate tag format (alphanumeric, hyphens, etc.)

**Acceptance Criteria**:
- [ ] Add tags operation with JSONB append
- [ ] Remove tags operation with JSONB removal
- [ ] Replace tags operation
- [ ] Duplicate tag prevention
- [ ] Tag format validation
- [ ] Bulk operation success/failure tracking
- [ ] Test with various tag scenarios

**Dependencies**: None

---

### 5. Implement Template Sharing
**Status**: ❌ Not Started
**File**: `api/internal/handlers/sessiontemplates.go:708-718`
**Effort**: 6-8 hours
**Impact**: MEDIUM - Collaboration feature missing

**Current Implementation**: Placeholder methods returning empty responses

**Required Implementation**:
1. **List Template Shares** (`ListTemplateShares`):
   - Query `template_shares` table
   - Return list of users/teams template is shared with
   - Include share permissions (read, write, manage)
   - Filter by template ID

2. **Share Template** (`ShareSessionTemplate`):
   - Create share record in `template_shares` table
   - Support sharing with users or teams
   - Set permission levels (read, write, manage)
   - Send notification to shared users
   - Audit log the share action

3. **Revoke Template Share** (`RevokeTemplateShare`):
   - Delete share record
   - Verify user has permission to revoke
   - Send notification to affected user
   - Audit log the revocation

**Database Schema** (if not exists):
```sql
CREATE TABLE IF NOT EXISTS template_shares (
    id SERIAL PRIMARY KEY,
    template_id VARCHAR(255) NOT NULL,
    shared_by VARCHAR(255) NOT NULL,
    shared_with_user_id VARCHAR(255),
    shared_with_team_id VARCHAR(255),
    permission_level VARCHAR(50) NOT NULL, -- 'read', 'write', 'manage'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_shared_with CHECK (
        (shared_with_user_id IS NOT NULL AND shared_with_team_id IS NULL) OR
        (shared_with_user_id IS NULL AND shared_with_team_id IS NOT NULL)
    )
);
```

**Implementation Notes**:
- Similar to session sharing functionality
- Should integrate with notifications system
- Permission levels: read (use), write (modify), manage (share/delete)
- Team shares apply to all team members

**Acceptance Criteria**:
- [ ] Database table created (or verified)
- [ ] List shares with permission details
- [ ] Share template with users/teams
- [ ] Revoke shares with permission check
- [ ] Notifications sent on share/revoke
- [ ] Audit logging for all operations
- [ ] Test permission inheritance for teams
- [ ] Test edge cases (self-share, duplicate shares)

**Dependencies**: None (notifications system already exists)

---

### 6. Implement Template Versioning
**Status**: ❌ Not Started
**File**: `api/internal/handlers/sessiontemplates.go:720-730`
**Effort**: 8-10 hours
**Impact**: MEDIUM - Version control missing

**Current Implementation**: Placeholder methods returning empty responses

**Required Implementation**:
1. **List Template Versions** (`ListTemplateVersions`):
   - Query `template_versions` table
   - Return version history with metadata
   - Include author, timestamp, description
   - Support pagination

2. **Create Template Version** (`CreateTemplateVersion`):
   - Snapshot current template configuration
   - Store as new version in `template_versions`
   - Auto-increment version number
   - Add version description/notes
   - Tag versions (optional)

3. **Restore Template Version** (`RestoreTemplateVersion`):
   - Load specified version from history
   - Create new version before overwriting (safety)
   - Update current template with version data
   - Audit log the restore action

**Database Schema** (if not exists):
```sql
CREATE TABLE IF NOT EXISTS template_versions (
    id SERIAL PRIMARY KEY,
    template_id VARCHAR(255) NOT NULL,
    version_number INT NOT NULL,
    template_data JSONB NOT NULL,
    description TEXT,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    tags TEXT[],
    UNIQUE(template_id, version_number)
);
```

**Implementation Notes**:
- Store entire template configuration as JSONB
- Version numbers auto-increment per template
- Should support diff viewing (nice-to-have)
- Consider storage limits (version retention policy)
- Similar to git version control concept

**Acceptance Criteria**:
- [ ] Database table created (or verified)
- [ ] List versions with metadata
- [ ] Create version with snapshot
- [ ] Auto-increment version numbers
- [ ] Restore version with safety backup
- [ ] Version descriptions support
- [ ] Optional tagging (v1.0, stable, etc.)
- [ ] Test version history integrity
- [ ] Test restore rollback scenarios

**Dependencies**: None

---

## 🟢 P3 - LOW PRIORITY (Nice-to-Have Features)

### 7. Implement Email Integration Testing
**Status**: ❌ Not Started
**File**: `api/internal/handlers/integrations.go:973-975`
**Effort**: 4-6 hours
**Impact**: LOW - Test endpoint only

**Current Implementation**: Returns placeholder message

**Required Implementation**:
1. **SMTP Configuration**:
   - Add SMTP settings to integration config
   - Support common providers (Gmail, SendGrid, AWS SES)
   - TLS/SSL support
   - Authentication (username/password, API key)

2. **Test Email Send**:
   - Send test email to configured address
   - Verify SMTP connection
   - Check authentication
   - Return detailed error messages

3. **Integration Health**:
   - Test connection on interval
   - Update health status
   - Alert on failures

**Environment Variables**:
```bash
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=notifications@streamspace.io
SMTP_PASSWORD=app-password
SMTP_FROM=StreamSpace <noreply@streamspace.io>
SMTP_TLS=true
```

**Implementation Notes**:
- Use Go's `net/smtp` package
- Support OAuth2 for Gmail
- Template system for email bodies
- This is for webhook notification emails

**Acceptance Criteria**:
- [ ] SMTP configuration structure
- [ ] Test email send functionality
- [ ] Connection validation
- [ ] TLS/SSL support
- [ ] Error message details
- [ ] Test with Gmail, SendGrid
- [ ] OAuth2 support for Gmail

**Dependencies**: None

---

### 8. Implement OIDC Authentication Mode
**Status**: ❌ Not Started
**File**: `api/internal/auth/providers.go:214-215`
**Effort**: 12-16 hours
**Impact**: LOW - Alternative to SAML (already implemented)

**Current Implementation**: Returns "not yet implemented" error

**Required Implementation**:
1. **OIDC Provider Configuration**:
   - Provider URL/discovery endpoint
   - Client ID and Secret
   - Redirect URI
   - Scope configuration
   - Token validation

2. **OIDC Flow**:
   - Discovery document fetching
   - Authorization request
   - Token exchange
   - User info retrieval
   - Session creation

3. **Provider Support**:
   - Generic OIDC (Keycloak, Okta, Auth0)
   - Google OIDC
   - Azure AD OIDC
   - Custom providers

**Configuration Structure**:
```go
type OIDCConfig struct {
    Enabled           bool   `json:"enabled"`
    ProviderURL       string `json:"provider_url"`
    ClientID          string `json:"client_id"`
    ClientSecret      string `json:"client_secret"`
    RedirectURI       string `json:"redirect_uri"`
    Scopes            []string `json:"scopes"`
    UsernameClaim     string `json:"username_claim"`
    EmailClaim        string `json:"email_claim"`
    GroupsClaim       string `json:"groups_claim"`
}
```

**Implementation Notes**:
- SAML is already fully implemented
- OIDC is alternative/additional auth method
- Can support multiple providers simultaneously
- Should reuse existing user management
- Group mapping similar to SAML

**Acceptance Criteria**:
- [ ] OIDC configuration structure
- [ ] Discovery document support
- [ ] Authorization code flow
- [ ] Token validation (JWT)
- [ ] User info extraction
- [ ] Group/role mapping
- [ ] Session management integration
- [ ] Test with Keycloak
- [ ] Test with Google
- [ ] Test with Azure AD

**Dependencies**: None (SAML already complete)

---

### 9. Complete Search Tag Aggregation
**Status**: ❌ Not Started
**File**: `api/internal/handlers/search.go:451-454`
**Effort**: 2-3 hours
**Impact**: LOW - Search optimization

**Current Implementation**: Simplified JSONB unnest query with comment

**Current Code**:
```go
// This is simplified - in production you'd want to parse the tags JSONB array
rows, err := h.db.DB().QueryContext(ctx, `
    SELECT DISTINCT unnest(tags::text[]::text[]) as tag, COUNT(*) as count
    FROM catalog_templates
    WHERE tags IS NOT NULL AND tags != '[]'::jsonb
    GROUP BY tag
    ORDER BY count DESC
    LIMIT $1
`, limit)
```

**Required Implementation**:
1. **Proper JSONB Array Handling**:
   - Use `jsonb_array_elements_text()` function
   - Handle nested structures if needed
   - Validate JSONB format

2. **Tag Aggregation**:
   - Count occurrences across all templates
   - Sort by popularity
   - Cache results for performance
   - Support filtering by category

3. **Performance Optimization**:
   - Consider materialized view
   - Add caching layer
   - Index optimization

**Better SQL**:
```sql
SELECT tag, COUNT(*) as count
FROM (
    SELECT jsonb_array_elements_text(tags) as tag
    FROM catalog_templates
    WHERE tags IS NOT NULL
      AND jsonb_typeof(tags) = 'array'
      AND jsonb_array_length(tags) > 0
) subquery
GROUP BY tag
ORDER BY count DESC, tag ASC
LIMIT $1;
```

**Implementation Notes**:
- Current implementation might work but has type casting issues
- Better to use proper JSONB functions
- Consider caching top tags (Redis or in-memory)
- Used for tag cloud/popular tags feature

**Acceptance Criteria**:
- [ ] Proper JSONB array parsing
- [ ] Tag occurrence counting
- [ ] Sorted by popularity
- [ ] Handle edge cases (null, empty arrays)
- [ ] Performance optimized
- [ ] Consider caching layer
- [ ] Test with various tag structures

**Dependencies**: None

---

## 📋 Implementation Guidelines

### Development Process
1. **Before Starting**:
   - Mark task as "In Progress" in this document
   - Update progress table
   - Create feature branch if needed

2. **During Implementation**:
   - Follow existing code patterns
   - Add comprehensive error handling
   - Include logging for debugging
   - Write tests for new functionality
   - Update API documentation

3. **After Completion**:
   - Mark task as "Completed" ✅
   - Update progress table
   - Update acceptance criteria checklist
   - Add notes about implementation decisions
   - Commit and push changes

### Code Quality Standards
- **Error Handling**: All errors must be properly handled and logged
- **Security**: Input validation, SQL injection prevention, auth checks
- **Performance**: Consider caching, indexing, bulk operations
- **Testing**: Unit tests for business logic, integration tests for APIs
- **Documentation**: Code comments, API docs, README updates

### Testing Requirements
- Unit tests for new functions
- Integration tests for API endpoints
- Manual testing with real data
- Edge case testing
- Performance testing for bulk operations

---

## 🔄 Session Continuity

### Current Session Status
- **Date**: 2025-11-15
- **Last Task Completed**: None (document just created)
- **Next Task**: Fix Mock Replica Count in Auto-Scaling (P0 #1)
- **Blockers**: None

### Notes for Next Session
- Start with P0 task (critical for auto-scaling)
- Then move to P1 tasks (snapshot functionality)
- Document any decisions made during implementation
- Update progress table after each task

---

## 📊 Detailed Progress Tracking

### P0 Tasks: 0/1 Complete (0%)
- [ ] Mock Replica Count Fix

### P1 Tasks: 0/2 Complete (0%)
- [ ] Snapshot Creation
- [ ] Snapshot Restore

### P2 Tasks: 0/3 Complete (0%)
- [ ] Batch Tag Operations
- [ ] Template Sharing
- [ ] Template Versioning

### P3 Tasks: 0/3 Complete (0%)
- [ ] Email Integration Testing
- [ ] OIDC Authentication
- [ ] Search Tag Aggregation

---

## 🎯 Success Criteria

The StreamSpace codebase will be considered **100% complete** when:
- ✅ All 9 tasks marked as completed
- ✅ All acceptance criteria met for each task
- ✅ All tests passing
- ✅ No "TODO", "FIXME", "mock", "placeholder", or "simplified" comments in production code
- ✅ All features fully functional in production environment
- ✅ Documentation updated to reflect all implemented features

---

**End of Roadmap** - Update this document as you complete tasks!
