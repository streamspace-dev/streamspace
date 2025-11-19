// Package events provides NATS event subscription for the StreamSpace controller.
//
// This package enables the controller to receive events from the API and perform
// platform-specific operations (creating pods, services, PVCs, etc.).
//
// The subscriber listens to NATS subjects and triggers the appropriate
// Kubernetes operations when events are received.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config holds configuration for the NATS subscriber.
type Config struct {
	URL      string
	User     string
	Password string
}

// Subscriber subscribes to NATS events and handles them.
type Subscriber struct {
	conn         *nats.Conn
	js           nats.JetStreamContext
	client       client.Client
	namespace    string
	controllerID string
	platform     string
	handlers     map[string]EventHandler
}

// EventHandler is a function that handles a specific event type.
type EventHandler func(ctx context.Context, data []byte) error

// NewSubscriber creates a new NATS event subscriber.
func NewSubscriber(cfg Config, k8sClient client.Client, namespace, controllerID string) (*Subscriber, error) {
	if cfg.URL == "" {
		cfg.URL = nats.DefaultURL
	}

	// Connect to NATS
	opts := []nats.Option{
		nats.Name("streamspace-kubernetes-controller"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1), // Infinite reconnects
	}

	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context for durable subscriptions
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	s := &Subscriber{
		conn:         conn,
		js:           js,
		client:       k8sClient,
		namespace:    namespace,
		controllerID: controllerID,
		platform:     PlatformKubernetes,
		handlers:     make(map[string]EventHandler),
	}

	// Register default handlers
	s.registerHandlers()

	return s, nil
}

// registerHandlers registers all event handlers.
func (s *Subscriber) registerHandlers() {
	// Session events
	s.handlers[SubjectSessionCreate] = s.handleSessionCreate
	s.handlers[SubjectSessionDelete] = s.handleSessionDelete
	s.handlers[SubjectSessionHibernate] = s.handleSessionHibernate
	s.handlers[SubjectSessionWake] = s.handleSessionWake

	// Application events
	s.handlers[SubjectAppInstall] = s.handleAppInstall
	s.handlers[SubjectAppUninstall] = s.handleAppUninstall

	// Template events
	s.handlers[SubjectTemplateCreate] = s.handleTemplateCreate
	s.handlers[SubjectTemplateDelete] = s.handleTemplateDelete

	// Node events
	s.handlers[SubjectNodeCordon] = s.handleNodeCordon
	s.handlers[SubjectNodeUncordon] = s.handleNodeUncordon
	s.handlers[SubjectNodeDrain] = s.handleNodeDrain
}

// Start starts the subscriber and begins processing events.
func (s *Subscriber) Start(ctx context.Context) error {
	// Subscribe to all registered subjects with platform filter
	for subject := range s.handlers {
		// Subscribe to platform-specific subject
		platformSubject := fmt.Sprintf("%s.%s", subject, s.platform)

		_, err := s.conn.Subscribe(platformSubject, func(msg *nats.Msg) {
			// Extract base subject from the platform-specific subject
			baseSubject := subject

			handler, ok := s.handlers[baseSubject]
			if !ok {
				log.Printf("No handler for subject: %s", baseSubject)
				return
			}

			if err := handler(ctx, msg.Data); err != nil {
				log.Printf("Error handling event %s: %v", baseSubject, err)
			}
		})
		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", platformSubject, err)
		}

		log.Printf("Subscribed to NATS subject: %s", platformSubject)
	}

	// Request sync from API to get all installed applications
	if err := s.requestSync(); err != nil {
		log.Printf("Warning: failed to request sync from API: %v", err)
		// Don't fail startup - applications can still be installed via events
	} else {
		log.Printf("Sent sync request to API for platform: %s", s.platform)
	}

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// requestSync publishes a sync request to the API to get all installed applications.
func (s *Subscriber) requestSync() error {
	event := ControllerSyncRequestEvent{
		EventID:      fmt.Sprintf("sync-%s-%d", s.controllerID, time.Now().UnixNano()),
		Timestamp:    time.Now(),
		ControllerID: s.controllerID,
		Platform:     s.platform,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Publish to generic subject (not platform-specific) so API receives it
	return s.conn.Publish(SubjectControllerSyncRequest, data)
}

// Close closes the NATS connection.
func (s *Subscriber) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

// publishStatus publishes a status update event back to NATS.
func (s *Subscriber) publishStatus(subject string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return s.conn.Publish(subject, data)
}
