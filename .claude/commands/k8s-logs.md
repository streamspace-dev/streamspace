# Fetch Kubernetes Component Logs

Fetch logs from StreamSpace components.

Component: $ARGUMENTS (api, k8s-agent, postgres, redis, or specific pod name)

!kubectl logs -n streamspace -l app.kubernetes.io/component=$ARGUMENTS --tail=100

## Analysis

Analyze logs for:

1. **Errors or Warnings**:
   - Stack traces
   - Error messages
   - Warning patterns

2. **Performance Issues**:
   - Slow queries
   - High latency
   - Resource constraints

3. **Connection Problems**:
   - WebSocket disconnections
   - Database connection failures
   - Redis connection issues

4. **Authentication Failures**:
   - Invalid credentials
   - Expired tokens
   - RBAC permission errors

5. **Agent Issues**:
   - Failed session provisioning
   - Command timeouts
   - VNC tunnel failures

## Output

Provide:
- Summary of issues found (if any)
- Severity level (CRITICAL, HIGH, MEDIUM, LOW)
- Suggested fixes with specific actions
- Related log lines with context

If no issues found, confirm logs look healthy.
