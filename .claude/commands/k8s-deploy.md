# Deploy to Kubernetes

Deploy StreamSpace to Kubernetes cluster.

## Verify Cluster Connectivity
!kubectl cluster-info

## Deploy Components
!kubectl apply -f manifests/

## Check Deployment Status
!kubectl get pods -n streamspace
!kubectl get services -n streamspace
!kubectl get deployments -n streamspace

## Verify Components
After deployment, verify:

1. **All pods running**:
   - streamspace-api
   - streamspace-k8s-agent
   - streamspace-postgres
   - streamspace-redis (if HA enabled)

2. **Services accessible**:
   - API service (8000)
   - PostgreSQL (5432)
   - Redis (6379)

3. **Agents connected**:
   - Check API logs for agent registration
   - Verify heartbeat messages

4. **Database migrations applied**:
   - Check API startup logs

If any issues found:
- Show detailed error messages
- Check pod events: `kubectl describe pod <name> -n streamspace`
- Review logs: `kubectl logs <pod> -n streamspace`
- Suggest fixes (image pull errors, resource constraints, etc.)
- Offer to troubleshoot with `/k8s-debug`
