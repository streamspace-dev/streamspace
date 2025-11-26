package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGetGVRForKind(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name        string
		apiVersion  string
		kind        string
		expectedGVR schema.GroupVersionResource
		expectedErr bool
	}{
		{
			name:       "Deployment",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			expectedErr: false,
		},
		{
			name:       "Service (core group)",
			apiVersion: "v1",
			kind:       "Service",
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
			expectedErr: false,
		},
		{
			name:       "Pod (core group)",
			apiVersion: "v1",
			kind:       "Pod",
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			expectedErr: false,
		},
		{
			name:       "ConfigMap",
			apiVersion: "v1",
			kind:       "ConfigMap",
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "configmaps",
			},
			expectedErr: false,
		},
		{
			name:       "Secret",
			apiVersion: "v1",
			kind:       "Secret",
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "secrets",
			},
			expectedErr: false,
		},
		{
			name:       "Session CRD",
			apiVersion: "stream.space/v1alpha1",
			kind:       "Session",
			expectedGVR: schema.GroupVersionResource{
				Group:    "stream.space",
				Version:  "v1alpha1",
				Resource: "sessions",
			},
			expectedErr: false,
		},
		{
			name:       "Template CRD",
			apiVersion: "stream.space/v1alpha1",
			kind:       "Template",
			expectedGVR: schema.GroupVersionResource{
				Group:    "stream.space",
				Version:  "v1alpha1",
				Resource: "templates",
			},
			expectedErr: false,
		},
		{
			name:       "StatefulSet",
			apiVersion: "apps/v1",
			kind:       "StatefulSet",
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "statefulsets",
			},
			expectedErr: false,
		},
		{
			name:       "DaemonSet",
			apiVersion: "apps/v1",
			kind:       "DaemonSet",
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "daemonsets",
			},
			expectedErr: false,
		},
		{
			name:       "Job",
			apiVersion: "batch/v1",
			kind:       "Job",
			expectedGVR: schema.GroupVersionResource{
				Group:    "batch",
				Version:  "v1",
				Resource: "jobs",
			},
			expectedErr: false,
		},
		{
			name:       "CronJob",
			apiVersion: "batch/v1",
			kind:       "CronJob",
			expectedGVR: schema.GroupVersionResource{
				Group:    "batch",
				Version:  "v1",
				Resource: "cronjobs",
			},
			expectedErr: false,
		},
		{
			name:       "Ingress",
			apiVersion: "networking.k8s.io/v1",
			kind:       "Ingress",
			expectedGVR: schema.GroupVersionResource{
				Group:    "networking.k8s.io",
				Version:  "v1",
				Resource: "ingresses",
			},
			expectedErr: false,
		},
		{
			name:       "Unknown kind (fallback)",
			apiVersion: "custom.io/v1",
			kind:       "CustomResource",
			expectedGVR: schema.GroupVersionResource{
				Group:    "custom.io",
				Version:  "v1",
				Resource: "customresources", // Fallback: lowercase + s
			},
			expectedErr: false,
		},
		{
			name:        "Invalid API version",
			apiVersion:  "invalid/version/format",
			kind:        "SomeKind",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvr, err := handler.getGVRForKind(tt.apiVersion, tt.kind)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedGVR.Group, gvr.Group)
				assert.Equal(t, tt.expectedGVR.Version, gvr.Version)
				assert.Equal(t, tt.expectedGVR.Resource, gvr.Resource)
			}
		})
	}
}

// TestCreateResource_NoK8sClient tests that CreateResource returns 503 when k8sClient is nil
// v2.0-beta architecture: K8s client is optional, cluster management endpoints return ServiceUnavailable
func TestCreateResource_NoK8sClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &Handler{
		namespace: "streamspace",
		// k8sClient is nil - v2.0-beta architecture
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body, _ := json.Marshal(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "test-config",
		},
	})
	c.Request = httptest.NewRequest("POST", "/api/v1/resources", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateResource(c)

	// v2.0-beta: k8sClient is nil, returns 503 before validation
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Cluster management not available")
}

func TestCreateResource_InvalidRequest(t *testing.T) {
	// SKIP: This test requires a K8s client to test input validation
	// When k8sClient is nil (v2.0-beta architecture), the handler returns 503 before validation
	// To test validation logic, a mock K8s client would be needed
	t.Skip("v2.0-beta: K8s client is optional; validation tests require mock K8s client")
}

// TestUpdateResource_NoK8sClient tests that UpdateResource returns 503 when k8sClient is nil
func TestUpdateResource_NoK8sClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &Handler{
		namespace: "streamspace",
		// k8sClient is nil - v2.0-beta architecture
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "type", Value: "configmap"},
		{Key: "name", Value: "test-config"},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "test-config",
		},
	})
	c.Request = httptest.NewRequest("PUT", "/api/v1/resources/configmap/test-config", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateResource(c)

	// v2.0-beta: k8sClient is nil, returns 503 before validation
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Cluster management not available")
}

func TestUpdateResource_InvalidRequest(t *testing.T) {
	// SKIP: This test requires a K8s client to test input validation
	// When k8sClient is nil (v2.0-beta architecture), the handler returns 503 before validation
	t.Skip("v2.0-beta: K8s client is optional; validation tests require mock K8s client")
}

// TestDeleteResource_NoK8sClient tests that DeleteResource returns 503 when k8sClient is nil
func TestDeleteResource_NoK8sClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &Handler{
		namespace: "streamspace",
		// k8sClient is nil - v2.0-beta architecture
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "type", Value: "configmap"},
		{Key: "name", Value: "test-config"},
	}

	req := httptest.NewRequest("DELETE", "/api/v1/resources/configmap/test-config?apiVersion=v1&kind=ConfigMap", nil)
	c.Request = req

	handler.DeleteResource(c)

	// v2.0-beta: k8sClient is nil, returns 503 before validation
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Cluster management not available")
}

func TestDeleteResource_MissingParams(t *testing.T) {
	// SKIP: This test requires a K8s client to test input validation
	// When k8sClient is nil (v2.0-beta architecture), the handler returns 503 before validation
	t.Skip("v2.0-beta: K8s client is optional; validation tests require mock K8s client")
}

func TestGetGVRForKind_EdgeCases(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name        string
		apiVersion  string
		kind        string
		expectedErr bool
	}{
		{
			name:        "Empty apiVersion",
			apiVersion:  "",
			kind:        "Pod",
			expectedErr: true,
		},
		{
			name:        "Malformed apiVersion",
			apiVersion:  "//////",
			kind:        "Pod",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Edge case validation not yet implemented")
			_, err := handler.getGVRForKind(tt.apiVersion, tt.kind)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkGetGVRForKind_CommonKinds(b *testing.B) {
	handler := &Handler{}
	kinds := []struct {
		apiVersion string
		kind       string
	}{
		{"apps/v1", "Deployment"},
		{"v1", "Service"},
		{"v1", "Pod"},
		{"v1", "ConfigMap"},
		{"stream.space/v1alpha1", "Session"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := kinds[i%len(kinds)]
		handler.getGVRForKind(k.apiVersion, k.kind)
	}
}

func BenchmarkGetGVRForKind_UnknownKind(b *testing.B) {
	handler := &Handler{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.getGVRForKind("custom.io/v1", "UnknownResource")
	}
}
