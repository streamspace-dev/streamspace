package middleware

import "time"

// Rate Limiting Constants
const (
	// DefaultMaxAttempts is the default maximum number of attempts allowed
	DefaultMaxAttempts = 5

	// DefaultRateLimitWindow is the default time window for rate limiting
	DefaultRateLimitWindow = 1 * time.Minute

	// CleanupInterval is how often the rate limiter cleans up old entries
	CleanupInterval = 5 * time.Minute

	// CleanupThreshold is the age threshold for removing old entries
	CleanupThreshold = 10 * time.Minute
)
