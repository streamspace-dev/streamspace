# StreamSpace Controller Specification

## Overview

The StreamSpace Controller is a platform-specific agent that manages the lifecycle of Sessions and Templates on a target infrastructure. It acts as a bridge between the central Control Plane (API) and the underlying platform (Kubernetes, Docker, Hyper-V, etc.).

## Architecture

### Agent Model

Controllers operate as **Agents**. They are installed on the target infrastructure and initiate an outbound connection to the Control Plane. This avoids the need for the Control Plane to have direct inbound access to the controllers, simplifying network configuration (firewalls, NAT).

### Communication Protocol

- **Transport**: Secure WebSocket (WSS) or gRPC over TLS.
- **Direction**: Outbound from Controller to Control Plane.
- **Authentication**: API Key or Mutual TLS (mTLS).

## Responsibilities

### 1. Control (Command Execution)

The Controller must execute commands received from the Control Plane:

- `StartSession(SessionSpec)`: Provision a new session.
- `StopSession(SessionID)`: Terminate a session.
- `HibernateSession(SessionID)`: Pause a session (release resources, keep state).
- `WakeSession(SessionID)`: Resume a hibernated session.

### 2. Monitor (Telemetry)

The Controller must collect and report metrics:

- **Node Metrics**: CPU, Memory, Disk usage of the host/node.
- **Session Metrics**: CPU, Memory usage of individual sessions.
- **Status**: Health status of the controller and the platform.

### 3. Log (Stream)

The Controller must provide access to session logs:

- Stream container/VM logs back to the Control Plane on demand.

### 4. Report (State Sync)

The Controller must periodically sync the state of all managed resources:

- List of active sessions.
- Status of each session (Running, Hibernated, Failed).
- Public endpoints (URLs) for accessing sessions.

## Data Models

### Session Spec (Abstract)

The Control Plane sends a platform-agnostic `SessionSpec`:

```json
{
  "id": "session-123",
  "user": "user-abc",
  "template": {
    "image": "lscr.io/linuxserver/firefox:latest",
    "env": {"PUID": "1000"},
    "ports": [{"name": "vnc", "port": 3000}]
  },
  "resources": {
    "cpu": "1000m",
    "memory": "2Gi"
  }
}
```

### Platform Translation

The Controller translates this spec into platform-specific resources:

- **Kubernetes**: Pod, Service, Ingress, PVC.
- **Docker**: Container, Network, Volume.
- **Hyper-V**: VM, VSwitch, VHDX.

## Implementation Guidelines

### Language

Go is recommended for all controllers to share common libraries (e.g., communication with Control Plane).

### Common Library (`streamspace-agent-sdk`)

A common SDK should be created to handle:

- WebSocket/gRPC connection management.
- Authentication handshake.
- Command dispatching.
- Metric collection primitives.

## Future Controllers

- **Docker Controller**: For single-node deployments (home labs).
- **vCenter Controller**: For enterprise VM-based environments.
- **LXD Controller**: For lightweight system containers.
