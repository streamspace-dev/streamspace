# StreamSpace Node Manager Plugin

Advanced Kubernetes node management plugin for StreamSpace, providing comprehensive cluster infrastructure control.

## Features

- **Node Listing**: View all nodes in the cluster with detailed information
- **Cluster Statistics**: Get overall cluster resource utilization and health
- **Label Management**: Add and remove labels from nodes
- **Taint Management**: Configure node taints for pod scheduling control
- **Node Scheduling**: Cordon/uncordon nodes to control workload placement
- **Node Draining**: Safely drain pods from nodes for maintenance
- **Resource Metrics**: Real-time CPU and memory usage (requires metrics-server)
- **Health Monitoring**: Automated health checks with alerting
- **Auto-Scaling Support**: Configure thresholds for cluster autoscaling

## Requirements

- StreamSpace >= 1.0.0
- Kubernetes >= 1.19.0
- Kubernetes metrics-server (optional, for resource metrics)
- Cluster autoscaler (optional, for auto-scaling)

## Installation

1. **Via Plugin Marketplace** (Recommended):
   ```bash
   # Navigate to Admin > Plugins > Marketplace
   # Search for "Node Manager"
   # Click "Install"
   ```

2. **Manual Installation**:
   ```bash
   # Copy plugin to plugins directory
   cp -r streamspace-node-manager /path/to/streamspace/plugins/

   # Restart StreamSpace API
   kubectl rollout restart deployment/streamspace-api -n streamspace
   ```

## Configuration

### Basic Configuration

```json
{
  "nodeSelectionStrategy": "least-sessions",
  "healthCheckInterval": 60,
  "metricsEnabled": true,
  "alertOnNodeFailure": true
}
```

### Auto-Scaling Configuration

```json
{
  "enableAutoScaling": true,
  "minNodes": 1,
  "maxNodes": 10,
  "scaleUpThreshold": 80,
  "scaleDownThreshold": 20
}
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enableAutoScaling` | boolean | false | Enable automatic node scaling |
| `scaleUpThreshold` | number | 80 | CPU/Memory % to trigger scale up |
| `scaleDownThreshold` | number | 20 | CPU/Memory % to trigger scale down |
| `nodeSelectionStrategy` | string | "least-sessions" | Node selection algorithm |
| `healthCheckInterval` | number | 60 | Seconds between health checks |
| `metricsEnabled` | boolean | true | Enable resource metrics collection |
| `alertOnNodeFailure` | boolean | true | Alert when nodes become NotReady |
| `minNodes` | number | 1 | Minimum cluster nodes |
| `maxNodes` | number | 10 | Maximum cluster nodes |

### Node Selection Strategies

- **least-sessions**: Place workloads on nodes with fewest sessions
- **most-resources**: Place workloads on nodes with most available resources
- **random**: Random node selection
- **round-robin**: Distribute workloads evenly across nodes

## API Endpoints

All endpoints require `admin` permissions and are prefixed with `/api/plugins/streamspace-node-manager`.

### List Nodes
```http
GET /nodes
```

**Response**:
```json
[
  {
    "name": "node-1",
    "labels": {"role": "worker"},
    "taints": [],
    "status": {"ready": true, "phase": "Ready"},
    "capacity": {"cpu": "4", "memory": "16Gi"},
    "allocatable": {"cpu": "3.8", "memory": "15Gi"},
    "usage": {"cpu_percent": 45.2, "memory_percent": 62.1},
    "pods": 12,
    "age": "5d3h"
  }
]
```

### Get Node Details
```http
GET /nodes/:name
```

### Get Cluster Statistics
```http
GET /nodes/stats
```

**Response**:
```json
{
  "total_nodes": 3,
  "ready_nodes": 3,
  "not_ready_nodes": 0,
  "total_capacity": {"cpu": "12", "memory": "48Gi"},
  "total_allocatable": {"cpu": "11.4", "memory": "45Gi"}
}
```

### Add Label to Node
```http
PUT /nodes/:name/labels
Content-Type: application/json

{
  "key": "environment",
  "value": "production"
}
```

### Remove Label from Node
```http
DELETE /nodes/:name/labels/:key
```

### Add Taint to Node
```http
POST /nodes/:name/taints
Content-Type: application/json

{
  "key": "dedicated",
  "value": "gpu-workloads",
  "effect": "NoSchedule"
}
```

**Taint Effects**:
- `NoSchedule`: Don't schedule new pods
- `PreferNoSchedule`: Avoid scheduling new pods
- `NoExecute`: Evict existing pods

### Remove Taint from Node
```http
DELETE /nodes/:name/taints/:key
```

### Cordon Node (Mark Unschedulable)
```http
POST /nodes/:name/cordon
```

### Uncordon Node (Mark Schedulable)
```http
POST /nodes/:name/uncordon
```

### Drain Node
```http
POST /nodes/:name/drain
Content-Type: application/json

{
  "grace_period_seconds": 30
}
```

**Response**:
```json
{
  "message": "Node drained successfully",
  "pods_deleted": 8
}
```

## Permissions

This plugin requires the following Kubernetes RBAC permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: streamspace-node-manager
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "update", "patch"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "delete"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["nodes"]
  verbs: ["get", "list"]
```

## Admin UI

The plugin adds the following to the admin panel:

### Pages
- **Node Management** (`/admin/nodes`): Full node management interface

### Dashboard Widgets
- **Cluster Health**: Overview of node status
- **Node Resources**: Resource utilization graphs

## Use Cases

### 1. Cluster Maintenance
```bash
# Cordon node for maintenance
POST /api/plugins/streamspace-node-manager/nodes/worker-1/cordon

# Drain all pods
POST /api/plugins/streamspace-node-manager/nodes/worker-1/drain

# Perform maintenance...

# Uncordon node
POST /api/plugins/streamspace-node-manager/nodes/worker-1/uncordon
```

### 2. Dedicated Node Pools
```bash
# Taint GPU nodes
POST /api/plugins/streamspace-node-manager/nodes/gpu-node-1/taints
{
  "key": "nvidia.com/gpu",
  "value": "true",
  "effect": "NoSchedule"
}

# Label GPU nodes
PUT /api/plugins/streamspace-node-manager/nodes/gpu-node-1/labels
{
  "key": "accelerator",
  "value": "nvidia-tesla-t4"
}
```

### 3. Environment Segregation
```bash
# Label production nodes
PUT /api/plugins/streamspace-node-manager/nodes/prod-1/labels
{
  "key": "environment",
  "value": "production"
}

# Taint to prevent non-production workloads
POST /api/plugins/streamspace-node-manager/nodes/prod-1/taints
{
  "key": "environment",
  "value": "production",
  "effect": "NoSchedule"
}
```

## Troubleshooting

### Metrics Not Available
**Problem**: Node usage metrics not showing

**Solution**: Install Kubernetes metrics-server
```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

### Permission Denied Errors
**Problem**: Plugin cannot access Kubernetes API

**Solution**: Ensure StreamSpace has proper RBAC:
```bash
kubectl apply -f streamspace-node-manager-rbac.yaml
```

### Auto-Scaling Not Working
**Problem**: Nodes not scaling automatically

**Solution**:
1. Ensure cluster autoscaler is installed
2. Verify `enableAutoScaling` is true in config
3. Check logs: `kubectl logs -l app=streamspace-api -f | grep node-manager`

## Best Practices

1. **Always drain nodes before maintenance** to avoid disrupting sessions
2. **Use taints for specialized workloads** (GPU, high-memory, etc.)
3. **Monitor cluster health regularly** via the dashboard widgets
4. **Set appropriate min/max nodes** based on workload patterns
5. **Use labels for organization** (environment, region, instance-type)

## Uninstallation

```bash
# Via UI: Admin > Plugins > Installed > Node Manager > Uninstall

# Or via API:
DELETE /api/plugins/streamspace-node-manager
```

**Note**: Uninstalling this plugin will not affect your nodes or their configurations. All node labels and taints will remain.

## Support

- **Issues**: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- **Documentation**: https://docs.streamspace.io/plugins/node-manager
- **Community**: https://discord.gg/streamspace

## License

MIT License - See LICENSE file for details

## Author

StreamSpace Team

## Changelog

### 1.0.0 (2025-11-16)
- Initial release
- Node listing and details
- Label and taint management
- Cordon/uncordon/drain operations
- Resource metrics support
- Health monitoring
- Auto-scaling support
