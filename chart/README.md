# Workspace Platform Helm Chart

Helm chart for deploying the Workspace Streaming Platform - a Kasm Workspaces alternative for Kubernetes.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- PostgreSQL database
- NFS storage provisioner
- Authentik (for SSO)

## Installation

### 1. Add Helm Repository (Optional)

```bash
helm repo add workspace-platform https://your-charts-repo.com
helm repo update
```

### 2. Install Chart

```bash
# Create namespace
kubectl create namespace workspaces

# Install with default values
helm install workspace-platform ./chart -n workspaces

# Or install from repository
helm install workspace-platform workspace-platform/workspace-platform -n workspaces
```

### 3. Custom Installation

```bash
# Create custom values file
cat > custom-values.yaml <<EOF
controller:
  replicas: 2

secrets:
  postgresPassword: your-secure-password
  jwtSecret: $(openssl rand -base64 32)
  authentikClientSecret: your-authentik-secret

ingress:
  hostname: workspaces.example.com
  tls:
    enabled: true
    secretName: workspaces-tls

monitoring:
  enabled: true
EOF

# Install with custom values
helm install workspace-platform ./chart -n workspaces -f custom-values.yaml
```

## Configuration

The following table lists the configurable parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.image.repository` | Controller image repository | `your-registry/workspace-controller` |
| `controller.replicas` | Number of controller replicas | `1` |
| `controller.config.postgres.host` | PostgreSQL host | `postgres.data.svc.cluster.local` |
| `controller.config.hibernation.enabled` | Enable auto-hibernation | `true` |
| `controller.config.hibernation.defaultIdleTimeout` | Default idle timeout | `30m` |
| `api.replicas` | Number of API replicas | `2` |
| `ui.replicas` | Number of UI replicas | `2` |
| `ingress.enabled` | Enable Ingress | `true` |
| `ingress.hostname` | Ingress hostname | `workspaces.local` |
| `monitoring.enabled` | Enable monitoring | `true` |
| `secrets.postgresPassword` | PostgreSQL password | `postgres` (CHANGE!) |

## Upgrading

```bash
# Upgrade to latest version
helm upgrade workspace-platform ./chart -n workspaces

# Upgrade with custom values
helm upgrade workspace-platform ./chart -n workspaces -f custom-values.yaml
```

## Uninstalling

```bash
# Uninstall chart (keeps PVCs)
helm uninstall workspace-platform -n workspaces

# Delete namespace and all resources
kubectl delete namespace workspaces
```

## Post-Installation

### 1. Check Deployment Status

```bash
kubectl get pods -n workspaces
kubectl get svc -n workspaces
kubectl get ingress -n workspaces
```

### 2. Initialize Database

```bash
# Database init job should run automatically
kubectl get jobs -n workspaces

# Check logs
kubectl logs -n workspaces job/workspace-db-init
```

### 3. Access Web UI

```bash
# Get UI URL
echo "https://$(kubectl get ingress -n workspaces workspace-platform -o jsonpath='{.spec.rules[0].host}')"

# Or via LoadBalancer IP
kubectl get svc -n workspaces workspace-platform-ui
```

### 4. Configure Authentik

1. Login to Authentik admin panel
2. Create OAuth2/OIDC Provider for Workspaces
3. Update `authentikClientSecret` in values
4. Upgrade Helm release

## Troubleshooting

### Controller Not Starting

```bash
# Check controller logs
kubectl logs -n workspaces deploy/workspace-platform-controller

# Check events
kubectl describe pod -n workspaces -l app=workspace-controller
```

### Database Connection Issues

```bash
# Test PostgreSQL connection
kubectl exec -n workspaces deploy/workspace-platform-controller -- \
  pg_isready -h postgres.data.svc.cluster.local -p 5432

# Check database init job
kubectl logs -n workspaces job/workspace-db-init
```

### Templates Not Loading

```bash
# Check WorkspaceTemplates
kubectl get workspacetemplates -n workspaces

# Deploy templates manually
kubectl apply -f k8s/templates/
```

## Examples

### Minimal Installation

```yaml
controller:
  replicas: 1

secrets:
  postgresPassword: mypassword

ingress:
  hostname: workspaces.local
  tls:
    enabled: false

monitoring:
  enabled: false
```

### Production Installation

```yaml
controller:
  replicas: 2
  resources:
    requests:
      memory: 512Mi
      cpu: 500m
    limits:
      memory: 1Gi
      cpu: 1000m

api:
  replicas: 3

ui:
  replicas: 3

ingress:
  hostname: workspaces.example.com
  tls:
    enabled: true
    secretName: workspaces-tls

monitoring:
  enabled: true

controller:
  config:
    hibernation:
      defaultIdleTimeout: 15m
    cluster:
      memoryThreshold: 80
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/yourusername/ai-infra-k3s/issues
- Documentation: docs/workspaces/
