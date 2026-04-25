---
description: GitHub Issue & Project Management Workflow
alwaysApply: false
---

# GitHub Workflow Guide

**Purpose**: Structured issue management aligned to Zencoder multi-agent framework (2-3 day waves).

## Overview

StreamSpace uses GitHub Issues as the **single source of truth** for task tracking, organized into **2-3 day waves** with clear roles for each agent:
- **Architect**: Wave planning, triage, master integration
- **Builder**: Implementation, feature development
- **Validator**: Testing, QA, security audits
- **Scribe**: Documentation, changelog maintenance
- **Security**: Vulnerability assessment, compliance

## Wave Structure

### Wave Lifecycle

```
Week View:
│ Mon 11/26 ├─ Wave 27 (2-3 days) ─┤ Wed 11/28
│ Thu 11/29 ├─ Wave 28 (2-3 days) ─┤ Sun 12/01
│ Mon 12/02 ├─ Wave 29 (2-3 days) ─┤ Wed 12/05
└─ ... until v2.0-beta.1 release
```

### Each Wave Has

1. **Planning Issue** (template: `.github/ISSUE_TEMPLATE/03-wave-planning.md`)
   - Links to all work items
   - DoD (Definition of Done) checklist
   - Daily standup template
   - Blocker management section

2. **Work Issues** (linked in wave)
   - Labeled with `wave:27`, `agent:builder`, `size:m`, etc.
   - Assigned to specific agent
   - Tracked through workflow states

3. **Execution**
   - Builder develops on feature branches
   - Validator creates PR for testing
   - Scribe updates docs in parallel
   - Architect merges to master when ready

## Issue Labels

### Priority Labels
- **P0**: Critical (blocks release, security, system down)
- **P1**: High (major feature, bug affecting workflow)
- **P2**: Medium (minor feature, nice-to-fix bugs)
- **P3**: Low (backlog, future consideration)

### Agent Assignment
- **agent:architect**: Wave planning, integration, triage
- **agent:builder**: Implementation, feature development
- **agent:validator**: Testing, QA, bug verification
- **agent:scribe**: Documentation, changelog, communication
- **agent:security**: Vulnerability assessment, compliance

### Status/Workflow Labels
- **status:blocked**: Waiting for another issue/PR
- **status:in-review**: PR submitted, awaiting review
- **wave:27**, **wave:28**: Current wave assignment
- **ready-for-testing**: Implementation complete, waiting for validation
- **needs-triage**: New issue, not yet assigned

### Component Labels
- **component:backend**: Go API, handlers, middleware
- **component:ui**: React frontend, components
- **component:k8s-agent**: Kubernetes agent
- **component:docker-agent**: Docker agent
- **component:infrastructure**: Helm, Terraform, deployment
- **component:database**: Database, schema migrations
- **component:websocket**: WebSocket protocol, streaming

### Size Labels (time estimates)
- **size:xs**: < 2 hours
- **size:s**: 2-4 hours
- **size:m**: 4-8 hours
- **size:l**: 1-2 days
- **size:xl**: 2-5 days

### Risk Labels
- **risk:breaking**: Breaking change (requires migration)
- **risk:high**: High risk of regressions
- **needs:security-review**: Requires security team sign-off
- **needs:testing**: Needs extra testing before merge

## Issue Workflow

### 1. Create Issue

**Use one of these templates:**
- `01-feature-request.md` for new features
- `02-bug-report.md` for bugs
- `03-wave-planning.md` for wave planning (Architect only)

**Example:**
```bash
# Create issue via GitHub CLI
gh issue create \
  --title "[FEATURE] Add org_id to JWT claims" \
  --label "enhancement,P0,component:backend,agent:builder" \
  --body "$(cat << 'EOF'
## Summary
JWT tokens need to include org_id for multi-tenant support.

## Acceptance Criteria
- [ ] JWT struct includes org_id field
- [ ] Auth service adds org_id to generated tokens
- [ ] Middleware extracts org_id to context
- [ ] Tests verify org_id in token

## Definition of Ready
- [x] Clear acceptance criteria
- [x] Component: api/internal/middleware, api/internal/services
- [ ] Size: (to be assigned)
- [ ] Agent: (to be assigned)
- [ ] Wave: (to be assigned)
EOF
)"
```

### 2. Triage & Planning (Architect)

**Definition of Ready (DoR):**
- [ ] Clear, specific acceptance criteria (no ambiguity)
- [ ] Component(s) identified
- [ ] Size estimated (XS/S/M/L/XL)
- [ ] Agent assigned (builder, validator, scribe, security)
- [ ] Wave planned (which 2-3 day cycle)
- [ ] Dependencies identified (links to blocking/dependent issues)

**Assign labels:**
```bash
gh issue edit 212 --add-label "agent:builder,size:l,wave:27,component:backend"
```

**Link dependencies:**
```bash
# If issue #212 blocks #211:
gh issue comment 211 --body "🔗 Blocked by #212"
gh issue edit 211 --add-label "status:blocked"
```

**Post wave assignment:**
```bash
gh issue comment 212 --body "✅ Triaged for Wave 27 (2025-11-26 → 2025-11-28)
- Assigned to: @builder
- Size: Large (1-2 days)
- Priority: P0 (critical for release)
- Dependencies: None

Work can begin immediately."
```

### 3. Implementation (Builder)

**Workflow:**
1. Create feature branch: `git checkout -b feature/issue-212-org-context`
2. Commit regularly with semantic messages
3. Push to origin: `git push origin feature/issue-212-org-context`
4. When complete, open PR linking to issue: `Closes #212`

**Before moving to Testing:**
```bash
make fmt lint test    # All must pass
git log master..HEAD --oneline  # Review changes
```

**Signal readiness:**
```bash
gh issue comment 212 --body "✅ Implementation complete. All tests passing (78% coverage). Ready for validation. See PR #XXX"
gh issue edit 212 --add-label "ready-for-testing"
```

### 4. Testing & Validation (Validator)

**Workflow:**
1. Review issue acceptance criteria
2. Test the implementation against DoD
3. File bugs if needed
4. Mark complete when ready

**If bug found:**
```bash
gh issue create \
  --title "[BUG] JWT org_id not extracted in middleware" \
  --label "bug,P1,component:backend" \
  --body "Found while testing #212...
  
Reproduction: 1. Create JWT with org_id...
  
Affects: #212 validation"

# Mark original as blocked
gh issue edit 212 --add-label "status:blocked"
gh issue comment 212 --body "⚠️ Blocking issue found: #XXX"
```

**When validation passes:**
```bash
gh issue comment 212 --body "✅ VALIDATION PASSED
- Acceptance criteria verified ✓
- Integration tests passing ✓
- Coverage: 78% (target: 70%+) ✓
- Security review: Not required (non-auth code) ✓

Ready for master merge."

gh issue edit 212 --remove-label "ready-for-testing" --add-label "status:in-review"
```

### 5. Documentation (Scribe)

**Workflow (parallel to Builder/Validator):**
1. Review issue and implementation
2. Update `CHANGELOG.md` with feature
3. Update relevant docs/ files
4. Update README if major feature

**Example CHANGELOG entry:**
```markdown
## [Unreleased]

### Added
- Multi-tenancy org context support (#212)
  - JWT claims now include `org_id`
  - Auth middleware extracts org_id to Gin context
  - See docs/MULTI_TENANCY.md for setup
```

**Signal completion:**
```bash
gh issue comment 212 --body "📝 Documentation complete
- CHANGELOG.md updated
- docs/MULTI_TENANCY.md created
- SECURITY.md org isolation section added

Ready for master merge."
```

### 6. Integration & Merge (Architect)

**Final checklist before merge:**
- [ ] Builder marked complete ✓
- [ ] Validator marked complete ✓
- [ ] Scribe marked complete ✓
- [ ] All CI/CD checks passing ✓
- [ ] No other blockers ✓

**Merge to master:**
```bash
git checkout master
git pull origin master
git merge --ff-only origin/claude/v2-scribe   # Docs first
git merge --ff-only origin/claude/v2-builder  # Implementation
git merge --ff-only origin/claude/v2-validator # Tests
git push origin master

# Close issue
gh issue close 212 --comment "✅ Merged to master. PR #XXX"
```

**After wave completes:**
```bash
# Create retrospective
gh issue edit 223 --body "$(cat << 'EOF'
## Wave 27 Retrospective

### Completed ✅
- #212: Org context and RBAC plumbing
- #211: WebSocket org scoping and auth guard
- #200: Fix Broken Test Suites

### Blockers (Resolved)
- [Date] #211 was blocked by #212 → Resolved when #212 completed

### Metrics
- Issues closed: 3
- Avg time per issue: 8 hours
- Test coverage: 78% → 82% (gain)
- Velocity: 3 issues / 2 days

### What Went Well
- Clear dependency planning prevented rework
- Daily standups kept team aligned

### Improvements for Next Wave
- Need earlier security review
- Consider splitting large issues

---

Next wave: #224 (Wave 28 - Testing & Release Prep)
EOF
)"
```

## Workflow States

```
New Issue
    ↓
[Triage] → Ready (DoR met)
    ↓
[Builder] → ready-for-testing (implementation complete)
    ↓
[Validator] → status:in-review (validation complete)
    ↓
[Scribe] → ready-for-merge (docs complete)
    ↓
[Architect] → Closed (merged to master)
```

## Commands Quick Reference

```bash
# Create issue with DoR template
gh issue create --title "[FEATURE] ..." \
  --label "enhancement,P0,component:backend,agent:builder"

# Assign to wave
gh issue edit 212 --add-label "wave:27"

# Mark ready for testing
gh issue edit 212 --add-label "ready-for-testing"

# Link dependency
gh issue comment 211 --body "🔗 Blocked by #212"
gh issue edit 211 --add-label "status:blocked"

# List wave issues
gh issue list --search "label:wave:27" --state open

# List builder backlog
gh issue list --label "agent:builder,P0" --state open

# Generate wave report
gh issue list --search "label:wave:27 state:closed" | wc -l

# Close issue
gh issue close 212 --comment "✅ Merged to master"
```

## Common Patterns

### Breaking Changes
```
Title: [BREAKING] Remove deprecated API endpoint
Labels: risk:breaking, P1, component:backend
Body:
- Deprecated in v1.9
- Removal in v2.0-beta.1
- Migration: See MIGRATION.md
```

### Security Issues
```
Title: [SECURITY] Fix JWT validation bypass
Labels: P0, security, needs:security-review
Body:
- Description: [Technical details]
- Impact: [What's at risk]
- Fix: [Proposed solution]
- CWE: [Reference]
```

### Large Issues (Multi-Day)
```
Title: [EPIC] Implement WebSocket Multi-Tenancy (Wave 27-28)
Labels: P0, component:websocket, size:xl
Related:
- #211: WebSocket org scoping
- #212: Org context plumbing
- #209: WebSocket tests
```

## Milestones

Current milestones:
- **v2.0-beta.1** (2025-12-14): Critical security & testing
- **v2.1** (2026-Q1): Plugin system enhancements
- **v3.0** (2026-H2): Multi-cloud support

**Milestone management:**
```bash
# List open issues for milestone
gh issue list --milestone "v2.0-beta.1" --state open

# Move issue to different milestone
gh issue edit 212 --milestone "v2.0-beta.1"

# Check milestone progress
gh api repos/streamspace-dev/streamspace/milestones/1 | jq '.{title, open_issues, closed_issues}'
```

## Reports & Dashboards

### Wave Status (Manual)
```bash
# Count issues in Wave 27
gh issue list --search "label:wave:27 state:open" | wc -l

# Count by agent in Wave 27
gh issue list --search "label:wave:27 label:agent:builder state:open" | wc -l
```

### Velocity Tracking
```bash
# Issues closed in last 7 days
gh issue list --search "state:closed closed:>=2025-11-19" | wc -l

# Average resolution time (manual review needed)
gh issue list --search "state:closed" --limit 20 | jq -r '.[] | "\(.number): \(.createdAt) → \(.closedAt)"'
```

## Best Practices

1. **Issue Hygiene**
   - One issue per feature/bug (no mega-issues)
   - Clear acceptance criteria (no ambiguity)
   - Link dependencies immediately
   - Close inactive issues after 30 days

2. **Wave Planning**
   - Plan waves 1 week ahead
   - Assign all issues before wave starts
   - Conduct daily standups during wave
   - Retrospective at wave end

3. **Communication**
   - Use issue comments for decisions (not Slack/Discord)
   - Link related PRs/issues
   - Close issues with summary comment
   - Update blockers daily

4. **Metric Tracking**
   - Velocity (issues/wave)
   - Cycle time (creation to close)
   - Coverage trends
   - Bug escape rate

## Integration with Zencoder Rules

- **Agent workflows**: Each agent follows their role in `.zencoder/rules/agent-*.md`
- **Testing standards**: All work must meet `.zencoder/rules/testing-standards.md`
- **Git workflow**: Commits follow `.zencoder/rules/git-workflow.md`
- **Security**: P0 issues reference `.zencoder/rules/p0-security-hardening.md`

## References

- [StreamSpace CONTRIBUTING.md](CONTRIBUTING.md)
- [Zencoder Agent Workflows](.zencoder/rules/)
- [Wave Planning Template](.github/ISSUE_TEMPLATE/03-wave-planning.md)
