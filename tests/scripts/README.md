# StreamSpace v2.0-beta.1 Integration Test Scripts

This directory contains executable integration test scripts for StreamSpace v2.0-beta.1 release validation.

## Overview

These tests validate the complete StreamSpace system including:
- Session lifecycle management
- Template management
- Agent failover and reliability
- Performance and capacity

## Quick Start

### 1. Setup Environment

```bash
# Navigate to scripts directory
cd tests/scripts

# Run environment setup
./setup_environment.sh
```

This will:
- Verify prerequisites (kubectl, helm, docker, jq)
- Build local images
- Deploy StreamSpace to k3s
- Setup port forwarding
- Generate authentication token

### 2. Source Environment Variables

```bash
# Load environment variables
source .env
```

This sets:
- `TOKEN` - Authentication token for API
- `API_BASE_URL` - API endpoint (http://localhost:8000)
- `NAMESPACE` - Kubernetes namespace (streamspace)

### 3. Verify Setup

```bash
./verify_environment.sh
```

This checks:
- Environment variables are set
- Kubernetes cluster is accessible
- StreamSpace pods are running
- API is responsive

### 4. Run Tests

```bash
# Run individual test
cd phase1
./test_1.1a_basic_session_creation.sh

# Or run all tests in a phase
for test in phase1/*.sh; do
  bash "$test"
done
```

## Test Structure

```
tests/scripts/
├── setup_environment.sh         # Environment setup
├── verify_environment.sh        # Verify setup
├── .env                         # Generated environment variables
├── helpers/                     # Helper scripts
│   ├── login.sh                 # Get authentication token
│   ├── create_session_and_wait.sh  # Create session helper
│   └── generate_resource_report.sh # Resource usage report
├── phase1/                      # Session Management Tests (6-8h)
│   ├── test_1.1a_basic_session_creation.sh
│   ├── test_1.1b_session_startup_time.sh
│   ├── test_1.1c_resource_provisioning.sh
│   ├── test_1.1d_vnc_browser_access.sh
│   ├── test_1.2_session_state_persistence.sh
│   ├── test_1.3_multi_user_concurrent.sh
│   └── test_1.4_session_hibernation.sh
├── phase2/                      # Template Management Tests (2-4h)
│   ├── test_2.1_template_creation.sh
│   ├── test_2.2_template_updates.sh
│   └── test_2.3_template_deletion.sh
├── phase3/                      # Agent Failover Tests (4-6h)
│   ├── test_3.3_agent_heartbeat.sh
│   └── test_3.4_load_balancing.sh
└── phase4/                      # Performance Tests (4-6h)
    ├── test_4.1_creation_throughput.sh
    ├── test_4.2_resource_profiling.sh
    ├── test_4.3_vnc_latency.sh
    └── test_4.4_concurrent_capacity.sh
```

## Test Phases

### Phase 1: Session Management (6-8 hours)
Tests core session lifecycle functionality:
- Session creation via API
- Resource allocation
- Pod creation and management
- VNC connectivity
- State persistence
- Multi-user isolation

**Run Phase 1:**
```bash
cd phase1
for test in test_*.sh; do
  echo "Running $test..."
  bash "$test" || echo "FAILED: $test"
  echo ""
done
```

### Phase 2: Template Management (2-4 hours)
Tests template CRUD operations:
- Template creation and validation
- Template updates
- Deletion safety

**Run Phase 2:**
```bash
cd phase2
for test in test_*.sh; do bash "$test"; done
```

### Phase 3: Agent Failover (4-6 hours)
Tests agent reliability:
- Agent heartbeat monitoring
- Load balancing across agents

**Note**: Tests 3.1 and 3.2 already completed in previous testing.

**Run Phase 3:**
```bash
cd phase3
for test in test_*.sh; do bash "$test"; done
```

### Phase 4: Performance (4-6 hours)
Tests system performance and capacity:
- Session creation throughput (target: ≥10/min)
- Resource usage profiling
- VNC latency measurement
- Concurrent session capacity

**Run Phase 4:**
```bash
cd phase4
for test in test_*.sh; do bash "$test"; done
```

## Helper Scripts

### login.sh
Authenticate and get JWT token:
```bash
TOKEN=$(./helpers/login.sh admin admin)
```

### create_session_and_wait.sh
Create session and wait for Running state:
```bash
SESSION_ID=$(./helpers/create_session_and_wait.sh "$TOKEN" "user1" "firefox-browser")
```

### generate_resource_report.sh
Generate resource usage report for a session:
```bash
./helpers/generate_resource_report.sh streamspace "my-session-name"
```

## Test Results

After running tests, document results using the report template:
- Template: `.claude/reports/templates/PHASE_TEST_REPORT_TEMPLATE.md`
- Save reports to: `.claude/reports/INTEGRATION_TEST_RESULTS_PHASE_[N]_[DATE].md`

## Prerequisites

### Required Tools
- **kubectl** (any recent version)
- **helm** (v3.x or v4.1+, NOT v4.0.x)
- **docker** (for building images)
- **jq** (for JSON parsing)
- **curl** (for API testing)
- **bc** (for calculations)

### Kubernetes Cluster
- Local: k3s or Docker Desktop Kubernetes
- Resources: Minimum 4 CPU, 8GB RAM
- Storage: NFS provisioner (included in setup)

## Troubleshooting

### Environment setup fails
```bash
# Check prerequisites
./verify_environment.sh

# Check kubectl connection
kubectl cluster-info

# Check helm version (must NOT be v4.0.x)
helm version
```

### Tests can't authenticate
```bash
# Re-run login
export TOKEN=$(./helpers/login.sh admin admin)

# Or source environment
source .env
```

### Pods not starting
```bash
# Check pod status
kubectl get pods -n streamspace

# Check pod logs
kubectl logs -n streamspace -l app=streamspace-api
kubectl logs -n streamspace -l app=streamspace-k8s-agent

# Check events
kubectl get events -n streamspace --sort-by='.lastTimestamp'
```

### Port forwarding not working
```bash
# Kill existing port forwards
pkill -f "kubectl port-forward.*streamspace"

# Restart port forward
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000
```

### Sessions not creating
```bash
# Check agent status
curl -s http://localhost:8000/api/v1/agents -H "Authorization: Bearer $TOKEN" | jq

# Check agent logs
kubectl logs -n streamspace -l app=streamspace-k8s-agent --tail=50

# Check API logs
kubectl logs -n streamspace -l app=streamspace-api --tail=50
```

## Cleanup

### Remove test sessions
```bash
# List all sessions
curl -s http://localhost:8000/api/v1/sessions -H "Authorization: Bearer $TOKEN" | jq

# Delete specific session
curl -X DELETE http://localhost:8000/api/v1/sessions/SESSION_ID \
  -H "Authorization: Bearer $TOKEN"
```

### Uninstall StreamSpace
```bash
helm uninstall streamspace -n streamspace
kubectl delete namespace streamspace
```

### Stop port forwarding
```bash
pkill -f "kubectl port-forward.*streamspace"
```

## Additional Resources

- **Integration Test Plan**: `.claude/reports/INTEGRATION_TEST_PLAN_v2.0-beta.1.md`
- **Test Report Template**: `.claude/reports/templates/PHASE_TEST_REPORT_TEMPLATE.md`
- **Project Documentation**: `../../README.md`
- **Architecture**: `../../docs/ARCHITECTURE.md`

## Support

For issues or questions:
1. Check troubleshooting section above
2. Review test plan: `.claude/reports/INTEGRATION_TEST_PLAN_v2.0-beta.1.md`
3. Check logs: `kubectl logs -n streamspace -l app=streamspace-api`
4. Open issue: https://github.com/streamspace-dev/streamspace/issues

---

**Note**: These tests are designed for v2.0-beta.1 release validation. Some features (like hibernation) may not be fully implemented and will be marked as skipped.
