# Product Lifecycle Management

**Version**: v1.0
**Last Updated**: 2025-11-26
**Owner**: Product + Engineering
**Status**: Policy Document
**Effective**: v2.1+

---

## Introduction

This document defines the lifecycle management policies for StreamSpace features, APIs, and components. It ensures predictable evolution, deprecation, and sunset processes that balance innovation with customer stability.

**Goals**:
- Predictable feature evolution (experimental → stable → deprecated)
- Clear API versioning and backwards compatibility
- Transparent deprecation process (advance notice, migration paths)
- Customer trust (no surprise breaking changes)

---

## Product Lifecycle Stages

### 1. Experimental (Alpha)

**Purpose**: Early-stage feature testing, rapid iteration

**Characteristics**:
- ⚠️ **No stability guarantees**
- May change or be removed without notice
- Not covered by SLAs
- Opt-in only (feature flags)
- May have bugs, incomplete functionality

**Labeling**:
- UI: "⚠️ Experimental" badge
- API: `/api/v1alpha1/...` or `?experimental=true`
- Docs: "Experimental Feature" warning

**Example**:
```markdown
## Session Recording (Experimental)

⚠️ **This feature is experimental and may change without notice.**

Session recording allows you to record VNC streams for audit/compliance.
This feature is under active development and may have performance issues.
```

**Support**:
- Community support only (GitHub Discussions)
- No SLA for bug fixes
- May be deprecated without migration path

**Graduation Criteria** (to Beta):
- Used by 10+ early adopter customers
- No critical bugs (P0/P1)
- Feedback incorporated from alpha users
- Documentation complete

---

### 2. Beta

**Purpose**: Feature hardening, broader testing, refinement

**Characteristics**:
- 🔄 **Limited stability guarantees**
- Breaking changes possible (with advance notice)
- Covered by SLA (best effort)
- Opt-in or default-on (configurable)
- Production-ready for early adopters

**Labeling**:
- UI: "🔄 Beta" badge
- API: `/api/v1beta1/...`
- Docs: "Beta Feature" notice

**Example**:
```markdown
## Multi-Cluster Support (Beta)

🔄 **This feature is in beta and may have breaking changes.**

Multi-cluster support allows agents to span multiple Kubernetes clusters.
We're gathering feedback and may adjust the API in future releases.
```

**Support**:
- Standard support (email, chat)
- SLA: Best effort (P0 within 24h, P1 within 3 days)
- Breaking changes: 30-day advance notice

**Graduation Criteria** (to Stable):
- Used by 50+ customers
- No critical bugs for 2 releases
- API stable for 3 months (no breaking changes)
- Performance benchmarks met
- Complete test coverage (>80%)

---

### 3. Stable (GA)

**Purpose**: Production-ready, fully supported

**Characteristics**:
- ✅ **Full stability guarantees**
- Backwards compatible (within major version)
- Covered by full SLA
- Default-on
- Production-ready for all customers

**Labeling**:
- UI: No badge (default assumption)
- API: `/api/v1/...`
- Docs: Standard feature documentation

**Support**:
- Full support (24/7 for enterprise)
- SLA: P0 within 1h, P1 within 4h
- Breaking changes: Only in major versions (v2 → v3)

**Backwards Compatibility Policy**:
- APIs: No breaking changes within major version
- UI: Visual changes allowed (functional compatibility maintained)
- Data: Forward/backward compatible schema migrations

**Example**:
```markdown
## Session Management

Create, view, and manage containerized sessions via web browser.
Fully supported for production use.
```

---

### 4. Deprecated

**Purpose**: Notify users of planned removal, provide migration path

**Characteristics**:
- ⚠️ **Will be removed in future release**
- Still functional (for migration period)
- Covered by SLA (during deprecation period)
- Warnings in UI, API responses, logs

**Labeling**:
- UI: "⚠️ Deprecated (will be removed in v3.0)" banner
- API: `Deprecation` HTTP header
  ```
  Deprecation: Sun, 01 Jun 2026 00:00:00 GMT
  Sunset: Sun, 01 Dec 2026 00:00:00 GMT
  Link: <https://docs.streamspace.io/migration/feature-x>; rel="alternate"
  ```
- Docs: "Deprecated" warning, migration guide

**Deprecation Notice Period**:
- **API**: 6 months minimum (2 major releases)
- **UI Feature**: 3 months minimum (1 major release)
- **CLI Command**: 3 months minimum

**Example**:
```markdown
## Legacy Template Format (Deprecated)

⚠️ **Deprecated: This format will be removed in v3.0 (December 2026)**

The v1 template format is deprecated in favor of the v2 format.
Please migrate your templates using the conversion tool:

\`\`\`bash
streamspace convert-templates --from-v1 --to-v2
\`\`\`

Migration guide: https://docs.streamspace.io/migration/templates-v2
```

**Support**:
- Standard support (bug fixes only, no new features)
- SLA: Best effort (P0 within 48h)
- Security patches: Yes (critical vulnerabilities only)

---

### 5. End-of-Life (EOL)

**Purpose**: Feature/API removed from product

**Characteristics**:
- ❌ **No longer available**
- Not functional
- No support
- Removed from documentation

**Notice**:
- Announced at deprecation (6+ months prior)
- Reminder emails (3 months, 1 month, 1 week before EOL)
- Final notice in release notes

**Example**:
```markdown
## v2.0 Release Notes

**Removed Features (End-of-Life):**
- Legacy Template Format (deprecated in v2.5, removed in v3.0)
  - Replacement: v2 template format
  - Migration guide: https://docs.streamspace.io/migration/templates-v2
```

---

## API Versioning

### Versioning Scheme

StreamSpace uses **URL-based API versioning**:
- **Stable**: `/api/v1/...`, `/api/v2/...`
- **Beta**: `/api/v1beta1/...`, `/api/v2beta1/...`
- **Alpha**: `/api/v1alpha1/...`, `/api/v2alpha1/...`

**Version Format**: `v{major}[{stability}]{incrementing}`

**Examples**:
- `/api/v1/sessions` - Stable v1
- `/api/v2/sessions` - Stable v2
- `/api/v1beta1/plugins` - Beta (v1 track)
- `/api/v1alpha1/recordings` - Alpha (v1 track)

### Version Support Policy

| Version | Support Duration | Security Patches | Bug Fixes |
|---------|------------------|------------------|-----------|
| **Current** (v2) | Indefinite (until v3) | ✅ Yes | ✅ Yes |
| **Previous** (v1) | 12 months after v2 GA | ✅ Yes | ✅ Yes |
| **Older** (v0) | EOL (6 months after v1 GA) | ❌ No | ❌ No |

**Example Timeline**:
- **v1 GA**: 2024-01-01
- **v2 GA**: 2025-06-01
- **v1 EOL**: 2026-06-01 (12 months after v2 GA)

### Breaking Changes

**What is a breaking change?**
- Removing an API endpoint
- Removing a request/response field
- Changing field types (string → int)
- Changing API semantics (behavior change)
- Renaming fields
- Adding required fields

**What is NOT a breaking change?**
- Adding new optional fields
- Adding new API endpoints
- Changing error messages (non-semantic)
- Performance improvements
- Bug fixes (that restore documented behavior)

### Deprecation Process

**Step 1: Announce (v2.0)**
- Add `Deprecation` header to API response
- Update API docs with deprecation notice
- Email customers using deprecated API

**Step 2: Warn (v2.5)**
- Log warnings when deprecated API called
- Dashboard notification (if UI affected)
- Reminder email (3 months before removal)

**Step 3: Remove (v3.0)**
- Delete endpoint from codebase
- Return 410 Gone for removed endpoints
  ```json
  {
    "error": "This endpoint was removed in v3.0",
    "deprecated_since": "v2.0",
    "removed_in": "v3.0",
    "alternative": "/api/v3/sessions",
    "migration_guide": "https://docs.streamspace.io/migration/sessions-v3"
  }
  ```

---

## Component Lifecycle

### Plugins

**Lifecycle**: Experimental → Beta → Stable → Deprecated → EOL

**Plugin Manifest** (`plugin.yaml`):
```yaml
name: session-recording
version: 0.5.0
stability: beta  # alpha, beta, stable, deprecated
deprecation:
  announced: "2025-11-01"
  sunset: "2026-05-01"
  alternative: "session-recording-v2"
  migration_guide: "https://docs.streamspace.io/plugins/recording-v2"
```

**Plugin Catalog Display**:
- **Experimental**: ⚠️ badge, warning in description
- **Beta**: 🔄 badge, "In beta" label
- **Stable**: No badge
- **Deprecated**: ⚠️ "Deprecated" banner, sunset date, migration link

### Templates

**Lifecycle**: Draft → Active → Deprecated → Archived

**Template Statuses**:
- **Draft**: Editable, not available for session creation
- **Active**: Published, available for sessions
- **Deprecated**: Visible but discouraged (warning in UI)
- **Archived**: Hidden from catalog, existing sessions continue

**Deprecation Process**:
1. **Mark Deprecated**: Template admin sets status to "deprecated"
2. **Notify Users**: Email sent to users with active sessions using template
3. **UI Warning**: "This template is deprecated. Use [alternative] instead."
4. **Sunset**: After 90 days, template archived (no new sessions)

---

## Backwards Compatibility

### Database Schema

**Policy**: Additive changes only (within major version)

**Allowed**:
- Add new tables
- Add new columns (with defaults)
- Add indexes
- Rename tables/columns (with aliases)

**Not Allowed**:
- Drop tables
- Drop columns
- Change column types (breaking)
- Remove indexes (performance regression)

**Migration Strategy**:
```sql
-- v2.1: Add new column (backwards compatible)
ALTER TABLE sessions ADD COLUMN hibernate_timeout_minutes INT DEFAULT 60;

-- v2.2: Deprecate old column (keep for compatibility)
-- (old column still works, reads from new column via trigger)

-- v3.0: Remove old column (breaking change, major version)
ALTER TABLE sessions DROP COLUMN old_timeout_field;
```

### Configuration

**Policy**: Defaults must maintain existing behavior

**Allowed**:
- Add new configuration options (with safe defaults)
- Change default values (if backwards compatible)
- Deprecate config options (with aliases)

**Not Allowed**:
- Remove config options (without deprecation period)
- Change config semantics (breaking behavior)

**Example** (Helm values):
```yaml
# v2.0
session:
  defaultTimeout: 3600  # seconds

# v2.1 (add new option, backwards compatible)
session:
  defaultTimeout: 3600
  hibernateTimeout: 1800  # NEW (default: half of defaultTimeout)

# v2.2 (deprecate old option, alias to new)
session:
  timeout:
    active: 3600        # replaces defaultTimeout (aliased)
    hibernate: 1800
```

---

## Deprecation Communication

### Announcement Channels

1. **Release Notes**: Deprecation section in CHANGELOG.md
2. **API Headers**: `Deprecation`, `Sunset` headers (RFC 8594)
3. **In-App Notifications**: Banner in admin UI
4. **Email**: Targeted emails to affected customers
5. **Blog**: Deprecation announcement post
6. **Docs**: Migration guides published

### Deprecation Notice Template

```markdown
## Deprecation Notice: [Feature Name]

**Deprecated**: [Date]
**Sunset**: [Date] (6 months minimum)
**Reason**: [Why being deprecated]
**Alternative**: [Replacement feature/API]
**Migration Guide**: [Link to migration docs]

### Impact
- **Customers Affected**: [Number] (query: [SQL/API filter])
- **Breaking Changes**: [Yes/No]
- **Action Required**: [What customers must do]

### Timeline
- **[Date]**: Deprecation announced (this notice)
- **[Date + 3mo]**: Warning emails sent
- **[Date + 5mo]**: Final reminder (1 month before sunset)
- **[Date + 6mo]**: Feature removed (EOL)

### Support
- **Deprecation Period**: Standard support (bug fixes)
- **After Sunset**: No support, feature removed

### Questions?
Contact support@streamspace.io or post in GitHub Discussions.
```

### Example: Deprecating Legacy API

**Announcement** (v2.0 Release Notes):
```markdown
## Deprecation: Legacy Session API

**Deprecated**: 2025-11-01
**Sunset**: 2026-05-01 (6 months)
**Reason**: Inconsistent response format, missing pagination
**Alternative**: New Session API (`/api/v2/sessions`)
**Migration Guide**: https://docs.streamspace.io/migration/sessions-v2

### Changes
| Legacy API (v1) | New API (v2) |
|-----------------|--------------|
| `GET /api/v1/sessions` | `GET /api/v2/sessions?page=1&limit=20` |
| Response: `{sessions: [...]}` | Response: `{data: [...], pagination: {...}}` |
| No filtering | Query params: `?status=running&template=ubuntu` |

### Migration
Update API calls from:
\`\`\`javascript
fetch('/api/v1/sessions')
\`\`\`

To:
\`\`\`javascript
fetch('/api/v2/sessions?page=1&limit=20')
  .then(res => res.json())
  .then(data => console.log(data.data)) // Note: data.data, not data.sessions
\`\`\`

### Timeline
- **2025-11-01**: v1 API marked deprecated (warnings in logs)
- **2026-02-01**: Warning emails sent (3 months before sunset)
- **2026-04-01**: Final reminder emails (1 month before sunset)
- **2026-05-01**: v1 API removed (returns 410 Gone)
```

---

## Version Support Matrix

### Current Versions (2025-11-26)

| Component | Version | Status | Support Until | Notes |
|-----------|---------|--------|---------------|-------|
| **API** | v1 | Stable | 2026-06-01 | 6 months remaining |
| **API** | v2 | Stable | TBD (current) | - |
| **UI** | v2.0-beta | Beta → Stable | - | GA in v2.0.0 |
| **K8s Agent** | v2.0 | Stable | TBD (current) | - |
| **Docker Agent** | v0.1-alpha | Experimental | TBD | Alpha phase |
| **Plugin System** | v1beta1 | Beta | TBD | Graduation to stable in v2.1 |

### Planned Deprecations

| Feature | Deprecated | Sunset | Replacement |
|---------|-----------|--------|-------------|
| API v1 | 2025-06-01 | 2026-06-01 | API v2 |
| Legacy Template Format | 2025-11-01 | 2026-05-01 | Template v2 format |
| Direct VNC URLs | 2026-01-01 | 2026-07-01 | VNC token API |

---

## References

- **Semantic Versioning**: https://semver.org/
- **API Deprecation Headers**: https://datatracker.ietf.org/doc/html/rfc8594
- **Kubernetes API Versioning**: https://kubernetes.io/docs/reference/using-api/deprecation-policy/
- **Stripe API Versioning**: https://stripe.com/docs/api/versioning (industry best practice)

---

**Version History**:
- **v1.0** (2025-11-26): Initial product lifecycle policy
- **Next Review**: v2.1 release (Q1 2026)
