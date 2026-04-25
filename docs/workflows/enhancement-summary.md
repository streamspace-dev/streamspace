---
title: GitHub Workflow Enhancement Summary
description: Overview of new workflow structure for Wave-based multi-agent development
---

# GitHub Workflow Enhancement Summary

**Date**: 2025-11-26  
**Status**: ✅ Implemented  
**Owner**: @architect

---

## What Changed

Your GitHub workflow has been **transformed from ad-hoc issue management to structured wave-based planning** aligned with the Zencoder multi-agent framework. This brings clarity, automation, and scalability to your multi-team coordination.

### Before → After

| Aspect | Before | After |
|--------|--------|-------|
| **Planning** | Manual MULTI_AGENT_PLAN.md | Automated wave milestones (#223-#225) |
| **Issue Status** | Labels defined but unused | Active workflow states (ready-for-testing, status:blocked, etc.) |
| **Wave Tracking** | No formal structure | 2-3 day cycles with daily standups |
| **Dependencies** | Manual notes in comments | Linked issues + status:blocked labels |
| **Assignments** | Inconsistent | Mandatory: agent + size + wave before work starts |
| **Templates** | Basic issue template | 3 comprehensive templates with DoR/DoD |
| **Automation** | None | GitHub Actions auto-labels PRs by wave |
| **Documentation** | Scattered | Centralized in github-workflow.md |
| **Visibility** | Hard to see progress | Wave milestones show real-time status |

---

## What You Get

### 1. Wave-Based Organization ✅

**Created 3 Wave Planning Issues:**
- **Wave 27** (#223): Org Context & Security Hardening (11/26-11/28) - **IN PROGRESS**
- **Wave 28** (#224): Testing & Release Prep (11/29-12/01) - PLANNED
- **Wave 29** (#225): Performance & Stability (12/02-12/05) - PLANNED

Each wave:
- ✅ Links all issues being worked
- ✅ Contains DoD checklist
- ✅ Tracks daily standup progress
- ✅ Identifies & manages blockers
- ✅ Retrospective template for lessons learned

**View current wave**: [wave-planning.md](wave-planning.md)

### 2. Enhanced Issue Templates ✅

Three new templates in `.github/ISSUE_TEMPLATE/`:

**01-feature-request.md**
- Summary + problem statement
- Scope & components
- Clear acceptance criteria
- DoR checklist ensures issues are ready before work starts

**02-bug-report.md**
- Structured reproduction steps
- Environment + component info
- Severity assessment
- Triage checklist for reviewers

**03-wave-planning.md**
- Wave overview (timeline, focus)
- Issue links
- DoD checkboxes for Builder/Validator/Scribe/Architect
- Daily standup template
- Blocker management section
- Retrospective template

### 3. GitHub Actions Automation ✅

New workflow: `.github/workflows/wave-tracking.yml`

**Auto-Labeling:**
- PRs auto-labeled by wave when merged
- `ready-for-testing` label auto-applied when PR closes
- Notification comments posted when issue moves to testing

**Wave Status Reporting:**
- Generate wave progress snapshots (manual trigger: comment `/wave-status`)
- Notification when all wave issues completed

### 4. Comprehensive Workflow Guide ✅

**New file**: [github-workflow.md](github-workflow.md)

Complete reference covering:
- Wave lifecycle & structure
- Issue workflow (create → triage → build → test → merge)
- Label system explained
- Commands quick reference
- Common patterns (breaking changes, security issues, epics)
- Milestone management
- Reporting & dashboards

### 5. Wave Planning Dashboard ✅

**New file**: [wave-planning.md](wave-planning.md)

At-a-glance view of:
- Current wave status
- Issues in each wave (table with agent/size/status)
- DoD checklists for each role
- Daily progress tracker
- Blocker mitigation plan
- Velocity metrics
- Integration schedule
- Adjustment protocol (if wave falls behind)

---

## How to Use It

### For Architects (Wave Planning)

**Each morning:**
```bash
# Review current wave
open wave-planning.md

# Check standup comments on wave issue
gh issue view 223

# List today's issues
gh issue list --search "label:wave:27" --state open
```

**Create new wave (every 2-3 days):**
```bash
# Use template
gh issue create --title "Wave 28: ..." \
  --label "agent:architect" \
  --milestone "v2.0-beta.1" \
  --body "$(cat .github/ISSUE_TEMPLATE/03-wave-planning.md)"
```

**Daily standup:**
1. Check wave issue for blocker comments
2. Run quick velocity check: `gh issue list --search "label:wave:27" --state closed | wc -l`
3. Post daily update to wave issue (use template from wave planning doc)

### For Builders (Implementation)

**When issue assigned:**
```bash
# Verify issue has DoR met (agent + size + wave labels)
gh issue view 212 | grep -E "agent:|size:|wave:"

# Create feature branch
git checkout -b feature/issue-212-org-context

# Work normally, commit with semantic messages
git commit -m "feat(auth): add org_id extraction to JWT"
```

**When ready for testing:**
```bash
# Verify tests pass
make fmt lint test

# Open PR
gh pr create --title "feat: add org_id extraction to JWT" \
  --body "Closes #212. All tests passing (78% coverage)."

# Comment on issue
gh issue comment 212 --body "✅ Implementation complete. Ready for validation. See PR #XXX"

# Add label
gh issue edit 212 --add-label "ready-for-testing"
```

### For Validators (Testing)

**When issue ready-for-testing:**
```bash
# Review acceptance criteria
gh issue view 212

# Fetch and test PR
gh pr checkout <pr-number>
make test  # Run full test suite

# If bug found
gh issue create --title "[BUG] Cross-org access not rejected" \
  --label "bug,P1,component:backend" \
  --body "Found while testing #212..."

# If validation passes
gh issue comment 212 --body "✅ VALIDATION PASSED
- Acceptance criteria verified ✓
- Integration tests passing ✓
- Coverage: 78% ✓

Ready for master merge."
```

### For Scribes (Documentation)

**Work in parallel with Builder:**
```bash
# Review issue acceptance criteria
gh issue view 212

# Start documentation
git checkout -b feature/issue-212-docs
vi CHANGELOG.md
vi docs/MULTI_TENANCY.md

# Commit
git commit -m "docs(multi-tenancy): add org context setup guide"
git push origin feature/issue-212-docs
```

**When docs complete:**
```bash
# Comment on issue
gh issue comment 212 --body "📝 Documentation complete
- CHANGELOG.md updated
- docs/MULTI_TENANCY.md created
- SECURITY.md org isolation section added

Ready for merge."
```

### For Everyone (Daily Workflow)

**Morning:**
1. Open [wave-planning.md](wave-planning.md)
2. Check current wave number
3. Review your assigned issues from that wave
4. Start work on highest-priority unblocked issue

**Mid-day:**
5. Post standup comment to wave issue (template in wave-planning.md)
6. Identify any blockers
7. Communicate blockers to @architect

**End of day:**
8. Push commits to feature branch
9. Update issue status if work complete

**Wave end (every 2-3 days):**
10. Prepare issue for merge (make sure DoD met)
11. Wait for architect merge to master
12. Close issue
13. Retrospective (what went well, what to improve)

---

## New Labels & Their Usage

### Workflow State Labels

| Label | When to Use | Who Applies |
|-------|------------|-------------|
| `wave:27` | Issue is in Wave 27 work | Architect (during triage) |
| `ready-for-testing` | Builder finished, ready for Validator | Builder or GitHub Actions |
| `status:blocked` | Issue waiting on another issue | Any (when blocker found) |
| `status:in-review` | Validation complete, ready for master | Validator |

### Agent Labels

| Label | Meaning |
|-------|---------|
| `agent:architect` | Assigned to Architect (wave planning, triage, merge) |
| `agent:builder` | Assigned to Builder (implementation) |
| `agent:validator` | Assigned to Validator (testing, QA) |
| `agent:scribe` | Assigned to Scribe (documentation) |
| `agent:security` | Assigned to Security (vulnerability assessment) |

### Existing Labels (Now More Systematic)

- **Priority**: P0 (critical) → P1 (high) → P2 (medium) → P3 (low)
- **Size**: xs (<2h) → s (2-4h) → m (4-8h) → l (1-2d) → xl (2-5d)
- **Component**: backend, ui, k8s-agent, docker-agent, infrastructure, database, websocket
- **Risk**: breaking (requires migration), high (regression risk)
- **Status**: needs-security-review, needs-testing

---

## Example Workflow: Issue #212

### Day 1: Triage (Architect)

```bash
# Review new issue
gh issue view 212

# Issue has:
# - Clear summary ✓
# - Acceptance criteria ✓
# - Component mapping ✓

# Assign for Wave 27
gh issue edit 212 --add-label "agent:builder,size:l,wave:27,component:backend,P0"

# Comment on issue
gh issue comment 212 --body "✅ Triaged for Wave 27 (2025-11-26 → 2025-11-28)
- Assigned to: @builder
- Size: Large (1-2 days)
- Priority: P0 (critical for release)
- Dependencies: None

Work can begin immediately."
```

### Days 1-2: Implementation (Builder)

```bash
# Create feature branch
git checkout -b feature/issue-212-org-context

# Implement, test, commit
git commit -m "feat(auth): add org_id to JWT claims"
git commit -m "feat(middleware): extract org_id from JWT to context"
git commit -m "test(auth): add org_id validation tests"

# Push and create PR
git push origin feature/issue-212-org-context
gh pr create --title "feat: add org_id extraction to JWT" \
  --body "Closes #212. Implements org context in JWT claims.

## Changes
- JWT now includes org_id claim
- Auth middleware extracts org_id to Gin context
- All handlers can access org context

## Tests
- Unit tests: 12 new tests
- Coverage: 78% (target: 70%+) ✓
- All tests passing locally

## Checklist
- [x] Tests passing
- [x] Code reviewed locally
- [x] CHANGELOG entry drafted
- [ ] Security review (waiting)
- [ ] Validator sign-off (waiting)"

# Comment on issue
gh issue comment 212 --body "✅ Implementation complete. Ready for validation. See PR #XXX"
gh issue edit 212 --add-label "ready-for-testing"
```

**GitHub Actions Auto-Labels PR #XXX:**
- Adds label `wave:27` (from issue)
- Adds label `agent:builder` (from issue)
- Posts comment: "🔄 PR ready-for-testing!"

### Day 2: Testing (Validator)

```bash
# Check issue
gh issue view 212

# Run tests
gh pr checkout <pr-number>
make fmt lint test

# All tests pass ✓
# Acceptance criteria met ✓

# Validate
gh issue comment 212 --body "✅ VALIDATION PASSED
- Acceptance criteria verified ✓
- Integration tests passing ✓
- Coverage: 78% (target: 70%+) ✓
- Security review: Not required (auth work) ✓

Ready for master merge."

gh issue edit 212 --remove-label "ready-for-testing" --add-label "status:in-review"
```

### Days 1-2: Documentation (Scribe, parallel)

```bash
# Create docs branch
git checkout -b feature/issue-212-docs

# Update CHANGELOG
echo "- Multi-tenancy org context support (#212)" >> CHANGELOG.md

# Create new docs
cat > docs/MULTI_TENANCY.md << 'EOF'
# Multi-Tenancy Setup

## Overview
JWT tokens now include org_id for multi-tenant support.

## Configuration
...
EOF

# Commit
git commit -m "docs(multi-tenancy): add org context setup guide"
git push origin feature/issue-212-docs

# Comment
gh issue comment 212 --body "📝 Documentation complete
- CHANGELOG.md updated
- docs/MULTI_TENANCY.md created

Ready for merge."
```

### Day 3: Integration (Architect)

```bash
# Verify all done
gh issue view 212

# Labels show:
# - ready-for-testing ✓
# - status:in-review ✓
# - agent:builder ✓
# - wave:27 ✓

# Check master gates
gh run list --workflow "test.yml" | head -5  # All green ✓

# Merge (in order)
git checkout master
git pull origin master

git merge --ff-only origin/claude/v2-scribe   # Docs
git merge --ff-only origin/claude/v2-builder  # Implementation

git push origin master

# Close issue
gh issue close 212 --comment "✅ Merged to master. Complete.

Final stats:
- Timeline: 2 days (planned: 1-2 days) ✓
- Coverage: 78% (target: 70%+) ✓
- All tests passing ✓
- Ready for Wave 28"
```

---

## Immediate Action Items

### For Architect (This Week)

- [ ] Review Wave 27 issues (#223)
- [ ] Conduct daily standups (see template in wave-planning.md)
- [ ] Update wave issue with daily progress
- [ ] Identify & escalate blockers
- [ ] Plan Wave 28 issues by end of Wave 27

### For All Agents

- [ ] Read [github-workflow.md](github-workflow.md) (30 min)
- [ ] Bookmark [wave-planning.md](wave-planning.md) (check daily)
- [ ] Review your assigned issues from [Wave 27 #223](https://github.com/streamspace-dev/streamspace/issues/223)
- [ ] Ensure your issues have all required labels (agent + size + wave)

### For Developers

- [ ] Use new templates when creating issues
- [ ] Apply issue labels according to [github-workflow.md](github-workflow.md)
- [ ] Comment on issue when moving to next stage
- [ ] Use wave issue for daily standup

---

## Success Metrics

After 2 waves, measure:

1. **Clarity**
   - Can each agent see their work? (✅ Wave label visible)
   - Is status clear? (✅ Workflow state labels)
   - Are blockers visible? (✅ Status:blocked label)

2. **Velocity**
   - Issues completed per wave?
   - Cycle time (days from creation to close)?
   - Blocker ratio (<10% target)?

3. **Quality**
   - Test coverage maintained?
   - Security reviews completed?
   - No regressions?

4. **Adoption**
   - All new issues use templates?
   - All issues labeled before work starts?
   - Daily standups posted?

---

## Files Created/Modified

**New Files:**
- ✅ `.github/ISSUE_TEMPLATE/01-feature-request.md`
- ✅ `.github/ISSUE_TEMPLATE/02-bug-report.md`
- ✅ `.github/ISSUE_TEMPLATE/03-wave-planning.md`
- ✅ `.github/workflows/wave-tracking.yml`
- ✅ `github-workflow.md`
- ✅ `wave-planning.md`
- ✅ `enhancement-summary.md` (this file)

**Existing Files to Review:**
- `CONTRIBUTING.md` (link to new templates)
- `.zencoder/rules/agent-architect.md` (wave planning section)
- `.zencoder/README.md` (link to github-workflow.md)

---

## Next Steps

1. **This Week**:
   - Wave 27 execution (org context + security)
   - Daily standups
   - Blockers resolution

2. **Wave 27 End (11/28)**:
   - Retrospective
   - Merge to master
   - Plan Wave 28

3. **Continuous**:
   - Monitor velocity metrics
   - Refine wave length (2 vs 3 days)
   - Adjust labels/templates as needed
   - Update documentation

---

## Questions?

- **Workflow questions**: See [github-workflow.md](github-workflow.md)
- **Current wave**: See [wave-planning.md](wave-planning.md)
- **Issue templates**: Check `.github/ISSUE_TEMPLATE/`
- **Automation**: Review `.github/workflows/wave-tracking.yml`
- **Agent roles**: See `.zencoder/rules/agent-*.md`

---

**Owner**: @architect  
**Last Updated**: 2025-11-26  
**Review Cycle**: Every wave (2-3 days)
