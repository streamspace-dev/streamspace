// Package events provides NATS event subscription for the Docker controller.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/streamspace/docker-controller/pkg/docker"
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
	docker       *docker.Client
	controllerID string
}

// NewSubscriber creates a new NATS event subscriber.
func NewSubscriber(cfg Config, dockerClient *docker.Client, controllerID string) (*Subscriber, error) {
	if cfg.URL == "" {
		cfg.URL = nats.DefaultURL
	}

	// Connect to NATS
	opts := []nats.Option{
		nats.Name("streamspace-docker-controller"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1),
	}

	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &Subscriber{
		conn:         conn,
		docker:       dockerClient,
		controllerID: controllerID,
	}, nil
}

// Start starts the subscriber and begins processing events.
func (s *Subscriber) Start(ctx context.Context) error {
	// Subscribe to Docker-specific events
	subjects := map[string]func(data []byte) error{
		"streamspace.session.create.docker":    s.handleSessionCreate,
		"streamspace.session.delete.docker":    s.handleSessionDelete,
		"streamspace.session.hibernate.docker": s.handleSessionHibernate,
		"streamspace.session.wake.docker":      s.handleSessionWake,
	}

	for subject, handler := range subjects {
		h := handler // Capture for closure
		_, err := s.conn.Subscribe(subject, func(msg *nats.Msg) {
			if err := h(msg.Data); err != nil {
				log.Printf("Error handling event %s: %v", subject, err)
			}
		})
		if err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}
		log.Printf("Subscribed to NATS subject: %s", subject)
	}

	// Block until context is cancelled
	<-ctx.Done()
	return nil
}

// Close closes the NATS connection.
func (s *Subscriber) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

// handleSessionCreate handles session creation events.
func (s *Subscriber) handleSessionCreate(data []byte) error {
	var event SessionCreateEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	log.Printf("Creating Docker session: %s for user %s", event.SessionID, event.UserID)

	// Ensure user volume exists for persistent home
	var homeVolume string
	if event.PersistentHome {
		var err error
		homeVolume, err = s.docker.EnsureUserVolume(context.Background(), event.UserID)
		if err != nil {
			s.publishStatus(event.SessionID, "failed", fmt.Sprintf("Failed to create home volume: %v", err))
			return err
		}
	}

	// Parse resources
	memory := int64(2 * 1024 * 1024 * 1024) // 2GB default
	cpuShares := int64(1024)                 // Default CPU shares

	// Get image and VNC port from template config, or use defaults
	image := "lscr.io/linuxserver/firefox:latest" // Default fallback
	vncPort := 3000                                // Default VNC port
	env := map[string]string{
		"PUID": "1000",
		"PGID": "1000",
	}

	if event.TemplateConfig != nil {
		if event.TemplateConfig.Image != "" {
			image = event.TemplateConfig.Image
		}
		if event.TemplateConfig.VNCPort > 0 {
			vncPort = event.TemplateConfig.VNCPort
		}
		// Merge template env vars with defaults
		for k, v := range event.TemplateConfig.Env {
			env[k] = v
		}
		log.Printf("Using template config: image=%s, vncPort=%d", image, vncPort)
	} else {
		log.Printf("No template config provided, using defaults: image=%s, vncPort=%d", image, vncPort)
	}

	// Create container
	config := docker.SessionConfig{
		SessionID:      event.SessionID,
		UserID:         event.UserID,
		TemplateID:     event.TemplateID,
		Image:          image,
		Memory:         memory,
		CPUShares:      cpuShares,
		VNCPort:        vncPort,
		PersistentHome: event.PersistentHome,
		HomeVolume:     homeVolume,
		Env:            env,
	}

	_, err := s.docker.CreateSession(context.Background(), config)
	if err != nil {
		s.publishStatus(event.SessionID, "failed", fmt.Sprintf("Failed to create container: %v", err))
		return err
	}

	// Get URL
	url, _ := s.docker.GetSessionURL(context.Background(), event.SessionID, vncPort)

	s.publishStatusWithURL(event.SessionID, "running", "Session created", url)
	return nil
}

// handleSessionDelete handles session deletion events.
func (s *Subscriber) handleSessionDelete(data []byte) error {
	var event SessionDeleteEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	log.Printf("Deleting Docker session: %s", event.SessionID)

	if err := s.docker.RemoveSession(context.Background(), event.SessionID, event.Force); err != nil {
		return err
	}

	s.publishStatus(event.SessionID, "deleted", "Session deleted")
	return nil
}

// handleSessionHibernate handles session hibernation events.
func (s *Subscriber) handleSessionHibernate(data []byte) error {
	var event SessionHibernateEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	log.Printf("Hibernating Docker session: %s", event.SessionID)

	if err := s.docker.StopSession(context.Background(), event.SessionID); err != nil {
		s.publishStatus(event.SessionID, "failed", fmt.Sprintf("Failed to hibernate: %v", err))
		return err
	}

	s.publishStatus(event.SessionID, "hibernated", "Session hibernated")
	return nil
}

// handleSessionWake handles session wake events.
func (s *Subscriber) handleSessionWake(data []byte) error {
	var event SessionWakeEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	log.Printf("Waking Docker session: %s", event.SessionID)

	if err := s.docker.StartSession(context.Background(), event.SessionID); err != nil {
		s.publishStatus(event.SessionID, "failed", fmt.Sprintf("Failed to wake: %v", err))
		return err
	}

	// Get URL
	url, _ := s.docker.GetSessionURL(context.Background(), event.SessionID, 3000)

	s.publishStatusWithURL(event.SessionID, "running", "Session woken", url)
	return nil
}

// publishStatus publishes a session status update.
func (s *Subscriber) publishStatus(sessionID, status, message string) {
	s.publishStatusWithURL(sessionID, status, message, "")
}

// publishStatusWithURL publishes a session status update with URL.
func (s *Subscriber) publishStatusWithURL(sessionID, status, message, url string) {
	event := SessionStatusEvent{
		EventID:      uuid.New().String(),
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		Status:       status,
		Message:      message,
		URL:          url,
		ControllerID: s.controllerID,
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal status event: %v", err)
		return
	}

	if err := s.conn.Publish("streamspace.session.status", data); err != nil {
		log.Printf("Failed to publish status: %v", err)
	}
}
