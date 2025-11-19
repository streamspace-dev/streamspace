package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TemplateSpec defines the desired state of a Template.
//
// Templates are application definitions that can be instantiated as Sessions.
// They define the container image, resource requirements, and configuration
// needed to run a specific application (e.g., Firefox, VS Code, GIMP).
//
// Example:
//
//	spec:
//	  displayName: "Firefox Web Browser"
//	  description: "Modern, privacy-focused web browser"
//	  category: "Web Browsers"
//	  icon: "https://example.com/firefox-icon.png"
//	  baseImage: "lscr.io/linuxserver/firefox:latest"
//	  defaultResources:
//	    requests:
//	      memory: "2Gi"
//	      cpu: "1000m"
//	    limits:
//	      memory: "4Gi"
//	      cpu: "2000m"
//	  ports:
//	    - name: vnc
//	      containerPort: 3000
//	      protocol: TCP
//	  env:
//	    - name: PUID
//	      value: "1000"
//	    - name: PGID
//	      value: "1000"
//	  vnc:
//	    enabled: true
//	    port: 3000
//	  capabilities: ["Network", "Audio", "Clipboard"]
//	  tags: ["browser", "web", "privacy"]
type TemplateSpec struct {
	// DisplayName is the human-readable name shown in the UI.
	//
	// This should be descriptive and user-friendly.
	//
	// Required: Yes
	// Example: "Firefox Web Browser", "Visual Studio Code", "GIMP Image Editor"
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Description provides detailed information about the application.
	//
	// This is shown in:
	//   - Template catalog listings
	//   - Template detail modals
	//   - Session creation forms
	//
	// Optional: Yes
	// Example: "Modern, privacy-focused web browser with built-in tracking protection"
	// +optional
	Description string `json:"description,omitempty"`

	// Category organizes templates into logical groups for easier discovery.
	//
	// Standard categories:
	//   - "Web Browsers": Firefox, Chromium, Brave
	//   - "Development": VS Code, IntelliJ, Eclipse
	//   - "Design": GIMP, Inkscape, Blender
	//   - "Productivity": LibreOffice, Calligra
	//   - "Media": Audacity, Kdenlive, OBS Studio
	//
	// Optional: Yes (defaults to "Uncategorized")
	// Example: "Web Browsers", "Development Tools"
	// +optional
	Category string `json:"category,omitempty"`

	// Icon is the URL to an icon image for this template.
	//
	// Icon specifications:
	//   - Format: PNG, SVG, or JPEG
	//   - Recommended size: 128x128 pixels
	//   - Used in catalog listings and session cards
	//
	// Optional: Yes
	// Example: "https://cdn.example.com/icons/firefox.png"
	// +optional
	Icon string `json:"icon,omitempty"`

	// BaseImage is the fully-qualified container image to run.
	//
	// Format: [registry/]repository[:tag|@digest]
	//
	// Currently uses LinuxServer.io images (temporary):
	//   - lscr.io/linuxserver/firefox:latest
	//   - lscr.io/linuxserver/chromium:latest
	//
	// Future: StreamSpace-native images with TigerVNC + noVNC:
	//   - ghcr.io/streamspace/firefox:latest
	//   - ghcr.io/streamspace/vscode:latest
	//
	// Required: Yes
	// Example: "lscr.io/linuxserver/firefox:latest"
	// +kubebuilder:validation:Required
	BaseImage string `json:"baseImage"`

	// DefaultResources specifies the default CPU and memory for sessions.
	//
	// Users can override these when creating sessions, but they serve as:
	//   - Sensible defaults for typical usage
	//   - Guidance for resource sizing
	//   - Baseline for capacity planning
	//
	// Example:
	//   defaultResources:
	//     requests:
	//       memory: "2Gi"
	//       cpu: "1000m"
	//     limits:
	//       memory: "4Gi"
	//       cpu: "2000m"
	//
	// Optional: Yes (platform defaults used if not specified)
	// +optional
	DefaultResources corev1.ResourceRequirements `json:"defaultResources,omitempty"`

	// Ports define the container ports that should be exposed.
	//
	// Common ports:
	//   - VNC: 5900 (standard) or 3000 (LinuxServer.io)
	//   - HTTP: 80, 8080
	//   - HTTPS: 443, 8443
	//
	// Each port creates a Kubernetes Service port mapping.
	//
	// Example:
	//   ports:
	//     - name: vnc
	//       containerPort: 3000
	//       protocol: TCP
	//     - name: http
	//       containerPort: 8080
	//       protocol: TCP
	//
	// Optional: Yes
	// +optional
	Ports []corev1.ContainerPort `json:"ports,omitempty"`

	// Env defines environment variables passed to the container.
	//
	// Common variables:
	//   - PUID: User ID for file permissions
	//   - PGID: Group ID for file permissions
	//   - TZ: Timezone (e.g., "America/New_York")
	//   - DISPLAY: X11 display number
	//
	// Example:
	//   env:
	//     - name: PUID
	//       value: "1000"
	//     - name: PGID
	//       value: "1000"
	//     - name: TZ
	//       value: "America/New_York"
	//
	// Optional: Yes
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// VolumeMounts specify where volumes should be mounted in the container.
	//
	// Standard mounts:
	//   - /config: User persistent home directory
	//   - /tmp: Temporary files (emptyDir)
	//
	// The SessionReconciler automatically adds the user's PVC mount if
	// persistentHome is enabled in the Session spec.
	//
	// Example:
	//   volumeMounts:
	//     - name: user-home
	//       mountPath: /config
	//     - name: tmp
	//       mountPath: /tmp
	//
	// Optional: Yes
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// VNC configures the VNC streaming settings for this template.
	//
	// IMPORTANT: This is VNC-agnostic and designed for migration.
	// Currently supports:
	//   - LinuxServer.io images with KasmVNC (temporary)
	//
	// Future target:
	//   - StreamSpace images with TigerVNC + noVNC (100% open source)
	//
	// Example:
	//   vnc:
	//     enabled: true
	//     port: 5900
	//     protocol: rfb
	//     encryption: false
	//
	// Optional: Yes (defaults to enabled on port 5900)
	// +optional
	VNC VNCConfig `json:"vnc,omitempty"`

	// Capabilities describe special features this application supports.
	//
	// Standard capabilities:
	//   - "Network": Requires internet access
	//   - "Audio": Supports audio streaming
	//   - "Clipboard": Supports clipboard sharing
	//   - "FileTransfer": Supports file upload/download
	//   - "GPU": Requires GPU acceleration
	//
	// Used for:
	//   - UI feature indicators
	//   - Resource scheduling
	//   - Security policy enforcement
	//
	// Example: ["Network", "Audio", "Clipboard"]
	// Optional: Yes
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`

	// Tags are keywords for search and filtering.
	//
	// Best practices:
	//   - Use lowercase
	//   - Include synonyms (e.g., "browser", "web")
	//   - Add use-case tags (e.g., "development", "design")
	//
	// Example: ["browser", "web", "privacy", "firefox"]
	// Optional: Yes
	// +optional
	Tags []string `json:"tags,omitempty"`
}

// VNCConfig defines generic VNC settings (VNC-agnostic, NOT Kasm-specific!).
//
// CRITICAL: StreamSpace is migrating to 100% open source VNC stack.
// This config is intentionally vendor-neutral to support the migration.
//
// Current implementation (temporary):
//   - LinuxServer.io containers with KasmVNC on port 3000
//
// Target implementation (Phase 6):
//   - StreamSpace containers with TigerVNC + noVNC on port 5900
//
// Example (current LinuxServer.io):
//
//	vnc:
//	  enabled: true
//	  port: 3000
//	  protocol: websocket
//
// Example (future TigerVNC):
//
//	vnc:
//	  enabled: true
//	  port: 5900
//	  protocol: rfb
//	  encryption: true
type VNCConfig struct {
	// Enabled determines whether VNC streaming is available for this template.
	//
	// When true:
	//   - VNC port is exposed via Service
	//   - WebSocket proxy routes are created
	//   - UI displays "Launch Session" button
	//
	// When false:
	//   - Template is headless (no GUI)
	//   - Suitable for CLI-only applications
	//
	// Default: true
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Port specifies the VNC server port inside the container.
	//
	// Standard ports:
	//   - 5900: RFB protocol standard (future TigerVNC)
	//   - 3000: LinuxServer.io convention (current)
	//   - 6080: noVNC HTTP port (alternative)
	//
	// Default: 5900
	// +kubebuilder:default=5900
	Port int `json:"port,omitempty"`

	// Protocol specifies the VNC protocol variant.
	//
	// Valid values:
	//   - "rfb": Raw RFB protocol (standard VNC)
	//   - "websocket": WebSocket-wrapped RFB (for browser clients)
	//
	// Default: rfb
	// +kubebuilder:default=rfb
	// +optional
	Protocol string `json:"protocol,omitempty"`

	// Encryption enables TLS encryption for VNC connections.
	//
	// When true:
	//   - VNC traffic is encrypted with TLS
	//   - Requires TLS certificates to be configured
	//   - Prevents eavesdropping on screen content
	//
	// When false:
	//   - VNC traffic is unencrypted (not recommended for production)
	//   - Encryption should be handled at ingress level
	//
	// Default: false (rely on ingress TLS termination)
	// +optional
	Encryption bool `json:"encryption,omitempty"`
}

// TemplateStatus defines the observed state of a Template.
//
// The status is managed by the TemplateReconciler and provides validation
// results and operational information.
//
// Example:
//
//	status:
//	  valid: true
//	  message: "Template validated successfully"
//	  conditions:
//	    - type: Validated
//	      status: "True"
//	      reason: "ImagePullable"
//	      message: "Container image is accessible"
type TemplateStatus struct {
	// Valid indicates whether the template specification is valid.
	//
	// Validation checks:
	//   - Container image exists and is pullable
	//   - Resource requests/limits are reasonable
	//   - Port numbers are valid (1-65535)
	//   - Environment variables are properly formatted
	//
	// Invalid templates cannot be used to create sessions.
	//
	// Optional: Yes (computed by controller)
	// +optional
	Valid bool `json:"valid"`

	// Message provides human-readable validation results.
	//
	// When Valid is true:
	//   - "Template validated successfully"
	//
	// When Valid is false:
	//   - Detailed error message explaining what failed
	//   - Example: "Container image not found: lscr.io/linuxserver/invalid:latest"
	//
	// Optional: Yes (computed by controller)
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent detailed validation and operational status.
	//
	// Standard condition types:
	//   - "Validated": Template passed all validation checks
	//   - "ImagePullable": Container image is accessible
	//   - "ResourcesValid": Resource requirements are within limits
	//
	// Conditions follow the Kubernetes standard:
	//   - type: Condition name
	//   - status: True, False, or Unknown
	//   - reason: Machine-readable reason code
	//   - message: Human-readable explanation
	//   - lastTransitionTime: When this condition last changed
	//
	// Optional: Yes (managed by controller)
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Template is the Schema for the templates API.
//
// Templates define application configurations that can be launched as Sessions.
// They provide:
//   - Container image specifications
//   - Default resource requirements
//   - Port and environment configurations
//   - VNC streaming settings
//   - Metadata for catalog discovery
//
// Templates are typically:
//   - Synced from external repositories (streamspace-templates)
//   - Created by platform operators
//   - Shared across all users in a namespace
//
// Example usage:
//
//	kubectl apply -f - <<EOF
//	apiVersion: stream.space/v1alpha1
//	kind: Template
//	metadata:
//	  name: firefox-browser
//	  namespace: streamspace
//	spec:
//	  displayName: "Firefox Web Browser"
//	  description: "Modern, privacy-focused web browser"
//	  category: "Web Browsers"
//	  baseImage: "lscr.io/linuxserver/firefox:latest"
//	  defaultResources:
//	    requests:
//	      memory: "2Gi"
//	      cpu: "1000m"
//	  ports:
//	    - name: vnc
//	      containerPort: 3000
//	  vnc:
//	    enabled: true
//	    port: 3000
//	  capabilities: ["Network", "Audio", "Clipboard"]
//	  tags: ["browser", "web", "privacy"]
//	EOF
//
// Kubebuilder annotations:
//   - +kubebuilder:object:root=true - Marks this as a root Kubernetes object
//   - +kubebuilder:subresource:status - Enables /status subresource
//   - +kubebuilder:resource:shortName=tpl - Allows "kubectl get tpl" as shorthand
//   - +kubebuilder:printcolumn - Defines columns shown in "kubectl get" output
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tpl
// +kubebuilder:printcolumn:name="DisplayName",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.baseImage`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemplateSpec   `json:"spec,omitempty"`
	Status TemplateStatus `json:"status,omitempty"`
}

// TemplateList contains a list of Template resources.
//
// This is the type returned by "kubectl get templates" and used by the Kubernetes
// API when listing multiple Template resources.
//
// Example response:
//
//	apiVersion: stream.space/v1alpha1
//	kind: TemplateList
//	metadata:
//	  resourceVersion: "789012"
//	items:
//	  - metadata:
//	      name: firefox-browser
//	    spec:
//	      displayName: "Firefox Web Browser"
//	      category: "Web Browsers"
//	  - metadata:
//	      name: vscode-dev
//	    spec:
//	      displayName: "Visual Studio Code"
//	      category: "Development"
//
// +kubebuilder:object:root=true
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

// init registers the Template and TemplateList types with the SchemeBuilder.
// This is called automatically when the package is imported and enables
// the controller-runtime to recognize these types.
func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}
