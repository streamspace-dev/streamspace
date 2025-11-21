// Package activity provides session activity tracking and idle detection for StreamSpace.
//
// The activity tracker monitors user interaction with sessions and implements
// idle timeout-based auto-hibernation. Unlike the connection tracker which
// monitors network connections, this tracker monitors actual user activity
// (keyboard, mouse, application interaction).
//
// Features:
//   - LastActivity timestamp tracking in Kubernetes Session status
//   - Idle duration calculation based on lastActivity
//   - Configurable idle timeouts per session (spec.idleTimeout)
//   - Auto-hibernation after idle threshold + grace period
//   - Background idle session monitor
//
// Architecture:
//   - Stateless (reads from Kubernetes directly)
//   - Updates Session.status.lastActivity via Kubernetes API
//   - Runs periodic checks for idle sessions
//   - Hibernates sessions by updating state to "hibernated"
//
// Hibernation triggers:
//   - User interaction stopped for > idleTimeout
//   - Grace period of 5 minutes after threshold
//   - Only applies to sessions with idleTimeout configured
//   - Only hibernates sessions in "running" state
//
// Example usage:
//
//	tracker := activity.NewTracker(k8sClient)
//
//	// Update activity on user interaction
//	tracker.UpdateSessionActivity(ctx, "streamspace", "user1-firefox")
//
//	// Start background idle monitor
//	go tracker.StartIdleMonitor(ctx, "streamspace", 1*time.Minute)
package activity

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/streamspace-dev/streamspace/api/internal/events"
	"github.com/streamspace-dev/streamspace/api/internal/k8s"
)

// Tracker manages session activity tracking for idle detection and auto-hibernation.
//
// This tracker is stateless and reads directly from Kubernetes Session resources.
// It updates the status.lastActivity field and monitors for idle sessions.
//
// Difference from connection tracker:
//   - Connection tracker: Monitors network connections (WebSocket, VNC)
//   - Activity tracker: Monitors user interaction (keyboard, mouse, app activity)
//
// A session can have active connections but be idle (user not interacting),
// or vice versa (background processes running, no active user).
//
// Example:
//
//	tracker := NewTracker(k8sClient, publisher, "kubernetes")
//	err := tracker.UpdateSessionActivity(ctx, namespace, sessionName)
type Tracker struct {
	// k8sClient interacts with Kubernetes to read and update Sessions.
	k8sClient *k8s.Client
	// publisher publishes NATS events for platform-agnostic operations.
	publisher *events.Publisher
	// platform identifies the target platform (kubernetes, docker, etc.)
	platform string
}

// NewTracker creates a new activity tracker instance.
//
// The tracker is stateless and can be shared across goroutines.
//
// Example:
//
//	tracker := NewTracker(k8sClient, publisher, "kubernetes")
//	go tracker.StartIdleMonitor(ctx, "streamspace", 1*time.Minute)
func NewTracker(k8sClient *k8s.Client, publisher *events.Publisher, platform string) *Tracker {
	if platform == "" {
		platform = events.PlatformKubernetes
	}
	return &Tracker{
		k8sClient: k8sClient,
		publisher: publisher,
		platform:  platform,
	}
}

// ActivityStatus represents the current activity state of a session.
//
// This status is calculated from:
//   - status.lastActivity: Last user interaction timestamp
//   - spec.idleTimeout: Configured idle timeout (e.g., "30m")
//   - Current time: Compared against lastActivity
//
// States:
//   - IsActive: User has interacted recently (within idle threshold)
//   - IsIdle: No interaction for longer than idle threshold
//   - ShouldHibernate: Idle + grace period elapsed (ready for hibernation)
//
// Example:
//
//	status := tracker.GetActivityStatus(session)
//	if status.ShouldHibernate {
//	    log.Printf("Session has been idle for %v", status.IdleDuration)
//	}
type ActivityStatus struct {
	// IsActive indicates if the session has recent activity.
	// True if lastActivity is within idleThreshold.
	IsActive bool

	// IsIdle indicates if the session has exceeded the idle threshold.
	// True if lastActivity is older than idleThreshold.
	IsIdle bool

	// LastActivity is the timestamp of the last user interaction.
	// Nil if no activity has been recorded yet (newly created session).
	LastActivity *time.Time

	// IdleDuration is how long the session has been idle.
	// Calculated as time.Since(lastActivity).
	IdleDuration time.Duration

	// IdleThreshold is the configured timeout from spec.idleTimeout.
	// Example: 30m, 1h, 2h30m
	IdleThreshold time.Duration

	// ShouldHibernate indicates if the session should be auto-hibernated.
	// True if idle for > threshold + 5 minute grace period.
	ShouldHibernate bool
}

// UpdateSessionActivity updates the lastActivity timestamp for a session
func (t *Tracker) UpdateSessionActivity(ctx context.Context, namespace, sessionName string) error {
	now := time.Now()

	// Get current session
	session, err := t.k8sClient.GetSession(ctx, namespace, sessionName)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Update lastActivity in session status
	session.Status.LastActivity = &now

	// Update the session in Kubernetes
	if err := t.k8sClient.UpdateSessionStatus(ctx, session); err != nil {
		return fmt.Errorf("failed to update session activity: %w", err)
	}

	log.Printf("Updated activity for session %s/%s", namespace, sessionName)
	return nil
}

// GetActivityStatus calculates the current activity status of a session
func (t *Tracker) GetActivityStatus(session *k8s.Session) *ActivityStatus {
	status := &ActivityStatus{
		IsActive:    false,
		IsIdle:      false,
		LastActivity: session.Status.LastActivity,
	}

	// If no last activity recorded, consider it active (newly created)
	if session.Status.LastActivity == nil {
		status.IsActive = true
		return status
	}

	// Parse idle timeout (e.g., "30m", "1h")
	idleThreshold, err := parseDuration(session.IdleTimeout)
	if err != nil || idleThreshold == 0 {
		// No idle timeout configured, always active
		status.IsActive = true
		return status
	}

	status.IdleThreshold = idleThreshold

	// Calculate idle duration
	idleDuration := time.Since(*session.Status.LastActivity)
	status.IdleDuration = idleDuration

	// Determine if session is idle
	if idleDuration >= idleThreshold {
		status.IsIdle = true
		status.IsActive = false

		// Should hibernate if idle for more than threshold
		// Add 5 minute grace period before auto-hibernation
		if idleDuration >= idleThreshold+5*time.Minute {
			status.ShouldHibernate = true
		}
	} else {
		status.IsActive = true
		status.IsIdle = false
	}

	return status
}

// CheckIdleSessions scans all sessions and returns those that are idle
func (t *Tracker) CheckIdleSessions(ctx context.Context, namespace string) ([]*k8s.Session, error) {
	sessions, err := t.k8sClient.ListSessions(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var idleSessions []*k8s.Session
	for _, session := range sessions {
		// Only check running sessions
		if session.State != "running" {
			continue
		}

		status := t.GetActivityStatus(session)
		if status.IsIdle {
			idleSessions = append(idleSessions, session)
		}
	}

	return idleSessions, nil
}

// HibernateIdleSession hibernates a session that has been idle
func (t *Tracker) HibernateIdleSession(ctx context.Context, namespace, sessionName string) error {
	session, err := t.k8sClient.GetSession(ctx, namespace, sessionName)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session is actually idle
	status := t.GetActivityStatus(session)
	if !status.ShouldHibernate {
		return fmt.Errorf("session %s is not idle enough to hibernate", sessionName)
	}

	// Update session state to hibernated
	session.State = "hibernated"
	if err := t.k8sClient.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to hibernate session: %w", err)
	}

	// Publish hibernate event for controllers
	event := &events.SessionHibernateEvent{
		SessionID: sessionName,
		UserID:    session.User,
		Platform:  t.platform,
	}
	if err := t.publisher.PublishSessionHibernate(ctx, event); err != nil {
		log.Printf("Warning: Failed to publish session hibernate event: %v", err)
	}

	log.Printf("Auto-hibernated idle session: %s/%s (idle for %v)", namespace, sessionName, status.IdleDuration)
	return nil
}

// parseDuration parses duration strings like "30m", "1h", "2h30m"
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// StartIdleMonitor starts a background goroutine that monitors for idle sessions
func (t *Tracker) StartIdleMonitor(ctx context.Context, namespace string, checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	log.Printf("Starting idle session monitor (check interval: %v)", checkInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Idle session monitor stopped")
			return
		case <-ticker.C:
			t.checkAndHibernateIdleSessions(ctx, namespace)
		}
	}
}

// checkAndHibernateIdleSessions checks for idle sessions and hibernates them
func (t *Tracker) checkAndHibernateIdleSessions(ctx context.Context, namespace string) {
	sessions, err := t.CheckIdleSessions(ctx, namespace)
	if err != nil {
		log.Printf("Error checking idle sessions: %v", err)
		return
	}

	if len(sessions) == 0 {
		return
	}

	log.Printf("Found %d idle sessions", len(sessions))

	for _, session := range sessions {
		status := t.GetActivityStatus(session)

		// Only auto-hibernate if configured and past grace period
		if status.ShouldHibernate && session.IdleTimeout != "" {
			log.Printf("Auto-hibernating session %s (idle for %v)", session.Name, status.IdleDuration)

			if err := t.HibernateIdleSession(ctx, namespace, session.Name); err != nil {
				log.Printf("Failed to hibernate session %s: %v", session.Name, err)
			}
		}
	}
}
