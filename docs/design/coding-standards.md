# Coding Standards & Style Guide

**Version**: v1.0
**Last Updated**: 2025-11-26
**Owner**: Team (Architect + Builder + Contributors)
**Status**: Living Document

---

## Introduction

This document defines coding standards for StreamSpace to ensure consistency, maintainability, and quality across the codebase. All contributors must follow these standards.

**Philosophy**: Favor clarity over cleverness. Code is read more often than written.

---

## General Principles

### 1. Code Quality Tenets

1. **Readability First**: Code should be self-explanatory with minimal comments
2. **Explicit > Implicit**: Prefer explicit error handling over silent failures
3. **Simple > Complex**: Choose the simplest solution that solves the problem
4. **Tested**: All new code must include tests (unit tests minimum)
5. **Secure by Default**: Validate inputs, escape outputs, use parameterized queries

### 2. File Organization

```
project/
├── cmd/              # Application entry points (main packages)
├── internal/         # Private application code
├── pkg/              # Public library code (reusable)
├── api/              # API definitions, OpenAPI specs
├── web/              # Web assets
├── configs/          # Configuration files
├── scripts/          # Build/deployment scripts
├── docs/             # Documentation
└── tests/            # Integration tests, E2E tests
```

### 3. Naming Conventions

**General Rules**:
- Use descriptive names (avoid abbreviations unless universally known)
- Follow language-specific conventions (Go: camelCase, Python: snake_case, etc.)
- Be consistent within a module/package

**Examples**:
- ✅ `getUserByID`, `sessionTimeout`, `maxRetryAttempts`
- ❌ `gubi`, `st`, `mra`

---

## Go (Backend, Agents)

### 1. Code Style

**Use Official Go Style**:
- Run `gofmt` before committing (automatic formatting)
- Run `golangci-lint run` (catches common issues)
- Follow [Effective Go](https://go.dev/doc/effective_go)

**Example**:
```go
// ✅ Good: Standard Go formatting
func (h *Handler) CreateSession(c *gin.Context) {
    var req CreateSessionRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }

    session, err := h.service.Create(c.Request.Context(), req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, session)
}

// ❌ Bad: Inconsistent formatting, missing error handling
func (h *Handler) CreateSession(c *gin.Context) {
  var req CreateSessionRequest
  c.ShouldBindJSON(&req)
  session,_:=h.service.Create(c.Request.Context(),req)
  c.JSON(201,session)
}
```

### 2. Error Handling

**Always Handle Errors**:
```go
// ✅ Good: Explicit error handling
result, err := fetchData()
if err != nil {
    return fmt.Errorf("failed to fetch data: %w", err)
}

// ❌ Bad: Ignoring errors
result, _ := fetchData()

// ❌ Bad: Silent error swallowing
if err != nil {
    log.Println("Error:", err) // Logs but doesn't propagate
}
```

**Error Wrapping**:
```go
// Use %w to wrap errors (enables errors.Is, errors.As)
if err != nil {
    return fmt.Errorf("create session failed: %w", err)
}
```

### 3. Naming Conventions

**Variables**:
- `camelCase` for local variables
- `PascalCase` for exported (public) variables
- Use short names for short scopes (`i` in loops, `err` for errors)

**Functions/Methods**:
- `PascalCase` for exported functions
- `camelCase` for private functions
- Verb-first naming: `GetUser`, `CreateSession`, `DeleteTemplate`

**Interfaces**:
- Single-method interfaces: `-er` suffix (`Reader`, `Writer`, `Validator`)
- Multi-method interfaces: Descriptive names (`SessionManager`, `TemplateRepository`)

**Examples**:
```go
// Variables
var sessionTimeout time.Duration        // Package-level exported
var maxRetries int                      // Package-level private
userID := "abc123"                      // Local variable

// Functions
func GetSession(id string) (*Session, error)  // Exported
func validateQuota(orgID string) error        // Private

// Interfaces
type SessionCreator interface {               // Exported interface
    CreateSession(ctx context.Context, req CreateSessionRequest) (*Session, error)
}
```

### 4. Context Usage

**Always Accept Context as First Parameter**:
```go
// ✅ Good: Context-aware function
func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (*Session, error) {
    // Use ctx for cancellation, deadlines, values
    session, err := s.db.InsertSession(ctx, req)
    return session, err
}

// ❌ Bad: No context support
func (s *Service) CreateSession(req CreateSessionRequest) (*Session, error) {
    session, err := s.db.InsertSession(req) // Can't cancel
    return session, err
}
```

### 5. Logging

**Use Structured Logging**:
```go
// ✅ Good: Structured logging with fields
log.Info("session created",
    "session_id", session.ID,
    "user_id", session.UserID,
    "org_id", session.OrgID,
)

// ❌ Bad: String concatenation
log.Info("Session created: " + session.ID + " for user " + session.UserID)
```

**Log Levels**:
- **Debug**: Verbose debugging information (disabled in production)
- **Info**: General informational messages (startup, shutdown, normal operations)
- **Warn**: Warning conditions (deprecated features, unusual but handled situations)
- **Error**: Error conditions (failures, exceptions)

### 6. Testing

**Test File Naming**:
- `*_test.go` for tests in same package
- `*_integration_test.go` for integration tests

**Test Function Naming**:
```go
func TestCreateSession_Success(t *testing.T) { ... }
func TestCreateSession_InvalidRequest(t *testing.T) { ... }
func TestCreateSession_QuotaExceeded(t *testing.T) { ... }
```

**Table-Driven Tests**:
```go
func TestValidateOrgID(t *testing.T) {
    tests := []struct {
        name    string
        orgID   string
        wantErr bool
    }{
        {"valid UUID", "550e8400-e29b-41d4-a716-446655440000", false},
        {"invalid format", "not-a-uuid", true},
        {"empty string", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateOrgID(tt.orgID)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateOrgID() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 7. Comments

**Package Comments** (required for public packages):
```go
// Package handlers implements HTTP request handlers for the StreamSpace API.
//
// This package provides REST endpoints for session management, template
// catalog, user/org administration, and VNC proxy functionality.
package handlers
```

**Function Comments** (required for exported functions):
```go
// CreateSession provisions a new session for the authenticated user.
//
// The request must include a valid template_id and optional resource overrides.
// Quota enforcement is applied before provisioning. Returns the created session
// or an error if quota exceeded, template not found, or provisioning fails.
func (h *Handler) CreateSession(c *gin.Context) { ... }
```

**Inline Comments** (use sparingly, explain "why" not "what"):
```go
// ✅ Good: Explains business logic
// Skip quota check for admin role (Issue #187)
if user.Role != "admin" {
    if err := h.quotaEnforcer.Check(user.OrgID); err != nil {
        return err
    }
}

// ❌ Bad: Repeats code
// Check if error is not nil
if err != nil {
    return err
}
```

### 8. Security

**Input Validation**:
```go
// ✅ Good: Validate all inputs
func (h *Handler) GetSession(c *gin.Context) {
    sessionID := c.Param("id")
    if !isValidUUID(sessionID) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
        return
    }
    // ... rest of handler
}
```

**SQL Injection Prevention**:
```go
// ✅ Good: Parameterized queries
query := "SELECT * FROM sessions WHERE org_id = $1 AND user_id = $2"
rows, err := db.Query(ctx, query, orgID, userID)

// ❌ Bad: String concatenation (SQL injection risk)
query := "SELECT * FROM sessions WHERE org_id = '" + orgID + "'"
```

**Secrets Management**:
```go
// ✅ Good: Secrets from environment/vault
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    log.Fatal("JWT_SECRET not set")
}

// ❌ Bad: Hardcoded secrets
const jwtSecret = "my-secret-key-123"
```

---

## React/TypeScript (Frontend)

### 1. Code Style

**Use Prettier + ESLint**:
- Run `npm run lint` before committing
- Prettier auto-formats on save (configure IDE)
- Follow [Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript)

**Example**:
```typescript
// ✅ Good: Consistent formatting, TypeScript types
interface CreateSessionRequest {
  templateId: string;
  resources?: {
    cpu?: string;
    memory?: string;
  };
}

const createSession = async (req: CreateSessionRequest): Promise<Session> => {
  const response = await fetch('/api/v1/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });

  if (!response.ok) {
    throw new Error(`Failed to create session: ${response.statusText}`);
  }

  return response.json();
};

// ❌ Bad: Inconsistent formatting, no types
const createSession=async(req)=>{
  const response=await fetch('/api/v1/sessions',{method:'POST',body:JSON.stringify(req)})
  return response.json()
}
```

### 2. TypeScript Types

**Always Use Explicit Types**:
```typescript
// ✅ Good: Explicit types
interface Session {
  id: string;
  userId: string;
  orgId: string;
  templateId: string;
  status: 'pending' | 'running' | 'stopped' | 'failed';
  createdAt: string;
}

const getSession = async (id: string): Promise<Session> => {
  // ...
};

// ❌ Bad: Using 'any'
const getSession = async (id: any): Promise<any> => {
  // ...
};
```

**Props Interfaces**:
```typescript
// ✅ Good: Explicit props interface
interface SessionCardProps {
  session: Session;
  onConnect: (sessionId: string) => void;
  onDelete: (sessionId: string) => void;
}

const SessionCard: React.FC<SessionCardProps> = ({ session, onConnect, onDelete }) => {
  // ...
};

// ❌ Bad: No props type
const SessionCard = ({ session, onConnect, onDelete }) => {
  // ...
};
```

### 3. Component Structure

**Functional Components with Hooks**:
```typescript
// ✅ Good: Functional component, hooks, TypeScript
import { useState, useEffect } from 'react';
import { Box, Button } from '@mui/material';

interface SessionListProps {
  orgId: string;
}

const SessionList: React.FC<SessionListProps> = ({ orgId }) => {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchSessions = async () => {
      setLoading(true);
      try {
        const data = await getSessions(orgId);
        setSessions(data);
      } catch (error) {
        console.error('Failed to fetch sessions:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchSessions();
  }, [orgId]);

  if (loading) return <CircularProgress />;

  return (
    <Box>
      {sessions.map((session) => (
        <SessionCard key={session.id} session={session} />
      ))}
    </Box>
  );
};

export default SessionList;
```

### 4. File Organization

**Component Files**:
```
src/
├── components/           # Reusable components
│   ├── SessionCard.tsx
│   ├── SessionCard.test.tsx
│   └── index.ts         # Barrel export
├── pages/               # Route pages
│   ├── Sessions.tsx
│   └── Dashboard.tsx
├── hooks/               # Custom hooks
│   ├── useSession.ts
│   └── useWebSocket.ts
├── store/               # State management (Zustand)
│   ├── userStore.ts
│   └── sessionStore.ts
├── api/                 # API client functions
│   ├── sessions.ts
│   └── templates.ts
├── types/               # TypeScript types/interfaces
│   └── index.ts
└── utils/               # Utility functions
    └── formatters.ts
```

### 5. Naming Conventions

**Components**:
- `PascalCase` for component files and names
- Descriptive names: `SessionCard`, `UserMenu`, `TemplateList`

**Hooks**:
- `camelCase` starting with `use`: `useSession`, `useAuth`, `useWebSocket`

**Files**:
- Components: `ComponentName.tsx`
- Hooks: `useHookName.ts`
- Types: `types.ts` or `index.ts`
- Tests: `ComponentName.test.tsx`

### 6. State Management

**Zustand Stores** (preferred for global state):
```typescript
// ✅ Good: Zustand store
import create from 'zustand';

interface UserState {
  user: User | null;
  isAuthenticated: boolean;
  login: (user: User) => void;
  logout: () => void;
}

export const useUserStore = create<UserState>((set) => ({
  user: null,
  isAuthenticated: false,
  login: (user) => set({ user, isAuthenticated: true }),
  logout: () => set({ user: null, isAuthenticated: false }),
}));
```

**Component State** (useState for local state):
```typescript
// ✅ Good: Local component state
const [isOpen, setIsOpen] = useState(false);
const [formData, setFormData] = useState<FormData>({ name: '', email: '' });
```

### 7. Error Handling

**Always Handle Errors**:
```typescript
// ✅ Good: Explicit error handling
const fetchSessions = async () => {
  try {
    const data = await getSessions();
    setSessions(data);
  } catch (error) {
    console.error('Failed to fetch sessions:', error);
    // Show error notification to user
    showNotification('Failed to load sessions', 'error');
  }
};

// ❌ Bad: Unhandled promise rejection
const fetchSessions = async () => {
  const data = await getSessions(); // No error handling
  setSessions(data);
};
```

### 8. Testing

**Component Tests** (React Testing Library):
```typescript
import { render, screen, fireEvent } from '@testing-library/react';
import SessionCard from './SessionCard';

describe('SessionCard', () => {
  const mockSession: Session = {
    id: 'sess-123',
    userId: 'user-456',
    status: 'running',
    // ...
  };

  it('renders session information', () => {
    render(<SessionCard session={mockSession} onConnect={jest.fn()} onDelete={jest.fn()} />);
    expect(screen.getByText('sess-123')).toBeInTheDocument();
  });

  it('calls onConnect when connect button clicked', () => {
    const handleConnect = jest.fn();
    render(<SessionCard session={mockSession} onConnect={handleConnect} onDelete={jest.fn()} />);

    fireEvent.click(screen.getByRole('button', { name: /connect/i }));
    expect(handleConnect).toHaveBeenCalledWith('sess-123');
  });
});
```

### 9. Accessibility

**Use Semantic HTML**:
```typescript
// ✅ Good: Semantic elements with ARIA labels
<Button
  variant="contained"
  onClick={handleConnect}
  aria-label="Connect to session"
>
  Connect
</Button>

// ❌ Bad: Generic div with onClick
<div onClick={handleConnect}>Connect</div>
```

**Keyboard Navigation**:
- All interactive elements must be keyboard-accessible
- Use `tabIndex` appropriately
- Provide focus indicators

---

## SQL (Database)

### 1. Query Style

**Formatting**:
```sql
-- ✅ Good: Readable formatting, explicit joins
SELECT
    s.session_id,
    s.user_id,
    s.status,
    s.created_at,
    t.template_name
FROM sessions s
INNER JOIN templates t ON s.template_id = t.template_id
WHERE s.org_id = $1
  AND s.status IN ('running', 'pending')
ORDER BY s.created_at DESC
LIMIT 50;

-- ❌ Bad: One-liner, hard to read
SELECT s.session_id,s.user_id,s.status FROM sessions s WHERE s.org_id=$1 AND s.status='running';
```

### 2. Security

**Always Use Parameterized Queries**:
```sql
-- ✅ Good: Parameterized query
SELECT * FROM sessions WHERE org_id = $1 AND user_id = $2;

-- ❌ Bad: String concatenation (SQL injection risk)
-- SELECT * FROM sessions WHERE org_id = '" + orgID + "';
```

### 3. Indexing

**Create Indexes for Query Performance**:
```sql
-- Create indexes on commonly queried columns
CREATE INDEX idx_sessions_org_id ON sessions(org_id);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_status ON sessions(status);

-- Composite index for common query patterns
CREATE INDEX idx_sessions_org_status ON sessions(org_id, status);
```

---

## Git Commit Messages

### 1. Commit Message Format

**Use Conventional Commits**:
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring (no feature/fix)
- `test`: Adding/updating tests
- `chore`: Build/tooling changes

**Examples**:
```
feat(api): add session hibernation endpoint

Implements POST /api/v1/sessions/:id/hibernate endpoint.
Pauses session container to save resources.

Closes #123

---

fix(ui): prevent duplicate session cards in list

Race condition in WebSocket handler caused duplicate renders.
Added session ID deduplication in SessionList component.

Fixes #456

---

docs(arch): add C4 architecture diagrams

Created comprehensive C4 diagrams showing system context,
containers, components, and deployment topology.
```

### 2. Commit Guidelines

**Atomic Commits**:
- One logical change per commit
- Commit compiles and tests pass
- Can be reverted independently

**Commit Frequency**:
- Commit often (multiple per day)
- Don't commit broken code to main branch
- Use feature branches for work-in-progress

---

## Pull Request (PR) Guidelines

### 1. PR Title

Use conventional commit format:
```
feat(api): add multi-tenancy org scoping
fix(ui): session list pagination bug
docs: update deployment guide
```

### 2. PR Description Template

```markdown
## Summary
Brief description of changes (1-3 sentences).

## Changes
- Added X feature
- Fixed Y bug
- Refactored Z component

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Screenshots (if UI changes)
[Add screenshots here]

## Related Issues
Closes #123
Relates to #456

## Checklist
- [ ] Code follows style guide
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] No new warnings/errors
- [ ] Reviewed own code
```

### 3. PR Review Checklist

**Reviewers Should Check**:
1. **Correctness**: Does code do what it claims?
2. **Tests**: Are there tests? Do they pass?
3. **Security**: Any security vulnerabilities?
4. **Performance**: Any performance concerns?
5. **Style**: Follows coding standards?
6. **Documentation**: Is documentation updated?

**Approval Criteria**:
- At least 1 approval from maintainer
- All CI checks pass (tests, linter)
- No unresolved comments

---

## Code Review Best Practices

### 1. Giving Feedback

**Be Constructive**:
```
// ✅ Good: Specific, actionable feedback
"Consider extracting this validation logic into a separate function
for reusability. Example: `validateSessionRequest(req)`"

// ❌ Bad: Vague, dismissive
"This is messy."
```

**Ask Questions**:
```
// ✅ Good: Open-ended question
"What's the reasoning behind using a channel here instead of a mutex?
I'm curious about the trade-offs."

// ❌ Bad: Accusatory
"Why did you do this wrong?"
```

### 2. Receiving Feedback

**Be Open**:
- Assume positive intent
- Ask clarifying questions if feedback unclear
- Don't take it personally

**Respond Promptly**:
- Address comments within 24 hours
- Mark resolved comments as resolved
- Explain decisions if needed

---

## Tooling

### Go Tools

```bash
# Format code
gofmt -w .

# Run linter
golangci-lint run

# Run tests with coverage
go test -v -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### TypeScript/React Tools

```bash
# Format code
npm run format

# Run linter
npm run lint

# Fix linting issues
npm run lint:fix

# Run tests
npm test

# Run tests with coverage
npm run test:coverage
```

### Pre-Commit Hooks

**Install pre-commit hooks** (`.git/hooks/pre-commit`):
```bash
#!/bin/bash
# Run linters before commit

# Go
cd api && golangci-lint run || exit 1

# TypeScript
cd ui && npm run lint || exit 1

echo "✅ Pre-commit checks passed"
```

---

## References

- **Go**: [Effective Go](https://go.dev/doc/effective_go)
- **TypeScript**: [TypeScript Handbook](https://www.typescriptlang.org/docs/handbook/intro.html)
- **React**: [React Docs](https://react.dev/)
- **Conventional Commits**: [conventionalcommits.org](https://www.conventionalcommits.org/)
- **Airbnb Style Guide**: [github.com/airbnb/javascript](https://github.com/airbnb/javascript)

---

## Enforcement

**Automated**:
- CI/CD pipeline runs linters, tests, security scans
- PRs blocked if checks fail

**Manual**:
- Code review enforcement by maintainers
- Style guide violations = request changes

**Education**:
- New contributors: Review this document
- Pair programming sessions for onboarding
- Regular style guide updates based on team feedback

---

**Version History**:
- **v1.0** (2025-11-26): Initial coding standards for v2.0-beta
- **Next Review**: v2.1 release (Q1 2026)
