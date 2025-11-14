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

// TemplateReconciler reconciles a Template object
type TemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
		template.Status.Phase = "Invalid"
		template.Status.Message = err.Error()
		metrics.RecordTemplateValidation(req.Namespace, "invalid")
		if updateErr := r.Status().Update(ctx, &template); updateErr != nil {
			log.Error(updateErr, "Failed to update Template status")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, err
	}

	// Update status to Ready
	template.Status.Phase = "Ready"
	template.Status.Message = "Template is valid and ready to use"
	metrics.RecordTemplateValidation(req.Namespace, "valid")
	if err := r.Status().Update(ctx, &template); err != nil {
		log.Error(err, "Failed to update Template status")
		return ctrl.Result{}, err
	}

	log.Info("Template reconciliation complete", "name", template.Name, "phase", template.Status.Phase)
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
