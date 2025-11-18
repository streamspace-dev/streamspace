package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationInstallSpec defines the desired state of ApplicationInstall.
//
// ApplicationInstall represents a request to install an application from the catalog.
// The controller watches these resources and creates the corresponding Template CRD.
//
// This pattern provides:
//   - Automatic retry on failure
//   - Clear status reporting
//   - Separation of concerns (API doesn't need K8s write permissions for Templates)
//   - Consistent with Kubernetes declarative patterns
//
// Example:
//
//	spec:
//	  catalogTemplateID: 5
//	  templateName: "firefox"
//	  displayName: "Firefox Web Browser"
//	  description: "Modern, privacy-focused web browser"
//	  category: "Web Browsers"
//	  manifest: |
//	    apiVersion: stream.space/v1alpha1
//	    kind: Template
//	    spec:
//	      baseImage: lscr.io/linuxserver/firefox:latest
//	      ...
//	  installedBy: "user-123"
type ApplicationInstallSpec struct {
	// CatalogTemplateID is the ID of the catalog template this was installed from.
	// Used for tracking and analytics.
	//
	// Required: Yes
	// +kubebuilder:validation:Required
	CatalogTemplateID int `json:"catalogTemplateID"`

	// TemplateName is the name for the Kubernetes Template CRD to create.
	// Must be a valid DNS subdomain name.
	//
	// Required: Yes
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	TemplateName string `json:"templateName"`

	// DisplayName is the human-readable name shown in the UI.
	//
	// Required: Yes
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Description provides detailed information about the application.
	//
	// Optional: Yes
	// +optional
	Description string `json:"description,omitempty"`

	// Category organizes templates into logical groups.
	//
	// Optional: Yes
	// +optional
	Category string `json:"category,omitempty"`

	// Icon is the URL to an icon image for this template.
	//
	// Optional: Yes
	// +optional
	Icon string `json:"icon,omitempty"`

	// Manifest is the YAML manifest for the Template CRD.
	// The controller will parse this and create the Template.
	//
	// Required: Yes
	// +kubebuilder:validation:Required
	Manifest string `json:"manifest"`

	// InstalledBy is the user ID who installed this application.
	//
	// Optional: Yes
	// +optional
	InstalledBy string `json:"installedBy,omitempty"`
}

// ApplicationInstallStatus defines the observed state of ApplicationInstall.
//
// The status is managed by the ApplicationInstallReconciler and provides
// information about the Template creation progress.
//
// Example:
//
//	status:
//	  phase: Ready
//	  templateName: firefox
//	  templateNamespace: streamspace
//	  message: "Template created successfully"
type ApplicationInstallStatus struct {
	// Phase indicates the current state of the installation.
	//
	// Valid values:
	//   - Pending: Waiting to be processed
	//   - Creating: Template creation in progress
	//   - Ready: Template created successfully
	//   - Failed: Template creation failed
	//
	// +kubebuilder:validation:Enum=Pending;Creating;Ready;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// TemplateName is the name of the created Template CRD.
	//
	// +optional
	TemplateName string `json:"templateName,omitempty"`

	// TemplateNamespace is the namespace of the created Template CRD.
	//
	// +optional
	TemplateNamespace string `json:"templateNamespace,omitempty"`

	// Message provides a human-readable status message.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime is the last time the status changed.
	//
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Conditions represent detailed status information.
	//
	// Standard condition types:
	//   - TemplateCreated: Template CRD was created successfully
	//   - ManifestParsed: Manifest was parsed without errors
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ApplicationInstall is the Schema for the applicationinstalls API.
//
// ApplicationInstall represents a request to install an application from the catalog.
// When created, the controller will:
//   1. Parse the manifest field
//   2. Create a corresponding Template CRD
//   3. Update the status to Ready or Failed
//
// This provides a declarative way to manage application installations with
// automatic retry, status tracking, and proper separation of concerns.
//
// Example usage:
//
//	kubectl apply -f - <<EOF
//	apiVersion: stream.space/v1alpha1
//	kind: ApplicationInstall
//	metadata:
//	  name: firefox-5
//	  namespace: streamspace
//	spec:
//	  catalogTemplateID: 5
//	  templateName: firefox
//	  displayName: "Firefox Web Browser"
//	  category: "Web Browsers"
//	  manifest: |
//	    apiVersion: stream.space/v1alpha1
//	    kind: Template
//	    spec:
//	      baseImage: lscr.io/linuxserver/firefox:latest
//	      defaultResources:
//	        requests:
//	          memory: "2Gi"
//	          cpu: "1000m"
//	  installedBy: "admin-user"
//	EOF
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=appinstall;ai
// +kubebuilder:printcolumn:name="Template",type=string,JSONPath=`.spec.templateName`
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ApplicationInstall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationInstallSpec   `json:"spec,omitempty"`
	Status ApplicationInstallStatus `json:"status,omitempty"`
}

// ApplicationInstallList contains a list of ApplicationInstall resources.
//
// +kubebuilder:object:root=true
type ApplicationInstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationInstall `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationInstall{}, &ApplicationInstallList{})
}
