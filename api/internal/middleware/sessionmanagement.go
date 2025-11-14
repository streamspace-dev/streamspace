package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SessionManager handles enhanced session security features
type SessionManager struct {
	// Track last activity time for each session
	lastActivity map[string]time.Time
	// Track concurrent sessions per user
	activeSessions map[string]int
	mu             sync.RWMutex
	idleTimeout    time.Duration
	maxSessions    int
	cleanupInterval time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(idleTimeout time.Duration, maxConcurrentSessions int) *SessionManager {
	sm := &SessionManager{
		lastActivity:    make(map[string]time.Time),
		activeSessions:  make(map[string]int),
		idleTimeout:     idleTimeout,
		maxSessions:     maxConcurrentSessions,
		cleanupInterval: 5 * time.Minute,
	}

	// Start cleanup goroutine
	go sm.cleanupRoutine()

	return sm
}

// cleanupRoutine periodically removes stale session data
func (sm *SessionManager) cleanupRoutine() {
	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()

		// Remove sessions that have been idle beyond timeout
		for sessionID, lastActive := range sm.lastActivity {
			if now.Sub(lastActive) > sm.idleTimeout {
				delete(sm.lastActivity, sessionID)
			}
		}

		// Prevent excessive memory usage
		if len(sm.lastActivity) > 100000 {
			sm.lastActivity = make(map[string]time.Time)
		}
		if len(sm.activeSessions) > 10000 {
			sm.activeSessions = make(map[string]int)
		}

		sm.mu.Unlock()
	}
}

// IdleTimeoutMiddleware checks for idle sessions and invalidates them
func (sm *SessionManager) IdleTimeoutMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session ID from JWT token (stored in context by auth middleware)
		sessionIDInterface, exists := c.Get("session_id")
		if !exists {
			// No session, skip idle check
			c.Next()
			return
		}

		sessionID, ok := sessionIDInterface.(string)
		if !ok || sessionID == "" {
			c.Next()
			return
		}

		// Check last activity
		sm.mu.RLock()
		lastActive, exists := sm.lastActivity[sessionID]
		sm.mu.RUnlock()

		if exists {
			// Check if session has been idle too long
			if time.Since(lastActive) > sm.idleTimeout {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error":   "Session expired",
					"message": "Your session has expired due to inactivity. Please log in again.",
					"reason":  "idle_timeout",
				})
				c.Abort()
				return
			}
		}

		// Update last activity time
		sm.mu.Lock()
		sm.lastActivity[sessionID] = time.Now()
		sm.mu.Unlock()

		c.Next()
	}
}

// ConcurrentSessionMiddleware enforces concurrent session limits per user
func (sm *SessionManager) ConcurrentSessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only check on authentication endpoints (login)
		if c.Request.Method != "POST" || c.Request.URL.Path != "/api/v1/auth/login" {
			c.Next()
			return
		}

		// This will be checked after successful authentication
		// Store the session manager in context for use in auth handler
		c.Set("session_manager", sm)

		c.Next()
	}
}

// RegisterSession registers a new session for a user
// Returns error if max concurrent sessions exceeded
func (sm *SessionManager) RegisterSession(username, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check concurrent session limit
	currentCount := sm.activeSessions[username]
	if currentCount >= sm.maxSessions {
		return &MaxSessionsError{
			Username:    username,
			MaxSessions: sm.maxSessions,
			CurrentCount: currentCount,
		}
	}

	// Register the session
	sm.activeSessions[username]++
	sm.lastActivity[sessionID] = time.Now()

	return nil
}

// UnregisterSession removes a session when user logs out
func (sm *SessionManager) UnregisterSession(username, sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Decrement active session count
	if sm.activeSessions[username] > 0 {
		sm.activeSessions[username]--
	}

	// Remove activity tracking
	delete(sm.lastActivity, sessionID)
}

// GetActiveSessions returns the number of active sessions for a user
func (sm *SessionManager) GetActiveSessions(username string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.activeSessions[username]
}

// MaxSessionsError represents an error when max concurrent sessions is exceeded
type MaxSessionsError struct {
	Username     string
	MaxSessions  int
	CurrentCount int
}

func (e *MaxSessionsError) Error() string {
	return "maximum concurrent sessions exceeded"
}

// SessionActivityMiddleware updates session activity timestamp
// This should be called on every authenticated request
func (sm *SessionManager) SessionActivityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session ID from context
		sessionIDInterface, exists := c.Get("session_id")
		if exists {
			if sessionID, ok := sessionIDInterface.(string); ok && sessionID != "" {
				sm.mu.Lock()
				sm.lastActivity[sessionID] = time.Now()
				sm.mu.Unlock()
			}
		}

		c.Next()
	}
}
