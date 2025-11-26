---
title: Zencoder Quick Start Guide
description: How to use Zencoder rules to work on StreamSpace
---

# 🚀 Zencoder Quick Start Guide

**What**: Zencoder is a rules engine that tells the AI assistant how to work on your project.  
**Where**: Rules live in `.zencoder/rules/` and are auto-applied to every interaction.  
**Why**: Ensures consistent patterns, standards, and workflows across all agents.

---

## TL;DR - Get Started in 30 Seconds

### **Three Ways to Work**

```bash
# 1. As a specific agent (easiest)
"@builder: Implement issue #212"

# 2. Reference a GitHub issue (recommended)
"Work on issue #212 (Org context and RBAC plumbing)"

# 3. Check your wave work (best for teams)
"I'm Builder in Wave 27. What should I work on?"
```

That's it. I'll automatically:
- ✅ Understand your role from `.zencoder/rules/agent-*.md`
- ✅ Know the codebase from `.zencoder/rules/repo.md`
- ✅ Follow coding patterns from `.zencoder/rules/coding-standards.md`
- ✅ Write tests per `.zencoder/rules/testing-standards.md`
- ✅ Commit properly per `.zencoder/rules/git-workflow.md`

---

## What Zencoder Rules Cover

| Rule File | What It Controls | Used For |
|-----------|------------------|----------|
| **agent-architect.md** | Wave planning, triage, integration | When you act as Architect |
| **agent-builder.md** | Implementation, code patterns, TDD | When you act as Builder |
| **agent-validator.md** | Testing, QA, security testing | When you act as Validator |
| **agent-scribe.md** | Documentation, CHANGELOG, readability | When you act as Scribe |
| **agent-security.md** | Vulnerability assessment, compliance | When you act as Security |
| **coding-standards.md** | Go + React style, naming, patterns | Writing code |
| **testing-standards.md** | Table-driven tests, >70% coverage | Writing tests |
| **git-workflow.md** | Branches, semantic commits, merge order | Git operations |
| **documentation-standards.md** | Writing style, document structure | Writing docs |
| **p0-security-hardening.md** | Multi-tenancy implementation guide | P0 security work |
| **repo.md** | Project structure, languages, dependencies | Understanding codebase |

---

## How to Use Zencoder

### **Option 1: Agent Commands (Easiest)**

Tell me which agent role you are:

```
"@builder: Implement JWT org_id extraction for issue #212"
```

I'll automatically:
1. Load `agent-builder.md` rules
2. Understand the codebase from `repo.md`
3. Follow Go patterns from `coding-standards.md`
4. Write tests per `testing-standards.md`
5. Commit correctly per `git-workflow.md`

**Other agent commands:**
```bash
@validator: Test issue #212 (org context implementation). PR is ready.
@scribe: Update CHANGELOG for issue #212
@architect: Plan Wave 28 after Wave 27 completes
@security: Audit issue #211 for cross-org vulnerabilities
```

### **Option 2: Issue-Based Workflow (Recommended)**

Reference a GitHub issue and let me handle it:

```
"Work on issue #212 (Org context and RBAC plumbing for API and WebSockets)"
```

I will:
1. Read issue #212 acceptance criteria
2. Understand components affected (from `repo.md`)
3. Create feature branch: `feature/issue-212-org-context`
4. Implement following Go patterns (`coding-standards.md`)
5. Write table-driven tests (`testing-standards.md`)
6. Ensure >70% coverage
7. Commit with semantic messages (`git-workflow.md`)
8. Signal "ready-for-testing"

**Works best with:**
- GitHub issue number: `#212`
- Issue title or description
- Mention your role: "As Builder, work on..."

### **Option 3: Wave-Based Coordination (Best for Teams)**

Tell me your wave and role:

```
"I'm Builder in Wave 27 (11/26-11/28). What should I work on?"
```

I will:
1. Check `WAVE_PLANNING.md` for current wave
2. Find your assigned issues
3. Show unblocked issues in order
4. Explain your Definition of Done (DoD)
5. Guide you through each issue
6. Track progress in wave issue comments

---

## Quick Examples

### **Example 1: Implement a Feature**

```
You: "@builder: Implement issue #212 JWT org_id extraction"

Me: I will:
  ✓ Read coding-standards.md Go patterns
  ✓ Create Go service with org_id claims
  ✓ Write table-driven tests (testing-standards.md)
  ✓ Ensure >70% coverage
  ✓ Commit: "feat(auth): add org_id extraction to JWT"
  ✓ Push to feature/issue-212-org-context
  ✓ Signal ready-for-testing
```

### **Example 2: Test Implementation**

```
You: "@validator: Test issue #212 implementation. PR #XXX is ready."

Me: I will:
  ✓ Review acceptance criteria from #212
  ✓ Check code against coding-standards.md
  ✓ Run tests from testing-standards.md
  ✓ Verify >70% coverage
  ✓ Test cross-org rejection (security-hardening.md)
  ✓ Comment "✅ VALIDATION PASSED"
```

### **Example 3: Update Documentation**

```
You: "@scribe: Update docs for issue #212 (org context)"

Me: I will:
  ✓ Add entry to CHANGELOG.md
  ✓ Create docs/MULTI_TENANCY.md
  ✓ Update SECURITY.md org section
  ✓ Follow documentation-standards.md style
  ✓ Test all links work
```

### **Example 4: Check Your Work**

```
You: "Review this code against coding-standards.md"

Me: I will:
  ✓ Check Go handler pattern
  ✓ Verify tests are table-driven
  ✓ Confirm >70% coverage
  ✓ Check semantic commit message
  ✓ Verify no secrets hardcoded
  ✓ Flag any issues
```

---

## Common Commands

### **Understanding the Project**

```bash
# Explain repo structure
"What's in the StreamSpace repo?"

# Find where to add code
"Where should I add org_id validation?"

# See similar patterns
"Show me a similar handler pattern"
```

### **Working on Issues**

```bash
# Get oriented
"I'm Builder in Wave 27. What's my work?"

# Understand issue scope
"Explain issue #212 acceptance criteria"

# Check dependencies
"What's blocking issue #211?"
```

### **Following Standards**

```bash
# Learn patterns
"Show me the Go handler pattern"
"What's the table-driven test pattern?"

# Verify work
"Does this follow coding-standards.md?"
"Is this test pattern correct?"

# Check requirements
"What test coverage is required?"
"How should I commit this?"
```

### **Building & Testing**

```bash
# Run tests
"Run all tests with coverage"

# Verify standards
"Check if this passes lint and format"

# Generate reports
"Show test coverage report"
```

### **Wave & Team Work**

```bash
# Get organized
"What's my work in Wave 27?"
"Show me my Definition of Done for this wave"

# Coordinate
"Check if #212 is blocked on anything"
"Mark #211 as blocked by #212"

# Track progress
"Generate wave status report"
```

---

## Workflow Example: Complete Task

### **Morning: Get Started**

```
You: "I'm Builder in Wave 27. Show me what to work on."

Me: 
  ✓ Check WAVE_PLANNING.md
  ✓ Show Wave 27 issues
  ✓ Recommend: Start with #212 (blocker for #211)
  ✓ Show Builder DoD checklist
```

### **Work: Implement**

```
You: "Implement JWT org_id extraction. Show me the pattern."

Me:
  ✓ Show Go handler pattern from coding-standards.md
  ✓ Show service pattern
  ✓ Show test pattern
  ✓ Implement #212
  ✓ Write tests (table-driven, >70% coverage)
  ✓ Commit with semantic message
```

### **End: Signal Ready**

```
You: "I'm done with #212. Update the issue."

Me:
  ✓ Add ready-for-testing label
  ✓ Post summary to issue #212
  ✓ Link PR
  ✓ Notify Validator role
```

### **Next: Validator Tests**

```
You (as Validator): "Test PR #XXX for issue #212"

Me:
  ✓ Verify acceptance criteria
  ✓ Run tests
  ✓ Check coverage >70%
  ✓ Test security (cross-org rejection)
  ✓ Comment "✅ VALIDATION PASSED"
```

---

## Key Files You Need to Know

### **Zencoder Rules** (`.zencoder/rules/`)
These auto-apply to every interaction:
- `agent-*.md`: Agent-specific workflows
- `coding-standards.md`: Code patterns
- `testing-standards.md`: Test requirements
- `repo.md`: Project structure

### **Workflow Documentation** (Root)
Created for Wave-based development:
- `WAVE_PLANNING.md`: Current wave + daily standup template
- `GITHUB_WORKFLOW.md`: Complete workflow reference
- `WORKFLOW_ENHANCEMENT_SUMMARY.md`: Overview

### **Quick Reference** (Right here!)
- `QUICK_START.md`: This file

---

## Best Practices

### ✅ Do This

**Be Specific About Your Role**
```
✓ "@builder: Implement issue #212"
✓ "I'm Builder in Wave 27"
✗ "Fix the org context thing"
```

**Reference Issues**
```
✓ "Work on issue #212 (Org context...)"
✓ "Issue #212: Add org_id to JWT"
✗ "Add org_id"
```

**Ask About Patterns First**
```
✓ "Show me the Go handler pattern"
✓ "What's the table-driven test pattern?"
✗ "Just write the code"
```

**Request Verification**
```
✓ "Verify this against coding-standards.md"
✓ "Does this follow testing-standards.md?"
✗ "Is this good?"
```

**Use Templates**
```
✓ "Post daily standup for Wave 27" (uses template)
✓ "Generate wave status"
✗ "What's happening?"
```

### ❌ Don't Do This

**Vague Requests**
```
✗ "Fix the code"
✗ "Make it better"
✗ "Add something"
```

**Skip Understanding Patterns**
```
✗ Jump to coding without learning standards
✗ Write tests without seeing examples
✗ Commit without understanding message format
```

**Ignore Blockers**
```
✗ Work on blocked issues instead of unblocked
✗ Skip dependencies
✗ Don't link related issues
```

**Deviate from Workflow**
```
✗ Skip testing, skip docs, skip commits
✗ Work outside waves without reason
✗ Change standards without consensus
```

---

## Daily Routine

### **Morning**
1. Open `WAVE_PLANNING.md`
2. Check current wave number
3. Find your assigned unblocked issues
4. Start work on highest priority

### **During Work**
1. Create feature branch
2. Follow patterns from `coding-standards.md`
3. Write tests per `testing-standards.md`
4. Commit with semantic messages
5. Push regularly

### **When Complete**
1. Add `ready-for-testing` label
2. Post summary to issue
3. Notify Validator
4. Move to next issue

### **Daily Standup**
1. Check `WAVE_PLANNING.md` standup template
2. Post to wave issue (e.g., #223)
3. Include: what done, what today, blockers

### **Wave End (Every 2-3 Days)**
1. Wrap up remaining issues
2. Complete retrospective in wave issue
3. Prepare next wave
4. Merge to master

---

## Cheat Sheet

### **Quick Commands**

| Goal | What to Say |
|------|------------|
| **Get oriented** | "I'm Builder in Wave 27. Show my work." |
| **Learn pattern** | "Show me the Go handler pattern" |
| **Implement** | "@builder: Implement issue #212" |
| **Test** | "@validator: Test PR #XXX" |
| **Document** | "@scribe: Update docs for #212" |
| **Verify work** | "Review this against coding-standards.md" |
| **Check wave** | "What's the current wave status?" |
| **Signal ready** | "I'm done with #212. Update issue." |

### **When You're Done With An Issue**

```
1. Make sure tests pass: make test
2. Verify coverage: >70%
3. Commit with semantic message
4. Push to feature branch
5. Say: "@validator: Issue #212 ready for testing. See PR #XXX"
6. I'll add ready-for-testing label
```

### **When Testing An Issue**

```
1. Check acceptance criteria from issue
2. Run tests
3. Verify >70% coverage
4. Test security if applicable
5. Comment: "✅ VALIDATION PASSED" or file bug
```

### **When Merging To Master**

```
1. Verify all DoD checks passed
2. Merge in order: Scribe → Builder → Validator
3. Close issue with summary
4. Update wave progress
```

---

## Troubleshooting

### **"I'm not sure what to work on"**
→ Check `WAVE_PLANNING.md` for your wave  
→ List your assigned issues  
→ Start with highest priority unblocked issue

### **"I don't know how to implement this"**
→ Ask: "Show me the pattern for [feature]"  
→ Review `coding-standards.md` for examples  
→ Look at similar code in codebase

### **"What test pattern should I use?"**
→ Ask: "What's the table-driven test pattern?"  
→ Review `testing-standards.md`  
→ Look at existing tests in `*_test.go` files

### **"How should I commit this?"**
→ Review `git-workflow.md`  
→ Use format: `feat(scope): message` or `fix(scope): message`  
→ Example: `feat(auth): add org_id extraction to JWT`

### **"Is this code correct?"**
→ Ask: "Review against [standard]"  
→ Options: `coding-standards.md`, `testing-standards.md`, `git-workflow.md`

### **"What's blocking issue #211?"**
→ Check issue #211 for `status:blocked` label  
→ Look for dependency comments  
→ Usually: #211 blocked by #212

---

## Key Concepts

### **Agents** (5 roles)
- **Architect**: Planning, triage, integration, wave coordination
- **Builder**: Implementation, features, bug fixes, code
- **Validator**: Testing, QA, security audits, verification
- **Scribe**: Documentation, CHANGELOG, communication, readability
- **Security**: Vulnerability assessment, compliance, security testing

### **Waves** (2-3 day cycles)
- **Wave 27** (11/26-11/28): Org Context & Security (NOW)
- **Wave 28** (11/29-12/01): Testing & Release Prep (NEXT)
- **Wave 29** (12/02-12/05): Performance & Stability
- Each wave has DoD (Definition of Done) checklist

### **Workflow States** (Labels)
- `wave:27`: Issue is in Wave 27 work
- `ready-for-testing`: Builder complete, Validator tests next
- `status:blocked`: Waiting on another issue
- `status:in-review`: Validation complete, ready to merge

### **Standards** (Auto-applied)
- Coding patterns (Go handlers, React components)
- Test patterns (table-driven, >70% coverage)
- Commit format (semantic messages)
- Documentation style (CHANGELOG, README, docs)

---

## Need Help?

### **Understanding Zencoder**
→ Read the full explanation in system-reminder (chat history)

### **Workflow Details**
→ See `GITHUB_WORKFLOW.md` (comprehensive reference)

### **Current Wave Status**
→ Check `WAVE_PLANNING.md` (daily dashboard)

### **Code Patterns**
→ Review `.zencoder/rules/coding-standards.md`

### **Test Patterns**
→ Review `.zencoder/rules/testing-standards.md`

### **Git Workflow**
→ Review `.zencoder/rules/git-workflow.md`

### **Agent Responsibilities**
→ Review `.zencoder/rules/agent-*.md`

### **P0 Security Work**
→ Review `.zencoder/rules/p0-security-hardening.md`

---

## Start Now

Pick one:

```bash
# Option 1: Get oriented
"I'm Builder in Wave 27. What should I work on?"

# Option 2: Start an issue
"@builder: Implement issue #212"

# Option 3: Learn patterns
"Show me the Go handler pattern"
```

That's it. Everything else follows from Zencoder rules.

---

**Last Updated**: 2025-11-26  
**Owner**: @architect  
**Location**: `/streamspace/QUICK_START.md`

Have fun! 🚀
