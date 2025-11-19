// Package events provides NATS event publishing for StreamSpace.
//
// This package enables event-driven communication between the API and
// platform controllers (Kubernetes, Docker, Hyper-V, vCenter, etc.).
//
// Events are published to NATS subjects and consumed by controllers
// that perform platform-specific operations.
package events

import (
	"time"
)

// SessionCreateEvent is published when a new session is requested.
type SessionCreateEvent struct {
	EventID        string            `json:"event_id"`
	Timestamp      time.Time         `json:"timestamp"`
	SessionID      string            `json:"session_id"`
	UserID         string            `json:"user_id"`
	TemplateID     string            `json:"template_id"`
	Platform       string            `json:"platform"`
	Resources      ResourceSpec      `json:"resources"`
	PersistentHome bool              `json:"persistent_home"`
	IdleTimeout    string            `json:"idle_timeout"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	// Template configuration - used by controllers to create sessions
	TemplateConfig *TemplateConfig `json:"template_config,omitempty"`
}

// TemplateConfig holds template configuration for session creation.
type TemplateConfig struct {
	Image       string            `json:"image"`
	VNCPort     int               `json:"vnc_port"`
	DisplayName string            `json:"display_name,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// SessionDeleteEvent is published when a session should be deleted.
type SessionDeleteEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
	Force     bool      `json:"force"`
}

// SessionHibernateEvent is published when a session should be hibernated.
type SessionHibernateEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
}

// SessionWakeEvent is published when a hibernated session should be woken.
type SessionWakeEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
}

// SessionStatusEvent is published by controllers when session status changes.
type SessionStatusEvent struct {
	EventID       string        `json:"event_id"`
	Timestamp     time.Time     `json:"timestamp"`
	SessionID     string        `json:"session_id"`
	Status        string        `json:"status"`
	Phase         string        `json:"phase"`
	URL           string        `json:"url,omitempty"`
	PodName       string        `json:"pod_name,omitempty"`
	Message       string        `json:"message,omitempty"`
	ResourceUsage *ResourceSpec `json:"resource_usage,omitempty"`
	ControllerID  string        `json:"controller_id"`
}

// AppInstallEvent is published when an application should be installed.
type AppInstallEvent struct {
	EventID           string    `json:"event_id"`
	Timestamp         time.Time `json:"timestamp"`
	InstallID         string    `json:"install_id"`
	CatalogTemplateID int       `json:"catalog_template_id"`
	TemplateName      string    `json:"template_name"`
	DisplayName       string    `json:"display_name"`
	Description       string    `json:"description,omitempty"`
	Category          string    `json:"category,omitempty"`
	IconURL           string    `json:"icon_url,omitempty"`
	Manifest          string    `json:"manifest"`
	InstalledBy       string    `json:"installed_by"`
	Platform          string    `json:"platform"`
}

// AppUninstallEvent is published when an application should be uninstalled.
type AppUninstallEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	InstallID    string    `json:"install_id"`
	TemplateName string    `json:"template_name"`
	Platform     string    `json:"platform"`
}

// AppStatusEvent is published by controllers when app installation status changes.
type AppStatusEvent struct {
	EventID           string    `json:"event_id"`
	Timestamp         time.Time `json:"timestamp"`
	InstallID         string    `json:"install_id"`
	Status            string    `json:"status"` // pending, installing, ready, failed
	TemplateName      string    `json:"template_name,omitempty"`
	TemplateNamespace string    `json:"template_namespace,omitempty"`
	Message           string    `json:"message,omitempty"`
	ControllerID      string    `json:"controller_id"`
}

// TemplateCreateEvent is published when a template is created.
type TemplateCreateEvent struct {
	EventID     string    `json:"event_id"`
	Timestamp   time.Time `json:"timestamp"`
	TemplateID  string    `json:"template_id"`
	DisplayName string    `json:"display_name"`
	Category    string    `json:"category,omitempty"`
	BaseImage   string    `json:"base_image,omitempty"`
	Manifest    string    `json:"manifest,omitempty"`
	Platform    string    `json:"platform"`
	CreatedBy   string    `json:"created_by,omitempty"`
}

// TemplateDeleteEvent is published when a template should be deleted.
type TemplateDeleteEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	TemplateName string    `json:"template_name"`
	Platform     string    `json:"platform"`
}

// NodeCordonEvent is published when a node should be cordoned.
type NodeCordonEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	NodeName  string    `json:"node_name"`
	Platform  string    `json:"platform"`
}

// NodeUncordonEvent is published when a node should be uncordoned.
type NodeUncordonEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	NodeName  string    `json:"node_name"`
	Platform  string    `json:"platform"`
}

// NodeDrainEvent is published when a node should be drained.
type NodeDrainEvent struct {
	EventID            string    `json:"event_id"`
	Timestamp          time.Time `json:"timestamp"`
	NodeName           string    `json:"node_name"`
	Platform           string    `json:"platform"`
	GracePeriodSeconds *int64    `json:"grace_period_seconds,omitempty"`
}

// ControllerHeartbeatEvent is published by controllers to indicate health.
type ControllerHeartbeatEvent struct {
	ControllerID string                 `json:"controller_id"`
	Platform     string                 `json:"platform"`
	Timestamp    time.Time              `json:"timestamp"`
	Status       string                 `json:"status"` // healthy, unhealthy
	Version      string                 `json:"version"`
	Capabilities []string               `json:"capabilities"`
	ClusterInfo  map[string]interface{} `json:"cluster_info,omitempty"`
}

// ControllerSyncRequestEvent is received from controllers requesting
// a list of all installed applications. The API responds by publishing
// AppInstallEvent for each installed application.
type ControllerSyncRequestEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	ControllerID string    `json:"controller_id"`
	Platform     string    `json:"platform"`
}

// ResourceSpec defines resource requirements.
type ResourceSpec struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}

// Platform constants
const (
	PlatformKubernetes = "kubernetes"
	PlatformDocker     = "docker"
	PlatformHyperV     = "hyperv"
	PlatformVCenter    = "vcenter"
)

// Status constants
const (
	StatusPending    = "pending"
	StatusCreating   = "creating"
	StatusRunning    = "running"
	StatusHibernated = "hibernated"
	StatusFailed     = "failed"
	StatusDeleting   = "deleting"
	StatusDeleted    = "deleted"
)

// Install status constants
const (
	InstallStatusPending    = "pending"
	InstallStatusInstalling = "installing"
	InstallStatusReady      = "ready"
	InstallStatusFailed     = "failed"
)
