# Review Pull Request

Perform automated code review on a pull request using the `@pr-reviewer` subagent.

**Use this when**: Reviewing PRs from other agents or external contributors before merging.

## Usage

Provide the PR number when running this command.

Example: `/review-pr 42`

## What This Does

Launches the `@pr-reviewer` subagent to perform comprehensive review:

### Code Quality Checks

- **Go**: gofmt, golint, go vet, ineffassign, staticcheck
- **TypeScript**: ESLint, TypeScript compiler, unused imports
- **General**: Code duplication, complexity, naming conventions

### Security Analysis

- **SQL Injection**: Unsafe query construction
- **XSS**: Unescaped output, dangerous HTML
- **Secrets**: Hardcoded credentials, API keys
- **Auth**: Missing auth checks, insecure sessions
- **RBAC**: Permission bypasses

### Performance Review

- **Database**: N+1 queries, missing indexes, inefficient queries
- **Caching**: Missing cache opportunities
- **Memory**: Potential leaks, inefficient allocations
- **Algorithms**: Inefficient loops, unnecessary work

### Testing & Documentation

- **Test Coverage**: Are tests included?
- **Test Quality**: Edge cases, mocks, assertions
- **Documentation**: Comments, README, API docs
- **Breaking Changes**: Documented and justified?

## Output Format

Creates review report in `.claude/reports/PR_REVIEW_<number>_<date>.md`:

```markdown
# PR Review #42: Add rate limiting middleware

## Summary
- **Status**: ✅ APPROVED / ⚠️ NEEDS CHANGES / ❌ REJECTED
- **Reviewer**: @pr-reviewer subagent
- **Date**: 2025-11-23
- **Files Changed**: 5 (+234/-12 lines)

## Findings

### P0 CRITICAL (Must Fix)
1. **SQL Injection Risk** (security.go:45)
   - Using string concatenation for SQL query
   - Recommendation: Use parameterized queries

### P1 HIGH PRIORITY (Should Fix)
2. **Missing Test Coverage** (rate_limiter.go)
   - No tests for error handling paths
   - Recommendation: Add test cases for Redis failures

### P2 MEDIUM PRIORITY (Consider Fixing)
3. **Performance**: Inefficient loop (handler.go:120)
   - Iterating entire slice when map lookup would work
   - Recommendation: Use map for O(1) lookup

### P3 LOW PRIORITY (Nice to Have)
4. **Documentation**: Missing godoc comment (middleware.go:15)

## Recommendations

1. Fix P0 issue (SQL injection) before merge
2. Add tests for error paths
3. Consider performance optimization
4. Update documentation

## Approval Status
⚠️ **NEEDS CHANGES** - P0 security issue must be fixed
```

## GitHub Integration

The command will use `gh` CLI to interact with the PR:

1. **Fetch PR details**: `gh pr view <number>`
2. **Fetch PR diff**: `gh pr diff <number>`
3. **Post review**:
   - Comment: `gh pr review <number> --comment --body "..."`
   - Request Changes: `gh pr review <number> --request-changes --body "..."`
   - Approve: `gh pr review <number> --approve --body "..."`
4. **Add labels**: `gh pr edit <number> --add-label "security-review-needed"`

## Follow-up Actions

After review:

- **P0 Issues**: PR blocked until fixed
- **P1 Issues**: Should fix before merge
- **P2/P3 Issues**: Can merge, fix in follow-up
- **Clean PR**: Approved for merge by Architect
