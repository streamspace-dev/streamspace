---
title: Workflow Documentation
description: Guides for GitHub workflow, wave planning, and Zencoder integration
---

# Workflow Documentation

**Purpose**: Everything you need to work on StreamSpace using GitHub waves, Zencoder rules, and multi-agent coordination.

---

## Quick Navigation

### **New to StreamSpace?**
Start here → **[Zencoder Quick Start](zencoder-quick-start.md)** (5 min read)
- What is Zencoder?
- Three ways to work
- Common commands
- Daily routine

### **Managing Your Wave?**
Dashboard & tracking → **[Wave Planning](wave-planning.md)** (reference)
- Current wave status
- Daily standup template
- DoD checklists per role
- Blocker management

### **GitHub Issue Workflow?**
Complete reference → **[GitHub Workflow](github-workflow.md)** (30 min read)
- Issue lifecycle
- Labels explained
- Automation setup
- Commands reference

### **Understand the Enhancement?**
Overview & context → **[Enhancement Summary](enhancement-summary.md)** (background)
- What changed
- Before/after comparison
- Key improvements
- Integration points

---

## File Overview

| File | Purpose | Audience | Read Time |
|------|---------|----------|-----------|
| **zencoder-quick-start.md** | How to use Zencoder rules to work | Everyone | 5 min |
| **wave-planning.md** | Current wave dashboard + standup | Daily users | 2 min |
| **github-workflow.md** | GitHub issue management + automation | GitHub users | 30 min |
| **enhancement-summary.md** | What's new and why | Stakeholders | 10 min |

---

## Common Workflows

### **Starting Work**

1. Read: [Zencoder Quick Start](zencoder-quick-start.md)
2. Check: [Wave Planning](wave-planning.md) for your current wave
3. Pick issue from your wave
4. Say: `"@builder: Implement issue #212"` (or your role)

### **During Work**

Follow patterns from `.zencoder/rules/`:
- Code patterns → `coding-standards.md`
- Test patterns → `testing-standards.md`
- Git workflow → `git-workflow.md`
- Your role → `agent-*.md`

Track progress:
- Update issue with progress comments
- Post daily standup to wave issue
- Link PRs and dependencies

### **Completing Work**

1. Tests passing, coverage >70%
2. Code follows standards
3. Commit with semantic message
4. Signal: `"ready-for-testing"` or mention @validator

### **End of Wave**

1. Complete retrospective in wave issue
2. Merge to master
3. Plan next wave
4. Update [Wave Planning](wave-planning.md)

---

## Key Concepts

### **Agents** (5 roles)
- **Architect**: Planning, triage, integration, wave coordination
- **Builder**: Implementation, features, code
- **Validator**: Testing, QA, security audits
- **Scribe**: Documentation, CHANGELOG, communication
- **Security**: Vulnerability assessment, compliance

**Read more**: `.zencoder/rules/agent-*.md`

### **Waves** (2-3 day cycles)
- **Wave 27** (11/26-11/28): Org Context & Security - NOW
- **Wave 28** (11/29-12/01): Testing & Release Prep - NEXT
- **Wave 29** (12/02-12/05): Performance & Stability - PLANNED

**Read more**: [Wave Planning](wave-planning.md)

### **Workflow States** (GitHub Labels)
- `wave:27`: Issue is in Wave 27 work
- `ready-for-testing`: Builder complete, Validator tests next
- `status:blocked`: Waiting on another issue
- `status:in-review`: Validation complete, ready to merge

**Read more**: [GitHub Workflow](github-workflow.md)

### **Definition of Ready (DoR)**
Before starting work, issue must have:
- ✅ Clear acceptance criteria
- ✅ Agent assigned (builder, validator, scribe, architect, security)
- ✅ Size estimated (xs, s, m, l, xl)
- ✅ Wave assigned (wave:27, wave:28, etc.)
- ✅ Component labeled (backend, ui, infrastructure, etc.)

**Read more**: [GitHub Workflow - Definition of Ready](github-workflow.md#definition-of-ready)

### **Definition of Done (DoD)**
Role-specific checklists before issue closes:

**Builder DoD**:
- [ ] Implementation complete
- [ ] Tests written (table-driven)
- [ ] Coverage >70%
- [ ] Code reviewed locally
- [ ] Semantic commit messages
- [ ] Ready label added

**Validator DoD**:
- [ ] Acceptance criteria verified
- [ ] All tests passing
- [ ] Coverage maintained
- [ ] Security review (if P0)
- [ ] No regressions
- [ ] Validation passed comment posted

**Scribe DoD**:
- [ ] CHANGELOG.md updated
- [ ] README.md reflects status
- [ ] docs/ files created/updated
- [ ] Breaking changes documented
- [ ] Links verified

**Architect DoD**:
- [ ] All agent work complete
- [ ] Master integration gates passing
- [ ] Wave completed on schedule
- [ ] Retrospective documented

**Read more**: [Wave Planning - Definition of Done](wave-planning.md)

---

## Zencoder Rules (Auto-Applied)

These rules automatically apply to all work:

| Rule | Controls | When Used |
|------|----------|-----------|
| `agent-architect.md` | Planning, triage, integration | Acting as Architect |
| `agent-builder.md` | Implementation patterns | Acting as Builder |
| `agent-validator.md` | Testing requirements | Acting as Validator |
| `agent-scribe.md` | Documentation standards | Acting as Scribe |
| `agent-security.md` | Security testing | Acting as Security |
| `coding-standards.md` | Go + React patterns | Writing code |
| `testing-standards.md` | Test patterns, coverage | Writing tests |
| `git-workflow.md` | Branches, commits, merges | Using Git |
| `documentation-standards.md` | Writing style | Writing docs |
| `p0-security-hardening.md` | Multi-tenancy guide | P0 security work |
| `repo.md` | Project structure | Understanding codebase |

**Location**: `.zencoder/rules/`  
**Auto-applied**: Yes (YAML frontmatter with `alwaysApply: true`)

---

## Common Questions

### **"How do I use Zencoder?"**
→ Start with [Zencoder Quick Start](zencoder-quick-start.md)  
→ Pick one command:
- `"@builder: Implement issue #212"`
- `"I'm Validator in Wave 27. What should I test?"`
- `"Show me the Go handler pattern"`

### **"What should I work on?"**
→ Check [Wave Planning](wave-planning.md)  
→ Find your wave  
→ Pick highest priority unblocked issue

### **"How do I know what to do?"**
→ Read issue acceptance criteria  
→ Check your role's DoD checklist in [Wave Planning](wave-planning.md)  
→ Ask: `"@validator: Test issue #212"`

### **"Where's my definition of done?"**
→ [Wave Planning](wave-planning.md) has role-specific DoD checklists  
→ Or ask: `"Show me Builder DoD for Wave 27"`

### **"How do I commit this?"**
→ Review [Git Workflow](github-workflow.md#commit-guidelines)  
→ Format: `feat(scope): message`  
→ Example: `feat(auth): add org_id extraction to JWT`

### **"What test pattern should I use?"**
→ Check `.zencoder/rules/testing-standards.md`  
→ Or ask: `"Show me the table-driven test pattern"`

### **"Is this code correct?"**
→ Ask: `"Review against coding-standards.md"`  
→ Or: `"Does this follow Go handler pattern?"`

### **"What's blocking issue #211?"**
→ Check [Wave Planning](wave-planning.md)  
→ Look for dependency notes  
→ Usually: Check issue #211 for `status:blocked` label

---

## Resources by Role

### **Architects**
- [Wave Planning](wave-planning.md) - daily dashboard
- `.zencoder/rules/agent-architect.md` - role guide
- [GitHub Workflow](github-workflow.md) - issue triage

### **Builders**
- [Zencoder Quick Start](zencoder-quick-start.md) - get started
- `.zencoder/rules/coding-standards.md` - code patterns
- `.zencoder/rules/agent-builder.md` - workflow

### **Validators**
- [Zencoder Quick Start](zencoder-quick-start.md) - get started
- `.zencoder/rules/testing-standards.md` - test patterns
- `.zencoder/rules/agent-validator.md` - workflow

### **Scribes**
- [Zencoder Quick Start](zencoder-quick-start.md) - get started
- `.zencoder/rules/documentation-standards.md` - writing guide
- `.zencoder/rules/agent-scribe.md` - workflow

### **Security**
- `.zencoder/rules/agent-security.md` - workflow
- `.zencoder/rules/p0-security-hardening.md` - P0 guide
- [GitHub Workflow](github-workflow.md) - issue tracking

---

## Integration Points

### **Within Workflows**
```
Zencoder Quick Start
    ↓
Wave Planning (current work)
    ↓
GitHub Workflow (how to track)
    ↓
.zencoder/rules/ (detailed standards)
```

### **With Project**
```
.zencoder/rules/ (auto-applied standards)
    ↓
docs/workflows/ (workflow guides) ← YOU ARE HERE
    ↓
.github/ISSUE_TEMPLATE/ (issue templates)
    ↓
CONTRIBUTING.md (contribution guidelines)
```

---

## Getting Help

| Question | Answer Location |
|----------|-----------------|
| What is Zencoder? | [Zencoder Quick Start - TL;DR](zencoder-quick-start.md#tldr---get-started-in-30-seconds) |
| How do I work on this project? | [Zencoder Quick Start - How to Use](zencoder-quick-start.md#how-to-use-zencoder) |
| What's my work in this wave? | [Wave Planning](wave-planning.md) |
| How do I track progress? | [GitHub Workflow](github-workflow.md) |
| What code patterns do I use? | `.zencoder/rules/coding-standards.md` |
| What test patterns do I use? | `.zencoder/rules/testing-standards.md` |
| How do I commit? | `.zencoder/rules/git-workflow.md` |
| What's my role's workflow? | `.zencoder/rules/agent-*.md` |
| How do I document? | `.zencoder/rules/documentation-standards.md` |
| What's new in the workflow? | [Enhancement Summary](enhancement-summary.md) |

---

## Quick Links

- **Rules**: `.zencoder/rules/`
- **Templates**: `.github/ISSUE_TEMPLATE/`
- **Contributing**: `CONTRIBUTING.md`
- **Issues**: [GitHub Issues](https://github.com/streamspace-dev/streamspace/issues)
- **Project**: [StreamSpace](https://github.com/streamspace-dev/streamspace)

---

## Navigation

**Start here for new developers:**
1. [Zencoder Quick Start](zencoder-quick-start.md) (5 min)
2. [Wave Planning](wave-planning.md) (reference as needed)
3. Ask: `"@builder: Implement issue #212"`

**Keep handy:**
- [Zencoder Quick Start - Cheat Sheet](zencoder-quick-start.md#cheat-sheet)
- [GitHub Workflow - Commands Reference](github-workflow.md#commands-quick-reference)
- [Wave Planning - Daily Routine](wave-planning.md#daily-routine)

---

**Last Updated**: 2025-11-26  
**Owner**: @architect  
**Location**: `docs/workflows/README.md`
