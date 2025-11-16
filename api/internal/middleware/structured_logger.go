// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements structured request logging.
//
// Purpose:
// The structured logger middleware captures detailed information about every HTTP
// request in a consistent, machine-parseable format. This enables log analysis,
// alerting, debugging, and observability in production environments.
//
// Implementation Details:
// - Structured format: Key-value pairs instead of unstructured text
// - Request correlation: Includes request ID for distributed tracing
// - User tracking: Logs authenticated user information when available
// - Performance metrics: Captures request duration in milliseconds
// - Error tracking: Logs Gin errors if any occurred during request processing
// - Configurable skipping: Can skip health check endpoints to reduce noise
//
// Logged Fields:
// - request_id: Correlation ID for distributed tracing (from RequestID middleware)
// - method: HTTP method (GET, POST, PUT, DELETE, etc.)
// - path: Request path (/api/v1/sessions)
// - query: Query string parameters (if enabled)
// - status: HTTP status code (200, 404, 500, etc.)
// - duration: Request processing time (human-readable: "125ms")
// - duration_ms: Request processing time in milliseconds (for metrics: 125)
// - client_ip: Client IP address
// - user_agent: Browser/client user agent string
// - user_id: Authenticated user ID (if authenticated)
// - username: Authenticated username (if authenticated)
// - errors: Concatenated error messages (if any errors occurred)
//
// Log Levels:
// - INFO: Successful requests (2xx status codes)
// - WARN: Client errors (4xx status codes)
// - ERROR: Server errors (5xx status codes)
//
// Thread Safety:
// Safe for concurrent use. Each request logs independently.
//
// Usage:
//   // Basic structured logging
//   router.Use(middleware.StructuredLogger())
//
//   // Custom configuration
//   config := middleware.DefaultStructuredLoggerConfig()
//   config.SkipHealthCheck = true  // Don't log /health endpoint
//   config.LogQuery = false         // Don't log query parameters (privacy)
//   router.Use(middleware.StructuredLoggerWithConfigFunc(config))
//
// Configuration:
//   SkipPaths: []string{}           // Paths to skip (e.g., ["/metrics", "/health"])
//   SkipHealthCheck: true            // Skip /health and /api/v1/health endpoints
//   LogQuery: true                   // Log query parameters
//   LogUserAgent: true               // Log user agent string
package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// StructuredLogger provides structured logging for all requests
// Logs include request ID, method, path, status, duration, and client IP
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Get request ID (if RequestID middleware is used)
		requestID := GetRequestID(c)

		// Get status code
		status := c.Writer.Status()

		// Build log entry with structured fields
		logEntry := map[string]interface{}{
			"request_id": requestID,
			"method":     c.Request.Method,
			"path":       path,
			"query":      raw,
			"status":     status,
			"duration":   duration.String(),
			"duration_ms": duration.Milliseconds(),
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}

		// Add user info if authenticated
		if userID, exists := c.Get("userID"); exists {
			logEntry["user_id"] = userID
		}
		if username, exists := c.Get("username"); exists {
			logEntry["username"] = username
		}

		// Add error if present
		if len(c.Errors) > 0 {
			logEntry["errors"] = c.Errors.String()
		}

		// Determine log level based on status code
		if status >= 500 {
			log.Printf("ERROR %v", logEntry)
		} else if status >= 400 {
			log.Printf("WARN %v", logEntry)
		} else {
			log.Printf("INFO %v", logEntry)
		}
	}
}

// StructuredLoggerWithConfig allows customization of structured logging
type StructuredLoggerConfig struct {
	// SkipPaths is a list of paths to skip logging (e.g., health checks)
	SkipPaths []string

	// SkipHealthCheck if true, skips logging for /health endpoint
	SkipHealthCheck bool

	// LogQuery if false, skips logging query parameters (for privacy)
	LogQuery bool

	// LogUserAgent if false, skips logging user agent
	LogUserAgent bool
}

// DefaultStructuredLoggerConfig returns default configuration
func DefaultStructuredLoggerConfig() StructuredLoggerConfig {
	return StructuredLoggerConfig{
		SkipPaths:       []string{},
		SkipHealthCheck: true,
		LogQuery:        true,
		LogUserAgent:    true,
	}
}

// StructuredLoggerWithConfigFunc creates a structured logger with custom config
func StructuredLoggerWithConfigFunc(config StructuredLoggerConfig) gin.HandlerFunc {
	// Build skip map for fast lookup
	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}
	if config.SkipHealthCheck {
		skipMap["/health"] = true
		skipMap["/api/v1/health"] = true
	}

	return func(c *gin.Context) {
		// Skip logging for certain paths
		path := c.Request.URL.Path
		if skipMap[path] {
			c.Next()
			return
		}

		// Start timer
		start := time.Now()
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Get request ID
		requestID := GetRequestID(c)

		// Get status code
		status := c.Writer.Status()

		// Build log entry
		logEntry := map[string]interface{}{
			"request_id": requestID,
			"method":     c.Request.Method,
			"path":       path,
			"status":     status,
			"duration":   duration.String(),
			"duration_ms": duration.Milliseconds(),
			"client_ip":  c.ClientIP(),
		}

		// Conditionally add query
		if config.LogQuery && raw != "" {
			logEntry["query"] = raw
		}

		// Conditionally add user agent
		if config.LogUserAgent {
			logEntry["user_agent"] = c.Request.UserAgent()
		}

		// Add user info if authenticated
		if userID, exists := c.Get("userID"); exists {
			logEntry["user_id"] = userID
		}
		if username, exists := c.Get("username"); exists {
			logEntry["username"] = username
		}

		// Add error if present
		if len(c.Errors) > 0 {
			logEntry["errors"] = c.Errors.String()
		}

		// Log based on status code
		if status >= 500 {
			log.Printf("ERROR %v", logEntry)
		} else if status >= 400 {
			log.Printf("WARN %v", logEntry)
		} else {
			log.Printf("INFO %v", logEntry)
		}
	}
}
