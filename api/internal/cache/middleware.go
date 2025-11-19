// Package cache provides Redis-based caching for StreamSpace API.
//
// This file implements HTTP caching middleware for Gin framework.
//
// Purpose:
// - Cache HTTP GET responses to reduce backend load
// - Automatically invalidate cache on mutations (POST, PUT, DELETE)
// - Add cache control headers for browser/CDN caching
// - Provide cache hit/miss transparency
//
// Features:
// - Response caching for GET requests
// - Cache key generation from request URI (SHA-256 hash)
// - Automatic cache invalidation after mutations
// - X-Cache header (HIT/MISS) for debugging
// - Async cache operations (non-blocking)
// - Cache-Control headers for browser caching
//
// Middleware Types:
//   - CacheMiddleware: Caches GET responses
//   - InvalidateCacheMiddleware: Clears cache after mutations
//   - CacheControl: Adds Cache-Control headers
//
// Implementation Details:
// - Only caches successful responses (2xx status codes)
// - Response body captured via custom ResponseWriter
// - Cache operations run asynchronously to avoid blocking requests
// - Cache keys generated via SHA-256 hash of request URI
// - Gracefully handles cache unavailability (continues without caching)
//
// Thread Safety:
// - Middleware is thread-safe (uses goroutines for async operations)
// - Safe for concurrent requests
//
// Dependencies:
// - github.com/gin-gonic/gin for HTTP framework
//
// Example Usage:
//
//	// Apply response caching middleware
//	router.Use(cache.CacheMiddleware(cacheClient, 5*time.Minute))
//
//	// Apply cache invalidation for mutations
//	router.POST("/sessions", cache.InvalidateCacheMiddleware(cacheClient, cache.SessionPattern()), handler)
//
//	// Add cache control headers
//	router.Use(cache.CacheControl(1*time.Hour))
//
//	// Result:
//	//   - GET /sessions: Cached for 5 minutes, X-Cache: HIT/MISS header added
//	//   - POST /sessions: Invalidates all session:* keys
//	//   - Response includes: Cache-Control: public, max-age=3600
package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ResponseWriter is a custom response writer that captures the response body
type ResponseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// CacheMiddleware returns a Gin middleware for caching GET requests
func CacheMiddleware(cache *Cache, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache GET requests
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// Skip if caching is disabled
		if !cache.IsEnabled() {
			c.Next()
			return
		}

		// Generate cache key from request path, query params, and userID for user-specific endpoints
		// This ensures each user gets their own cached response for endpoints like /applications/user
		userID := ""
		if uid, exists := c.Get("userID"); exists {
			if id, ok := uid.(string); ok {
				userID = id
			}
		}
		cacheKey := generateCacheKey(c.Request.URL.RequestURI(), userID)

		// Try to get cached response
		var cachedResp CachedResponse
		if err := cache.Get(c.Request.Context(), cacheKey, &cachedResp); err == nil {
			// Cache hit - return cached response
			for key, value := range cachedResp.Headers {
				c.Header(key, value)
			}
			c.Header("X-Cache", "HIT")
			c.Data(cachedResp.StatusCode, "application/json", []byte(cachedResp.Body))
			c.Abort()
			return
		}

		// Cache miss - capture the response
		writer := &ResponseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer([]byte{}),
		}
		c.Writer = writer

		c.Next()

		// Only cache successful responses
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			// Capture headers, excluding security-sensitive ones that shouldn't be cached
			headers := make(map[string]string)
			excludeHeaders := map[string]bool{
				"X-Csrf-Token":  true, // CSRF tokens must be fresh per-request
				"X-CSRF-Token":  true, // CSRF tokens (alternate case)
				"Set-Cookie":    true, // Cookies are user-specific
				"Authorization": true, // Auth headers shouldn't be cached
				"X-Request-Id":  true, // Request IDs are unique per request
			}
			for key := range c.Writer.Header() {
				if !excludeHeaders[key] {
					headers[key] = c.Writer.Header().Get(key)
				}
			}

			// Store in cache
			resp := CachedResponse{
				StatusCode: c.Writer.Status(),
				Headers:    headers,
				Body:       writer.body.String(),
			}

			// Set cache asynchronously to avoid blocking the response
			go func() {
				_ = cache.Set(c.Request.Context(), cacheKey, resp, ttl)
			}()

			c.Header("X-Cache", "MISS")
		}
	}
}

// generateCacheKey creates a consistent cache key from the request URI and optional userID
// Including userID ensures user-specific responses are cached separately
func generateCacheKey(uri string, userID string) string {
	// Combine URI and userID for the hash
	// This ensures each user gets their own cache entry for user-specific endpoints
	keyInput := uri
	if userID != "" {
		keyInput = fmt.Sprintf("%s:user:%s", uri, userID)
	}
	hash := sha256.Sum256([]byte(keyInput))
	return fmt.Sprintf("response:%s", hex.EncodeToString(hash[:]))
}

// InvalidateCacheMiddleware clears related cache entries after mutations
func InvalidateCacheMiddleware(cache *Cache, pattern string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Only invalidate on successful mutations
		if c.Request.Method != http.MethodGet && c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			if cache.IsEnabled() {
				// Invalidate asynchronously
				go func() {
					_ = cache.DeletePattern(c.Request.Context(), pattern)
				}()
			}
		}
	}
}

// CacheControl middleware adds cache control headers to responses
func CacheControl(maxAge time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Never cache authentication/authorization endpoints
		noCachePaths := []string{
			"/api/v1/auth/",     // All auth endpoints (login, logout, setup, etc.)
			"/api/v1/users/me",  // Current user info
			"/api/v1/sessions/", // Session state (dynamic)
		}

		shouldCache := true
		for _, prefix := range noCachePaths {
			if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
				shouldCache = false
				break
			}
		}

		// Only add cache headers for GET requests on cacheable paths
		if c.Request.Method == http.MethodGet && shouldCache {
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge.Seconds())))
		} else {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		}
		c.Next()
	}
}
