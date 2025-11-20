package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test event type marshaling
func TestSessionCreateEvent_JSONMarshaling(t *testing.T) {
	event := &SessionCreateEvent{
		EventID:    uuid.New().String(),
		Timestamp:  time.Now(),
		SessionID:  "session123",
		UserID:     "user456",
		TemplateID: "template789",
		Platform:   PlatformKubernetes,
		Resources: ResourceSpec{
			Memory: "4Gi",
			CPU:    "2000m",
		},
		PersistentHome: true,
		IdleTimeout:    "3600",
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal back
	var decoded SessionCreateEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify critical fields
	assert.Equal(t, event.SessionID, decoded.SessionID)
	assert.Equal(t, event.UserID, decoded.UserID)
	assert.Equal(t, event.Platform, decoded.Platform)
	assert.Equal(t, event.Resources.Memory, decoded.Resources.Memory)
}

func TestSessionDeleteEvent_JSONMarshaling(t *testing.T) {
	event := &SessionDeleteEvent{
		EventID:   uuid.New().String(),
		Timestamp: time.Now(),
		SessionID: "session123",
		UserID:    "user456",
		Platform:  PlatformKubernetes,
		Force:     true,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded SessionDeleteEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.SessionID, decoded.SessionID)
	assert.Equal(t, event.Force, decoded.Force)
}

func TestAppInstallEvent_JSONMarshaling(t *testing.T) {
	event := &AppInstallEvent{
		EventID:           uuid.New().String(),
		Timestamp:         time.Now(),
		InstallID:         "install123",
		CatalogTemplateID: 42,
		TemplateName:      "vscode",
		DisplayName:       "VS Code",
		Description:       "Code editor",
		Category:          "development",
		Manifest:          `{"version": "1.0"}`,
		InstalledBy:       "admin",
		Platform:          PlatformKubernetes,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded AppInstallEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.TemplateName, decoded.TemplateName)
	assert.Equal(t, event.CatalogTemplateID, decoded.CatalogTemplateID)
	assert.Equal(t, event.Manifest, decoded.Manifest)
}

func TestResourceSpec_JSONMarshaling(t *testing.T) {
	spec := ResourceSpec{
		Memory: "8Gi",
		CPU:    "4000m",
	}

	data, err := json.Marshal(spec)
	require.NoError(t, err)

	var decoded ResourceSpec
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, spec.Memory, decoded.Memory)
	assert.Equal(t, spec.CPU, decoded.CPU)
}

func TestPlatformConstants(t *testing.T) {
	// Verify platform constants exist and are unique
	platforms := []string{
		PlatformKubernetes,
		PlatformDocker,
		PlatformHyperV,
		PlatformVCenter,
	}

	assert.Equal(t, "kubernetes", PlatformKubernetes)
	assert.Equal(t, "docker", PlatformDocker)
	assert.Equal(t, "hyperv", PlatformHyperV)
	assert.Equal(t, "vcenter", PlatformVCenter)

	// Verify all are unique
	seen := make(map[string]bool)
	for _, p := range platforms {
		assert.False(t, seen[p], "Duplicate platform: %s", p)
		seen[p] = true
	}
}

func TestStatusConstants(t *testing.T) {
	statuses := []string{
		StatusPending,
		StatusCreating,
		StatusRunning,
		StatusHibernated,
		StatusFailed,
		StatusDeleting,
		StatusDeleted,
	}

	// Verify expected values
	assert.Equal(t, "pending", StatusPending)
	assert.Equal(t, "running", StatusRunning)
	assert.Equal(t, "failed", StatusFailed)

	// Verify uniqueness
	seen := make(map[string]bool)
	for _, s := range statuses {
		assert.False(t, seen[s], "Duplicate status: %s", s)
		seen[s] = true
	}
}

func TestSessionStatusEvent_JSONMarshaling(t *testing.T) {
	event := &SessionStatusEvent{
		EventID:      uuid.New().String(),
		Timestamp:    time.Now(),
		SessionID:    "session123",
		Status:       StatusRunning,
		Phase:        "ready",
		URL:          "https://session.example.com",
		PodName:      "pod-abc123",
		Message:      "Session is ready",
		ControllerID: "controller-1",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded SessionStatusEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.SessionID, decoded.SessionID)
	assert.Equal(t, event.Status, decoded.Status)
	assert.Equal(t, event.URL, decoded.URL)
}

func TestTemplateCreateEvent_JSONMarshaling(t *testing.T) {
	event := &TemplateCreateEvent{
		EventID:     uuid.New().String(),
		Timestamp:   time.Now(),
		TemplateID:  "template123",
		DisplayName: "Ubuntu Desktop",
		Category:    "linux",
		BaseImage:   "ubuntu:22.04",
		Manifest:    `{"vnc_port": 5900}`,
		Platform:    PlatformKubernetes,
		CreatedBy:   "admin",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded TemplateCreateEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.TemplateID, decoded.TemplateID)
	assert.Equal(t, event.DisplayName, decoded.DisplayName)
}

func TestControllerHeartbeatEvent_JSONMarshaling(t *testing.T) {
	event := &ControllerHeartbeatEvent{
		ControllerID: "controller-k8s-1",
		Platform:     PlatformKubernetes,
		Timestamp:    time.Now(),
		Status:       "healthy",
		Version:      "1.0.0",
		Capabilities: []string{"sessions", "templates", "scaling"},
		ClusterInfo: map[string]interface{}{
			"nodes": 5,
			"cpu":   "32000m",
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded ControllerHeartbeatEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.ControllerID, decoded.ControllerID)
	assert.Equal(t, event.Status, decoded.Status)
	assert.Len(t, decoded.Capabilities, 3)
}

func TestNodeEvents_JSONMarshaling(t *testing.T) {
	t.Run("NodeCordonEvent", func(t *testing.T) {
		event := &NodeCordonEvent{
			EventID:   uuid.New().String(),
			Timestamp: time.Now(),
			NodeName:  "node-1",
			Platform:  PlatformKubernetes,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var decoded NodeCordonEvent
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, event.NodeName, decoded.NodeName)
	})

	t.Run("NodeDrainEvent", func(t *testing.T) {
		gracePeriod := int64(300)
		event := &NodeDrainEvent{
			EventID:            uuid.New().String(),
			Timestamp:          time.Now(),
			NodeName:           "node-2",
			Platform:           PlatformKubernetes,
			GracePeriodSeconds: &gracePeriod,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var decoded NodeDrainEvent
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, event.NodeName, decoded.NodeName)
		assert.NotNil(t, decoded.GracePeriodSeconds)
		assert.Equal(t, gracePeriod, *decoded.GracePeriodSeconds)
	})
}

// Test Publisher disabled mode
func TestPublisher_DisabledMode(t *testing.T) {
	publisher := &Publisher{enabled: false}

	assert.False(t, publisher.IsEnabled())

	// Should not error when publishing while disabled
	err := publisher.Publish("test.subject", map[string]string{"key": "value"})
	assert.NoError(t, err)
}

// Test event ID generation
func TestPublisher_EventIDGeneration(t *testing.T) {
	// Test that PublishSession methods auto-generate event IDs
	publisher := &Publisher{enabled: false} // Disabled to avoid NATS connection
	ctx := context.Background()

	t.Run("SessionCreateEvent auto-generates ID", func(t *testing.T) {
		event := &SessionCreateEvent{
			SessionID: "session123",
			UserID:    "user456",
			Platform:  PlatformKubernetes,
		}

		assert.Empty(t, event.EventID)
		assert.True(t, event.Timestamp.IsZero())

		err := publisher.PublishSessionCreate(ctx, event)
		assert.NoError(t, err)

		// Should have generated ID and timestamp
		assert.NotEmpty(t, event.EventID)
		assert.False(t, event.Timestamp.IsZero())
	})

	t.Run("AppInstallEvent auto-generates ID", func(t *testing.T) {
		event := &AppInstallEvent{
			InstallID:    "install123",
			TemplateName: "vscode",
			Platform:     PlatformKubernetes,
		}

		err := publisher.PublishAppInstall(ctx, event)
		assert.NoError(t, err)

		assert.NotEmpty(t, event.EventID)
		assert.False(t, event.Timestamp.IsZero())
	})
}

// Test that install status constants are defined
func TestInstallStatusConstants(t *testing.T) {
	assert.Equal(t, "pending", InstallStatusPending)
	assert.Equal(t, "installing", InstallStatusInstalling)
	assert.Equal(t, "ready", InstallStatusReady)
	assert.Equal(t, "failed", InstallStatusFailed)
}
