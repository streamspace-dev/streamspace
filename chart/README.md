# StreamSpace Helm Chart

This Helm chart deploys StreamSpace, a Kubernetes-native multi-user platform for streaming containerized applications to web browsers.

## Overview

StreamSpace provides:
- **Browser-based access** to any containerized application via KasmVNC
- **Multi-user support** with SSO authentication (Authentik/Keycloak)
- **Persistent home directories** using NFS storage
- **Auto-hibernation** for resource efficiency
- **200+ application templates** from LinuxServer.io
- **Plugin system** for extending platform functionality
- **Resource quotas** and limits per user
- **Comprehensive monitoring** with Grafana and Prometheus

## Prerequisites

- Kubernetes 1.19+ cluster
- Helm 3.x
- NFS storage provisioner (for ReadWriteMany PVCs)
- Ingress controller (Traefik, nginx, etc.)
- PostgreSQL database (can be deployed by chart or use external)

## Quick Start

### Install with Default Settings

```bash
# Add StreamSpace Helm repository (if published)
helm repo add streamspace https://streamspace.github.io/charts
helm repo update

# Install StreamSpace
helm install streamspace streamspace/streamspace \
  --namespace streamspace \
  --create-namespace
```

### Install from Local Chart

```bash
# From the repository root
helm install streamspace ./chart \
  --namespace streamspace \
  --create-namespace
```

### Access the UI

After installation, follow the instructions in the NOTES output to access StreamSpace:

```bash
# If using ingress (default)
echo "Access StreamSpace at: http://streamspace.local"

# Or port-forward for local access
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80
# Then visit http://localhost:3000
```

## Configuration

### Key Configuration Options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.enabled` | Deploy the StreamSpace controller | `true` |
| `controller.replicaCount` | Number of controller replicas | `1` |
| `controller.config.ingressDomain` | Base domain for session ingresses | `streamspace.local` |
| `controller.config.ingressClass` | Ingress class to use | `traefik` |
| `api.enabled` | Deploy the API backend | `true` |
| `api.replicaCount` | Number of API replicas | `2` |
| `api.autoscaling.enabled` | Enable HPA for API | `false` |
| `ui.enabled` | Deploy the web UI | `true` |
| `ui.replicaCount` | Number of UI replicas | `2` |
| `postgresql.enabled` | Deploy PostgreSQL database | `true` |
| `postgresql.external.enabled` | Use external PostgreSQL | `false` |
| `plugins.enabled` | Enable plugin system | `true` |
| `plugins.catalog.syncInterval` | Plugin repository sync interval | `1h` |
| `plugins.isolation.enabled` | Enable plugin sandboxing | `true` |
| `ingress.enabled` | Create ingress resources | `true` |
| `ingress.tls.enabled` | Enable TLS for ingress | `false` |
| `monitoring.enabled` | Deploy monitoring resources | `true` |
| `networkPolicy.enabled` | Create network policies | `false` |

### Example: Production Deployment

Create a `production-values.yaml` file:

```yaml
global:
  imageRegistry: "registry.mycompany.com"
  storageClass: "nfs-client"

controller:
  replicaCount: 3
  leaderElection:
    enabled: true
  resources:
    requests:
      memory: 512Mi
      cpu: 500m
    limits:
      memory: 1Gi
      cpu: 2000m
  podDisruptionBudget:
    enabled: true
    minAvailable: 2

api:
  replicaCount: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
    targetCPUUtilizationPercentage: 70
  config:
    corsOrigins: "https://streamspace.mycompany.com"
  podDisruptionBudget:
    enabled: true
    minAvailable: 2

ui:
  replicaCount: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
  podDisruptionBudget:
    enabled: true
    minAvailable: 2

postgresql:
  external:
    enabled: true
    host: "postgres.database.svc.cluster.local"
    port: 5432
    database: "streamspace"
    username: "streamspace"
    existingSecret: "streamspace-db-secret"

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: streamspace.mycompany.com
      paths:
        - path: /
          pathType: Prefix
          service: ui
        - path: /api
          pathType: Prefix
          service: api
  tls:
    enabled: true
    secretName: streamspace-tls

secrets:
  create: false
  existingSecret: "streamspace-secrets"

monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
    labels:
      prometheus: kube-prometheus
  prometheusRules:
    enabled: true
  grafanaDashboard:
    enabled: true
    namespace: "observability"

networkPolicy:
  enabled: true
  ingress:
    namespace: "ingress-nginx"
  monitoring:
    namespace: "observability"
```

Install with production values:

```bash
helm install streamspace ./chart \
  --namespace streamspace \
  --create-namespace \
  --values production-values.yaml
```

### Example: Development Deployment

For local development with minimal resources:

```yaml
controller:
  replicaCount: 1
  resources:
    requests:
      memory: 128Mi
      cpu: 100m

api:
  replicaCount: 1
  resources:
    requests:
      memory: 128Mi
      cpu: 100m

ui:
  replicaCount: 1
  resources:
    requests:
      memory: 64Mi
      cpu: 50m

postgresql:
  enabled: true
  internal:
    resources:
      requests:
        memory: 128Mi
        cpu: 100m
    persistence:
      size: 5Gi

ingress:
  enabled: false

monitoring:
  enabled: false
```

## Upgrading

### Upgrade to Latest Version

```bash
helm repo update
helm upgrade streamspace streamspace/streamspace \
  --namespace streamspace
```

### Upgrade with New Values

```bash
helm upgrade streamspace ./chart \
  --namespace streamspace \
  --values production-values.yaml
```

### Rollback

```bash
# List releases
helm history streamspace -n streamspace

# Rollback to previous version
helm rollback streamspace -n streamspace

# Rollback to specific revision
helm rollback streamspace 2 -n streamspace
```

## Uninstalling

```bash
# Uninstall the release
helm uninstall streamspace -n streamspace

# Optionally delete the namespace
kubectl delete namespace streamspace
```

**Warning:** Uninstalling will delete all sessions and user data unless you have configured external storage or backups.

## Advanced Configuration

### Using External PostgreSQL

To use an existing PostgreSQL database:

```yaml
postgresql:
  enabled: false  # Don't deploy internal PostgreSQL
  external:
    enabled: true
    host: "postgres.example.com"
    port: 5432
    database: "streamspace"
    username: "streamspace"
    existingSecret: "postgres-credentials"
    existingSecretPasswordKey: "password"
```

Create the secret:

```bash
kubectl create secret generic postgres-credentials \
  --from-literal=password='your-secure-password' \
  -n streamspace
```

### Custom Image Registry

To use a private registry:

```yaml
global:
  imageRegistry: "registry.mycompany.com"
  imagePullSecrets:
    - name: regcred

controller:
  image:
    repository: "streamspace/controller"
    tag: "v0.2.0"

api:
  image:
    repository: "streamspace/api"
    tag: "v0.2.0"

ui:
  image:
    repository: "streamspace/ui"
    tag: "v0.2.0"
```

Create the image pull secret:

```bash
kubectl create secret docker-registry regcred \
  --docker-server=registry.mycompany.com \
  --docker-username=your-username \
  --docker-password=your-password \
  --docker-email=your-email@example.com \
  -n streamspace
```

### TLS Configuration

#### Using cert-manager

```yaml
ingress:
  enabled: true
  className: "nginx"
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: streamspace.example.com
      paths:
        - path: /
          pathType: Prefix
          service: ui
        - path: /api
          pathType: Prefix
          service: api
  tls:
    enabled: true
    secretName: streamspace-tls  # Created by cert-manager
```

#### Using Existing Certificate

```bash
# Create TLS secret
kubectl create secret tls streamspace-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n streamspace
```

```yaml
ingress:
  tls:
    enabled: true
    secretName: streamspace-tls
```

### Resource Quotas

Configure default resources for user sessions:

```yaml
sessionDefaults:
  resources:
    requests:
      memory: 4Gi
      cpu: 2000m
    limits:
      memory: 8Gi
      cpu: 4000m
  persistentHome:
    enabled: true
    size: 100Gi
  idleTimeout: 1h
  maxSessionDuration: 12h
```

### High Availability Setup

For production HA deployment:

```yaml
controller:
  replicaCount: 3
  leaderElection:
    enabled: true
  podDisruptionBudget:
    enabled: true
    minAvailable: 2
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          podAffinityTerm:
            labelSelector:
              matchLabels:
                app.kubernetes.io/component: controller
            topologyKey: kubernetes.io/hostname

api:
  replicaCount: 5
  autoscaling:
    enabled: true
    minReplicas: 5
    maxReplicas: 30
  podDisruptionBudget:
    enabled: true
    minAvailable: 3

postgresql:
  external:
    enabled: true
    host: "postgres-ha.database.svc.cluster.local"
```

### Monitoring Setup

Enable Prometheus and Grafana integration:

```yaml
monitoring:
  enabled: true

  serviceMonitor:
    enabled: true
    namespace: "observability"
    labels:
      prometheus: kube-prometheus
    interval: 30s

  prometheusRules:
    enabled: true
    labels:
      prometheus: kube-prometheus
    interval: 30s
    alerts:
      highSessionCount:
        threshold: 200
        duration: 15m

  grafanaDashboard:
    enabled: true
    namespace: "observability"
    labels:
      grafana_dashboard: "1"
```

### Network Policies

Enable network policies for enhanced security:

```yaml
networkPolicy:
  enabled: true
  ingress:
    namespace: "ingress-nginx"
  monitoring:
    namespace: "observability"
  controller:
    restrictEgress: true  # Only allow specific egress
```

## Troubleshooting

### Check Installation Status

```bash
# Check all resources
kubectl get all -n streamspace

# Check pods
kubectl get pods -n streamspace

# Check services
kubectl get svc -n streamspace

# Check ingress
kubectl get ingress -n streamspace
```

### View Logs

```bash
# Controller logs
kubectl logs -n streamspace deploy/streamspace-controller -f

# API logs
kubectl logs -n streamspace deploy/streamspace-api -f

# UI logs
kubectl logs -n streamspace deploy/streamspace-ui -f

# PostgreSQL logs
kubectl logs -n streamspace statefulset/streamspace-postgres -f
```

### Common Issues

#### Pods Not Starting

Check pod events:
```bash
kubectl describe pod <pod-name> -n streamspace
```

Common causes:
- Image pull errors: Check image names and pull secrets
- Resource constraints: Check node capacity
- PVC issues: Verify storage provisioner

#### Database Connection Failures

Check API logs:
```bash
kubectl logs -n streamspace deploy/streamspace-api | grep -i database
```

Verify database connection:
```bash
kubectl exec -it -n streamspace deploy/streamspace-api -- sh -c 'nc -zv $DB_HOST $DB_PORT'
```

#### Ingress Not Working

Check ingress status:
```bash
kubectl describe ingress streamspace -n streamspace
```

Verify ingress controller is running:
```bash
kubectl get pods -n kube-system | grep -i ingress
# or
kubectl get pods -n ingress-nginx
```

#### Sessions Not Creating

Check controller logs:
```bash
kubectl logs -n streamspace deploy/streamspace-controller -f
```

Verify CRDs are installed:
```bash
kubectl get crds | grep streamspace
```

Test creating a session manually:
```bash
kubectl apply -f - <<EOF
apiVersion: stream.streamspace.io/v1alpha1
kind: Session
metadata:
  name: test-session
  namespace: streamspace
spec:
  user: testuser
  template: firefox-browser
  state: running
EOF

# Check session status
kubectl get sessions -n streamspace
kubectl describe session test-session -n streamspace
```

## Values Reference

See [values.yaml](values.yaml) for complete configuration options with comments.

Key sections:
- `global.*` - Global settings (registry, storage class)
- `controller.*` - Controller configuration
- `api.*` - API backend configuration
- `ui.*` - Web UI configuration
- `postgresql.*` - Database configuration
- `ingress.*` - Ingress configuration
- `secrets.*` - Secret management
- `monitoring.*` - Prometheus/Grafana integration
- `networkPolicy.*` - Network policy settings
- `sessionDefaults.*` - Default session resources

## Support

- **Documentation**: https://docs.streamspace.io
- **GitHub Issues**: https://github.com/streamspace/streamspace/issues
- **Discussions**: https://github.com/streamspace/streamspace/discussions
- **Discord**: https://discord.gg/streamspace

## License

MIT License - see [LICENSE](../LICENSE) for details.
