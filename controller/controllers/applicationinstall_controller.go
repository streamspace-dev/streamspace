// Package controllers contains Kubernetes controllers for StreamSpace CRDs.
//
// This file implements the ApplicationInstallReconciler which watches for
// ApplicationInstall resources and creates corresponding Template CRDs.
package controllers

import (
	"context"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamspacev1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
)

// ApplicationInstallReconciler reconciles ApplicationInstall objects.
//
// When an ApplicationInstall is created, this controller:
//   1. Parses the manifest field to extract template configuration
//   2. Creates a corresponding Template CRD
//   3. Updates the ApplicationInstall status to Ready or Failed
//
// This provides automatic retry on failure and clear status reporting.
type ApplicationInstallReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=stream.space,resources=applicationinstalls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stream.space,resources=applicationinstalls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=stream.space,resources=applicationinstalls/finalizers,verbs=update
// +kubebuilder:rbac:groups=stream.space,resources=templates,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles ApplicationInstall reconciliation.
//
// It creates a Template CRD from the manifest in the ApplicationInstall spec.
// If the template already exists, it updates the status accordingly.
func (r *ApplicationInstallReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ApplicationInstall
	var appInstall streamspacev1alpha1.ApplicationInstall
	if err := r.Get(ctx, req.NamespacedName, &appInstall); err != nil {
		if errors.IsNotFound(err) {
			// ApplicationInstall was deleted, nothing to do
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ApplicationInstall")
		return ctrl.Result{}, err
	}

	// Skip if already processed
	if appInstall.Status.Phase == "Ready" || appInstall.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	// Update status to Creating
	if err := r.updateStatus(ctx, &appInstall, "Creating", "Processing manifest..."); err != nil {
		logger.Error(err, "Failed to update status to Creating")
		return ctrl.Result{}, err
	}

	// Parse the manifest
	templateSpec, err := r.parseManifest(appInstall.Spec.Manifest)
	if err != nil {
		logger.Error(err, "Failed to parse manifest")
		if updateErr := r.updateStatus(ctx, &appInstall, "Failed", fmt.Sprintf("Failed to parse manifest: %v", err)); updateErr != nil {
			logger.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{}, nil // Don't retry, manifest is invalid
	}

	// Create the Template CRD
	template := &streamspacev1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appInstall.Spec.TemplateName,
			Namespace: appInstall.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "streamspace-controller",
				"stream.space/catalog-id":      fmt.Sprintf("%d", appInstall.Spec.CatalogTemplateID),
				"stream.space/installed-by":    appInstall.Spec.InstalledBy,
			},
		},
		Spec: *templateSpec,
	}

	// Set owner reference so Template is deleted when ApplicationInstall is deleted
	if err := ctrl.SetControllerReference(&appInstall, template, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference")
		if updateErr := r.updateStatus(ctx, &appInstall, "Failed", fmt.Sprintf("Failed to set owner reference: %v", err)); updateErr != nil {
			logger.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	// Create the Template
	if err := r.Create(ctx, template); err != nil {
		if errors.IsAlreadyExists(err) {
			// Template already exists, that's OK
			logger.Info("Template already exists", "templateName", appInstall.Spec.TemplateName)
			if updateErr := r.updateStatus(ctx, &appInstall, "Ready", "Template already exists"); updateErr != nil {
				logger.Error(updateErr, "Failed to update status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to create Template")
		if updateErr := r.updateStatus(ctx, &appInstall, "Failed", fmt.Sprintf("Failed to create Template: %v", err)); updateErr != nil {
			logger.Error(updateErr, "Failed to update status")
		}
		// Retry after delay
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	logger.Info("Successfully created Template", "templateName", appInstall.Spec.TemplateName)

	// Update status to Ready
	appInstall.Status.TemplateName = template.Name
	appInstall.Status.TemplateNamespace = template.Namespace
	if err := r.updateStatus(ctx, &appInstall, "Ready", "Template created successfully"); err != nil {
		logger.Error(err, "Failed to update status to Ready")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// parseManifest parses the YAML manifest and returns a TemplateSpec.
func (r *ApplicationInstallReconciler) parseManifest(manifest string) (*streamspacev1alpha1.TemplateSpec, error) {
	// Parse the YAML manifest
	var manifestData map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest), &manifestData); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	spec := &streamspacev1alpha1.TemplateSpec{}

	// Extract spec from manifest
	specData, ok := manifestData["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("manifest missing 'spec' field")
	}

	// Map fields from manifest to TemplateSpec
	if displayName, ok := specData["displayName"].(string); ok {
		spec.DisplayName = displayName
	}
	if description, ok := specData["description"].(string); ok {
		spec.Description = description
	}
	if category, ok := specData["category"].(string); ok {
		spec.Category = category
	}
	if icon, ok := specData["icon"].(string); ok {
		spec.Icon = icon
	}
	if baseImage, ok := specData["baseImage"].(string); ok {
		spec.BaseImage = baseImage
	}

	// Parse defaultResources
	if defaultRes, ok := specData["defaultResources"].(map[string]interface{}); ok {
		if requests, ok := defaultRes["requests"].(map[string]interface{}); ok {
			spec.DefaultResources.Requests = corev1.ResourceList{}
			if memory, ok := requests["memory"].(string); ok {
				quantity, err := parseQuantity(memory)
				if err == nil {
					spec.DefaultResources.Requests[corev1.ResourceMemory] = quantity
				}
			}
			if cpu, ok := requests["cpu"].(string); ok {
				quantity, err := parseQuantity(cpu)
				if err == nil {
					spec.DefaultResources.Requests[corev1.ResourceCPU] = quantity
				}
			}
		}
		if limits, ok := defaultRes["limits"].(map[string]interface{}); ok {
			spec.DefaultResources.Limits = corev1.ResourceList{}
			if memory, ok := limits["memory"].(string); ok {
				quantity, err := parseQuantity(memory)
				if err == nil {
					spec.DefaultResources.Limits[corev1.ResourceMemory] = quantity
				}
			}
			if cpu, ok := limits["cpu"].(string); ok {
				quantity, err := parseQuantity(cpu)
				if err == nil {
					spec.DefaultResources.Limits[corev1.ResourceCPU] = quantity
				}
			}
		}
	}

	// Parse ports
	if ports, ok := specData["ports"].([]interface{}); ok {
		for _, p := range ports {
			if portMap, ok := p.(map[string]interface{}); ok {
				port := corev1.ContainerPort{}
				if name, ok := portMap["name"].(string); ok {
					port.Name = name
				}
				if containerPort, ok := portMap["containerPort"].(float64); ok {
					port.ContainerPort = int32(containerPort)
				}
				if protocol, ok := portMap["protocol"].(string); ok {
					port.Protocol = corev1.Protocol(protocol)
				}
				spec.Ports = append(spec.Ports, port)
			}
		}
	}

	// Parse env
	if envVars, ok := specData["env"].([]interface{}); ok {
		for _, e := range envVars {
			if envMap, ok := e.(map[string]interface{}); ok {
				env := corev1.EnvVar{}
				if name, ok := envMap["name"].(string); ok {
					env.Name = name
				}
				if value, ok := envMap["value"].(string); ok {
					env.Value = value
				}
				spec.Env = append(spec.Env, env)
			}
		}
	}

	// Parse VNC config
	if vnc, ok := specData["vnc"].(map[string]interface{}); ok {
		if enabled, ok := vnc["enabled"].(bool); ok {
			spec.VNC.Enabled = enabled
		}
		if port, ok := vnc["port"].(float64); ok {
			spec.VNC.Port = int(port)
		}
		if protocol, ok := vnc["protocol"].(string); ok {
			spec.VNC.Protocol = protocol
		}
	}

	// Parse tags
	if tags, ok := specData["tags"].([]interface{}); ok {
		for _, t := range tags {
			if tag, ok := t.(string); ok {
				spec.Tags = append(spec.Tags, tag)
			}
		}
	}

	// Parse capabilities
	if capabilities, ok := specData["capabilities"].([]interface{}); ok {
		for _, c := range capabilities {
			if cap, ok := c.(string); ok {
				spec.Capabilities = append(spec.Capabilities, cap)
			}
		}
	}

	return spec, nil
}

// parseQuantity parses a Kubernetes resource quantity string.
func parseQuantity(s string) (resource.Quantity, error) {
	return resource.ParseQuantity(s)
}

// updateStatus updates the ApplicationInstall status.
func (r *ApplicationInstallReconciler) updateStatus(ctx context.Context, appInstall *streamspacev1alpha1.ApplicationInstall, phase, message string) error {
	appInstall.Status.Phase = phase
	appInstall.Status.Message = message
	now := metav1.Now()
	appInstall.Status.LastTransitionTime = &now

	return r.Status().Update(ctx, appInstall)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationInstallReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streamspacev1alpha1.ApplicationInstall{}).
		Owns(&streamspacev1alpha1.Template{}).
		Complete(r)
}
