// Package leaderelection - File-based leader election backend
package leaderelection

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// fileBackend implements leader election using file-based locking (flock).
//
// This backend is suitable for:
//   - Single-host deployments (all replicas on same machine)
//   - Development and testing
//   - Simple deployments without Redis
//
// How it works:
//   - Creates a lock file at LockFilePath
//   - Uses flock (BSD) or lockf (POSIX) for exclusive locking
//   - Leader holds the lock, standby instances wait
//   - Lock is automatically released on process exit
//
// Limitations:
//   - Only works on Unix-like systems (Linux, macOS, BSD)
//   - All agent replicas must be on the same host
//   - Lock file must be on local filesystem (not NFS)
type fileBackend struct {
	config   *LeaderElectorConfig
	lockFile *os.File
	lockPath string
}

// newFileBackend creates a new file-based leader election backend.
func newFileBackend(config *LeaderElectorConfig) (*fileBackend, error) {
	lockPath := config.LockFilePath
	if lockPath == "" {
		return nil, fmt.Errorf("lock file path is required for file backend")
	}

	// Ensure parent directory exists
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	log.Printf("[LeaderElection:File] Using lock file: %s", lockPath)

	return &fileBackend{
		config:   config,
		lockPath: lockPath,
	}, nil
}

// TryAcquire attempts to acquire leadership by acquiring the file lock.
func (fb *fileBackend) TryAcquire(ctx context.Context) (bool, error) {
	// Open (or create) the lock file
	file, err := os.OpenFile(fb.lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK {
			// Lock is held by another process
			return false, nil
		}
		return false, fmt.Errorf("flock error: %w", err)
	}

	// Successfully acquired lock
	fb.lockFile = file

	// Write instance ID to lock file (for debugging)
	file.Truncate(0)
	file.Seek(0, 0)
	fmt.Fprintf(file, "%s\n%s\n", fb.config.InstanceID, time.Now().Format(time.RFC3339))
	file.Sync()

	log.Printf("[LeaderElection:File] Acquired lock: %s", fb.lockPath)
	return true, nil
}

// Renew renews the leadership lease.
//
// For file backend, the lock is held until explicitly released,
// so we just verify the lock is still held.
func (fb *fileBackend) Renew(ctx context.Context) error {
	if fb.lockFile == nil {
		return fmt.Errorf("not holding lock")
	}

	// Update timestamp in lock file
	fb.lockFile.Truncate(0)
	fb.lockFile.Seek(0, 0)
	fmt.Fprintf(fb.lockFile, "%s\n%s\n", fb.config.InstanceID, time.Now().Format(time.RFC3339))
	fb.lockFile.Sync()

	return nil
}

// Release releases the leadership lock.
func (fb *fileBackend) Release(ctx context.Context) error {
	if fb.lockFile == nil {
		return nil
	}

	// Release lock
	if err := syscall.Flock(int(fb.lockFile.Fd()), syscall.LOCK_UN); err != nil {
		log.Printf("[LeaderElection:File] Error releasing lock: %v", err)
	}

	// Close file
	if err := fb.lockFile.Close(); err != nil {
		log.Printf("[LeaderElection:File] Error closing lock file: %v", err)
	}

	fb.lockFile = nil
	log.Printf("[LeaderElection:File] Released lock: %s", fb.lockPath)

	return nil
}

// GetLeader returns the current leader's instance ID.
//
// Reads the instance ID from the lock file if available.
func (fb *fileBackend) GetLeader(ctx context.Context) (string, error) {
	// Try to read lock file
	data, err := os.ReadFile(fb.lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No leader yet
		}
		return "", err
	}

	// Parse instance ID (first line)
	lines := string(data)
	if len(lines) == 0 {
		return "", nil
	}

	// Find first newline
	var instanceID string
	for i, c := range lines {
		if c == '\n' {
			instanceID = lines[:i]
			break
		}
	}

	return instanceID, nil
}

// Close cleans up backend resources.
func (fb *fileBackend) Close() error {
	return fb.Release(context.Background())
}
