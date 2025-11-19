// Package integration provides integration tests for StreamSpace.
// These tests validate batch operations including hibernate, wake, and delete
// with proper error collection.
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

// BatchResponse represents the response from batch operations
type BatchResponse struct {
	Total     int              `json:"total"`
	Succeeded int              `json:"succeeded"`
	Failed    int              `json:"failed"`
	Errors    []BatchError     `json:"errors"`
	Results   []BatchResult    `json:"results,omitempty"`
}

// BatchError represents an error from a batch operation
type BatchError struct {
	Name    string `json:"name"`
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// BatchResult represents a single result in a batch operation
type BatchResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Success bool   `json:"success"`
}

// TestBatchHibernate validates batch hibernation with error collection (TC-INT-001).
//
// Related Issue: Batch Operations Error Collection - errors not collected in array
// Impact: Users can't see what failed in batch operations
func TestBatchHibernate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Create multiple test sessions
	sessionNames := make([]string, 5)
	for i := 0; i < 5; i++ {
		createReq := CreateSessionRequest{
			User:     "testuser",
			Template: "firefox-browser",
		}
		body, _ := json.Marshal(createReq)
		req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(t, req)
		addCSRFToken(t, req)

		resp, err := client.Do(req)
		require.NoError(t, err)

		var createResp SessionResponse
		json.NewDecoder(resp.Body).Decode(&createResp)
		resp.Body.Close()

		sessionNames[i] = createResp.Name

		// Wait for running
		waitForCondition(60*time.Second, 2*time.Second, func() bool {
			req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+createResp.Name, nil)
			addAuthHeader(t, req)
			resp, _ := client.Do(req)
			var s SessionResponse
			json.NewDecoder(resp.Body).Decode(&s)
			resp.Body.Close()
			return s.Phase == "Running"
		})
	}

	// Batch hibernate all sessions
	batchReq := map[string][]string{"sessions": sessionNames}
	body, _ := json.Marshal(batchReq)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/batch/hibernate", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Batch hibernate should return 200")

	var batchResp BatchResponse
	err = json.NewDecoder(resp.Body).Decode(&batchResp)
	require.NoError(t, err)

	// Verify response structure
	assert.Equal(t, 5, batchResp.Total, "Total should match number of sessions")
	assert.Equal(t, batchResp.Succeeded+batchResp.Failed, batchResp.Total, "Succeeded + Failed should equal Total")

	// If there were failures, errors array should be populated
	if batchResp.Failed > 0 {
		assert.Len(t, batchResp.Errors, batchResp.Failed,
			"Errors array should contain details for all failures")
		for _, err := range batchResp.Errors {
			assert.NotEmpty(t, err.Name, "Error should include session name")
			assert.NotEmpty(t, err.Error, "Error should include error message")
		}
	}

	// Verify sessions are actually hibernated
	for _, name := range sessionNames {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+name, nil)
		addAuthHeader(t, req)
		resp, _ := client.Do(req)
		var s SessionResponse
		json.NewDecoder(resp.Body).Decode(&s)
		resp.Body.Close()

		if batchResp.Succeeded == 5 {
			assert.Equal(t, "Hibernated", s.Phase, "Session should be hibernated")
		}
	}

	// Cleanup
	for _, name := range sessionNames {
		req, _ := http.NewRequestWithContext(ctx, "DELETE", baseURL+"/api/v1/sessions/"+name, nil)
		addAuthHeader(t, req)
		client.Do(req)
	}

	t.Logf("Batch hibernate test passed: %d/%d succeeded", batchResp.Succeeded, batchResp.Total)
}

// TestBatchWake validates batch wake operation with error collection (TC-INT-003).
func TestBatchWake(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Create and hibernate sessions
	sessionNames := make([]string, 3)
	for i := 0; i < 3; i++ {
		createReq := CreateSessionRequest{
			User:     "testuser",
			Template: "firefox-browser",
		}
		body, _ := json.Marshal(createReq)
		req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(t, req)
		addCSRFToken(t, req)

		resp, _ := client.Do(req)
		var createResp SessionResponse
		json.NewDecoder(resp.Body).Decode(&createResp)
		resp.Body.Close()

		sessionNames[i] = createResp.Name

		// Wait for running then hibernate
		waitForCondition(60*time.Second, 2*time.Second, func() bool {
			req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+createResp.Name, nil)
			addAuthHeader(t, req)
			resp, _ := client.Do(req)
			var s SessionResponse
			json.NewDecoder(resp.Body).Decode(&s)
			resp.Body.Close()
			return s.Phase == "Running"
		})

		// Hibernate
		req, _ = http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/"+createResp.Name+"/hibernate", nil)
		addAuthHeader(t, req)
		addCSRFToken(t, req)
		client.Do(req)
	}

	// Wait for all to be hibernated
	time.Sleep(5 * time.Second)

	// Batch wake all sessions
	batchReq := map[string][]string{"sessions": sessionNames}
	body, _ := json.Marshal(batchReq)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/batch/wake", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var batchResp BatchResponse
	json.NewDecoder(resp.Body).Decode(&batchResp)

	assert.Equal(t, 3, batchResp.Total)
	assert.Equal(t, batchResp.Succeeded+batchResp.Failed, batchResp.Total)

	// Cleanup
	for _, name := range sessionNames {
		req, _ := http.NewRequestWithContext(ctx, "DELETE", baseURL+"/api/v1/sessions/"+name, nil)
		addAuthHeader(t, req)
		client.Do(req)
	}

	t.Logf("Batch wake test passed: %d/%d succeeded", batchResp.Succeeded, batchResp.Total)
}

// TestBatchDelete validates batch deletion with error collection (TC-INT-002).
func TestBatchDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Create multiple sessions
	sessionNames := make([]string, 3)
	for i := 0; i < 3; i++ {
		createReq := CreateSessionRequest{
			User:     "testuser",
			Template: "firefox-browser",
		}
		body, _ := json.Marshal(createReq)
		req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		addAuthHeader(t, req)
		addCSRFToken(t, req)

		resp, _ := client.Do(req)
		var createResp SessionResponse
		json.NewDecoder(resp.Body).Decode(&createResp)
		resp.Body.Close()

		sessionNames[i] = createResp.Name
	}

	// Batch delete all sessions
	batchReq := map[string][]string{"sessions": sessionNames}
	body, _ := json.Marshal(batchReq)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/batch/delete", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var batchResp BatchResponse
	json.NewDecoder(resp.Body).Decode(&batchResp)

	assert.Equal(t, 3, batchResp.Total)
	assert.Equal(t, batchResp.Succeeded+batchResp.Failed, batchResp.Total)

	// Verify sessions are deleted
	for _, name := range sessionNames {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+name, nil)
		addAuthHeader(t, req)
		resp, _ := client.Do(req)

		// Should return 404 for deleted sessions
		if batchResp.Succeeded == 3 {
			assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Deleted session should return 404")
		}
		resp.Body.Close()
	}

	t.Logf("Batch delete test passed: %d/%d succeeded", batchResp.Succeeded, batchResp.Total)
}

// TestBatchPartialFailure validates that partial failures are properly reported (TC-INT-004).
func TestBatchPartialFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Create one valid session
	createReq := CreateSessionRequest{
		User:     "testuser",
		Template: "firefox-browser",
	}
	body, _ := json.Marshal(createReq)
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, _ := client.Do(req)
	var createResp SessionResponse
	json.NewDecoder(resp.Body).Decode(&createResp)
	resp.Body.Close()

	validSession := createResp.Name

	// Wait for running
	waitForCondition(60*time.Second, 2*time.Second, func() bool {
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/sessions/"+validSession, nil)
		addAuthHeader(t, req)
		resp, _ := client.Do(req)
		var s SessionResponse
		json.NewDecoder(resp.Body).Decode(&s)
		resp.Body.Close()
		return s.Phase == "Running"
	})

	// Batch operation with mix of valid and invalid sessions
	sessionNames := []string{
		validSession,
		"nonexistent-session-1",
		"nonexistent-session-2",
	}

	batchReq := map[string][]string{"sessions": sessionNames}
	body, _ = json.Marshal(batchReq)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/batch/hibernate", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should still return 200 for partial success
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Partial failure should still return 200")

	var batchResp BatchResponse
	json.NewDecoder(resp.Body).Decode(&batchResp)

	// Verify counts
	assert.Equal(t, 3, batchResp.Total, "Total should be 3")
	assert.GreaterOrEqual(t, batchResp.Succeeded, 1, "At least one should succeed")
	assert.GreaterOrEqual(t, batchResp.Failed, 2, "At least two should fail")

	// CRITICAL CHECK: Errors array should be populated
	assert.Len(t, batchResp.Errors, batchResp.Failed,
		"Errors array must contain all failures - this is the bug being tested!")

	// Verify error details
	for _, err := range batchResp.Errors {
		assert.NotEmpty(t, err.Name, "Each error must have session name")
		assert.True(t, err.Name == "nonexistent-session-1" || err.Name == "nonexistent-session-2",
			"Errors should be for nonexistent sessions")
	}

	// Cleanup
	req, _ = http.NewRequestWithContext(ctx, "DELETE", baseURL+"/api/v1/sessions/"+validSession, nil)
	addAuthHeader(t, req)
	client.Do(req)

	t.Logf("Batch partial failure test passed: %d succeeded, %d failed, %d errors reported",
		batchResp.Succeeded, batchResp.Failed, len(batchResp.Errors))
}

// TestBatchEmptyRequest validates handling of empty batch requests.
func TestBatchEmptyRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := setupTestHTTPClient(t)
	baseURL := getAPIBaseURL(t)

	// Empty sessions array
	batchReq := map[string][]string{"sessions": []string{}}
	body, _ := json.Marshal(batchReq)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/sessions/batch/hibernate", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(t, req)
	addCSRFToken(t, req)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 Bad Request for empty input
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusOK,
		"Empty batch should return 400 or 200 with 0 results")

	t.Log("Batch empty request test passed")
}
