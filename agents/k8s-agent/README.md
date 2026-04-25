# StreamSpace Kubernetes Agent

The Kubernetes Agent is a standalone binary that runs inside a Kubernetes cluster and connects TO the Control Plane via WebSocket. It receives commands from the Control Plane and manages session resources on the local Kubernetes cluster.

## Architecture

**v1.0 (Controller-based)**:
```
CRD (Session) → Controller watches → Creates Pod/Service/PVC
```

**v2.0 (Agent-based)**:
```
Control Plane → WebSocket → Agent → Creates Pod/Service/PVC
```

### Key Changes

- **Outbound Connection**: Agent connects TO Control Plane (firewall-friendly)
- **Command-Driven**: Agent receives commands instead of watching CRDs
- **Centralized Control**: All session state managed by Control Plane
- **Multi-Platform**: Same architecture supports K8s, Docker, VMs, Cloud

## Building

### Prerequisites

- Go 1.21+
- Docker (for container builds)

### Build Binary

```bash
cd agents/k8s-agent
go build -o k8s-agent .
```

### Build Container Image

```bash
docker build -t streamspace/k8s-agent:v2.0 .
```

## Configuration

The agent can be configured via:
- Command-line flags
- Environment variables
- ConfigMap (when running in Kubernetes)

### Required Configuration

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--agent-id` | `AGENT_ID` | Unique agent identifier (e.g., `k8s-prod-us-east-1`) |
| `--control-plane-url` | `CONTROL_PLANE_URL` | Control Plane WebSocket URL (e.g., `wss://control.example.com`) |

### Optional Configuration

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--platform` | `PLATFORM` | `kubernetes` | Platform type |
| `--region` | `REGION` | - | Deployment region |
| `--namespace` | `NAMESPACE` | `streamspace` | Kubernetes namespace for sessions |
| `--kubeconfig` | `KUBECONFIG` | - | Path to kubeconfig (empty for in-cluster) |
| `--max-cpu` | `MAX_CPU` | `100` | Maximum CPU cores available |
| `--max-memory` | `MAX_MEMORY` | `128` | Maximum memory in GB |
| `--max-sessions` | `MAX_SESSIONS` | `100` | Maximum concurrent sessions |

## Deployment

### 1. Create Namespace

```bash
kubectl create namespace streamspace
```

### 2. Apply RBAC Permissions

```bash
kubectl apply -f k8s/rbac.yaml
```

### 3. Configure Agent

Edit `k8s/deployment.yaml` and set:
- `AGENT_ID`: Unique identifier for this agent
- `CONTROL_PLANE_URL`: Your Control Plane WebSocket URL

### 4. Deploy Agent

```bash
kubectl apply -f k8s/deployment.yaml
```

### 5. Verify Deployment

```bash
kubectl -n streamspace get pods -l component=k8s-agent
kubectl -n streamspace logs -l component=k8s-agent
```

Expected log output:
```
[K8sAgent] Starting agent: k8s-prod-us-east-1 (platform: kubernetes, region: us-east-1)
[K8sAgent] Connecting to Control Plane...
[K8sAgent] Registered successfully: k8s-prod-us-east-1 (status: online)
[K8sAgent] WebSocket connected
[K8sAgent] Connected to Control Plane: wss://control.example.com
[K8sAgent] Starting heartbeat sender (interval: 10s)
```

## Local Development

### Running Locally

```bash
# Set environment variables
export AGENT_ID=k8s-dev-local
export CONTROL_PLANE_URL=ws://localhost:8000
export NAMESPACE=streamspace
export KUBECONFIG=~/.kube/config

# Run agent
go run . --agent-id=$AGENT_ID --control-plane-url=$CONTROL_PLANE_URL
```

### Testing with Control Plane

1. Start the Control Plane:
```bash
cd api
go run ./cmd/main.go
```

2. Start the K8s Agent:
```bash
cd agents/k8s-agent
go run . --agent-id=k8s-dev-local --control-plane-url=ws://localhost:8000
```

3. Send a test command via Control Plane API:
```bash
curl -X POST http://localhost:8000/api/v1/agents/k8s-dev-local/command \
  -H "Content-Type: application/json" \
  -d '{
    "action": "start_session",
    "sessionId": "test-session-123",
    "payload": {
      "sessionId": "test-session-123",
      "user": "testuser",
      "template": "firefox",
      "persistentHome": false,
      "memory": "2Gi",
      "cpu": "1000m"
    }
  }'
```

## Commands

The agent handles four command types:

### 1. start_session

Creates a new session with Deployment, Service, and optionally PVC.

**Payload**:
```json
{
  "sessionId": "sess-123",
  "user": "alice",
  "template": "firefox",
  "persistentHome": true,
  "memory": "2Gi",
  "cpu": "1000m"
}
```

### 2. stop_session

Deletes session resources.

**Payload**:
```json
{
  "sessionId": "sess-123",
  "deletePVC": false
}
```

### 3. hibernate_session

Scales session deployment to 0 replicas.

**Payload**:
```json
{
  "sessionId": "sess-123"
}
```

### 4. wake_session

Scales session deployment to 1 replica.

**Payload**:
```json
{
  "sessionId": "sess-123"
}
```

## WebSocket Protocol

The agent implements the StreamSpace v2.0 WebSocket protocol defined in `api/internal/models/agent_protocol.go`.

### Messages from Control Plane → Agent

- **command**: Execute a session command
- **ping**: Keep-alive ping
- **shutdown**: Graceful shutdown request

### Messages from Agent → Control Plane

- **heartbeat**: Regular status update (every 10 seconds)
- **ack**: Command acknowledged
- **complete**: Command completed successfully
- **failed**: Command failed
- **status**: Session status update
- **pong**: Ping response

## Monitoring

### Health Checks

The deployment includes liveness and readiness probes:

```yaml
livenessProbe:
  exec:
    command: [sh, -c, pgrep -x k8s-agent]
  initialDelaySeconds: 30
  periodSeconds: 30

readinessProbe:
  exec:
    command: [sh, -c, pgrep -x k8s-agent]
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Logs

View agent logs:
```bash
kubectl -n streamspace logs -f -l component=k8s-agent
```

### Metrics

Check agent status in Control Plane:
```bash
curl http://localhost:8000/api/v1/agents
```

## Troubleshooting

### Agent Not Connecting

**Check**:
1. Control Plane URL is correct and reachable
2. Control Plane is running and listening on WebSocket port
3. Network policies allow outbound connections
4. Agent has correct RBAC permissions

**Logs**:
```bash
kubectl -n streamspace logs -l component=k8s-agent
```

### Commands Failing

**Check**:
1. Agent has necessary RBAC permissions
2. Kubernetes resources (storage class, etc.) exist
3. Namespace exists and is accessible
4. Resource quotas are not exceeded

**Debugging**:
```bash
# Check agent logs
kubectl -n streamspace logs -l component=k8s-agent

# Check session resources
kubectl -n streamspace get deployments,services,pvcs -l app=streamspace-session

# Check pod status
kubectl -n streamspace get pods -l app=streamspace-session
kubectl -n streamspace describe pod <pod-name>
```

### Reconnection Issues

The agent implements exponential backoff for reconnection:
- 2s, 4s, 8s, 16s, 32s (max)

If reconnection fails after 5 attempts, the agent will exit and Kubernetes will restart it.

## Production Considerations

1. **High Availability**: Run multiple agent replicas across different nodes
2. **Resource Limits**: Set appropriate CPU/memory limits for agent pod
3. **Storage Classes**: Configure appropriate storage classes for PVCs
4. **Network Policies**: Ensure agent can reach Control Plane
5. **TLS/SSL**: Use `wss://` (not `ws://`) for secure WebSocket connections
6. **Monitoring**: Integrate with Prometheus/Grafana for metrics
7. **Alerting**: Set up alerts for agent disconnections or command failures

## License

Copyright (C) 2024 StreamSpace. All rights reserved.
