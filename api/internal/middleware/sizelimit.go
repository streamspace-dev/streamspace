package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Request Size Limits
const (
	// MaxRequestBodySize is the maximum allowed request body size (10MB)
	MaxRequestBodySize int64 = 10 * 1024 * 1024 // 10 MB

	// MaxJSONPayloadSize is the maximum size for JSON payloads (5MB)
	MaxJSONPayloadSize int64 = 5 * 1024 * 1024 // 5 MB

	// MaxFileUploadSize is the maximum size for file uploads (50MB)
	MaxFileUploadSize int64 = 50 * 1024 * 1024 // 50 MB
)

// RequestSizeLimiter limits the size of incoming HTTP requests
// to prevent DoS attacks via oversized payloads
func RequestSizeLimiter(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for GET, HEAD, OPTIONS requests (no body)
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Get Content-Length header
		contentLength := c.Request.ContentLength

		// Check if Content-Length exceeds limit
		if contentLength > maxSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":      "Request entity too large",
				"message":    "Request body exceeds maximum allowed size",
				"max_size_mb": float64(maxSize) / (1024 * 1024),
			})
			return
		}

		// Wrap the request body with a LimitReader
		// This prevents reading more than maxSize bytes even if Content-Length is lying
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}

// JSONSizeLimiter limits JSON payload size for API endpoints
func JSONSizeLimiter() gin.HandlerFunc {
	return RequestSizeLimiter(MaxJSONPayloadSize)
}

// FileUploadLimiter limits file upload size
func FileUploadLimiter() gin.HandlerFunc {
	return RequestSizeLimiter(MaxFileUploadSize)
}

// DefaultSizeLimiter uses the default max request body size
func DefaultSizeLimiter() gin.HandlerFunc {
	return RequestSizeLimiter(MaxRequestBodySize)
}
