package leaderelection

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileBackend_New tests creating a new file backend
func TestFileBackend_New(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		config    *LeaderElectorConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid config",
			config: &LeaderElectorConfig{
				AgentID:      "test-agent",
				InstanceID:   "instance-1",
				LockFilePath: filepath.Join(tmpDir, "test.lock"),
			},
			wantErr: false,
		},
		{
			name: "missing lock file path",
			config: &LeaderElectorConfig{
				AgentID:    "test-agent",
				InstanceID: "instance-1",
				// LockFilePath not set
			},
			wantErr:   true,
			errString: "lock file path is required",
		},
		{
			name: "creates parent directory",
			config: &LeaderElectorConfig{
				AgentID:      "test-agent",
				InstanceID:   "instance-1",
				LockFilePath: filepath.Join(tmpDir, "subdir", "test.lock"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := newFileBackend(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("newFileBackend() error = nil, wantErr true")
					return
				}
				if tt.errString != "" && err.Error() != tt.errString {
					t.Logf("Error message: %v", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("newFileBackend() unexpected error = %v", err)
			}

			if backend == nil {
				t.Fatal("backend is nil")
			}

			if backend.lockPath != tt.config.LockFilePath {
				t.Errorf("lockPath = %v, want %v", backend.lockPath, tt.config.LockFilePath)
			}

			// Cleanup
			backend.Close()
		})
	}
}

// TestFileBackend_TryAcquire tests acquiring the lock
func TestFileBackend_TryAcquire(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("acquire lock successfully", func(t *testing.T) {
		lockPath := filepath.Join(tmpDir, "acquire-test.lock")
		config := &LeaderElectorConfig{
			AgentID:      "test-agent",
			InstanceID:   "instance-1",
			LockFilePath: lockPath,
		}

		backend, err := newFileBackend(config)
		if err != nil {
			t.Fatalf("newFileBackend() error = %v", err)
		}
		defer backend.Close()

		ctx := context.Background()
		acquired, err := backend.TryAcquire(ctx)

		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}

		if !acquired {
			t.Error("TryAcquire() = false, want true")
		}

		// Verify lock file exists
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			t.Error("Lock file was not created")
		}

		// Verify instance ID written to file
		data, err := os.ReadFile(lockPath)
		if err != nil {
			t.Fatalf("Failed to read lock file: %v", err)
		}

		content := string(data)
		if len(content) == 0 {
			t.Error("Lock file is empty")
		}
	})

	t.Run("second instance cannot acquire", func(t *testing.T) {
		lockPath := filepath.Join(tmpDir, "contention-test.lock")
		config1 := &LeaderElectorConfig{
			AgentID:      "test-agent",
			InstanceID:   "instance-1",
			LockFilePath: lockPath,
		}

		// First backend acquires lock
		backend1, err := newFileBackend(config1)
		if err != nil {
			t.Fatalf("newFileBackend() error = %v", err)
		}
		defer backend1.Close()

		ctx := context.Background()
		acquired, err := backend1.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}
		if !acquired {
			t.Fatal("First instance should acquire lock")
		}

		// Second backend tries to acquire same lock
		config2 := &LeaderElectorConfig{
			AgentID:      "test-agent",
			InstanceID:   "instance-2",
			LockFilePath: lockPath,
		}

		backend2, err := newFileBackend(config2)
		if err != nil {
			t.Fatalf("newFileBackend() error = %v", err)
		}
		defer backend2.Close()

		acquired2, err := backend2.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}

		if acquired2 {
			t.Error("Second instance should not acquire lock")
		}
	})
}

// TestFileBackend_Renew tests renewing the lock
func TestFileBackend_Renew(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "renew-test.lock")

	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		InstanceID:   "instance-1",
		LockFilePath: lockPath,
	}

	backend, err := newFileBackend(config)
	if err != nil {
		t.Fatalf("newFileBackend() error = %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("renew without lock fails", func(t *testing.T) {
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

		// Verify timestamp was updated in file
		data, err := os.ReadFile(lockPath)
		if err != nil {
			t.Fatalf("Failed to read lock file: %v", err)
		}

		if len(data) == 0 {
			t.Error("Lock file is empty after renew")
		}
	})
}

// TestFileBackend_Release tests releasing the lock
func TestFileBackend_Release(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "release-test.lock")

	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		InstanceID:   "instance-1",
		LockFilePath: lockPath,
	}

	backend, err := newFileBackend(config)
	if err != nil {
		t.Fatalf("newFileBackend() error = %v", err)
	}

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
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}
		if !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Release lock
		err = backend.Release(ctx)
		if err != nil {
			t.Errorf("Release() error = %v", err)
		}

		// Verify can acquire again
		acquired2, err := backend.TryAcquire(ctx)
		if err != nil {
			t.Fatalf("TryAcquire() after release error = %v", err)
		}
		if !acquired2 {
			t.Error("Should be able to acquire lock after release")
		}
	})

	backend.Close()
}

// TestFileBackend_GetLeader tests getting the current leader
func TestFileBackend_GetLeader(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "getleader-test.lock")

	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		InstanceID:   "instance-1",
		LockFilePath: lockPath,
	}

	backend, err := newFileBackend(config)
	if err != nil {
		t.Fatalf("newFileBackend() error = %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("no leader initially", func(t *testing.T) {
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
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}
		if !acquired {
			t.Fatal("Failed to acquire lock")
		}

		// Get leader
		leader, err := backend.GetLeader(ctx)
		if err != nil {
			t.Errorf("GetLeader() error = %v", err)
		}

		if leader != config.InstanceID {
			t.Errorf("GetLeader() = %v, want %v", leader, config.InstanceID)
		}
	})
}

// TestFileBackend_Close tests closing the backend
func TestFileBackend_Close(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "close-test.lock")

	config := &LeaderElectorConfig{
		AgentID:      "test-agent",
		InstanceID:   "instance-1",
		LockFilePath: lockPath,
	}

	backend, err := newFileBackend(config)
	if err != nil {
		t.Fatalf("newFileBackend() error = %v", err)
	}

	ctx := context.Background()

	// Acquire lock
	acquired, err := backend.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("Failed to acquire lock")
	}

	// Close
	err = backend.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify lock was released (another instance can acquire)
	backend2, err := newFileBackend(config)
	if err != nil {
		t.Fatalf("newFileBackend() error = %v", err)
	}
	defer backend2.Close()

	acquired2, err := backend2.TryAcquire(ctx)
	if err != nil {
		t.Fatalf("TryAcquire() after close error = %v", err)
	}
	if !acquired2 {
		t.Error("Should be able to acquire lock after first backend closed")
	}
}

// TestFileBackend_ConcurrentAccess tests concurrent lock attempts
func TestFileBackend_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "concurrent-test.lock")

	// Create multiple backends trying to acquire same lock
	numBackends := 5
	backends := make([]*fileBackend, numBackends)
	configs := make([]*LeaderElectorConfig, numBackends)

	for i := 0; i < numBackends; i++ {
		configs[i] = &LeaderElectorConfig{
			AgentID:      "test-agent",
			InstanceID:   filepath.Base(tmpDir) + "-instance-" + string(rune('A'+i)),
			LockFilePath: lockPath,
		}

		var err error
		backends[i], err = newFileBackend(configs[i])
		if err != nil {
			t.Fatalf("newFileBackend() error = %v", err)
		}
		defer backends[i].Close()
	}

	ctx := context.Background()

	// Try to acquire lock from all backends concurrently
	results := make(chan bool, numBackends)
	for i := 0; i < numBackends; i++ {
		go func(idx int) {
			acquired, _ := backends[idx].TryAcquire(ctx)
			results <- acquired
		}(i)
	}

	// Collect results
	acquiredCount := 0
	for i := 0; i < numBackends; i++ {
		if <-results {
			acquiredCount++
		}
	}

	// Exactly one should have acquired the lock
	if acquiredCount != 1 {
		t.Errorf("Acquired count = %d, want 1 (exactly one leader)", acquiredCount)
	}
}
