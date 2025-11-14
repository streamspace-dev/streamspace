# StreamSpace Controller Testing Guide

This document describes how to test the StreamSpace Kubernetes controller.

## Table of Contents

- [Unit Tests](#unit-tests)
- [Integration Tests](#integration-tests)
- [End-to-End Tests](#end-to-end-tests)
- [Manual Testing](#manual-testing)
- [CI/CD Testing](#cicd-testing)

---

## Unit Tests

### Running Unit Tests

```bash
cd controller

# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific test suite
go test ./controllers -v -run TestSession

# Run with race detector
go test -race ./...
```

### Test Structure

Tests use Ginkgo (BDD-style) and Gomega (assertions):

```go
var _ = Describe("Session Controller", func() {
    Context("When creating a new Session", func() {
        It("Should create a Deployment", func() {
            // Test implementation
        })
    })
})
```

### Test Suites

- `controllers/suite_test.go`: Test environment setup
- `controllers/session_controller_test.go`: Session lifecycle tests
- `controllers/template_controller_test.go`: Template validation tests
- `controllers/hibernation_controller_test.go`: Hibernation logic tests

### Coverage Goals

- **Target**: 80%+ code coverage
- **Critical paths**: 95%+ coverage
  - Session state transitions
  - Resource creation/deletion
  - Hibernation triggers

### Running Tests in CI

```yaml
# .github/workflows/test.yml
- name: Run controller tests
  run: |
    cd controller
    make test-coverage

- name: Upload coverage
  uses: codecov/codecov-action@v3
  with:
    files: ./controller/cover.out
```

---

## Integration Tests

Integration tests verify the controller works correctly with a real Kubernetes API.

### Prerequisites

- `kubebuilder` installed
- `envtest` binaries downloaded

### Setup envtest

```bash
# Install envtest binaries
make envtest

# Set up test environment
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/current"
```

### Running Integration Tests

```bash
# Run all integration tests
make test-integration

# Run with verbose output
go test ./controllers -v -ginkgo.v
```

### Integration Test Scenarios

1. **Session Lifecycle**:
   - Create session → Deployment created
   - Hibernate session → Deployment scaled to 0
   - Resume session → Deployment scaled to 1
   - Terminate session → Resources deleted

2. **Resource Management**:
   - PVC creation for persistent homes
   - Service creation for VNC access
   - Ingress creation for external access
   - Owner references and garbage collection

3. **Template Validation**:
   - Valid template → Status Ready
   - Invalid template → Status Invalid with error message
   - VNC configuration validation
   - WebApp configuration validation

4. **Hibernation Logic**:
   - Idle timeout detection
   - Automatic hibernation trigger
   - Skip non-running sessions
   - Handle missing lastActivity

### Test Data

Sample resources for testing:

```yaml
# controller/config/samples/session_test.yaml
apiVersion: stream.streamspace.io/v1alpha1
kind: Session
metadata:
  name: test-session
  namespace: default
spec:
  user: testuser
  template: firefox-browser
  state: running
  persistentHome: true
  idleTimeout: 30m
```

---

## End-to-End Tests

E2E tests verify the entire StreamSpace platform works together.

### Test Environment Setup

1. **Create test cluster**:
   ```bash
   k3d cluster create streamspace-test \
     --api-port 6550 \
     --servers 1 \
     --agents 2
   ```

2. **Deploy StreamSpace**:
   ```bash
   # Deploy CRDs
   kubectl apply -f controller/config/crd/bases/

   # Deploy controller
   make deploy IMG=streamspace-controller:test

   # Deploy API and UI
   helm install streamspace ./chart --namespace streamspace --create-namespace
   ```

3. **Run E2E tests**:
   ```bash
   cd tests/e2e
   go test -v ./...
   ```

### E2E Test Scenarios

1. **User Session Flow**:
   - User logs into UI
   - Creates session from template
   - Connects to running session
   - Hibernates session
   - Resumes session
   - Deletes session

2. **Admin Workflows**:
   - Create user via API
   - Set user quota
   - Create group
   - Add user to group
   - View all sessions

3. **Hibernation End-to-End**:
   - Create session with idle timeout
   - Simulate inactivity
   - Verify automatic hibernation
   - Wake session via API
   - Verify deployment scaled up

4. **Template Management**:
   - Add template repository
   - Sync templates
   - Create session from synced template
   - Update template
   - Delete template

### Performance Tests

```bash
# Load test: Create 100 sessions
kubectl apply -f tests/e2e/load/100-sessions.yaml

# Monitor resource usage
kubectl top pods -n streamspace
kubectl top nodes

# Verify all sessions running
kubectl get sessions -n streamspace
```

---

## Manual Testing

### Test Session Creation

```bash
# 1. Create a template
kubectl apply -f - <<EOF
apiVersion: stream.streamspace.io/v1alpha1
kind: Template
metadata:
  name: test-firefox
  namespace: streamspace
spec:
  displayName: "Firefox Browser"
  baseImage: "lscr.io/linuxserver/firefox:latest"
  ports:
    - name: vnc
      containerPort: 3000
  vnc:
    enabled: true
    port: 3000
EOF

# 2. Create a session
kubectl apply -f - <<EOF
apiVersion: stream.streamspace.io/v1alpha1
kind: Session
metadata:
  name: test-session
  namespace: streamspace
spec:
  user: testuser
  template: test-firefox
  state: running
  persistentHome: true
EOF

# 3. Verify resources created
kubectl get sessions,deployments,services,pvcs -n streamspace

# 4. Check session status
kubectl describe session test-session -n streamspace

# 5. Get pod logs
kubectl logs -n streamspace -l session=test-session
```

### Test Hibernation

```bash
# 1. Hibernate the session
kubectl patch session test-session -n streamspace \
  --type merge -p '{"spec":{"state":"hibernated"}}'

# 2. Verify deployment scaled to 0
kubectl get deployment -n streamspace -l session=test-session

# 3. Resume the session
kubectl patch session test-session -n streamspace \
  --type merge -p '{"spec":{"state":"running"}}'

# 4. Verify deployment scaled to 1
kubectl get deployment -n streamspace -l session=test-session
```

### Test Idle Timeout

```bash
# 1. Create session with short idle timeout
kubectl apply -f - <<EOF
apiVersion: stream.streamspace.io/v1alpha1
kind: Session
metadata:
  name: timeout-test
  namespace: streamspace
spec:
  user: timeoutuser
  template: test-firefox
  state: running
  idleTimeout: 2m
EOF

# 2. Set lastActivity to 3 minutes ago
kubectl patch session timeout-test -n streamspace \
  --type merge --subresource status \
  -p '{"status":{"lastActivity":"'$(date -u -d '3 minutes ago' --iso-8601=seconds)'"}}'

# 3. Wait for hibernation controller (check every minute)
watch kubectl get session timeout-test -n streamspace

# Should automatically change to hibernated state
```

### Test Template Validation

```bash
# Test invalid template (missing baseImage)
kubectl apply -f - <<EOF
apiVersion: stream.streamspace.io/v1alpha1
kind: Template
metadata:
  name: invalid-template
  namespace: streamspace
spec:
  displayName: "Invalid Template"
  # Missing baseImage
EOF

# Check status
kubectl get template invalid-template -n streamspace -o jsonpath='{.status}'
# Should show state: Invalid
```

---

## CI/CD Testing

### GitHub Actions Workflow

```yaml
name: Controller Tests

on:
  pull_request:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install dependencies
        run: |
          cd controller
          go mod download

      - name: Run unit tests
        run: |
          cd controller
          make test

      - name: Run integration tests
        run: |
          cd controller
          make envtest
          make test-integration

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./controller/cover.out

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Create k3d cluster
        run: |
          curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
          k3d cluster create test --wait

      - name: Build and load image
        run: |
          cd controller
          make docker-build IMG=streamspace-controller:test
          k3d image import streamspace-controller:test

      - name: Deploy CRDs
        run: kubectl apply -f controller/config/crd/bases/

      - name: Deploy controller
        run: |
          cd controller
          make deploy IMG=streamspace-controller:test

      - name: Run E2E tests
        run: |
          cd tests/e2e
          go test -v ./...

      - name: Collect logs on failure
        if: failure()
        run: kubectl logs -n streamspace --all-containers
```

### Local CI Testing

```bash
# Simulate CI environment locally
make ci-test
```

---

## Troubleshooting Tests

### envtest Not Found

```bash
# Install envtest binaries
make envtest

# Or manually:
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
setup-envtest use 1.26
```

### Tests Timing Out

- Increase timeout in test specs:
  ```go
  const timeout = time.Second * 30  // Increased from 10
  ```
- Check for resource conflicts (e.g., existing resources not cleaned up)

### Flaky Tests

- Use `Eventually` with proper timeout and polling:
  ```go
  Eventually(func() error {
      return k8sClient.Get(ctx, key, &obj)
  }, timeout, interval).Should(Succeed())
  ```
- Avoid hard sleeps, use polling instead

### Test Isolation Issues

- Each test should clean up its resources:
  ```go
  AfterEach(func() {
      Expect(k8sClient.Delete(ctx, session)).To(Succeed())
  })
  ```
- Use unique names for test resources

---

## Best Practices

1. **Test Naming**: Use descriptive names that explain what is being tested
2. **Arrange-Act-Assert**: Structure tests clearly
3. **Mock External Dependencies**: Don't rely on real external services
4. **Fast Tests**: Unit tests should run in seconds, not minutes
5. **Deterministic**: Tests should not be flaky or depend on timing
6. **Clean Up**: Always clean up resources after tests
7. **Coverage**: Aim for >80% coverage, focus on critical paths

---

## Test Metrics

Track these metrics in CI:

- **Test Success Rate**: Should be 100%
- **Code Coverage**: Target 80%+
- **Test Duration**: Unit tests <1min, Integration <5min, E2E <10min
- **Flakiness Rate**: <1% (tests that sometimes pass/fail)

---

**For more information**:
- [Ginkgo Documentation](https://onsi.github.io/ginkgo/)
- [Gomega Documentation](https://onsi.github.io/gomega/)
- [Kubebuilder Testing Guide](https://book.kubebuilder.io/reference/testing.html)
