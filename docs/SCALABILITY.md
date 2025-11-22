# StreamSpace Horizontal Scalability Guide

**Version**: v2.0-beta
**Last Updated**: 2025-11-22
**Status**: Production Ready

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Component Scalability](#component-scalability)
4. [Configuration Guide](#configuration-guide)
5. [Deployment Examples](#deployment-examples)
6. [Performance Tuning](#performance-tuning)
7. [Monitoring & Troubleshooting](#monitoring--troubleshooting)
8. [Best Practices](#best-practices)

---

## Overview

StreamSpace v2.0-beta is designed for **horizontal scalability** across all major components. This guide covers how to scale your StreamSpace deployment from a single-node development setup to a multi-node production cluster serving thousands of users.

### What's Horizontally Scalable?

| Component | Scalability | Min Replicas | Max Replicas | Notes |
|-----------|-------------|--------------|--------------|-------|
| **API Server** | ✅ Full | 1 | Unlimited | Requires Redis for multi-pod AgentHub |
| **UI Server** | ✅ Full | 1 | Unlimited | Stateless React app |
| **Agents (Multi-Cluster)** | ✅ Full | 1 per cluster | 1000+ clusters | Different agent per cluster |
| **k8s-Agent (HA)** | ✅ Full | 1 | Unlimited | Leader election with Kubernetes Leases |
| **docker-Agent (HA)** | ✅ Full | 1 | Unlimited | Multi-backend leader election (file/redis/swarm) |
| **PostgreSQL** | ⚠️ External | 1 | N/A | Use PostgreSQL HA solution |
| **Redis** | ⚠️ External | 1 | N/A | Use Redis Sentinel/Cluster |

### Key Features

- **Stateless API**: JWT sessions and agent state stored in Redis
- **Load Balanced Connections**: UI and agents can connect to any API pod
- **Cross-Pod Command Routing**: Redis pub/sub routes commands between pods
- **Automatic Failover**: Agent reconnections work with any available API pod
- **Zero Downtime Scaling**: Add/remove replicas without disrupting sessions

---

## Architecture

### Single-Pod Architecture (Development)

```
┌─────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                      │
│                                                              │
│  ┌──────────┐      ┌──────────┐      ┌──────────────────┐  │
│  │    UI    │─────▶│   API    │─────▶│   PostgreSQL     │  │
│  │  Pod 1   │      │  Pod 1   │      │   (Single Node)  │  │
│  └──────────┘      └──────────┘      └──────────────────┘  │
│                         │                                    │
│                         ▼                                    │
│                  ┌──────────────┐                           │
│                  │  k8s-agent   │                           │
│                  │   Pod 1      │                           │
│                  └──────────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

**Characteristics:**
- Single replica of each component
- No Redis required
- AgentHub uses in-memory connections
- Suitable for development/testing
- **Limitation**: Not highly available

---

### Multi-Pod Architecture (Production)

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                            │
│                                                                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                           │
│  │   UI 1   │  │   UI 2   │  │   UI 3   │◀─── Load Balancer         │
│  └──────────┘  └──────────┘  └──────────┘                           │
│        │             │             │                                  │
│        └─────────────┴─────────────┘                                  │
│                      ▼                                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                           │
│  │  API 1   │  │  API 2   │  │  API 3   │◀─── Load Balancer         │
│  │ POD_NAME │  │ POD_NAME │  │ POD_NAME │                           │
│  └──────────┘  └──────────┘  └──────────┘                           │
│        │             │             │                                  │
│        └─────────────┴─────────────┘                                  │
│                      ▼                                                │
│           ┌──────────────────────┐                                   │
│           │   Redis (Shared)     │                                   │
│           │  - DB 0: Cache       │                                   │
│           │  - DB 1: AgentHub    │                                   │
│           └──────────────────────┘                                   │
│                      ▼                                                │
│           ┌──────────────────────┐                                   │
│           │    PostgreSQL        │                                   │
│           │   (Single/Cluster)   │                                   │
│           └──────────────────────┘                                   │
│                                                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ k8s-agent    │  │ k8s-agent    │  │ k8s-agent    │              │
│  │ (Cluster A)  │  │ (Cluster B)  │  │ (Cluster C)  │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│         │                  │                  │                       │
│         └──────────────────┴──────────────────┘                       │
│                            ▲                                          │
│              Connect to any API pod (WebSocket)                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Characteristics:**
- Multiple replicas of UI and API
- Redis required for AgentHub state sharing
- Agents can connect to any API pod
- Commands route via Redis pub/sub
- High availability and load balancing
- **Production Ready**

---

## Component Scalability

### 1. API Server

#### How It Scales

**Session Management:**
- JWT tokens validated against Redis-backed session store
- Any API pod can validate any user session
- Session invalidation propagates across all pods

**AgentHub (Agent Connection Management):**
- **Without Redis (Single-Pod)**:
  - Connections stored in-memory
  - Only one API replica supported
  - Agent must reconnect to same pod

- **With Redis (Multi-Pod)**:
  - Agent→Pod mapping stored in Redis
  - Connection state shared across pods
  - Commands route via Redis pub/sub
  - Agents can connect to any pod

**WebSocket Connections:**
- UI WebSocket connections work with any API pod
- Load balancer distributes connections
- Session state persists across reconnections

#### Configuration

**Enable Multi-Pod Mode:**
```yaml
# values.yaml
api:
  replicaCount: 3  # Scale to 3 replicas

redis:
  enabled: true  # Required for multi-pod
  agentHubEnabled: true  # Enable AgentHub Redis
```

**Environment Variables:**
- `AGENTHUB_REDIS_ENABLED=true` - Enables Redis for AgentHub
- `POD_NAME` - Auto-injected by Kubernetes (used for pub/sub routing)
- `REDIS_HOST` - Redis server address
- `REDIS_PORT` - Redis server port (default: 6379)

#### Scaling Commands

```bash
# Scale up to 5 replicas
kubectl scale deployment streamspace-api --replicas=5 -n streamspace

# Scale down to 2 replicas
kubectl scale deployment streamspace-api --replicas=2 -n streamspace

# Check pod status
kubectl get pods -n streamspace -l app.kubernetes.io/component=api
```

---

### 2. UI Server

#### How It Scales

**Stateless React App:**
- Static files served via nginx
- No server-side state
- No Redis required
- Unlimited horizontal scaling

**API Communication:**
- REST API calls to `/api/*`
- WebSocket connections to `/api/v1/ws/*`
- Load balancer distributes requests
- Session cookies work across all pods

#### Configuration

```yaml
# values.yaml
ui:
  replicaCount: 3  # Scale to 3 replicas
```

#### Scaling Commands

```bash
# Scale up to 10 replicas
kubectl scale deployment streamspace-ui --replicas=10 -n streamspace

# Enable autoscaling
kubectl autoscale deployment streamspace-ui \
  --min=2 --max=10 --cpu-percent=70 -n streamspace
```

---

### 3. Agents (k8s-agent)

#### How It Scales

**Multi-Cluster Architecture:**
- **One agent per Kubernetes cluster**
- Each agent has unique `agentId`
- Example: `k8s-prod-us-east-1`, `k8s-staging-eu-west-1`
- Agents connect to any available API pod
- Agent state shared via Redis across API pods

**Agent Connection Flow:**
1. Agent connects to API WebSocket endpoint
2. Registers with unique `agentId`
3. API stores `agent:{agentId}:pod` → pod name in Redis
4. Heartbeats every 30 seconds (refreshes 5-minute TTL)
5. If disconnected, reconnects to any available API pod

**Command Routing:**
- API receives command for agent
- Checks if agent connected locally (fastest)
- If not local, looks up pod in Redis
- Publishes command to pod-specific channel: `pod:{podName}:commands`
- Target pod forwards command to local WebSocket connection

#### Configuration

**Deploy Agent for Cluster A:**
```yaml
# values.yaml (Cluster A)
k8sAgent:
  enabled: true
  replicaCount: 1  # One per cluster
  config:
    agentId: "k8s-prod-us-east-1"
    controlPlaneUrl: "wss://streamspace-api.example.com"
    region: "us-east-1"
```

**Deploy Agent for Cluster B:**
```yaml
# values.yaml (Cluster B)
k8sAgent:
  enabled: true
  replicaCount: 1
  config:
    agentId: "k8s-prod-eu-west-1"
    controlPlaneUrl: "wss://streamspace-api.example.com"
    region: "eu-west-1"
```

#### Agent HA (High Availability)

**Current Status:** ✅ **Implemented** (v2.0)

**Features:**
- Leader election using Kubernetes Leases
- Active-Standby failover pattern
- Only one active agent replica at a time
- Automatic failover in ~15-20 seconds on leader failure
- Graceful leader handoff on shutdown

**How It Works:**
1. Multiple k8s-agent replicas deployed in same cluster
2. All replicas participate in leader election
3. Only the leader processes agent operations
4. Standby replicas wait for leadership
5. If leader fails, standby automatically takes over
6. On leader shutdown, leadership released gracefully

**Configuration:**
```yaml
# values.yaml
k8sAgent:
  enabled: true
  replicaCount: 3  # Deploy 3 replicas for HA
  ha:
    enabled: true  # Enable leader election
```

**Environment Variables:**
- `ENABLE_HA=true` - Enables leader election mode
- `POD_NAME` - Auto-injected (identifies replica for leader election)

**Leader Election Parameters:**
- **Lease Duration**: 15 seconds (how long leader holds lease)
- **Renew Deadline**: 10 seconds (how often leader renews lease)
- **Retry Period**: 2 seconds (how often standby checks for leadership)

**Verify HA Status:**
```bash
# Check leader election lease
kubectl get lease -n streamspace | grep streamspace-agent

# View which pod is leader
kubectl get lease streamspace-agent-k8s-prod-us-east-1 -n streamspace -o jsonpath='{.spec.holderIdentity}'

# Check agent logs
kubectl logs -n streamspace -l app.kubernetes.io/component=k8s-agent -f
```

**Failover Testing:**
```bash
# Delete leader pod to trigger failover
LEADER=$(kubectl get lease streamspace-agent-k8s-prod-us-east-1 -n streamspace -o jsonpath='{.spec.holderIdentity}')
kubectl delete pod $LEADER -n streamspace

# Watch new leader election
watch kubectl get lease -n streamspace
```

---

### 4. Agents (docker-agent)

#### How It Scales

**Multi-Host Architecture:**
- **One agent per Docker host/cluster**
- Each agent has unique `agentId`
- Example: `docker-prod-host1`, `docker-staging-host2`
- Agents connect to Control Plane via WebSocket
- Supports standalone Docker, multi-host, and Docker Swarm

**Agent Connection Flow:**
1. Agent connects to API WebSocket endpoint
2. Registers with unique `agentId`
3. API stores `agent:{agentId}:pod` → pod name in Redis (if multi-pod)
4. Heartbeats every 30 seconds
5. If disconnected, reconnects to any available API pod

**Command Routing:**
- Same as k8s-agent: Redis pub/sub routing between API pods
- Commands routed to pod where agent is connected
- Supports cross-pod command delivery

#### Configuration

**Standalone Deployment:**
```bash
docker run -d \
  --name streamspace-docker-agent \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AGENT_ID=docker-prod-host1 \
  -e CONTROL_PLANE_URL=wss://streamspace-api.example.com \
  -e PLATFORM=docker \
  -e REGION=us-east-1 \
  streamspace/docker-agent:latest
```

**Docker Compose Deployment:**
```yaml
# docker-compose.standalone.yaml
version: '3.8'
services:
  docker-agent:
    image: streamspace/docker-agent:latest
    environment:
      AGENT_ID: docker-prod-host1
      CONTROL_PLANE_URL: wss://streamspace-api.example.com
      ENABLE_HA: "false"  # Standalone mode
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

#### Agent HA (High Availability)

**Current Status:** ✅ **Implemented** (v2.0)

**Features:**
- Multi-backend leader election (file, redis, swarm)
- Active-Standby failover pattern
- Only one active agent replica per Docker host/cluster
- Automatic failover in ~15-20 seconds
- Graceful leader handoff on shutdown
- Flexible deployment: standalone Docker, multi-host, Docker Swarm

**Leader Election Backends:**

##### File Backend (Single Host)

**Use Case**: Single Docker host with multiple agent processes

**How It Works:**
- Uses `flock` (file locking) for exclusive access
- Lock file shared via Docker volume or host mount
- Only works on single host (not NFS)
- Simplest HA option without external dependencies

**Configuration:**
```yaml
# docker-compose.ha-file.yaml
version: '3.8'
services:
  docker-agent:
    image: streamspace/docker-agent:latest
    environment:
      AGENT_ID: docker-prod-host1
      CONTROL_PLANE_URL: wss://streamspace-api.example.com
      ENABLE_HA: "true"
      LEADER_ELECTION_BACKEND: "file"
      LOCK_FILE_PATH: "/var/run/streamspace/agent.lock"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - leader-locks:/var/run/streamspace

volumes:
  leader-locks:
    driver: local
```

**Deploy with 3 replicas:**
```bash
docker-compose -f docker-compose.ha-file.yaml up -d --scale docker-agent=3
```

**Verify:**
```bash
# Check lock file
docker exec streamspace-docker-agent-1 cat /var/run/streamspace/agent.lock

# View leader
docker-compose -f docker-compose.ha-file.yaml logs -f docker-agent
```

##### Redis Backend (Multi-Host)

**Use Case**: Multiple Docker hosts without orchestration

**How It Works:**
- Uses Redis `SET NX` with TTL for distributed locking
- Atomic operations via Lua scripts
- Works across multiple Docker hosts
- Requires Redis server accessible to all agents

**Configuration:**
```yaml
# docker-compose.ha-redis.yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  docker-agent:
    image: streamspace/docker-agent:latest
    depends_on:
      - redis
    environment:
      AGENT_ID: docker-prod-cluster
      CONTROL_PLANE_URL: wss://streamspace-api.example.com
      ENABLE_HA: "true"
      LEADER_ELECTION_BACKEND: "redis"
      REDIS_URL: "redis://redis:6379/0"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

**Deploy across multiple hosts:**
```bash
# Host 1
docker-compose -f docker-compose.ha-redis.yaml up -d

# Host 2 (same config, different REDIS_URL pointing to shared Redis)
docker-compose -f docker-compose.ha-redis.yaml up -d

# Host 3
docker-compose -f docker-compose.ha-redis.yaml up -d
```

**Verify:**
```bash
# Check Redis leader key
redis-cli GET streamspace:agent:leader:docker-prod-cluster
redis-cli TTL streamspace:agent:leader:docker-prod-cluster
```

##### Swarm Backend (Docker Swarm)

**Use Case**: Production Docker Swarm clusters

**How It Works:**
- Uses Docker Swarm service labels for leader election
- Atomic updates via Swarm API
- Leverages Swarm's Raft consensus
- Requires manager node access
- Most native option for Swarm deployments

**Configuration:**
```yaml
# docker-swarm.yaml
version: '3.8'
services:
  docker-agent:
    image: streamspace/docker-agent:latest
    deploy:
      mode: replicated
      replicas: 3
      placement:
        constraints:
          - node.role == manager  # Required for Swarm API access
    environment:
      AGENT_ID: docker-swarm-prod
      CONTROL_PLANE_URL: wss://streamspace-api.example.com
      ENABLE_HA: "true"
      LEADER_ELECTION_BACKEND: "swarm"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
```

**Deploy to Swarm:**
```bash
# Initialize Swarm (if not already)
docker swarm init

# Deploy stack
docker stack deploy -c docker-swarm.yaml streamspace-agent

# Scale agent
docker service scale streamspace-agent_docker-agent=5

# View service status
docker service ps streamspace-agent_docker-agent
```

**Verify:**
```bash
# Check service labels
docker service inspect streamspace-agent_docker-agent \
  --format '{{ json .Spec.Labels }}' | jq

# View leader from labels
docker service inspect streamspace-agent_docker-agent \
  --format '{{ index .Spec.Labels "streamspace.agent.leader.docker-swarm-prod" }}'
```

**Leader Election Parameters:**
- **Lease Duration**: 15 seconds (how long leader holds lease)
- **Renew Deadline**: 10 seconds (how often leader renews lease)
- **Retry Period**: 2 seconds (how often standby checks for leadership)

**Systemd Deployment (Bare Metal):**

**Installation:**
```bash
# Copy binary
sudo cp docker-agent /usr/local/bin/docker-agent

# Copy systemd unit
sudo cp docker-agent.service /etc/systemd/system/

# Create environment file
sudo mkdir -p /etc/streamspace
sudo cp docker-agent.env.example /etc/streamspace/docker-agent.env
sudo chmod 600 /etc/streamspace/docker-agent.env

# Edit configuration
sudo vi /etc/streamspace/docker-agent.env

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable docker-agent
sudo systemctl start docker-agent
```

**HA Configuration:**
```bash
# /etc/streamspace/docker-agent.env
AGENT_ID=docker-prod-host1
CONTROL_PLANE_URL=wss://streamspace-api.example.com
ENABLE_HA=true
LEADER_ELECTION_BACKEND=redis  # or file, swarm
REDIS_URL=redis://redis.example.com:6379/0
```

**Verify:**
```bash
# Check service status
sudo systemctl status docker-agent

# View logs
sudo journalctl -u docker-agent -f

# Check leader election
# For file backend:
cat /var/run/streamspace/docker-agent-*.lock

# For Redis backend:
redis-cli GET streamspace:agent:leader:docker-prod-host1
```

**Deployment Examples:**

See `agents/docker-agent/deployments/README.md` for comprehensive deployment guides including:
- Docker Compose configurations (standalone, HA with file, HA with Redis)
- Docker Swarm stack definitions
- Systemd service files
- Environment variable reference
- Troubleshooting guides

**Backend Comparison:**

| Backend | Use Case | Multi-Host | Dependencies | Complexity |
|---------|----------|------------|--------------|------------|
| **File** | Dev/Testing, Single Host | ❌ | None | Low |
| **Redis** | Production, Multi-Host | ✅ | Redis | Medium |
| **Swarm** | Production Swarm | ✅ | Swarm | Medium |

**Failover Testing:**

```bash
# File backend: Kill leader process
docker kill $(docker ps --filter name=docker-agent -q | head -1)

# Redis backend: Verify new leader
redis-cli GET streamspace:agent:leader:docker-prod-cluster

# Swarm backend: Remove leader task
docker service update --force streamspace-agent_docker-agent
```

---

### 5. PostgreSQL

#### Current Approach

**Single Instance (Default):**
- Helm chart deploys single PostgreSQL pod
- Suitable for development/testing
- **Not recommended for production**

**External PostgreSQL (Recommended):**
```yaml
# values.yaml
postgresql:
  enabled: false  # Disable internal PostgreSQL
  external:
    enabled: true
    host: "postgres.example.com"
    port: 5432
    database: "streamspace"
    username: "streamspace"
    existingSecret: "postgres-credentials"
```

#### Production Options

**Option 1: PostgreSQL High Availability (Patroni + etcd)**
```bash
# Deploy Patroni cluster (3 nodes)
helm install postgres-ha bitnami/postgresql-ha \
  --set postgresql.replicaCount=3 \
  --set postgresql.database=streamspace
```

**Option 2: Cloud-Managed PostgreSQL**
- AWS RDS PostgreSQL with Multi-AZ
- Google Cloud SQL with HA
- Azure Database for PostgreSQL
- DigitalOcean Managed Databases

**Option 3: PostgreSQL Operator (Zalando, CrunchyData)**
```yaml
apiVersion: acid.zalan.do/v1
kind: postgresql
metadata:
  name: streamspace-db
spec:
  numberOfInstances: 3
  postgresql:
    version: "15"
  volume:
    size: 100Gi
```

---

### 5. Redis

#### Current Approach

**Single Instance (Default):**
- Helm chart deploys single Redis pod
- Suitable for development/testing
- **Not recommended for production**

**External Redis (Recommended):**
```yaml
# values.yaml
redis:
  enabled: true
  external:
    enabled: true
    host: "redis.example.com"
    port: 6379
    password: ""
    existingSecret: "redis-credentials"
```

#### Production Options

**Option 1: Redis Sentinel (HA)**
```bash
# Deploy Redis with Sentinel (3 nodes)
helm install redis-ha bitnami/redis \
  --set sentinel.enabled=true \
  --set master.count=1 \
  --set replica.replicaCount=2
```

**Option 2: Redis Cluster (Sharding + HA)**
```bash
# Deploy Redis Cluster (6 nodes: 3 masters + 3 replicas)
helm install redis-cluster bitnami/redis-cluster \
  --set cluster.nodes=6 \
  --set cluster.replicas=1
```

**Option 3: Cloud-Managed Redis**
- AWS ElastiCache for Redis
- Google Cloud Memorystore
- Azure Cache for Redis
- DigitalOcean Managed Redis

---

## Configuration Guide

### Development (Single-Pod)

**values.yaml:**
```yaml
# Minimal setup for local development
api:
  replicaCount: 1  # Single API pod

ui:
  replicaCount: 1  # Single UI pod

k8sAgent:
  enabled: true
  replicaCount: 1
  config:
    agentId: "k8s-dev-local"

redis:
  enabled: false  # No Redis needed

postgresql:
  enabled: true  # Use bundled PostgreSQL
  internal:
    persistence:
      enabled: false  # No persistence needed
```

**Install:**
```bash
helm install streamspace ./chart \
  -n streamspace --create-namespace
```

---

### Staging (Multi-Pod with Internal Redis)

**values.yaml:**
```yaml
# Multi-pod setup with internal Redis
api:
  replicaCount: 2  # 2 API pods for testing HA

ui:
  replicaCount: 2  # 2 UI pods

k8sAgent:
  enabled: true
  replicaCount: 1
  config:
    agentId: "k8s-staging-cluster"

redis:
  enabled: true  # Enable Redis for multi-pod
  agentHubEnabled: true  # Enable AgentHub Redis
  internal:
    persistence:
      enabled: true  # Persist Redis data
      size: 5Gi

postgresql:
  enabled: true
  internal:
    persistence:
      enabled: true
      size: 20Gi
```

**Install:**
```bash
helm install streamspace ./chart \
  -n streamspace --create-namespace \
  -f values-staging.yaml
```

---

### Production (Multi-Pod with External Services)

**values.yaml:**
```yaml
# Production-ready multi-pod setup
api:
  replicaCount: 5  # 5 API pods for HA and load balancing
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70

  resources:
    requests:
      memory: 512Mi
      cpu: 500m
    limits:
      memory: 1Gi
      cpu: 2000m

  podDisruptionBudget:
    enabled: true
    minAvailable: 2  # Always keep 2 pods running

ui:
  replicaCount: 3
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70

  podDisruptionBudget:
    enabled: true
    minAvailable: 1

k8sAgent:
  enabled: true
  replicaCount: 1
  config:
    agentId: "k8s-prod-us-east-1"
    controlPlaneUrl: "wss://streamspace-api.example.com"
    region: "us-east-1"
    capacity:
      maxSessions: 200

redis:
  enabled: true
  agentHubEnabled: true
  external:
    enabled: true
    host: "redis-sentinel.example.com"
    port: 26379
    existingSecret: "redis-credentials"

postgresql:
  enabled: false
  external:
    enabled: true
    host: "postgres-ha.example.com"
    port: 5432
    database: "streamspace"
    username: "streamspace"
    existingSecret: "postgres-credentials"

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
  hosts:
    - host: streamspace.example.com
  tls:
    enabled: true
    - secretName: streamspace-tls
      hosts:
        - streamspace.example.com
```

**Install:**
```bash
# Create secrets first
kubectl create secret generic postgres-credentials \
  --from-literal=postgres-password='<secure-password>' \
  -n streamspace

kubectl create secret generic redis-credentials \
  --from-literal=redis-password='<secure-password>' \
  -n streamspace

# Install chart
helm install streamspace ./chart \
  -n streamspace --create-namespace \
  -f values-production.yaml
```

---

## Deployment Examples

### Example 1: Scale API Horizontally

```bash
# Start with 2 replicas
kubectl scale deployment streamspace-api --replicas=2 -n streamspace

# Wait for pods to be ready
kubectl rollout status deployment/streamspace-api -n streamspace

# Check pod distribution
kubectl get pods -n streamspace -l app.kubernetes.io/component=api -o wide

# Verify Redis connection
kubectl logs -n streamspace deployment/streamspace-api | grep "AgentHub Redis"
# Expected: "AgentHub Redis connected - multi-pod support enabled"

# Scale to 5 replicas
kubectl scale deployment streamspace-api --replicas=5 -n streamspace
```

---

### Example 2: Deploy Multi-Cluster Agents

**Cluster A (US East):**
```bash
# Deploy agent for Cluster A
helm install streamspace-agent-us-east ./chart \
  -n streamspace \
  --set k8sAgent.enabled=true \
  --set k8sAgent.config.agentId="k8s-prod-us-east-1" \
  --set k8sAgent.config.controlPlaneUrl="wss://api.streamspace.com" \
  --set k8sAgent.config.region="us-east-1" \
  --set api.enabled=false \
  --set ui.enabled=false \
  --set postgresql.enabled=false
```

**Cluster B (EU West):**
```bash
# Deploy agent for Cluster B
helm install streamspace-agent-eu-west ./chart \
  -n streamspace \
  --set k8sAgent.enabled=true \
  --set k8sAgent.config.agentId="k8s-prod-eu-west-1" \
  --set k8sAgent.config.controlPlaneUrl="wss://api.streamspace.com" \
  --set k8sAgent.config.region="eu-west-1" \
  --set api.enabled=false \
  --set ui.enabled=false \
  --set postgresql.enabled=false
```

**Verify Agents:**
```bash
# Check Control Plane API logs
kubectl logs -n streamspace deployment/streamspace-api | grep "Agent registered"
# Expected:
# [AgentHub] Agent registered: k8s-prod-us-east-1 (platform: kubernetes, region: us-east-1)
# [AgentHub] Agent registered: k8s-prod-eu-west-1 (platform: kubernetes, region: eu-west-1)
```

---

### Example 3: Enable Autoscaling

**API Autoscaling:**
```yaml
# values.yaml
api:
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80
```

**Or via kubectl:**
```bash
kubectl autoscale deployment streamspace-api \
  --min=3 --max=10 --cpu-percent=70 -n streamspace

# Check HPA status
kubectl get hpa -n streamspace

# Describe HPA
kubectl describe hpa streamspace-api -n streamspace
```

**UI Autoscaling:**
```bash
kubectl autoscale deployment streamspace-ui \
  --min=2 --max=10 --cpu-percent=70 -n streamspace
```

---

## Performance Tuning

### API Server Optimization

**1. Resource Requests/Limits:**
```yaml
api:
  resources:
    requests:
      memory: 512Mi  # Baseline for normal load
      cpu: 500m
    limits:
      memory: 1Gi  # Allow burst to 1GB
      cpu: 2000m   # Allow burst to 2 cores
```

**2. Connection Pool Tuning:**
```yaml
# PostgreSQL connection pool (set via env vars)
DB_MAX_OPEN_CONNS: "25"  # Max connections per API pod
DB_MAX_IDLE_CONNS: "5"   # Idle connections to keep
DB_CONN_MAX_LIFETIME: "5m"  # Recycle connections

# Redis connection pool
REDIS_POOL_SIZE: "10"  # Connections per pod
```

**3. Pod Disruption Budget:**
```yaml
api:
  podDisruptionBudget:
    enabled: true
    minAvailable: 2  # Keep at least 2 pods during updates
```

---

### Redis Optimization

**1. Memory Limits:**
```yaml
redis:
  internal:
    resources:
      limits:
        memory: 512Mi
    config:
      maxMemory: "450mb"  # Leave headroom for overhead
      maxMemoryPolicy: "allkeys-lru"  # Evict LRU keys when full
```

**2. Persistence (if needed):**
```yaml
redis:
  internal:
    config:
      # Option 1: AOF (more durable, slower)
      appendOnly: "yes"
      appendFsync: "everysec"

      # Option 2: RDB (faster, less durable)
      save: "900 1 300 10 60 10000"  # Snapshot rules
```

**3. Connection Tuning:**
```yaml
redis:
  internal:
    config:
      maxClients: "1000"  # Max concurrent connections
      timeout: "300"  # Close idle connections after 5 min
```

---

### PostgreSQL Optimization

**1. Connection Pooling (PgBouncer):**
```yaml
# Deploy PgBouncer between API and PostgreSQL
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pgbouncer
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: pgbouncer
        image: pgbouncer/pgbouncer:1.21
        env:
          - name: DATABASES_HOST
            value: "postgres.example.com"
          - name: DATABASES_PORT
            value: "5432"
          - name: DATABASES_DATABASE
            value: "streamspace"
          - name: POOL_MODE
            value: "transaction"
          - name: MAX_CLIENT_CONN
            value: "1000"
          - name: DEFAULT_POOL_SIZE
            value: "25"
```

**2. PostgreSQL Parameters:**
```sql
-- Increase connection limit
ALTER SYSTEM SET max_connections = 200;

-- Tune shared buffers (25% of RAM)
ALTER SYSTEM SET shared_buffers = '4GB';

-- Tune work memory
ALTER SYSTEM SET work_mem = '64MB';

-- Enable query caching
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
```

---

## Monitoring & Troubleshooting

### Health Checks

**API Pods:**
```bash
# Check API pod health
kubectl get pods -n streamspace -l app.kubernetes.io/component=api

# Check logs for Redis connection
kubectl logs -n streamspace deployment/streamspace-api | grep "Redis"

# Expected:
# "Redis cache enabled and connected"
# "AgentHub Redis connected - multi-pod support enabled"
```

**Agent Connections:**
```bash
# Check registered agents
kubectl logs -n streamspace deployment/streamspace-api | grep "Agent registered"

# Check heartbeats
kubectl logs -n streamspace deployment/streamspace-api | grep "heartbeat"
```

---

### Common Issues

#### Issue 1: "No agents available"

**Symptoms:**
- Session creation fails with "No agents available" error
- Agent is connected but not visible to API

**Diagnosis:**
```bash
# Check if Redis is enabled
kubectl get pods -n streamspace | grep redis

# Check API logs
kubectl logs -n streamspace deployment/streamspace-api | grep "AgentHub Redis"

# Check agent logs
kubectl logs -n streamspace deployment/streamspace-k8s-agent
```

**Solutions:**
1. **Enable Redis**:
   ```yaml
   redis:
     enabled: true
     agentHubEnabled: true
   ```

2. **Verify POD_NAME is set**:
   ```bash
   kubectl exec -n streamspace deployment/streamspace-api -- env | grep POD_NAME
   ```

3. **Check Redis keys**:
   ```bash
   kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli KEYS "agent:*"
   ```

---

#### Issue 2: Commands not reaching agents

**Symptoms:**
- Commands timeout or fail to execute
- Agent connected but not receiving commands

**Diagnosis:**
```bash
# Check Redis pub/sub channels
kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli PUBSUB CHANNELS

# Check API pod names
kubectl get pods -n streamspace -l app.kubernetes.io/component=api -o name

# Verify channel format: pod:<pod-name>:commands
```

**Solutions:**
1. **Verify Redis pub/sub is working**:
   ```bash
   # Terminal 1: Subscribe
   kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli SUBSCRIBE "pod:streamspace-api-abc123:commands"

   # Terminal 2: Publish test message
   kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli PUBLISH "pod:streamspace-api-abc123:commands" "test"
   ```

2. **Check API logs for routing**:
   ```bash
   kubectl logs -n streamspace deployment/streamspace-api | grep "Published command"
   ```

---

#### Issue 3: Stale agent entries in Redis

**Symptoms:**
- Old agents still show as connected
- Commands fail with "agent not found"

**Diagnosis:**
```bash
# Check agent TTL in Redis
kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli TTL "agent:k8s-prod-cluster:connected"

# Check all agent keys
kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli KEYS "agent:*"
```

**Solutions:**
1. **Verify heartbeats are working**:
   ```bash
   # Agent should send heartbeats every 30s
   kubectl logs -n streamspace deployment/streamspace-k8s-agent | grep "heartbeat"
   ```

2. **Manually clean stale entries**:
   ```bash
   kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli DEL "agent:k8s-prod-cluster:connected"
   kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli DEL "agent:k8s-prod-cluster:pod"
   ```

---

### Metrics & Alerts

**Prometheus Metrics:**

```yaml
# ServiceMonitor for API pods
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: streamspace-api
  namespace: streamspace
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: api
  endpoints:
  - port: http
    path: /api/v1/metrics
    interval: 30s
```

**Key Metrics:**
- `streamspace_agents_connected` - Number of connected agents
- `streamspace_sessions_active` - Active sessions per agent
- `streamspace_commands_dispatched_total` - Commands sent to agents
- `streamspace_commands_failed_total` - Failed command dispatches

**Alert Rules:**
```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: streamspace-alerts
spec:
  groups:
  - name: streamspace
    interval: 30s
    rules:
    - alert: NoAgentsConnected
      expr: streamspace_agents_connected == 0
      for: 5m
      annotations:
        summary: "No agents connected to StreamSpace"

    - alert: HighCommandFailureRate
      expr: rate(streamspace_commands_failed_total[5m]) > 0.1
      for: 10m
      annotations:
        summary: "High command failure rate (>10%)"
```

---

## Best Practices

### 1. Always Use Redis in Production

**Why:**
- Enables multi-pod API deployments
- Provides high availability
- Enables load balancing

**Configuration:**
```yaml
redis:
  enabled: true
  agentHubEnabled: true
  external:
    enabled: true  # Use external Redis in production
    host: "redis-sentinel.example.com"
```

---

### 2. Enable Pod Disruption Budgets

**Why:**
- Prevents all pods from being terminated during updates
- Ensures minimum availability during rolling updates

**Configuration:**
```yaml
api:
  podDisruptionBudget:
    enabled: true
    minAvailable: 2  # Keep at least 2 pods

ui:
  podDisruptionBudget:
    enabled: true
    minAvailable: 1
```

---

### 3. Use Autoscaling for Variable Load

**Why:**
- Automatically scales based on CPU/memory
- Reduces costs during low usage
- Handles traffic spikes

**Configuration:**
```yaml
api:
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
```

---

### 4. Separate API Replicas Across Nodes

**Why:**
- Prevents all replicas from being on same node
- Improves availability during node failures

**Configuration:**
```yaml
api:
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/component: api
          topologyKey: kubernetes.io/hostname
```

---

### 5. Monitor Redis Health

**Why:**
- Redis is critical for multi-pod operations
- Early detection of Redis issues prevents outages

**Monitoring:**
```bash
# Check Redis memory usage
kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli INFO memory

# Check connection count
kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli INFO clients

# Check keyspace
kubectl exec -n streamspace deployment/streamspace-redis -- redis-cli INFO keyspace
```

---

### 6. Test Failover Scenarios

**Why:**
- Validates high availability configuration
- Identifies issues before production incidents

**Test Cases:**

**Test 1: API Pod Failure**
```bash
# Delete random API pod
kubectl delete pod -n streamspace -l app.kubernetes.io/component=api --field-selector=status.phase=Running --dry-run=client

# Verify agent reconnects to different pod
kubectl logs -n streamspace deployment/streamspace-k8s-agent | grep "reconnect"

# Verify sessions still accessible
curl https://streamspace.example.com/api/v1/sessions
```

**Test 2: Redis Failure**
```bash
# Delete Redis pod
kubectl delete pod -n streamspace -l app.kubernetes.io/component=redis

# Verify graceful degradation (if no Redis, API should log warning and continue in single-pod mode)
kubectl logs -n streamspace deployment/streamspace-api | grep "Redis"
```

**Test 3: Load Balancer Failure**
```bash
# Simulate load balancer distributing traffic
for i in {1..10}; do
  curl -s https://streamspace.example.com/health | jq .pod_name
done

# Should see different pod names (indicates load balancing)
```

---

## Summary

StreamSpace v2.0-beta provides **production-ready horizontal scalability** for:

✅ **API Servers** - Unlimited replicas with Redis-backed AgentHub
✅ **UI Servers** - Unlimited replicas (stateless)
✅ **Agents** - One per cluster, unlimited clusters
⚠️ **PostgreSQL** - Use external HA solution
⚠️ **Redis** - Use external HA solution (Sentinel/Cluster)

**Quick Start for Production:**

1. **Enable Redis**:
   ```yaml
   redis:
     enabled: true
     agentHubEnabled: true
   ```

2. **Scale API**:
   ```bash
   kubectl scale deployment streamspace-api --replicas=5 -n streamspace
   ```

3. **Scale UI**:
   ```bash
   kubectl scale deployment streamspace-ui --replicas=3 -n streamspace
   ```

4. **Deploy Agents** (one per cluster):
   ```yaml
   k8sAgent:
     config:
       agentId: "k8s-prod-<region>"
   ```

5. **Monitor**:
   ```bash
   kubectl get pods -n streamspace
   kubectl top pods -n streamspace
   ```

For questions or issues, see:
- [GitHub Issues](https://github.com/streamspace-dev/streamspace/issues)
- [Documentation](https://docs.streamspace.dev)
- [Slack Community](https://streamspace.slack.com)
