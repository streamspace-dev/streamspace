# Test VNC Streaming End-to-End

Test VNC streaming complete flow from browser to container.

Platform: $ARGUMENTS (k8s or docker)

## Test Flow

### 1. Session Creation
Create session with VNC-enabled template:
- Template: firefox-browser or similar VNC template
- Resources: 512Mi memory, 250m CPU
- User: test-user

Verify session created in database with state="pending"

### 2. VNC Tunnel Creation

**For K8s Agent**:
- Verify port-forward tunnel created (agent → pod:5900)
- Check RBAC permissions (pods/portforward)
- Confirm tunnel in agent logs

**For Docker Agent**:
- Verify VNC port mapped (container:5900 → host port)
- Check docker port mapping
- Confirm container VNC process running

### 3. Control Plane VNC Proxy

Test VNC proxy endpoint:
- GET /api/v1/sessions/{sessionId}/vnc
- Verify WebSocket upgrade
- Check proxy authentication
- Confirm routing to correct agent

### 4. WebSocket Connection Flow

Simulate browser connection:
```
Browser WebSocket → Control Plane VNC Proxy → Agent VNC Tunnel → Container VNC Server
```

Verify:
- WebSocket connection established
- Proxy forwards to correct agent pod
- Agent forwards to correct session
- VNC server accepts connection

### 5. Bidirectional Data Flow

Test data streaming:
- Send VNC protocol handshake
- Verify screen updates received
- Test keyboard input forwarded
- Test mouse events forwarded
- Measure latency (should be < 100ms for local)

### 6. Connection Stability

Test for 30-60 seconds:
- No disconnections
- Consistent frame rate
- No data corruption
- Memory usage stable

### 7. Connection Cleanup

Terminate session:
- Close WebSocket connection
- Verify proxy cleanup
- Check tunnel cleanup
- Confirm container/pod terminated
- Verify no resource leaks

## Verification Checklist

- [ ] Session created successfully
- [ ] VNC tunnel established
- [ ] VNC proxy accessible
- [ ] WebSocket connection working
- [ ] Screen updates received
- [ ] Input events forwarded
- [ ] Latency acceptable (< 100ms)
- [ ] Connection stable (no drops)
- [ ] Cleanup successful
- [ ] No resource leaks

## Performance Metrics

Measure and report:
- Session creation time
- VNC tunnel creation time
- First frame time (from connection to first screen update)
- Average latency
- Frame rate (fps)
- Memory usage (proxy, agent, container)

## Report Results

Create report in `.claude/reports/INTEGRATION_TEST_VNC_E2E_[K8S|DOCKER]_YYYY-MM-DD.md` with:
- Platform tested
- Test execution details
- All verification results
- Performance metrics
- Screenshots (if possible)
- Any issues encountered
- Recommendations

## Common Issues

If tests fail, check:
- VNC server running in container
- Port 5900 accessible
- Firewall rules
- WebSocket proxy configuration
- Agent tunnel implementation
- Network policies (K8s)
