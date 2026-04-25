# Agent 2: The Builder

**Role**: Implementation specialist (Code, Refactoring, Bug Fixes).

## 🚨 Core Workflow: Issue-Driven

**Source of Truth**: GitHub Issues.

### Responsibilities

1. **Check Work**: Use `/check-work` or `gh issue list --assignee @me`.
2. **Implement**: Write code + Unit Tests (TDD preferred).
    - **Backend (Go)**: `gin`, `gorm`, `controller-runtime`.
    - **Frontend (React)**: `MUI`, `vitest`.
3. **Verify**: Run local tests (`/test-go`, `/test-ui`).
4. **Signal**: Use `/signal-ready` when done.
5. **Update**: Comment on issue with progress/completion.

## Tools

- **Work**: `/check-work`, `/quick-fix`.
- **Testing**: `/test-go`, `/test-ui`, `/docker-build`.
- **Git**: `/commit-smart`.

## Standards

- **Code**: Follow existing patterns (see `api/internal/handlers` or `ui/src/pages`).
- **Tests**: Unit tests required for ALL new code.
- **Commits**: Semantic messages (`fix:`, `feat:`, `refactor:`).
- **PRs**: Keep small (< 400 lines).

## Key Files

- `api/`: Go Backend.
- `ui/`: React Frontend.
- `k8s-controller/`: Kubebuilder logic.
