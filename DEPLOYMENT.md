# StreamSpace Deployment Guide

Complete guide for deploying StreamSpace to a Kubernetes cluster.

## Prerequisites

### Required Tools

- **Kubernetes cluster** (1.19+)
  - Recommended: k3s for development, GKE/EKS/AKS for production
  - Support for Custom Resource Definitions (CRDs)
- **kubectl** configured with cluster access
- **Docker** for building container images
- **Container registry** (Docker Hub, GitHub Container Registry, or private registry)

### Required Cluster Features

- **Storage provisioner** with ReadWriteMany support (NFS recommended)
- **Ingress controller** (Traefik or Nginx)
- **PostgreSQL** (can be deployed in-cluster or external)

---

## Quick Start (All-in-One Deployment)

### 1. Create Namespace

```bash
kubectl create namespace streamspace
```

### 2. Deploy CRDs

```bash
kubectl apply -f controller/config/crd/bases/
```

Verify:
```bash
kubectl get crds | grep stream.streamspace.io
# Should see:
# sessions.stream.streamspace.io
# templates.stream.streamspace.io
```

### 3. Deploy PostgreSQL

```bash
# Review and update password in manifests/config/streamspace-postgres.yaml
kubectl apply -f manifests/config/streamspace-postgres.yaml
```

Wait for PostgreSQL to be ready:
```bash
kubectl wait --for=condition=ready pod -l component=database -n streamspace --timeout=120s
```

### 4. Build and Push Container Images

#### Controller

```bash
cd controller
docker build -t your-registry/streamspace-controller:v0.2.0 .
docker push your-registry/streamspace-controller:v0.2.0
```

#### API Backend

```bash
cd ../api
docker build -t your-registry/streamspace-api:v0.2.0 .
docker push your-registry/streamspace-api:v0.2.0
```

#### Web UI

```bash
cd ../ui

# Update API URL for production (optional)
# Edit .env.production or pass build arg
# VITE_API_URL=https://streamspace.yourdomain.com/api

docker build -t your-registry/streamspace-ui:v0.2.0 .
docker push your-registry/streamspace-ui:v0.2.0
```

### 5. Update Image References

Edit the deployment manifests to use your registry:

```bash
# Update controller image
sed -i 's|your-registry/streamspace-controller:v0.2.0|ghcr.io/yourname/streamspace-controller:v0.2.0|' \
  controller/config/manager/controller-deployment.yaml

# Update API image
sed -i 's|your-registry/streamspace-api:v0.2.0|ghcr.io/yourname/streamspace-api:v0.2.0|' \
  manifests/config/streamspace-api-deployment.yaml

# Update UI image
sed -i 's|your-registry/streamspace-ui:v0.2.0|ghcr.io/yourname/streamspace-ui:v0.2.0|' \
  manifests/config/streamspace-ui-deployment.yaml
```

### 6. Deploy Controller

```bash
cd controller

# Deploy RBAC
kubectl apply -f config/rbac/

# Deploy controller
kubectl apply -f config/manager/

# Verify
kubectl get pods -n streamspace -l control-plane=controller-manager
kubectl logs -n streamspace deploy/streamspace-controller -f
```

### 7. Deploy API Backend

```bash
kubectl apply -f manifests/config/streamspace-api-deployment.yaml

# Verify
kubectl get pods -n streamspace -l component=api
kubectl logs -n streamspace deploy/streamspace-api -f
```

### 8. Deploy Web UI

```bash
# Update domain in ingress (edit manifests/config/streamspace-ui-deployment.yaml)
# Change streamspace.local to your domain

kubectl apply -f manifests/config/streamspace-ui-deployment.yaml

# Verify
kubectl get pods -n streamspace -l component=ui
kubectl get ingress -n streamspace
```

### 9. Create Sample Templates

```bash
kubectl apply -f manifests/templates/browsers/firefox.yaml
kubectl apply -f manifests/templates/development/vscode.yaml

# Verify
kubectl get templates -n streamspace
```

### 10. Access the Platform

#### Local Development (Port Forward)

```bash
# Forward UI
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80

# Forward API (if needed)
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000

# Visit http://localhost:3000
```

#### Production (via Ingress)

Update your DNS to point to the ingress controller's IP, then visit:
```
https://streamspace.yourdomain.com
```

---

## Detailed Component Deployment

### PostgreSQL Database

#### Option 1: In-Cluster PostgreSQL (Development)

Use the provided manifest:
```bash
kubectl apply -f manifests/config/streamspace-postgres.yaml
```

**Important**: Update the password in `streamspace-secrets` before deploying to production!

#### Option 2: External PostgreSQL (Production)

Create a secret with connection details:

```bash
kubectl create secret generic streamspace-secrets \
  -n streamspace \
  --from-literal=postgres-password='your-secure-password'
```

Update `manifests/config/streamspace-api-deployment.yaml`:
```yaml
env:
  - name: DB_HOST
    value: your-postgres-server.example.com
  - name: DB_PORT
    value: "5432"
  - name: DB_NAME
    value: streamspace
  - name: DB_USER
    value: streamspace
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: streamspace-secrets
        key: postgres-password
  - name: DB_SSLMODE
    value: require  # Enable SSL for production
```

### Controller

The controller watches Session and Template CRDs and manages their lifecycle.

**Configuration via Environment Variables:**

Edit `controller/config/manager/controller-deployment.yaml`:

```yaml
env:
  - name: INGRESS_DOMAIN
    value: streamspace.yourdomain.com
  - name: INGRESS_CLASS
    value: nginx  # or traefik
  - name: LEADER_ELECT
    value: "true"  # Enable for HA deployments
```

**Scaling for High Availability:**

```bash
kubectl scale deployment streamspace-controller -n streamspace --replicas=3
```

Note: Leader election ensures only one controller is active at a time.

### API Backend

The API backend provides REST and WebSocket endpoints.

**Configuration:**

```yaml
env:
  # Database
  - name: DB_HOST
    value: postgres.streamspace.svc.cluster.local
  - name: DB_PORT
    value: "5432"

  # Repository sync
  - name: SYNC_INTERVAL
    value: 1h  # How often to sync Git repositories

  # CORS (restrict in production)
  - name: CORS_ORIGINS
    value: https://streamspace.yourdomain.com
```

**Horizontal Scaling:**

```bash
kubectl scale deployment streamspace-api -n streamspace --replicas=5
```

The API is stateless and can scale horizontally. WebSocket connections will be distributed across replicas.

### Web UI

The UI is a static React application served by nginx.

**Build-Time Configuration:**

API URL is configured at build time. Create a custom build:

```bash
cd ui
echo "VITE_API_URL=https://streamspace.yourdomain.com/api" > .env.production
docker build -t your-registry/streamspace-ui:v0.2.0 .
```

**Horizontal Scaling:**

```bash
kubectl scale deployment streamspace-ui -n streamspace --replicas=3
```

---

## Storage Configuration

StreamSpace requires ReadWriteMany (RWX) storage for user home directories.

### NFS Provisioner (Recommended)

#### Install NFS CSI Driver

```bash
helm repo add nfs-subdir-external-provisioner \
  https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/

helm install nfs-provisioner nfs-subdir-external-provisioner/nfs-subdir-external-provisioner \
  --namespace kube-system \
  --set nfs.server=your-nfs-server.local \
  --set nfs.path=/exported/path \
  --set storageClass.name=nfs-client \
  --set storageClass.defaultClass=true
```

#### Verify

```bash
kubectl get storageclass
kubectl get pods -n kube-system -l app=nfs-subdir-external-provisioner
```

### Alternative: Longhorn, Rook/Ceph, or Cloud Provider RWX

Update the `storageClassName` in user PVC creation (controller code or CRD defaults).

---

## Ingress Configuration

### Traefik (Default)

StreamSpace is configured for Traefik by default.

#### Install Traefik

```bash
helm repo add traefik https://helm.traefik.io/traefik
helm install traefik traefik/traefik --namespace kube-system
```

#### Configure DNS

Point your domain to the Traefik LoadBalancer IP:

```bash
kubectl get svc traefik -n kube-system

# Add DNS A record:
# streamspace.yourdomain.com -> <EXTERNAL-IP>
# *.streamspace.yourdomain.com -> <EXTERNAL-IP>  (for session subdomains)
```

### Nginx Ingress

To use Nginx instead of Traefik:

1. Install Nginx Ingress Controller
2. Update `manifests/config/streamspace-ui-deployment.yaml`:
   ```yaml
   spec:
     ingressClassName: nginx
   ```
3. Update controller environment:
   ```yaml
   env:
     - name: INGRESS_CLASS
       value: nginx
   ```

---

## TLS/HTTPS Configuration

### Option 1: cert-manager (Recommended)

#### Install cert-manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

#### Create ClusterIssuer

```bash
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
      - http01:
          ingress:
            class: traefik  # or nginx
EOF
```

#### Enable TLS in Ingress

Edit `manifests/config/streamspace-ui-deployment.yaml`:

```yaml
metadata:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod

spec:
  tls:
    - hosts:
        - streamspace.yourdomain.com
      secretName: streamspace-tls
```

Apply:
```bash
kubectl apply -f manifests/config/streamspace-ui-deployment.yaml
```

cert-manager will automatically obtain and renew certificates.

### Option 2: Manual TLS Certificate

Create a secret with your certificate:

```bash
kubectl create secret tls streamspace-tls \
  -n streamspace \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key
```

---

## Monitoring & Observability

### Prometheus Metrics

The controller exposes Prometheus metrics on port `:8080/metrics`.

#### Install Prometheus

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace
```

#### Create ServiceMonitor

```bash
kubectl apply -f manifests/monitoring/servicemonitor.yaml
```

#### View Metrics

Access Prometheus:
```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
```

Visit http://localhost:9090 and query:
```promql
streamspace_sessions_total
streamspace_sessions_by_user
streamspace_session_reconciliation_duration_seconds
```

### Grafana Dashboards

Import the pre-built dashboard:

```bash
kubectl apply -f manifests/monitoring/grafana-dashboard-workspace-overview.yaml
```

Access Grafana:
```bash
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
```

Default credentials: `admin / prom-operator`

---

## Upgrading StreamSpace

### Rolling Update

```bash
# Build new images with new tag
docker build -t your-registry/streamspace-api:v0.3.0 .
docker push your-registry/streamspace-api:v0.3.0

# Update deployment
kubectl set image deployment/streamspace-api \
  api=your-registry/streamspace-api:v0.3.0 \
  -n streamspace

# Watch rollout
kubectl rollout status deployment/streamspace-api -n streamspace
```

### Rollback

```bash
kubectl rollout undo deployment/streamspace-api -n streamspace
```

### CRD Updates

When updating CRDs:

```bash
# Backup existing resources
kubectl get sessions -n streamspace -o yaml > sessions-backup.yaml

# Update CRDs
kubectl apply -f controller/config/crd/bases/

# Verify no resources were lost
kubectl get sessions -n streamspace
```

---

## Production Checklist

### Security

- [ ] Change PostgreSQL password from default
- [ ] Restrict CORS origins to your domain
- [ ] Enable TLS/HTTPS with valid certificates
- [ ] Configure network policies
- [ ] Enable Pod Security Standards
- [ ] Use secrets management (Vault, Sealed Secrets, etc.)
- [ ] Configure RBAC with minimal permissions

### Reliability

- [ ] Scale controller to 3 replicas with leader election
- [ ] Scale API to 3+ replicas
- [ ] Configure resource requests and limits
- [ ] Set up liveness and readiness probes
- [ ] Configure PodDisruptionBudgets
- [ ] Test backup and restore procedures

### Performance

- [ ] Configure horizontal pod autoscaling (HPA)
- [ ] Tune PostgreSQL for your workload
- [ ] Enable caching in API (Redis)
- [ ] Configure CDN for static assets
- [ ] Optimize container images (multi-stage builds)

### Monitoring

- [ ] Deploy Prometheus and Grafana
- [ ] Create alert rules for critical metrics
- [ ] Configure log aggregation (Loki, ElasticSearch)
- [ ] Set up uptime monitoring
- [ ] Create runbooks for common issues

---

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n streamspace

# View pod events
kubectl describe pod <pod-name> -n streamspace

# Check logs
kubectl logs <pod-name> -n streamspace
```

Common issues:
- Image pull errors: Check registry credentials
- CrashLoopBackOff: Check logs for errors
- Pending: Check node resources and PVC provisioning

### Database Connection Errors

```bash
# Test PostgreSQL connectivity
kubectl run -it --rm psql --image=postgres:15 --restart=Never -- \
  psql -h postgres.streamspace.svc.cluster.local -U streamspace -d streamspace

# Check database pod
kubectl logs -n streamspace statefulset/postgres
```

### Ingress Not Working

```bash
# Check ingress status
kubectl get ingress -n streamspace
kubectl describe ingress streamspace -n streamspace

# Check ingress controller
kubectl get pods -n kube-system -l app.kubernetes.io/name=traefik

# Test DNS resolution
nslookup streamspace.yourdomain.com
```

### Sessions Not Creating

```bash
# Check controller logs
kubectl logs -n streamspace deploy/streamspace-controller -f

# Check CRDs are installed
kubectl get crds | grep stream.streamspace.io

# Check RBAC permissions
kubectl auth can-i create deployments --as=system:serviceaccount:streamspace:streamspace-controller -n streamspace
```

---

## Architecture Diagram

```
┌─────────────────┐
│    Internet     │
└────────┬────────┘
         │
    ┌────▼─────┐
    │ Ingress  │ (Traefik/Nginx + TLS)
    └────┬─────┘
         │
    ┌────┴──────────────────────┐
    │                           │
┌───▼───┐                  ┌───▼───┐
│  UI   │                  │  API  │
│(nginx)│                  │ (Go)  │
└───────┘                  └───┬───┘
                               │
         ┌─────────────────────┼──────────────────────┐
         │                     │                      │
    ┌────▼────┐          ┌────▼────┐         ┌──────▼──────┐
    │ Postgres│          │   K8s   │         │  WebSocket  │
    │ Database│          │   API   │         │     Hub     │
    └─────────┘          └────┬────┘         └─────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
              ┌─────▼─────┐      ┌─────▼──────┐
              │Controller │      │  Sessions  │
              │  (Go)     │      │   (CRDs)   │
              └───────────┘      └────────────┘
```

---

## Next Steps

After deployment:

1. **Test the platform**: Create a session, connect, verify hibernation
2. **Add templates**: Create custom templates for your applications
3. **Configure authentication**: Integrate with OIDC provider (Phase 2.3)
4. **Set up monitoring**: Configure alerts and dashboards
5. **Performance tuning**: Optimize based on your workload

---

## Support

For issues and questions:
- GitHub Issues: https://github.com/yourname/streamspace/issues
- Documentation: https://docs.streamspace.io (coming soon)

---

**License**: MIT
**Version**: v0.2.0
**Updated**: 2025-11-14
