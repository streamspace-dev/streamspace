# Update GitHub Issue

Update a GitHub issue with progress, findings, or status changes.

**Use this when**: You want to update an issue without signaling it's complete.

## Usage

Provide the issue number when running this command.

Example: `/update-issue 200` (for Issue #200)

## What This Does

 1. **Fetch current issue**: Run `gh issue view <number>` to get context.
 2. **Prompt for update type**:
    - Progress update
    - Blocker found
    - Question for issue creator
    - Additional findings
    - Status change
 3. **Post comment**: Use `gh issue comment <number> --body "..."`
 4. **Update labels**: Use `gh issue edit <number> --add-label "..."` (if needed)
 5. **Log in MULTI_AGENT_PLAN.md**: Update the plan file directly.

## Update Types

### 1. Progress Update

```markdown
## 🔄 Progress Update

**Agent**: Builder
**Date**: 2025-11-23

### Completed
- Fixed API handler test mock expectations
- Updated response assertions

### In Progress
- Investigating PostgreSQL array type handling
- Debugging SQL pattern matching

### Blocked By
None

**Estimated Completion**: 4 hours
```

### 2. Blocker Found

```markdown
## 🚨 Blocker Discovered

**Agent**: Builder
**Severity**: P0

### Issue
Cannot proceed with test fixes due to missing test database setup.

### Impact
- Blocks all API handler test fixes
- Blocks Issue #200 completion

### Proposed Solution
1. Create test database configuration
2. Add mock PostgreSQL setup
3. Update test initialization

**Needs**: Architect review and priority decision
```

### 3. Question

```markdown
## ❓ Question for Clarification

**Agent**: Validator

### Question
Should we test with Redis HA (3-node cluster) or single Redis instance?

### Context
Issue #134 validation - testing multi-pod AgentHub

### Options
A. Single Redis (faster, simpler)
B. Redis HA (production-like, slower)

**Awaiting**: Decision from Architect or issue creator
```

### 4. Findings

```markdown
## 🔍 Additional Findings

**Agent**: Validator

### Discovery
While testing Issue #200, discovered 3 additional failing test files:
1. `sessions_test.go` - 5 failures
2. `templates_test.go` - 3 failures
3. `auth_test.go` - 2 failures

### Recommendation
Expand scope of Issue #200 or create separate issues?

**Impact on Timeline**: +4 hours if included in #200
```

## Interactive Prompts

1. **Issue number**: Which issue to update?
2. **Update type**: Progress/Blocker/Question/Findings/Status
3. **Details**: Specific information based on type
4. **Label changes**: Should any labels be added/removed?
5. **Notify others**: Tag other agents or users?
