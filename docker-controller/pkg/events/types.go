// Package events provides NATS event types for the Docker controller.
package events

import "time"

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
	EventID      string    `json:"event_id"`
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id"`
	Status       string    `json:"status"`
	Phase        string    `json:"phase,omitempty"`
	URL          string    `json:"url,omitempty"`
	Message      string    `json:"message,omitempty"`
	ControllerID string    `json:"controller_id"`
}

// ResourceSpec defines resource requirements.
type ResourceSpec struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}
