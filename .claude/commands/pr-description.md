# Generate Pull Request Description

Generate comprehensive PR description from branch commits.

!git log main..HEAD --oneline
!git diff main...HEAD --stat

Create PR description with the following structure:

## Summary
[High-level overview of changes - what and why]

## Changes
**API Backend**:
- [Bullet points of API changes]

**K8s Agent**:
- [Bullet points of K8s agent changes]

**Docker Agent**:
- [Bullet points of Docker agent changes]

**UI**:
- [Bullet points of UI changes]

**Tests**:
- [Test coverage changes]
- [New tests added]

**Documentation**:
- [Documentation updates]

## Testing Performed
- [ ] Unit tests passing
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Tested on: [K8s cluster / Docker / local]

## Performance Impact
- [Session creation time]
- [Resource usage]
- [Any performance improvements/degradations]

## Breaking Changes
- [List any breaking changes or "None"]

## Migration Notes
- [Database migrations required]
- [Configuration changes needed]
- [Or "None required"]

## Checklist
- [ ] Tests passing
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] No breaking changes (or documented above)
- [ ] Reviewed by: [Agent name or "Ready for review"]

## Related Issues
Closes #[issue number]
Relates to #[issue number]

---

🤖 Generated with [Claude Code](https://claude.com/claude-code)
