# Debug Kubernetes Issues

Debug Kubernetes deployment issues for StreamSpace.

## Get Overall Status
!kubectl get all -n streamspace

## Check Pod Details
!kubectl describe pods -n streamspace | grep -A 10 "Events:"

## Recent Events
!kubectl get events -n streamspace --sort-by='.lastTimestamp' | tail -20

## Common Issues to Check

1. **Image Pull Failures**:
   - Check image names and tags
   - Verify registry access
   - Check imagePullSecrets

2. **CrashLoopBackOff**:
   - Review application logs
   - Check environment variables
   - Verify database connectivity
   - Check resource limits

3. **Resource Constraints**:
   - CPU/Memory limits too low
   - Insufficient cluster resources
   - PVC not bound

4. **ConfigMap/Secret Missing**:
   - Required configs not created
   - Wrong namespace
   - Typos in names

5. **RBAC Permission Errors**:
   - ServiceAccount missing
   - Role/RoleBinding not configured
   - Missing CRD permissions (Templates, Sessions)

## Troubleshooting Steps

For each issue found:
1. Identify root cause from events/logs
2. Explain the problem clearly
3. Provide step-by-step fix
4. Show exact commands to run
5. Verify fix worked

If multiple issues, prioritize by:
- CRITICAL: Prevents deployment
- HIGH: Impacts functionality
- MEDIUM: Degraded performance
- LOW: Minor issues
