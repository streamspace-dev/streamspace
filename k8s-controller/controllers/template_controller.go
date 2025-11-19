// Package controllers implements Kubernetes controllers for StreamSpace.
//
// TEMPLATE CONTROLLER
//
// The TemplateReconciler validates Template custom resources and updates their
// status to indicate whether they are ready to be used for creating sessions.
//
// WHAT ARE TEMPLATES:
//
// Templates define application configurations that users can launch as sessions.
// Each template specifies:
// - Container image to run
// - Resource requirements (CPU, memory)
// - VNC configuration for browser streaming
// - Environment variables
// - Display name and description for catalog
//
// EXAMPLES:
//
// - firefox-browser: Mozilla Firefox with VNC server
// - vscode: Visual Studio Code with web interface
// - gimp: GIMP image editor with VNC streaming
// - jupyter: Jupyter notebooks for data science
//
// Templates are created by:
// - Platform administrators (via kubectl or API)
// - Template catalog sync (from GitHub)
// - Plugin system (dynamic template generation)
//
// CONTROLLER PURPOSE:
//
// The TemplateReconciler serves two main purposes:
//
// 1. VALIDATION:
//    - Ensures required fields are present (baseImage, displayName)
//    - Validates VNC port range (1024-65535)
//    - Sets default VNC port if not specified (5900)
//    - Prevents invalid templates from being used
//
// 2. STATUS MANAGEMENT:
//    - Sets Template.Status.Valid = true/false
//    - Sets Template.Status.Message with validation errors
//    - Allows UI to show template availability
//    - Prevents SessionReconciler from using invalid templates
//
// WHY CONTROLLER VALIDATION (not admission webhooks):
//
// Admission webhooks would be better for validation, but:
// - Requires TLS certificate setup (complexity)
// - Requires webhook service deployment
// - Adds dependency for cluster startup
// - Controller validation is simpler for Phase 1
//
// FUTURE: Migrate to ValidatingWebhook in Phase 3 for:
// - Immediate feedback on template creation
// - Prevent invalid templates from being created
// - Remove validation logic from controller
//
// VALIDATION RULES:
//
// Required fields:
// - BaseImage: Container image to run (e.g., "lscr.io/linuxserver/firefox:latest")
// - DisplayName: Human-readable name for catalog (e.g., "Firefox Web Browser")
//
// VNC validation:
// - If VNC.Enabled=true:
//   * Port must be set (defaults to 5900 if empty)
//   * Port must be in range 1024-65535
//   * Port < 1024 rejected (requires root)
//   * Port > 65535 rejected (invalid)
//
// TEMPLATE LIFECYCLE:
//
// 1. Administrator creates Template:
//    kubectl apply -f firefox-template.yaml
//
// 2. TemplateReconciler validates:
//    - Check baseImage: ✓ present
//    - Check displayName: ✓ present
//    - Check VNC port: ✓ valid (5900)
//
// 3. Status updated:
//    Status.Valid = true
//    Status.Message = "Template is valid and ready to use"
//
// 4. Template appears in UI catalog:
//    - Users can browse and launch
//    - SessionReconciler can use it
//
// 5. User launches session:
//    - SessionReconciler fetches Template
//    - Uses baseImage, resources, VNC config
//    - Creates Deployment with template settings
//
// INVALID TEMPLATE HANDLING:
//
// If template fails validation:
// 1. Status.Valid = false
// 2. Status.Message = error details
// 3. Template hidden from catalog
// 4. SessionReconciler rejects session creation
// 5. Administrator must fix and re-apply
//
// EXAMPLE INVALID TEMPLATE:
//
//   apiVersion: stream.streamspace.io/v1alpha1
//   kind: Template
//   metadata:
//     name: broken-firefox
//   spec:
//     baseImage: ""  # ❌ Required field missing
//     displayName: "Firefox"
//     vnc:
//       enabled: true
//       port: 80  # ❌ Port below 1024 (requires root)
//
// Status after reconciliation:
//   status:
//     valid: false
//     message: "baseImage is required"
//
// METRICS:
//
// The controller exports Prometheus metrics:
// - template_validations_total{status="valid"}: Successful validations
// - template_validations_total{status="invalid"}: Failed validations
//
// These metrics help:
// - Identify misconfigured templates
// - Monitor template catalog health
// - Alert on validation failures
//
// RELATIONSHIP WITH SESSION CONTROLLER:
//
// TemplateReconciler validates templates
//   ↓
// Templates marked as Status.Valid=true
//   ↓
// SessionReconciler fetches valid templates
//   ↓
// Sessions created from template configuration
//
// FUTURE ENHANCEMENTS:
//
// - Image existence validation (pull image to verify)
// - Resource limit validation (prevent requesting too much)
// - Security policy validation (reject privileged containers)
// - Template versioning (multiple versions of same template)
// - A/B testing (gradual rollout of new template versions)
package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
	"github.com/streamspace/streamspace/pkg/metrics"
)

// TemplateReconciler reconciles Template custom resources.
//
// This controller validates templates and updates their status to indicate
// whether they are ready to be used for creating sessions.
//
// FIELDS:
//
// - Client: Kubernetes client for reading/writing Templates
// - Scheme: Runtime scheme for type information
//
// RBAC PERMISSIONS (defined by kubebuilder markers below):
//
// Templates: get, list, watch, create, update, patch, delete, update status
//
// WHY THESE PERMISSIONS:
//
// - Full CRUD: Allows controller to validate any template
// - Status update: Required to set Valid/Message fields
// - No dependencies: Template controller only manages templates
//
// RECONCILIATION TRIGGER:
//
// Controller reconciles when:
// - New Template created
// - Template spec updated (baseImage changed, VNC config changed)
// - Periodic resync (default: 10 hours)
//
// Note: Templates are typically created once and rarely updated,
// so reconciliation frequency is low.
//
// VALIDATION STRATEGY:
//
// Synchronous validation during reconciliation:
// - Simple, no background jobs
// - Immediate status update
// - Easy to understand and debug
//
// Asynchronous validation (not implemented):
// - Could validate image existence
// - Could test container startup
// - Adds complexity, delayed feedback
type TemplateReconciler struct {
	client.Client  // Kubernetes API client
	Scheme *runtime.Scheme  // Type information for objects
}

//+kubebuilder:rbac:groups=stream.streamspace.io,resources=templates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=templates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=templates/finalizers,verbs=update

// Reconcile is the main reconciliation loop for Template resources.
//
// This function validates template specifications and updates their status
// to indicate whether they can be used for creating sessions.
//
// RECONCILIATION LOGIC:
//
// 1. Fetch the Template resource from the Kubernetes API
// 2. Verify the Template exists (handle deletion case)
// 3. Apply default values to spec (e.g., VNC port defaults to 5900)
// 4. Persist defaults back to API server
// 5. Validate template fields (baseImage, displayName, VNC config)
// 6. Update status with validation results
// 7. Record metrics for monitoring
//
// DEFAULT VALUE HANDLING:
//
// Some fields have sensible defaults that are applied automatically:
//   - VNC.Port: Defaults to 5900 (standard VNC port)
//   - Future: Additional defaults as needed
//
// BUG FIX: Defaults are now persisted by updating the spec.
// Previously, defaults were set during validation but never saved,
// causing them to be lost on next reconciliation.
//
// VALIDATION CHECKS:
//
// Required fields:
//   - baseImage: Must be non-empty
//   - displayName: Must be non-empty
//
// VNC validation (if enabled):
//   - Port must be set (after defaults applied)
//   - Port must be in range 1024-65535
//   - Ports < 1024 require root (security risk)
//
// SECURITY CONSIDERATIONS:
//
// Template validation prevents:
//   - Sessions with missing container images
//   - Invalid port configurations
//   - Privileged ports that require root access
//   - Templates without proper metadata
//
// This is defense-in-depth - even without admission webhooks,
// templates are validated by the controller.
//
// STATUS MANAGEMENT:
//
// Template.Status.Valid indicates readiness:
//   - true: Template can be used for sessions
//   - false: Template is broken and should not be used
//
// Template.Status.Message provides details:
//   - Valid templates: "Template is valid and ready to use"
//   - Invalid templates: Error message explaining what's wrong
//
// BUG FIX: Invalid templates return nil instead of error after status update.
// Previously, returning error caused retry loops even after status was updated.
// The status already indicates the problem, no need to requeue.
//
// FUTURE ENHANCEMENTS:
//
// TODO: Add advanced validation:
//   - Image existence check (docker pull simulation)
//   - Image vulnerability scanning integration (Trivy)
//   - Resource limit reasonableness checks
//   - Security policy compliance validation
//   - Semantic version validation for image tags
//
// TODO: Add ValidatingWebhook for immediate feedback:
//   - Reject invalid templates at creation time
//   - Provide better user experience (fail fast)
//   - Reduce controller workload
func (r *TemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Template resource from the cluster
	var template streamv1alpha1.Template
	if err := r.Get(ctx, req.NamespacedName, &template); err != nil {
		if errors.IsNotFound(err) {
			// Template was deleted - this is normal during cleanup
			log.Info("Template resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Other error (API server issue, network problem, etc.)
		log.Error(err, "Failed to get Template")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Template", "name", template.Name)

	// BUG FIX: Apply defaults before validation and persist them
	// Previously validateTemplate() set defaults but never persisted them
	specChanged := false

	// Apply VNC port default if VNC is enabled but port not specified
	// Standard VNC port is 5900 (RFB protocol)
	if template.Spec.VNC.Enabled && template.Spec.VNC.Port == 0 {
		template.Spec.VNC.Port = 5900 // Standard VNC port
		specChanged = true
	}

	// Persist defaults back to the API server if spec was modified
	// This ensures defaults survive across reconciliations
	if specChanged {
		if err := r.Update(ctx, &template); err != nil {
			log.Error(err, "Failed to update Template with defaults")
			return ctrl.Result{}, err
		}
		log.Info("Applied default values to Template", "name", template.Name)

		// Re-fetch the template to get the updated ResourceVersion
		// This is required because the status update needs the latest version
		if err := r.Get(ctx, req.NamespacedName, &template); err != nil {
			log.Error(err, "Failed to re-fetch Template after spec update")
			return ctrl.Result{}, err
		}
	}

	// Validate template configuration
	// Validation is now read-only (doesn't mutate the template)
	// All mutations happen above in the defaults section
	if err := r.validateTemplate(&template); err != nil {
		// Validation failed - mark template as invalid
		log.Error(err, "Template validation failed")
		template.Status.Valid = false
		template.Status.Message = err.Error()
		metrics.RecordTemplateValidation(req.Namespace, "invalid")

		// Update status to reflect validation failure
		// This prevents the template from being used for sessions
		if updateErr := r.Status().Update(ctx, &template); updateErr != nil {
			log.Error(updateErr, "Failed to update Template status")
			return ctrl.Result{}, updateErr
		}

		// BUG FIX: Return nil instead of err after successful status update
		// Returning err here causes retry loop even though status was updated correctly
		// The status.Valid=false already indicates the problem to users
		return ctrl.Result{}, nil
	}

	// Validation passed - mark template as valid
	template.Status.Valid = true
	template.Status.Message = "Template is valid and ready to use"
	metrics.RecordTemplateValidation(req.Namespace, "valid")

	// Update status to reflect successful validation
	if err := r.Status().Update(ctx, &template); err != nil {
		log.Error(err, "Failed to update Template status")
		return ctrl.Result{}, err
	}

	log.Info("Template reconciliation complete", "name", template.Name, "valid", template.Status.Valid)
	return ctrl.Result{}, nil
}

// validateTemplate performs validation on template fields.
//
// BUG FIX: This function is now read-only and does not mutate the template.
// Defaults are applied in Reconcile() before validation is called.
//
// VALIDATION RULES:
//
// 1. Required fields:
//    - baseImage: Container image must be specified
//    - displayName: Human-readable name must be provided
//
// 2. VNC validation (if VNC.Enabled is true):
//    - Port must be set (should be set by defaults in Reconcile())
//    - Port must be in range 1024-65535
//
// PORT RANGE RATIONALE:
//
// - Ports < 1024 are privileged (require root)
// - Running as root is a security risk
// - Ports > 65535 are invalid
// - Range 1024-65535 allows non-root containers
//
// COMMON VALIDATION ERRORS:
//
// - "baseImage is required": Template created without image
// - "displayName is required": Template missing catalog name
// - "VNC port is required when VNC is enabled": Port is 0 after defaults
// - "VNC port must be between 1024 and 65535": Invalid port number
//
// ERROR HANDLING:
//
// Validation errors are returned as BadRequest errors:
//   - Error message is user-friendly
//   - Error is stored in Template.Status.Message
//   - Template is marked as invalid (Status.Valid = false)
//
// FUTURE ENHANCEMENTS:
//
// TODO: Add validation for:
//   - Image tag format (semantic versioning)
//   - Environment variable name format
//   - Resource request/limit reasonableness
//   - Security context settings
func (r *TemplateReconciler) validateTemplate(template *streamv1alpha1.Template) error {
	// Validate required field: baseImage
	// Without a container image, sessions cannot be created
	if template.Spec.BaseImage == "" {
		return errors.NewBadRequest("baseImage is required")
	}

	// Validate required field: displayName
	// Without a display name, template cannot appear in catalog
	if template.Spec.DisplayName == "" {
		return errors.NewBadRequest("displayName is required")
	}

	// VNC validation (only if VNC streaming is enabled)
	if template.Spec.VNC.Enabled {
		// Port should already be set to default (5900) by Reconcile()
		// If it's still 0, that's an error condition
		if template.Spec.VNC.Port == 0 {
			return errors.NewBadRequest("VNC port is required when VNC is enabled")
		}

		// Validate port is in valid non-privileged range
		// Ports < 1024 require root (security risk)
		// Ports > 65535 are invalid
		if template.Spec.VNC.Port < 1024 || template.Spec.VNC.Port > 65535 {
			return errors.NewBadRequest("VNC port must be between 1024 and 65535")
		}
	}

	return nil
}

// SetupWithManager registers the TemplateReconciler with the controller manager.
//
// This function configures:
//   - Primary resource to watch (Template)
//   - Event filtering
//
// WATCH CONFIGURATION:
//
// For(&streamv1alpha1.Template{}):
//   - Reconcile when Template is created, updated, or deleted
//   - No owned resources (Templates don't own other resources)
//
// RECONCILIATION TRIGGER:
//
// Controller reconciles when:
//   - New Template created
//   - Template spec updated
//   - Template deleted
//   - Periodic resync (default: 10 hours)
//
// OWNERSHIP:
//
// Templates don't own other resources:
//   - No Owns() declarations needed
//   - Sessions reference Templates but don't have owner references
//   - Deleting a Template doesn't delete Sessions (intentional)
//
// FUTURE ENHANCEMENTS:
//
// TODO: Add event filtering predicates:
//   - Only reconcile on spec changes (ignore status updates)
//   - Filter out metadata-only changes
//   - Reduce unnecessary reconciliation loops
func (r *TemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streamv1alpha1.Template{}).
		Complete(r)
}
