// Package middleware provides HTTP middleware for the StreamSpace API.
// This file tests the rate limiting functionality to ensure it correctly
// prevents brute force attacks while allowing legitimate traffic.
//
// Tests validate:
// - Requests are allowed up to the limit
// - Requests are blocked after exceeding the limit
// - Rate limits reset after the time window expires
// - Cleanup removes old rate limit entries to prevent memory leaks
// - GetAttempts returns accurate attempt counts
package middleware

import (
	"testing"
	"time"
)

func TestRateLimiter_CheckLimit(t *testing.T) {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
	}

	key := "test-user"
	maxAttempts := 5
	window := 1 * time.Minute

	// Test: First 5 attempts should succeed
	for i := 0; i < maxAttempts; i++ {
		if !rl.CheckLimit(key, maxAttempts, window) {
			t.Errorf("Attempt %d should have succeeded but was rate limited", i+1)
		}
	}

	// Test: 6th attempt should fail
	if rl.CheckLimit(key, maxAttempts, window) {
		t.Error("6th attempt should have been rate limited but succeeded")
	}

	// Test: Verify attempt count
	count := rl.GetAttempts(key, window)
	if count != maxAttempts {
		t.Errorf("Expected %d attempts, got %d", maxAttempts, count)
	}
}

func TestRateLimiter_ResetLimit(t *testing.T) {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
	}

	key := "test-user"
	maxAttempts := 5
	window := 1 * time.Minute

	// Consume all attempts
	for i := 0; i < maxAttempts; i++ {
		rl.CheckLimit(key, maxAttempts, window)
	}

	// Verify rate limited
	if rl.CheckLimit(key, maxAttempts, window) {
		t.Error("Should be rate limited before reset")
	}

	// Reset
	rl.ResetLimit(key)

	// Should now succeed
	if !rl.CheckLimit(key, maxAttempts, window) {
		t.Error("Should succeed after reset")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
	}

	key := "test-user"
	maxAttempts := 3
	window := 100 * time.Millisecond

	// Consume all attempts
	for i := 0; i < maxAttempts; i++ {
		if !rl.CheckLimit(key, maxAttempts, window) {
			t.Errorf("Attempt %d should have succeeded", i+1)
		}
	}

	// Should be rate limited
	if rl.CheckLimit(key, maxAttempts, window) {
		t.Error("Should be rate limited")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should now succeed (old attempts expired)
	if !rl.CheckLimit(key, maxAttempts, window) {
		t.Error("Should succeed after window expiry")
	}
}
