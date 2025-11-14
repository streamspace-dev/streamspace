# StreamSpace - Next Steps

**Status**: Phase 1 (Foundation) - Ready to Begin Implementation
**Last Updated**: 2025-11-14

---

## ✅ What's Been Completed

### Strategic Planning (100% Complete)

1. **ROADMAP.md** - Comprehensive 6-phase development plan
   - 18-month timeline to v1.0
   - Phase 3 focus on VNC independence
   - Feature comparison with commercial platforms
   - Container image build strategy

2. **CLAUDE.md** - AI assistant guide with independence strategy
   - Strategic vision section
   - VNC abstraction patterns
   - Critical rules for development
   - Migration checklist

3. **README.md** - Updated branding
   - Removed Kasm comparisons
   - Emphasized 100% open source
   - Updated acknowledgments

4. **docs/VNC_MIGRATION.md** - Detailed migration guide
   - Architecture diagrams
   - Component implementation details
   - 8-phase migration process
   - Testing and rollback strategies

5. **Repository Structure** - Complete foundation
   - CRD definitions (Session, Template)
   - Kubernetes manifests
   - Helm chart structure
   - Monitoring configuration
   - 22 pre-built templates

---

## 🎯 Immediate Next Steps (Phase 1)

### Week 1-2: Development Environment Setup

**Goal**: Set up local development environment for controller implementation.

#### Tasks

1. **Install Development Tools**:
   ```bash
   # Go 1.21+
   go version  # Verify Go installation

   # Kubebuilder 3.x
   curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
   chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

   # Docker
   docker --version

   # kubectl with cluster access (k3s recommended)
   kubectl version

   # Make
   make --version
   ```

2. **Set Up Local Kubernetes Cluster**:
   ```bash
   # Option 1: k3s (recommended for ARM64)
   curl -sfL https://get.k3s.io | sh -

   # Option 2: kind (for quick testing)
   kind create cluster --name streamspace-dev

   # Option 3: minikube
   minikube start --cpus=4 --memory=8192
   ```

3. **Clone and Initialize Project**:
   ```bash
   cd /home/user/streamspace

   # Verify current state
   git status
   git log --oneline -5

   # Create controller directory
   mkdir -p controller
   cd controller
   ```

4. **Install NFS Provisioner** (for PVC testing):
   ```bash
   # Install NFS server provisioner
   helm repo add nfs-ganesha-server-and-external-provisioner \
     https://kubernetes-sigs.github.io/nfs-ganesha-server-and-external-provisioner/

   helm install nfs-provisioner \
     nfs-ganesha-server-and-external-provisioner/nfs-server-provisioner
   ```

**Deliverables**:
- [ ] Go 1.21+ installed
- [ ] Kubebuilder installed
- [ ] Local k3s/kind cluster running
- [ ] kubectl configured
- [ ] NFS provisioner deployed

---

### Week 3-4: Initialize Kubebuilder Project

**Goal**: Initialize the controller project structure with Kubebuilder.

#### Tasks

1. **Initialize Go Module**:
   ```bash
   cd /home/user/streamspace/controller

   go mod init github.com/yourusername/streamspace
   ```

2. **Initialize Kubebuilder Project**:
   ```bash
   # IMPORTANT: Use streamspace.io domain (not stream.space)
   # This is for the Go package, CRDs still use stream.space
   kubebuilder init \
     --domain streamspace.io \
     --repo github.com/yourusername/streamspace \
     --project-name streamspace \
     --owner "StreamSpace Community"
   ```

3. **Create API Resources**:
   ```bash
   # Create Session API
   kubebuilder create api \
     --group stream \
     --version v1alpha1 \
     --kind Session \
     --resource \
     --controller

   # Create Template API
   kubebuilder create api \
     --group stream \
     --version v1alpha1 \
     --kind Template \
     --resource \
     --controller
   ```

4. **Verify Generated Structure**:
   ```bash
   tree -L 3
   # Expected structure:
   # controller/
   # ├── api/
   # │   └── v1alpha1/
   # │       ├── session_types.go
   # │       └── template_types.go
   # ├── controllers/
   # │   ├── session_controller.go
   # │   └── template_controller.go
   # ├── config/
   # │   ├── crd/
   # │   ├── rbac/
   # │   └── manager/
   # ├── Dockerfile
   # ├── Makefile
   # └── main.go
   ```

5. **Initial Build Test**:
   ```bash
   make manifests generate
   make build
   ```

**Deliverables**:
- [ ] Kubebuilder project initialized
- [ ] Session and Template APIs created
- [ ] Initial build successful
- [ ] Project structure verified

---

### Week 5-6: Define CRD Types

**Goal**: Implement the Session and Template CRD type definitions.

#### Tasks

1. **Edit Session Types** (`api/v1alpha1/session_types.go`):
   ```go
   // CRITICAL: Use generic VNC terminology, not Kasm-specific
   type SessionSpec struct {
       User               string                       `json:"user"`
       Template           string                       `json:"template"`
       State              string                       `json:"state"` // running, hibernated, terminated
       Resources          corev1.ResourceRequirements  `json:"resources,omitempty"`
       PersistentHome     bool                         `json:"persistentHome,omitempty"`
       IdleTimeout        *metav1.Duration             `json:"idleTimeout,omitempty"`
       MaxSessionDuration *metav1.Duration             `json:"maxSessionDuration,omitempty"`
   }

   type SessionStatus struct {
       Phase          string              `json:"phase,omitempty"`
       PodName        string              `json:"podName,omitempty"`
       URL            string              `json:"url,omitempty"`
       LastActivity   *metav1.Time        `json:"lastActivity,omitempty"`
       ResourceUsage  *ResourceUsage      `json:"resourceUsage,omitempty"`
       Conditions     []metav1.Condition  `json:"conditions,omitempty"`
   }
   ```

2. **Edit Template Types** (`api/v1alpha1/template_types.go`):
   ```go
   // CRITICAL: Use vnc not kasmvnc
   type TemplateSpec struct {
       DisplayName       string                      `json:"displayName"`
       Description       string                      `json:"description"`
       Category          string                      `json:"category"`
       Icon              string                      `json:"icon,omitempty"`
       BaseImage         string                      `json:"baseImage"`
       DefaultResources  corev1.ResourceRequirements `json:"defaultResources,omitempty"`
       Ports             []corev1.ContainerPort      `json:"ports,omitempty"`
       Env               []corev1.EnvVar             `json:"env,omitempty"`
       VolumeMounts      []corev1.VolumeMount        `json:"volumeMounts,omitempty"`
       VNC               VNCConfig                   `json:"vnc,omitempty"` // Generic VNC config
       Capabilities      []string                    `json:"capabilities,omitempty"`
       Tags              []string                    `json:"tags,omitempty"`
   }

   type VNCConfig struct {
       Enabled    bool   `json:"enabled"`
       Port       int    `json:"port"`
       Protocol   string `json:"protocol,omitempty"`  // "rfb", "websocket"
       Encryption bool   `json:"encryption,omitempty"`
   }
   ```

3. **Add Kubebuilder Markers**:
   ```go
   //+kubebuilder:object:root=true
   //+kubebuilder:subresource:status
   //+kubebuilder:resource:shortName=ss
   //+kubebuilder:printcolumn:name="User",type=string,JSONPath=`.spec.user`
   //+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.state`
   //+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
   //+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
   //+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

   type Session struct {
       metav1.TypeMeta   `json:",inline"`
       metav1.ObjectMeta `json:"metadata,omitempty"`
       Spec              SessionSpec   `json:"spec,omitempty"`
       Status            SessionStatus `json:"status,omitempty"`
   }
   ```

4. **Generate CRDs and DeepCopy**:
   ```bash
   make manifests generate

   # Verify CRDs generated
   ls -la config/crd/bases/
   # Should see: stream.streamspace.io_sessions.yaml
   #             stream.streamspace.io_templates.yaml
   ```

5. **Install CRDs to Cluster**:
   ```bash
   make install

   # Verify
   kubectl get crds | grep stream.streamspace.io
   ```

**Deliverables**:
- [ ] Session types defined
- [ ] Template types defined
- [ ] VNC config abstracted (not Kasm-specific)
- [ ] CRDs generated
- [ ] CRDs installed to cluster

---

### Week 7-10: Implement Session Controller

**Goal**: Build the core Session reconciliation logic.

#### Tasks

1. **Implement Session Reconciler** (`controllers/session_controller.go`):

   Follow the pattern in `docs/CONTROLLER_GUIDE.md`, but with key differences:

   - Use generic `vnc` field, not `kasmvnc`
   - VNC port: configurable (default 5900, support 3000 for LinuxServer.io)
   - Prepare for future TigerVNC migration

   Key functions to implement:
   ```go
   func (r *SessionReconciler) Reconcile(ctx, req) (ctrl.Result, error)
   func (r *SessionReconciler) handleRunning(ctx, session, template)
   func (r *SessionReconciler) handleHibernated(ctx, session)
   func (r *SessionReconciler) handleTerminated(ctx, session)
   func (r *SessionReconciler) createDeployment(session, template)
   func (r *SessionReconciler) ensureUserPVC(ctx, session)
   func (r *SessionReconciler) ensureService(ctx, session, template)
   func (r *SessionReconciler) ensureIngress(ctx, session)
   ```

2. **Add RBAC Markers**:
   ```go
   //+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions,verbs=get;list;watch;create;update;patch;delete
   //+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions/status,verbs=get;update;patch
   //+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
   //+kubebuilder:rbac:groups="",resources=pods;services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
   //+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
   ```

3. **Add Prometheus Metrics**:
   ```go
   import (
       "github.com/prometheus/client_golang/prometheus"
       "sigs.k8s.io/controller-runtime/pkg/metrics"
   )

   var (
       activeSessionsGauge = prometheus.NewGaugeVec(
           prometheus.GaugeOpts{
               Name: "streamspace_active_sessions_total",
               Help: "Number of active sessions",
           },
           []string{"user", "template"},
       )

       sessionStartsCounter = prometheus.NewCounterVec(
           prometheus.CounterOpts{
               Name: "streamspace_session_starts_total",
               Help: "Total number of session starts",
           },
           []string{"template"},
       )
   )

   func init() {
       metrics.Registry.MustRegister(
           activeSessionsGauge,
           sessionStartsCounter,
       )
   }
   ```

4. **Test Locally**:
   ```bash
   # Run controller locally
   make run

   # In another terminal, create a test session
   kubectl apply -f config/samples/stream_v1alpha1_session.yaml

   # Watch controller logs for reconciliation
   # Verify Deployment, Service, PVC creation
   ```

**Deliverables**:
- [ ] Session reconciler implemented
- [ ] State transitions working (running/hibernated/terminated)
- [ ] User PVC provisioning
- [ ] Deployment/Service/Ingress creation
- [ ] Prometheus metrics exposed
- [ ] Local testing successful

---

### Week 11-12: Implement Template Controller

**Goal**: Build Template reconciliation logic (simpler than Session).

#### Tasks

1. **Implement Template Reconciler**:
   ```go
   // Template controller is mostly read-only
   // Main job: Validate template definitions
   func (r *TemplateReconciler) Reconcile(ctx, req) (ctrl.Result, error) {
       // Fetch template
       // Validate spec
       // Update status
       // No resource creation needed
   }
   ```

2. **Add Validation Webhooks** (optional for Phase 1):
   ```go
   //+kubebuilder:webhook:path=/validate-stream-streamspace-io-v1alpha1-template,mutating=false,failurePolicy=fail,groups=stream.streamspace.io,resources=templates,verbs=create;update,versions=v1alpha1,name=vtemplate.kb.io
   ```

3. **Test Template CRUD**:
   ```bash
   # Create template
   kubectl apply -f manifests/templates/browsers/firefox.yaml

   # Verify template
   kubectl get templates -n streamspace
   kubectl describe template firefox-browser -n streamspace
   ```

**Deliverables**:
- [ ] Template reconciler implemented
- [ ] Template validation
- [ ] Template CRUD testing

---

### Week 13-14: Integration Testing

**Goal**: End-to-end testing of controller with real workloads.

#### Tasks

1. **Deploy All Templates**:
   ```bash
   kubectl apply -f manifests/templates/
   kubectl get templates -n streamspace
   ```

2. **Create Test Sessions**:
   ```bash
   # Test each template category
   kubectl apply -f - <<EOF
   apiVersion: stream.streamspace.io/v1alpha1
   kind: Session
   metadata:
     name: test-firefox
     namespace: streamspace
   spec:
     user: testuser
     template: firefox-browser
     state: running
     resources:
       requests:
         memory: 2Gi
         cpu: 1000m
     persistentHome: true
     idleTimeout: 30m
   EOF
   ```

3. **Verify Session Lifecycle**:
   ```bash
   # Watch session creation
   kubectl get sessions -n streamspace -w

   # Check resources created
   kubectl get deployments,services,pvcs -n streamspace

   # Test hibernation
   kubectl patch session test-firefox -n streamspace \
     --type merge -p '{"spec":{"state":"hibernated"}}'

   # Verify deployment scaled to 0
   kubectl get deployment -n streamspace

   # Test wake
   kubectl patch session test-firefox -n streamspace \
     --type merge -p '{"spec":{"state":"running"}}'
   ```

4. **Load Testing** (10 concurrent sessions):
   ```bash
   for i in {1..10}; do
     kubectl apply -f - <<EOF
   apiVersion: stream.streamspace.io/v1alpha1
   kind: Session
   metadata:
     name: test-session-$i
     namespace: streamspace
   spec:
     user: user$i
     template: firefox-browser
     state: running
   EOF
   done

   # Monitor controller performance
   kubectl top pod -n streamspace
   ```

**Deliverables**:
- [ ] All templates deployed
- [ ] Session create/update/delete working
- [ ] Hibernation working
- [ ] 10+ concurrent sessions tested
- [ ] No memory leaks or crashes

---

### Week 15-16: Build and Deploy Controller

**Goal**: Build container image and deploy to cluster.

#### Tasks

1. **Build Docker Image**:
   ```bash
   # Build for your architecture
   make docker-build IMG=ghcr.io/yourusername/streamspace-controller:v0.1.0

   # Multi-arch build
   docker buildx build \
     --platform linux/amd64,linux/arm64 \
     -t ghcr.io/yourusername/streamspace-controller:v0.1.0 \
     --push .
   ```

2. **Push to Registry**:
   ```bash
   make docker-push IMG=ghcr.io/yourusername/streamspace-controller:v0.1.0
   ```

3. **Deploy to Cluster**:
   ```bash
   make deploy IMG=ghcr.io/yourusername/streamspace-controller:v0.1.0

   # Verify deployment
   kubectl get deployment -n streamspace-system
   kubectl logs -n streamspace-system deploy/streamspace-controller-manager -f
   ```

4. **Update Helm Chart**:
   ```yaml
   # chart/values.yaml
   controller:
     image:
       repository: ghcr.io/yourusername/streamspace-controller
       tag: v0.1.0
       pullPolicy: IfNotPresent
   ```

**Deliverables**:
- [ ] Controller image built
- [ ] Image pushed to ghcr.io
- [ ] Controller deployed to cluster
- [ ] Controller running stably
- [ ] Helm chart updated

---

## 🎯 Phase 1 Completion Criteria

**Definition of Done**:

- [ ] ✅ Go controller implemented with Kubebuilder
- [ ] ✅ Session CRD with full lifecycle management
- [ ] ✅ Template CRD with validation
- [ ] ✅ User PVC provisioning working
- [ ] ✅ Hibernation (scale to 0) working
- [ ] ✅ Prometheus metrics exposed
- [ ] ✅ 10+ concurrent sessions tested
- [ ] ✅ Controller runs for 7+ days without crashes
- [ ] ✅ Container image published
- [ ] ✅ Helm chart deployment working
- [ ] ✅ Documentation updated

**Key Metrics**:
- Session creation time: < 30 seconds
- Hibernation time: < 5 seconds
- Wake time: < 20 seconds
- Memory usage: < 100MB per controller
- CPU usage: < 100m per controller

---

## 📚 Reference Documentation

**Primary References**:
1. `ROADMAP.md` - Overall project roadmap
2. `docs/CONTROLLER_GUIDE.md` - Controller implementation guide
3. `docs/ARCHITECTURE.md` - System architecture
4. `CLAUDE.md` - AI assistant guide with strategic vision
5. `CONTRIBUTING.md` - Contribution guidelines

**Kubebuilder Resources**:
- Book: https://book.kubebuilder.io/
- Quick Start: https://book.kubebuilder.io/quick-start.html
- CRD Tutorial: https://book.kubebuilder.io/cronjob-tutorial/cronjob-tutorial.html

**Controller-Runtime**:
- Docs: https://pkg.go.dev/sigs.k8s.io/controller-runtime
- Client: https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client
- Reconciler: https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile

---

## 🚨 Important Reminders

### ⚠️ CRITICAL: VNC Independence Strategy

**ALWAYS remember**:
1. ❌ **NEVER** introduce new Kasm/KasmVNC dependencies
2. ❌ **NEVER** use `kasmvnc` field names - use `vnc` instead
3. ✅ **ALWAYS** use VNC-agnostic patterns
4. ✅ **ALWAYS** prepare for TigerVNC migration (Phase 3)

**Example - BAD**:
```go
// ❌ DON'T DO THIS
type TemplateSpec struct {
    KasmVNC KasmVNCConfig `json:"kasmvnc"`
}
```

**Example - GOOD**:
```go
// ✅ DO THIS
type TemplateSpec struct {
    VNC VNCConfig `json:"vnc"` // Generic VNC config
}

type VNCConfig struct {
    Enabled  bool   `json:"enabled"`
    Port     int    `json:"port"`     // 5900 (standard) or 3000 (LinuxServer.io)
    Protocol string `json:"protocol"` // "rfb", "websocket"
}
```

### 📖 Before Writing Code

Always check:
1. **ROADMAP.md** - Ensure you're in Phase 1 scope
2. **CLAUDE.md** - Review "Strategic Vision" and "Development Guidelines"
3. **docs/CONTROLLER_GUIDE.md** - Follow controller patterns
4. **This file** - Stay on track with weekly milestones

---

## 🤝 Getting Help

**Stuck? Check these resources**:

1. **Documentation** first (docs/ directory)
2. **GitHub Issues** for known problems
3. **Discord** for community support (#phase-1-controller channel)
4. **CLAUDE.md** for AI assistant guidance

**When asking for help**:
- Specify which week/task you're on
- Include error messages and logs
- Share your environment details (OS, Go version, k8s version)

---

## 📅 Timeline Summary

| Week | Focus | Deliverable |
|------|-------|-------------|
| 1-2 | Dev Environment | Tools installed, cluster running |
| 3-4 | Kubebuilder Init | Project structure created |
| 5-6 | Define CRDs | Types implemented, CRDs installed |
| 7-10 | Session Controller | Core reconciliation logic |
| 11-12 | Template Controller | Template validation |
| 13-14 | Integration Testing | E2E tests passing |
| 15-16 | Build & Deploy | Controller in production |

**Estimated Duration**: 16 weeks (4 months)
**Target Completion**: Q2 2025

---

## ✅ Quick Start Checklist

If you're starting today, do these first:

```bash
# 1. Verify tools
[ ] go version          # Should be 1.21+
[ ] docker --version
[ ] kubectl version
[ ] make --version

# 2. Install Kubebuilder
[ ] curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
[ ] chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
[ ] kubebuilder version

# 3. Set up cluster
[ ] k3s or kind or minikube running
[ ] kubectl get nodes  # Should show Ready

# 4. Initialize project
[ ] cd /home/user/streamspace/controller
[ ] go mod init github.com/yourusername/streamspace
[ ] kubebuilder init --domain streamspace.io

# 5. Read documentation
[ ] Read ROADMAP.md
[ ] Read CLAUDE.md "Strategic Vision" section
[ ] Read docs/CONTROLLER_GUIDE.md

# 6. Start coding!
```

---

**Ready to build the future of open source container streaming!** 🚀

**Next Review**: Weekly (update this file with progress)
