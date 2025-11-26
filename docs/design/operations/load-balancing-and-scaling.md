# Load Balancing and Scaling Strategy

**Version**: v1.0
**Last Updated**: 2025-11-26
**Owner**: Architect + SRE
**Status**: Living Document
**Target Release**: v2.2

---

## Introduction

This document defines the load balancing and horizontal scaling strategy for StreamSpace Control Plane and agents. It covers API pod scaling, database scaling, VNC proxy load balancing, and capacity planning for production deployments.

**Goals**:
- Support 1,000+ concurrent sessions
- < 200ms API response time (p95)
- 99.9% uptime (3 nines)
- Linear horizontal scaling

---

## Architecture Overview

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │  (AWS ALB / GCP)│
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
   ┌────▼────┐         ┌────▼────┐         ┌────▼────┐
   │ API Pod │         │ API Pod │         │ API Pod │
   │    1    │         │    2    │         │    3    │
   └────┬────┘         └────┬────┘         └────┬────┘
        │                   │                    │
        └───────────────────┼────────────────────┘
                            │
          ┌─────────────────┴─────────────────┐
          │                                   │
     ┌────▼────┐                        ┌────▼────┐
     │PostgreSQL│                        │  Redis  │
     │ Primary  │──────────────────────> │ Cluster │
     └────┬────┘    Replication          └─────────┘
          │
     ┌────▼────┐
     │PostgreSQL│
     │ Standby  │
     └─────────┘
```

---

## 1. API Server Load Balancing

### 1.1 Load Balancer Configuration

**Technology**: AWS Application Load Balancer (ALB) or GCP Load Balancer (L7)

**Configuration**:
```yaml
# Example: AWS ALB Ingress
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: streamspace-api
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443}]'
    alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:...
    alb.ingress.kubernetes.io/healthcheck-path: /health
    alb.ingress.kubernetes.io/healthcheck-interval-seconds: '15'
    alb.ingress.kubernetes.io/healthcheck-timeout-seconds: '5'
    alb.ingress.kubernetes.io/healthy-threshold-count: '2'
    alb.ingress.kubernetes.io/unhealthy-threshold-count: '2'
spec:
  rules:
  - host: api.streamspace.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: streamspace-api
            port:
              number: 8000
```

**Health Checks**:
- **Endpoint**: `GET /health`
- **Interval**: 15 seconds
- **Timeout**: 5 seconds
- **Healthy Threshold**: 2 consecutive successes
- **Unhealthy Threshold**: 2 consecutive failures

**Response**:
```json
{
  "status": "healthy",
  "version": "v2.0-beta.1",
  "database": "connected",
  "redis": "connected",
  "agents": 3
}
```

### 1.2 Session Affinity (Sticky Sessions)

**Requirement**: VNC proxy connections require session affinity (same user → same API pod)

**Why**: VNC WebSocket tunnels are stateful (agent → API pod → user)

**Configuration**:
```yaml
# ALB sticky sessions
alb.ingress.kubernetes.io/target-group-attributes: |
  stickiness.enabled=true,
  stickiness.lb_cookie.duration_seconds=3600,
  stickiness.type=lb_cookie
```

**Cookie**: `AWSALB` (automatically managed by ALB)

**Duration**: 1 hour (VNC token expiry)

**Behavior**:
- First request: Load balancer assigns pod, sets cookie
- Subsequent requests (with cookie): Routed to same pod
- Pod failure: Cookie invalidated, new pod assigned

### 1.3 Connection Draining

**Purpose**: Graceful shutdown (don't drop active connections)

**Configuration**:
```yaml
# Kubernetes PreStop hook
lifecycle:
  preStop:
    exec:
      command: ["/bin/sh", "-c", "sleep 30"]
```

**Drain Duration**: 30 seconds

**Process**:
1. Pod receives SIGTERM (shutdown signal)
2. PreStop hook delays shutdown for 30 seconds
3. Load balancer stops sending new traffic
4. Existing connections complete (VNC streams, API requests)
5. After 30 seconds, pod terminates

---

## 2. Horizontal Pod Autoscaling (HPA)

### 2.1 HPA Configuration

**Metrics**: CPU, Memory, Custom (QPS, VNC connections)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: streamspace-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: streamspace-api
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: http_requests_per_second
      target:
        type: AverageValue
        averageValue: "100"
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Pods
        value: 1
        periodSeconds: 60
```

### 2.2 Scaling Triggers

| Metric | Threshold | Action | Cooldown |
|--------|-----------|--------|----------|
| **CPU** | > 70% | Scale up by 50% | 60s |
| **Memory** | > 80% | Scale up by 50% | 60s |
| **QPS** | > 100 req/s/pod | Scale up by 50% | 60s |
| **CPU** | < 40% | Scale down by 1 pod | 300s (5 min) |
| **Memory** | < 50% | Scale down by 1 pod | 300s |

### 2.3 Scaling Limits

**Production**:
- **Min Replicas**: 3 (HA, no single point of failure)
- **Max Replicas**: 20 (capacity limit, adjust based on cluster)

**Development/Staging**:
- **Min Replicas**: 1
- **Max Replicas**: 5

### 2.4 Pod Resource Requests/Limits

```yaml
resources:
  requests:
    cpu: "500m"      # 0.5 CPU cores
    memory: "1Gi"    # 1 GB RAM
  limits:
    cpu: "2000m"     # 2 CPU cores
    memory: "4Gi"    # 4 GB RAM
```

**Rationale**:
- **Requests**: Guaranteed resources (scheduler uses for placement)
- **Limits**: Maximum burst capacity
- **Ratio**: 1:4 (allows burst without over-provisioning)

---

## 3. Database Scaling

### 3.1 PostgreSQL Architecture

**Primary-Standby Replication**:
```
┌─────────────────┐
│ PostgreSQL      │
│ Primary (RW)    │ ← All writes
└────────┬────────┘
         │ Streaming replication (async)
         ↓
┌─────────────────┐
│ PostgreSQL      │
│ Standby (RO)    │ ← Read replicas (optional)
└─────────────────┘
```

**Write Path**: All writes → Primary
**Read Path**: Reads → Primary (default) or Standby (if configured)

### 3.2 Connection Pooling

**Technology**: PgBouncer (connection pooler)

**Configuration**:
```ini
[databases]
streamspace = host=postgres-primary port=5432 dbname=streamspace

[pgbouncer]
listen_port = 6432
listen_addr = *
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 25
reserve_pool_size = 5
reserve_pool_timeout = 3
```

**Pool Sizing**:
- **max_client_conn**: 1,000 (total client connections)
- **default_pool_size**: 25 per database (actual PostgreSQL connections)
- **Reserve**: 5 extra connections for bursts

**Why**: PostgreSQL has overhead per connection (~10MB); pooling allows 1,000 clients with only 25 DB connections

### 3.3 Read Replicas (Optional)

**Use Case**: Read-heavy workloads (e.g., listing sessions, analytics)

**Configuration**:
```go
// Separate connection for read replicas
readDB, err := pgxpool.Connect(ctx, "postgres://replica:5432/streamspace")
writeDB, err := pgxpool.Connect(ctx, "postgres://primary:5432/streamspace")

// Route reads to replica
sessions, err := readDB.Query(ctx, "SELECT * FROM sessions WHERE org_id = $1", orgID)

// Route writes to primary
_, err = writeDB.Exec(ctx, "INSERT INTO sessions (...) VALUES (...)")
```

**Replication Lag**: Monitor lag (typically < 1 second)
```sql
SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn(),
       pg_last_wal_receive_lsn() - pg_last_wal_replay_lsn() AS lag;
```

### 3.4 Database Vertical Scaling

**When to Scale Up**:
- CPU > 80% sustained
- Disk IOPS saturated
- Connection pool exhausted

**Sizing Guidelines**:

| Sessions | vCPUs | RAM | Storage | IOPS |
|----------|-------|-----|---------|------|
| 100 | 2 | 8 GB | 100 GB | 3,000 |
| 500 | 4 | 16 GB | 200 GB | 6,000 |
| 1,000 | 8 | 32 GB | 500 GB | 12,000 |
| 5,000 | 16 | 64 GB | 1 TB | 20,000 |

---

## 4. Redis Scaling

### 4.1 Redis Cluster Mode

**Use Cases**:
- Session cache (reduce DB load)
- Agent routing (multi-pod API, see ADR-005)
- Rate limiting counters

**Configuration**: Redis Cluster (3 masters + 3 replicas)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: redis-cluster-config
data:
  redis.conf: |
    cluster-enabled yes
    cluster-config-file nodes.conf
    cluster-node-timeout 5000
    appendonly yes
    maxmemory 2gb
    maxmemory-policy allkeys-lru
```

**Sharding**: Automatic (Redis Cluster hash slots: 0-16383)

**Replication**: Each master has 1 replica (HA)

### 4.2 Redis Failover

**Automatic Failover**:
- Master fails → Replica promoted to master (< 5 seconds)
- Clients reconnect automatically (retry logic)

**Monitoring**:
```bash
# Check cluster health
redis-cli --cluster check localhost:6379

# Monitor replication lag
redis-cli info replication
```

### 4.3 Redis Cache Eviction

**Policy**: `allkeys-lru` (Least Recently Used)

**Why**: Cache should fail open (evict old entries, not reject writes)

**Monitoring**:
```bash
redis-cli info stats | grep evicted_keys
```

**Alert**: If `evicted_keys` > 10% of `keyspace`, increase memory or add shards

---

## 5. VNC Proxy Load Balancing

### 5.1 Challenge

VNC WebSocket tunnels are **stateful**:
- User ↔ API Pod ↔ Agent ↔ Session
- Connection must stay on same API pod for duration

### 5.2 Solution: Sticky Sessions

**Mechanism**: Load balancer cookie (AWSALB)

**Flow**:
1. User requests VNC token: `GET /api/v1/sessions/:id/vnc`
2. Any API pod responds with token
3. User connects VNC WebSocket: `wss://api/ws/vnc?token=...`
4. Load balancer sets sticky cookie, routes to Pod A
5. All subsequent VNC frames → Pod A (via cookie)
6. Cookie expires after 1 hour (VNC token expiry)

### 5.3 VNC Connection Limits

**Per Pod**:
- **Max Concurrent VNC Connections**: 100 (conservative)
- **Bandwidth**: ~10-50 KB/s per VNC stream
- **Total Bandwidth/Pod**: ~1-5 MB/s (100 streams)

**Scaling**:
- 3 API pods = 300 concurrent VNC connections
- 10 API pods = 1,000 concurrent VNC connections

**Monitoring**:
```prometheus
# Prometheus metric
streamspace_vnc_connections_active{pod="api-1"}
```

**Alert**: If `vnc_connections_active > 80` per pod → Scale up

---

## 6. Agent Load Balancing

### 6.1 Agent Selection Strategy

**Current** (v2.0-beta): Round-robin (simple)

**Future** (v2.1): Weighted least-connections

**Algorithm** (Weighted Least-Connections):
```go
func selectAgent(agents []Agent) Agent {
    bestAgent := agents[0]
    bestScore := float64(bestAgent.ActiveSessions) / float64(bestAgent.Capacity)

    for _, agent := range agents[1:] {
        score := float64(agent.ActiveSessions) / float64(agent.Capacity)
        if score < bestScore {
            bestAgent = agent
            bestScore = score
        }
    }

    return bestAgent
}
```

**Metrics**:
- **ActiveSessions**: Current sessions on agent
- **Capacity**: Max sessions (configured per agent)
- **Score**: Utilization percentage (lower is better)

### 6.2 Agent Capacity Planning

**Per Agent** (Kubernetes):
- **Node Size**: 8 vCPUs, 32 GB RAM
- **Max Sessions**: 20 (assuming 0.4 vCPU, 1.6 GB RAM per session)
- **Headroom**: 20% (for system overhead)

**Scaling**:
- 1 agent = 20 sessions
- 10 agents = 200 sessions
- 50 agents = 1,000 sessions

---

## 7. Capacity Planning

### 7.1 Capacity Targets

| Metric | v2.0 (Target) | v2.1 (Goal) | v3.0 (Vision) |
|--------|---------------|-------------|---------------|
| **Concurrent Sessions** | 100 | 1,000 | 10,000 |
| **API Pods** | 3 | 10 | 50 |
| **Agents** | 2 | 10 | 100 |
| **Database** | 2 vCPU, 8 GB | 8 vCPU, 32 GB | 32 vCPU, 128 GB |
| **Redis** | 3 nodes (1 GB) | 6 nodes (2 GB) | 12 nodes (4 GB) |

### 7.2 Resource Estimation

**Per Session**:
- **CPU**: 0.4 vCPU (avg), 1 vCPU (burst)
- **Memory**: 1.6 GB (avg), 4 GB (limit)
- **Storage**: 10 GB (home directory)
- **VNC Bandwidth**: 10-50 KB/s

**1,000 Sessions**:
- **Total CPU**: 400 vCPU (avg), 1,000 vCPU (burst)
- **Total Memory**: 1.6 TB (avg), 4 TB (limit)
- **Total Storage**: 10 TB (persistent volumes)
- **VNC Bandwidth**: 10-50 MB/s

### 7.3 Kubernetes Cluster Sizing

**Node Type**: m5.2xlarge (8 vCPU, 32 GB RAM) or equivalent

**Nodes Required** (1,000 sessions):
- **Compute Nodes**: 50 (20 sessions/node)
- **Control Nodes**: 3 (HA)
- **Total Nodes**: 53

**Cluster Autoscaling**:
```yaml
apiVersion: autoscaling.k8s.io/v1
kind: ClusterAutoscaler
spec:
  minNodes: 5
  maxNodes: 100
  scaleDownUnneededTime: 10m
  scaleDownDelayAfterAdd: 10m
```

---

## 8. Performance Benchmarks

### 8.1 API Performance

**Target** (p95):
- **GET /sessions**: < 100ms
- **POST /sessions**: < 200ms (includes command dispatch)
- **GET /sessions/:id/vnc**: < 50ms (token generation)

**Actual** (v2.0-beta, 3 API pods):
- **GET /sessions**: 45ms (p95) ✅
- **POST /sessions**: 180ms (p95) ✅
- **GET /sessions/:id/vnc**: 30ms (p95) ✅

### 8.2 Database Performance

**Queries/Second** (PostgreSQL):
- **Reads**: 5,000 QPS (with connection pooling)
- **Writes**: 1,000 QPS

**Connection Pool Saturation**: Monitor `pool_wait_time`
```sql
SELECT usename, count(*) FROM pg_stat_activity GROUP BY usename;
```

### 8.3 VNC Latency

**Target**: < 50ms (p95) total latency (User → Session)

**Breakdown**:
- User → Load Balancer: 5ms
- Load Balancer → API Pod: 5ms
- API Pod → Agent: 10ms
- Agent → Session Pod: 10ms
- **Total**: 30ms (p95) ✅

---

## 9. Monitoring and Alerting

### 9.1 Key Metrics

**API Server**:
- `streamspace_api_requests_total` (counter)
- `streamspace_api_request_duration_seconds` (histogram)
- `streamspace_vnc_connections_active` (gauge)
- `streamspace_api_pods_available` (gauge)

**Database**:
- `pg_stat_database_tup_fetched` (rows read)
- `pg_stat_database_tup_inserted` (rows written)
- `pg_stat_activity_count` (active connections)
- `pg_replication_lag_seconds` (replica lag)

**Redis**:
- `redis_connected_clients` (gauge)
- `redis_keyspace_hits_total` (counter)
- `redis_keyspace_misses_total` (counter)
- `redis_evicted_keys_total` (counter)

**Agents**:
- `streamspace_agent_sessions_active` (gauge)
- `streamspace_agent_heartbeat_last_seen` (timestamp)

### 9.2 Alerts

**Critical** (PagerDuty):
- API pods < 2 (HA broken)
- Database primary down
- Redis cluster quorum lost
- API error rate > 5%

**Warning** (Slack):
- API CPU > 80% (scale up)
- Database connections > 80% of pool
- Redis evictions > 10% of keyspace
- VNC connections > 80 per pod

---

## 10. Scaling Runbook

### 10.1 Scale Up API

**Trigger**: CPU > 70% or QPS > 100 per pod

**Manual**:
```bash
kubectl scale deployment streamspace-api --replicas=5
```

**Automatic**: HPA handles (see section 2)

### 10.2 Scale Up Database

**Trigger**: CPU > 80%, IOPS saturated

**Process**:
1. **Snapshot** database (backup before resize)
2. **Resize** instance (AWS RDS: Modify → Instance Type)
3. **Reboot** (required for instance type change)
4. **Monitor** replication lag (standby catches up)

**Downtime**: 5-10 minutes (during reboot)

### 10.3 Scale Up Agents

**Trigger**: Agent utilization > 80%

**Manual**:
```bash
kubectl scale deployment streamspace-k8s-agent --replicas=10
```

**Validation**:
```bash
kubectl get pods -l app=streamspace-k8s-agent
# Verify all agents register with Control Plane
```

---

## References

- **HPA Docs**: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
- **AWS ALB**: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/
- **PostgreSQL Pooling**: https://www.pgbouncer.org/
- **Redis Cluster**: https://redis.io/topics/cluster-tutorial

---

**Version History**:
- **v1.0** (2025-11-26): Initial load balancing and scaling strategy
- **Next Review**: v2.2 release (Q2 2026)
