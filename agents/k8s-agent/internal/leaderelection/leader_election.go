// Package leaderelection implements Kubernetes leader election for k8s-agent HA.
//
// This enables running multiple k8s-agent replicas for the same cluster with
// active-standby failover. Only one agent instance will be active at a time.
//
// Features:
//   - Automatic leader election using Kubernetes leases
//   - Graceful leader handoff on pod termination
//   - Automatic failover on leader failure
//   - Configurable lease duration and renew deadline
//
// Usage:
//   elector := NewLeaderElector(kubeClient, config)
//   elector.Run(onBecomeLeader, onLoseLeadership)
package leaderelection

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// LeaderElectorConfig configures the leader election behavior.
type LeaderElectorConfig struct {
	// AgentID is the unique identifier for this agent cluster
	// Example: "k8s-prod-us-east-1"
	AgentID string

	// Namespace where the lease resource will be created
	Namespace string

	// PodName is the name of this pod (must be unique)
	// Automatically set from POD_NAME environment variable
	PodName string

	// LeaseDuration is the duration that non-leader candidates will
	// wait to force acquire leadership. This is measured against time of
	// last observed ack. Default: 15 seconds.
	LeaseDuration time.Duration

	// RenewDeadline is the duration the leader will retry refreshing leadership
	// before giving up. Default: 10 seconds.
	RenewDeadline time.Duration

	// RetryPeriod is the duration the LeaderElector clients should wait
	// between tries of actions. Default: 2 seconds.
	RetryPeriod time.Duration
}

// DefaultConfig returns default leader election configuration.
func DefaultConfig(agentID, namespace string) *LeaderElectorConfig {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		// Fallback to hostname if POD_NAME not set
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown-pod"
		}
		podName = hostname
		log.Printf("[LeaderElection] WARNING: POD_NAME not set, using hostname: %s", podName)
	}

	return &LeaderElectorConfig{
		AgentID:       agentID,
		Namespace:     namespace,
		PodName:       podName,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
	}
}

// LeaderElector manages leader election for agent HA.
type LeaderElector struct {
	config      *LeaderElectorConfig
	kubeClient  *kubernetes.Clientset
	elector     *leaderelection.LeaderElector
	stopChan    chan struct{}
	isLeader    bool
	leaderChan  chan bool // Notifies leadership state changes
}

// NewLeaderElector creates a new leader election manager.
func NewLeaderElector(kubeClient *kubernetes.Clientset, config *LeaderElectorConfig) *LeaderElector {
	return &LeaderElector{
		config:     config,
		kubeClient: kubeClient,
		stopChan:   make(chan struct{}),
		isLeader:   false,
		leaderChan: make(chan bool, 1),
	}
}

// Run starts the leader election process.
//
// Callbacks:
//   - onBecomeLeader: Called when this instance becomes the leader
//   - onLoseLeadership: Called when this instance loses leadership
//
// This function blocks until stopped via Stop().
func (le *LeaderElector) Run(ctx context.Context, onBecomeLeader, onLoseLeadership func()) error {
	// Create lease resource lock
	// The lock name is based on the agentID to ensure only one leader per agent cluster
	lockName := fmt.Sprintf("streamspace-agent-%s", le.config.AgentID)

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockName,
			Namespace: le.config.Namespace,
		},
		Client: le.kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: le.config.PodName,
		},
	}

	// Create leader election configuration
	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   le.config.LeaseDuration,
		RenewDeadline:   le.config.RenewDeadline,
		RetryPeriod:     le.config.RetryPeriod,
		ReleaseOnCancel: true,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Printf("[LeaderElection] 🎖️  Became leader for agent: %s", le.config.AgentID)
				le.isLeader = true

				// Notify leadership change
				select {
				case le.leaderChan <- true:
				default:
				}

				// Call user-provided callback
				if onBecomeLeader != nil {
					onBecomeLeader()
				}
			},

			OnStoppedLeading: func() {
				log.Printf("[LeaderElection] ⚠️  Lost leadership for agent: %s", le.config.AgentID)
				le.isLeader = false

				// Notify leadership change
				select {
				case le.leaderChan <- false:
				default:
				}

				// Call user-provided callback
				if onLoseLeadership != nil {
					onLoseLeadership()
				}
			},

			OnNewLeader: func(identity string) {
				if identity == le.config.PodName {
					log.Printf("[LeaderElection] I am the new leader: %s", identity)
				} else {
					log.Printf("[LeaderElection] New leader elected: %s (I am standby)", identity)
				}
			},
		},
	}

	// Create leader elector
	elector, err := leaderelection.NewLeaderElector(leaderElectionConfig)
	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	le.elector = elector

	log.Printf("[LeaderElection] Starting leader election for agent: %s (pod: %s)",
		le.config.AgentID, le.config.PodName)
	log.Printf("[LeaderElection] Lease: %s, Renew: %s, Retry: %s",
		le.config.LeaseDuration, le.config.RenewDeadline, le.config.RetryPeriod)

	// Run leader election (blocks until context cancelled)
	elector.Run(ctx)

	log.Println("[LeaderElection] Leader election stopped")
	return nil
}

// Stop stops the leader election process.
func (le *LeaderElector) Stop() {
	close(le.stopChan)
}

// IsLeader returns true if this instance is currently the leader.
func (le *LeaderElector) IsLeader() bool {
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

// GetLeaderIdentity returns the current leader's identity (pod name).
//
// Returns empty string if leader is unknown or election not started.
func (le *LeaderElector) GetLeaderIdentity() string {
	if le.elector == nil {
		return ""
	}

	return le.elector.GetLeader()
}
