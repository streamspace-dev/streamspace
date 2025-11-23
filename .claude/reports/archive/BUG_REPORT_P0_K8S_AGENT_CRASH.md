# BUG REPORT: P0 - K8s Agent Crashes on Startup (Heartbeat Ticker)

**Date**: 2025-11-21
**Reporter**: Agent 3 (Validator)
**Severity**: P0 - CRITICAL (Blocks all integration testing)
**Status**: NEW - Requires Builder (Agent 2) fix
**Branch**: `claude/v2-validator`

---

## Executive Summary

The K8s Agent successfully connects and registers with the Control Plane, but immediately crashes with a panic due to attempting to create a ticker with 0 duration. This is caused by the `HeartbeatInterval` configuration field not being loaded from the `HEALTH_CHECK_INTERVAL` environment variable.

**Impact**: **ALL 8 integration test scenarios are blocked** - the agent cannot stay running to handle commands.

---

## Bug Details

### Panic Stack Trace

```
2025/11/21 16:45:34 [K8sAgent] Starting agent: k8s-prod-cluster (platform: kubernetes, region: default)
2025/11/21 16:45:34 [K8sAgent] Connecting to Control Plane...
2025/11/21 16:45:34 [K8sAgent] Registered successfully: k8s-prod-cluster (status: online)
2025/11/21 16:45:34 [K8sAgent] WebSocket connected
2025/11/21 16:45:34 [K8sAgent] Connected to Control Plane: ws://streamspace-api:8000
panic: non-positive interval for NewTicker

goroutine 31 [running]:
time.NewTicker(0x0?)
	/usr/local/go/src/time/tick.go:22 +0xe5
main.(*K8sAgent).SendHeartbeats(0xc00012cde0)
	/app/main.go:454 +0x4f
created by main.(*K8sAgent).Run in goroutine 18
	/app/main.go:169 +0x190
```

### Root Cause Analysis

**File**: `agents/k8s-agent/main.go`
**Location**: Lines 244-257 (config creation)

The `AgentConfig` struct is initialized in the `main()` function, but the `HeartbeatInterval` field is never set:

```go
// Create agent configuration
config := &config.AgentConfig{
	AgentID:         *agentID,
	ControlPlaneURL: *controlPlaneURL,
	Platform:        *platform,
	Region:          *region,
	Namespace:       *namespace,
	KubeConfig:      *kubeConfig,
	Capacity: config.AgentCapacity{
		MaxCPU:      *maxCPU,
		MaxMemory:   *maxMemory,
		MaxSessions: *maxSessions,
	},
	// ❌ HeartbeatInterval is MISSING!
}
```

As a result:
1. `HeartbeatInterval` defaults to 0 (zero value for `int`)
2. `config.Validate()` is never called (or called too late)
3. When `SendHeartbeats()` is called, it creates: `interval := time.Duration(0) * time.Second` → 0 duration
4. `time.NewTicker(0)` panics with "non-positive interval for NewTicker"

**File**: `agents/k8s-agent/main.go`
**Location**: Line 453

```go
func (a *K8sAgent) SendHeartbeats() {
	interval := time.Duration(a.config.HeartbeatInterval) * time.Second  // ← 0 * time.Second = 0
	ticker := time.NewTicker(interval)  // ← PANIC: non-positive interval
	// ...
}
```

### Why This Bug Exists

The Helm chart passes `HEALTH_CHECK_INTERVAL` as an environment variable (lines 68-69 in `chart/templates/k8s-agent-deployment.yaml`):

```yaml
- name: HEALTH_CHECK_INTERVAL
  value: {{ .Values.k8sAgent.config.health.checkInterval | quote }}  # "30s"
```

And `values.yaml` sets it to `"30s"`:

```yaml
health:
  checkInterval: "30s"
```

But the agent code **never reads** the `HEALTH_CHECK_INTERVAL` environment variable. All other config fields are read via flags with `os.Getenv()` fallbacks (lines 224-232), but `HeartbeatInterval` is completely missing.

---

## Reproduction Steps

1. Deploy v2.0-beta with K8s Agent:
   ```bash
   helm install streamspace ./chart \
     --namespace streamspace \
     --create-namespace \
     --set k8sAgent.enabled=true \
     --set k8sAgent.image.tag=local \
     --wait
   ```

2. Check pod status:
   ```bash
   kubectl get pods -n streamspace
   ```
   **Result**: `streamspace-k8s-agent-xxx` is in `CrashLoopBackOff`

3. Check logs:
   ```bash
   kubectl logs -n streamspace streamspace-k8s-agent-xxx
   ```
   **Result**: Panic "non-positive interval for NewTicker"

---

## Expected Behavior

1. Agent should read `HEALTH_CHECK_INTERVAL` environment variable
2. Parse it as an integer (seconds)
3. Set `config.HeartbeatInterval` to the parsed value
4. Validate the config (ensuring heartbeat interval > 0)
5. Start heartbeat ticker with valid interval
6. Agent should run continuously, sending heartbeats to Control Plane

---

## Fix Required (For Builder - Agent 2)

### File: `agents/k8s-agent/main.go`

**Location**: Lines 224-233 (flag definitions)

**Add** heartbeat interval flag:

```go
// Command-line flags
agentID := flag.String("agent-id", os.Getenv("AGENT_ID"), "Agent ID (e.g., k8s-prod-us-east-1)")
controlPlaneURL := flag.String("control-plane-url", os.Getenv("CONTROL_PLANE_URL"), "Control Plane WebSocket URL")
platform := flag.String("platform", getEnvOrDefault("PLATFORM", "kubernetes"), "Platform type")
region := flag.String("region", os.Getenv("REGION"), "Deployment region")
namespace := flag.String("namespace", getEnvOrDefault("NAMESPACE", "streamspace"), "Kubernetes namespace for sessions")
kubeConfig := flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file (empty for in-cluster)")
maxCPU := flag.Int("max-cpu", 100, "Maximum CPU cores available")
maxMemory := flag.Int("max-memory", 128, "Maximum memory in GB")
maxSessions := flag.Int("max-sessions", 100, "Maximum concurrent sessions")

// ✅ ADD THIS:
heartbeatInterval := flag.Int("heartbeat-interval", getEnvIntOrDefault("HEALTH_CHECK_INTERVAL", 30), "Heartbeat interval in seconds")
```

**Location**: Lines 244-257 (config creation)

**Update** config initialization to include `HeartbeatInterval`:

```go
// Create agent configuration
config := &config.AgentConfig{
	AgentID:           *agentID,
	ControlPlaneURL:   *controlPlaneURL,
	Platform:          *platform,
	Region:            *region,
	Namespace:         *namespace,
	KubeConfig:        *kubeConfig,
	HeartbeatInterval: *heartbeatInterval,  // ✅ ADD THIS LINE
	Capacity: config.AgentCapacity{
		MaxCPU:      *maxCPU,
		MaxMemory:   *maxMemory,
		MaxSessions: *maxSessions,
	},
}
```

**Location**: After line 282 (helper functions)

**Add** helper function for parsing integer environment variables:

```go
// getEnvIntOrDefault returns environment variable value as int or default.
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Try parsing as duration string (e.g., "30s", "1m")
		if duration, err := time.ParseDuration(value); err == nil {
			return int(duration.Seconds())
		}
		// Try parsing as integer
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
```

**Location**: Line 259 (after config creation)

**Add** config validation call:

```go
// Create agent configuration
config := &config.AgentConfig{
	// ... (fields as above)
}

// ✅ ADD THIS:
if err := config.Validate(); err != nil {
	log.Fatalf("Invalid configuration: %v", err)
}

// Create agent
agent, err := NewK8sAgent(config)
// ...
```

---

## Testing After Fix

### Unit Test (Optional - can be added later)

```go
// agents/k8s-agent/main_test.go

func TestGetEnvIntOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{"Duration string", "30s", 30},
		{"Duration minutes", "2m", 120},
		{"Integer string", "45", 45},
		{"Empty string", "", 10}, // default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TEST_INTERVAL", tt.envValue)
				defer os.Unsetenv("TEST_INTERVAL")
			}
			result := getEnvIntOrDefault("TEST_INTERVAL", 10)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}
```

### Integration Test

After fix is applied:

```bash
# 1. Rebuild K8s Agent image
cd agents/k8s-agent
docker build -t streamspace/streamspace-k8s-agent:local .

# 2. Redeploy Helm chart
helm upgrade streamspace ./chart \
  --namespace streamspace \
  --set k8sAgent.image.tag=local \
  --wait

# 3. Verify agent is running
kubectl get pods -n streamspace
# Expected: streamspace-k8s-agent-xxx is Running (not CrashLoopBackOff)

# 4. Check logs for heartbeat messages
kubectl logs -n streamspace streamspace-k8s-agent-xxx --tail=20
# Expected: "Starting heartbeat sender (interval: 30s)"
#           No panic, continuous heartbeat logs
```

---

## Additional Issues (Optional - P1 Priority)

### Issue 1: `config.Validate()` Not Called

The `config.Validate()` function exists (lines 64-90 in `agents/k8s-agent/internal/config/config.go`) but is never called in `main()`. This function provides defaults and validation, including setting `HeartbeatInterval` to 10 if it's <= 0.

**Recommendation**: Call `config.Validate()` after creating the config struct (see fix above).

### Issue 2: Reconnection Backoff Not Loaded

The `ReconnectBackoff` field is also not being loaded from environment variables:
- Helm chart sets: `RECONNECT_INITIAL_DELAY`, `RECONNECT_MAX_DELAY`, `RECONNECT_MULTIPLIER` (lines 74-79)
- Agent code doesn't read these environment variables

**Impact**: Low priority - the `Validate()` function provides sensible defaults.

**Recommendation**: Add similar loading logic for reconnection config if needed for production deployments.

---

## Impact Assessment

### Blocked Functionality

**ALL integration test scenarios are completely blocked**:

1. ❌ **Agent Registration**: Agent connects and registers successfully, but then crashes immediately
2. ❌ **Session Creation**: Agent cannot handle commands (it's crashed)
3. ❌ **VNC Connection**: Requires agent to provision session pods
4. ❌ **VNC Streaming**: Requires agent to manage VNC tunnels
5. ❌ **Session Lifecycle**: Requires agent to handle commands
6. ❌ **Agent Failover**: Cannot test reconnection (agent crashes before disconnect)
7. ❌ **Concurrent Sessions**: Cannot create any sessions
8. ❌ **Error Handling**: Cannot test error scenarios (agent itself is the error)

### Release Impact

- **v2.0-beta Release**: **BLOCKED** - integration testing cannot begin
- **Expected Delay**: 2-4 hours for Builder to fix + rebuild + test
- **Testing Timeline**: Validator can resume integration testing once fix is deployed

---

## Success Criteria

After fix is applied, the following should be verified:

✅ **Agent Starts Successfully**:
- Pod status: `Running` (not `CrashLoopBackOff`)
- No panic in logs
- Log message: "Starting heartbeat sender (interval: XXs)"

✅ **Heartbeats Sent**:
- Check Control Plane logs for heartbeat reception
- Or check API database for agent heartbeat updates
- Verify agent status remains "online" in database

✅ **Configuration Loaded**:
- Verify `HEALTH_CHECK_INTERVAL` is read correctly
- Test with different values (10s, 30s, 1m) to ensure parsing works
- Verify defaults are applied when env var is missing

✅ **Integration Testing Can Proceed**:
- Validator (Agent 3) can begin Test Scenario 1: Agent Registration
- Agent remains running for extended period (>5 minutes)
- Agent can receive and handle commands from Control Plane

---

## Notes for Builder (Agent 2)

### Priority

**P0 - CRITICAL**: This is the **highest priority** bug blocking the v2.0-beta release. Integration testing cannot proceed without a running agent.

### Estimated Effort

- **Code Changes**: 15-20 lines across 3 locations
- **Testing**: 5-10 minutes (rebuild image + redeploy + verify)
- **Total Time**: 30-60 minutes

### Implementation Order

1. Add `getEnvIntOrDefault()` helper function
2. Add `heartbeatInterval` flag definition
3. Update config initialization to include `HeartbeatInterval`
4. Add `config.Validate()` call
5. Rebuild Docker image
6. Test deployment

### Testing Checklist

- [ ] Agent pod status is `Running`
- [ ] Agent logs show "Starting heartbeat sender"
- [ ] No panic in agent logs
- [ ] Heartbeats appear in Control Plane logs
- [ ] Agent stays running for at least 5 minutes
- [ ] Validator confirms Test Scenario 1 can proceed

---

## Related Files

- `agents/k8s-agent/main.go` (lines 220-283) - Main entry point
- `agents/k8s-agent/internal/config/config.go` (lines 11-46) - Config struct
- `chart/templates/k8s-agent-deployment.yaml` (lines 68-79) - Helm template with env vars
- `chart/values.yaml` (lines 660-676) - Default health check config

---

**Status**: REPORTED - Awaiting Builder (Agent 2) fix

**Next Steps**:
1. Builder applies fix to `claude/v2-builder` branch
2. Architect integrates fix into `feature/streamspace-v2-agent-refactor`
3. Validator pulls update and redeploys
4. Validator resumes integration testing (Test Scenario 1)
