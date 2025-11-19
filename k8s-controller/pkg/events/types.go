// Package events provides NATS event types for the StreamSpace controller.
package events

import (
	"time"
)

// NATS subject constants - must match API events package
const (
	SubjectSessionCreate    = "streamspace.session.create"
	SubjectSessionDelete    = "streamspace.session.delete"
	SubjectSessionHibernate = "streamspace.session.hibernate"
	SubjectSessionWake      = "streamspace.session.wake"
	SubjectSessionStatus    = "streamspace.session.status"

	SubjectAppInstall   = "streamspace.app.install"
	SubjectAppUninstall = "streamspace.app.uninstall"
	SubjectAppStatus    = "streamspace.app.status"

	SubjectTemplateCreate = "streamspace.template.create"
	SubjectTemplateDelete = "streamspace.template.delete"

	SubjectNodeCordon   = "streamspace.node.cordon"
	SubjectNodeUncordon = "streamspace.node.uncordon"
	SubjectNodeDrain    = "streamspace.node.drain"

	SubjectControllerHeartbeat   = "streamspace.controller.heartbeat"
	SubjectControllerSyncRequest = "streamspace.controller.sync.request"
)

// Platform constants
const (
	PlatformKubernetes = "kubernetes"
	PlatformDocker     = "docker"
	PlatformHyperV     = "hyperv"
	PlatformVCenter    = "vcenter"
)

// SessionCreateEvent is received when a new session should be created.
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
}

// SessionDeleteEvent is received when a session should be deleted.
type SessionDeleteEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
	Force     bool      `json:"force"`
}

// SessionHibernateEvent is received when a session should be hibernated.
type SessionHibernateEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
}

// SessionWakeEvent is received when a hibernated session should be woken.
type SessionWakeEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
}

// SessionStatusEvent is published when session status changes.
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

// AppInstallEvent is received when an application should be installed.
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

// AppUninstallEvent is received when an application should be uninstalled.
type AppUninstallEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	InstallID    string    `json:"install_id"`
	TemplateName string    `json:"template_name"`
	Platform     string    `json:"platform"`
}

// AppStatusEvent is published when app installation status changes.
type AppStatusEvent struct {
	EventID           string    `json:"event_id"`
	Timestamp         time.Time `json:"timestamp"`
	InstallID         string    `json:"install_id"`
	Status            string    `json:"status"`
	TemplateName      string    `json:"template_name,omitempty"`
	TemplateNamespace string    `json:"template_namespace,omitempty"`
	Message           string    `json:"message,omitempty"`
	ControllerID      string    `json:"controller_id"`
}

// TemplateCreateEvent is received when a template should be created.
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

// TemplateDeleteEvent is received when a template should be deleted.
type TemplateDeleteEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	TemplateName string    `json:"template_name"`
	TemplateID   string    `json:"template_id"`
	Platform     string    `json:"platform"`
}

// NodeCordonEvent is received when a node should be cordoned.
type NodeCordonEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	NodeName  string    `json:"node_name"`
	Platform  string    `json:"platform"`
}

// NodeUncordonEvent is received when a node should be uncordoned.
type NodeUncordonEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	NodeName  string    `json:"node_name"`
	Platform  string    `json:"platform"`
}

// NodeDrainEvent is received when a node should be drained.
type NodeDrainEvent struct {
	EventID            string    `json:"event_id"`
	Timestamp          time.Time `json:"timestamp"`
	NodeName           string    `json:"node_name"`
	Platform           string    `json:"platform"`
	GracePeriodSeconds *int64    `json:"grace_period_seconds,omitempty"`
}

// ResourceSpec defines resource requirements.
type ResourceSpec struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}

// ControllerSyncRequestEvent is published when a controller starts and needs
// to sync its state with the API. The API should respond by publishing
// AppInstallEvent for each installed application.
type ControllerSyncRequestEvent struct {
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	ControllerID string    `json:"controller_id"`
	Platform     string    `json:"platform"`
}
