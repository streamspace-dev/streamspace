// Package integration provides integration tests for StreamSpace.
// These tests validate core platform functionality including session creation,
// template resolution, and VNC connectivity.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: fmt was removed as it's not used in the simplified tests

// TestSessionNameInAPIResponse validates that the API returns session name,
// not database ID (TC-CORE-001).
//
// Related Issue: Session Name/ID Mismatch - API returns database ID instead of session name
// Impact: UI cannot find sessions, SessionViewer fails, all session navigation broken
func TestSessionNameInAPIResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Setup test client
	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Step 1: Create a session
	createReq := CreateSessionRequest{
		User:     "testuser",
		Template: "firefox-browser",
		Resources: map[string]interface{}{
			"memory": "2Gi",
			"cpu":    "1000m",
		},
	}

	body, err := json.Marshal(createReq)
	require.NoError(t, err, "Failed to marshal create request")

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to create session")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected 201 Created")

	var createResp SessionResponse
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	require.NoError(t, err, "Failed to decode create response")

	// CRITICAL CHECK: Response must include name field, not just ID
	assert.NotEmpty(t, createResp.Name, "Session name must not be empty")
	assert.NotContains(t, createResp.Name, "-", "Session name should not be a UUID (contains dashes)")

	// Store the created session name for later tests
	createdName := createResp.Name

	// Step 2: List sessions and verify name field
	req, err = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions", nil)
	require.NoError(t, err, "Failed to create list request")
	addAuthHeader(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err, "Failed to list sessions")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

	var listResp SessionListResponse
	err = json.NewDecoder(resp.Body).Decode(&listResp)
	require.NoError(t, err, "Failed to decode list response")

	// Find our session in the list
	var foundSession *SessionResponse
	for i, s := range listResp.Sessions {
		if s.Name == createdName {
			foundSession = &listResp.Sessions[i]
			break
		}
	}

	require.NotNil(t, foundSession, "Created session not found in list by name")

	// CRITICAL CHECK: Name field must match what we expect
	assert.Equal(t, createdName, foundSession.Name, "Session name mismatch in list response")

	// Step 3: Get single session by name
	req, err = http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+createdName, nil)
	require.NoError(t, err, "Failed to create get request")
	addAuthHeader(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err, "Failed to get session")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK for get by name")

	var getResp SessionResponse
	err = json.NewDecoder(resp.Body).Decode(&getResp)
	require.NoError(t, err, "Failed to decode get response")

	assert.Equal(t, createdName, getResp.Name, "Session name mismatch in get response")

	// Cleanup: Delete the test session
	req, err = http.NewRequestWithContext(ctx, "DELETE", baseURL+"/api/v1/sessions/"+createdName, nil)
	require.NoError(t, err, "Failed to create delete request")
	addAuthHeader(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err, "Failed to delete session")
	resp.Body.Close()

	t.Logf("Session name test passed - API correctly returns name: %s", createdName)
}

// TestTemplateNameUsedInSessionCreation validates that the resolved template name
// is used when creating sessions via applicationId (TC-CORE-002).
//
// Related Issue: Template Name Not Used - uses req.Template instead of resolved templateName
// Impact: Sessions created with wrong/empty template names, controller can't find template
func TestTemplateNameUsedInSessionCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Step 1: Get application ID for Firefox
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/applications", nil)
	require.NoError(t, err, "Failed to create applications request")
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to get applications")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK for applications list")

	var appsResp struct {
		Applications []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			TemplateName string `json:"templateName"`
		} `json:"applications"`
	}
	err = json.NewDecoder(resp.Body).Decode(&appsResp)
	require.NoError(t, err, "Failed to decode applications response")

	// Find Firefox application
	var firefoxAppID string
	var expectedTemplate string
	for _, app := range appsResp.Applications {
		if app.Name == "Firefox" || app.TemplateName == "firefox-browser" {
			firefoxAppID = app.ID
			expectedTemplate = app.TemplateName
			break
		}
	}

	require.NotEmpty(t, firefoxAppID, "Firefox application not found")
	require.NotEmpty(t, expectedTemplate, "Firefox template name not found")

	// Step 2: Create session using applicationId (not template name directly)
	createReq := CreateSessionRequest{
		User:          "testuser",
		ApplicationID: firefoxAppID,
		// Note: Template field is intentionally empty - should be resolved from applicationId
		Resources: map[string]interface{}{
			"memory": "2Gi",
			"cpu":    "1000m",
		},
	}

	body, err := json.Marshal(createReq)
	require.NoError(t, err, "Failed to marshal create request")

	req, err = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err, "Failed to create session")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected 201 Created")

	var createResp SessionResponse
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	require.NoError(t, err, "Failed to decode create response")

	// CRITICAL CHECK: Template field must be the resolved template name, not empty
	assert.NotEmpty(t, createResp.Template, "Template name must not be empty")
	assert.Equal(t, expectedTemplate, createResp.Template,
		"Template name should be resolved from applicationId")

	// Verify template is not the applicationId itself
	assert.NotEqual(t, firefoxAppID, createResp.Template,
		"Template should not be the applicationId - should be resolved template name")

	// Step 3: Wait for session to reach Running state (verifies controller can find template)
	sessionName := createResp.Name
	running := waitForCondition(60*time.Second, 2*time.Second, func() bool {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+sessionName, nil)
		addAuthHeader(t, req)
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		var s SessionResponse
		json.NewDecoder(resp.Body).Decode(&s)
		return s.Phase == "Running" || s.Status == "Running"
	})

	assert.True(t, running, "Session should reach Running state - controller must find template")

	// Cleanup
	req, _ = http.NewRequestWithContext(ctx, "DELETE", baseURL+"/api/v1/sessions/"+sessionName, nil)
	addAuthHeader(t, req)
	client.Do(req)

	t.Logf("Template name test passed - Session created with template: %s", createResp.Template)
}

// TestVNCURLAvailableOnConnection validates that the VNC URL is available
// when connecting to a session (TC-CORE-004).
//
// Related Issue: VNC URL Empty When Connecting - session.Status.URL may be empty
// Impact: Session viewer shows blank iframe, users cannot see session
func TestVNCURLAvailableOnConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Step 1: Create a session
	createReq := CreateSessionRequest{
		User:     "testuser",
		Template: "firefox-browser",
		Resources: map[string]interface{}{
			"memory": "2Gi",
			"cpu":    "1000m",
		},
	}

	body, err := json.Marshal(createReq)
	require.NoError(t, err, "Failed to marshal create request")

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
	require.NoError(t, err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to create session")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected 201 Created")

	var createResp SessionResponse
	err = json.NewDecoder(resp.Body).Decode(&createResp)
	require.NoError(t, err, "Failed to decode create response")

	sessionName := createResp.Name

	// Step 2: Connect to session (may need to wait/poll for URL)
	var connectResp ConnectResponse
	connected := waitForCondition(90*time.Second, 3*time.Second, func() bool {
		req, _ := http.NewRequestWithContext(ctx, "POST",
			baseURL+"/api/v1/sessions/"+sessionName+"/connect", nil)
		addAuthHeader(t, req)

		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		err = json.NewDecoder(resp.Body).Decode(&connectResp)
		if err != nil {
			return false
		}

		// CRITICAL CHECK: URL must not be empty
		return connectResp.URL != ""
	})

	// Assertions
	assert.True(t, connected, "Should be able to connect to session with non-empty URL")
	assert.NotEmpty(t, connectResp.URL, "VNC URL must not be empty")
	assert.NotEmpty(t, connectResp.ConnectionID, "Connection ID must not be empty")

	// Verify URL format
	assert.Contains(t, connectResp.URL, "http", "URL should be a valid HTTP(S) URL")

	// Step 3: Verify URL is accessible (basic check)
	if connectResp.URL != "" {
		urlReq, err := http.NewRequestWithContext(ctx, "HEAD", connectResp.URL, nil)
		if err == nil {
			urlResp, err := client.Do(urlReq)
			if err == nil {
				urlResp.Body.Close()
				// We just check it's reachable, actual VNC content is tested elsewhere
				t.Logf("VNC URL reachable: %s (status: %d)", connectResp.URL, urlResp.StatusCode)
			}
		}
	}

	// Cleanup
	req, _ = http.NewRequestWithContext(ctx, "DELETE", baseURL+"/api/v1/sessions/"+sessionName, nil)
	addAuthHeader(t, req)
	client.Do(req)

	t.Logf("VNC URL test passed - URL: %s, ConnectionID: %s", connectResp.URL, connectResp.ConnectionID)
}

// TestHeartbeatValidatesConnection validates that heartbeat validates
// connection ownership (TC-CORE-005).
//
// Related Issue: Heartbeat Has No Connection Validation
// Impact: Auto-hibernation never triggers, resource leaks
func TestHeartbeatValidatesConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Create two sessions to test cross-session validation
	sessions := make([]string, 2)
	connections := make([]string, 2)

	for i := 0; i < 2; i++ {
		// Create session
		createReq := CreateSessionRequest{
			User:     "testuser",
			Template: "firefox-browser",
		}
		body, _ := json.Marshal(createReq)
		req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(t, req)
		resp, err := client.Do(req)
		require.NoError(t, err)

		var createResp SessionResponse
		json.NewDecoder(resp.Body).Decode(&createResp)
		resp.Body.Close()
		sessions[i] = createResp.Name

		// Wait for running and connect
		waitForCondition(60*time.Second, 2*time.Second, func() bool {
			req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+sessions[i], nil)
			addAuthHeader(t, req)
			resp, _ := client.Do(req)
			var s SessionResponse
			json.NewDecoder(resp.Body).Decode(&s)
			resp.Body.Close()
			return s.Phase == "Running"
		})

		// Connect
		req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/"+sessions[i]+"/connect", nil)
		addAuthHeader(t, req)
		resp, _ = client.Do(req)
		var connectResp ConnectResponse
		json.NewDecoder(resp.Body).Decode(&connectResp)
		resp.Body.Close()
		connections[i] = connectResp.ConnectionID
	}

	// Test 1: Valid heartbeat (correct session and connectionId)
	heartbeatReq := map[string]string{"connectionId": connections[0]}
	body, _ := json.Marshal(heartbeatReq)
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/"+sessions[0]+"/heartbeat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Valid heartbeat should succeed")
	resp.Body.Close()

	// Test 2: Invalid heartbeat (wrong session's connectionId)
	heartbeatReq = map[string]string{"connectionId": connections[1]} // Wrong connection
	body, _ = json.Marshal(heartbeatReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/"+sessions[0]+"/heartbeat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	resp, _ = client.Do(req)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Heartbeat with wrong session's connectionId should be rejected")
	resp.Body.Close()

	// Test 3: Invalid heartbeat (nonexistent connectionId)
	heartbeatReq = map[string]string{"connectionId": "invalid-connection-id"}
	body, _ = json.Marshal(heartbeatReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/"+sessions[0]+"/heartbeat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	resp, _ = client.Do(req)
	assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden,
		"Heartbeat with invalid connectionId should be rejected")
	resp.Body.Close()

	// Cleanup
	for _, s := range sessions {
		req, _ := http.NewRequestWithContext(ctx, "DELETE", baseURL+"/api/v1/sessions/"+s, nil)
		addAuthHeader(t, req)
		client.Do(req)
	}

	t.Log("Heartbeat validation test passed")
}
