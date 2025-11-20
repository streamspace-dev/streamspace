# Builder Task: Controller Test Coverage

**Assigned:** 2025-11-20
**Priority:** P0 (CRITICAL)
**Estimated Effort:** 2-3 weeks
**Target:** 30-40% coverage → 70%+ coverage

---

## Quick Reference

**Location:** `/home/user/streamspace/k8s-controller/controllers/`

**Files to Expand:**
1. `session_controller_test.go` (7,242 bytes) - HIGH PRIORITY
2. `hibernation_controller_test.go` (6,412 bytes) - HIGH PRIORITY
3. `template_controller_test.go` (4,971 bytes) - MEDIUM PRIORITY

**Test Commands:**
```bash
cd /home/user/streamspace/k8s-controller

# Run all tests
make test

# Run specific controller tests
go test ./controllers -v

# Check coverage
go test ./controllers -coverprofile=coverage.out
go tool cover -func=coverage.out

# View coverage in browser
go tool cover -html=coverage.out
```

---

## Test Priority Matrix

### 1. Session Controller Tests (HIGHEST PRIORITY)

**File:** `session_controller_test.go`

**Current Coverage:** ~35% (estimate)
**Target Coverage:** 75%+

**Critical Test Cases to Add:**

#### A. Error Handling (Priority 1)
```go
Context("When pod creation fails", func() {
    It("Should retry with exponential backoff", func() {
        // Test retry logic
    })
    It("Should update Session status with error", func() {
        // Test error status reporting
    })
})

Context("When user PVC creation fails", func() {
    It("Should not create pod without persistent storage", func() {
        // Test PVC prerequisite
    })
})

Context("When template doesn't exist", func() {
    It("Should set Session to Failed state", func() {
        // Test invalid template reference
    })
})
```

#### B. Edge Cases (Priority 1)
```go
Context("When duplicate session names exist", func() {
    It("Should reject duplicate session creation", func() {
        // Test name collision
    })
})

Context("When resource quota exceeded", func() {
    It("Should reject session creation", func() {
        // Test quota enforcement
    })
    It("Should return clear error message to user", func() {
        // Test user-facing error
    })
})
```

#### C. State Transitions (Priority 2)
```go
Context("When session state changes", func() {
    It("Should transition running → hibernated correctly", func() {
        // Test hibernation
    })
    It("Should transition hibernated → running correctly", func() {
        // Test wake
    })
    It("Should transition running → terminated correctly", func() {
        // Test deletion
    })
})
```

#### D. Concurrent Operations (Priority 2)
```go
Context("When multiple sessions created simultaneously", func() {
    It("Should handle concurrent user session creation", func() {
        // Test race conditions
    })
    It("Should respect max sessions per user quota", func() {
        // Test concurrent quota checks
    })
})
```

#### E. Resource Cleanup (Priority 1)
```go
Context("When session is deleted", func() {
    It("Should delete associated pod", func() {
        // Test pod cleanup
    })
    It("Should NOT delete user PVC (shared resource)", func() {
        // Test PVC persistence
    })
    It("Should remove finalizers correctly", func() {
        // Test finalizer cleanup
    })
})
```

---

### 2. Hibernation Controller Tests (HIGH PRIORITY)

**File:** `hibernation_controller_test.go`

**Current Coverage:** ~30% (estimate)
**Target Coverage:** 70%+

**Critical Test Cases to Add:**

#### A. Idle Detection (Priority 1)
```go
Context("When detecting idle sessions", func() {
    It("Should identify sessions past idle timeout", func() {
        // Set lastActivity to 31 minutes ago
        // idleTimeout = 30m
        // Expect: session marked for hibernation
    })
    It("Should respect custom idleTimeout values", func() {
        // Test per-session timeout override
    })
    It("Should NOT hibernate active sessions", func() {
        // lastActivity = 5 minutes ago
        // Expect: session remains running
    })
})
```

#### B. Scale to Zero (Priority 1)
```go
Context("When hibernating a session", func() {
    It("Should set Deployment replicas to 0", func() {
        // Verify scale-down
    })
    It("Should update Session phase to Hibernated", func() {
        // Verify status update
    })
    It("Should preserve PVC (persistent storage)", func() {
        // Verify PVC not deleted
    })
})
```

#### C. Wake Cycle (Priority 1)
```go
Context("When waking a hibernated session", func() {
    It("Should set Deployment replicas to 1", func() {
        // Verify scale-up
    })
    It("Should wait for pod readiness", func() {
        // Test readiness checks
    })
    It("Should update Session phase to Running", func() {
        // Verify status update
    })
    It("Should update lastActivity timestamp", func() {
        // Reset idle timer
    })
})
```

#### D. Edge Cases (Priority 2)
```go
Context("When session deleted while hibernated", func() {
    It("Should clean up hibernated deployment", func() {
        // Test cleanup of scaled-down resources
    })
})

Context("When concurrent wake/hibernate requests", func() {
    It("Should handle race conditions gracefully", func() {
        // Test state machine locks
    })
})
```

---

### 3. Template Controller Tests (MEDIUM PRIORITY)

**File:** `template_controller_test.go`

**Current Coverage:** ~40% (estimate)
**Target Coverage:** 70%+

**Critical Test Cases to Add:**

#### A. Template Validation (Priority 1)
```go
Context("When template has invalid image", func() {
    It("Should reject template with empty image name", func() {
        // Test validation
    })
    It("Should reject template with invalid image format", func() {
        // Test image name format
    })
})

Context("When template has missing required fields", func() {
    It("Should reject template without displayName", func() {
        // Test required field validation
    })
})
```

#### B. Resource Defaults (Priority 2)
```go
Context("When template defines defaultResources", func() {
    It("Should apply defaults to new sessions", func() {
        // Test resource propagation
    })
    It("Should allow session-level overrides", func() {
        // Test override behavior
    })
})
```

#### C. Template Lifecycle (Priority 2)
```go
Context("When template is updated", func() {
    It("Should not affect existing sessions", func() {
        // Test isolation
    })
    It("Should apply to new sessions", func() {
        // Test propagation
    })
})

Context("When template is deleted", func() {
    It("Should mark existing sessions (optional behavior)", func() {
        // Define and test deletion policy
    })
})
```

---

## Testing Best Practices

### 1. Use envtest for Kubernetes API Simulation
```go
// Already set up in suite_test.go
testEnv = &envtest.Environment{
    CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
}
```

### 2. Follow Ginkgo/Gomega BDD Patterns
```go
var _ = Describe("SessionController", func() {
    Context("When creating a session", func() {
        It("Should create a pod", func() {
            // Arrange
            session := createTestSession()

            // Act
            result, err := reconciler.Reconcile(ctx, req)

            // Assert
            Expect(err).NotTo(HaveOccurred())
            Expect(result.Requeue).To(BeFalse())

            pod := &corev1.Pod{}
            err = k8sClient.Get(ctx, types.NamespacedName{
                Name:      "ss-" + session.Name,
                Namespace: session.Namespace,
            }, pod)
            Expect(err).NotTo(HaveOccurred())
            Expect(pod.Spec.Containers).To(HaveLen(1))
        })
    })
})
```

### 3. Test Helper Functions
```go
// Create test fixtures
func createTestSession(name, user, template string) *streamspacev1alpha1.Session {
    return &streamspacev1alpha1.Session{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: "default",
        },
        Spec: streamspacev1alpha1.SessionSpec{
            User:     user,
            Template: template,
            State:    streamspacev1alpha1.SessionStateRunning,
            Resources: corev1.ResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceMemory: resource.MustParse("2Gi"),
                    corev1.ResourceCPU:    resource.MustParse("1000m"),
                },
            },
        },
    }
}

// Wait for condition
func waitForSessionPhase(ctx context.Context, client client.Client, name, namespace string, phase streamspacev1alpha1.SessionPhase) error {
    return wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
        session := &streamspacev1alpha1.Session{}
        err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, session)
        if err != nil {
            return false, err
        }
        return session.Status.Phase == phase, nil
    })
}
```

### 4. Mock External Dependencies
```go
// If reconciler calls external APIs, mock them
type mockTemplateClient struct {
    templates map[string]*streamspacev1alpha1.Template
}

func (m *mockTemplateClient) Get(ctx context.Context, name string) (*streamspacev1alpha1.Template, error) {
    if tpl, ok := m.templates[name]; ok {
        return tpl, nil
    }
    return nil, errors.New("template not found")
}
```

---

## Coverage Targets by File

| File | Current | Target | Priority |
|------|---------|--------|----------|
| `session_controller.go` | ~35% | 75%+ | P0 |
| `hibernation_controller.go` | ~30% | 70%+ | P0 |
| `template_controller.go` | ~40% | 70%+ | P1 |
| `applicationinstall_controller.go` | ~20% | 60%+ | P2 |

---

## Success Criteria Checklist

- [ ] **Coverage Goals Met**
  - [ ] Session controller ≥ 75% coverage
  - [ ] Hibernation controller ≥ 70% coverage
  - [ ] Template controller ≥ 70% coverage

- [ ] **Critical Paths Tested**
  - [ ] Session creation (happy path)
  - [ ] Session deletion and cleanup
  - [ ] Hibernation trigger and wake
  - [ ] Error handling for pod failures
  - [ ] Resource quota enforcement
  - [ ] User PVC creation and reuse

- [ ] **Edge Cases Covered**
  - [ ] Concurrent session operations
  - [ ] Invalid template references
  - [ ] Resource limit exceeded
  - [ ] Duplicate session names
  - [ ] Hibernated session deletion
  - [ ] Template updates mid-lifecycle

- [ ] **Tests Pass Locally**
  - [ ] `make test` completes successfully
  - [ ] No flaky tests (run 5 times)
  - [ ] Coverage report generated
  - [ ] All assertions meaningful (no placeholder tests)

- [ ] **Documentation**
  - [ ] Test cases document what they test (clear descriptions)
  - [ ] Complex test logic has inline comments
  - [ ] README updated if new test patterns introduced

---

## Estimated Timeline

**Week 1:** Session controller tests (75% → complete)
- Days 1-2: Error handling tests
- Days 3-4: Edge case tests
- Day 5: State transition tests

**Week 2:** Hibernation controller tests (70% → complete)
- Days 1-2: Idle detection and scale-to-zero tests
- Days 3-4: Wake cycle tests
- Day 5: Edge case tests

**Week 3:** Template controller + polish (70% → complete)
- Days 1-2: Template validation and lifecycle tests
- Day 3: ApplicationInstall controller (if time permits)
- Days 4-5: Coverage review, fix flaky tests, documentation

---

## Reporting Progress

Update MULTI_AGENT_PLAN.md regularly:

```markdown
### Task: Test Coverage - Controller Tests
- **Status**: In Progress
- **Progress**: Session controller tests 60% complete (15/25 test cases)
- **Blockers**: None / [describe blocker]
- **Next**: Completing hibernation edge cases
- **Last Updated**: 2025-11-21 - Builder
```

---

## Questions & Support

**Need help?** Post in MULTI_AGENT_PLAN.md Notes and Blockers section:

```markdown
### Builder → Architect - [Date/Time]
**Question:** How should we handle [specific scenario]?
**Context:** [Describe the situation]
**Options Considered:** [What you've tried]
```

**Found bugs?** Document immediately:
- Create GitHub issue
- Add to MULTI_AGENT_PLAN.md task notes
- Continue with testing (don't block on bug fixes)

---

## Next Task After Completion

Once controller tests are done (≥70% coverage):
→ **API Handler Tests** (Task 2, P0, 3-4 weeks)

You'll test the 63 untested handler files in `api/internal/handlers/`.

---

**Good luck, Builder! You've got this!** 💪

*Document maintained by: Agent 1 (Architect)*
*Last updated: 2025-11-20*
