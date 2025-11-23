# Check for Assigned Work

Check GitHub issues for work assigned to your agent role.

**Use this when**: Starting a new session or looking for your next task.

## What This Checks

 1. **Issues assigned to your role**:

    ```bash
    gh issue list --label "agent:builder" --state open
    ```

 2. **Current milestone**:

    ```bash
    gh issue list --milestone "v2.0-beta.1"
    ```

 3. **Priority order**: Filter results by P0/P1/P2 labels.
 4. **Blocking issues**: Check issue body for "Blocks #" text.
 5. **Ready-for-testing**:

    ```bash
    gh issue list --label "ready-for-testing"
    ```

## Output Format

```markdown
## 🎯 Your Assigned Work

### P0 CRITICAL (Do First)
- #200 [TEST] Fix Broken Test Suites (agent:validator, P0)
  Status: In Progress
  Labels: P0, agent:validator, component:testing

### P1 HIGH PRIORITY
- #203 [TEST] K8s Agent Leader Election Tests (agent:validator, P1)
  Status: Open
  Dependencies: #200 (must be fixed first)

### P2 MEDIUM PRIORITY
- #205 [TEST] Integration Test Suite (agent:validator, P1)
  Status: Open

### 🔔 Ready for Testing (Validator Only)
- #134 P1-MULTI-POD-001 - marked ready-for-testing by Builder
- #135 P1-SCHEMA-002 - marked ready-for-testing by Builder

### 📋 Recommendations
1. Start with #200 (P0, blocking other work)
2. Then proceed to #203 (P1, depends on #200)
3. #205 can run in parallel after #200 complete
```

## Filters

You can filter by:

- `/check-work milestone:v2.0-beta.2` - Check specific milestone
- `/check-work priority:P0` - Only P0 issues
- `/check-work status:open` - Only open issues

## Integration with MULTI_AGENT_PLAN.md

The command will also check MULTI_AGENT_PLAN.md for:

- Current wave assignments
- Coordination notes from Architect
- Blocked work dependencies
