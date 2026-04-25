# StreamSpace Test Implementation Guide

**Quick Start Guide for Achieving Full Test Coverage**

---

## ✅ Completed Setup

The following infrastructure is now ready:

### 1. API Build Fixes
- ✅ Fixed `quota/enforcer.go` method name issues:
  - Changed `GetByUsername` → `GetUserByUsername`
  - Changed `GetByName` → `GetGroupByName`

### 2. UI Test Infrastructure
- ✅ Created `vitest.config.ts` with coverage thresholds (80%)
- ✅ Created `ui/src/test/setup.ts` with test environment configuration
- ✅ Updated `package.json` with test dependencies and scripts
- ✅ Created `ui/src/test/README.md` with testing guidelines

### 3. Test Coverage Analysis
- ✅ Comprehensive analysis documented in `TEST_COVERAGE_REPORT.md`
- ✅ Current coverage: ~15-20% overall
- ✅ Target coverage: 85% overall

---

## 🚀 Next Steps (Immediate Actions)

### Step 1: Install Dependencies (5 minutes)

```bash
# Install UI test dependencies
cd /home/user/streamspace/ui
npm install

# Verify installation
npm run test:run
```

Expected output: Existing 2 tests should pass (SessionCard, SecuritySettings)

### Step 2: Verify API Tests Can Build (5 minutes)

```bash
# Try building API tests (may still have network/dependency issues)
cd /home/user/streamspace/api
go mod tidy

# If network issues persist, use vendor mode
go mod vendor
go test -mod=vendor ./internal/quota/... -v
```

### Step 3: Set Up Controller Test Environment (15 minutes)

Option A - Install envtest binaries:
```bash
# Install setup-envtest tool
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Install Kubernetes binaries
setup-envtest use -p path 1.28.0
export KUBEBUILDER_ASSETS=$(setup-envtest use -p path 1.28.0)

# Run controller tests
cd /home/user/streamspace/controller
go test -v ./...
```

Option B - Skip controller tests for now (focus on API/UI first)

---

## 📋 Priority 1: Critical Path Tests (Week 1)

### Controller Tests to Add

Create these files in `controller/`:

1. **`controllers/session_controller_error_test.go`** - Error handling
   - Template not found
   - Invalid resource specs
   - PVC creation failures
   - Deployment failures
   - Concurrent updates

2. **`pkg/metrics/metrics_test.go`** - Metrics registration and updates
   - Verify all metrics are registered
   - Test metric value updates
   - Test Prometheus format

### API Tests to Add

Create these files in `api/internal/`:

1. **`auth/jwt_test.go`** - JWT token handling
   - Token generation
   - Token validation
   - Token expiration
   - Refresh token flow

2. **`auth/oidc_test.go`** - OIDC OAuth2 integration
   - Provider configuration
   - Authorization flow
   - Token exchange
   - User profile sync

3. **`db/users_test.go`** - User database operations
   - Create user
   - Get user by ID/username
   - Update user
   - Delete user
   - User quota operations

4. **`handlers/sessions_test.go`** - Session CRUD operations
   - Create session
   - List sessions
   - Get session details
   - Hibernate/wake session
   - Terminate session
   - Error cases

5. **`k8s/client_test.go`** - Kubernetes client wrapper
   - Create session resources
   - Get session status
   - Update session
   - Delete session
   - Error handling

### UI Tests to Add

Create these files in `ui/src/`:

1. **`components/Layout.test.tsx`** - Main layout component
   - Renders navigation
   - Renders content area
   - Handles auth state
   - Responsive behavior

2. **`pages/Dashboard.test.tsx`** - User dashboard
   - Renders session list
   - Renders quota information
   - Handles empty state
   - Handles loading state

3. **`hooks/useApi.test.ts`** - API client hook
   - Successful fetch
   - Error handling
   - Loading states
   - Retry logic

4. **`hooks/useWebSocket.test.ts`** - WebSocket hook
   - Connection established
   - Message received
   - Connection error
   - Reconnection logic

5. **`lib/api.test.ts`** - API client library
   - Request headers (JWT)
   - Error responses
   - Request/response interceptors

---

## 📝 Test Template Examples

### Controller Test Template

```go
// controller/controllers/session_controller_error_test.go
package controllers

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Session Controller Error Handling", func() {
	Context("When template does not exist", func() {
		It("Should mark session as failed", func() {
			ctx := context.Background()

			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-missing-template",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:     "testuser",
					Template: "nonexistent-template",
					State:    "running",
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			Eventually(func() string {
				_ = k8sClient.Get(ctx, /* ... */, session)
				return session.Status.Phase
			}, timeout, interval).Should(Equal("Failed"))
		})
	})
})
```

### API Handler Test Template

```go
// api/internal/handlers/sessions_test.go
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCreateSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Setup handler
	handler := NewSessionHandler(/* mocked dependencies */)
	router.POST("/api/sessions", handler.CreateSession)

	t.Run("creates session successfully", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"template": "firefox-browser",
			"resources": map[string]string{
				"memory": "2Gi",
				"cpu":    "1000m",
			},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NotEmpty(t, resp["id"])
	})

	t.Run("returns error for invalid template", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"template": "nonexistent",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
```

### React Component Test Template

```typescript
// ui/src/components/Layout.test.tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { BrowserRouter } from 'react-router-dom';
import Layout from './Layout';

// Mock the user store
vi.mock('../store/userStore', () => ({
  useUserStore: () => ({
    user: { username: 'testuser', email: 'test@example.com' },
    isAuthenticated: true,
    logout: vi.fn(),
  }),
}));

describe('Layout Component', () => {
  const renderLayout = (children: React.ReactNode) => {
    return render(
      <BrowserRouter>
        <Layout>{children}</Layout>
      </BrowserRouter>
    );
  };

  it('renders navigation bar', () => {
    renderLayout(<div>Content</div>);

    expect(screen.getByRole('navigation')).toBeInTheDocument();
  });

  it('renders user menu when authenticated', () => {
    renderLayout(<div>Content</div>);

    expect(screen.getByText('testuser')).toBeInTheDocument();
  });

  it('renders children in content area', () => {
    renderLayout(<div>Test Content</div>);

    expect(screen.getByText('Test Content')).toBeInTheDocument();
  });
});
```

---

## 🎯 Milestones and Estimates

### Week 1: Foundation & Critical Path
- ✅ Setup complete (done!)
- 🎯 Controller: Session error handling tests (4 hours)
- 🎯 API: Auth tests (JWT, OIDC) (6 hours)
- 🎯 API: User DB tests (4 hours)
- 🎯 UI: Layout and Dashboard tests (4 hours)
- 🎯 UI: API hook tests (2 hours)
- **Estimated Coverage**: 40%

### Week 2: Core Features
- 🎯 Controller: Hibernation edge cases (3 hours)
- 🎯 Controller: Metrics tests (2 hours)
- 🎯 API: Session handlers (6 hours)
- 🎯 API: Template handlers (4 hours)
- 🎯 UI: Plugin components (6 hours)
- 🎯 UI: Template components (4 hours)
- **Estimated Coverage**: 60%

### Week 3-4: Comprehensive Coverage
- 🎯 API: All remaining handlers (60+ files) (20 hours)
- 🎯 API: Database models (10 hours)
- 🎯 UI: All components (40+ files) (24 hours)
- 🎯 UI: All pages (20+ files) (16 hours)
- **Estimated Coverage**: 80%

### Week 5-6: Integration Tests
- 🎯 API integration tests (8 hours)
- 🎯 Controller integration tests (6 hours)
- 🎯 E2E user workflows (10 hours)
- **Estimated Coverage**: 85%+

---

## 📊 Daily Progress Tracking

Track your daily progress with this checklist:

### Day 1
- [ ] Install UI dependencies (`npm install`)
- [ ] Run existing UI tests successfully
- [ ] Add `Layout.test.tsx`
- [ ] Add `Dashboard.test.tsx`
- [ ] Run tests and verify they pass

### Day 2
- [ ] Add `useApi.test.ts`
- [ ] Add `useWebSocket.test.ts`
- [ ] Add `api.test.ts`
- [ ] Generate coverage report (`npm run test:coverage`)
- [ ] Document coverage percentage

### Day 3
- [ ] Set up controller envtest
- [ ] Add `session_controller_error_test.go`
- [ ] Add `metrics_test.go`
- [ ] Run controller tests

### Day 4
- [ ] Add `auth/jwt_test.go`
- [ ] Add `auth/oidc_test.go`
- [ ] Run API tests (resolve any dependency issues)

### Day 5
- [ ] Add `db/users_test.go`
- [ ] Add `handlers/sessions_test.go`
- [ ] Add `k8s/client_test.go`
- [ ] Generate API coverage report

---

## 🔍 Quality Checklist

For each test file, ensure:

- ✅ **Positive cases**: Test expected behavior with valid input
- ✅ **Negative cases**: Test error handling with invalid input
- ✅ **Edge cases**: Test boundary conditions (empty, max, zero)
- ✅ **Async behavior**: Test loading states, race conditions
- ✅ **Mocking**: Mock external dependencies (API, K8s, DB)
- ✅ **Assertions**: Clear, specific assertions (not just "truthy")
- ✅ **Test names**: Descriptive names explaining what's tested
- ✅ **Comments**: Document complex test scenarios
- ✅ **Coverage**: Each test increases coverage meaningfully

---

## 🛠 Troubleshooting Common Issues

### Issue: "Cannot find module '@testing-library/react'"
**Solution**: Run `npm install` in the `ui/` directory

### Issue: "fork/exec /usr/local/kubebuilder/bin/etcd: no such file or directory"
**Solution**: Install envtest binaries (see Step 3 above)

### Issue: "dial tcp: lookup storage.googleapis.com"
**Solution**: Use `go mod vendor` and `go test -mod=vendor ./...`

### Issue: "Method 'GetByUsername' not found"
**Solution**: Already fixed! Use `GetUserByUsername` instead

### Issue: Vitest tests fail with import errors
**Solution**: Check `vitest.config.ts` has correct path aliases

---

## 📚 Resources

### Documentation
- [Ginkgo Testing Framework](https://onsi.github.io/ginkgo/)
- [Gomega Matchers](https://onsi.github.io/gomega/)
- [Testify Assert](https://github.com/stretchr/testify)
- [Vitest Documentation](https://vitest.dev/)
- [Testing Library](https://testing-library.com/)

### Best Practices
- [Go Testing Best Practices](https://go.dev/doc/tutorial/add-a-test)
- [React Testing Patterns](https://kentcdodds.com/blog/common-mistakes-with-react-testing-library)
- [Test Coverage Goals](https://martinfowler.com/bliki/TestCoverage.html)

---

## 🎉 Success Criteria

Your test suite is successful when:

1. ✅ All existing tests pass without errors
2. ✅ Coverage is ≥80% for each component (Controller, API, UI)
3. ✅ Coverage is ≥90% for critical paths (auth, session creation)
4. ✅ All tests run in <5 minutes total
5. ✅ Tests are stable (no flaky tests)
6. ✅ CI/CD pipeline enforces coverage thresholds
7. ✅ New PRs include tests for new code

---

## 📞 Need Help?

If you get stuck:

1. Check `TEST_COVERAGE_REPORT.md` for detailed analysis
2. Review test templates in this guide
3. Check existing test files for patterns:
   - `controller/controllers/session_controller_test.go`
   - `api/internal/handlers/validation_test.go`
   - `ui/src/components/SessionCard.test.tsx`
4. Refer to framework documentation (linked above)

Good luck with implementing full test coverage! 🚀
