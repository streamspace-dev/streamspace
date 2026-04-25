// Package services provides background services for the StreamSpace API.
//
// This file implements the Session Reconciliation Loop, which handles
// stuck sessions that are out of sync with their actual platform state.
package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	"github.com/streamspace-dev/streamspace/api/internal/models"
	"github.com/streamspace-dev/streamspace/api/internal/websocket"
)

// SessionReconciler handles stuck sessions in "terminating" or "pending" states.
//
// It runs a background loop that:
//  1. Detects sessions stuck in "terminating" for >5 minutes
//  2. Detects sessions stuck in "pending" for >5 minutes
//  3. Retries commands if agent is available
//  4. Force-updates database if agent is gone for >10 minutes
//
// This solves Issues #235 and #236 (partial fix until agent pools implemented).
type SessionReconciler struct {
	// db is the database connection
	db *db.Database

	// agentHub manages agent connections
	agentHub *websocket.AgentHub

	// commandDispatcher sends commands to agents
	commandDispatcher *CommandDispatcher

	// ctx is the context for cancellation
	ctx context.Context

	// cancel stops the reconciliation loop
	cancel context.CancelFunc

	// reconcileInterval is how often to check for stuck sessions
	reconcileInterval time.Duration

	// stuckThreshold is when a session is considered "stuck"
	stuckThreshold time.Duration

	// forceCleanupThreshold is when to force-cleanup a stuck session
	forceCleanupThreshold time.Duration
}

// NewSessionReconciler creates a new session reconciler.
//
// Example:
//
//	reconciler := NewSessionReconciler(database, agentHub, dispatcher)
//	go reconciler.Start()
func NewSessionReconciler(
	database *db.Database,
	agentHub *websocket.AgentHub,
	dispatcher *CommandDispatcher,
) *SessionReconciler {
	ctx, cancel := context.WithCancel(context.Background())

	return &SessionReconciler{
		db:                    database,
		agentHub:              agentHub,
		commandDispatcher:     dispatcher,
		ctx:                   ctx,
		cancel:                cancel,
		reconcileInterval:     60 * time.Second,  // Check every 60s
		stuckThreshold:        5 * time.Minute,   // Session stuck if >5min in state
		forceCleanupThreshold: 10 * time.Minute,  // Force cleanup if >10min
	}
}

// Start begins the reconciliation loop.
//
// This should be called in a goroutine:
//
//	go reconciler.Start()
func (r *SessionReconciler) Start() {
	log.Println("[SessionReconciler] Starting session reconciliation loop")
	ticker := time.NewTicker(r.reconcileInterval)
	defer ticker.Stop()

	// Run immediately on start, then every interval
	r.reconcile()

	for {
		select {
		case <-ticker.C:
			r.reconcile()
		case <-r.ctx.Done():
			log.Println("[SessionReconciler] Stopping reconciliation loop")
			return
		}
	}
}

// Stop gracefully stops the reconciliation loop.
func (r *SessionReconciler) Stop() {
	r.cancel()
}

// reconcile checks for stuck sessions and attempts to fix them.
func (r *SessionReconciler) reconcile() {
	log.Println("[SessionReconciler] Running reconciliation check")

	// Handle stuck terminating sessions
	r.reconcileTerminatingSessions()

	// Handle stuck pending sessions
	r.reconcilePendingSessions()
}

// reconcileTerminatingSessions handles sessions stuck in "terminating" state.
//
// Flow:
//  1. Find sessions in "terminating" for > stuckThreshold
//  2. Check if assigned agent is connected
//  3. If agent available: retry stop_session command
//  4. If agent gone for > forceCleanupThreshold: force mark as "terminated"
func (r *SessionReconciler) reconcileTerminatingSessions() {
	now := time.Now()

	// Find stuck terminating sessions
	rows, err := r.db.DB().Query(`
		SELECT id, agent_id, updated_at
		FROM sessions
		WHERE state = 'terminating'
		  AND updated_at < $1
		ORDER BY updated_at ASC
	`, now.Add(-r.stuckThreshold))

	if err != nil {
		log.Printf("[SessionReconciler] Error querying terminating sessions: %v", err)
		return
	}
	defer rows.Close()

	stuckCount := 0
	retriedCount := 0
	forcedCount := 0

	for rows.Next() {
		var sessionID, agentID string
		var updatedAt time.Time

		if err := rows.Scan(&sessionID, &agentID, &updatedAt); err != nil {
			log.Printf("[SessionReconciler] Error scanning row: %v", err)
			continue
		}

		stuckCount++
		stuckDuration := now.Sub(updatedAt)

		log.Printf("[SessionReconciler] Found stuck terminating session: %s (agent: %s, stuck for: %v)",
			sessionID, agentID, stuckDuration)

		// Check if agent is connected
		agentConnected := r.agentHub.IsAgentConnected(agentID)

		if agentConnected {
			// Agent is back online - retry stop command
			log.Printf("[SessionReconciler] Retrying stop_session for %s (agent available)", sessionID)

			if err := r.createAndDispatchCommand(agentID, sessionID, "stop_session", map[string]interface{}{
				"sessionId": sessionID,
				"deletePVC": false, // Don't delete PVC on retry
			}); err != nil {
				log.Printf("[SessionReconciler] Failed to retry stop_session for %s: %v", sessionID, err)
			} else {
				retriedCount++
			}
		} else if stuckDuration > r.forceCleanupThreshold {
			// Agent is gone and session stuck too long - force cleanup
			log.Printf("[SessionReconciler] Force-terminating session %s (agent gone, stuck for %v)",
				sessionID, stuckDuration)

			if err := r.forceTerminateSession(sessionID, "agent_unavailable"); err != nil {
				log.Printf("[SessionReconciler] Failed to force-terminate %s: %v", sessionID, err)
			} else {
				forcedCount++
			}
		} else {
			log.Printf("[SessionReconciler] Session %s waiting for agent (stuck for %v, threshold: %v)",
				sessionID, stuckDuration, r.forceCleanupThreshold)
		}
	}

	if stuckCount > 0 {
		log.Printf("[SessionReconciler] Terminating sessions: %d stuck, %d retried, %d forced",
			stuckCount, retriedCount, forcedCount)
	}
}

// reconcilePendingSessions handles sessions stuck in "pending" state.
//
// Flow:
//  1. Find sessions in "pending" for > stuckThreshold
//  2. Check if assigned agent is connected
//  3. If agent available: retry start_session command
//  4. If agent gone for > forceCleanupThreshold: mark as "failed"
func (r *SessionReconciler) reconcilePendingSessions() {
	now := time.Now()

	// Find stuck pending sessions
	rows, err := r.db.DB().Query(`
		SELECT id, agent_id, user_id, template_name, updated_at
		FROM sessions
		WHERE state = 'pending'
		  AND updated_at < $1
		ORDER BY updated_at ASC
	`, now.Add(-r.stuckThreshold))

	if err != nil {
		log.Printf("[SessionReconciler] Error querying pending sessions: %v", err)
		return
	}
	defer rows.Close()

	stuckCount := 0
	retriedCount := 0
	failedCount := 0

	for rows.Next() {
		var sessionID, agentID, userID, templateName string
		var updatedAt time.Time

		if err := rows.Scan(&sessionID, &agentID, &userID, &templateName, &updatedAt); err != nil {
			log.Printf("[SessionReconciler] Error scanning row: %v", err)
			continue
		}

		stuckCount++
		stuckDuration := now.Sub(updatedAt)

		log.Printf("[SessionReconciler] Found stuck pending session: %s (agent: %s, stuck for: %v)",
			sessionID, agentID, stuckDuration)

		// Check if agent is connected
		agentConnected := r.agentHub.IsAgentConnected(agentID)

		if agentConnected {
			// Agent is back online - retry start command
			log.Printf("[SessionReconciler] Retrying start_session for %s (agent available)", sessionID)

			// Note: This requires fetching template manifest
			// For now, just log that we would retry
			// TODO: Implement actual retry logic with template fetch
			log.Printf("[SessionReconciler] Would retry start_session for %s, but need template manifest", sessionID)
			// retriedCount++ would go here when implemented
		} else if stuckDuration > r.forceCleanupThreshold {
			// Agent is gone and session stuck too long - mark as failed
			log.Printf("[SessionReconciler] Marking session %s as failed (agent gone, stuck for %v)",
				sessionID, stuckDuration)

			if err := r.forceFailSession(sessionID, "agent_unavailable"); err != nil {
				log.Printf("[SessionReconciler] Failed to mark %s as failed: %v", sessionID, err)
			} else {
				failedCount++
			}
		} else {
			log.Printf("[SessionReconciler] Session %s waiting for agent (stuck for %v, threshold: %v)",
				sessionID, stuckDuration, r.forceCleanupThreshold)
		}
	}

	if stuckCount > 0 {
		log.Printf("[SessionReconciler] Pending sessions: %d stuck, %d retried, %d failed",
			stuckCount, retriedCount, failedCount)
	}
}

// forceTerminateSession marks a session as terminated in the database.
//
// This is used when the agent is unavailable and manual cleanup is required.
// Logs a warning for manual Kubernetes resource cleanup.
func (r *SessionReconciler) forceTerminateSession(sessionID, reason string) error {
	now := time.Now()

	_, err := r.db.DB().Exec(`
		UPDATE sessions
		SET state = 'terminated',
		    termination_reason = $1,
		    terminated_at = $2,
		    updated_at = $2
		WHERE id = $3
	`, reason, now, sessionID)

	if err != nil {
		return fmt.Errorf("failed to update database: %w", err)
	}

	log.Printf("[SessionReconciler] ⚠️  Session %s force-terminated (reason: %s)", sessionID, reason)
	log.Printf("[SessionReconciler] ⚠️  Manual Kubernetes cleanup may be required:")
	log.Printf("[SessionReconciler]     kubectl delete deployment,service -n <namespace> -l session=%s", sessionID)

	// TODO: Create audit log event
	// TODO: Emit metric: sessions_force_terminated_total

	return nil
}

// forceFailSession marks a session as failed in the database.
//
// This is used when a pending session can't be started due to agent unavailability.
func (r *SessionReconciler) forceFailSession(sessionID, reason string) error {
	now := time.Now()

	_, err := r.db.DB().Exec(`
		UPDATE sessions
		SET state = 'failed',
		    termination_reason = $1,
		    updated_at = $2
		WHERE id = $3
	`, reason, now, sessionID)

	if err != nil {
		return fmt.Errorf("failed to update database: %w", err)
	}

	log.Printf("[SessionReconciler] Session %s marked as failed (reason: %s)", sessionID, reason)

	// TODO: Create audit log event
	// TODO: Emit metric: sessions_failed_total

	return nil
}

// createAndDispatchCommand creates a command in the database and dispatches it to the agent.
//
// This ensures the command is persisted before being sent over WebSocket.
func (r *SessionReconciler) createAndDispatchCommand(agentID, sessionID, action string, payload map[string]interface{}) error {
	// Generate command ID
	commandID := "cmd-" + uuid.New().String()

	// Convert payload to CommandPayload type
	var cmdPayload *models.CommandPayload
	if payload != nil {
		p := models.CommandPayload(payload)
		cmdPayload = &p
	}

	// Create command in database
	now := time.Now()
	var command models.AgentCommand
	err := r.db.DB().QueryRow(`
		INSERT INTO agent_commands (command_id, agent_id, session_id, action, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id, command_id, agent_id, session_id, action, payload, status, error_message, created_at, sent_at, acknowledged_at, completed_at
	`, commandID, agentID, sessionID, action, cmdPayload, now).Scan(
		&command.ID,
		&command.CommandID,
		&command.AgentID,
		&command.SessionID,
		&command.Action,
		&command.Payload,
		&command.Status,
		&command.ErrorMessage,
		&command.CreatedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create command in database: %w", err)
	}

	// Dispatch command to agent via CommandDispatcher
	if err := r.commandDispatcher.DispatchCommand(&command); err != nil {
		return fmt.Errorf("failed to dispatch command: %w", err)
	}

	return nil
}

// GetStats returns reconciliation statistics.
//
// Returns the number of sessions in each stuck state.
func (r *SessionReconciler) GetStats() (map[string]int, error) {
	stats := make(map[string]int)
	now := time.Now()

	// Count stuck terminating sessions
	var terminatingCount int
	err := r.db.DB().QueryRow(`
		SELECT COUNT(*)
		FROM sessions
		WHERE state = 'terminating'
		  AND updated_at < $1
	`, now.Add(-r.stuckThreshold)).Scan(&terminatingCount)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	stats["stuck_terminating"] = terminatingCount

	// Count stuck pending sessions
	var pendingCount int
	err = r.db.DB().QueryRow(`
		SELECT COUNT(*)
		FROM sessions
		WHERE state = 'pending'
		  AND updated_at < $1
	`, now.Add(-r.stuckThreshold)).Scan(&pendingCount)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	stats["stuck_pending"] = pendingCount

	return stats, nil
}
