// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements request ID generation and correlation.
//
// Purpose:
// The request ID middleware provides unique identifiers for each HTTP request,
// enabling distributed tracing, log correlation, and debugging across multiple
// services and components. This is essential for troubleshooting issues in
// production environments.
//
// Implementation Details:
// - Generates UUIDv4 for each request (or accepts existing from client)
// - Stores in Gin context for handlers to access
// - Adds to response header (X-Request-ID) so clients can reference
// - Enables correlation across logs, metrics, and traces
// - Idempotent: Preserves existing request ID from upstream services
//
// Use Cases:
// 1. Distributed Tracing: Follow a request across multiple microservices
//    - API Gateway → StreamSpace API → Kubernetes Controller → Database
//    - All logs share the same request ID for easy correlation
//
// 2. Log Correlation: Find all log entries for a specific request
//    - User reports error at 10:35:42 AM
//    - Search logs for request ID from error response
//    - View complete request lifecycle (auth, validation, processing, error)
//
// 3. Customer Support: Users can provide request ID when reporting issues
//    - Error message shows: "Request ID: 550e8400-e29b-41d4-a716-446655440000"
//    - Support team searches logs with this ID
//    - Full context available for debugging
//
// 4. Performance Analysis: Track slow requests end-to-end
//    - Identify which service/component caused the delay
//    - Compare timing across different layers
//
// Thread Safety:
// Safe for concurrent use. Each request gets its own unique UUID.
//
// Usage:
//   // Add to middleware chain (should be first for complete tracing)
//   router.Use(middleware.RequestID())
//
//   // Access in handlers
//   func MyHandler(c *gin.Context) {
//       requestID := middleware.GetRequestID(c)
//       log.Printf("[%s] Processing request", requestID)
//   }
//
//   // Client can send existing request ID for distributed tracing
//   // curl -H "X-Request-ID: my-trace-id" https://api.streamspace.io/sessions
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"

	// RequestIDKey is the context key for request ID
	RequestIDKey = "request_id"
)

// RequestID middleware generates or extracts a correlation ID for each request
// This enables request tracing across distributed systems and log correlation
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get request ID from header first (for distributed tracing)
		requestID := c.GetHeader(RequestIDHeader)

		// If not provided, generate a new UUID
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for use by handlers
		c.Set(RequestIDKey, requestID)

		// Set response header so client can reference this request
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from the Gin context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
