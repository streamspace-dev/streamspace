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

// Reconcile is the main reconciliation loop for Templates
func (r *TemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Template
	var template streamv1alpha1.Template
	if err := r.Get(ctx, req.NamespacedName, &template); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Template resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Template")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Template", "name", template.Name)

	// Validate template configuration
	if err := r.validateTemplate(&template); err != nil {
		log.Error(err, "Template validation failed")
		template.Status.Valid = false
		template.Status.Message = err.Error()
		metrics.RecordTemplateValidation(req.Namespace, "invalid")
		if updateErr := r.Status().Update(ctx, &template); updateErr != nil {
			log.Error(updateErr, "Failed to update Template status")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, err
	}

	// Update status to valid
	template.Status.Valid = true
	template.Status.Message = "Template is valid and ready to use"
	metrics.RecordTemplateValidation(req.Namespace, "valid")
	if err := r.Status().Update(ctx, &template); err != nil {
		log.Error(err, "Failed to update Template status")
		return ctrl.Result{}, err
	}

	log.Info("Template reconciliation complete", "name", template.Name, "valid", template.Status.Valid)
	return ctrl.Result{}, nil
}

// validateTemplate performs basic validation on the template
func (r *TemplateReconciler) validateTemplate(template *streamv1alpha1.Template) error {
	// Basic validation - ensure required fields are present
	if template.Spec.BaseImage == "" {
		return errors.NewBadRequest("baseImage is required")
	}

	if template.Spec.DisplayName == "" {
		return errors.NewBadRequest("displayName is required")
	}

	// VNC validation
	if template.Spec.VNC.Enabled {
		if template.Spec.VNC.Port == 0 {
			// Set default port if not specified
			template.Spec.VNC.Port = 5900 // Standard VNC port
		}
		if template.Spec.VNC.Port < 1024 || template.Spec.VNC.Port > 65535 {
			return errors.NewBadRequest("VNC port must be between 1024 and 65535")
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *TemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streamv1alpha1.Template{}).
		Complete(r)
}
