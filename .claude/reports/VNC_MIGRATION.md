# VNC Migration Guide

**Status**: Planning Document (Phase 3 - Not Yet Implemented)
**Target Timeline**: Months 7-9 (Q3 2025)
**Last Updated**: 2025-11-14

---

## 🎯 Overview

This document provides a comprehensive guide for migrating StreamSpace from KasmVNC to a fully open source VNC stack based on TigerVNC and noVNC.

**Migration Goal**: Achieve 100% open source independence by replacing proprietary KasmVNC technology with community-maintained alternatives while maintaining or improving performance and user experience.

---

## 📊 Current State Analysis

### Dependencies to Replace

**KasmVNC References** (50+ locations):

```bash
# Find all KasmVNC references
grep -ri "kasm\|Kasm\|KASM" --include="*.{go,yaml,yml,md}" .

# Key files affected:
- manifests/crds/template.yaml (kasmvnc field)
- manifests/crds/workspacetemplate.yaml (kasmvnc field)
- manifests/config/database-init.yaml (kasmvnc columns)
- manifests/templates/*/*.yaml (22 template files)
- docs/ARCHITECTURE.md
- docs/CONTROLLER_GUIDE.md
- scripts/generate-templates.py
```

**LinuxServer.io Images** (22 templates):

```bash
# All current templates use LinuxServer.io
ls manifests/templates/*/*.yaml

# Image pattern: lscr.io/linuxserver/<app>:latest
# Port pattern: 3000 (KasmVNC default)
```

---

## 🏗️ Target Architecture

### New VNC Stack

```
┌─────────────────────────────────────────────────────┐
│  User's Web Browser                                  │
│  - Modern browser (Chrome, Firefox, Safari, Edge)   │
└────────────────────┬────────────────────────────────┘
                     │ HTTPS (Port 443)
                     ↓
┌─────────────────────────────────────────────────────┐
│  Ingress Controller (Traefik)                        │
│  - TLS termination                                   │
│  - ForwardAuth (Authentik SSO)                      │
│  - Route: {session-name}.streamspace.local          │
└────────────────────┬────────────────────────────────┘
                     │ HTTP/WebSocket
                     ↓
┌─────────────────────────────────────────────────────┐
│  StreamSpace API Backend (Go)                        │
│  - WebSocket Proxy (/vnc/{session-id})              │
│  - JWT Authentication                                │
│  - Connection Routing                                │
│  - Rate Limiting                                     │
│  - Metrics Collection                                │
└────────────────────┬────────────────────────────────┘
                     │ WebSocket → TCP (Port 5900)
                     ↓
┌─────────────────────────────────────────────────────┐
│  noVNC Client (JavaScript) [EMBEDDED IN WEB UI]      │
│  - RFB Protocol Implementation                       │
│  - HTML5 Canvas Rendering                            │
│  - Input Event Handling (Keyboard, Mouse)           │
│  - Clipboard Synchronization                         │
│  - StreamSpace Custom Branding                       │
└────────────────────┬────────────────────────────────┘
                     │ RFB Protocol over WebSocket
                     ↓
┌─────────────────────────────────────────────────────┐
│  Session Pod (Kubernetes)                            │
│  ┌─────────────────────────────────────────────┐   │
│  │ Container: streamspace/<app>:latest         │   │
│  │                                              │   │
│  │  ┌────────────────────────────────────┐    │   │
│  │  │ Application (Firefox, VS Code...)  │    │   │
│  │  └───────────────┬────────────────────┘    │   │
│  │                  │ X11                      │   │
│  │  ┌───────────────▼────────────────────┐    │   │
│  │  │ Window Manager (XFCE/i3/Openbox)   │    │   │
│  │  └───────────────┬────────────────────┘    │   │
│  │                  │ X11                      │   │
│  │  ┌───────────────▼────────────────────┐    │   │
│  │  │ Xvfb (Virtual Framebuffer)         │    │   │
│  │  │ Display: :1                         │    │   │
│  │  │ Resolution: 1920x1080x24           │    │   │
│  │  └───────────────┬────────────────────┘    │   │
│  │                  │ X11 Protocol             │   │
│  │  ┌───────────────▼────────────────────┐    │   │
│  │  │ TigerVNC Server (Xvnc)             │    │   │
│  │  │ Port: 5900                          │    │   │
│  │  │ Protocol: RFB 3.8                   │    │   │
│  │  │ Compression: Tight, JPEG            │    │   │
│  │  └────────────────────────────────────┘    │   │
│  │                                              │   │
│  │  Volumes:                                    │   │
│  │  - /home/user → PVC (home-{username})      │   │
│  │  - /tmp/.X11-unix → tmpfs                  │   │
│  └─────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

---

## 🔧 Component Implementation Details

### 1. TigerVNC Server

**Installation in Container**:

```dockerfile
FROM ubuntu:22.04

# Install TigerVNC and dependencies
RUN apt-get update && apt-get install -y \
    tigervnc-standalone-server \
    tigervnc-common \
    xvfb \
    xfce4 \
    xfce4-terminal \
    dbus-x11 \
    && rm -rf /var/lib/apt/lists/*

# Configure VNC
RUN mkdir -p ~/.vnc && \
    echo "password" | vncpasswd -f > ~/.vnc/passwd && \
    chmod 600 ~/.vnc/passwd

# VNC startup script
COPY vnc-startup.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/vnc-startup.sh

EXPOSE 5900

CMD ["/usr/local/bin/vnc-startup.sh"]
```

**VNC Startup Script** (`vnc-startup.sh`):

```bash
#!/bin/bash
set -e

# Set display
export DISPLAY=:1

# Start Xvfb
Xvfb :1 -screen 0 1920x1080x24 -ac +extension GLX +render -noreset &
XVFB_PID=$!

# Wait for X server
sleep 2

# Start window manager
startxfce4 &

# Start TigerVNC server
vncserver :1 \
    -geometry 1920x1080 \
    -depth 24 \
    -SecurityTypes None \
    -AlwaysShared \
    -AcceptPointerEvents \
    -AcceptKeyEvents \
    -AcceptSetDesktopSize \
    -SendCutText \
    -AcceptCutText

# Keep container running
tail -f ~/.vnc/*.log
```

**Configuration Options**:

```bash
# ~/.vnc/config
geometry=1920x1080
depth=24
SecurityTypes=None
AlwaysShared=1
AcceptPointerEvents=1
AcceptKeyEvents=1
AcceptSetDesktopSize=1
```

### 2. noVNC Client

**Integration Approach**:

```typescript
// Web UI: components/VNCViewer.tsx
import React, { useEffect, useRef } from 'react';
import RFB from '@novnc/novnc/core/rfb';

interface VNCViewerProps {
  sessionId: string;
  wsUrl: string; // wss://api.streamspace.local/vnc/{sessionId}
}

export const VNCViewer: React.FC<VNCViewerProps> = ({ sessionId, wsUrl }) => {
  const canvasRef = useRef<HTMLDivElement>(null);
  const rfbRef = useRef<RFB | null>(null);

  useEffect(() => {
    if (!canvasRef.current) return;

    // Initialize noVNC
    const rfb = new RFB(canvasRef.current, wsUrl, {
      credentials: {
        // Authentication handled by WebSocket proxy
      },
      wsProtocols: ['binary'],
    });

    // Event handlers
    rfb.addEventListener('connect', () => {
      console.log('VNC connected');
    });

    rfb.addEventListener('disconnect', () => {
      console.log('VNC disconnected');
    });

    rfb.scaleViewport = true;
    rfb.resizeSession = true;
    rfb.clipViewport = false;

    rfbRef.current = rfb;

    return () => {
      rfb.disconnect();
    };
  }, [wsUrl]);

  return (
    <div
      ref={canvasRef}
      style={{ width: '100%', height: '100vh' }}
    />
  );
};
```

**Custom Branding**:

```css
/* Custom noVNC styling */
.novnc-canvas {
  cursor: default;
}

.novnc-control-bar {
  background: var(--streamspace-primary);
  /* Hide noVNC logo, show StreamSpace branding */
}
```

### 3. WebSocket Proxy

**Go Implementation**:

```go
// api/internal/vnc/proxy.go
package vnc

import (
    "context"
    "io"
    "net"
    "net/http"
    "time"

    "github.com/gorilla/websocket"
    "go.uber.org/zap"
)

type VNCProxy struct {
    logger   *zap.Logger
    upgrader websocket.Upgrader
}

func NewVNCProxy(logger *zap.Logger) *VNCProxy {
    return &VNCProxy{
        logger: logger,
        upgrader: websocket.Upgrader{
            ReadBufferSize:  1024 * 64,
            WriteBufferSize: 1024 * 64,
            CheckOrigin: func(r *http.Request) bool {
                // TODO: Implement proper CORS checking
                return true
            },
        },
    }
}

func (p *VNCProxy) HandleConnection(w http.ResponseWriter, r *http.Request, sessionID string) error {
    // Upgrade HTTP connection to WebSocket
    wsConn, err := p.upgrader.Upgrade(w, r, nil)
    if err != nil {
        return err
    }
    defer wsConn.Close()

    // Get VNC server address from session
    vncAddr, err := p.getVNCAddress(r.Context(), sessionID)
    if err != nil {
        return err
    }

    // Connect to VNC server
    vncConn, err := net.DialTimeout("tcp", vncAddr, 10*time.Second)
    if err != nil {
        return err
    }
    defer vncConn.Close()

    // Bidirectional copy
    errChan := make(chan error, 2)

    // WebSocket → VNC
    go func() {
        errChan <- p.wsToTCP(wsConn, vncConn)
    }()

    // VNC → WebSocket
    go func() {
        errChan <- p.tcpToWS(vncConn, wsConn)
    }()

    // Wait for either direction to complete
    return <-errChan
}

func (p *VNCProxy) wsToTCP(ws *websocket.Conn, tcp net.Conn) error {
    for {
        messageType, data, err := ws.ReadMessage()
        if err != nil {
            return err
        }

        if messageType == websocket.BinaryMessage {
            if _, err := tcp.Write(data); err != nil {
                return err
            }
        }
    }
}

func (p *VNCProxy) tcpToWS(tcp net.Conn, ws *websocket.Conn) error {
    buffer := make([]byte, 32*1024)
    for {
        n, err := tcp.Read(buffer)
        if err != nil {
            if err != io.EOF {
                return err
            }
            return nil
        }

        if err := ws.WriteMessage(websocket.BinaryMessage, buffer[:n]); err != nil {
            return err
        }
    }
}

func (p *VNCProxy) getVNCAddress(ctx context.Context, sessionID string) (string, error) {
    // Query Kubernetes for session pod
    // Return: "ss-user1-firefox-abc123.streamspace.svc.cluster.local:5900"
    // TODO: Implement
    return "", nil
}
```

---

## 📦 Container Image Migration

### Base Image Strategy

**Tier 1: Base Images** (Build First):

```dockerfile
# Dockerfile.base-ubuntu-vnc
FROM ubuntu:22.04

ARG DEBIAN_FRONTEND=noninteractive

# Install VNC stack
RUN apt-get update && apt-get install -y \
    # VNC
    tigervnc-standalone-server \
    tigervnc-common \
    # X11
    xvfb \
    x11-utils \
    x11-xserver-utils \
    # Window Manager
    xfce4 \
    xfce4-terminal \
    # Utilities
    dbus-x11 \
    wget \
    curl \
    sudo \
    supervisor \
    # Cleanup
    && rm -rf /var/lib/apt/lists/*

# Create user
RUN useradd -m -s /bin/bash -u 1000 streamspace && \
    echo "streamspace:streamspace" | chpasswd && \
    usermod -aG sudo streamspace

# VNC configuration
USER streamspace
RUN mkdir -p ~/.vnc && \
    echo "streamspace" | vncpasswd -f > ~/.vnc/passwd && \
    chmod 600 ~/.vnc/passwd

# Startup scripts
COPY --chown=streamspace:streamspace scripts/ /usr/local/bin/
RUN chmod +x /usr/local/bin/*.sh

# Environment
ENV DISPLAY=:1 \
    VNC_PORT=5900 \
    VNC_RESOLUTION=1920x1080 \
    VNC_DEPTH=24

EXPOSE 5900

USER streamspace
WORKDIR /home/streamspace

CMD ["/usr/local/bin/entrypoint.sh"]
```

**Application Images** (Tier 2):

```dockerfile
# images/firefox/Dockerfile
FROM ghcr.io/streamspace/base-ubuntu-vnc:22.04

USER root

# Install Firefox
RUN apt-get update && apt-get install -y \
    firefox \
    && rm -rf /var/lib/apt/lists/*

USER streamspace

# Auto-start Firefox
RUN echo "firefox &" >> ~/.config/autostart.sh

LABEL org.opencontainers.image.source="https://github.com/streamspace-dev/streamspace"
LABEL org.opencontainers.image.description="Firefox browser for StreamSpace"
LABEL org.opencontainers.image.licenses="MIT"
```

### Build Infrastructure

**GitHub Actions Workflow**:

```yaml
# .github/workflows/build-images.yml
name: Build Container Images

on:
  schedule:
    - cron: '0 0 * * 0'  # Weekly on Sunday
  push:
    branches: [main]
    paths:
      - 'images/**'
      - '.github/workflows/build-images.yml'
  workflow_dispatch:

env:
  REGISTRY: ghcr.io
  IMAGE_PREFIX: ghcr.io/${{ github.repository_owner }}/streamspace

jobs:
  build-base-images:
    name: Build Base Images
    runs-on: ubuntu-latest
    strategy:
      matrix:
        base: [ubuntu, alpine, debian]
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push base image
        uses: docker/build-push-action@v5
        with:
          context: ./images/base-${{ matrix.base }}-vnc
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ${{ env.IMAGE_PREFIX }}/base-${{ matrix.base }}-vnc:latest
            ${{ env.IMAGE_PREFIX }}/base-${{ matrix.base }}-vnc:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Run Trivy security scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: ${{ env.IMAGE_PREFIX }}/base-${{ matrix.base }}-vnc:latest
          format: 'sarif'
          output: 'trivy-results.sarif'

      - name: Upload Trivy results to GitHub Security
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: 'trivy-results.sarif'

  build-app-images:
    name: Build Application Images
    needs: build-base-images
    runs-on: ubuntu-latest
    strategy:
      matrix:
        app:
          - firefox
          - chromium
          - brave
          - vscode
          - gimp
          - inkscape
          - blender
          # ... more apps
    steps:
      # Similar to base images but depends on base images being built first
      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: ./images/${{ matrix.app }}
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ${{ env.IMAGE_PREFIX }}/${{ matrix.app }}:latest
            ${{ env.IMAGE_PREFIX }}/${{ matrix.app }}:${{ github.sha }}
```

---

## 🔄 Migration Process

### Phase 1: Preparation (Week 1-2)

**Tasks**:

- [ ] Research TigerVNC configuration options
- [ ] Test noVNC client with TigerVNC server
- [ ] Build proof-of-concept base image
- [ ] Test WebSocket proxy implementation
- [ ] Performance benchmarking vs KasmVNC

**Deliverables**:

- Working POC: TigerVNC + noVNC
- Performance comparison report
- Technical specification document

### Phase 2: Base Image Development (Week 3-4)

**Tasks**:

- [ ] Create `base-ubuntu-vnc:22.04`
- [ ] Create `base-alpine-vnc:3.18`
- [ ] Create `base-debian-vnc:12`
- [ ] Optimize image sizes
- [ ] Security hardening
- [ ] ARM64 testing

**Deliverables**:

- 3 base images published to ghcr.io
- Dockerfile templates
- Build documentation

### Phase 3: Application Image Migration (Week 5-8)

**Priority 1** (Week 5-6):

- [ ] Firefox, Chromium, Brave, LibreWolf (browsers)
- [ ] VS Code, Code Server (development)
- [ ] GIMP, Inkscape (design - lightweight)

**Priority 2** (Week 7):

- [ ] Blender, Krita, FreeCAD (design - heavyweight)
- [ ] LibreOffice, Calligra (productivity)
- [ ] Audacity, Kdenlive (media)

**Priority 3** (Week 8):

- [ ] Gaming emulators
- [ ] Scientific tools
- [ ] Specialized applications

**Deliverables**:

- 100+ application images
- Template YAML updates
- Testing results

### Phase 4: WebSocket Proxy Implementation (Week 9-10)

**Tasks**:

- [ ] Implement proxy in API backend
- [ ] Add authentication
- [ ] Add rate limiting
- [ ] Add connection monitoring
- [ ] Load testing

**Deliverables**:

- Production-ready WebSocket proxy
- API documentation
- Load test results

### Phase 5: Template and CRD Updates (Week 11)

**Tasks**:

- [ ] Update CRD: `kasmvnc` → `vnc` field
- [ ] Update all 22 template YAMLs
- [ ] Update database schema
- [ ] Update template generator script
- [ ] Update controller code

**Deliverables**:

- Updated CRDs
- Updated templates
- Database migration script

### Phase 6: Documentation Update (Week 12)

**Tasks**:

- [ ] Remove all KasmVNC references
- [ ] Update ARCHITECTURE.md
- [ ] Update CONTROLLER_GUIDE.md
- [ ] Update README.md
- [ ] Create migration guide for users

**Deliverables**:

- Complete documentation overhaul
- User migration guide
- Video tutorial

### Phase 7: Testing and Validation (Week 13-14)

**Tasks**:

- [ ] End-to-end testing
- [ ] Performance comparison
- [ ] Security audit
- [ ] User acceptance testing
- [ ] Load testing

**Success Criteria**:

- ✅ Zero KasmVNC references in codebase
- ✅ All images build successfully
- ✅ Performance ≥ KasmVNC baseline
- ✅ 100% template coverage
- ✅ Security scan passed

### Phase 8: Deployment (Week 15-16)

**Tasks**:

- [ ] Staged rollout plan
- [ ] Blue-green deployment
- [ ] Monitoring and alerts
- [ ] Rollback procedure
- [ ] User communication

**Deliverables**:

- Production deployment
- Monitoring dashboards
- Incident response plan

---

## 📊 Performance Targets

### Benchmarks (vs KasmVNC baseline)

| Metric | KasmVNC | Target (TigerVNC) | Measurement |
|--------|---------|-------------------|-------------|
| **Latency** |
| Input lag | 50ms | ≤ 60ms | Keyboard/mouse event timing |
| Frame rate | 30 FPS | ≥ 30 FPS | VNC frame rate at 1080p |
| **Resource Usage** |
| Memory (idle) | 150MB | ≤ 200MB | Container RSS |
| Memory (active) | 500MB | ≤ 600MB | With Firefox open |
| CPU (idle) | 2% | ≤ 3% | Container CPU % |
| CPU (active) | 25% | ≤ 30% | Scrolling/typing |
| **Network** |
| Bandwidth | 2 Mbps | ≤ 3 Mbps | 1080p @ 30 FPS |
| Compression | 80% | ≥ 75% | JPEG compression ratio |
| **Startup** |
| Container start | 5s | ≤ 7s | Pod ready time |
| VNC ready | 3s | ≤ 5s | First frame received |
| **Quality** |
| Image quality | Good | ≥ Good | Subjective assessment |
| Color depth | 24-bit | 24-bit | Full color |

---

## 🔒 Security Considerations

### Authentication Flow

```
1. User requests session via Web UI
2. UI calls API: POST /api/v1/sessions/{id}/connect
3. API validates JWT token
4. API generates one-time WebSocket token
5. API returns: wss://api.streamspace.local/vnc/{session}?token={ws_token}
6. noVNC connects with WebSocket token
7. Proxy validates token and establishes VNC connection
8. Token expires after connection established
```

### Network Security

```yaml
# NetworkPolicy: Restrict VNC port access
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: session-vnc-policy
spec:
  podSelector:
    matchLabels:
      app: streamspace-session
  policyTypes:
    - Ingress
  ingress:
    # Only allow VNC connections from API pods
    - from:
      - podSelector:
          matchLabels:
            app: streamspace-api
      ports:
      - protocol: TCP
        port: 5900
```

### TLS Encryption

- ✅ User → Ingress: TLS 1.3
- ✅ Ingress → API: TLS (mutual TLS optional)
- ⚠️ API → VNC Server: Plaintext (within cluster)
- Future: VNC-TLS support

---

## 🧪 Testing Strategy

### Unit Tests

```go
// api/internal/vnc/proxy_test.go
func TestVNCProxy_HandleConnection(t *testing.T) {
    tests := []struct {
        name      string
        sessionID string
        wantErr   bool
    }{
        {
            name:      "valid session",
            sessionID: "user1-firefox",
            wantErr:   false,
        },
        {
            name:      "invalid session",
            sessionID: "nonexistent",
            wantErr:   true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

```bash
#!/bin/bash
# tests/integration/vnc-stack-test.sh

# 1. Deploy test session
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Session
metadata:
  name: test-vnc-firefox
spec:
  user: testuser
  template: firefox-browser
  state: running
EOF

# 2. Wait for pod ready
kubectl wait --for=condition=ready pod -l session=test-vnc-firefox --timeout=60s

# 3. Test VNC connection
VNC_POD=$(kubectl get pod -l session=test-vnc-firefox -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward $VNC_POD 5900:5900 &
PF_PID=$!

# 4. Connect with VNC client
vncviewer localhost:5900 &
VNC_PID=$!

sleep 10

# 5. Cleanup
kill $VNC_PID $PF_PID
kubectl delete session test-vnc-firefox
```

### Load Tests

```javascript
// tests/load/vnc-concurrent-connections.js
import { check } from 'k6';
import ws from 'k6/ws';

export let options = {
  stages: [
    { duration: '2m', target: 10 },   // Ramp up to 10 connections
    { duration: '5m', target: 50 },   // Ramp up to 50 connections
    { duration: '10m', target: 100 }, // Sustain 100 connections
    { duration: '2m', target: 0 },    // Ramp down
  ],
};

export default function () {
  const url = 'wss://api.streamspace.local/vnc/test-session?token=...';

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', () => console.log('Connected'));
    socket.on('message', (data) => console.log('Message received'));
    socket.on('close', () => console.log('Disconnected'));

    // Simulate user activity
    socket.setTimeout(function () {
      socket.send('binary data...');
    }, 1000);
  });

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}
```

---

## 📋 Migration Checklist

### Pre-Migration

- [ ] Backup all production data
- [ ] Document current KasmVNC configuration
- [ ] Performance baseline measurements
- [ ] Communicate migration plan to users
- [ ] Set up rollback procedure

### Migration Execution

- [ ] Build and test all base images
- [ ] Build and test all application images
- [ ] Deploy WebSocket proxy
- [ ] Update CRDs (blue-green deployment)
- [ ] Update templates
- [ ] Update controller
- [ ] Update API backend
- [ ] Update Web UI

### Post-Migration

- [ ] Verify all sessions working
- [ ] Performance comparison
- [ ] User feedback collection
- [ ] Monitor for issues (7 days)
- [ ] Document lessons learned
- [ ] Remove KasmVNC dependencies
- [ ] Update all documentation

---

## 🚨 Rollback Plan

### Trigger Conditions

- Critical performance degradation (>50% slower)
- Security vulnerability discovered
- >10% of sessions failing to connect
- Data loss or corruption

### Rollback Steps

1. **Immediate** (< 15 minutes):

   ```bash
   # Revert CRD to previous version
   kubectl apply -f backups/crds/session-kasmvnc.yaml

   # Revert templates
   kubectl apply -f backups/templates/

   # Revert controller
   kubectl rollout undo deployment/streamspace-controller -n streamspace
   ```

2. **Communication** (< 30 minutes):
   - Notify users of rollback
   - Status page update
   - Incident report

3. **Root Cause Analysis** (< 24 hours):
   - Identify failure cause
   - Document findings
   - Create fix plan

---

## 📚 Resources

### TigerVNC Documentation

- Official: <https://tigervnc.org/>
- GitHub: <https://github.com/TigerVNC/tigervnc>
- Wiki: <https://github.com/TigerVNC/tigervnc/wiki>

### noVNC Documentation

- Official: <https://novnc.com/>
- GitHub: <https://github.com/novnc/noVNC>
- API Docs: <https://github.com/novnc/noVNC/blob/master/docs/API.md>

### RFB Protocol

- Specification: <https://github.com/rfbproto/rfbproto/blob/master/rfbproto.rst>
- Wikipedia: <https://en.wikipedia.org/wiki/RFB_protocol>

---

## 📞 Support

For migration questions or issues:

- **GitHub Issues**: <https://github.com/streamspace-dev/streamspace/issues>
- **Discord**: <https://discord.gg/streamspace> #vnc-migration
- **Email**: <migration-support@streamspace.io>

---

**Document Version**: 1.0
**Next Review**: Before Phase 3 starts (Q3 2025)
