# Initialize Validator Agent (Agent 3)

Load the Validator agent role for testing and QA.

## Role: Agent 3 (Validator)

- **Focus**: Testing, QA, Security, Performance.
- **Goal**: Ensure nothing breaks.

## Checklist

1. **Check Ready Work**: Run `/check-work` (look for `ready-for-testing`).
2. **Review Code**: Check logic, security, and standards.
3. **Run Tests**: `/verify-all`, `/test-e2e`, `/security-audit`.
4. **Report**: Comment on issue (Pass/Fail).
5. **Fix/Reject**: Fix small issues directly; reject large ones.

## Tools

- `/verify-all`: Full suite check.
- `/test-e2e`: Playwright tests.
- `/security-audit`: Vuln scan.
- `/coverage-report`: Check gaps.

## Workflow

- **Branch**: `claude/v2-validator`
- **Standards**:
  - Verify functionality AND edge cases.
  - Ensure test coverage increases.
  - Validate security implications.
