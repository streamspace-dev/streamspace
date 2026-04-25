# StreamSpace Project Management Guide

**Last Updated**: 2025-11-23
**Version**: 2.0

Complete guide to managing the StreamSpace project using GitHub's project management features.

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [GitHub Actions (Automation)](#github-actions-automation)
3. [Issue Templates](#issue-templates)
4. [Branch Protection](#branch-protection)
5. [Code Owners](#code-owners)
6. [Labels & Organization](#labels--organization)
7. [Saved Queries](#saved-queries)
8. [Workflows & Processes](#workflows--processes)
9. [Best Practices](#best-practices)

---

## Overview

StreamSpace uses a comprehensive GitHub-based project management system with:

- **GitHub Issues** - All task tracking
- **GitHub Projects** - Kanban board visualization
- **GitHub Milestones** - Release planning
- **GitHub Actions** - Automated workflows
- **Branch Protection** - Code quality enforcement
- **CODEOWNERS** - Automatic reviewer assignment

### Quick Links

- **Project Board**: https://github.com/orgs/streamspace-dev/projects/2
- **Milestones**: https://github.com/streamspace-dev/streamspace/milestones
- **All Issues**: https://github.com/streamspace-dev/streamspace/issues
- **Saved Queries**: `.github/SAVED_QUERIES.md`

---

## GitHub Actions (Automation)

### 1. Auto-Label PRs (`.github/workflows/auto-label.yml`)

**What it does**: Automatically labels PRs based on files changed

**When it runs**: On PR open, synchronize, reopen

**Examples**:
- Changes to `ui/**` → adds `component:ui` label
- Changes to `api/**` → adds `component:backend` label
- Changes to `**/*.md` → adds `documentation` label

**Configuration**: `.github/labeler.yml`

### 2. Add Issues to Project (`.github/workflows/add-to-project.yml`)

**What it does**: Automatically adds new issues to the project board

**When it runs**: When an issue is opened

**Benefit**: No manual step needed - all issues tracked automatically

### 3. Weekly Status Report (`.github/workflows/weekly-report.yml`)

**What it does**: Generates automated weekly status report

**When it runs**:
- Every Monday at 9 AM UTC (automated)
- Manual trigger available (workflow_dispatch)

**Report includes**:
- Milestone progress percentages
- Issues completed this week
- P0 critical issues
- Blocked issues

**Output**: Creates a new issue with `status-report` label

### 4. Stale Issue Management (`.github/workflows/stale-issues.yml`)

**What it does**: Marks inactive issues/PRs as stale

**When it runs**: Daily at midnight UTC

**Timeline**:
- After 30 days of inactivity → marked as `stale`
- After 7 more days → automatically closed
- Exemptions: P0, status:blocked, enhancement issues

**Purpose**: Keeps issue list clean and actionable

---

## Issue Templates

### Bug Report (`.github/ISSUE_TEMPLATE/bug_report.yml`)

**Use for**: Comprehensive bug reports

**Required fields**:
- Severity (P0/P1/P2)
- Component affected
- Bug description
- Steps to reproduce

**Optional fields**:
- Error message
- Expected/actual behavior
- Suggested fix
- Estimated effort

### Quick Bug Report (`.github/ISSUE_TEMPLATE/quick_bug.yml`)

**Use for**: Simple, obvious bugs

**Required fields**:
- Where (page/endpoint)
- What's wrong
- Severity

**When to use**:
- Simple bugs with obvious fix
- Don't need detailed reproduction steps
- Quick issues that need fast triage

### Feature Request (`.github/ISSUE_TEMPLATE/feature_request.yml`)

**Use for**: New features or enhancements

**Required fields**:
- Priority
- Component
- Problem statement
- Proposed solution
- Target milestone
- Acceptance criteria

**Optional fields**:
- Alternatives considered
- UI mockup
- Estimated effort

### Agent Task (`.github/ISSUE_TEMPLATE/agent_task.yml`)

**Use for**: Architect assigning work to agents

**Required fields**:
- Assigned agent (Builder/Validator/Scribe)
- Priority
- Milestone
- Task objective
- Requirements
- Acceptance criteria

**Purpose**: Structured task assignment for multi-agent workflow

### Performance Issue (`.github/ISSUE_TEMPLATE/performance_issue.yml`)

**Use for**: Performance regressions or optimization opportunities

**Required fields**:
- Component
- Performance issue description
- Current performance metrics
- Target performance
- Acceptance criteria

**Optional fields**:
- Profiling data
- Proposed solution

### Sprint Planning (`.github/ISSUE_TEMPLATE/sprint_planning.yml`)

**Use for**: Weekly/bi-weekly sprint planning (Architect only)

**Required fields**:
- Sprint start/end dates
- Primary milestone
- Sprint goals
- Agent task assignments
- Success criteria

**Optional fields**:
- Risks & dependencies
- Team capacity

---

## Branch Protection

### Main Branch Protection

**Configured via**: GitHub API

**Rules enforced**:
1. **Require PR reviews**: 1 approval required before merge
2. **Dismiss stale reviews**: Re-review after new commits
3. **Require conversation resolution**: All review comments must be resolved
4. **No force pushes**: Prevents history rewriting
5. **No deletions**: Prevents accidental branch deletion

**Benefits**:
- Code quality maintained
- No accidental direct commits to main
- All changes go through review process
- Git history remains clean

**How to merge**:
1. Create feature branch
2. Make changes and commit
3. Open PR to `main`
4. Get 1 approval
5. Resolve all comments
6. Merge (squash & merge recommended)

---

## Code Owners (`.github/CODEOWNERS`)

### What is CODEOWNERS?

Auto-assigns reviewers based on files changed in a PR.

### Teams Defined

- `@streamspace-dev/maintainers` - Overall project owners
- `@streamspace-dev/frontend-team` - UI/React code
- `@streamspace-dev/backend-team` - API/Go code
- `@streamspace-dev/agent-team` - K8s/Docker agents
- `@streamspace-dev/devops-team` - Infrastructure/deployments
- `@streamspace-dev/docs-team` - Documentation
- `@streamspace-dev/qa-team` - Testing

### Example Auto-Assignment

**If you change**:
- `ui/src/pages/Sessions.tsx` → @streamspace-dev/frontend-team
- `api/internal/handlers/sessions.go` → @streamspace-dev/backend-team
- `api/migrations/005_add_column.sql` → @streamspace-dev/backend-team + @streamspace-dev/maintainers
- `README.md` → @streamspace-dev/docs-team

### Security-Sensitive Files

**Extra scrutiny** (require maintainer review):
- `.github/workflows/**`
- `api/internal/auth/**`
- `api/internal/security/**`
- Database migrations

---

## Labels & Organization

### Agent Assignment
- `agent:architect` - Coordination/planning tasks
- `agent:builder` - Implementation work
- `agent:validator` - Testing tasks
- `agent:scribe` - Documentation tasks

### Priority
- `P0` - Critical (blocks release/production)
- `P1` - High (important feature broken)
- `P2` - Low (minor issue)

### Size/Effort
- `size:xs` - < 2 hours
- `size:s` - 2-4 hours
- `size:m` - 4-8 hours
- `size:l` - 1-2 days
- `size:xl` - 2-5 days

### Status
- `status:blocked` - Blocked by another issue
- `status:in-review` - PR awaiting review
- `stale` - No recent activity

### Risk Management
- `risk:high` - High risk of causing issues
- `risk:breaking` - Breaking change (requires migration)
- `needs:testing` - Needs extra testing
- `needs:security-review` - Requires security review

### Component
- `component:ui`, `component:backend`, `component:database`
- `component:k8s-agent`, `component:docker-agent`
- `component:websocket`, `component:vnc-proxy`
- `component:plugin-system`

### Type
- `bug`, `enhancement`, `documentation`, `testing`
- `performance`, `sprint-planning`, `status-report`

### Community
- `good-first-issue` - Good for newcomers
- `help-wanted` - Community help wanted

---

## Saved Queries

See `.github/SAVED_QUERIES.md` for comprehensive list of saved searches.

### Most Useful Queries

**For Builder**:
- My Work Queue: `is:open label:agent:builder sort:created-asc`
- P0 Critical: `is:open label:agent:builder label:P0`
- Quick Wins: `is:open label:agent:builder label:size:xs,size:s`

**For Validator**:
- My Testing Queue: `is:open label:agent:validator`
- Needs Testing: `is:open label:needs:testing`

**For Scribe**:
- My Docs: `is:open label:agent:scribe`
- Completed This Week: `is:closed closed:>=2025-11-18`

**For Architect**:
- Current Sprint: `is:open milestone:v2.0-beta.1 sort:priority-desc`
- Blocked Issues: `is:open label:status:blocked`
- High Risk: `is:open label:risk:high`

---

## Workflows & Processes

### Issue Lifecycle

```
1. Issue Created → Auto-added to project board (Todo column)
2. Agent Assigned → Label added (agent:builder, etc.)
3. Work Starts → Comment added, move to "In Progress"
4. PR Created → Links to issue ("Closes #123")
5. PR Reviewed → Auto-labeled by file changes
6. PR Merged → Issue automatically closed, move to "Done"
7. Scribe Updates → CHANGELOG.md updated
```

### Agent Workflow

**Builder (Agent 2)**:
1. Check open issues at session start
2. Comment when starting work
3. Create PR when ready
4. Link PR to issue
5. Comment when complete

**Validator (Agent 3)**:
1. Create issues for all bugs found
2. Add test results as comments
3. Close when validated
4. Label with priority/component

**Scribe (Agent 4)**:
1. Check for closed issues
2. Update CHANGELOG.md
3. Update affected docs
4. Comment on issues when documented

**Architect (Agent 1)**:
1. Create feature issues
2. Assign to agents & milestones
3. Monitor progress via project board
4. Triage new issues
5. Generate sprint plans

### Sprint Planning Process

**Weekly (Every Monday)**:
1. Automated status report generated (GitHub Action)
2. Architect reviews report
3. Create sprint planning issue (use template)
4. Assign tasks to agents
5. Update milestone goals
6. Monitor progress daily via project board

### PR Review Process

1. **Open PR**: Auto-labeled, reviewers auto-assigned (CODEOWNERS)
2. **CI Runs**: Tests must pass
3. **Review**: 1 approval required
4. **Resolve Comments**: All conversations must be resolved
5. **Merge**: Squash & merge to main
6. **Auto-Close**: Linked issues close automatically

---

## Best Practices

### For Issue Creation

✅ **DO**:
- Use appropriate template
- Fill in all required fields
- Link related issues
- Add labels (priority, component, agent)
- Assign to milestone
- Write clear acceptance criteria

❌ **DON'T**:
- Create issues without templates
- Leave required fields empty
- Create duplicate issues
- Skip milestone assignment

### For Pull Requests

✅ **DO**:
- Link to issue(s) - **REQUIRED**
- Fill out PR template completely
- Write clear commit messages (`feat:`, `fix:`, `docs:`)
- Add tests for new code
- Update documentation
- Request review from appropriate team
- Apply risk labels if applicable

❌ **DON'T**:
- Create PR without linked issue
- Skip tests
- Commit directly to main
- Ignore review comments
- Force push after review started

### For Labels

✅ **DO**:
- Apply agent label immediately
- Set priority (P0/P1/P2)
- Add component labels
- Include size estimate
- Mark risks (high/breaking)

❌ **DON'T**:
- Leave issues unlabeled
- Use wrong priority
- Forget size estimates

### For Milestones

✅ **DO**:
- Assign to appropriate milestone
- Review milestone progress weekly
- Move issues if priorities change
- Close milestone when all issues complete

❌ **DON'T**:
- Leave issues without milestone
- Overload a single milestone
- Create milestones without due dates

### For Communication

✅ **DO**:
- Comment when starting work
- Update status regularly
- Use @mentions for urgent items
- Mark issues as blocked if stuck
- Comment when complete with details

❌ **DON'T**:
- Work silently without updates
- Leave blocked issues unmarked
- Skip completion comments

---

## Metrics & Reporting

### Velocity Tracking

**Measure**:
- Issues closed per week
- Story points completed
- Cycle time (open → close)

**Formula**:
```
Velocity = Closed Issues / Week
Cycle Time = Close Date - Open Date (average)
Throughput = Issues Closed / Sprint
```

### Milestone Health

**Track**:
- Open vs. closed ratio
- Days until due date
- P0 issues remaining
- Blocked issue count

**Red Flags**:
- > 50% open 1 week before due date
- Multiple P0 issues unresolved
- Increasing blocked count

### Team Capacity

**Estimate effort**:
- XS = 1 point, S = 2, M = 3, L = 5, XL = 8
- Sum points for milestone
- Divide by weeks available
- Compare to team velocity

**Example**:
```
v2.0-beta.1:
- 4 issues: 3 × M (9 points) + 1 × XL (8 points) = 17 points
- 3 weeks available
- Required velocity: 17 / 3 = ~6 points/week
```

---

## Troubleshooting

### Issue Not Added to Project

**Problem**: New issue not on project board

**Solution**: GitHub Action may be pending. Check:
1. `.github/workflows/add-to-project.yml` status
2. Manually add: `gh project item-add 2 --owner streamspace-dev --url <issue-url>`

### PR Not Auto-Labeled

**Problem**: PR opened but no component labels

**Solution**:
1. Check `.github/workflows/auto-label.yml` ran successfully
2. Verify `.github/labeler.yml` has matching patterns
3. Manually apply labels if needed

### Stale Bot Closed Important Issue

**Problem**: Issue closed due to inactivity but still needed

**Solution**:
1. Reopen the issue
2. Add `status:blocked` or `P0` label (exempt from stale bot)
3. Add comment explaining why it's still relevant

### Can't Merge PR

**Problem**: Merge button disabled

**Common causes**:
1. No approval → Get review
2. Failing checks → Fix tests
3. Unresolved conversations → Resolve comments
4. Branch out of date → Rebase/merge main

---

## Quick Reference Card

### Common Commands

```bash
# Create issue (with milestone)
gh issue create --repo streamspace-dev/streamspace \
  --label "bug,P1,agent:builder" \
  --milestone "v2.0-beta.1"

# Add issue to project
gh project item-add 2 --owner streamspace-dev \
  --url https://github.com/streamspace-dev/streamspace/issues/123

# List my issues
gh issue list --repo streamspace-dev/streamspace \
  --assignee @me --state open

# Close issue
gh issue close 123 --repo streamspace-dev/streamspace \
  --comment "Fixed in #456"

# Create PR
gh pr create --repo streamspace-dev/streamspace \
  --base main --head feature-branch \
  --title "feat: Add new feature" \
  --body "Closes #123"
```

### Keyboard Shortcuts (GitHub Web)

- `c` - Create new issue
- `g` + `i` - Go to issues
- `g` + `p` - Go to pull requests
- `/` - Focus search bar
- `?` - Show all shortcuts

---

## Support & Resources

**Documentation**:
- This guide: `.github/PROJECT_MANAGEMENT_GUIDE.md`
- Saved queries: `.github/SAVED_QUERIES.md`
- Agent instructions: `.claude/multi-agent/agent*-instructions.md`

**GitHub Docs**:
- [Issues](https://docs.github.com/en/issues)
- [Projects](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
- [Actions](https://docs.github.com/en/actions)
- [Branch Protection](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches)

**Questions?**
- Create issue with `question` label
- Tag `@streamspace-dev/maintainers`
- Check project board discussions

---

**Last Updated**: 2025-11-23 | **Version**: 2.0
