// Package events provides stub event publishing for backwards compatibility.
// NATS has been removed - all event publishing is now a no-op.
// Agents communicate directly via WebSocket instead of via message broker.
package events

import (
	"context"
	"log"
)

// Platform constants (preserved for backwards compatibility)
const (
	PlatformKubernetes = "kubernetes"
	PlatformDocker     = "docker"
)

// Install status constants
const (
	InstallStatusPending = "pending"
)

// Publisher is a no-op stub that replaces the NATS event publisher
type Publisher struct{}

// Config is a stub config struct
type Config struct {
	URL      string
	User     string
	Password string
}

// NewPublisher creates a no-op publisher
func NewPublisher(cfg Config) (*Publisher, error) {
	log.Println("NATS removed - event publishing is now a no-op (agents use WebSocket)")
	return &Publisher{}, nil
}

// Close is a no-op
func (p *Publisher) Close() error {
	return nil
}

// Event types for backwards compatibility

type ResourceSpec struct {
	Memory string
	CPU    string
}

type TemplateConfig struct {
	Image       string
	VNCPort     int
	DisplayName string
	Env         map[string]string
}

type SessionCreateEvent struct {
	SessionID      string
	UserID         string
	TemplateID     string
	Platform       string
	Resources      ResourceSpec
	PersistentHome bool
	IdleTimeout    string
	TemplateConfig *TemplateConfig
}

type SessionDeleteEvent struct {
	SessionID string
	UserID    string
	Platform  string
}

type SessionHibernateEvent struct {
	SessionID string
	UserID    string
	Platform  string
}

type SessionWakeEvent struct {
	SessionID string
	UserID    string
	Platform  string
}

type AppInstallEvent struct {
	InstallID         string
	CatalogTemplateID int
	TemplateName      string
	DisplayName       string
	Description       string
	Category          string
	IconURL           string
	Manifest          string
	InstalledBy       string
	Platform          string
}

type AppUninstallEvent struct {
	InstallID    string
	TemplateName string
	Platform     string
}

type TemplateCreateEvent struct {
	TemplateName string
	TemplateID   string // Alias for TemplateName
	Platform     string
	DisplayName  string
	Category     string
	BaseImage    string
}

type TemplateDeleteEvent struct {
	TemplateName string
	Platform     string
}

// Publish methods - all no-ops now that agents use WebSocket

func (p *Publisher) PublishSessionCreate(ctx context.Context, event *SessionCreateEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}

func (p *Publisher) PublishSessionDelete(ctx context.Context, event *SessionDeleteEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}

func (p *Publisher) PublishSessionHibernate(ctx context.Context, event *SessionHibernateEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}

func (p *Publisher) PublishSessionWake(ctx context.Context, event *SessionWakeEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}

func (p *Publisher) PublishAppInstall(ctx context.Context, event *AppInstallEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}

func (p *Publisher) PublishAppUninstall(ctx context.Context, event *AppUninstallEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}

func (p *Publisher) PublishTemplateCreate(ctx context.Context, event *TemplateCreateEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}

func (p *Publisher) PublishTemplateDelete(ctx context.Context, event *TemplateDeleteEvent) error {
	// No-op: Agents receive commands via WebSocket CommandDispatcher
	return nil
}
