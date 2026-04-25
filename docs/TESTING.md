<div align="center">

# 🧪 StreamSpace Testing Guide

**Version**: v2.0-beta • **Platform**: Kubernetes

</div>

---

## 📋 Overview

This guide covers testing for the StreamSpace v2.0 architecture, including the Control Plane, K8s Agent, and Web UI.

## 🛠️ Component Testing

### 1. Kubernetes Agent

**Verify Agent Status**:

```bash
# Check pod
kubectl get pods -n streamspace -l app=streamspace-k8s-agent

# Check logs
kubectl logs -n streamspace deploy/streamspace-k8s-agent -f
```

**Verify Connection**:
The agent should log: `Connected to Control Plane`

### 2. Control Plane (API)

**Verify API Status**:

```bash
kubectl get pods -n streamspace -l app=streamspace-api
```

**Test Health Endpoint**:

```bash
kubectl port-forward -n streamspace svc/streamspace-api 8000:8000
curl http://localhost:8000/health
```

### 3. Web UI

**Verify UI Status**:

```bash
kubectl get pods -n streamspace -l app=streamspace-ui
```

**Access UI**:

```bash
kubectl port-forward -n streamspace svc/streamspace-ui 3000:80
# Open http://localhost:3000
```

## 🔄 Integration Testing

### Session Lifecycle Test

1. **Create Session**:

    ```bash
    kubectl apply -f manifests/examples/session-firefox.yaml
    ```

2. **Verify Agent Action**:
    Check agent logs to see it receiving the command and creating the pod.

    ```bash
    kubectl logs -n streamspace deploy/streamspace-k8s-agent
    ```

3. **Verify Pod Creation**:

    ```bash
    kubectl get pods -n streamspace -l session=my-firefox
    ```

4. **Verify VNC Tunnel**:
    In v2.0, the agent tunnels VNC traffic. Connect via the UI and verify the WebSocket connection in the browser network tab.

### Hibernation Test

1. **Trigger Hibernation**:

    ```bash
    kubectl patch session my-firefox -n streamspace --type merge -p '{"spec":{"state":"hibernated"}}'
    ```

2. **Verify Scale Down**:

    ```bash
    kubectl get deploy -n streamspace -l session=my-firefox
    # Replicas should be 0
    ```

## 🐛 Troubleshooting

| Issue | Check |
| :--- | :--- |
| **Agent not connecting** | Check `API_URL` env var in agent deployment. |
| **Session pending** | Check agent logs for errors creating K8s resources. |
| **VNC disconnects** | Check WebSocket connection in browser console. |

---

<div align="center">
  <sub>StreamSpace Testing Guide</sub>
</div>
