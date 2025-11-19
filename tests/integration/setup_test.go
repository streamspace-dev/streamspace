// Package integration provides integration tests for StreamSpace.
// These tests verify API endpoint functionality for Phase 5.5 fixes.
package integration

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"testing"
	"time"
)

var (
	testClient *http.Client
	apiBaseURL string
)

// TestMain sets up the test environment for integration tests.
func TestMain(m *testing.M) {
	// Set up HTTP client for API testing
	testClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // For self-signed certs in test
			},
		},
	}

	// Get API base URL from environment or use default
	apiBaseURL = os.Getenv("STREAMSPACE_API_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080"
	}

	code := m.Run()
	os.Exit(code)
}

// Helper functions for integration tests

// setupTestHTTPClient returns a configured HTTP client for testing.
func setupTestHTTPClient(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

// getAPIBaseURL returns the base URL for API requests.
func getAPIBaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("STREAMSPACE_API_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	return url
}

// addAuthHeader adds authentication header to request.
func addAuthHeader(t *testing.T, req *http.Request) {
	t.Helper()
	// Use test auth token from environment or default test token
	token := os.Getenv("STREAMSPACE_TEST_TOKEN")
	if token == "" {
		token = "test-auth-token"
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

// waitForCondition waits for a condition to be true with timeout.
func waitForCondition(timeout time.Duration, interval time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// getTestContext returns a context with timeout for tests.
func getTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

// Common request/response types for API testing

// CreateSessionRequest represents a session creation request.
type CreateSessionRequest struct {
	User          string                 `json:"user,omitempty"`
	Template      string                 `json:"template,omitempty"`
	ApplicationID string                 `json:"applicationId,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Resources     map[string]interface{} `json:"resources,omitempty"`
}

// SessionResponse represents a session in API responses.
type SessionResponse struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	User       string                 `json:"user"`
	Template   string                 `json:"template"`
	Phase      string                 `json:"phase"`
	Status     string                 `json:"status"`
	URL        string                 `json:"url"`
	Resources  map[string]interface{} `json:"resources"`
	CreatedAt  string                 `json:"createdAt"`
	ModifiedAt string                 `json:"modifiedAt"`
}

// SessionListResponse represents the API response for listing sessions.
type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Total    int               `json:"total"`
}

// ConnectResponse represents the API response for session connection.
type ConnectResponse struct {
	URL          string `json:"url"`
	ConnectionID string `json:"connectionId"`
}

// ConnectionResponse represents connection details.
type ConnectionResponse struct {
	SessionID    string `json:"sessionId"`
	ConnectionID string `json:"connectionId"`
	URL          string `json:"url"`
}

// PluginResponse represents a plugin in API responses.
type PluginResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Enabled     bool                   `json:"enabled"`
	Installed   bool                   `json:"installed"`
	Config      map[string]interface{} `json:"config"`
	InstalledAt string                 `json:"installedAt"`
}

// PluginListResponse represents a list of plugins.
type PluginListResponse struct {
	Plugins []PluginResponse `json:"plugins"`
	Total   int              `json:"total"`
}
