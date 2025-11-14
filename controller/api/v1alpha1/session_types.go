package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SessionSpec defines the desired state of Session
type SessionSpec struct {
	// User who owns this session
	// +kubebuilder:validation:Required
	User string `json:"user"`

	// Template name to use for this session
	// +kubebuilder:validation:Required
	Template string `json:"template"`

	// Desired state: running, hibernated, or terminated
	// +kubebuilder:validation:Enum=running;hibernated;terminated
	// +kubebuilder:default=running
	State string `json:"state"`

	// Resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Enable persistent home directory
	// +kubebuilder:default=true
	// +optional
	PersistentHome bool `json:"persistentHome,omitempty"`

	// Idle timeout before hibernation (e.g., "30m", "1h")
	// +optional
	IdleTimeout string `json:"idleTimeout,omitempty"`

	// Maximum session duration before forced termination
	// +optional
	MaxSessionDuration string `json:"maxSessionDuration,omitempty"`
}

// SessionStatus defines the observed state of Session
type SessionStatus struct {
	// Phase of the session lifecycle
	// +optional
	Phase string `json:"phase,omitempty"`

	// Name of the created pod
	// +optional
	PodName string `json:"podName,omitempty"`

	// URL to access the session
	// +optional
	URL string `json:"url,omitempty"`

	// Last activity timestamp
	// +optional
	LastActivity *metav1.Time `json:"lastActivity,omitempty"`

	// Current resource usage
	// +optional
	ResourceUsage *ResourceUsage `json:"resourceUsage,omitempty"`

	// Conditions represent the latest available observations of the session's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ResourceUsage tracks current resource consumption
type ResourceUsage struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ss
// +kubebuilder:printcolumn:name="User",type=string,JSONPath=`.spec.user`
// +kubebuilder:printcolumn:name="Template",type=string,JSONPath=`.spec.template`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.state`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Session is the Schema for the sessions API
type Session struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SessionSpec   `json:"spec,omitempty"`
	Status SessionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SessionList contains a list of Session
type SessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Session `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Session{}, &SessionList{})
}
