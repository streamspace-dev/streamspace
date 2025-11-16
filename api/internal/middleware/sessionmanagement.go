// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements enhanced session security with idle timeout and concurrency limits.
//
// Purpose:
// The session management middleware provides security features for user authentication
// sessions including automatic logout after inactivity and enforcement of concurrent
// session limits to prevent credential sharing and session hijacking.
//
// Implementation Details:
// - Idle timeout: Automatically invalidates sessions after period of inactivity
// - Activity tracking: Updates last activity timestamp on every authenticated request
// - Concurrent sessions: Limits number of simultaneous logins per user account
// - Memory cleanup: Periodic background cleanup prevents memory leaks
// - In-memory storage: Fast but not distributed (single-server only)
//
// Security Notes:
// This middleware addresses critical security concerns:
//
// 1. Idle Timeout (Prevents Session Hijacking):
//    - User forgets to log out on public computer
//    - Without timeout: Session stays active indefinitely (attacker can use it)
//    - With timeout: Session expires after 30 minutes of inactivity
//
// 2. Concurrent Session Limits (Prevents Credential Sharing):
//    - User shares login credentials with colleagues
//    - Without limit: Unlimited concurrent sessions (credential abuse)
//    - With limit: Max 5 concurrent sessions (discourages sharing)
//
// 3. Session Cleanup (Prevents Memory Leaks):
//    - Long-running server accumulates millions of session entries
//    - Without cleanup: Memory usage grows unbounded (eventual crash)
//    - With cleanup: Stale sessions removed every 5 minutes
//
// Thread Safety:
// Safe for concurrent use. Uses sync.RWMutex for thread-safe access to session maps.
//
// Usage:
//   // Create session manager with 30-minute idle timeout and max 5 concurrent sessions
//   sessionMgr := middleware.NewSessionManager(30*time.Minute, 5)
//
//   // Apply idle timeout check to all authenticated routes
//   router.Use(sessionMgr.IdleTimeoutMiddleware())
//
//   // Track session activity on every request
//   router.Use(sessionMgr.SessionActivityMiddleware())
//
//   // In login handler, register new session
//   if err := sessionMgr.RegisterSession(username, sessionID); err != nil {
//       // Max sessions exceeded
//       return c.JSON(403, gin.H{"error": "Maximum concurrent sessions reached"})
//   }
//
//   // In logout handler, unregister session
//   sessionMgr.UnregisterSession(username, sessionID)
//
// Configuration:
//   idleTimeout: 30*time.Minute    // Session expires after 30 min inactivity
//   maxSessions: 5                  // Max 5 concurrent logins per user
//   cleanupInterval: 5*time.Minute  // Cleanup runs every 5 minutes
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
