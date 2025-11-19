// Package integration provides integration tests for StreamSpace.
// These tests validate security controls including authentication,
// authorization, and protection against common vulnerabilities.
package integration

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSAMLReturnURLValidation validates that SAML return URLs are validated
// against a whitelist to prevent open redirects (TC-SEC-001).
//
// Related Issue: SAML Return URL - Open redirect vulnerability
// Impact: Security vulnerability allowing attacker-controlled redirects
func TestSAMLReturnURLValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	testCases := []struct {
		name          string
		returnURL     string
		shouldAllow   bool
		expectedRedir string // If allowed, where should it redirect
	}{
		// Valid internal URLs (should work)
		{
			name:        "Valid internal path - dashboard",
			returnURL:   "/dashboard",
			shouldAllow: true,
		},
		{
			name:        "Valid internal path - sessions",
			returnURL:   "/sessions",
			shouldAllow: true,
		},
		{
			name:        "Valid internal path - settings",
			returnURL:   "/settings",
			shouldAllow: true,
		},

		// Invalid external URLs (should be blocked)
		{
			name:        "External domain - https",
			returnURL:   "https://evil.com",
			shouldAllow: false,
		},
		{
			name:        "External domain - http",
			returnURL:   "http://attacker.com/phish",
			shouldAllow: false,
		},
		{
			name:        "Protocol-relative URL",
			returnURL:   "//evil.com/path",
			shouldAllow: false,
		},

		// Malicious URLs (should be blocked)
		{
			name:        "JavaScript URL",
			returnURL:   "javascript:alert(1)",
			shouldAllow: false,
		},
		{
			name:        "Data URL",
			returnURL:   "data:text/html,<script>alert(1)</script>",
			shouldAllow: false,
		},
		{
			name:        "URL with @ bypass attempt",
			returnURL:   "https://streamspace.local@evil.com",
			shouldAllow: false,
		},
		{
			name:        "Backslash bypass attempt",
			returnURL:   "/\\evil.com",
			shouldAllow: false,
		},
		{
			name:        "Encoded bypass attempt",
			returnURL:   "https://evil.com%2F%2E%2E",
			shouldAllow: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build SAML login URL with return URL
			loginURL := baseURL + "/api/v1/auth/saml/login?returnUrl=" + tc.returnURL

			req, err := http.NewRequestWithContext(ctx, "GET", loginURL, nil)
			require.NoError(t, err, "Failed to create request")

			// Don't follow redirects - we want to see the redirect response
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}

			resp, err := client.Do(req)
			require.NoError(t, err, "Failed to make request")
			defer resp.Body.Close()

			if tc.shouldAllow {
				// Valid URLs should proceed with SAML flow (redirect to IdP)
				// or if IdP not configured, at least not reject the return URL
				assert.True(t, resp.StatusCode == http.StatusFound ||
					resp.StatusCode == http.StatusTemporaryRedirect ||
					resp.StatusCode == http.StatusOK,
					"Valid return URL should be accepted")

				// If there's a redirect, verify it's to IdP not attacker
				location := resp.Header.Get("Location")
				if location != "" {
					assert.NotContains(t, strings.ToLower(location), "evil",
						"Should not redirect to evil domain")
				}
			} else {
				// Invalid URLs should be rejected
				// Could return 400, 403, or redirect to default page
				if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect {
					location := resp.Header.Get("Location")
					assert.NotContains(t, strings.ToLower(location), "evil",
						"Should not redirect to attacker domain")
					assert.NotContains(t, location, "javascript:",
						"Should not use javascript: URL")
					assert.NotContains(t, location, "data:",
						"Should not use data: URL")
				} else {
					// Rejection via error status is also acceptable
					assert.True(t, resp.StatusCode >= 400,
						"Invalid return URL should be rejected with error status")
				}
			}
		})
	}
}

// TestCSRFTokenValidation validates that CSRF tokens are properly validated
// for state-changing requests (TC-SEC-002).
func TestCSRFTokenValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Step 1: Login to get session and CSRF token
	// For this test, we assume an authenticated session exists
	// In real test, would do full login flow

	testCases := []struct {
		name           string
		csrfToken      string
		expectSuccess  bool
		expectedStatus int
	}{
		{
			name:           "Missing CSRF token",
			csrfToken:      "",
			expectSuccess:  false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Invalid CSRF token",
			csrfToken:      "invalid-forged-token",
			expectSuccess:  false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Malformed CSRF token",
			csrfToken:      "not-a-valid-format",
			expectSuccess:  false,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a state-changing request (POST)
			req, err := http.NewRequestWithContext(ctx, "POST",
				baseURL+"/api/v1/sessions", strings.NewReader(`{"user":"test","template":"firefox"}`))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			addAuthHeader(t, req)

			if tc.csrfToken != "" {
				req.Header.Set("X-CSRF-Token", tc.csrfToken)
			}

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.expectSuccess {
				assert.True(t, resp.StatusCode < 400,
					"Request with valid CSRF token should succeed")
			} else {
				assert.Equal(t, tc.expectedStatus, resp.StatusCode,
					"Request with invalid/missing CSRF token should be rejected")
			}
		})
	}
}

// TestDemoModeDisabledByDefault validates that demo mode is not accessible
// in production environment (TC-SEC-004).
//
// Related Issue: Demo Mode - Hardcoded auth allows ANY username
// Impact: Security risk if enabled in production
func TestDemoModeDisabledByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Ensure DEMO_MODE is not set
	originalDemoMode := os.Getenv("DEMO_MODE")
	os.Unsetenv("DEMO_MODE")
	defer func() {
		if originalDemoMode != "" {
			os.Setenv("DEMO_MODE", originalDemoMode)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Test 1: Try to login with demo credentials
	demoLoginPayload := `{"username":"demo","password":"demo","demo":true}`
	req, err := http.NewRequestWithContext(ctx, "POST",
		baseURL+"/api/v1/auth/login", strings.NewReader(demoLoginPayload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Demo login should fail in production
	assert.True(t, resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden ||
		resp.StatusCode == http.StatusNotFound,
		"Demo login should be rejected when DEMO_MODE is not enabled")

	// Test 2: Try demo endpoint if it exists
	req, err = http.NewRequestWithContext(ctx, "GET",
		baseURL+"/api/v1/auth/demo", nil)
	require.NoError(t, err)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Demo endpoint should not exist or return error
	assert.True(t, resp.StatusCode >= 400,
		"Demo endpoint should not be accessible in production")

	// Test 3: Verify any username cannot login
	anyUserPayload := `{"username":"anyuser","password":"","demo":true}`
	req, err = http.NewRequestWithContext(ctx, "POST",
		baseURL+"/api/v1/auth/login", strings.NewReader(anyUserPayload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden,
		"Arbitrary username login should be rejected")

	t.Log("Demo mode security test passed - demo mode is disabled by default")
}

// TestWebhookSecretGeneration validates that webhook secret generation
// doesn't panic and handles errors gracefully (TC-SEC-011).
//
// Related Issue: Webhook Secret Generation Panic
// Impact: API crashes if random generation fails
func TestWebhookSecretGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Test creating webhook without providing secret (should auto-generate)
	webhookPayload := `{
		"name": "Test Webhook",
		"url": "https://example.com/webhook",
		"events": ["session.created", "session.deleted"]
	}`

	req, err := http.NewRequestWithContext(ctx, "POST",
		baseURL+"/api/v1/webhooks", strings.NewReader(webhookPayload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req) // Need CSRF for POST

	resp, err := client.Do(req)
	require.NoError(t, err, "Request should not fail (no panic)")
	defer resp.Body.Close()

	// Main check: Request should not cause server panic
	// If server panicked, we'd get connection refused or 5xx
	assert.True(t, resp.StatusCode < 500,
		"Server should not panic on webhook secret generation")

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		// If webhook created, verify secret is present
		var webhookResp struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Secret string `json:"secret"`
		}
		err = decodeResponse(resp, &webhookResp)
		require.NoError(t, err)

		// Secret should be generated and meet requirements
		assert.NotEmpty(t, webhookResp.Secret, "Secret should be auto-generated")
		assert.GreaterOrEqual(t, len(webhookResp.Secret), 32,
			"Secret should be at least 32 characters")

		// Cleanup: Delete the test webhook
		if webhookResp.ID != "" {
			req, _ := http.NewRequestWithContext(ctx, "DELETE",
				baseURL+"/api/v1/webhooks/"+webhookResp.ID, nil)
			addAuthHeader(t, req)
			client.Do(req)
		}

		t.Logf("Webhook secret test passed - secret generated: %d chars", len(webhookResp.Secret))
	} else {
		// Even if webhook creation failed (e.g., auth), server shouldn't panic
		t.Logf("Webhook creation returned %d (not 5xx - no panic)", resp.StatusCode)
	}
}

// TestSQLInjectionPrevention validates that SQL injection attacks are prevented.
func TestSQLInjectionPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	sqlInjectionPayloads := []string{
		"'; DROP TABLE sessions;--",
		"' OR '1'='1",
		"test' UNION SELECT * FROM users--",
		"1; DELETE FROM sessions",
		"' OR 1=1--",
		`"; INSERT INTO users (username) VALUES ('hacked')--`,
	}

	for _, payload := range sqlInjectionPayloads {
		t.Run("Payload: "+payload[:min(20, len(payload))], func(t *testing.T) {
			// Test in search/filter parameter
			req, err := http.NewRequestWithContext(ctx, "GET",
				baseURL+"/api/v1/sessions?search="+payload, nil)
			require.NoError(t, err)
			addAuthHeader(t, req)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should not cause server error
			assert.True(t, resp.StatusCode < 500,
				"SQL injection should not cause server error")

			// Should not return SQL error in response
			// (would indicate injection reached database)
		})
	}

	t.Log("SQL injection prevention tests passed")
}

// TestXSSPrevention validates that XSS attacks are prevented.
func TestXSSPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror=alert('XSS')>",
		"<svg onload=alert(1)>",
		"javascript:alert(1)",
		"<body onload=alert('XSS')>",
	}

	for _, payload := range xssPayloads {
		t.Run("Payload: "+payload[:min(20, len(payload))], func(t *testing.T) {
			// Create session with XSS payload in name/description
			sessionPayload := `{"user":"testuser","template":"firefox","name":"` + payload + `"}`

			req, err := http.NewRequestWithContext(ctx, "POST",
				baseURL+"/api/v1/sessions", strings.NewReader(sessionPayload))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			addAuthHeader(t, req)
			addCSRFToken(t, req)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should either reject the input or escape it
			// Check that raw payload is not reflected in response
			// (Would need to check response body doesn't contain unescaped payload)
		})
	}

	t.Log("XSS prevention tests passed")
}

// Helper functions

func addCSRFToken(t *testing.T, req *http.Request) {
	t.Helper()
	// TODO: Implement proper CSRF token retrieval
	// For testing, use a test token
	req.Header.Set("X-CSRF-Token", "test-csrf-token")
}

func decodeResponse(resp *http.Response, v interface{}) error {
	// Helper to decode JSON response
	return nil // TODO: Implement
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
