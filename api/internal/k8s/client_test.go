package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestGVRConstants(t *testing.T) {
	// Verify GVR constants are correctly defined
	assert.Equal(t, "stream.space", sessionGVR.Group)
	assert.Equal(t, "v1alpha1", sessionGVR.Version)
	assert.Equal(t, "sessions", sessionGVR.Resource)

	assert.Equal(t, "stream.space", templateGVR.Group)
	assert.Equal(t, "v1alpha1", templateGVR.Version)
	assert.Equal(t, "templates", templateGVR.Resource)

	assert.Equal(t, "stream.space", applicationInstallGVR.Group)
	assert.Equal(t, "v1alpha1", applicationInstallGVR.Version)
	assert.Equal(t, "applicationinstalls", applicationInstallGVR.Resource)
}

func TestCreateSession_Success(t *testing.T) {
	// Create fake dynamic client
	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme)

	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	session := &Session{
		Name:           "test-session",
		Namespace:      "streamspace",
		User:           "user1",
		Template:       "ubuntu-desktop",
		State:          "running",
		PersistentHome: true,
		IdleTimeout:    "30m",
	}
	session.Resources.Memory = "4Gi"
	session.Resources.CPU = "2000m"

	created, err := client.CreateSession(ctx, session)

	require.NoError(t, err)
	assert.NotNil(t, created)
	assert.Equal(t, "test-session", created.Name)
	assert.Equal(t, "user1", created.User)
	assert.Equal(t, "ubuntu-desktop", created.Template)
	assert.Equal(t, "running", created.State)
}

func TestGetSession_Success(t *testing.T) {
	// Pre-create a session in fake client
	sessionObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Session",
			"metadata": map[string]interface{}{
				"name":      "existing-session",
				"namespace": "streamspace",
			},
			"spec": map[string]interface{}{
				"user":           "user1",
				"template":       "firefox",
				"state":          "running",
				"persistentHome": true,
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, sessionObj)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	session, err := client.GetSession(ctx, "streamspace", "existing-session")

	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "existing-session", session.Name)
	assert.Equal(t, "user1", session.User)
	assert.Equal(t, "firefox", session.Template)
}

func TestListSessions_Success(t *testing.T) {
	// Create multiple sessions
	sessions := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "stream.space/v1alpha1",
				"kind":       "Session",
				"metadata": map[string]interface{}{
					"name":      "session-1",
					"namespace": "streamspace",
				},
				"spec": map[string]interface{}{
					"user":     "user1",
					"template": "ubuntu",
					"state":    "running",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "stream.space/v1alpha1",
				"kind":       "Session",
				"metadata": map[string]interface{}{
					"name":      "session-2",
					"namespace": "streamspace",
				},
				"spec": map[string]interface{}{
					"user":     "user2",
					"template": "debian",
					"state":    "running",
				},
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, sessions...)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	list, err := client.ListSessions(ctx, "streamspace")

	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "session-1", list[0].Name)
	assert.Equal(t, "session-2", list[1].Name)
}

func TestUpdateSessionState_Success(t *testing.T) {
	sessionObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Session",
			"metadata": map[string]interface{}{
				"name":      "test-session",
				"namespace": "streamspace",
			},
			"spec": map[string]interface{}{
				"user":     "user1",
				"template": "ubuntu",
				"state":    "running",
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, sessionObj)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	updated, err := client.UpdateSessionState(ctx, "streamspace", "test-session", "hibernated")

	require.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, "hibernated", updated.State)
}

func TestDeleteSession_Success(t *testing.T) {
	sessionObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Session",
			"metadata": map[string]interface{}{
				"name":      "test-session",
				"namespace": "streamspace",
			},
			"spec": map[string]interface{}{
				"user":  "user1",
				"state": "running",
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, sessionObj)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	err := client.DeleteSession(ctx, "streamspace", "test-session")

	assert.NoError(t, err)

	// Verify it was deleted
	_, err = client.GetSession(ctx, "streamspace", "test-session")
	assert.Error(t, err)
}

func TestCreateTemplate_Success(t *testing.T) {
	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	template := &Template{
		Name:        "firefox-template",
		Namespace:   "streamspace",
		DisplayName: "Firefox Browser",
		Description: "Web browser",
		Category:    "browsers",
		BaseImage:   "firefox:latest",
		AppType:     "desktop",
	}

	created, err := client.CreateTemplate(ctx, template)

	require.NoError(t, err)
	assert.NotNil(t, created)
	assert.Equal(t, "firefox-template", created.Name)
	assert.Equal(t, "Firefox Browser", created.DisplayName)
}

func TestGetTemplate_Success(t *testing.T) {
	templateObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Template",
			"metadata": map[string]interface{}{
				"name":      "vscode-template",
				"namespace": "streamspace",
			},
			"spec": map[string]interface{}{
				"displayName": "VS Code",
				"description": "Code editor",
				"category":    "development",
				"baseImage":   "vscode:latest",
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, templateObj)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	template, err := client.GetTemplate(ctx, "streamspace", "vscode-template")

	require.NoError(t, err)
	assert.NotNil(t, template)
	assert.Equal(t, "vscode-template", template.Name)
	assert.Equal(t, "VS Code", template.DisplayName)
	assert.Equal(t, "development", template.Category)
}

func TestListTemplates_Success(t *testing.T) {
	templates := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "stream.space/v1alpha1",
				"kind":       "Template",
				"metadata": map[string]interface{}{
					"name":      "firefox",
					"namespace": "streamspace",
				},
				"spec": map[string]interface{}{
					"displayName": "Firefox",
					"category":    "browsers",
					"baseImage":   "firefox:latest",
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "stream.space/v1alpha1",
				"kind":       "Template",
				"metadata": map[string]interface{}{
					"name":      "chrome",
					"namespace": "streamspace",
				},
				"spec": map[string]interface{}{
					"displayName": "Chrome",
					"category":    "browsers",
					"baseImage":   "chrome:latest",
				},
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, templates...)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	list, err := client.ListTemplates(ctx, "streamspace")

	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestDeleteTemplate_Success(t *testing.T) {
	templateObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Template",
			"metadata": map[string]interface{}{
				"name":      "test-template",
				"namespace": "streamspace",
			},
			"spec": map[string]interface{}{
				"displayName": "Test",
				"baseImage":   "test:latest",
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, templateObj)
	client := &Client{
		dynamicClient: dynClient,
		namespace:     "streamspace",
	}

	ctx := context.Background()
	err := client.DeleteTemplate(ctx, "streamspace", "test-template")

	assert.NoError(t, err)

	// Verify deletion
	_, err = client.GetTemplate(ctx, "streamspace", "test-template")
	assert.Error(t, err)
}

func TestParseSession_WithStatus(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Session",
			"metadata": map[string]interface{}{
				"name":              "test-session",
				"namespace":         "streamspace",
				"creationTimestamp": "2024-01-01T00:00:00Z",
			},
			"spec": map[string]interface{}{
				"user":           "user1",
				"template":       "ubuntu",
				"state":          "running",
				"persistentHome": true,
				"idleTimeout":    "30m",
				"resources": map[string]interface{}{
					"memory": "4Gi",
					"cpu":    "2000m",
				},
				"tags": []interface{}{"dev", "test"},
			},
			"status": map[string]interface{}{
				"phase":   "Running",
				"podName": "session-pod-123",
				"url":     "https://session.example.com",
			},
		},
	}

	session, err := parseSession(obj)

	require.NoError(t, err)
	assert.Equal(t, "test-session", session.Name)
	assert.Equal(t, "user1", session.User)
	assert.Equal(t, "ubuntu", session.Template)
	assert.Equal(t, "running", session.State)
	assert.True(t, session.PersistentHome)
	assert.Equal(t, "30m", session.IdleTimeout)
	assert.Equal(t, "4Gi", session.Resources.Memory)
	assert.Equal(t, "2000m", session.Resources.CPU)
	assert.Len(t, session.Tags, 2)
	assert.Equal(t, "Running", session.Status.Phase)
	assert.Equal(t, "session-pod-123", session.Status.PodName)
	assert.Equal(t, "https://session.example.com", session.Status.URL)
}
