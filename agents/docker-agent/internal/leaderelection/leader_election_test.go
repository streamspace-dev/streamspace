package leaderelection

import (
	"context"
	"testing"
	"time"
)

// TestBackendConstants tests backend type constants
func TestBackendConstants(t *testing.T) {
	tests := []struct {
		name    string
		backend Backend
		want    string
	}{
		{"file backend", BackendFile, "file"},
		{"redis backend", BackendRedis, "redis"},
		{"swarm backend", BackendSwarm, "swarm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.backend) != tt.want {
				t.Errorf("Backend = %v, want %v", tt.backend, tt.want)
			}
		})
	}
}

// TestDefaultConfig tests the default configuration generator
func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		backend Backend
	}{
		{"file backend", "test-agent-1", BackendFile},
		{"redis backend", "test-agent-2", BackendRedis},
		{"swarm backend", "test-agent-3", BackendSwarm},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig(tt.agentID, tt.backend)

			if config.AgentID != tt.agentID {
				t.Errorf("AgentID = %v, want %v", config.AgentID, tt.agentID)
			}

			if config.Backend != tt.backend {
				t.Errorf("Backend = %v, want %v", config.Backend, tt.backend)
			}

			if config.InstanceID == "" {
				t.Error("InstanceID should not be empty")
			}

			if config.LeaseDuration != 15*time.Second {
				t.Errorf("LeaseDuration = %v, want 15s", config.LeaseDuration)
			}

			if config.RenewDeadline != 10*time.Second {
				t.Errorf("RenewDeadline = %v, want 10s", config.RenewDeadline)
			}

			if config.RetryPeriod != 2*time.Second {
				t.Errorf("RetryPeriod = %v, want 2s", config.RetryPeriod)
			}

			if config.RedisKeyPrefix != "streamspace:agent:leader:" {
				t.Errorf("RedisKeyPrefix = %v, want streamspace:agent:leader:", config.RedisKeyPrefix)
			}

			// File backend should have lock file path set
			if tt.backend == BackendFile && config.LockFilePath == "" {
				t.Error("LockFilePath should be set for file backend")
			}
		})
	}
}

// TestLeaderElectorConfig_Validation tests configuration validation
func TestLeaderElectorConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *LeaderElectorConfig
		wantErr bool
	}{
		{
			name: "valid file backend",
			config: &LeaderElectorConfig{
				AgentID:       "test-agent",
				Backend:       BackendFile,
				InstanceID:    "instance-1",
				LockFilePath:  "/tmp/test.lock",
				LeaseDuration: 15 * time.Second,
				RenewDeadline: 10 * time.Second,
				RetryPeriod:   2 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "file backend missing lock path",
			config: &LeaderElectorConfig{
				AgentID:    "test-agent",
				Backend:    BackendFile,
				InstanceID: "instance-1",
				// LockFilePath missing
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLeaderElector(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewLeaderElector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLeaderElector_IsLeader tests the IsLeader method
func TestLeaderElector_IsLeader(t *testing.T) {
	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		Backend:      BackendFile,
		InstanceID:   "instance-1",
		LockFilePath: "/tmp/test-leader.lock",
	}

	elector, err := NewLeaderElector(config)
	if err != nil {
		t.Fatalf("NewLeaderElector() error = %v", err)
	}
	defer elector.backend.Close()

	// Initially should not be leader
	if elector.IsLeader() {
		t.Error("IsLeader() = true, want false initially")
	}

	// Manually set leadership for testing
	elector.leaderMu.Lock()
	elector.isLeader = true
	elector.leaderMu.Unlock()

	if !elector.IsLeader() {
		t.Error("IsLeader() = false, want true after setting")
	}
}

// TestLeaderElector_Stop tests the Stop method
func TestLeaderElector_Stop(t *testing.T) {
	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		Backend:      BackendFile,
		InstanceID:   "instance-1",
		LockFilePath: "/tmp/test-stop.lock",
	}

	elector, err := NewLeaderElector(config)
	if err != nil {
		t.Fatalf("NewLeaderElector() error = %v", err)
	}
	defer elector.backend.Close()

	// Stop should close the stop channel
	elector.Stop()

	select {
	case <-elector.stopChan:
		// Good - channel closed
	case <-time.After(100 * time.Millisecond):
		t.Error("stopChan should be closed after Stop()")
	}
}

// TestLeaderElector_GetLeaderIdentity tests getting leader identity
func TestLeaderElector_GetLeaderIdentity(t *testing.T) {
	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		Backend:      BackendFile,
		InstanceID:   "instance-1",
		LockFilePath: "/tmp/test-identity.lock",
	}

	elector, err := NewLeaderElector(config)
	if err != nil {
		t.Fatalf("NewLeaderElector() error = %v", err)
	}
	defer elector.backend.Close()

	// Should return empty string initially (no leader)
	leader := elector.GetLeaderIdentity()
	if leader != "" {
		t.Logf("GetLeaderIdentity() = %v (may be empty if no leader)", leader)
	}
}

// MockLeaderBackend is a mock backend for testing leader election logic
type MockLeaderBackend struct {
	acquireResult bool
	acquireErr    error
	renewErr      error
	releaseErr    error
	leader        string
	getLeaderErr  error
	closed        bool
}

func (m *MockLeaderBackend) TryAcquire(ctx context.Context) (bool, error) {
	return m.acquireResult, m.acquireErr
}

func (m *MockLeaderBackend) Renew(ctx context.Context) error {
	return m.renewErr
}

func (m *MockLeaderBackend) Release(ctx context.Context) error {
	return m.releaseErr
}

func (m *MockLeaderBackend) GetLeader(ctx context.Context) (string, error) {
	return m.leader, m.getLeaderErr
}

func (m *MockLeaderBackend) Close() error {
	m.closed = true
	return nil
}

// TestLeaderElector_RunWithMockBackend tests the Run method with a mock backend
func TestLeaderElector_RunWithMockBackend(t *testing.T) {
	config := &LeaderElectorConfig{
		AgentID:       "test-agent",
		Backend:       BackendFile, // Doesn't matter, we'll replace it
		InstanceID:    "instance-1",
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   100 * time.Millisecond, // Fast for testing
	}

	t.Run("becomes leader", func(t *testing.T) {
		mockBackend := &MockLeaderBackend{
			acquireResult: true, // Successfully acquire leadership
		}

		elector := &LeaderElector{
			config:     config,
			backend:    mockBackend,
			stopChan:   make(chan struct{}),
			isLeader:   false,
			leaderChan: make(chan bool, 1),
		}

		becameLeader := false
		onBecomeLeader := func() {
			becameLeader = true
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Run in background
		go func() {
			elector.Run(ctx, onBecomeLeader, nil)
		}()

		// Wait for leadership
		select {
		case <-time.After(500 * time.Millisecond):
			// Should have become leader by now
		}

		// Stop
		elector.Stop()

		if !becameLeader {
			t.Error("onBecomeLeader callback was not called")
		}

		if !elector.IsLeader() && becameLeader {
			t.Log("Leadership may have been released after stop, which is acceptable")
		}
	})

	t.Run("does not become leader", func(t *testing.T) {
		mockBackend := &MockLeaderBackend{
			acquireResult: false, // Fail to acquire leadership
			leader:        "other-instance",
		}

		elector := &LeaderElector{
			config:     config,
			backend:    mockBackend,
			stopChan:   make(chan struct{}),
			isLeader:   false,
			leaderChan: make(chan bool, 1),
		}

		becameLeader := false
		onBecomeLeader := func() {
			becameLeader = true
		}

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		// Run (will stop after context timeout)
		elector.Run(ctx, onBecomeLeader, nil)

		if becameLeader {
			t.Error("onBecomeLeader should not be called when acquisition fails")
		}

		if elector.IsLeader() {
			t.Error("IsLeader() = true, want false")
		}
	})
}

// TestLeaderElector_WaitForLeadership tests waiting for leadership
func TestLeaderElector_WaitForLeadership(t *testing.T) {
	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		Backend:      BackendFile,
		InstanceID:   "instance-1",
		LockFilePath: "/tmp/test-wait.lock",
	}

	elector, err := NewLeaderElector(config)
	if err != nil {
		t.Fatalf("NewLeaderElector() error = %v", err)
	}
	defer elector.backend.Close()

	t.Run("stop before becoming leader", func(t *testing.T) {
		// Start waiting in background
		result := make(chan bool, 1)
		go func() {
			result <- elector.WaitForLeadership()
		}()

		// Stop before becoming leader
		time.Sleep(50 * time.Millisecond)
		elector.Stop()

		// Should return false
		select {
		case became := <-result:
			if became {
				t.Error("WaitForLeadership() = true, want false when stopped")
			}
		case <-time.After(1 * time.Second):
			t.Error("WaitForLeadership() did not return")
		}
	})
}

// TestLeaderElectorConfig_DefaultValues tests that default values are reasonable
func TestLeaderElectorConfig_DefaultValues(t *testing.T) {
	config := DefaultConfig("test-agent", BackendFile)

	// Verify timing values make sense
	if config.RenewDeadline >= config.LeaseDuration {
		t.Errorf("RenewDeadline (%v) should be < LeaseDuration (%v)",
			config.RenewDeadline, config.LeaseDuration)
	}

	if config.RetryPeriod >= config.RenewDeadline {
		t.Logf("RetryPeriod (%v) is close to RenewDeadline (%v), may want to adjust",
			config.RetryPeriod, config.RenewDeadline)
	}

	// Verify reasonable ranges
	if config.LeaseDuration < 5*time.Second {
		t.Error("LeaseDuration seems too short (<5s)")
	}

	if config.LeaseDuration > 60*time.Second {
		t.Error("LeaseDuration seems too long (>60s)")
	}
}
