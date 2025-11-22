// Package leaderelection implements leader election for docker-agent HA.
//
// Unlike k8s-agent which uses Kubernetes Leases, docker-agent supports multiple
// leader election backends for different deployment scenarios:
//   - File-based locking (single-host deployments)
//   - Redis-based locking (multi-host deployments without orchestration)
//   - Docker Swarm service labels (Swarm-native deployments)
//
// This enables running multiple docker-agent replicas for the same Docker host
// with active-standby failover.
//
// Features:
//   - Automatic leader election using configurable backend
//   - Graceful leader handoff on process termination
//   - Automatic failover on leader failure
//   - Configurable lease duration and renew deadline
//
// Usage:
//   config := &LeaderElectorConfig{
//       AgentID: "docker-prod-host1",
//       Backend: "swarm",  // or "redis", "file"
//   }
//   elector := NewLeaderElector(config)
//   elector.Run(onBecomeLeader, onLoseLeadership)
package leaderelection

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Backend represents the leader election backend type.
type Backend string

const (
	// BackendFile uses file-based locking (flock)
	// Best for: Single-host deployments, development
	BackendFile Backend = "file"

	// BackendRedis uses Redis SET NX with TTL
	// Best for: Multi-host deployments without orchestration
	BackendRedis Backend = "redis"

	// BackendSwarm uses Docker Swarm service labels
	// Best for: Docker Swarm deployments, native Swarm HA
	BackendSwarm Backend = "swarm"
)

// LeaderElectorConfig configures the leader election behavior.
type LeaderElectorConfig struct {
	// AgentID is the unique identifier for this agent cluster
	// Example: "docker-prod-host1"
	AgentID string

	// Backend determines the leader election mechanism
	// Options: "file" or "redis"
	Backend Backend

	// InstanceID uniquely identifies this agent instance
	// Automatically set from HOSTNAME environment variable
	InstanceID string

	// LockFilePath is the path to the lock file (for file backend)
	// Default: /var/run/streamspace/docker-agent-{agentID}.lock
	LockFilePath string

	// RedisClient is the Redis client (for redis backend)
	RedisClient *redis.Client

	// RedisKeyPrefix is the prefix for Redis keys (for redis backend)
	// Default: "streamspace:agent:leader:"
	RedisKeyPrefix string

	// LeaseDuration is how long the leader lease lasts
	// Default: 15 seconds
	LeaseDuration time.Duration

	// RenewDeadline is how often the leader renews the lease
	// Must be < LeaseDuration. Default: 10 seconds
	RenewDeadline time.Duration

	// RetryPeriod is how often non-leaders check for leadership
	// Default: 2 seconds
	RetryPeriod time.Duration
}

// DefaultConfig returns default leader election configuration.
func DefaultConfig(agentID string, backend Backend) *LeaderElectorConfig {
	instanceID, err := os.Hostname()
	if err != nil {
		instanceID = fmt.Sprintf("instance-%d", time.Now().Unix())
		log.Printf("[LeaderElection] WARNING: Failed to get hostname, using: %s", instanceID)
	}

	config := &LeaderElectorConfig{
		AgentID:        agentID,
		Backend:        backend,
		InstanceID:     instanceID,
		LeaseDuration:  15 * time.Second,
		RenewDeadline:  10 * time.Second,
		RetryPeriod:    2 * time.Second,
		RedisKeyPrefix: "streamspace:agent:leader:",
	}

	// Set backend-specific defaults
	if backend == BackendFile {
		config.LockFilePath = filepath.Join("/var/run/streamspace", fmt.Sprintf("docker-agent-%s.lock", agentID))
	}

	return config
}

// LeaderElector manages leader election for agent HA.
type LeaderElector struct {
	config     *LeaderElectorConfig
	backend    leaderBackend
	stopChan   chan struct{}
	isLeader   bool
	leaderMu   sync.RWMutex
	leaderChan chan bool // Notifies leadership state changes
}

// leaderBackend is the interface for leader election backends.
type leaderBackend interface {
	// TryAcquire attempts to acquire leadership
	TryAcquire(ctx context.Context) (bool, error)

	// Renew renews the leadership lease
	Renew(ctx context.Context) error

	// Release releases leadership
	Release(ctx context.Context) error

	// GetLeader returns the current leader's instance ID
	GetLeader(ctx context.Context) (string, error)

	// Close cleans up backend resources
	Close() error
}

// NewLeaderElector creates a new leader election manager.
func NewLeaderElector(config *LeaderElectorConfig) (*LeaderElector, error) {
	var backend leaderBackend
	var err error

	// Create backend based on configuration
	switch config.Backend {
	case BackendFile:
		backend, err = newFileBackend(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create file backend: %w", err)
		}

	case BackendRedis:
		if config.RedisClient == nil {
			return nil, fmt.Errorf("redis client is required for redis backend")
		}
		backend = newRedisBackend(config)

	case BackendSwarm:
		backend, err = newSwarmBackend(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create swarm backend: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported backend: %s", config.Backend)
	}

	return &LeaderElector{
		config:     config,
		backend:    backend,
		stopChan:   make(chan struct{}),
		isLeader:   false,
		leaderChan: make(chan bool, 1),
	}, nil
}

// Run starts the leader election process.
//
// Callbacks:
//   - onBecomeLeader: Called when this instance becomes the leader
//   - onLoseLeadership: Called when this instance loses leadership
//
// This function blocks until stopped via Stop().
func (le *LeaderElector) Run(ctx context.Context, onBecomeLeader, onLoseLeadership func()) error {
	log.Printf("[LeaderElection] Starting leader election for agent: %s (instance: %s, backend: %s)",
		le.config.AgentID, le.config.InstanceID, le.config.Backend)

	ticker := time.NewTicker(le.config.RetryPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[LeaderElection] Context cancelled, stopping...")
			le.releaseIfLeader(context.Background())
			return nil

		case <-le.stopChan:
			log.Println("[LeaderElection] Stop signal received, stopping...")
			le.releaseIfLeader(context.Background())
			return nil

		case <-ticker.C:
			// Check current leadership status
			le.leaderMu.RLock()
			wasLeader := le.isLeader
			le.leaderMu.RUnlock()

			if wasLeader {
				// We are the leader, renew the lease
				if err := le.backend.Renew(ctx); err != nil {
					log.Printf("[LeaderElection] ⚠️  Failed to renew lease: %v", err)
					le.leaderMu.Lock()
					le.isLeader = false
					le.leaderMu.Unlock()

					// Lost leadership
					log.Printf("[LeaderElection] ⚠️  Lost leadership for agent: %s", le.config.AgentID)
					select {
					case le.leaderChan <- false:
					default:
					}

					if onLoseLeadership != nil {
						onLoseLeadership()
					}
				}
			} else {
				// We are not the leader, try to acquire
				acquired, err := le.backend.TryAcquire(ctx)
				if err != nil {
					log.Printf("[LeaderElection] Failed to acquire leadership: %v", err)
					continue
				}

				if acquired {
					// We became the leader!
					le.leaderMu.Lock()
					le.isLeader = true
					le.leaderMu.Unlock()

					log.Printf("[LeaderElection] 🎖️  Became leader for agent: %s", le.config.AgentID)
					select {
					case le.leaderChan <- true:
					default:
					}

					if onBecomeLeader != nil {
						onBecomeLeader()
					}

					// Update ticker to renew more frequently
					ticker.Reset(le.config.RenewDeadline)
				} else {
					// Check who is the leader
					if leader, err := le.backend.GetLeader(ctx); err == nil {
						if leader != "" && leader != le.config.InstanceID {
							log.Printf("[LeaderElection] Current leader: %s (I am standby)", leader)
						}
					}
				}
			}
		}
	}
}

// Stop stops the leader election process.
func (le *LeaderElector) Stop() {
	close(le.stopChan)
}

// IsLeader returns true if this instance is currently the leader.
func (le *LeaderElector) IsLeader() bool {
	le.leaderMu.RLock()
	defer le.leaderMu.RUnlock()
	return le.isLeader
}

// WaitForLeadership blocks until this instance becomes the leader.
//
// Returns:
//   - true if became leader
//   - false if stopped before becoming leader
func (le *LeaderElector) WaitForLeadership() bool {
	for {
		select {
		case isLeader := <-le.leaderChan:
			if isLeader {
				return true
			}
		case <-le.stopChan:
			return false
		}
	}
}

// GetLeaderIdentity returns the current leader's identity (instance ID).
//
// Returns empty string if leader is unknown.
func (le *LeaderElector) GetLeaderIdentity() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	leader, _ := le.backend.GetLeader(ctx)
	return leader
}

// releaseIfLeader releases leadership if this instance is the leader.
func (le *LeaderElector) releaseIfLeader(ctx context.Context) {
	le.leaderMu.RLock()
	isLeader := le.isLeader
	le.leaderMu.RUnlock()

	if isLeader {
		log.Println("[LeaderElection] Releasing leadership...")
		if err := le.backend.Release(ctx); err != nil {
			log.Printf("[LeaderElection] Error releasing leadership: %v", err)
		}

		le.leaderMu.Lock()
		le.isLeader = false
		le.leaderMu.Unlock()
	}

	// Close backend
	if err := le.backend.Close(); err != nil {
		log.Printf("[LeaderElection] Error closing backend: %v", err)
	}
}
