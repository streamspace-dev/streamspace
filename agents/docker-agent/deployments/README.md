# Docker Agent Deployments

This directory contains deployment configurations for the StreamSpace Docker Agent across different environments and orchestration platforms.

## Directory Structure

```
deployments/
├── compose/                    # Docker Compose configurations
│   ├── docker-compose.standalone.yaml     # Single instance (no HA)
│   ├── docker-compose.ha-file.yaml        # HA with file backend
│   └── docker-compose.ha-redis.yaml       # HA with Redis backend
├── swarm/                      # Docker Swarm configurations
│   └── docker-swarm.yaml                  # Swarm service with Swarm backend
├── systemd/                    # Systemd service configurations
│   ├── docker-agent.service               # Systemd unit file
│   └── docker-agent.env.example          # Environment configuration
└── README.md                   # This file
```

## Deployment Options

### 1. Docker Compose - Standalone Mode

**Use Case**: Development, testing, simple deployments without HA

**File**: `compose/docker-compose.standalone.yaml`

**Features**:
- Single docker-agent instance
- No leader election
- Simplest deployment option

**Usage**:
```bash
cd compose
docker-compose -f docker-compose.standalone.yaml up -d
```

**Configuration**:
```bash
export AGENT_ID=docker-agent-1
export CONTROL_PLANE_URL=ws://localhost:8000
docker-compose -f docker-compose.standalone.yaml up -d
```

---

### 2. Docker Compose - HA Mode with File Backend

**Use Case**: Single Docker host with multiple agent processes, simple HA

**File**: `compose/docker-compose.ha-file.yaml`

**Features**:
- Multiple replicas with leader election
- File-based locking (flock)
- Shared lock file via Docker volume
- Automatic failover (~15-20 seconds)

**Usage**:
```bash
cd compose
docker-compose -f docker-compose.ha-file.yaml up -d --scale docker-agent=3
```

**How it Works**:
- Uses `flock` (file locking) for leader election
- Only one replica is active at a time
- Standby replicas wait for leadership
- Lock file stored in shared Docker volume

**Limitations**:
- Only works on single Docker host
- Not suitable for multi-host deployments

---

### 3. Docker Compose - HA Mode with Redis Backend

**Use Case**: Multi-host Docker deployments without orchestration

**File**: `compose/docker-compose.ha-redis.yaml`

**Features**:
- Multiple replicas across multiple hosts
- Redis-based leader election
- Atomic operations via Lua scripts
- Automatic failover (~15-20 seconds)

**Usage**:
```bash
cd compose

# Option 1: Use bundled Redis (for testing)
docker-compose -f docker-compose.ha-redis.yaml up -d --scale docker-agent=3

# Option 2: Use external Redis (for production)
export REDIS_URL=redis://redis.example.com:6379/0
docker-compose -f docker-compose.ha-redis.yaml up -d --scale docker-agent=3
```

**How it Works**:
- Uses Redis `SET NX` with TTL for leader election
- Leader sets key with instance ID and TTL
- Lua scripts ensure atomic operations
- Works across multiple Docker hosts

**Requirements**:
- Redis server accessible to all agents
- Network connectivity between agents and Redis

---

### 4. Docker Swarm - Swarm-Native HA

**Use Case**: Production Docker Swarm clusters, native Swarm orchestration

**File**: `swarm/docker-swarm.yaml`

**Features**:
- Docker Swarm service with multiple replicas
- Swarm-native leader election
- Leverages Swarm's distributed consensus
- Automatic failover via Swarm scheduling

**Usage**:
```bash
# Initialize Swarm (if not already)
docker swarm init

# Deploy stack
docker stack deploy -c swarm/docker-swarm.yaml streamspace-agent

# Scale agent
docker service scale streamspace-agent_docker-agent=5

# View service status
docker service ps streamspace-agent_docker-agent

# Remove stack
docker stack rm streamspace-agent
```

**How it Works**:
- Uses Docker Swarm service labels for leader election
- Updates service labels atomically via Swarm API
- Leverages Swarm's Raft consensus
- Requires manager node access

**Requirements**:
- Docker Swarm mode enabled
- Manager node access (for service label updates)
- `/var/run/docker.sock` mounted

---

### 5. Systemd Service - Bare Metal Deployment

**Use Case**: Traditional Linux servers, VMs, bare-metal deployments

**Files**:
- `systemd/docker-agent.service` - Systemd unit file
- `systemd/docker-agent.env.example` - Environment configuration

**Features**:
- Runs as system service
- Automatic restart on failure
- Journal logging
- Security hardening

**Installation**:
```bash
# 1. Copy binary
sudo cp docker-agent /usr/local/bin/docker-agent
sudo chmod +x /usr/local/bin/docker-agent

# 2. Copy systemd unit
sudo cp systemd/docker-agent.service /etc/systemd/system/

# 3. Create environment file
sudo mkdir -p /etc/streamspace
sudo cp systemd/docker-agent.env.example /etc/streamspace/docker-agent.env
sudo chmod 600 /etc/streamspace/docker-agent.env

# 4. Edit configuration
sudo vi /etc/streamspace/docker-agent.env

# 5. Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable docker-agent
sudo systemctl start docker-agent
```

**Usage**:
```bash
# Check status
sudo systemctl status docker-agent

# View logs
sudo journalctl -u docker-agent -f

# Restart service
sudo systemctl restart docker-agent

# Stop service
sudo systemctl stop docker-agent
```

**HA Mode with Systemd**:

For HA deployments, run multiple systemd services on different hosts:

**Example: File Backend (Single Host)**
```bash
# /etc/streamspace/docker-agent.env
AGENT_ID=docker-agent-prod
ENABLE_HA=true
LEADER_ELECTION_BACKEND=file
LOCK_FILE_PATH=/var/run/streamspace/docker-agent-prod.lock
```

**Example: Redis Backend (Multi-Host)**
```bash
# /etc/streamspace/docker-agent.env
AGENT_ID=docker-agent-prod
ENABLE_HA=true
LEADER_ELECTION_BACKEND=redis
REDIS_URL=redis://redis.example.com:6379/0
```

---

## Leader Election Backends

### File Backend

**Best For**: Single Docker host, development, testing

**How it Works**:
- Uses `flock` (file locking) for exclusive access
- Lock file stored locally or in shared volume
- Only works on single host (not NFS)

**Configuration**:
```yaml
environment:
  ENABLE_HA: "true"
  LEADER_ELECTION_BACKEND: "file"
  LOCK_FILE_PATH: "/var/run/streamspace/agent.lock"
volumes:
  - leader-locks:/var/run/streamspace
```

---

### Redis Backend

**Best For**: Multi-host deployments, production without Swarm

**How it Works**:
- Uses Redis `SET NX` with TTL
- Atomic operations via Lua scripts
- Works across multiple hosts

**Configuration**:
```yaml
environment:
  ENABLE_HA: "true"
  LEADER_ELECTION_BACKEND: "redis"
  REDIS_URL: "redis://redis:6379/0"
```

---

### Swarm Backend

**Best For**: Docker Swarm clusters, native Swarm orchestration

**How it Works**:
- Uses Docker Swarm service labels
- Atomic updates via Swarm API
- Leverages Swarm's Raft consensus

**Configuration**:
```yaml
environment:
  ENABLE_HA: "true"
  LEADER_ELECTION_BACKEND: "swarm"
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

**Requirements**:
- Swarm mode enabled
- Agent running as Swarm service
- Manager node access

---

## Environment Variables

All deployment methods support the same environment variables:

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `AGENT_ID` | Unique agent identifier | `docker-prod-us-east-1` |
| `CONTROL_PLANE_URL` | Control Plane WebSocket URL | `wss://control.example.com` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `PLATFORM` | Platform type | `docker` |
| `REGION` | Deployment region | `default` |
| `DOCKER_HOST` | Docker daemon socket | `unix:///var/run/docker.sock` |
| `NETWORK_NAME` | Docker network name | `streamspace` |
| `VOLUME_DRIVER` | Docker volume driver | `local` |
| `MAX_CPU` | Maximum CPU cores | `100` |
| `MAX_MEMORY` | Maximum memory (GB) | `128` |
| `MAX_SESSIONS` | Maximum concurrent sessions | `100` |
| `HEALTH_CHECK_INTERVAL` | Heartbeat interval | `30s` |

### High Availability

| Variable | Description | Default |
|----------|-------------|---------|
| `ENABLE_HA` | Enable HA mode | `false` |
| `LEADER_ELECTION_BACKEND` | Backend type | `file` |
| `LOCK_FILE_PATH` | Lock file path (file backend) | `/var/run/streamspace/agent.lock` |
| `REDIS_URL` | Redis URL (redis backend) | - |

---

## Comparison Matrix

| Feature | Standalone | HA File | HA Redis | HA Swarm | Systemd |
|---------|-----------|---------|----------|----------|---------|
| High Availability | ❌ | ✅ | ✅ | ✅ | ✅* |
| Multi-Host Support | ❌ | ❌ | ✅ | ✅ | ✅* |
| Automatic Failover | ❌ | ✅ | ✅ | ✅ | ✅* |
| External Dependencies | None | None | Redis | Swarm | Redis* |
| Deployment Complexity | Low | Low | Medium | Medium | Medium |
| Production Ready | Testing | Testing | ✅ | ✅ | ✅ |

\* Systemd supports HA when configured with appropriate backend (file/redis)

---

## Quick Start Guide

### Development (Standalone)

```bash
cd compose
export AGENT_ID=docker-dev
export CONTROL_PLANE_URL=ws://localhost:8000
docker-compose -f docker-compose.standalone.yaml up -d
```

### Staging (HA with File Backend)

```bash
cd compose
export AGENT_ID=docker-staging
export CONTROL_PLANE_URL=ws://staging.example.com
docker-compose -f docker-compose.ha-file.yaml up -d --scale docker-agent=3
```

### Production (HA with Redis Backend)

```bash
cd compose
export AGENT_ID=docker-prod
export CONTROL_PLANE_URL=wss://prod.example.com
export REDIS_URL=redis://redis.prod.example.com:6379/0
docker-compose -f docker-compose.ha-redis.yaml up -d --scale docker-agent=5
```

### Production Swarm (Swarm Backend)

```bash
docker swarm init
docker stack deploy -c swarm/docker-swarm.yaml streamspace-agent
docker service scale streamspace-agent_docker-agent=5
```

---

## Troubleshooting

### Check Agent Logs

**Docker Compose**:
```bash
docker-compose -f docker-compose.*.yaml logs -f docker-agent
```

**Docker Swarm**:
```bash
docker service logs -f streamspace-agent_docker-agent
```

**Systemd**:
```bash
sudo journalctl -u docker-agent -f
```

### Check Leader Election Status

**File Backend**:
```bash
# Check lock file
cat /var/run/streamspace/docker-agent-*.lock
```

**Redis Backend**:
```bash
# Connect to Redis
redis-cli
> GET streamspace:agent:leader:docker-agent-prod
> TTL streamspace:agent:leader:docker-agent-prod
```

**Swarm Backend**:
```bash
# Check service labels
docker service inspect streamspace-agent_docker-agent --format '{{ json .Spec.Labels }}'
```

### Common Issues

**Issue**: Agent not connecting to Control Plane
- Check `CONTROL_PLANE_URL` is correct
- Verify network connectivity
- Check Control Plane logs

**Issue**: Multiple agents active (split-brain)
- File backend: Check lock file is on local filesystem (not NFS)
- Redis backend: Verify Redis is accessible
- Swarm backend: Ensure manager node access

**Issue**: Frequent failovers
- Check network stability
- Increase `HEALTH_CHECK_INTERVAL`
- Review agent logs for errors

---

## Security Considerations

1. **Docker Socket Access**: Agent requires access to `/var/run/docker.sock`
2. **Redis Credentials**: Use authentication for production Redis
3. **Environment Files**: Restrict permissions to `600` for systemd env files
4. **Network Security**: Use TLS for Control Plane connections (`wss://`)
5. **Resource Limits**: Set appropriate CPU/memory limits in production

---

## Production Recommendations

1. **Use HA Mode**: Always enable HA in production
2. **Choose Right Backend**:
   - Swarm clusters → Swarm backend
   - Multi-host without Swarm → Redis backend
   - Single host → File backend (for testing only)
3. **Monitor Leader Election**: Track failover events
4. **Test Failover**: Regularly test failover scenarios
5. **Resource Limits**: Configure appropriate limits
6. **Logging**: Centralize logs for all replicas
7. **Metrics**: Monitor agent health and performance

---

## Support

For additional help:
- Documentation: https://streamspace.dev/docs/agents/docker
- GitHub Issues: https://github.com/streamspace-dev/streamspace/issues
- Community: https://streamspace.dev/community
