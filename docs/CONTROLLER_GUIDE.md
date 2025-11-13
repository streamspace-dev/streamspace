# Workspace Controller Implementation Guide

This guide will help you implement the Workspace Controller using Go and Kubebuilder.

## Architecture

```
                     ┌─────────────────────────────────┐
                     │   Kubernetes API Server         │
                     └────────────┬────────────────────┘
                                  │ Watch WorkspaceSessions
                                  ↓
                     ┌─────────────────────────────────┐
                     │   Workspace Controller          │
                     │                                 │
                     │  ┌──────────────────────────┐  │
                     │  │ WorkspaceSession         │  │
                     │  │ Reconciler               │  │
                     │  │ - Create/Update/Delete   │  │
                     │  │ - Status updates         │  │
                     │  └──────────────────────────┘  │
                     │                                 │
                     │  ┌──────────────────────────┐  │
                     │  │ Hibernation Controller   │  │
                     │  │ - Monitor activity       │  │
                     │  │ - Scale to zero          │  │
                     │  └──────────────────────────┘  │
                     │                                 │
                     │  ┌──────────────────────────┐  │
                     │  │ User Manager             │  │
                     │  │ - PVC provisioning       │  │
                     │  │ - Quota enforcement      │  │
                     │  └──────────────────────────┘  │
                     └─────────────────────────────────┘
                                  │
                     ┌────────────┴────────────┐
                     ↓                         ↓
          ┌──────────────────┐    ┌──────────────────┐
          │ Workspace Pods   │    │ User PVCs        │
          │ (KasmVNC)        │    │ (NFS)            │
          └──────────────────┘    └──────────────────┘
```

## Setup Instructions

### 1. Initialize Kubebuilder Project

```bash
mkdir -p workspace-controller
cd workspace-controller

# Initialize Go module
go mod init github.com/yourusername/workspace-controller

# Initialize Kubebuilder project
kubebuilder init --domain aiinfra.io --repo github.com/yourusername/workspace-controller

# Create API for WorkspaceSession
kubebuilder create api --group workspaces --version v1alpha1 --kind WorkspaceSession

# Create API for WorkspaceTemplate
kubebuilder create api --group workspaces --version v1alpha1 --kind WorkspaceTemplate
```

### 2. Define CRD Types

Edit `api/v1alpha1/workspacesession_types.go`:

```go
package v1alpha1

import (
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkspaceSessionSpec defines the desired state
type WorkspaceSessionSpec struct {
    // User who owns this workspace
    User string `json:"user"`

    // Template to use for this workspace
    Template string `json:"template"`

    // Desired state (running, hibernated, terminated)
    State string `json:"state"`

    // Resource requirements
    Resources corev1.ResourceRequirements `json:"resources,omitempty"`

    // Enable persistent home directory
    PersistentHome bool `json:"persistentHome,omitempty"`

    // Idle timeout before hibernation
    IdleTimeout *metav1.Duration `json:"idleTimeout,omitempty"`

    // Maximum session duration
    MaxSessionDuration *metav1.Duration `json:"maxSessionDuration,omitempty"`
}

// WorkspaceSessionStatus defines the observed state
type WorkspaceSessionStatus struct {
    // Phase of the workspace
    Phase string `json:"phase,omitempty"`

    // Name of the created pod
    PodName string `json:"podName,omitempty"`

    // URL to access workspace
    URL string `json:"url,omitempty"`

    // Last activity timestamp
    LastActivity *metav1.Time `json:"lastActivity,omitempty"`

    // Current resource usage
    ResourceUsage *ResourceUsage `json:"resourceUsage,omitempty"`

    // Conditions
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ResourceUsage struct {
    Memory string `json:"memory,omitempty"`
    CPU    string `json:"cpu,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="User",type=string,JSONPath=`.spec.user`
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.state`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`

type WorkspaceSession struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   WorkspaceSessionSpec   `json:"spec,omitempty"`
    Status WorkspaceSessionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type WorkspaceSessionList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []WorkspaceSession `json:"items"`
}

func init() {
    SchemeBuilder.Register(&WorkspaceSession{}, &WorkspaceSessionList{})
}
```

### 3. Implement Reconciler Logic

Edit `controllers/workspacesession_controller.go`:

```go
package controllers

import (
    "context"
    "fmt"
    "time"

    corev1 "k8s.io/api/core/v1"
    appsv1 "k8s.io/api/apps/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"

    workspacesv1alpha1 "github.com/yourusername/workspace-controller/api/v1alpha1"
)

type WorkspaceSessionReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=workspaces.aiinfra.io,resources=workspacesessions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=workspaces.aiinfra.io,resources=workspacesessions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods;services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

func (r *WorkspaceSessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // Fetch the WorkspaceSession
    var ws workspacesv1alpha1.WorkspaceSession
    if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
        if errors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        log.Error(err, "Failed to fetch WorkspaceSession")
        return ctrl.Result{}, err
    }

    // Get the WorkspaceTemplate
    template, err := r.getTemplate(ctx, ws.Spec.Template, ws.Namespace)
    if err != nil {
        log.Error(err, "Failed to get WorkspaceTemplate")
        return ctrl.Result{}, err
    }

    // Ensure user PVC exists
    if ws.Spec.PersistentHome {
        if err := r.ensureUserPVC(ctx, &ws); err != nil {
            log.Error(err, "Failed to ensure user PVC")
            return ctrl.Result{}, err
        }
    }

    // Handle state transitions
    switch ws.Spec.State {
    case "running":
        return r.handleRunning(ctx, &ws, template)
    case "hibernated":
        return r.handleHibernated(ctx, &ws)
    case "terminated":
        return r.handleTerminated(ctx, &ws)
    }

    return ctrl.Result{}, nil
}

func (r *WorkspaceSessionReconciler) handleRunning(
    ctx context.Context,
    ws *workspacesv1alpha1.WorkspaceSession,
    template *workspacesv1alpha1.WorkspaceTemplate,
) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // Check if deployment exists
    deployment := &appsv1.Deployment{}
    deploymentName := fmt.Sprintf("ws-%s-%s", ws.Spec.User, ws.Spec.Template)
    err := r.Get(ctx, types.NamespacedName{
        Name:      deploymentName,
        Namespace: ws.Namespace,
    }, deployment)

    if errors.IsNotFound(err) {
        // Create new deployment
        deployment = r.createDeployment(ws, template)
        if err := r.Create(ctx, deployment); err != nil {
            log.Error(err, "Failed to create Deployment")
            return ctrl.Result{}, err
        }
        log.Info("Created Deployment", "name", deploymentName)
    } else if err != nil {
        return ctrl.Result{}, err
    } else {
        // Deployment exists, ensure it's scaled up
        if *deployment.Spec.Replicas == 0 {
            deployment.Spec.Replicas = int32Ptr(1)
            if err := r.Update(ctx, deployment); err != nil {
                log.Error(err, "Failed to scale up Deployment")
                return ctrl.Result{}, err
            }
            log.Info("Scaled up Deployment", "name", deploymentName)
        }
    }

    // Create Service if it doesn't exist
    if err := r.ensureService(ctx, ws, template); err != nil {
        return ctrl.Result{}, err
    }

    // Update status
    ws.Status.Phase = "Running"
    ws.Status.PodName = deploymentName
    ws.Status.URL = fmt.Sprintf("https://%s.workspaces.local", ws.Name)
    if err := r.Status().Update(ctx, ws); err != nil {
        return ctrl.Result{}, err
    }

    // Requeue after idle timeout to check for hibernation
    requeueAfter := 5 * time.Minute
    if ws.Spec.IdleTimeout != nil {
        requeueAfter = ws.Spec.IdleTimeout.Duration
    }

    return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *WorkspaceSessionReconciler) handleHibernated(
    ctx context.Context,
    ws *workspacesv1alpha1.WorkspaceSession,
) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // Scale deployment to 0
    deployment := &appsv1.Deployment{}
    deploymentName := fmt.Sprintf("ws-%s-%s", ws.Spec.User, ws.Spec.Template)
    err := r.Get(ctx, types.NamespacedName{
        Name:      deploymentName,
        Namespace: ws.Namespace,
    }, deployment)

    if err == nil && *deployment.Spec.Replicas > 0 {
        deployment.Spec.Replicas = int32Ptr(0)
        if err := r.Update(ctx, deployment); err != nil {
            log.Error(err, "Failed to scale down Deployment")
            return ctrl.Result{}, err
        }
        log.Info("Scaled down Deployment (hibernated)", "name", deploymentName)
    }

    // Update status
    ws.Status.Phase = "Hibernated"
    if err := r.Status().Update(ctx, ws); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}

func (r *WorkspaceSessionReconciler) handleTerminated(
    ctx context.Context,
    ws *workspacesv1alpha1.WorkspaceSession,
) (ctrl.Result, error) {
    // Delete all resources
    // Implementation here...
    return ctrl.Result{}, nil
}

func (r *WorkspaceSessionReconciler) createDeployment(
    ws *workspacesv1alpha1.WorkspaceSession,
    template *workspacesv1alpha1.WorkspaceTemplate,
) *appsv1.Deployment {
    name := fmt.Sprintf("ws-%s-%s", ws.Spec.User, ws.Spec.Template)
    labels := map[string]string{
        "app":       "workspace",
        "user":      ws.Spec.User,
        "template":  ws.Spec.Template,
        "workspace": ws.Name,
    }

    // Build volume mounts
    volumeMounts := []corev1.VolumeMount{}
    volumes := []corev1.Volume{}

    if ws.Spec.PersistentHome {
        volumeMounts = append(volumeMounts, corev1.VolumeMount{
            Name:      "user-home",
            MountPath: "/config",
        })
        volumes = append(volumes, corev1.Volume{
            Name: "user-home",
            VolumeSource: corev1.VolumeSource{
                PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
                    ClaimName: fmt.Sprintf("home-%s", ws.Spec.User),
                },
            },
        })
    }

    deployment := &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: ws.Namespace,
            Labels:    labels,
            OwnerReferences: []metav1.OwnerReference{
                *metav1.NewControllerRef(ws, workspacesv1alpha1.GroupVersion.WithKind("WorkspaceSession")),
            },
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: int32Ptr(1),
            Selector: &metav1.LabelSelector{
                MatchLabels: labels,
            },
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: labels,
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        {
                            Name:  "workspace",
                            Image: template.Spec.BaseImage,
                            Ports: []corev1.ContainerPort{
                                {
                                    Name:          "vnc",
                                    ContainerPort: int32(template.Spec.KasmVNC.Port),
                                    Protocol:      corev1.ProtocolTCP,
                                },
                            },
                            Env: []corev1.EnvVar{
                                {Name: "PUID", Value: "1000"},
                                {Name: "PGID", Value: "1000"},
                                {Name: "TZ", Value: "America/New_York"},
                            },
                            Resources: ws.Spec.Resources,
                            VolumeMounts: volumeMounts,
                        },
                    },
                    Volumes: volumes,
                },
            },
        },
    }

    return deployment
}

func (r *WorkspaceSessionReconciler) ensureUserPVC(
    ctx context.Context,
    ws *workspacesv1alpha1.WorkspaceSession,
) error {
    pvcName := fmt.Sprintf("home-%s", ws.Spec.User)
    pvc := &corev1.PersistentVolumeClaim{}

    err := r.Get(ctx, types.NamespacedName{
        Name:      pvcName,
        Namespace: ws.Namespace,
    }, pvc)

    if errors.IsNotFound(err) {
        // Create PVC
        pvc = &corev1.PersistentVolumeClaim{
            ObjectMeta: metav1.ObjectMeta{
                Name:      pvcName,
                Namespace: ws.Namespace,
                Labels: map[string]string{
                    "user": ws.Spec.User,
                },
            },
            Spec: corev1.PersistentVolumeClaimSpec{
                AccessModes: []corev1.PersistentVolumeAccessMode{
                    corev1.ReadWriteMany,
                },
                StorageClassName: stringPtr("nfs-client"),
                Resources: corev1.ResourceRequirements{
                    Requests: corev1.ResourceList{
                        corev1.ResourceStorage: resource.MustParse("50Gi"),
                    },
                },
            },
        }

        return r.Create(ctx, pvc)
    }

    return err
}

// Helper functions
func int32Ptr(i int32) *int32 { return &i }
func stringPtr(s string) *string { return &s }

func (r *WorkspaceSessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&workspacesv1alpha1.WorkspaceSession{}).
        Owns(&appsv1.Deployment{}).
        Owns(&corev1.Service{}).
        Complete(r)
}
```

### 4. Build and Deploy

```bash
# Generate CRDs
make manifests

# Install CRDs
make install

# Run controller locally (for testing)
make run

# Or build Docker image and deploy
make docker-build docker-push IMG=your-registry/workspace-controller:latest
make deploy IMG=your-registry/workspace-controller:latest
```

## Key Implementation Details

### Hibernation Logic

Create a separate hibernation controller that runs periodically:

```go
type HibernationReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

func (r *HibernationReconciler) Reconcile(ctx context.Context) {
    // List all running WorkspaceSessions
    wsList := &workspacesv1alpha1.WorkspaceSessionList{}
    if err := r.List(ctx, wsList); err != nil {
        return
    }

    for _, ws := range wsList.Items {
        if ws.Spec.State != "running" {
            continue
        }

        // Check idle timeout
        if ws.Status.LastActivity != nil && ws.Spec.IdleTimeout != nil {
            timeSinceActivity := time.Since(ws.Status.LastActivity.Time)
            if timeSinceActivity > ws.Spec.IdleTimeout.Duration {
                // Trigger hibernation
                ws.Spec.State = "hibernated"
                r.Update(ctx, &ws)
            }
        }
    }
}
```

### Activity Tracking

Update `LastActivity` timestamp via:
1. Sidecar container monitoring KasmVNC WebSocket connections
2. API endpoint called by frontend when user interacts with workspace
3. Pod logs parsing (less reliable)

### Metrics

Expose Prometheus metrics:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
    activeSessionsGauge = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "workspaces_active_sessions_total",
            Help: "Number of active workspace sessions",
        },
    )

    sessionStartsCounter = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "workspaces_session_starts_total",
            Help: "Total number of workspace session starts",
        },
    )
)

func init() {
    metrics.Registry.MustRegister(
        activeSessionsGauge,
        sessionStartsCounter,
    )
}
```

## Testing

```bash
# Create a test WorkspaceSession
kubectl apply -f - <<EOF
apiVersion: workspaces.aiinfra.io/v1alpha1
kind: WorkspaceSession
metadata:
  name: test-firefox
  namespace: workspaces
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

# Watch controller logs
kubectl logs -n workspaces deploy/workspace-controller -f

# Check status
kubectl get ws -n workspaces test-firefox -o yaml
```

## Next Steps

1. Implement WorkspaceTemplate reconciler
2. Add user authentication (Authentik integration)
3. Implement API backend (Phase 2)
4. Build React UI (Phase 2)
5. Add hibernation controller (Phase 3)
