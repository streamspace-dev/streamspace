## Description

<!-- Provide a brief description of the changes in this PR -->

## ⚠️ REQUIRED: Related Issues

<!-- Link to related issues using "Closes #123" or "Relates to #456" -->
<!-- THIS IS REQUIRED - PRs must be linked to an issue for tracking -->
Closes #

**✅ Requirement Check:**
- [ ] This PR is linked to at least one issue (required for merge)

## Type of Change

<!-- Check all that apply -->
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Refactoring (no functional changes)
- [ ] Performance improvement
- [ ] Test coverage improvement

## Component

<!-- Check the primary component affected -->
- [ ] UI (Frontend/React)
- [ ] Backend (API/Go)
- [ ] K8s Agent
- [ ] Docker Agent
- [ ] Database
- [ ] WebSocket
- [ ] VNC Proxy
- [ ] Plugin System
- [ ] Documentation

## Changes Made

<!-- Describe the changes in detail -->

### Files Modified
- `path/to/file.go` - Description of changes
- `path/to/other.tsx` - Description of changes

### Key Changes
1.
2.
3.

## Testing

<!-- Describe the testing you've done -->

### Unit Tests
- [ ] Unit tests added/updated
- [ ] All unit tests pass
- [ ] Code coverage maintained/improved (target: 80%+)

### Integration Tests
- [ ] Integration tests added/updated
- [ ] All integration tests pass
- [ ] E2E flow validated

### Manual Testing
- [ ] Manual testing completed
- [ ] Tested in development environment
- [ ] Tested edge cases

### Test Results
<!-- Paste test output or describe test results -->
```
go test ./... -v
# OR
npm test
```

## Screenshots (if UI changes)

<!-- Add screenshots showing before/after if this affects the UI -->

## Performance Impact

<!-- Describe any performance implications -->
- [ ] No performance impact
- [ ] Performance improved
- [ ] Performance degraded (explain why acceptable)

## Documentation

<!-- Check all that apply -->
- [ ] Code comments added/updated
- [ ] API documentation updated
- [ ] User documentation updated
- [ ] README updated (if needed)
- [ ] CHANGELOG updated

## Security Considerations

<!-- Describe any security implications -->
- [ ] No security impact
- [ ] Security improved
- [ ] New authentication/authorization added
- [ ] Input validation added
- [ ] SQL injection prevention verified
- [ ] XSS prevention verified

## Database Changes

<!-- If this PR includes database changes -->
- [ ] No database changes
- [ ] Migration script included (`api/migrations/XXX_description.sql`)
- [ ] Migration tested locally
- [ ] Migration is backwards compatible
- [ ] Rollback plan documented

## Deployment Notes

<!-- Special deployment considerations -->
- [ ] No special deployment requirements
- [ ] Requires configuration changes (document below)
- [ ] Requires database migration
- [ ] Requires service restart
- [ ] Breaking changes (document migration path)

### Configuration Changes
<!-- If configuration changes are needed, document them -->
```yaml
# New environment variables:
NEW_VAR: value
```

## Risk Assessment

<!-- Evaluate the risk level of this change -->
- [ ] Low risk (isolated change, well-tested)
- [ ] Medium risk (affects multiple components)
- [ ] High risk (core functionality change)
- [ ] Breaking change (requires migration)

**If High Risk or Breaking:**
- [ ] Added `risk:high` or `risk:breaking` label
- [ ] Migration guide included (if breaking)
- [ ] Extra testing completed
- [ ] Rollback plan documented

## Checklist

<!-- Ensure all items are checked before requesting review -->
- [ ] **✅ Linked to issue(s)** (REQUIRED - see above)
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] No new warnings introduced
- [ ] Tests pass locally
- [ ] Documentation is clear and complete
- [ ] Commit messages follow convention (`feat:`, `fix:`, `docs:`, etc.)
- [ ] PR title is clear and descriptive
- [ ] Branch is up to date with base branch
- [ ] Applied appropriate labels (component, priority, agent, risk)

## Agent Workflow (for multi-agent development)

<!-- For Agent 2 (Builder) - comment on related issue when PR is opened -->
**Builder Agent Checklist:**
- [ ] Commented on issue #XXX when starting work
- [ ] Code implements requirements from issue
- [ ] All acceptance criteria met
- [ ] Ready for Validator (Agent 3) testing

<!-- For Agent 3 (Validator) - validation results -->
**Validator Agent Checklist:**
- [ ] All tests pass
- [ ] No regressions detected
- [ ] Performance validated
- [ ] Ready to merge

## Reviewer Notes

<!-- Any specific areas you'd like reviewers to focus on -->

