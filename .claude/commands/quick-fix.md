# Quick Fix

Create a quick bug fix with automated commit, push, and issue update.

**Use this when**: Fixing a small, isolated bug (< 50 lines changed).

## Usage

Provide issue number: `/quick-fix 165`

Or describe the fix: `/quick-fix "Add missing security headers"`

## What This Does

1. **Interactive Fix Session**:
   - Shows the issue details
   - Helps you identify files to fix
   - Guides you through the changes
   - Reviews your changes

2. **Quality Checks**:
   - Runs `/verify-all` (tests, lint, format)
   - Ensures no breaking changes
   - Validates related tests pass

3. **Automated Commit & Push**:
   - Generates semantic commit message
   - Commits to your agent branch
   - Pushes to remote

4. **Issue Management**:
   - Posts update comment with fix details
   - Adds `ready-for-testing` label
   - Notifies Validator if needed
   - Links commit SHA

## Quick Fix Criteria

A fix is eligible for `/quick-fix` if:
- ✅ Changes < 50 lines
- ✅ Single file or closely related files
- ✅ No breaking changes
- ✅ Tests already exist (or not needed)
- ✅ Low risk of side effects

If your fix doesn't meet these criteria, use normal workflow instead.

## Example Flow

```bash
# You run the command
/quick-fix 165

# It fetches the issue
Fetching Issue #165: Add Security Headers Middleware...

Title: [SECURITY] Add Security Headers Middleware
Priority: P0
Component: Backend API
Agent: Builder

# It guides you through the fix
Files to modify:
1. api/internal/middleware/security.go (create new)
2. api/cmd/main.go (add middleware)

Proceed? [y/n]: y

# You make the changes with guidance
# Then it validates

Running quality checks...
✅ Tests pass (go test ./...)
✅ Linting clean (golangci-lint)
✅ Formatting clean (gofmt)

# It commits and pushes
Creating commit...
✅ Committed: fix(security): Add security headers middleware (#165)
✅ Pushed to claude/v2-builder

# It updates the issue
✅ Comment added to Issue #165
✅ Label added: ready-for-testing
✅ Validator notified

Done! Issue #165 ready for testing.
```

## Generated Commit Message

Automatically follows semantic commit format:

```
fix(security): Add security headers middleware (#165)

Added security headers middleware to API:
- X-Content-Type-Options: nosniff
- X-Frame-Options: DENY
- X-XSS-Protection: 1; mode=block
- Strict-Transport-Security: max-age=31536000

Resolves #165

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## When NOT to Use

Don't use `/quick-fix` for:
- ❌ Changes > 50 lines
- ❌ Multiple unrelated files
- ❌ Breaking changes
- ❌ Requires new tests
- ❌ Complex refactoring
- ❌ Database migrations

For these cases, use the standard workflow with manual commits.

## Benefits

- **Speed**: Fix small bugs in minutes
- **Consistency**: Standardized commit messages
- **Automation**: No manual commit/push/update
- **Quality**: Automatic validation before push
- **Tracking**: Issue automatically updated
