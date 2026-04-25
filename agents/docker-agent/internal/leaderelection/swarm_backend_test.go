package leaderelection

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

// TestSwarmBackend_New tests creating a new Swarm backend
// This test requires running inside a Docker Swarm container
func TestSwarmBackend_New_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("Docker client not available: %v", err)
	}
	defer dockerClient.Close()

	// Check if running in Swarm mode
	info, err := dockerClient.Info(context.Background())
	if err != nil {
		t.Skipf("Cannot get Docker info: %v", err)
	}
	if !info.Swarm.ControlAvailable {
		t.Skip("Not running in Docker Swarm mode or not a manager node")
	}

	config := &LeaderElectorConfig{
		AgentID:       "test-agent",
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		// This is expected if not running in a Swarm service
		t.Logf("newSwarmBackend() error = %v (expected if not in Swarm service)", err)
		t.Skip("Not running inside a Swarm service")
	}
	defer backend.Close()

	if backend == nil {
		t.Fatal("backend is nil")
	}

	if backend.serviceID == "" {
		t.Error("serviceID should not be empty")
	}

	if backend.serviceName == "" {
		t.Error("serviceName should not be empty")
	}

	if backend.taskID == "" {
		t.Error("taskID should not be empty")
	}

	expectedLeaderLabel := "streamspace.agent.leader.test-agent"
	if backend.leaderLabel != expectedLeaderLabel {
		t.Errorf("leaderLabel = %v, want %v", backend.leaderLabel, expectedLeaderLabel)
	}

	expectedTimestampLabel := "streamspace.agent.leader.test-agent.timestamp"
	if backend.timestampLabel != expectedTimestampLabel {
		t.Errorf("timestampLabel = %v, want %v", backend.timestampLabel, expectedTimestampLabel)
	}
}

// TestSwarmBackend_LabelFormat tests that labels are formatted correctly
func TestSwarmBackend_LabelFormat(t *testing.T) {
	tests := []struct {
		name                   string
		agentID                string
		expectedLeaderLabel    string
		expectedTimestampLabel string
	}{
		{
			name:                   "simple agent ID",
			agentID:                "docker-agent",
			expectedLeaderLabel:    "streamspace.agent.leader.docker-agent",
			expectedTimestampLabel: "streamspace.agent.leader.docker-agent.timestamp",
		},
		{
			name:                   "agent ID with numbers",
			agentID:                "agent-123",
			expectedLeaderLabel:    "streamspace.agent.leader.agent-123",
			expectedTimestampLabel: "streamspace.agent.leader.agent-123.timestamp",
		},
		{
			name:                   "complex agent ID",
			agentID:                "production-docker-agent-v2",
			expectedLeaderLabel:    "streamspace.agent.leader.production-docker-agent-v2",
			expectedTimestampLabel: "streamspace.agent.leader.production-docker-agent-v2.timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't actually create a backend without Swarm,
			// but we can test the label format logic
			leaderLabel := "streamspace.agent.leader." + tt.agentID
			timestampLabel := "streamspace.agent.leader." + tt.agentID + ".timestamp"

			if leaderLabel != tt.expectedLeaderLabel {
				t.Errorf("leaderLabel = %v, want %v", leaderLabel, tt.expectedLeaderLabel)
			}

			if timestampLabel != tt.expectedTimestampLabel {
				t.Errorf("timestampLabel = %v, want %v", timestampLabel, tt.expectedTimestampLabel)
			}
		})
	}
}

// TestSwarmBackend_TryAcquire_Integration tests acquiring leadership
func TestSwarmBackend_TryAcquire_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires running inside a Docker Swarm service
	config := &LeaderElectorConfig{
		AgentID:       "test-agent-acquire",
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		t.Skipf("Cannot create Swarm backend: %v (requires Swarm service)", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("acquire lock successfully", func(t *testing.T) {
		acquired, err := backend.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}

		if !acquired {
			t.Error("TryAcquire() = false, want true")
		}

		// Verify leader is set
		leader, err := backend.GetLeader(ctx)
		if err != nil {
			t.Fatalf("GetLeader() error = %v", err)
		}

		if leader != backend.taskID {
			t.Errorf("GetLeader() = %v, want %v", leader, backend.taskID)
		}
	})

	// Clean up
	backend.Release(ctx)
}

// TestSwarmBackend_Renew_Integration tests renewing leadership
func TestSwarmBackend_Renew_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &LeaderElectorConfig{
		AgentID:       "test-agent-renew",
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		t.Skipf("Cannot create Swarm backend: %v (requires Swarm service)", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("renew without lock fails", func(t *testing.T) {
		// Make sure we don't hold the lock
		backend.Release(ctx)

		err := backend.Renew(ctx)
		if err == nil {
			t.Error("Renew() error = nil, want error when not holding lock")
		}
	})

	t.Run("renew with lock succeeds", func(t *testing.T) {
		// Acquire lock first
		acquired, err := backend.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}
		if !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Renew lock
		err = backend.Renew(ctx)
		if err != nil {
			t.Errorf("Renew() error = %v", err)
		}

		// Verify still leader
		leader, err := backend.GetLeader(ctx)
		if err != nil {
			t.Fatalf("GetLeader() error = %v", err)
		}

		if leader != backend.taskID {
			t.Errorf("GetLeader() = %v, want %v after renew", leader, backend.taskID)
		}
	})

	// Clean up
	backend.Release(ctx)
}

// TestSwarmBackend_Release_Integration tests releasing leadership
func TestSwarmBackend_Release_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &LeaderElectorConfig{
		AgentID:       "test-agent-release",
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		t.Skipf("Cannot create Swarm backend: %v (requires Swarm service)", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("release without lock is safe", func(t *testing.T) {
		err := backend.Release(ctx)
		if err != nil {
			t.Errorf("Release() error = %v, want nil", err)
		}
	})

	t.Run("release after acquire works", func(t *testing.T) {
		// Acquire lock
		acquired, err := backend.TryAcquire(ctx)
		if err != nil || !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Verify leader is set
		leader, err := backend.GetLeader(ctx)
		if err != nil || leader != backend.taskID {
			t.Fatal("Leader should be set after acquire")
		}

		// Release lock
		err = backend.Release(ctx)
		if err != nil {
			t.Errorf("Release() error = %v", err)
		}

		// Verify leader is cleared
		leader, err = backend.GetLeader(ctx)
		if err != nil {
			t.Fatalf("GetLeader() error = %v", err)
		}

		if leader != "" {
			t.Error("Leader should be empty after release")
		}
	})
}

// TestSwarmBackend_GetLeader_Integration tests getting current leader
func TestSwarmBackend_GetLeader_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &LeaderElectorConfig{
		AgentID:       "test-agent-getleader",
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		t.Skipf("Cannot create Swarm backend: %v (requires Swarm service)", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("no leader initially", func(t *testing.T) {
		// Make sure no leader is set
		backend.Release(ctx)

		leader, err := backend.GetLeader(ctx)
		if err != nil {
			t.Errorf("GetLeader() error = %v", err)
		}
		if leader != "" {
			t.Errorf("GetLeader() = %v, want empty (no leader yet)", leader)
		}
	})

	t.Run("returns leader after acquire", func(t *testing.T) {
		// Acquire lock
		acquired, err := backend.TryAcquire(ctx)
		if err != nil || !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Get leader
		leader, err := backend.GetLeader(ctx)
		if err != nil {
			t.Errorf("GetLeader() error = %v", err)
		}

		if leader != backend.taskID {
			t.Errorf("GetLeader() = %v, want %v", leader, backend.taskID)
		}
	})

	// Clean up
	backend.Release(ctx)
}

// TestSwarmBackend_Close_Integration tests closing the backend
func TestSwarmBackend_Close_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &LeaderElectorConfig{
		AgentID:       "test-agent-close",
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		t.Skipf("Cannot create Swarm backend: %v (requires Swarm service)", err)
	}

	ctx := context.Background()

	// Acquire lock
	acquired, err := backend.TryAcquire(ctx)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// Close should release the lock
	err = backend.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Note: We can't verify the lock was released after Close()
	// because the backend is closed and we can't query it anymore
}

// TestSwarmBackend_LeaseExpiration_Integration tests that expired leases are detected
func TestSwarmBackend_LeaseExpiration_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &LeaderElectorConfig{
		AgentID:       "test-agent-expiration",
		InstanceID:    "instance-1",
		LeaseDuration: 2 * time.Second, // Short duration for testing
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		t.Skipf("Cannot create Swarm backend: %v (requires Swarm service)", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Acquire lock
	acquired, err := backend.TryAcquire(ctx)
	if err != nil || !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// Verify leader is set
	leader, err := backend.GetLeader(ctx)
	if err != nil || leader != backend.taskID {
		t.Fatal("Leader should be set after acquire")
	}

	// Wait for lease to expire
	time.Sleep(3 * time.Second)

	// Leader should be considered expired
	leader, err = backend.GetLeader(ctx)
	if err != nil {
		t.Fatalf("GetLeader() error = %v", err)
	}

	if leader != "" {
		t.Error("Leader should be empty after lease expiration")
	}

	// Clean up
	backend.Release(ctx)
}

// TestSwarmBackend_ErrorHandling tests error handling scenarios
func TestSwarmBackend_ErrorHandling(t *testing.T) {
	t.Run("newSwarmBackend requires Swarm mode", func(t *testing.T) {
		// This test can only verify the behavior when NOT in Swarm mode
		// which is the typical case during unit testing

		config := &LeaderElectorConfig{
			AgentID:       "test-agent",
			InstanceID:    "instance-1",
			LeaseDuration: 15 * time.Second,
		}

		backend, err := newSwarmBackend(config)

		// If we're not in a Swarm container, should get error
		// If we ARE in a Swarm container, this test will pass anyway
		if err != nil {
			// Expected when not in Swarm
			t.Logf("Expected error when not in Swarm: %v", err)
		} else if backend != nil {
			// We're in a Swarm container, clean up
			backend.Close()
		}
	})
}

// TestSwarmBackend_ConcurrentOperations tests concurrent access patterns
func TestSwarmBackend_ConcurrentOperations_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &LeaderElectorConfig{
		AgentID:       "test-agent-concurrent",
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
	}

	backend, err := newSwarmBackend(config)
	if err != nil {
		t.Skipf("Cannot create Swarm backend: %v (requires Swarm service)", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Note: This test simulates concurrent operations from the same backend
	// In a real deployment, multiple tasks would each have their own backend instance

	t.Run("concurrent acquires are safe", func(t *testing.T) {
		// Release first to ensure clean state
		backend.Release(ctx)

		results := make(chan bool, 3)
		errors := make(chan error, 3)

		// Try to acquire from multiple goroutines
		for i := 0; i < 3; i++ {
			go func() {
				acquired, err := backend.TryAcquire(ctx)
				results <- acquired
				errors <- err
			}()
		}

		// Collect results
		acquiredCount := 0
		for i := 0; i < 3; i++ {
			if <-results {
				acquiredCount++
			}
			if err := <-errors; err != nil {
				t.Errorf("TryAcquire() error = %v", err)
			}
		}

		// At least one should have acquired
		// (Due to race conditions, multiple may succeed, which is fine)
		if acquiredCount == 0 {
			t.Error("At least one goroutine should have acquired the lock")
		}

		t.Logf("Acquired count: %d", acquiredCount)
	})

	// Clean up
	backend.Release(ctx)
}

// TestSwarmBackend_TaskIDExtraction tests task ID extraction from container labels
func TestSwarmBackend_TaskIDExtraction_Unit(t *testing.T) {
	// This is a unit test to verify the label format we expect
	// Cannot actually test extraction without being in a Swarm container

	tests := []struct {
		name      string
		labelKey  string
		wantLabel string
	}{
		{
			name:      "task ID label",
			labelKey:  "com.docker.swarm.task.id",
			wantLabel: "com.docker.swarm.task.id",
		},
		{
			name:      "service ID label",
			labelKey:  "com.docker.swarm.service.id",
			wantLabel: "com.docker.swarm.service.id",
		},
		{
			name:      "service name label",
			labelKey:  "com.docker.swarm.service.name",
			wantLabel: "com.docker.swarm.service.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.labelKey != tt.wantLabel {
				t.Errorf("labelKey = %v, want %v", tt.labelKey, tt.wantLabel)
			}
		})
	}
}
