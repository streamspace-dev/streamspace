# StreamSpace Testing Guide

**Last Updated:** 2025-11-20
**Target Audience:** Developers, QA Engineers, Contributors
**Goal:** Achieve 70%+ test coverage across all components

---

## Table of Contents

- [Overview](#overview)
- [Testing Strategy](#testing-strategy)
- [Test Coverage Goals](#test-coverage-goals)
- [Controller Testing](#controller-testing)
- [API Testing](#api-testing)
- [UI Testing](#ui-testing)
- [Integration Testing](#integration-testing)
- [E2E Testing](#e2e-testing)
- [Test Patterns](#test-patterns)
- [Running Tests](#running-tests)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)

---

## Overview

StreamSpace follows a comprehensive testing strategy covering:

- **Unit Tests** - Individual functions and methods
- **Integration Tests** - Component interactions
- **E2E Tests** - Complete user workflows
- **Performance Tests** - Load and stress testing
- **Security Tests** - Vulnerability scanning

### Current Test Coverage (2025-11-20)

| Component | Current | Target | Status |
|-----------|---------|--------|--------|
| K8s Controller | 30-40% | 70%+ | ⚠️ Needs expansion |
| API Handlers | 10-20% | 70%+ | ⚠️ Needs expansion |
| UI Components | 5% | 70%+ | ⚠️ Needs expansion |
| Integration | 100% | 100% | ✅ Complete |
| E2E | 60% | 80%+ | ⚠️ Some TODOs |

---

## Testing Strategy

### Test Pyramid

```
      ╱╲
     ╱  ╲     E2E Tests (10%)
    ╱────╲    - Complete user workflows
   ╱      ╲   - Browser automation
  ╱────────╲
 ╱          ╲ Integration Tests (20%)
╱────────────╲ - API + Controller + Database
───────────────
               Unit Tests (70%)
               - Functions, methods, components
```

### Testing Phases for v1.0.0

**Phase 1: Controller Tests (Weeks 1-3)**
- Expand existing 4 test files
- Add error handling tests
- Test edge cases and race conditions
- Target: 30-40% → 70%+

**Phase 2: API Handler Tests (Weeks 4-7)**
- Test 63 untested handler files
- Focus on critical paths first
- Fix existing test build errors
- Target: 10-20% → 70%+

**Phase 3: UI Component Tests (Weeks 8-10)**
- Test 48 untested components
- Test all pages and user flows
- Vitest already configured
- Target: 5% → 70%+

---

## Test Coverage Goals

### Coverage Targets by Component

**Controller (Kubernetes)**
- `session_controller.go`: 75%+
- `hibernation_controller.go`: 75%+
- `template_controller.go`: 75%+
- `applicationinstall_controller.go`: 70%+

**API Backend**
- Critical handlers (sessions, users, auth): 80%+
- Other handlers: 70%+
- Middleware: 75%+
- Database layer: 70%+

**UI**
- Critical components (SessionCard, PluginCard): 80%+
- Other components: 70%+
- Pages: 70%+
- Utilities: 80%+

### What to Test

**✅ Always Test:**
- Happy path (expected behavior)
- Error conditions (API failures, validation errors)
- Edge cases (empty inputs, maximum limits)
- Authorization (user permissions)
- Concurrent operations (race conditions)
- Resource cleanup (memory leaks, goroutines)

**❌ Don't Waste Time Testing:**
- Generated code (unless you added custom logic)
- Third-party libraries (trust but verify integration)
- Trivial getters/setters
- Constants and enums

---

## Controller Testing

### Technology Stack

- **Framework:** Ginkgo + Gomega (BDD-style)
- **Environment:** envtest (local Kubernetes API)
- **Mocking:** controller-runtime fake client

### Test File Structure

```go
package controllers_test

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    streamv1alpha1 "github.com/yourusername/streamspace/api/v1alpha1"
    "github.com/yourusername/streamspace/controllers"
)

var _ = Describe("SessionController", func() {
    var (
        ctx        context.Context
        reconciler *controllers.SessionReconciler
        session    *streamv1alpha1.Session
    )

    BeforeEach(func() {
        ctx = context.Background()
        // Setup test resources
    })

    AfterEach(func() {
        // Cleanup
    })

    Context("When creating a new Session", func() {
        BeforeEach(func() {
            session = &streamv1alpha1.Session{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-session",
                    Namespace: "default",
                },
                Spec: streamv1alpha1.SessionSpec{
                    User:     "testuser",
                    Template: "firefox",
                },
            }
        })

        It("Should create a Deployment", func() {
            // Test implementation
            Expect(k8sClient.Create(ctx, session)).To(Succeed())

            // Wait for reconciliation
            Eventually(func() error {
                var deployment appsv1.Deployment
                return k8sClient.Get(ctx, types.NamespacedName{
                    Name:      "ss-testuser-firefox",
                    Namespace: "default",
                }, &deployment)
            }, timeout, interval).Should(Succeed())
        })

        It("Should create a Service", func() {
            // Test service creation
        })

        It("Should create user PVC if it doesn't exist", func() {
            // Test PVC creation
        })
    })

    Context("When session enters hibernated state", func() {
        It("Should scale Deployment to 0 replicas", func() {
            // Test hibernation
        })
    })

    Context("When template doesn't exist", func() {
        It("Should set error status condition", func() {
            // Test error handling
        })
    })
})
```

### Critical Test Scenarios

**Session Controller:**
1. **Happy Path:** Create session → deployment/service/ingress created → status updated
2. **User PVC:** First session creates PVC, subsequent sessions reuse it
3. **State Transitions:** running → hibernated → running → terminated
4. **Resource Limits:** Respect memory/CPU quotas from template
5. **Cleanup:** Deleting session removes deployment/service but keeps PVC
6. **Concurrent:** Multiple sessions for same user don't conflict
7. **Error Handling:** Missing template, invalid image, quota exceeded

**Hibernation Controller:**
1. **Idle Detection:** Correctly identifies sessions past idleTimeout
2. **Scale to Zero:** Sets deployment replicas to 0
3. **Wake on Access:** Updates lastActivity, scales back to 1 replica
4. **Custom Timeouts:** Respects per-session idleTimeout overrides
5. **Edge Cases:** Session deleted while hibernated, concurrent wake/hibernate

**Template Controller:**
1. **Validation:** Rejects invalid templates (missing image, bad resources)
2. **Updates:** Changes to template don't affect running sessions
3. **Deletion:** Can't delete template with active sessions
4. **Defaults:** Properly applies defaultResources when session doesn't specify

### Running Controller Tests

```bash
cd k8s-controller

# Run all tests
make test

# Run specific controller tests
go test ./controllers -run TestSessionController -v

# Run with coverage
go test ./controllers -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run specific test case
go test ./controllers -ginkgo.focus="Should create a Deployment" -v

# Check coverage percentage
go tool cover -func=coverage.out | grep total
```

### Example: Testing Error Handling

```go
Context("When Kubernetes API fails", func() {
    var fakeClient client.Client

    BeforeEach(func() {
        // Use fake client that returns errors
        fakeClient = fake.NewClientBuilder().
            WithScheme(scheme).
            WithInterceptorFuncs(interceptor.Funcs{
                Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
                    return errors.New("API error")
                },
            }).
            Build()

        reconciler = &controllers.SessionReconciler{
            Client: fakeClient,
            Scheme: scheme,
        }
    })

    It("Should requeue with error", func() {
        result, err := reconciler.Reconcile(ctx, reconcile.Request{
            NamespacedName: types.NamespacedName{
                Name:      "test-session",
                Namespace: "default",
            },
        })

        Expect(err).To(HaveOccurred())
        Expect(result.Requeue).To(BeFalse())
    })
})
```

---

## API Testing

### Technology Stack

- **Framework:** Go testing + testify
- **HTTP:** httptest for HTTP testing
- **Database:** SQLite in-memory for tests
- **Mocking:** testify/mock

### Test File Structure

```go
package handlers_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"

    "github.com/yourusername/streamspace/api/internal/handlers"
)

func TestCreateSession(t *testing.T) {
    // Setup
    gin.SetMode(gin.TestMode)
    router := gin.Default()

    // Initialize test database
    db := setupTestDB(t)
    defer db.Close()

    // Register handler
    handler := handlers.NewSessionHandler(db)
    router.POST("/api/v1/sessions", handler.CreateSession)

    // Test cases
    tests := []struct {
        name           string
        requestBody    interface{}
        expectedStatus int
        expectedBody   string
    }{
        {
            name: "Valid session creation",
            requestBody: map[string]interface{}{
                "user":     "testuser",
                "template": "firefox",
                "resources": map[string]string{
                    "memory": "2Gi",
                },
            },
            expectedStatus: http.StatusCreated,
        },
        {
            name: "Missing required field",
            requestBody: map[string]interface{}{
                "user": "testuser",
                // Missing template
            },
            expectedStatus: http.StatusBadRequest,
        },
        {
            name: "Invalid resource format",
            requestBody: map[string]interface{}{
                "user":     "testuser",
                "template": "firefox",
                "resources": map[string]string{
                    "memory": "invalid",
                },
            },
            expectedStatus: http.StatusBadRequest,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create request
            body, _ := json.Marshal(tt.requestBody)
            req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBuffer(body))
            req.Header.Set("Content-Type", "application/json")
            req.Header.Set("Authorization", "Bearer test-token")

            // Record response
            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            // Assert
            assert.Equal(t, tt.expectedStatus, w.Code)
        })
    }
}
```

### Critical Test Scenarios

**Session Handlers:**
1. **Create:** Valid input → session created, invalid → 400, unauthorized → 401
2. **List:** Returns user's sessions, admin sees all, pagination works
3. **Get:** Returns session details, 404 for non-existent, 403 for other user's session
4. **Delete:** Removes session, 404 for non-existent, 403 for other user's session
5. **Update:** Changes state, validates transitions, rejects invalid states

**User Handlers:**
1. **Create:** Admin creates user, validates email format, rejects duplicates
2. **Update:** Changes properties, password hashing, unauthorized users rejected
3. **Delete:** Removes user and sessions, admin-only operation
4. **Quota:** Enforces user quotas, rejects over-quota operations

**Auth Handlers:**
1. **Login:** Valid credentials → JWT token, invalid → 401, rate limiting works
2. **Logout:** Invalidates token, returns 200
3. **Refresh:** Valid refresh token → new access token, expired → 401
4. **MFA:** TOTP validation, rate limiting, backup codes

### Testing Middleware

```go
func TestAuthMiddleware(t *testing.T) {
    gin.SetMode(gin.TestMode)

    tests := []struct {
        name           string
        authHeader     string
        expectedStatus int
    }{
        {
            name:           "Valid JWT token",
            authHeader:     "Bearer " + generateValidToken(),
            expectedStatus: http.StatusOK,
        },
        {
            name:           "Missing auth header",
            authHeader:     "",
            expectedStatus: http.StatusUnauthorized,
        },
        {
            name:           "Invalid token format",
            authHeader:     "Bearer invalid",
            expectedStatus: http.StatusUnauthorized,
        },
        {
            name:           "Expired token",
            authHeader:     "Bearer " + generateExpiredToken(),
            expectedStatus: http.StatusUnauthorized,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            router := gin.Default()
            router.Use(middleware.AuthMiddleware())
            router.GET("/test", func(c *gin.Context) {
                c.JSON(http.StatusOK, gin.H{"status": "ok"})
            })

            req := httptest.NewRequest(http.MethodGet, "/test", nil)
            if tt.authHeader != "" {
                req.Header.Set("Authorization", tt.authHeader)
            }

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)
        })
    }
}
```

### Running API Tests

```bash
cd api

# Run all tests
go test ./... -v

# Run specific package
go test ./internal/handlers -v

# Run with coverage
go test ./internal/handlers -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific test
go test ./internal/handlers -run TestCreateSession -v

# Check coverage by file
go tool cover -func=coverage.out
```

---

## UI Testing

### Technology Stack

- **Framework:** Vitest + React Testing Library
- **Assertion:** Vitest expect
- **Mocking:** vi.mock()
- **Coverage:** Vitest coverage (v8)

### Test File Structure

```typescript
// SessionCard.test.tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import SessionCard from './SessionCard'

describe('SessionCard', () => {
  const mockSession = {
    id: '123',
    name: 'test-firefox',
    user: 'testuser',
    template: 'firefox',
    status: 'running',
    vncUrl: 'https://example.com/vnc/123',
    createdAt: '2025-11-20T10:00:00Z'
  }

  it('renders session information correctly', () => {
    render(<SessionCard session={mockSession} />)

    expect(screen.getByText('test-firefox')).toBeInTheDocument()
    expect(screen.getByText('firefox')).toBeInTheDocument()
    expect(screen.getByText('Running')).toBeInTheDocument()
  })

  it('calls onConnect when Connect button clicked', () => {
    const onConnect = vi.fn()
    render(<SessionCard session={mockSession} onConnect={onConnect} />)

    const connectButton = screen.getByRole('button', { name: /connect/i })
    fireEvent.click(connectButton)

    expect(onConnect).toHaveBeenCalledWith(mockSession.id)
  })

  it('shows hibernated status with wake button', () => {
    const hibernatedSession = { ...mockSession, status: 'hibernated' }
    const onWake = vi.fn()

    render(<SessionCard session={hibernatedSession} onWake={onWake} />)

    expect(screen.getByText('Hibernated')).toBeInTheDocument()

    const wakeButton = screen.getByRole('button', { name: /wake/i })
    fireEvent.click(wakeButton)

    expect(onWake).toHaveBeenCalledWith(hibernatedSession.id)
  })

  it('shows delete confirmation dialog', async () => {
    render(<SessionCard session={mockSession} />)

    const deleteButton = screen.getByRole('button', { name: /delete/i })
    fireEvent.click(deleteButton)

    expect(screen.getByText(/confirm deletion/i)).toBeInTheDocument()
  })
})
```

### Testing Pages with API Calls

```typescript
// Dashboard.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import Dashboard from './Dashboard'
import * as api from '../lib/api'

vi.mock('../lib/api')

describe('Dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('displays sessions after loading', async () => {
    const mockSessions = [
      { id: '1', name: 'firefox-1', status: 'running' },
      { id: '2', name: 'vscode-1', status: 'hibernated' }
    ]

    vi.mocked(api.getSessions).mockResolvedValue(mockSessions)

    render(<Dashboard />)

    // Shows loading state
    expect(screen.getByText(/loading/i)).toBeInTheDocument()

    // Shows sessions after load
    await waitFor(() => {
      expect(screen.getByText('firefox-1')).toBeInTheDocument()
      expect(screen.getByText('vscode-1')).toBeInTheDocument()
    })
  })

  it('handles API errors gracefully', async () => {
    vi.mocked(api.getSessions).mockRejectedValue(new Error('API Error'))

    render(<Dashboard />)

    await waitFor(() => {
      expect(screen.getByText(/error loading sessions/i)).toBeInTheDocument()
    })
  })
})
```

### Running UI Tests

```bash
cd ui

# Run all tests
npm test

# Run with coverage
npm run test:coverage

# Run in watch mode
npm run test:watch

# Run specific test file
npm test SessionCard.test.tsx

# Update snapshots
npm test -- -u
```

### Vitest Configuration

```typescript
// vitest.config.ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      exclude: [
        'node_modules/',
        'src/test/',
        '**/*.test.tsx',
        '**/*.test.ts',
      ],
      thresholds: {
        lines: 70,
        functions: 70,
        branches: 70,
        statements: 70,
      },
    },
  },
})
```

---

## Integration Testing

Integration tests verify that multiple components work together correctly.

### Test Structure

```go
// tests/integration/core_platform_test.go
package integration_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSessionLifecycle(t *testing.T) {
    // Setup test environment
    ctx := context.Background()
    testEnv := setupTestEnvironment(t)
    defer testEnv.Cleanup()

    // Create user
    user := testEnv.CreateUser("testuser", "test@example.com")
    require.NotNil(t, user)

    // Create session via API
    session := testEnv.CreateSession(user.ID, "firefox", map[string]string{
        "memory": "2Gi",
        "cpu":    "1000m",
    })
    require.NotNil(t, session)
    assert.Equal(t, "pending", session.Status)

    // Wait for session to become running
    session = testEnv.WaitForSessionStatus(session.ID, "running", 2*time.Minute)
    require.NotNil(t, session)
    assert.Equal(t, "running", session.Status)

    // Verify Kubernetes resources created
    deployment := testEnv.GetDeployment(session.Name)
    require.NotNil(t, deployment)
    assert.Equal(t, int32(1), *deployment.Spec.Replicas)

    service := testEnv.GetService(session.Name)
    require.NotNil(t, service)

    // Test hibernation
    time.Sleep(35 * time.Second) // Wait past idle timeout (30s)
    testEnv.TriggerHibernationCheck()

    session = testEnv.WaitForSessionStatus(session.ID, "hibernated", 1*time.Minute)
    assert.Equal(t, "hibernated", session.Status)

    deployment = testEnv.GetDeployment(session.Name)
    assert.Equal(t, int32(0), *deployment.Spec.Replicas)

    // Test wake
    testEnv.UpdateSessionActivity(session.ID)
    session = testEnv.WaitForSessionStatus(session.ID, "running", 1*time.Minute)
    assert.Equal(t, "running", session.Status)

    // Delete session
    testEnv.DeleteSession(session.ID)

    // Verify resources cleaned up
    assert.Eventually(t, func() bool {
        return testEnv.GetDeployment(session.Name) == nil
    }, 30*time.Second, 1*time.Second)
}
```

### Running Integration Tests

```bash
cd tests/integration

# Run all integration tests
./run-integration-tests.sh

# Run specific test
go test -run TestSessionLifecycle -v

# Run with race detector
go test -race ./...
```

---

## E2E Testing

End-to-end tests simulate real user workflows in a browser.

### Technology Stack

- **Framework:** Playwright
- **Languages:** TypeScript
- **Browsers:** Chromium, Firefox, WebKit

### Example E2E Test

```typescript
// e2e/session-creation.spec.ts
import { test, expect } from '@playwright/test'

test.describe('Session Creation Flow', () => {
  test('user can create and connect to session', async ({ page }) => {
    // Login
    await page.goto('http://localhost:3000/login')
    await page.fill('[name="username"]', 'testuser')
    await page.fill('[name="password"]', 'testpass')
    await page.click('button[type="submit"]')

    // Wait for dashboard
    await expect(page).toHaveURL('http://localhost:3000/dashboard')

    // Navigate to catalog
    await page.click('text=Catalog')
    await expect(page).toHaveURL('http://localhost:3000/catalog')

    // Launch Firefox template
    await page.click('text=Firefox Browser')
    await page.click('button:has-text("Launch")')

    // Fill session form
    await page.fill('[name="sessionName"]', 'my-firefox')
    await page.selectOption('[name="memory"]', '2Gi')
    await page.click('button:has-text("Create Session")')

    // Wait for session to start
    await expect(page.locator('text=my-firefox')).toBeVisible({ timeout: 120000 })
    await expect(page.locator('[data-status="running"]')).toBeVisible()

    // Connect to session
    await page.click('button:has-text("Connect")')

    // Verify VNC viewer opens
    await expect(page).toHaveURL(/.*\/vnc\/.*/)
    await expect(page.locator('canvas')).toBeVisible()
  })
})
```

### Running E2E Tests

```bash
cd e2e

# Install dependencies
npm install

# Run all E2E tests
npx playwright test

# Run in headed mode (see browser)
npx playwright test --headed

# Run specific browser
npx playwright test --project=chromium

# Generate test report
npx playwright show-report
```

---

## Test Patterns

### Pattern 1: Table-Driven Tests (Go)

```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"missing @", "userexample.com", true},
        {"missing domain", "user@", true},
        {"empty string", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateEmail(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateEmail() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Pattern 2: Test Fixtures (TypeScript)

```typescript
// test/fixtures/sessions.ts
export const mockSessions = {
  running: {
    id: '123',
    name: 'firefox-1',
    status: 'running',
    vncUrl: 'https://example.com/vnc/123'
  },
  hibernated: {
    id: '456',
    name: 'vscode-1',
    status: 'hibernated',
    vncUrl: null
  },
  pending: {
    id: '789',
    name: 'gimp-1',
    status: 'pending',
    vncUrl: null
  }
}

// Usage in tests
import { mockSessions } from './fixtures/sessions'

it('renders running session', () => {
  render(<SessionCard session={mockSessions.running} />)
  // ...
})
```

### Pattern 3: Test Helpers

```go
// test/helpers/k8s.go
func CreateTestSession(name, user, template string) *streamv1alpha1.Session {
    return &streamv1alpha1.Session{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: "default",
        },
        Spec: streamv1alpha1.SessionSpec{
            User:     user,
            Template: template,
            Resources: streamv1alpha1.ResourceRequirements{
                Memory: "2Gi",
                CPU:    "1000m",
            },
        },
    }
}

func WaitForDeployment(ctx context.Context, client client.Client, name, namespace string, timeout time.Duration) (*appsv1.Deployment, error) {
    var deployment appsv1.Deployment
    err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
        err := client.Get(ctx, types.NamespacedName{
            Name:      name,
            Namespace: namespace,
        }, &deployment)
        return err == nil, nil
    })
    if err != nil {
        return nil, err
    }
    return &deployment, nil
}
```

---

## Running Tests

### Local Development

```bash
# Controller tests
cd k8s-controller && make test

# API tests
cd api && go test ./... -v

# UI tests
cd ui && npm test

# Integration tests
cd tests && ./run-integration-tests.sh

# E2E tests
cd e2e && npx playwright test
```

### Check Coverage

```bash
# Controller coverage
cd k8s-controller
go test ./controllers -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# API coverage
cd api
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# UI coverage
cd ui
npm run test:coverage
# Opens coverage report in browser
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/test.yml
name: Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

jobs:
  controller-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run controller tests
        working-directory: k8s-controller
        run: make test

      - name: Check coverage
        working-directory: k8s-controller
        run: |
          go test ./controllers -coverprofile=coverage.out
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$COVERAGE < 70" | bc -l) )); then
            echo "Coverage is below 70%: $COVERAGE%"
            exit 1
          fi

  api-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run API tests
        working-directory: api
        run: go test ./... -v -coverprofile=coverage.out

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./api/coverage.out

  ui-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Install dependencies
        working-directory: ui
        run: npm ci

      - name: Run UI tests
        working-directory: ui
        run: npm run test:coverage

      - name: Check coverage threshold
        working-directory: ui
        run: |
          # Vitest enforces thresholds in config
          npm test -- --run
```

---

## Best Practices

### General Testing Principles

1. **Write Tests First** (TDD when possible)
   - Define expected behavior before implementation
   - Helps clarify requirements
   - Prevents over-engineering

2. **Test Behavior, Not Implementation**
   - Test what the code does, not how it does it
   - Makes tests resilient to refactoring
   - Focus on public APIs

3. **Keep Tests Independent**
   - Each test should run in isolation
   - No shared state between tests
   - Use setup/teardown properly

4. **Use Descriptive Test Names**
   - Good: `TestCreateSession_WithInvalidTemplate_ReturnsError`
   - Bad: `TestCreateSession1`

5. **Test One Thing Per Test**
   - Each test validates one specific behavior
   - Makes failures easy to diagnose
   - Keeps tests short and focused

### Specific to StreamSpace

**Controller Tests:**
- Use `envtest` for realistic Kubernetes API simulation
- Test status conditions thoroughly (they're how users see state)
- Test finalizers and cleanup logic
- Test reconciliation idempotency (running twice should be safe)

**API Tests:**
- Mock database for unit tests, use real DB for integration
- Test authorization on every endpoint
- Test validation of all input fields
- Test JSON marshaling/unmarshaling

**UI Tests:**
- Mock API calls to avoid flakiness
- Test user interactions (clicks, form inputs)
- Test error states and loading states
- Use accessibility queries (`getByRole`, `getByLabelText`)

### Common Pitfalls

❌ **Don't:**
- Test third-party code
- Hardcode timestamps (use freezeTime or relative checks)
- Ignore test failures ("flaky tests")
- Write tests that depend on external services
- Commit failing tests

✅ **Do:**
- Mock external dependencies
- Clean up resources in teardown
- Use test-specific namespaces/databases
- Run tests locally before pushing
- Keep tests fast (<100ms per unit test)

---

## Success Metrics

### v1.0.0 Test Coverage Goals

- [ ] Controller tests: 70%+ coverage
- [ ] API handler tests: 70%+ coverage
- [ ] UI component tests: 70%+ coverage
- [ ] Integration tests: 100% coverage (already achieved)
- [ ] E2E tests: 80%+ coverage
- [ ] All CI builds pass
- [ ] Zero flaky tests in CI

### Tracking Progress

```bash
# Weekly coverage report
./scripts/generate-coverage-report.sh

# Output:
# Controller: 45% → Target: 70% (56% remaining)
# API: 32% → Target: 70% (54% remaining)
# UI: 18% → Target: 70% (74% remaining)
```

---

## Resources

### Documentation
- [Ginkgo Documentation](https://onsi.github.io/ginkgo/)
- [Kubebuilder Testing Guide](https://book.kubebuilder.io/reference/testing)
- [Go Testing Best Practices](https://go.dev/doc/tutorial/add-a-test)
- [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/)
- [Vitest Documentation](https://vitest.dev/)
- [Playwright Documentation](https://playwright.dev/)

### StreamSpace-Specific
- [VALIDATOR_TASK_CONTROLLER_TESTS.md](../.claude/multi-agent/VALIDATOR_TASK_CONTROLLER_TESTS.md) - Detailed controller testing guide
- [CODEBASE_AUDIT_REPORT.md](./CODEBASE_AUDIT_REPORT.md) - Current test coverage status
- [V1_ROADMAP_SUMMARY.md](./V1_ROADMAP_SUMMARY.md) - Testing priorities for v1.0.0

---

## Questions?

For testing questions or issues:

1. Check existing test files for examples
2. Review this guide and linked documentation
3. Ask in `#testing` channel or GitHub Discussions
4. Tag Validator (Agent 3) in multi-agent sessions

---

**Last Updated:** 2025-11-20
**Maintained By:** Agent 4 (Scribe)
**Next Review:** When test coverage reaches 70%
