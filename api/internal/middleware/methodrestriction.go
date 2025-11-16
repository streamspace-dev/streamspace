// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements HTTP method restriction to prevent abuse through uncommon methods.
//
// Purpose:
// This middleware restricts incoming requests to only commonly-used, safe HTTP methods.
// It prevents security issues and attacks that exploit uncommon or dangerous methods
// like TRACE (XSS via HTTP response splitting) and CONNECT (proxy abuse).
//
// Implementation Details:
// - AllowedHTTPMethods: Whitelist approach (only allow known-safe methods)
// - DisallowedHTTPMethods: Blacklist approach (explicitly block dangerous methods)
// - Returns 405 Method Not Allowed with Allow header
// - Defense in depth: Use both middlewares together for maximum security
//
// Security Notes:
// Dangerous HTTP methods and why they're blocked:
// - TRACE: Can be used in XSS attacks (cross-site tracing)
//   * Reflects request in response body
//   * Can expose authentication cookies
//   * Bypasses HttpOnly cookie protection
// - TRACK: Microsoft proprietary variant of TRACE (same vulnerability)
// - CONNECT: Used for HTTP tunneling (proxy abuse)
//   * Only needed for proxy servers
//   * Can be used to bypass firewalls
//   * Can create unauthorized tunnels
//
// Allowed methods (safe for web APIs):
// - GET: Read resources (idempotent, safe)
// - POST: Create resources (not idempotent)
// - PUT: Update resources (idempotent)
// - PATCH: Partial update (not idempotent)
// - DELETE: Remove resources (idempotent)
// - OPTIONS: CORS preflight (required for browser APIs)
// - HEAD: Metadata only (idempotent, safe)
//
// Thread Safety:
// Safe for concurrent use. No shared state between requests.
//
// Usage:
//   // Whitelist safe methods (recommended)
//   router.Use(middleware.AllowedHTTPMethods())
//
//   // Blacklist dangerous methods (additional protection)
//   router.Use(middleware.DisallowedHTTPMethods())
//
//   // Defense in depth (use both)
//   router.Use(middleware.AllowedHTTPMethods())
//   router.Use(middleware.DisallowedHTTPMethods())
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
