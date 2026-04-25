# StreamSpace Docker Agent

The Docker Agent is a standalone binary that runs on a Docker host and connects TO the Control Plane via WebSocket. It receives commands from the Control Plane and manages session containers on the local Docker daemon.

## Architecture

**v2.0 (Agent-based)**:
```
Control Plane → WebSocket → Agent → Creates Container/Network/Volume
```

### Key Features

- **Outbound Connection**: Agent connects TO Control Plane (firewall-friendly)
- **Command-Driven**: Agent receives commands instead of polling
- **Centralized Control**: All session state managed by Control Plane
- **Multi-Platform**: Same architecture supports K8s, Docker, VMs, Cloud
- **Lightweight**: No Kubernetes required, runs on any Docker host

## Building

### Prerequisites

- Go 1.21+
- Docker daemon running
- Access to Docker socket (`/var/run/docker.sock`)

### Build Binary

```bash
cd agents/docker-agent
go build -o docker-agent .
```

### Build Container Image

```bash
docker build -t streamspace/docker-agent:v2.0 .
```

## Configuration

The agent can be configured via:
- Command-line flags
- Environment variables
- Configuration file (optional)

### Required Configuration

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--agent-id` | `AGENT_ID` | Unique agent identifier (e.g., `docker-prod-us-east-1`) |
| `--control-plane-url` | `CONTROL_PLANE_URL` | Control Plane WebSocket URL (e.g., `wss://control.example.com`) |

### Optional Configuration

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--platform` | `PLATFORM` | `docker` | Platform type |
| `--region` | `REGION` | - | Deployment region |
| `--docker-host` | `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon socket |
| `--network` | `NETWORK_NAME` | `streamspace` | Docker network name |
| `--volume-driver` | `VOLUME_DRIVER` | `local` | Docker volume driver |
| `--max-cpu` | `MAX_CPU` | `100` | Maximum CPU cores available |
| `--max-memory` | `MAX_MEMORY` | `128` | Maximum memory in GB |
| `--max-sessions` | `MAX_SESSIONS` | `100` | Maximum concurrent sessions |
| `--heartbeat-interval` | `HEALTH_CHECK_INTERVAL` | `30` | Heartbeat interval in seconds |
| `--api-key` | `API_KEY` | - | Agent API key for authentication (64 hex chars) |

### High Availability Configuration

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--enable-ha` | `ENABLE_HA` | `false` | Enable HA mode with leader election |
| `--leader-election-backend` | `LEADER_ELECTION_BACKEND` | `file` | Backend: `file`, `redis`, or `swarm` |
| `--redis-url` | `REDIS_URL` | - | Redis URL for redis backend (e.g., `redis://localhost:6379/0`) |
| `--lock-file-path` | `LOCK_FILE_PATH` | `/var/run/streamspace/agent.lock` | Lock file path for file backend |

**Leader Election Backends**:
- **`redis`** (Recommended for production): Distributed leader election using Redis SET NX with TTL. Best for multi-host deployments.
- **`file`**: File-based locking using flock. Only works on single-host deployments.
- **`swarm`**: Docker Swarm service labels. Native Swarm HA (requires Swarm mode).

## Deployment

### Option 1: Run as Binary

#### 1. Build the Agent

```bash
go build -o docker-agent .
```

#### 2. Run the Agent

```bash
./docker-agent \
  --agent-id=docker-prod-us-east-1 \
  --control-plane-url=wss://control.example.com \
  --region=us-east-1
```

### Option 2: Run as Docker Container

#### 1. Build Container Image

```bash
docker build -t streamspace/docker-agent:v2.0 .
```

#### 2. Create StreamSpace Network

```bash
docker network create streamspace
```

#### 3. Run Agent Container

```bash
docker run -d \
  --name streamspace-agent \
  --network streamspace \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AGENT_ID=docker-prod-us-east-1 \
  -e CONTROL_PLANE_URL=wss://control.example.com \
  -e REGION=us-east-1 \
  streamspace/docker-agent:v2.0
```

**Important**: The agent container needs access to the Docker socket (`/var/run/docker.sock`) to manage session containers.

### Option 3: Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  streamspace-agent:
    image: streamspace/docker-agent:v2.0
    container_name: streamspace-agent
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      AGENT_ID: docker-prod-us-east-1
      CONTROL_PLANE_URL: wss://control.example.com
      REGION: us-east-1
      MAX_CPU: 100
      MAX_MEMORY: 128
      MAX_SESSIONS: 100
    networks:
      - streamspace

networks:
  streamspace:
    driver: bridge
```

Run with:

```bash
docker-compose up -d
```

### Option 4: High Availability Deployment with Redis

For production deployments requiring failover and zero downtime, run multiple agent replicas with Redis-based leader election.

#### Prerequisites

- Redis server accessible to all agent instances
- Same `AGENT_ID` for all replicas (identifies the agent cluster)
- Unique hostnames for each replica (automatically used as instance ID)

#### 1. Deploy Redis (if not already available)

```bash
docker run -d \
  --name redis \
  --network streamspace \
  -p 6379:6379 \
  redis:7-alpine
```

#### 2. Deploy Agent Replicas

Create `docker-compose.ha.yml`:

```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: streamspace-redis
    restart: unless-stopped
    networks:
      - streamspace
    ports:
      - "6379:6379"

  agent-1:
    image: streamspace/docker-agent:v2.0
    container_name: streamspace-agent-1
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      AGENT_ID: docker-prod-cluster  # Same for all replicas
      CONTROL_PLANE_URL: wss://control.example.com
      API_KEY: ${AGENT_API_KEY}  # Required for authentication
      REGION: us-east-1
      ENABLE_HA: "true"
      LEADER_ELECTION_BACKEND: redis  # Use Redis backend
      REDIS_URL: redis://redis:6379/0
    networks:
      - streamspace
    depends_on:
      - redis

  agent-2:
    image: streamspace/docker-agent:v2.0
    container_name: streamspace-agent-2
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      AGENT_ID: docker-prod-cluster  # Same for all replicas
      CONTROL_PLANE_URL: wss://control.example.com
      API_KEY: ${AGENT_API_KEY}  # Required for authentication
      REGION: us-east-1
      ENABLE_HA: "true"
      LEADER_ELECTION_BACKEND: redis
      REDIS_URL: redis://redis:6379/0
    networks:
      - streamspace
    depends_on:
      - redis

  agent-3:
    image: streamspace/docker-agent:v2.0
    container_name: streamspace-agent-3
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      AGENT_ID: docker-prod-cluster  # Same for all replicas
      CONTROL_PLANE_URL: wss://control.example.com
      API_KEY: ${AGENT_API_KEY}  # Required for authentication
      REGION: us-east-1
      ENABLE_HA: "true"
      LEADER_ELECTION_BACKEND: redis
      REDIS_URL: redis://redis:6379/0
    networks:
      - streamspace
    depends_on:
      - redis

networks:
  streamspace:
    driver: bridge
```

#### 3. Set Agent API Key

```bash
export AGENT_API_KEY="your-64-char-hex-api-key"
```

#### 4. Deploy HA Stack

```bash
docker-compose -f docker-compose.ha.yml up -d
```

#### 5. Verify Leader Election

```bash
# Check logs - only one agent should be leader
docker logs streamspace-agent-1 | grep -i leader
docker logs streamspace-agent-2 | grep -i leader
docker logs streamspace-agent-3 | grep -i leader
```

Expected output (one leader, two standbys):
```
[LeaderElection] 🎖️  Became leader for agent: docker-prod-cluster
[DockerAgent] 🎖️  I am the LEADER - starting agent...
```

#### 6. Test Failover

```bash
# Stop the leader container
docker stop streamspace-agent-1

# Watch standby logs - one should become leader within 15 seconds
docker logs -f streamspace-agent-2
```

Expected output:
```
[LeaderElection] 🎖️  Became leader for agent: docker-prod-cluster
[DockerAgent] 🎖️  I am the LEADER - starting agent...
```

**Benefits of Redis Backend**:
- ✅ Automatic failover (typically 5-15 seconds)
- ✅ Works across multiple Docker hosts
- ✅ No shared filesystem required
- ✅ Battle-tested Redis reliability
- ✅ Simple to deploy and maintain

## Verification

### Check Agent Logs

```bash
# If running as binary
tail -f /var/log/streamspace/docker-agent.log

# If running in Docker
docker logs -f streamspace-agent
```

### Verify Connection

Look for these log messages:
```
[DockerAgent] Starting agent: docker-prod-us-east-1 (platform: docker, region: us-east-1)
[DockerAgent] Connecting to Control Plane...
[DockerAgent] Registered successfully (ID: xxx, Status: online)
[DockerAgent] WebSocket connected
[DockerAgent] Connected to Control Plane: wss://control.example.com
[Heartbeat] Sent heartbeat (activeSessions: 0)
```

### Check Agent Status in Control Plane

```bash
# Query Control Plane API
curl -X GET https://control.example.com/api/v1/agents/docker-prod-us-east-1 \
  -H "Authorization: Bearer $TOKEN"
```

## Session Lifecycle

When a session is created:

1. **Control Plane** sends `start_session` command via WebSocket
2. **Agent** receives command and:
   - Creates Docker network (if needed)
   - Creates volume for persistent storage (if needed)
   - Pulls container image
   - Creates and starts container
   - Waits for container to be running
   - Creates VNC tunnel (if VNC enabled)
3. **Agent** reports success/failure back to Control Plane
4. **Control Plane** updates session status in database

## Troubleshooting

### Agent Cannot Connect to Control Plane

Check:
- Control Plane URL is accessible from agent host
- Firewall allows outbound connections to Control Plane
- TLS certificates are valid (if using wss://)

```bash
# Test WebSocket connection
wscat -c wss://control.example.com/api/v1/agents/connect?agent_id=test
```

### Agent Cannot Access Docker Daemon

Check:
- Docker socket exists: `ls -la /var/run/docker.sock`
- Agent has permission to access socket: `groups` (should include `docker`)
- Docker daemon is running: `docker info`

If running as container:
- Socket is mounted: `-v /var/run/docker.sock:/var/run/docker.sock`

### Session Containers Not Starting

Check:
- Docker network exists: `docker network ls | grep streamspace`
- Image can be pulled: `docker pull <image>`
- Resource limits are valid: CPU/memory settings
- Agent logs for error messages

```bash
docker logs streamspace-agent | grep ERROR
```

## Security Considerations

### Docker Socket Access

The agent requires access to the Docker socket (`/var/run/docker.sock`). This provides **root-equivalent** access to the host system.

**Security Best Practices**:
- Run agent in dedicated environment (isolated host or VM)
- Use Docker socket proxy (e.g., [tecnativa/docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy)) to limit API access
- Monitor agent logs for suspicious activity
- Implement resource quotas to prevent resource exhaustion

### Network Isolation

Session containers run on the same Docker network as the agent by default. Consider:
- Using custom network driver (e.g., overlay) for isolation
- Implementing network policies via firewall rules
- Running agent and sessions on dedicated Docker host

### Volume Security

Persistent volumes are created on the local Docker host. Consider:
- Using volume encryption
- Implementing backup strategy
- Setting volume quotas
- Using NFS or other network storage for multi-host setups

## Development

### Running Tests

```bash
go test ./...
```

### Local Development

For local development against Control Plane running on localhost:

```bash
./docker-agent \
  --agent-id=docker-dev-local \
  --control-plane-url=ws://localhost:8000 \
  --docker-host=unix:///var/run/docker.sock
```

### Debugging

Enable verbose logging:

```bash
LOG_LEVEL=debug ./docker-agent --agent-id=test --control-plane-url=ws://localhost:8000
```

## TODO

The following features are planned but not yet implemented:

- [ ] Command handlers (start/stop/hibernate/wake session)
- [ ] Docker operations module (container/network/volume management)
- [ ] Message handler (WebSocket message processing)
- [ ] VNC tunnel support (port-forward to session containers)
- [ ] Resource monitoring and reporting
- [ ] Session auto-hibernation
- [ ] Health checks and auto-recovery
- [ ] Metrics and logging improvements

## Contributing

See the main [StreamSpace CONTRIBUTING.md](../../CONTRIBUTING.md) for contribution guidelines.

## License

See the main [StreamSpace LICENSE](../../LICENSE) file.
