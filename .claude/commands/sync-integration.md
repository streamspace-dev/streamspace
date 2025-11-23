# Sync Integration Branch to Agent Branch

Merge the latest `feature/streamspace-v2-agent-refactor` into your current agent branch.

**Use this when**: You need to sync your agent branch with the latest integrated work from other agents.

## Step 1: Identify Current Branch

!git branch --show-current

## Step 2: Fetch Latest Integration Branch

!git fetch origin feature/streamspace-v2-agent-refactor

## Step 3: Show What's New in Integration

!git log --oneline --stat origin/feature/streamspace-v2-agent-refactor ^HEAD

## Step 4: Merge Integration Branch

!git merge origin/feature/streamspace-v2-agent-refactor --no-edit

## Step 5: Push Updated Branch

!git push origin HEAD

---

## If Conflicts Occur

1. **Identify conflicting files**:
   !git status

2. **Analyze conflicts**:
   Read conflicting files and understand what changed

3. **Resolve conflicts**:
   - Keep your changes if they're newer/better
   - Keep integration changes if they fix bugs
   - Combine both if needed

4. **Complete merge**:
   !git add [resolved files]
   !git commit --no-edit
   !git push origin HEAD

---

## Notes

- **Before syncing**: Commit any uncommitted work on your branch
- **After syncing**: Verify tests still pass
- **Conflict resolution**: Ask Architect if unsure which changes to keep
- **Regular syncing**: Sync at least once per wave to avoid large conflicts
