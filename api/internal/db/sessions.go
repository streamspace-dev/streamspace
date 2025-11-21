// Package db provides PostgreSQL database access for StreamSpace.
//
// This file implements session management operations for multi-platform support.
// Sessions are the source of truth in the database, updated by controller status events.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Session represents a StreamSpace session in the database.
// This mirrors the k8s.Session structure for API compatibility.
type Session struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	TeamID             string     `json:"team_id,omitempty"`
	TemplateName       string     `json:"template_name"`
	State              string     `json:"state"` // running, hibernated, terminated, pending, failed
	AppType            string     `json:"app_type"`
	ActiveConnections  int        `json:"active_connections"`
	URL                string     `json:"url,omitempty"`
	Namespace          string     `json:"namespace"`
	Platform           string     `json:"platform"`
	AgentID            string     `json:"agent_id,omitempty"` // v2.0-beta: Agent managing this session
	PodName            string     `json:"pod_name,omitempty"`
	Memory             string     `json:"memory,omitempty"`
	CPU                string     `json:"cpu,omitempty"`
	PersistentHome     bool       `json:"persistent_home"`
	IdleTimeout        string     `json:"idle_timeout,omitempty"`
	MaxSessionDuration string     `json:"max_session_duration,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastConnection     *time.Time `json:"last_connection,omitempty"`
	LastDisconnect     *time.Time `json:"last_disconnect,omitempty"`
	LastActivity       *time.Time `json:"last_activity,omitempty"`
}

// SessionDB handles database operations for sessions.
type SessionDB struct {
	db *sql.DB
}

// NewSessionDB creates a new SessionDB instance.
func NewSessionDB(db *sql.DB) *SessionDB {
	return &SessionDB{db: db}
}

// CreateSession creates a new session in the database.
func (s *SessionDB) CreateSession(ctx context.Context, session *Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	session.UpdatedAt = time.Now()

	query := `
		INSERT INTO sessions (
			id, user_id, team_id, template_name, state, app_type,
			active_connections, url, namespace, platform, agent_id, pod_name,
			memory, cpu, persistent_home, idle_timeout, max_session_duration,
			created_at, updated_at, last_connection, last_disconnect, last_activity
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		ON CONFLICT (id) DO UPDATE SET
			state = EXCLUDED.state,
			url = EXCLUDED.url,
			agent_id = EXCLUDED.agent_id,
			pod_name = EXCLUDED.pod_name,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		session.ID, session.UserID, nullString(session.TeamID), session.TemplateName, session.State, session.AppType,
		session.ActiveConnections, session.URL, session.Namespace, session.Platform, nullString(session.AgentID), session.PodName,
		session.Memory, session.CPU, session.PersistentHome, session.IdleTimeout, session.MaxSessionDuration,
		session.CreatedAt, session.UpdatedAt, session.LastConnection, session.LastDisconnect, session.LastActivity,
	)
	if err != nil {
		return fmt.Errorf("failed to create session %s for user %s: %w", session.ID, session.UserID, err)
	}
	return nil
}

// GetSession retrieves a session by ID.
func (s *SessionDB) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	session := &Session{}

	query := `
		SELECT
			id, user_id, COALESCE(team_id, ''), template_name, state, COALESCE(app_type, 'desktop'),
			active_connections, COALESCE(url, ''), COALESCE(namespace, 'streamspace'),
			COALESCE(platform, 'kubernetes'), COALESCE(pod_name, ''),
			COALESCE(memory, ''), COALESCE(cpu, ''), COALESCE(persistent_home, false),
			COALESCE(idle_timeout, ''), COALESCE(max_session_duration, ''),
			created_at, updated_at, last_connection, last_disconnect, last_activity
		FROM sessions
		WHERE id = $1
	`

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID, &session.UserID, &session.TeamID, &session.TemplateName, &session.State, &session.AppType,
		&session.ActiveConnections, &session.URL, &session.Namespace, &session.Platform, &session.PodName,
		&session.Memory, &session.CPU, &session.PersistentHome, &session.IdleTimeout, &session.MaxSessionDuration,
		&session.CreatedAt, &session.UpdatedAt, &session.LastConnection, &session.LastDisconnect, &session.LastActivity,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session %s: %w", sessionID, err)
	}

	return session, nil
}

// ListSessions retrieves all sessions.
func (s *SessionDB) ListSessions(ctx context.Context) ([]*Session, error) {
	query := `
		SELECT
			id, user_id, COALESCE(team_id, ''), template_name, state, COALESCE(app_type, 'desktop'),
			active_connections, COALESCE(url, ''), COALESCE(namespace, 'streamspace'),
			COALESCE(platform, 'kubernetes'), COALESCE(pod_name, ''),
			COALESCE(memory, ''), COALESCE(cpu, ''), COALESCE(persistent_home, false),
			COALESCE(idle_timeout, ''), COALESCE(max_session_duration, ''),
			created_at, updated_at, last_connection, last_disconnect, last_activity
		FROM sessions
		WHERE state != 'deleted'
		ORDER BY created_at DESC
	`

	return s.querySessions(ctx, query)
}

// ListSessionsByUser retrieves all sessions for a specific user.
func (s *SessionDB) ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	query := `
		SELECT
			id, user_id, COALESCE(team_id, ''), template_name, state, COALESCE(app_type, 'desktop'),
			active_connections, COALESCE(url, ''), COALESCE(namespace, 'streamspace'),
			COALESCE(platform, 'kubernetes'), COALESCE(pod_name, ''),
			COALESCE(memory, ''), COALESCE(cpu, ''), COALESCE(persistent_home, false),
			COALESCE(idle_timeout, ''), COALESCE(max_session_duration, ''),
			created_at, updated_at, last_connection, last_disconnect, last_activity
		FROM sessions
		WHERE user_id = $1 AND state != 'deleted'
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions for user %s: %w", userID, err)
	}
	defer rows.Close()

	sessions, err := s.scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan sessions for user %s: %w", userID, err)
	}
	return sessions, nil
}

// ListSessionsByState retrieves all sessions with a specific state.
func (s *SessionDB) ListSessionsByState(ctx context.Context, state string) ([]*Session, error) {
	query := `
		SELECT
			id, user_id, COALESCE(team_id, ''), template_name, state, COALESCE(app_type, 'desktop'),
			active_connections, COALESCE(url, ''), COALESCE(namespace, 'streamspace'),
			COALESCE(platform, 'kubernetes'), COALESCE(pod_name, ''),
			COALESCE(memory, ''), COALESCE(cpu, ''), COALESCE(persistent_home, false),
			COALESCE(idle_timeout, ''), COALESCE(max_session_duration, ''),
			created_at, updated_at, last_connection, last_disconnect, last_activity
		FROM sessions
		WHERE state = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, state)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions with state %s: %w", state, err)
	}
	defer rows.Close()

	sessions, err := s.scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan sessions with state %s: %w", state, err)
	}
	return sessions, nil
}

// UpdateSessionState updates the state of a session.
func (s *SessionDB) UpdateSessionState(ctx context.Context, sessionID, state string) error {
	query := `
		UPDATE sessions
		SET state = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := s.db.ExecContext(ctx, query, state, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update state to %s for session %s: %w", state, sessionID, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// UpdateSessionURL updates the URL of a session.
func (s *SessionDB) UpdateSessionURL(ctx context.Context, sessionID, url string) error {
	query := `
		UPDATE sessions
		SET url = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := s.db.ExecContext(ctx, query, url, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update URL for session %s: %w", sessionID, err)
	}
	return nil
}

// UpdateSessionStatus updates session state, URL, and pod name from controller status events.
func (s *SessionDB) UpdateSessionStatus(ctx context.Context, sessionID, state, url, podName string) error {
	query := `
		UPDATE sessions
		SET state = $1, url = $2, pod_name = $3, updated_at = $4
		WHERE id = $5
	`

	result, err := s.db.ExecContext(ctx, query, state, url, podName, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update status for session %s (state=%s, url=%s, pod=%s): %w", sessionID, state, url, podName, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// UpdateLastActivity updates the last activity timestamp.
func (s *SessionDB) UpdateLastActivity(ctx context.Context, sessionID string) error {
	query := `
		UPDATE sessions
		SET last_activity = $1, updated_at = $1
		WHERE id = $2
	`

	_, err := s.db.ExecContext(ctx, query, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update last activity for session %s: %w", sessionID, err)
	}
	return nil
}

// UpdateActiveConnections updates the connection count for a session.
func (s *SessionDB) UpdateActiveConnections(ctx context.Context, sessionID string, count int) error {
	now := time.Now()
	query := `
		UPDATE sessions
		SET active_connections = $1, last_connection = $2, updated_at = $2
		WHERE id = $3
	`

	_, err := s.db.ExecContext(ctx, query, count, now, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update active connections to %d for session %s: %w", count, sessionID, err)
	}
	return nil
}

// DeleteSession marks a session as deleted.
func (s *SessionDB) DeleteSession(ctx context.Context, sessionID string) error {
	query := `
		UPDATE sessions
		SET state = 'deleted', updated_at = $1
		WHERE id = $2
	`

	_, err := s.db.ExecContext(ctx, query, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to mark session %s as deleted: %w", sessionID, err)
	}
	return nil
}

// HardDeleteSession permanently removes a session from the database.
func (s *SessionDB) HardDeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = $1", sessionID)
	if err != nil {
		return fmt.Errorf("failed to permanently delete session %s: %w", sessionID, err)
	}
	return nil
}

// CountSessionsByUser returns the number of active sessions for a user.
func (s *SessionDB) CountSessionsByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE user_id = $1 AND state IN ('running', 'pending', 'hibernated')
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions for user %s: %w", userID, err)
	}
	return count, nil
}

// GetIdleSessions returns sessions that have been idle beyond their timeout.
func (s *SessionDB) GetIdleSessions(ctx context.Context) ([]*Session, error) {
	query := `
		SELECT
			id, user_id, COALESCE(team_id, ''), template_name, state, COALESCE(app_type, 'desktop'),
			active_connections, COALESCE(url, ''), COALESCE(namespace, 'streamspace'),
			COALESCE(platform, 'kubernetes'), COALESCE(pod_name, ''),
			COALESCE(memory, ''), COALESCE(cpu, ''), COALESCE(persistent_home, false),
			COALESCE(idle_timeout, ''), COALESCE(max_session_duration, ''),
			created_at, updated_at, last_connection, last_disconnect, last_activity
		FROM sessions
		WHERE state = 'running'
			AND idle_timeout != ''
			AND last_activity IS NOT NULL
			AND last_activity < NOW() - (idle_timeout || ' seconds')::INTERVAL
		ORDER BY last_activity ASC
	`

	return s.querySessions(ctx, query)
}

// querySessions executes a query and returns sessions.
func (s *SessionDB) querySessions(ctx context.Context, query string, args ...interface{}) ([]*Session, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute session query: %w", err)
	}
	defer rows.Close()

	sessions, err := s.scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan session results: %w", err)
	}
	return sessions, nil
}

// scanSessions scans rows into Session structs.
func (s *SessionDB) scanSessions(rows *sql.Rows) ([]*Session, error) {
	var sessions []*Session

	for rows.Next() {
		session := &Session{}
		err := rows.Scan(
			&session.ID, &session.UserID, &session.TeamID, &session.TemplateName, &session.State, &session.AppType,
			&session.ActiveConnections, &session.URL, &session.Namespace, &session.Platform, &session.PodName,
			&session.Memory, &session.CPU, &session.PersistentHome, &session.IdleTimeout, &session.MaxSessionDuration,
			&session.CreatedAt, &session.UpdatedAt, &session.LastConnection, &session.LastDisconnect, &session.LastActivity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating session rows: %w", err)
	}

	return sessions, nil
}

// nullString returns a sql.NullString for empty strings.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
