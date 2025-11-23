# Check Work

Find assigned tasks and priorities.

## Usage

`/check-work`

## Logic

1. **Assignments**: `gh issue list --assignee @me`
2. **Priorities**: Filter by P0/P1.
3. **Ready**: Check `label:ready-for-testing` (if Validator).
4. **Plan**: Check `MULTI_AGENT_PLAN.md`.

## Output

- List of active issues.
- Next recommended action.
