# Fast Agent Integration (Token-Optimized)

**Purpose:** Quickly integrate agent updates WITHOUT reading all test files.
**Use When:** Regular wave integrations (not bug investigations).
**Architect Only:** This command is for Agent 1 (Architect) use only.

---

## Step 1: Check for Updates

```bash
git fetch origin claude/v2-scribe claude/v2-builder claude/v2-validator
```

## Step 2: Quick Diff Summary (Stats Only)

```bash
echo "=== Scribe Updates ==="
git log --oneline feature/streamspace-v2-agent-refactor..origin/claude/v2-scribe

echo -e "\n=== Builder Updates ==="
git log --oneline feature/streamspace-v2-agent-refactor..origin/claude/v2-builder

echo -e "\n=== Validator Updates ==="
git log --oneline feature/streamspace-v2-agent-refactor..origin/claude/v2-validator
```

## Step 3: Get Stats (NO file reads)

```bash
echo "=== Scribe Changes ==="
git diff --stat feature/streamspace-v2-agent-refactor origin/claude/v2-scribe

echo -e "\n=== Builder Changes ==="
git diff --stat feature/streamspace-v2-agent-refactor origin/claude/v2-builder

echo -e "\n=== Validator Changes ==="
git diff --stat feature/streamspace-v2-agent-refactor origin/claude/v2-validator
```

## Step 4: Merge in Order (Scribe → Builder → Validator)

```bash
# Scribe first (docs)
git merge origin/claude/v2-scribe --no-edit -m "merge: Wave X integration - Scribe (docs)"

# Builder second (code)
git merge origin/claude/v2-builder --no-edit -m "merge: Wave X integration - Builder (code)"

# Validator last (tests)
git merge origin/claude/v2-validator --no-edit -m "merge: Wave X integration - Validator (tests)"
```

## Step 5: Update MULTI_AGENT_PLAN (Summary Only)

**DO NOT read old waves** - just add new wave summary at top:

```markdown
### 📦 Integration Wave X - [Title] (2025-11-23)

**Integration Date:** 2025-11-23
**Integrated By:** Agent 1 (Architect)
**Status:** ✅ COMPLETE

**Integration Summary:**
- **Files Changed**: X files
- **Lines Added**: +X
- **Lines Removed**: -X
- **Merge Strategy**: 3-way merge (Scribe → Builder → Validator)
- **Conflicts**: None/Resolved

**Changes Integrated:**
- Scribe: [brief summary]
- Builder: [brief summary]
- Validator: [brief summary]

**Impact:**
- [Key achievements]
- [Issues closed if any]
```

## Step 6: Commit & Push

```bash
git add .claude/multi-agent/MULTI_AGENT_PLAN.md
git commit -m "merge: Wave X integration - [brief description]"
git push origin feature/streamspace-v2-agent-refactor
```

---

## 🚫 What NOT to Do (Token Waste)

❌ DO NOT read test files unless investigating bugs
❌ DO NOT read all changed files - trust `git diff --stat`
❌ DO NOT read historical waves in MULTI_AGENT_PLAN
❌ DO NOT read archived reports in `.claude/reports/archive/`

## ✅ What TO Do (Efficient)

✅ Use `git log --oneline` for commit messages
✅ Use `git diff --stat` for change summary
✅ Read ONLY the top of MULTI_AGENT_PLAN to add new wave
✅ Read specific files ONLY if investigating bugs/conflicts

---

## Token Optimization Tips

- **Historical waves** → `.claude/multi-agent/WAVE_HISTORY.md` (don't read)
- **Old reports** → `.claude/reports/archive/` (don't read)
- **Test files** → Only read when debugging failures
- **MULTI_AGENT_PLAN** → Only read/edit top section (current wave)

---

**Estimated Tokens:** <5,000 (vs 60,000+ with old method)
**Time Saved:** ~90% reduction in token usage
