// Package events provides NATS event publishing and subscribing for StreamSpace.
//
// The subscriber handles incoming status events from platform controllers
// and updates the API database accordingly.
package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// Subscriber handles receiving events from NATS.
type Subscriber struct {
	conn         *nats.Conn
	db           *sql.DB
	publisher    *Publisher
	enabled      bool
	controllerID string
	subs         []*nats.Subscription
}

// NewSubscriber creates a new NATS event subscriber.
// If NATS is unavailable, returns a disabled subscriber.
func NewSubscriber(cfg Config, db *sql.DB, publisher *Publisher) (*Subscriber, error) {
	if cfg.URL == "" {
		log.Println("Warning: NATS_URL not configured, event subscription disabled")
		return &Subscriber{enabled: false}, nil
	}

	// Build connection options
	opts := []nats.Option{
		nats.Name("streamspace-api-subscriber"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(10),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Printf("NATS subscriber disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS subscriber reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("NATS subscriber error: %v", err)
		}),
	}

	// Add authentication if configured
	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	// Connect to NATS
	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		log.Printf("Warning: Failed to connect subscriber to NATS at %s: %v", cfg.URL, err)
		log.Println("Event subscription disabled - API will not receive controller status updates")
		return &Subscriber{enabled: false}, nil
	}

	log.Printf("API subscriber connected to NATS at %s", conn.ConnectedUrl())

	return &Subscriber{
		conn:      conn,
		db:        db,
		publisher: publisher,
		enabled:   true,
		subs:      make([]*nats.Subscription, 0),
	}, nil
}

// Start begins subscribing to status events from controllers.
func (s *Subscriber) Start(ctx context.Context) error {
	if !s.enabled {
		log.Println("NATS subscriber disabled, not starting")
		return nil
	}

	// Subscribe to session status events (from all platforms)
	sessionSub, err := s.conn.Subscribe(SubjectSessionStatus, func(msg *nats.Msg) {
		s.handleSessionStatus(msg.Data)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to session status: %w", err)
	}
	s.subs = append(s.subs, sessionSub)
	log.Printf("Subscribed to %s", SubjectSessionStatus)

	// Subscribe to app status events (from all platforms)
	appSub, err := s.conn.Subscribe(SubjectAppStatus, func(msg *nats.Msg) {
		s.handleAppStatus(msg.Data)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to app status: %w", err)
	}
	s.subs = append(s.subs, appSub)
	log.Printf("Subscribed to %s", SubjectAppStatus)

	// Subscribe to controller heartbeats
	heartbeatSub, err := s.conn.Subscribe(SubjectControllerHeartbeat, func(msg *nats.Msg) {
		s.handleControllerHeartbeat(msg.Data)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to controller heartbeat: %w", err)
	}
	s.subs = append(s.subs, heartbeatSub)
	log.Printf("Subscribed to %s", SubjectControllerHeartbeat)

	// Subscribe to controller sync requests
	syncSub, err := s.conn.Subscribe(SubjectControllerSyncRequest, func(msg *nats.Msg) {
		s.handleControllerSyncRequest(msg.Data)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to controller sync request: %w", err)
	}
	s.subs = append(s.subs, syncSub)
	log.Printf("Subscribed to %s", SubjectControllerSyncRequest)

	log.Println("API event subscriber started, listening for controller status events")

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// Close closes the NATS connection and unsubscribes from all subjects.
func (s *Subscriber) Close() {
	if s.conn != nil {
		for _, sub := range s.subs {
			sub.Unsubscribe()
		}
		s.conn.Drain()
		s.conn.Close()
	}
}

// IsEnabled returns whether event subscription is enabled.
func (s *Subscriber) IsEnabled() bool {
	return s.enabled
}

// handleSessionStatus processes session status events from controllers.
func (s *Subscriber) handleSessionStatus(data []byte) {
	var event SessionStatusEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal session status event: %v", err)
		return
	}

	log.Printf("Received session status: session=%s status=%s phase=%s from=%s",
		event.SessionID, event.Status, event.Phase, event.ControllerID)

	// Update session in database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Update the session state and URL
	query := `
		UPDATE sessions
		SET state = $1, url = $2, updated_at = $3
		WHERE id = $4
	`

	result, err := s.db.ExecContext(ctx, query, event.Status, event.URL, time.Now(), event.SessionID)
	if err != nil {
		log.Printf("Failed to update session %s status: %v", event.SessionID, err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		log.Printf("Session %s not found in database (may not be created yet)", event.SessionID)
	} else {
		log.Printf("Updated session %s to status=%s", event.SessionID, event.Status)
	}
}

// handleAppStatus processes application installation status events from controllers.
func (s *Subscriber) handleAppStatus(data []byte) {
	var event AppStatusEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal app status event: %v", err)
		return
	}

	log.Printf("Received app status: install=%s status=%s from=%s",
		event.InstallID, event.Status, event.ControllerID)

	// Update installed application in database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		UPDATE installed_applications
		SET install_status = $1, install_message = $2, updated_at = $3
		WHERE id = $4
	`

	result, err := s.db.ExecContext(ctx, query, event.Status, event.Message, time.Now(), event.InstallID)
	if err != nil {
		log.Printf("Failed to update app %s status: %v", event.InstallID, err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		log.Printf("Application %s not found in database", event.InstallID)
	} else {
		log.Printf("Updated application %s to status=%s", event.InstallID, event.Status)
	}
}

// handleControllerHeartbeat processes heartbeat events from controllers.
func (s *Subscriber) handleControllerHeartbeat(data []byte) {
	var event ControllerHeartbeatEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal controller heartbeat: %v", err)
		return
	}

	log.Printf("Controller heartbeat: id=%s platform=%s status=%s",
		event.ControllerID, event.Platform, event.Status)

	// Could update a controllers table here to track controller health
	// For now, just log it
}

// handleControllerSyncRequest processes sync requests from controllers.
// It queries the database for installed applications and publishes AppInstallEvent
// for each one so the controller can create the necessary resources.
func (s *Subscriber) handleControllerSyncRequest(data []byte) {
	var event ControllerSyncRequestEvent
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Failed to unmarshal controller sync request: %v", err)
		return
	}

	log.Printf("Controller sync request: id=%s platform=%s",
		event.ControllerID, event.Platform)

	if s.publisher == nil || !s.publisher.enabled {
		log.Printf("Warning: Cannot process sync request - publisher not available")
		return
	}

	// Query database for installed applications
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := `
		SELECT
			ia.id,
			ia.catalog_template_id,
			ia.template_name,
			ct.display_name,
			ct.description,
			ct.category,
			ct.icon_url,
			ct.manifest,
			ia.installed_by
		FROM installed_applications ia
		JOIN catalog_templates ct ON ia.catalog_template_id = ct.id
		WHERE ia.install_status = 'installed'
		ORDER BY ia.installed_at
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Failed to query installed applications for sync: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			id                string
			catalogTemplateID int
			templateName      string
			displayName       string
			description       sql.NullString
			category          sql.NullString
			iconURL           sql.NullString
			manifest          string
			installedBy       string
		)

		if err := rows.Scan(&id, &catalogTemplateID, &templateName, &displayName,
			&description, &category, &iconURL, &manifest, &installedBy); err != nil {
			log.Printf("Failed to scan installed application: %v", err)
			continue
		}

		// Publish AppInstallEvent for this application
		if err := s.publisher.PublishAppInstall(ctx, AppInstallEvent{
			InstallID:         id,
			CatalogTemplateID: catalogTemplateID,
			TemplateName:      templateName,
			DisplayName:       displayName,
			Description:       description.String,
			Category:          category.String,
			IconURL:           iconURL.String,
			Manifest:          manifest,
			InstalledBy:       installedBy,
			Platform:          event.Platform,
		}); err != nil {
			log.Printf("Failed to publish app install event for %s: %v", templateName, err)
			continue
		}

		count++
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating installed applications: %v", err)
	}

	log.Printf("Sync complete: sent %d app install events to controller %s", count, event.ControllerID)
}
