---
name: "🐛 Bug Report"
about: "Report a bug with clear reproduction steps"
title: "[BUG] "
labels: ["bug", "needs-triage"]
assignees: []
---

<!-- ============================================
     PROVIDE CLEAR REPRODUCTION STEPS
     Poor bug reports get closed immediately
     ============================================ -->

## Summary (REQUIRED)
<!-- One-sentence description of the bug -->


## Environment (REQUIRED)
- **OS**: 
- **Go Version**: `go version`
- **Node Version**: `node --version`
- **Docker**: Yes / No
- **Deployment**: Local / K8s / Docker Compose


## Steps to Reproduce (REQUIRED)
<!-- Clear, numbered steps to reproduce the issue -->
1. 
2. 
3. 


## Expected Behavior (REQUIRED)
<!-- What should happen? -->


## Actual Behavior (REQUIRED)
<!-- What actually happened? -->


## Logs & Error Output
<!-- Provide full error messages, stack traces, log excerpts -->
```
Paste logs here
```


## Screenshots
<!-- If UI-related, add screenshots of the bug -->


## Affected Component (REQUIRED)
- [ ] Backend API (`api/`)
- [ ] React UI (`ui/`)
- [ ] K8s Agent (`agents/`)
- [ ] Docker Agent
- [ ] Infrastructure (Helm, Terraform)
- [ ] Other:


## Severity Assessment (REQUIRED)
- [ ] **P0** - System down / security issue / blocks release
- [ ] **P1** - Major functionality broken
- [ ] **P2** - Minor bug / workaround exists
- [ ] **P3** - Nice to fix


## Workaround
<!-- Is there a temporary workaround? -->


## Possible Root Cause
<!-- If you have ideas on the cause, share them -->


## References
<!-- Related issues, PRs, docs -->


---

### For Triage Team

**Triage Checklist:**
- [ ] Severity assessed (add P0/P1/P2/P3 label)
- [ ] Component tagged (backend, ui, etc.)
- [ ] Reproduce steps are clear and complete
- [ ] No sensitive information in logs (check passwords, tokens, IPs)
- [ ] Assign to relevant agent (builder for fix, validator for test)
- [ ] Link to related issues/PRs

**If this is security-related:** Add label `security` and notify @streamspace-dev/security-team
