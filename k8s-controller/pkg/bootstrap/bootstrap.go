// Package bootstrap handles controller startup reconciliation tasks.
//
// This package provides functionality to ensure default applications are installed
// when the controller starts. It reads from a ConfigMap and creates any missing
// ApplicationInstall resources.
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
)

// DefaultApplication represents an application that should be installed by default.
type DefaultApplication struct {
	// TemplateName is the name of the Template/ApplicationInstall to create
	TemplateName string `json:"templateName" yaml:"templateName"`

	// CatalogTemplateID is the catalog ID for tracking
	CatalogTemplateID int `json:"catalogTemplateID" yaml:"catalogTemplateID"`

	// DisplayName is the human-readable name
	DisplayName string `json:"displayName" yaml:"displayName"`

	// Description provides information about the application
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Category organizes the application (e.g., "Web Browsers")
	Category string `json:"category,omitempty" yaml:"category,omitempty"`

	// Manifest contains the Template spec as YAML/JSON
	Manifest string `json:"manifest" yaml:"manifest"`
}

// Reconciler handles startup reconciliation of default applications.
type Reconciler struct {
	client    client.Client
	namespace string
}

// NewReconciler creates a new bootstrap reconciler.
func NewReconciler(c client.Client, namespace string) *Reconciler {
	return &Reconciler{
		client:    c,
		namespace: namespace,
	}
}

// Start implements manager.Runnable interface.
// It runs once when the controller manager starts.
func (r *Reconciler) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("bootstrap")
	logger.Info("Starting bootstrap reconciliation")

	// Wait a moment for caches to sync
	time.Sleep(2 * time.Second)

	// Read default applications from ConfigMap
	apps, err := r.readDefaultApplications(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("No default applications ConfigMap found, skipping bootstrap")
			return nil
		}
		logger.Error(err, "Failed to read default applications")
		return nil // Don't fail startup, just log and continue
	}

	if len(apps) == 0 {
		logger.Info("No default applications configured")
		return nil
	}

	logger.Info("Found default applications to reconcile", "count", len(apps))

	// Reconcile each application
	installed := 0
	skipped := 0
	failed := 0

	for _, app := range apps {
		exists, err := r.applicationExists(ctx, app.TemplateName)
		if err != nil {
			logger.Error(err, "Failed to check if application exists", "name", app.TemplateName)
			failed++
			continue
		}

		if exists {
			logger.V(1).Info("Application already installed", "name", app.TemplateName)
			skipped++
			continue
		}

		// Create ApplicationInstall
		if err := r.createApplicationInstall(ctx, app); err != nil {
			logger.Error(err, "Failed to create ApplicationInstall", "name", app.TemplateName)
			failed++
			continue
		}

		logger.Info("Created ApplicationInstall for default application", "name", app.TemplateName)
		installed++
	}

	logger.Info("Bootstrap reconciliation complete",
		"installed", installed,
		"skipped", skipped,
		"failed", failed,
	)

	return nil
}

// readDefaultApplications reads the list of default applications from a ConfigMap.
func (r *Reconciler) readDefaultApplications(ctx context.Context) ([]DefaultApplication, error) {
	configMap := &corev1.ConfigMap{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      "streamspace-default-apps",
		Namespace: r.namespace,
	}, configMap)
	if err != nil {
		return nil, err
	}

	// Get applications data
	data, ok := configMap.Data["applications"]
	if !ok {
		return nil, fmt.Errorf("ConfigMap missing 'applications' key")
	}

	// Parse as YAML (which also handles JSON)
	var apps []DefaultApplication
	if err := yaml.Unmarshal([]byte(data), &apps); err != nil {
		// Try JSON format
		if jsonErr := json.Unmarshal([]byte(data), &apps); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse applications: yaml error: %v, json error: %v", err, jsonErr)
		}
	}

	return apps, nil
}

// applicationExists checks if an ApplicationInstall or Template already exists.
func (r *Reconciler) applicationExists(ctx context.Context, name string) (bool, error) {
	// Check for existing ApplicationInstall
	appInstall := &streamv1alpha1.ApplicationInstall{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: r.namespace,
	}, appInstall)
	if err == nil {
		return true, nil // ApplicationInstall exists
	}
	if !errors.IsNotFound(err) {
		return false, err
	}

	// Check for existing Template (might have been created directly)
	template := &streamv1alpha1.Template{}
	err = r.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: r.namespace,
	}, template)
	if err == nil {
		return true, nil // Template exists
	}
	if !errors.IsNotFound(err) {
		return false, err
	}

	return false, nil
}

// createApplicationInstall creates an ApplicationInstall resource for a default application.
func (r *Reconciler) createApplicationInstall(ctx context.Context, app DefaultApplication) error {
	appInstall := &streamv1alpha1.ApplicationInstall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.TemplateName,
			Namespace: r.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "streamspace-bootstrap",
				"streamspace.io/default-app":   "true",
			},
		},
		Spec: streamv1alpha1.ApplicationInstallSpec{
			CatalogTemplateID: app.CatalogTemplateID,
			TemplateName:      app.TemplateName,
			DisplayName:       app.DisplayName,
			Description:       app.Description,
			Category:          app.Category,
			Manifest:          app.Manifest,
			InstalledBy:       "system",
		},
	}

	return r.client.Create(ctx, appInstall)
}

// NeedLeaderElection returns true because bootstrap should only run on leader.
func (r *Reconciler) NeedLeaderElection() bool {
	return true
}
