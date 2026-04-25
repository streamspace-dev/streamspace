# Test Agent Lifecycle

Test complete agent lifecycle (K8s or Docker).

Agent type: $ARGUMENTS (k8s or docker)

## Test Sequence

### 1. Agent Registration
- Start agent
- Verify WebSocket connection to Control Plane
- Check agent registration in database
- Confirm agent ID and metadata

### 2. Heartbeat Mechanism
- Wait 30 seconds
- Verify heartbeat messages sent
- Check `last_heartbeat` timestamp updated
- Confirm agent status = "online"

### 3. Session Creation Command
- Send `start_session` command from API
- Verify agent receives command
- Check command processing
- Monitor session provisioning

For K8s:
- Pod creation
- Service creation
- Template CRD application

For Docker:
- Container creation
- Network creation
- Volume creation

### 4. Session Status Updates
- Verify agent sends status updates
- Check session state transitions (pending → starting → running)
- Confirm VNC ready status
- Verify database sync

### 5. VNC Tunnel Creation
- Verify VNC tunnel established
- Check port-forward (K8s) or port mapping (Docker)
- Test tunnel accessibility
- Confirm VNC proxy can connect

### 6. Session Termination
- Send `stop_session` command
- Verify cleanup process
- Check resource deletion (pods, containers, networks, volumes)
- Confirm database state updated

### 7. Agent Deregistration
- Stop agent gracefully
- Verify cleanup
- Check WebSocket disconnection
- Confirm agent status updated

## Verification Checklist

- [ ] Agent connects successfully
- [ ] Heartbeats working (30s interval)
- [ ] Commands processed correctly
- [ ] Session provisioned successfully
- [ ] VNC tunnel operational
- [ ] Database state accurate
- [ ] Resource cleanup complete
- [ ] No resource leaks
- [ ] No error logs

## Report Results

Create report in `.claude/reports/AGENT_LIFECYCLE_TEST_[K8S|DOCKER]_YYYY-MM-DD.md` with:
- Test execution timestamp
- Agent type and version
- All test steps with pass/fail
- Performance metrics (timing for each step)
- Any issues found
- Recommendations
