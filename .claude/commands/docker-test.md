# Test Docker Agent Locally

Test Docker Agent locally without Kubernetes.

## Start Test Environment
!docker-compose -f docker-compose.test.yml up -d

## Wait for Services
!sleep 5

## Verify Agent Connection
!docker logs streamspace-docker-agent --tail=50 | grep -E "Connected|Registered|Heartbeat"

## Test Session Creation

Create test session via API:
1. Send session creation request
2. Verify container created: `docker ps | grep streamspace-session`
3. Check VNC port mapping: `docker port <container> 5900`
4. Verify network isolation
5. Test session termination
6. Verify cleanup (container removed)

## Test Scenarios

1. **Basic Lifecycle**:
   - Session start → running → stop

2. **Hibernate/Wake**:
   - Create session
   - Hibernate (container stop, volume persist)
   - Wake (container restart)
   - Verify data persistence

3. **Multiple Sessions**:
   - Create 3-5 concurrent sessions
   - Verify isolation
   - Check resource limits
   - Clean up all

4. **Error Handling**:
   - Invalid template
   - Resource limit exceeded
   - Docker daemon issues

## Cleanup
!docker-compose -f docker-compose.test.yml down -v

Report results with:
- Test scenarios executed
- Pass/fail status
- Any issues found
- Performance metrics (creation time, etc.)
