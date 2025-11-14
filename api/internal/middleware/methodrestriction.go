package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AllowedHTTPMethods restricts incoming requests to only allowed HTTP methods
// This prevents abuse through uncommon HTTP methods (TRACE, CONNECT, etc.)
func AllowedHTTPMethods() gin.HandlerFunc {
	// Define allowed methods
	allowedMethods := map[string]bool{
		http.MethodGet:     true,
		http.MethodPost:    true,
		http.MethodPut:     true,
		http.MethodPatch:   true,
		http.MethodDelete:  true,
		http.MethodOptions: true, // Required for CORS preflight
		http.MethodHead:    true, // Common for health checks
	}

	return func(c *gin.Context) {
		method := c.Request.Method

		// Check if method is allowed
		if !allowedMethods[method] {
			c.Header("Allow", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"error":   "Method not allowed",
				"message": "The HTTP method " + method + " is not allowed for this resource.",
				"allowed_methods": []string{
					"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// DisallowedHTTPMethods explicitly blocks specific dangerous HTTP methods
// Use this in addition to AllowedHTTPMethods for defense in depth
func DisallowedHTTPMethods() gin.HandlerFunc {
	// Methods that should never be allowed
	disallowedMethods := map[string]bool{
		"TRACE":   true, // Can be used for XSS attacks
		"TRACK":   true, // Microsoft proprietary, similar to TRACE
		"CONNECT": true, // Typically only for proxies
	}

	return func(c *gin.Context) {
		method := c.Request.Method

		// Check if method is explicitly disallowed
		if disallowedMethods[method] {
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"error":   "Method not allowed",
				"message": "The HTTP method " + method + " is not permitted.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
