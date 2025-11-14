package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TemplateSpec defines the desired state of Template
type TemplateSpec struct {
	// Display name for the UI
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Description of the application
	// +optional
	Description string `json:"description,omitempty"`

	// Category for organization (e.g., "Web Browsers", "Development")
	// +optional
	Category string `json:"category,omitempty"`

	// Icon URL
	// +optional
	Icon string `json:"icon,omitempty"`

	// Container image to use
	// +kubebuilder:validation:Required
	BaseImage string `json:"baseImage"`

	// Default resource requirements
	// +optional
	DefaultResources corev1.ResourceRequirements `json:"defaultResources,omitempty"`

	// Container ports to expose
	// +optional
	Ports []corev1.ContainerPort `json:"ports,omitempty"`

	// Environment variables
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Volume mounts
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// VNC configuration (generic, VNC-agnostic)
	// +optional
	VNC VNCConfig `json:"vnc,omitempty"`

	// Capabilities (e.g., "Network", "Audio", "Clipboard")
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`

	// Tags for searching and filtering
	// +optional
	Tags []string `json:"tags,omitempty"`
}

// VNCConfig defines generic VNC settings (NOT Kasm-specific!)
type VNCConfig struct {
	// Enable VNC streaming
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// VNC port (5900 standard, 3000 for LinuxServer.io compat)
	// +kubebuilder:default=5900
	Port int `json:"port,omitempty"`

	// Protocol: "rfb", "websocket"
	// +kubebuilder:default=rfb
	// +optional
	Protocol string `json:"protocol,omitempty"`

	// Enable encryption
	// +optional
	Encryption bool `json:"encryption,omitempty"`
}

// TemplateStatus defines the observed state of Template
type TemplateStatus struct {
	// Validation status
	// +optional
	Valid bool `json:"valid,omitempty"`

	// Validation message
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tpl
// +kubebuilder:printcolumn:name="DisplayName",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.baseImage`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Template is the Schema for the templates API
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemplateSpec   `json:"spec,omitempty"`
	Status TemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TemplateList contains a list of Template
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}
