<div align="center">

# ⚡ StreamSpace Quick Start

**Get up and running in under 10 minutes.**

[![Status](https://img.shields.io/badge/Status-v2.0--beta-success.svg)](CHANGELOG.md)

</div>

---

## 📋 Prerequisites

- **Kubernetes Cluster** (1.19+): k3s (recommended for dev) or managed K8s.
- **kubectl**: Configured with cluster access.
- **Helm 3.0+**: Installed.
- **Storage**: ReadWriteMany (RWX) provisioner (e.g., NFS).

## 🚀 Installation

### 1. Create Namespace

```bash
kubectl create namespace streamspace
```

### 2. Install StreamSpace (Helm)

```bash
helm install streamspace ./chart -n streamspace --create-namespace
```

### 3. Verify Deployment

Ensure all components are running:

```bash
kubectl get pods -n streamspace
```

You should see:

- `streamspace-api` (Control Plane)
- `streamspace-ui` (Web Interface)
- `streamspace-k8s-agent` (Execution Agent)
- `postgres` (Database)

## 🖥️ First Steps

### 1. Access the UI

**Port Forward (Development)**:

```bash
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80
```

Open [http://localhost:3000](http://localhost:3000).

**Ingress (Production)**:
Access via your configured domain (e.g., `https://streamspace.yourdomain.com`).

### 2. Login

**Default Admin Credentials**:

- **Username**: `admin`
- **Password**: Retrieve via:

  ```bash
  kubectl get secret streamspace-admin-credentials -n streamspace -o jsonpath='{.data.password}' | base64 -d
  ```

### 3. Launch a Session

1. Go to **Catalog**.
2. Click **Launch** on "Firefox Web Browser".
3. Wait for the session to start (~30s).
4. Click the session card to connect.

> [!NOTE]
> **v2.0 Architecture**: The connection is proxied through the Control Plane via the Agent. No direct connection to the pod is required!

## 🛠️ Common Operations

### Create Session via CLI

```bash
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: cli-firefox
  namespace: streamspace
spec:
  user: admin
  template: firefox-browser
  state: running
  resources:
    memory: 2Gi
EOF
```

### Hibernate Session

```bash
kubectl patch session cli-firefox -n streamspace --type merge -p '{"spec":{"state":"hibernated"}}'
```

### View Logs

**Control Plane**:

```bash
kubectl logs -n streamspace deploy/streamspace-api -f
```

**Agent**:

```bash
kubectl logs -n streamspace deploy/streamspace-k8s-agent -f
```

---

<div align="center">
  <sub>StreamSpace Quick Start</sub>
</div>
