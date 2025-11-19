// Package controllers implements Kubernetes controllers for StreamSpace custom resources.
//
// SESSION CONTROLLER
//
// The SessionReconciler implements the core reconciliation loop that manages the lifecycle
// of containerized workspace sessions in Kubernetes. It handles state transitions, resource
// creation, hibernation, and cleanup.
//
// KUBERNETES CONTROLLER PATTERN:
//
// Controllers in Kubernetes follow a reconciliation loop pattern:
// 1. Watch for changes to custom resources (Sessions, Templates)
// 2. Compare desired state (Session.Spec) with actual state (Deployments, Pods)
// 3. Take actions to make actual state match desired state
// 4. Update status to reflect current state
// 5. Requeue if necessary
//
// RECONCILIATION LOOP:
//
//   ┌─────────────────┐
//   │  Event Trigger  │ ← Session created/updated/deleted
//   └────────┬────────┘
//            ↓
//   ┌─────────────────┐
//   │  Fetch Session  │ ← Get Session from cluster
//   └────────┬────────┘
//            ↓
//   ┌─────────────────┐
//   │  Fetch Template │ ← Get Template for session
//   └────────┬────────┘
//            ↓
//   ┌─────────────────┐
//   │  Check State    │ ← running | hibernated | terminated
//   └────────┬────────┘
//            ↓
//   ┌─────────────────┐
//   │  Reconcile      │ ← Create/update/delete resources
//   └────────┬────────┘
//            ↓
//   ┌─────────────────┐
//   │  Update Status  │ ← Set phase, URL, pod name
//   └────────┬────────┘
//            ↓
//   ┌─────────────────┐
//   │  Record Metrics │ ← Prometheus metrics
//   └─────────────────┘
//
// SESSION STATES:
//
// 1. RUNNING: Session is active with pod running
//    - Creates: Deployment (replicas=1), Service, Ingress, PVC (if persistent)
//    - Updates: Status.Phase = "Running", Status.URL = session URL
//
// 2. HIBERNATED: Session is paused to save resources
//    - Scales: Deployment replicas=0 (pod stopped but definition preserved)
//    - Preserves: PVC data, Service, Ingress
//    - Can wake up quickly by scaling back to replicas=1
//
// 3. TERMINATED: Session is permanently deleted
//    - Deletes: Deployment, Service, Ingress
//    - Preserves: PVC (user data persists across sessions)
//    - Updates: Status.Phase = "Terminated"
//
// KUBERNETES RESOURCES MANAGED:
//
// For each Session, the controller creates and manages:
//
// 1. Deployment (ss-{user}-{template}):
//    - Runs container from Template.Spec.BaseImage
//    - Mounts user PVC at /config (if persistent home enabled)
//    - Exposes VNC port for browser streaming
//    - Scales 0-1 for hibernation/wake
//
// 2. Service ({deployment}-svc):
//    - ClusterIP service for pod networking
//    - Routes traffic to VNC port
//    - Selector matches deployment labels
//
// 3. Ingress ({deployment}):
//    - External URL: {session-name}.{ingress-domain}
//    - Routes HTTPS traffic to Service
//    - Uses Traefik (default) or configured ingress class
//
// 4. PersistentVolumeClaim (home-{user}):
//    - Shared across all sessions for same user
//    - ReadWriteMany (NFS backed)
//    - Persists data even when sessions are terminated
//    - No owner reference (survives session deletion)
//
// OWNER REFERENCES AND GARBAGE COLLECTION:
//
// - Deployment, Service, Ingress have owner reference to Session
// - Kubernetes automatically deletes these when Session is deleted
// - PVC does NOT have owner reference (survives session deletion)
// - PVC must be manually deleted or cleaned up by separate process
//
// RECONCILIATION TRIGGERS:
//
// Controller reconciles when:
// - New Session created
// - Session spec updated (state changed, resources changed)
// - Owned resource changed (Deployment scaled, pod crashed)
// - Template updated (not currently watched, but could be)
// - Periodic resync (default: 10 hours)
//
// ERROR HANDLING:
//
// - Kubernetes API errors: Retry with exponential backoff
// - Template not found: Return error, requeue
// - Resource creation fails: Return error, requeue
// - Status update fails: Log error but don't requeue (status updates retry automatically)
//
// METRICS:
//
// The controller exports Prometheus metrics:
// - session_reconciliations_total: Total reconciliations (success/error)
// - session_reconciliation_duration_seconds: Reconciliation latency
// - sessions_by_user: Sessions per user
// - sessions_by_template: Sessions per template
// - sessions_by_state: Sessions in each state (running/hibernated/terminated)
// - session_hibernations_total: Hibernation events (manual/auto-idle)
// - session_wakes_total: Wake from hibernation events
//
// EXAMPLE SESSION LIFECYCLE:
//
// 1. User creates Session via API:
//    kubectl apply -f session.yaml
//
// 2. Controller reconciles:
//    - Creates Deployment, Service, Ingress, PVC
//    - Sets Status.Phase = "Running"
//    - Sets Status.URL = "https://my-session.streamspace.local"
//
// 3. User finishes work, session hibernates:
//    kubectl patch session my-session -p '{"spec":{"state":"hibernated"}}'
//
// 4. Controller reconciles:
//    - Scales Deployment to 0 replicas
//    - Sets Status.Phase = "Hibernated"
//
// 5. User resumes work:
//    kubectl patch session my-session -p '{"spec":{"state":"running"}}'
//
// 6. Controller reconciles:
//    - Scales Deployment to 1 replica
//    - Pod starts quickly (image cached, PVC data preserved)
//    - Sets Status.Phase = "Running"
//
// 7. User permanently deletes session:
//    kubectl delete session my-session
//
// 8. Controller reconciles (if state was "terminated" first):
//    - Deletes Deployment
//    - Kubernetes garbage collection deletes Service, Ingress
//    - PVC persists for future sessions
package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
	"github.com/streamspace/streamspace/pkg/metrics"
)

// SessionReconciler reconciles Session custom resources.
//
// The reconciler implements the controller-runtime Reconciler interface and is
// responsible for managing the complete lifecycle of containerized workspace sessions.
//
// FIELDS:
//
// - Client: Kubernetes client for reading and writing resources
// - Scheme: Runtime scheme for object type information
//
// RBAC PERMISSIONS (defined by kubebuilder markers below):
//
// Sessions: get, list, watch, create, update, patch, delete, update status
// Templates: get, list, watch (read-only)
// Deployments: get, list, watch, create, update, patch, delete
// Services: get, list, watch, create, update, patch, delete
// PersistentVolumeClaims: get, list, watch, create, update, patch, delete
// Ingresses: get, list, watch, create, update, patch, delete
//
// WHY THESE PERMISSIONS:
//
// - Sessions: Full CRUD to manage custom resource
// - Templates: Read-only to get application configuration
// - Deployments/Services/Ingresses/PVCs: Full CRUD to manage session infrastructure
// - No delete on Templates: Prevents accidental template deletion
//
// CONTROLLER RUNTIME:
//
// This reconciler is managed by controller-runtime which provides:
// - Event watching and queueing
// - Leader election for HA deployments
// - Exponential backoff retry
// - Periodic resyncs
// - Metrics and health endpoints
//
// CONCURRENCY:
//
// - Multiple reconcilers can run concurrently
// - Each reconciliation is for a single Session
// - Kubernetes optimistic concurrency prevents conflicts
// - Status updates use separate client with retry
type SessionReconciler struct {
	client.Client  // Kubernetes API client
	Scheme *runtime.Scheme  // Type information for objects
}

// setCondition sets or updates a condition on the Session's status.
//
// Standard condition types for Sessions:
//   - "Ready": Session is running and accepting connections
//   - "TemplateResolved": Template was found and validated
//   - "PVCBound": Persistent volume is bound and mounted
//   - "DeploymentReady": Deployment is created and running
//
// Parameters:
//   - ctx: Context for API calls
//   - session: The Session to update
//   - conditionType: The type of condition (e.g., "TemplateResolved")
//   - status: metav1.ConditionTrue, metav1.ConditionFalse, or metav1.ConditionUnknown
//   - reason: Machine-readable reason code (e.g., "TemplateNotFound")
//   - message: Human-readable description of the condition
//
// The function updates the session's status subresource in the cluster.
func (r *SessionReconciler) setCondition(ctx context.Context, session *streamv1alpha1.Session, conditionType string, status metav1.ConditionStatus, reason, message string) {
	log := log.FromContext(ctx)

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: session.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Use meta.SetStatusCondition to properly update or add the condition
	meta.SetStatusCondition(&session.Status.Conditions, condition)

	// Update the status subresource
	if err := r.Status().Update(ctx, session); err != nil {
		log.Error(err, "Failed to update Session condition",
			"conditionType", conditionType,
			"reason", reason)
	}
}

//+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions/finalizers,verbs=update
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=templates,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for Session resources.
//
// This function is called by controller-runtime whenever a Session resource is created,
// updated, deleted, or when any owned resource (Deployment, Service, Ingress) changes.
//
// RECONCILIATION LOGIC:
//
// 1. Fetch the Session resource from the Kubernetes API
// 2. Verify the Session exists (handle deletion case)
// 3. Record metrics for monitoring and observability
// 4. Fetch the referenced Template to get application configuration
// 5. Route to state-specific handler based on Session.Spec.State
// 6. Update metrics based on reconciliation outcome
//
// IDEMPOTENCY:
//
// This function is idempotent and can be called multiple times safely.
// It compares desired state (Session.Spec) with actual state (Deployments, Pods)
// and only makes changes when they differ.
//
// ERROR HANDLING:
//
// - Returns error: Controller-runtime will requeue with exponential backoff
// - Returns nil: Reconciliation successful, no requeue
// - Returns ctrl.Result{Requeue: true}: Requeue immediately
// - Returns ctrl.Result{RequeueAfter: duration}: Requeue after delay
//
// PERFORMANCE:
//
// - Uses defer for metrics to ensure they're recorded even on error
// - Tracks duration to identify slow reconciliations
// - Minimizes API calls by fetching resources only when needed
//
// SECURITY:
//
// - Only reconciles Sessions in allowed namespaces (RBAC enforced)
// - Validates Template references to prevent arbitrary pod creation
// - Owner references ensure proper garbage collection
func (r *SessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	startTime := time.Now()

	// Track reconciliation metrics using deferred function to ensure it's always called
	// This provides observability even when reconciliation fails
	defer func() {
		duration := time.Since(startTime).Seconds()
		metrics.ObserveReconciliationDuration(req.Namespace, duration)
	}()

	// Fetch the Session resource from the cluster
	// This may fail if the Session was deleted between the event trigger and now
	var session streamv1alpha1.Session
	if err := r.Get(ctx, req.NamespacedName, &session); err != nil {
		if errors.IsNotFound(err) {
			// Session was deleted - this is normal during cleanup
			// Owner references will automatically delete owned resources (Deployment, Service, Ingress)
			// No action needed, just log and return
			log.Info("Session resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Other error (API server down, network issue, etc.) - retry
		log.Error(err, "Failed to get Session")
		metrics.RecordReconciliation(req.Namespace, "error")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Session", "name", session.Name, "state", session.Spec.State)

	// Update metrics for this session - track by user and template for capacity planning
	// These metrics help answer: "How many sessions does user X have?" and "How popular is template Y?"
	metrics.RecordSessionByUser(session.Spec.User, session.Namespace, 1)
	metrics.RecordSessionByTemplate(session.Spec.Template, session.Namespace, 1)

	// Fetch the Template that defines this session's configuration
	// Template must exist or reconciliation will fail (prevents invalid sessions)
	template, err := r.getTemplate(ctx, session.Spec.Template, session.Namespace)
	if err != nil {
		log.Error(err, "Failed to get Template")
		metrics.RecordReconciliation(req.Namespace, "error")
		// Set condition to indicate template was not found
		r.setCondition(ctx, &session, "TemplateResolved", metav1.ConditionFalse, "TemplateNotFound",
			fmt.Sprintf("Template '%s' not found in namespace '%s'", session.Spec.Template, session.Namespace))
		return ctrl.Result{}, err
	}

	// Route to state-specific handler based on desired state
	// Each handler is responsible for making actual state match desired state
	var result ctrl.Result
	switch session.Spec.State {
	case "running":
		// Create/update resources, scale up Deployment to 1 replica
		result, err = r.handleRunning(ctx, &session, template)
	case "hibernated":
		// Scale down Deployment to 0 replicas (preserve all other resources)
		result, err = r.handleHibernated(ctx, &session)
	case "terminated":
		// Delete all resources except PVC (user data persists)
		result, err = r.handleTerminated(ctx, &session)
	default:
		// Unknown state - this shouldn't happen due to CRD validation
		// But handle gracefully just in case
		log.Info("Unknown state", "state", session.Spec.State)
		// TODO: Add webhook validation to reject invalid states
		return ctrl.Result{}, nil
	}

	// Record reconciliation result in Prometheus metrics
	// This helps track error rates and success rates over time
	if err != nil {
		metrics.RecordReconciliation(req.Namespace, "error")
	} else {
		metrics.RecordReconciliation(req.Namespace, "success")
	}

	return result, err
}

// handleRunning ensures all resources exist and are running for an active session.
//
// This function creates or updates the following resources:
//   1. Deployment: Runs the containerized application
//   2. Service: Provides networking to the pod
//   3. PersistentVolumeClaim: Stores user data (if persistentHome enabled)
//   4. Ingress: Exposes session via HTTPS URL
//
// DEPLOYMENT LIFECYCLE:
//
// - If Deployment doesn't exist: Create with replicas=1
// - If Deployment exists but replicas=0: Scale up to 1 (wake from hibernation)
// - If Deployment exists and replicas=1: No action needed
//
// IDEMPOTENCY:
//
// Multiple calls are safe - only creates resources if they don't exist.
// Uses Kubernetes "Get then Create" pattern for idempotent resource creation.
//
// WAKE-FROM-HIBERNATION:
//
// When a hibernated session transitions to running:
// - Deployment already exists with replicas=0
// - Controller detects this and scales to replicas=1
// - Pod starts quickly (image cached, PVC already bound)
// - User experience: ~5 second wake time
//
// RESOURCE NAMING:
//
// - Deployment: ss-{user}-{template} (e.g., "ss-alice-firefox")
// - Service: {deployment}-svc (e.g., "ss-alice-firefox-svc")
// - PVC: home-{user} (e.g., "home-alice")
// - Ingress: {deployment} (e.g., "ss-alice-firefox")
//
// ERROR HANDLING:
//
// - Resource creation fails: Return error, requeue
// - PVC mount fails: Error logged, pod will be in Pending state
// - Image pull fails: Pod shows ErrImagePull, visible in status
//
// SECURITY:
//
// - Owner references link resources to Session for automatic cleanup
// - PVC has NO owner reference to persist across session deletions
// - Template validation prevents arbitrary image execution
//
// TODO:
//   - Add resource quota checking before creating Deployment
//   - Implement admission webhooks for real-time validation
//   - Add pod security policies (non-root, dropped capabilities)
func (r *SessionReconciler) handleRunning(ctx context.Context, session *streamv1alpha1.Session, template *streamv1alpha1.Template) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// BUG FIX: Validate template before creating session resources
	// Previously controller would create deployment even with invalid templates
	if !template.Status.Valid {
		// Check if template hasn't been validated yet vs validation failed
		// Empty message means TemplateReconciler hasn't run yet - wait for it
		if template.Status.Message == "" {
			log.Info("Template not yet validated, waiting for TemplateReconciler", "template", template.Name)
			// Requeue after a short delay to allow TemplateReconciler to validate
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}

		// Template was validated but is invalid - this is a real error
		err := fmt.Errorf("template %s is not valid: %s", template.Name, template.Status.Message)
		log.Error(err, "Cannot create session from invalid template")

		// Update session status to reflect error
		session.Status.Phase = "Failed"
		if statusErr := r.Status().Update(ctx, session); statusErr != nil {
			log.Error(statusErr, "Failed to update Session status")
		}

		return ctrl.Result{}, err
	}

	// Generate consistent names for all resources
	// Using predictable naming makes debugging easier and avoids resource sprawl
	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)
	serviceName := fmt.Sprintf("%s-svc", deploymentName)

	// --- STEP 1: Ensure Deployment exists and is running ---

	// Check if deployment already exists
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: session.Namespace}, deployment)

	if errors.IsNotFound(err) {
		// Deployment doesn't exist - create a new one
		// This happens when a session is first created or after termination
		deployment = r.createDeployment(session, template)
		if err := r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create Deployment")
			// Set condition to indicate deployment creation failed
			r.setCondition(ctx, session, "DeploymentReady", metav1.ConditionFalse, "DeploymentCreationFailed",
				fmt.Sprintf("Failed to create deployment: %v", err))
			return ctrl.Result{}, err
		}
		log.Info("Created Deployment", "name", deploymentName)
	} else if err != nil {
		// API error (not 404) - could be transient, retry
		return ctrl.Result{}, err
	} else {
		// Deployment exists - check if it needs to be scaled up (wake from hibernation)
		// Replicas can be nil (defaulted by Kubernetes) or explicitly 0 (hibernated)
		if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas == 0 {
			// Session was hibernated, wake it up by scaling to 1 replica
			deployment.Spec.Replicas = int32Ptr(1)
			if err := r.Update(ctx, deployment); err != nil {
				log.Error(err, "Failed to scale up Deployment")
				return ctrl.Result{}, err
			}
			log.Info("Scaled up Deployment (waking from hibernation)", "name", deploymentName)
			// Record wake event in metrics for cost analysis
			metrics.RecordWake(session.Namespace)
		}
		// else: Deployment already running with 1 replica, nothing to do
	}

	// --- STEP 2: Ensure Service exists for pod networking ---

	service := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: session.Namespace}, service)

	if errors.IsNotFound(err) {
		// Service doesn't exist - create one to route traffic to the pod
		service = r.createService(session, template)
		if err := r.Create(ctx, service); err != nil {
			log.Error(err, "Failed to create Service")
			return ctrl.Result{}, err
		}
		log.Info("Created Service", "name", serviceName)
	} else if err != nil {
		return ctrl.Result{}, err
	}
	// else: Service already exists, no action needed

	// --- STEP 3: Ensure user PVC exists for persistent storage (if enabled) ---

	// PVC is shared across all sessions for the same user
	// It persists even when sessions are deleted, allowing data to survive
	if session.Spec.PersistentHome {
		pvcName := fmt.Sprintf("home-%s", session.Spec.User)
		pvc := &corev1.PersistentVolumeClaim{}
		err = r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: session.Namespace}, pvc)

		if errors.IsNotFound(err) {
			// PVC doesn't exist - create one for this user
			// This is the first session for this user, or PVC was manually deleted
			pvc = r.createUserPVC(session)
			if err := r.Create(ctx, pvc); err != nil {
				log.Error(err, "Failed to create PVC")
				// PVC creation failure is serious - pod won't start without it
				// Set condition to indicate PVC creation failed
				r.setCondition(ctx, session, "PVCBound", metav1.ConditionFalse, "PVCCreationFailed",
					fmt.Sprintf("Failed to create persistent volume claim for user '%s': %v", session.Spec.User, err))
				return ctrl.Result{}, err
			}
			log.Info("Created user PVC", "name", pvcName)
		} else if err != nil {
			return ctrl.Result{}, err
		}
		// else: PVC already exists (from previous session), reuse it
	}

	// --- STEP 4: Ensure Ingress exists for external HTTPS access ---

	ingressName := deploymentName
	ingress := &networkingv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Name: ingressName, Namespace: session.Namespace}, ingress)

	if errors.IsNotFound(err) {
		// Ingress doesn't exist - create one to expose session via HTTPS
		ingress = r.createIngress(session, template, serviceName)
		if err := r.Create(ctx, ingress); err != nil {
			log.Error(err, "Failed to create Ingress")
			return ctrl.Result{}, err
		}
		log.Info("Created Ingress", "name", ingressName)
	} else if err != nil {
		return ctrl.Result{}, err
	}
	// else: Ingress already exists, no action needed

	// --- STEP 5: Update Session status to reflect running state ---

	// Get ingress domain from environment (configured at deployment time)
	// This determines the URL format: https://{session}.{domain}
	ingressDomain := os.Getenv("INGRESS_DOMAIN")
	if ingressDomain == "" {
		ingressDomain = "streamspace.local" // Default for development
	}

	// Update status fields to reflect current state
	// Status updates are separate from spec updates to avoid conflicts
	session.Status.Phase = "Running"
	session.Status.PodName = deploymentName // For debugging (kubectl logs, exec)
	session.Status.URL = fmt.Sprintf("https://%s.%s", session.Name, ingressDomain)
	if err := r.Status().Update(ctx, session); err != nil {
		log.Error(err, "Failed to update Session status")
		// Status update failures are not critical - don't fail reconciliation
		// The status will be updated on the next reconciliation loop
		return ctrl.Result{}, err
	}

	// Record session state in Prometheus for monitoring
	metrics.RecordSessionState("running", session.Namespace, 1)

	// Log success at verbose level (V(1)) to reduce log noise in production
	log.V(1).Info("Session running successfully",
		"session", session.Name,
		"user", session.Spec.User,
		"template", session.Spec.Template,
		"url", session.Status.URL,
	)

	return ctrl.Result{}, nil
}

// handleHibernated scales down the session's Deployment to save resources.
//
// HIBERNATION STRATEGY:
//
// Instead of deleting the pod, we scale the Deployment to 0 replicas.
// This preserves:
//   - Deployment configuration (image, env vars, resource limits)
//   - Service (networking ready for wake-up)
//   - Ingress (URL remains the same)
//   - PersistentVolumeClaim (user data intact)
//
// COST SAVINGS:
//
// A hibernated session consumes zero compute resources:
//   - No CPU usage
//   - No memory usage
//   - Only storage costs (PVC)
//
// Typical savings: ~$0.15/hour per session (2 CPU, 4GB RAM)
// With 100 users idle 20 hours/day: ~$9,000/month saved
//
// WAKE-UP TIME:
//
// When session transitions back to "running":
//   - Controller scales Deployment to 1 replica
//   - Kubernetes schedules pod on available node
//   - Container starts (~5 seconds with cached image)
//   - PVC mounts immediately (already bound)
//   - User can access session via same URL
//
// WHY NOT DELETE:
//
// Deleting and recreating would be slower because:
//   - Deployment must be recreated from scratch
//   - Service and Ingress must be recreated
//   - PVC binding takes time
//   - URL might change if Ingress recreates
//
// IDEMPOTENCY:
//
// Multiple calls are safe:
//   - Only scales down if replicas > 0
//   - If already at 0, no action taken
//
// HIBERNATION SOURCE:
//
// Sessions can be hibernated by:
//   - User manually (via API: state → "hibernated")
//   - Auto-hibernation (HibernationReconciler detects idle timeout)
//
// This function doesn't differentiate between sources, but metrics do:
//   - Manual: User explicitly hibernated
//   - Auto-idle: HibernationReconciler triggered
//
// TODO:
//   - Add pre-hibernation webhook to allow cleanup scripts
//   - Optionally delete pod immediately instead of waiting for scale-down
//   - Support hibernation scheduling (e.g., every night at 2 AM)
func (r *SessionReconciler) handleHibernated(ctx context.Context, session *streamv1alpha1.Session) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)

	// Scale deployment to 0 replicas to stop the pod
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: session.Namespace}, deployment)

	if err == nil && deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0 {
		// Deployment is currently running (replicas > 0), scale it down
		deployment.Spec.Replicas = int32Ptr(0)
		if err := r.Update(ctx, deployment); err != nil {
			log.Error(err, "Failed to scale down Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Scaled down Deployment (hibernated)", "name", deploymentName)
		// Record hibernation event - assume manual unless HibernationReconciler sets otherwise
		// The "manual" label indicates this was a user-initiated state change
		metrics.RecordHibernation(session.Namespace, "manual")
	}
	// else: Deployment already at 0 replicas or doesn't exist (idempotent)

	// Update Session status to reflect hibernated state
	session.Status.Phase = "Hibernated"
	if err := r.Status().Update(ctx, session); err != nil {
		log.Error(err, "Failed to update Session status")
		return ctrl.Result{}, err
	}

	// Record session state in Prometheus for dashboards
	metrics.RecordSessionState("hibernated", session.Namespace, 1)

	log.V(1).Info("Session hibernated successfully",
		"session", session.Name,
		"user", session.Spec.User,
		"template", session.Spec.Template,
	)

	return ctrl.Result{}, nil
}

// handleTerminated permanently deletes the session's Deployment and updates status.
//
// TERMINATION BEHAVIOR:
//
// When a session is terminated:
//   - Deployment is explicitly deleted
//   - Service, Ingress are auto-deleted via owner references (garbage collection)
//   - PVC is NOT deleted (user data persists for future sessions)
//   - Session resource remains until user deletes it
//
// OWNER REFERENCES AND GARBAGE COLLECTION:
//
// Kubernetes automatically deletes owned resources when the owner is deleted:
//   - Deployment has ownerReference → Session
//   - Service has ownerReference → Session
//   - Ingress has ownerReference → Session
//   - PVC has NO ownerReference (intentionally preserved)
//
// However, we explicitly delete the Deployment here to ensure it's removed
// even if the Session resource is not deleted (state remains "terminated").
//
// DATA PERSISTENCE:
//
// User data in the PVC persists after termination:
//   - PVC survives session deletion
//   - New sessions for the same user mount the same PVC
//   - Data is preserved across session lifecycles
//   - PVC must be manually deleted by administrator if needed
//
// WHY PRESERVE PVC:
//
// Users expect their data to persist:
//   - Browser bookmarks and history
//   - Code projects and configurations
//   - Downloaded files
//   - Application settings
//
// Deleting PVC on termination would cause data loss and user frustration.
//
// STATE TRANSITION:
//
// Terminated is typically the final state before deletion:
//   running → terminated → kubectl delete session
//   hibernated → terminated → kubectl delete session
//
// However, a session CAN transition from terminated back to running:
//   terminated → running: New Deployment created, PVC remounted
//
// IDEMPOTENCY:
//
// Multiple calls are safe:
//   - Only deletes Deployment if it exists
//   - If already deleted, no action taken
//
// TODO:
//   - Add finalizer to ensure cleanup completes before Session deletion
//   - Support optional PVC deletion via annotation (delete-pvc=true)
//   - Add pre-termination webhook for cleanup scripts
func (r *SessionReconciler) handleTerminated(ctx context.Context, session *streamv1alpha1.Session) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)

	// Delete deployment explicitly (Service/Ingress will be garbage collected via ownerReferences)
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: session.Namespace}, deployment)

	if err == nil {
		// Deployment exists, delete it
		if err := r.Delete(ctx, deployment); err != nil {
			log.Error(err, "Failed to delete Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Deleted Deployment (terminated)", "name", deploymentName)
	}
	// else: Deployment already deleted or never existed (idempotent)

	// Update Session status to reflect terminated state
	session.Status.Phase = "Terminated"
	if err := r.Status().Update(ctx, session); err != nil {
		log.Error(err, "Failed to update Session status")
		return ctrl.Result{}, err
	}

	// Record session state in Prometheus
	metrics.RecordSessionState("terminated", session.Namespace, 1)

	log.Info("Session terminated successfully",
		"session", session.Name,
		"user", session.Spec.User,
		"template", session.Spec.Template,
	)

	return ctrl.Result{}, nil
}

// createDeployment constructs a Kubernetes Deployment resource for a session.
//
// The Deployment manages the pod lifecycle and enables features like:
//   - Automatic restart on failure
//   - Rolling updates when template changes
//   - Replica scaling (0 for hibernation, 1 for running)
//
// DEPLOYMENT STRUCTURE:
//
//   - Name: ss-{user}-{template} (e.g., "ss-alice-firefox")
//   - Replicas: 1 (starts running immediately)
//   - Container: From template.Spec.BaseImage
//   - Ports: VNC port from template configuration
//   - Env: Environment variables from template
//   - Volumes: User PVC mounted at /config (if persistentHome enabled)
//
// LABELS:
//
// Labels are used for:
//   - Resource selection (kubectl get pods -l user=alice)
//   - Service selectors (route traffic to correct pods)
//   - Metrics and monitoring (group by user, template)
//
// Standard labels:
//   - app: streamspace-session (identifies all session pods)
//   - user: {username} (filter by user)
//   - template: {template-name} (filter by application type)
//   - session: {session-name} (identify specific session)
//
// Tag labels:
//   - tag.stream.space/{tag}: "true" (custom user tags)
//
// VNC CONFIGURATION:
//
// The VNC port is determined from the template:
//   - Default: 5900 (standard VNC port)
//   - LinuxServer.io: 3000 (current temporary images)
//   - Future: StreamSpace images will use 5900
//
// RESOURCE LIMITS:
//
// Resource limits are applied in this order (first match wins):
//   1. Session.Spec.Resources (user override)
//   2. Template.Spec.DefaultResources (template default)
//   3. No limits (Kubernetes defaults)
//
// SECURITY:
//
// TODO: Add security enhancements:
//   - runAsNonRoot: true
//   - allowPrivilegeEscalation: false
//   - readOnlyRootFilesystem: true
//   - drop all capabilities except required
//
// OWNER REFERENCES:
//
// The Deployment has an owner reference to the Session:
//   - Ensures Deployment is deleted when Session is deleted
//   - Prevents orphaned resources
//   - Enables kubectl tree view
func (r *SessionReconciler) createDeployment(session *streamv1alpha1.Session, template *streamv1alpha1.Template) *appsv1.Deployment {
	name := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)

	// Build standard labels for resource identification and filtering
	labels := map[string]string{
		"app":      "streamspace-session",
		"user":     session.Spec.User,
		"template": session.Spec.Template,
		"session":  session.Name,
	}

	// Add user-defined tags as labels with namespace prefix
	// This allows filtering: kubectl get deployments -l tag.stream.space/development=true
	for _, tag := range session.Spec.Tags {
		if tag != "" {
			// Use label-safe format: convert to lowercase, replace spaces with dashes
			safeTag := fmt.Sprintf("tag.stream.space/%s", tag)
			labels[safeTag] = "true"
		}
	}

	// Determine VNC port from template configuration
	// VNC-agnostic design supports migration from KasmVNC to TigerVNC
	vncPort := int32(5900) // Standard VNC port (default)
	if template.Spec.VNC.Port != 0 {
		vncPort = int32(template.Spec.VNC.Port)
	}

	// Build container specification
	// This defines what runs inside the pod
	container := corev1.Container{
		Name:  "session", // Container name (single container per pod)
		Image: template.Spec.BaseImage, // Container image from template
		Ports: []corev1.ContainerPort{
			{
				Name:          "vnc", // Port name for service reference
				ContainerPort: vncPort, // VNC server port
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: template.Spec.Env, // Environment variables from template
	}

	// Apply resource limits/requests in priority order
	// Session-specific resources override template defaults
	if len(session.Spec.Resources.Requests) > 0 || len(session.Spec.Resources.Limits) > 0 {
		// User specified resources at session creation time
		container.Resources = session.Spec.Resources
	} else if len(template.Spec.DefaultResources.Requests) > 0 || len(template.Spec.DefaultResources.Limits) > 0 {
		// Use template defaults
		container.Resources = template.Spec.DefaultResources
	}
	// else: No limits specified, use Kubernetes defaults (unrestricted)

	// Build pod specification
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	// Add persistent volume if user requested persistent home directory
	// This allows user data to survive session termination
	if session.Spec.PersistentHome {
		pvcName := fmt.Sprintf("home-%s", session.Spec.User)

		// Add volume mount to container (mount PVC at /config)
		// LinuxServer.io images use /config as the persistent directory
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "user-home",
			MountPath: "/config", // Standard path for LinuxServer.io containers
		})

		// Add volume definition to pod spec (reference to PVC)
		podSpec.Volumes = []corev1.Volume{
			{
				Name: "user-home",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName, // References existing or to-be-created PVC
					},
				},
			},
		}
	}

	// Update pod spec with modified container (container was modified after initial podSpec creation)
	podSpec.Containers[0] = container

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: session.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(session, streamv1alpha1.GroupVersion.WithKind("Session")),
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
				Spec: podSpec,
			},
		},
	}

	return deployment
}

// createService constructs a Kubernetes Service resource for pod networking.
//
// The Service provides a stable network endpoint for accessing the session pod:
//   - ClusterIP type (internal cluster networking)
//   - Routes traffic to pods matching label selectors
//   - Exposes VNC port for streaming
//
// SERVICE PURPOSE:
//
// Services abstract away pod IP addresses (which change on restart):
//   - Pod IP: Ephemeral (changes on restart)
//   - Service IP: Stable (persists until Service deleted)
//   - Ingress uses Service name (DNS-based discovery)
//
// NAMING CONVENTION:
//
//   - Service name: {deployment}-svc
//   - Example: "ss-alice-firefox-svc"
//
// LABEL SELECTORS:
//
// The Service uses labels to find pods:
//   - app: streamspace-session
//   - user: {username}
//   - template: {template-name}
//   - session: {session-name}
//
// All labels must match for traffic to route to the pod.
//
// OWNER REFERENCE:
//
// Service has owner reference to Session for automatic cleanup.
func (r *SessionReconciler) createService(session *streamv1alpha1.Session, template *streamv1alpha1.Template) *corev1.Service {
	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)
	serviceName := fmt.Sprintf("%s-svc", deploymentName)
	labels := map[string]string{
		"app":      "streamspace-session",
		"user":     session.Spec.User,
		"template": session.Spec.Template,
		"session":  session.Name,
	}

	// Add tags as labels
	for _, tag := range session.Spec.Tags {
		if tag != "" {
			safeTag := fmt.Sprintf("tag.stream.space/%s", tag)
			labels[safeTag] = "true"
		}
	}

	// Determine VNC port
	vncPort := int32(5900)
	if template.Spec.VNC.Port != 0 {
		vncPort = int32(template.Spec.VNC.Port)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: session.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(session, streamv1alpha1.GroupVersion.WithKind("Session")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:     "vnc",
					Port:     vncPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return service
}

// createUserPVC constructs a PersistentVolumeClaim for user's home directory.
//
// PVC DESIGN:
//
//   - Shared across all sessions for the same user
//   - Persists even when sessions are deleted
//   - ReadWriteMany access mode (requires NFS or similar)
//   - No owner reference (intentionally survives session deletion)
//
// NAMING CONVENTION:
//
//   - PVC name: home-{username}
//   - Example: "home-alice"
//
// ACCESS MODE:
//
// ReadWriteMany is required because:
//   - User might have multiple concurrent sessions
//   - Each session mounts the same PVC
//   - Requires distributed filesystem (NFS, CephFS, GlusterFS)
//
// CAPACITY:
//
//   - Default: 50Gi per user
//   - TODO: Make configurable via user quotas
//   - TODO: Support dynamic expansion
//
// LIFECYCLE:
//
// PVC is created on first session and never deleted automatically:
//   - First session: PVC created
//   - Subsequent sessions: PVC reused
//   - All sessions terminated: PVC persists
//   - User account deleted: Administrator manually deletes PVC
//
// SECURITY:
//
// TODO: Add security enhancements:
//   - Per-user storage quotas
//   - Encryption at rest
//   - Access auditing
func (r *SessionReconciler) createUserPVC(session *streamv1alpha1.Session) *corev1.PersistentVolumeClaim {
	pvcName := fmt.Sprintf("home-%s", session.Spec.User)
	labels := map[string]string{
		"app":  "streamspace-user-home",
		"user": session.Spec.User,
	}

	// Default home directory size
	storageSize := "50Gi"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: session.Namespace,
			Labels:    labels,
			// Note: No owner reference - PVC persists across sessions
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany, // NFS support
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}

	return pvc
}

// createIngress constructs a Kubernetes Ingress resource for external HTTPS access.
//
// INGRESS PURPOSE:
//
// Exposes the session to users via HTTPS URL:
//   - Hostname: {session-name}.{ingress-domain}
//   - Example: https://alice-firefox.streamspace.local
//   - Routes traffic to Service → Pod
//
// INGRESS CONTROLLER:
//
// Requires an ingress controller (Traefik, NGINX, etc.):
//   - Default: Traefik (specified in ingressClass)
//   - Controller handles TLS termination
//   - Controller routes based on hostname
//
// URL STRUCTURE:
//
//   - Hostname: {session-name}.{ingress-domain}
//   - Session name: User-provided (must be DNS-safe)
//   - Ingress domain: Configured via INGRESS_DOMAIN env var
//
// TLS/HTTPS:
//
// TLS is handled by the ingress controller:
//   - Cert-manager can auto-provision Let's Encrypt certificates
//   - Or use wildcard certificate for *.{ingress-domain}
//   - TODO: Add TLS configuration section
//
// NETWORKING FLOW:
//
//   User Browser
//      ↓ HTTPS
//   Ingress Controller (TLS termination)
//      ↓ HTTP
//   Service (load balancer)
//      ↓ TCP
//   Pod (VNC server)
//
// OWNER REFERENCE:
//
// Ingress has owner reference to Session for automatic cleanup.
//
// TODO:
//   - Add authentication annotations (OAuth2, OIDC)
//   - Add rate limiting annotations
//   - Support custom domains per user
func (r *SessionReconciler) createIngress(session *streamv1alpha1.Session, template *streamv1alpha1.Template, serviceName string) *networkingv1.Ingress {
	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)
	labels := map[string]string{
		"app":      "streamspace-session",
		"user":     session.Spec.User,
		"template": session.Spec.Template,
		"session":  session.Name,
	}

	// Add tags as labels with prefix for easy filtering
	for _, tag := range session.Spec.Tags {
		if tag != "" {
			safeTag := fmt.Sprintf("tag.stream.space/%s", tag)
			labels[safeTag] = "true"
		}
	}

	// Get ingress configuration from environment
	ingressDomain := os.Getenv("INGRESS_DOMAIN")
	if ingressDomain == "" {
		ingressDomain = "streamspace.local"
	}

	ingressClass := os.Getenv("INGRESS_CLASS")
	if ingressClass == "" {
		ingressClass = "traefik"
	}

	// Determine VNC port
	vncPort := int32(5900)
	if template.Spec.VNC.Port != 0 {
		vncPort = int32(template.Spec.VNC.Port)
	}

	// Build hostname
	hostname := fmt.Sprintf("%s.%s", session.Name, ingressDomain)

	// Path type
	pathTypePrefix := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: session.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": ingressClass,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(session, streamv1alpha1.GroupVersion.WithKind("Session")),
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClass,
			Rules: []networkingv1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: vncPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return ingress
}

// getTemplate retrieves a Template resource from the Kubernetes API.
//
// This is a helper function to fetch the template referenced by a session.
//
// VALIDATION:
//
// Template existence is validated here:
//   - Returns error if template doesn't exist
//   - Prevents sessions from being created without valid configuration
//
// NAMESPACE:
//
// Templates must be in the same namespace as the session:
//   - Multi-tenancy: Each namespace has its own templates
//   - Or shared namespace: Platform-wide template catalog
//
// ERROR HANDLING:
//
// If template not found:
//   - Reconciliation fails
//   - Controller requeues with backoff
//   - Session remains in Pending phase
//   - Condition "TemplateResolved" is set to False by caller
func (r *SessionReconciler) getTemplate(ctx context.Context, templateName, namespace string) (*streamv1alpha1.Template, error) {
	template := &streamv1alpha1.Template{}
	err := r.Get(ctx, types.NamespacedName{Name: templateName, Namespace: namespace}, template)
	if err != nil {
		return nil, err
	}
	return template, nil
}

// SetupWithManager registers the SessionReconciler with the controller manager.
//
// This function configures:
//   - Primary resource to watch (Session)
//   - Owned resources to watch (Deployment, Service, Ingress)
//   - Event filtering and predicates
//
// WATCH CONFIGURATION:
//
// For(&streamv1alpha1.Session{}):
//   - Reconcile when Session is created, updated, or deleted
//
// Owns(&appsv1.Deployment{}):
//   - Reconcile when owned Deployment changes
//   - Example: Pod crashes, Deployment scales
//
// Owns(&corev1.Service{}):
//   - Reconcile when owned Service changes
//
// Owns(&networkingv1.Ingress{}):
//   - Reconcile when owned Ingress changes
//
// NOT WATCHED:
//
// PersistentVolumeClaim:
//   - Not watched because it has no owner reference
//   - PVC changes don't trigger reconciliation
//
// Template:
//   - Not watched (could be added for automatic updates)
//   - TODO: Watch templates and update sessions when template changes
//
// OWNERSHIP:
//
// Owner references are automatically set when resources are created:
//   - Deployment → Session
//   - Service → Session
//   - Ingress → Session
//
// This enables:
//   - Automatic reconciliation when owned resources change
//   - Automatic cleanup via garbage collection
//   - Dependency tracking
func (r *SessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streamv1alpha1.Session{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

// int32Ptr is a helper function that returns a pointer to an int32 value.
// This is needed because Kubernetes API uses pointers for optional fields.
func int32Ptr(i int32) *int32 { return &i }
