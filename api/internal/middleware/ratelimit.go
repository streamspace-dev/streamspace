package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter implements per-IP rate limiting using token bucket algorithm
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
}

// NewRateLimiter creates a new rate limiter
// requestsPerSecond: number of requests allowed per second
// burst: maximum burst size
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		cleanup:  5 * time.Minute, // Clean up stale limiters every 5 minutes
	}

	// Start cleanup goroutine to prevent memory leaks
	go rl.cleanupRoutine()

	return rl
}

// getLimiter returns the rate limiter for the given key (usually IP address)
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = limiter
		rl.mu.Unlock()
	}

	return limiter
}

// cleanupRoutine periodically removes limiters that haven't been used recently
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		// Simple cleanup: reset the map periodically
		// In production, you might want more sophisticated tracking
		if len(rl.limiters) > 10000 { // Prevent excessive memory usage
			rl.limiters = make(map[string]*rate.Limiter)
		}
		rl.mu.Unlock()
	}
}

// Middleware returns a Gin middleware that rate limits requests by IP
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()

		// Get limiter for this IP
		limiter := rl.getLimiter(clientIP)

		// Check if request is allowed
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// StrictMiddleware returns a stricter rate limiter for sensitive operations
func (rl *RateLimiter) StrictMiddleware(requestsPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// Create a per-minute limiter for sensitive operations
		limiter := rate.NewLimiter(rate.Limit(float64(requestsPerMinute)/60.0), requestsPerMinute)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests to this endpoint. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UserRateLimiter implements per-user rate limiting (in addition to IP-based)
// This prevents abuse from compromised tokens or accounts
type UserRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
}

// NewUserRateLimiter creates a new per-user rate limiter
// requestsPerHour: number of requests allowed per hour per user
// burst: maximum burst size
func NewUserRateLimiter(requestsPerHour float64, burst int) *UserRateLimiter {
	url := &UserRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerHour / 3600.0), // Convert to per-second
		burst:    burst,
		cleanup:  10 * time.Minute,
	}

	// Start cleanup goroutine
	go url.cleanupRoutine()

	return url
}

// getLimiter returns the rate limiter for the given user
func (url *UserRateLimiter) getLimiter(username string) *rate.Limiter {
	url.mu.RLock()
	limiter, exists := url.limiters[username]
	url.mu.RUnlock()

	if !exists {
		url.mu.Lock()
		limiter = rate.NewLimiter(url.rate, url.burst)
		url.limiters[username] = limiter
		url.mu.Unlock()
	}

	return limiter
}

// cleanupRoutine periodically removes limiters that haven't been used recently
func (url *UserRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(url.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		url.mu.Lock()
		// Reset the map periodically to prevent memory leaks
		if len(url.limiters) > 5000 { // Reasonable limit for user count
			url.limiters = make(map[string]*rate.Limiter)
		}
		url.mu.Unlock()
	}
}

// Middleware returns a Gin middleware that rate limits requests by authenticated user
// This must be placed AFTER authentication middleware
func (url *UserRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get username from context (set by auth middleware)
		usernameInterface, exists := c.Get("username")
		if !exists {
			// No authenticated user, skip user-based rate limiting
			// (IP-based rate limiting still applies)
			c.Next()
			return
		}

		username, ok := usernameInterface.(string)
		if !ok || username == "" {
			// Invalid username format, skip
			c.Next()
			return
		}

		// Get limiter for this user
		limiter := url.getLimiter(username)

		// Check if request is allowed
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":     "User rate limit exceeded",
				"message":   "You have exceeded your hourly request quota. Please try again later.",
				"retry_after": "Please wait before making more requests",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// EndpointRateLimiter implements per-user, per-endpoint rate limiting
// For example: limit session creation to 10/hour per user
type EndpointRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewEndpointRateLimiter creates a rate limiter for specific endpoints
func NewEndpointRateLimiter(requestsPerHour int, burst int) *EndpointRateLimiter {
	return &EndpointRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(float64(requestsPerHour) / 3600.0),
		burst:    burst,
	}
}

// Middleware returns middleware for endpoint-specific rate limiting
func (erl *EndpointRateLimiter) Middleware(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get username from context
		usernameInterface, exists := c.Get("username")
		if !exists {
			c.Next()
			return
		}

		username, ok := usernameInterface.(string)
		if !ok || username == "" {
			c.Next()
			return
		}

		// Create key: username:endpoint
		key := username + ":" + endpoint

		// Get or create limiter
		erl.mu.RLock()
		limiter, exists := erl.limiters[key]
		erl.mu.RUnlock()

		if !exists {
			erl.mu.Lock()
			limiter = rate.NewLimiter(erl.rate, erl.burst)
			erl.limiters[key] = limiter
			erl.mu.Unlock()
		}

		// Check rate limit
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":     "Endpoint rate limit exceeded",
				"message":   "You have exceeded the rate limit for this operation.",
				"endpoint":  endpoint,
				"retry_after": "Please wait before trying this operation again",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
