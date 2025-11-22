<div align="center">

# 🚀 StreamSpace Deployment Guide

**Version**: v2.0-beta • **Last Updated**: 2025-11-21

[![Status](https://img.shields.io/badge/Status-v2.0--beta-success.svg)](CHANGELOG.md)

</div>

---

> [!IMPORTANT]
> **Prerequisites**
>
> - **Kubernetes Cluster** (1.19+): k3s (dev) or GKE/EKS/AKS (prod).
> - **kubectl**: Configured with cluster access.
> - **Helm 3.0+**: For package management.
> - **Storage**: ReadWriteMany (RWX) provisioner (e.g., NFS).

## ⚡ Quick Start

### 1. Create Namespace

```bash
kubectl create namespace streamspace
```

### 2. Deploy CRDs

```bash
kubectl apply -f manifests/crds/
```

> [!NOTE]
> Verify CRDs are installed: `kubectl get crds | grep stream.space`

### 3. Install via Helm

```bash
helm install streamspace ./chart -n streamspace --create-namespace
```

### 4. Create a Session

```bash
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: my-firefox
  namespace: streamspace
spec:
  user: admin
  template: firefox-browser
  state: running
  resources:
    memory: 2Gi
EOF
```

## 🛠️ Detailed Configuration

### PostgreSQL Database

> [!WARNING]
> **Production Security**: Do NOT use the default password in production.

**Option 1: In-Cluster (Development)**

```bash
kubectl apply -f manifests/config/streamspace-postgres.yaml
```

**Option 2: External (Production)**
Create a secret with your connection details:

```bash
kubectl create secret generic streamspace-secrets \
  -n streamspace \
  --from-literal=postgres-password='YOUR_SECURE_PASSWORD'
```

### Storage Configuration

StreamSpace requires **ReadWriteMany (RWX)** storage for user home directories.

**NFS Provisioner (Recommended)**

```bash
helm repo add nfs-subdir-external-provisioner https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/
helm install nfs-provisioner nfs-subdir-external-provisioner/nfs-subdir-external-provisioner \
  --namespace kube-system \
  --set nfs.server=YOUR_NFS_SERVER_IP \
  --set nfs.path=/exported/path
```

### Ingress & TLS

**Cert-Manager (Recommended)**

1. Install cert-manager.
2. Create a `ClusterIssuer`.
3. Enable TLS in your Helm values or Ingress manifest.

```yaml
ingress:
  enabled: true
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: streamspace.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: streamspace-tls
      hosts:
        - streamspace.yourdomain.com
```

## 📊 Monitoring

StreamSpace exposes Prometheus metrics.

1. **Install Prometheus Stack**:

   ```bash
   helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring
   ```

2. **Apply ServiceMonitor**:

   ```bash
   kubectl apply -f manifests/monitoring/servicemonitor.yaml
   ```

3. **Access Grafana**:
   Login with default credentials (`admin` / `prom-operator`) and import the StreamSpace dashboard.

## 🔍 Troubleshooting

| Issue | Check | Command |
| :--- | :--- | :--- |
| **Pods Pending** | Storage/Resources | `kubectl describe pod <pod-name> -n streamspace` |
| **DB Error** | Connection/Secret | `kubectl logs deploy/streamspace-api -n streamspace` |
| **Ingress 404** | Ingress Class | `kubectl get ingress -n streamspace` |
| **Session Fail** | Controller Logs | `kubectl logs deploy/streamspace-controller -n streamspace` |

---

<div align="center">
  <sub>StreamSpace Deployment Guide</sub>
</div>
