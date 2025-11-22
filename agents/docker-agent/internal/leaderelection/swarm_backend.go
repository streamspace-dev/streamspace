// Package leaderelection - Docker Swarm-based leader election backend
package leaderelection

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

// swarmBackend implements leader election using Docker Swarm service labels.
//
// This backend is suitable for:
//   - Docker Swarm deployments
//   - Production multi-node Docker environments
//   - Swarm-orchestrated HA setups
//
// How it works:
//   - Uses Docker service labels to track leader identity
//   - Label key: streamspace.agent.leader.<agentID> = <taskID>
//   - Leader sets label via atomic service update operations
//   - Uses Docker Swarm's distributed consensus for atomicity
//   - Standby tasks check service labels to determine leadership
//   - TTL implemented via label timestamp checking
//
// Benefits over file/Redis backends:
//   - No external dependencies (uses Swarm's built-in consensus)
//   - Atomic operations guaranteed by Docker Swarm
//   - Native Swarm integration
//   - Works across Swarm nodes automatically
//
// Requirements:
//   - Running in Docker Swarm mode
//   - Access to Docker socket (/var/run/docker.sock)
//   - Service running with replicated or global mode
type swarmBackend struct {
	config      *LeaderElectorConfig
	dockerClient *client.Client
	serviceID    string
	serviceName  string
	taskID       string
	leaderLabel  string
	timestampLabel string
}

// newSwarmBackend creates a new Docker Swarm-based leader election backend.
func newSwarmBackend(config *LeaderElectorConfig) (*swarmBackend, error) {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Verify we're running in Swarm mode
	info, err := dockerClient.Info(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}
	if !info.Swarm.ControlAvailable {
		return nil, fmt.Errorf("not running in Docker Swarm mode or not a manager node")
	}

	// Get current task/container ID from hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// In Docker Swarm, hostname is typically the task ID
	// Format: <service-name>.<replica-number>.<task-id>
	taskID := hostname
	if len(hostname) > 25 {
		// Docker task IDs are 25 characters
		taskID = hostname[:25]
	}

	// Find service ID by filtering tasks
	taskFilter := filters.NewArgs()
	taskFilter.Add("id", taskID)
	tasks, err := dockerClient.TaskList(context.Background(), types.TaskListOptions{
		Filters: taskFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no task found with ID: %s", taskID)
	}

	serviceID := tasks[0].ServiceID
	serviceName := tasks[0].Spec.ContainerSpec.Labels["com.docker.swarm.service.name"]

	leaderLabel := fmt.Sprintf("streamspace.agent.leader.%s", config.AgentID)
	timestampLabel := fmt.Sprintf("streamspace.agent.leader.%s.timestamp", config.AgentID)

	log.Printf("[LeaderElection:Swarm] Using service: %s (ID: %s), task: %s", serviceName, serviceID, taskID)
	log.Printf("[LeaderElection:Swarm] Leader label: %s", leaderLabel)

	return &swarmBackend{
		config:         config,
		dockerClient:   dockerClient,
		serviceID:      serviceID,
		serviceName:    serviceName,
		taskID:         taskID,
		leaderLabel:    leaderLabel,
		timestampLabel: timestampLabel,
	}, nil
}

// TryAcquire attempts to acquire leadership by setting the service label.
//
// Uses Docker service update with version check for atomic operations.
func (sb *swarmBackend) TryAcquire(ctx context.Context) (bool, error) {
	// Get current service
	service, _, err := sb.dockerClient.ServiceInspectWithRaw(ctx, sb.serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to inspect service: %w", err)
	}

	// Check if there's already a leader
	currentLeader, leaderExists := service.Spec.Labels[sb.leaderLabel]
	if leaderExists {
		// Check if leader lease is still valid
		timestampStr, timestampExists := service.Spec.Labels[sb.timestampLabel]
		if timestampExists {
			timestamp, err := time.Parse(time.RFC3339, timestampStr)
			if err == nil {
				// Leader lease is valid if within LeaseDuration
				if time.Since(timestamp) < sb.config.LeaseDuration {
					// Leader exists and lease is valid
					log.Printf("[LeaderElection:Swarm] Leader exists: %s (age: %v)", currentLeader, time.Since(timestamp))
					return false, nil
				}
				log.Printf("[LeaderElection:Swarm] Leader lease expired for %s (age: %v)", currentLeader, time.Since(timestamp))
			}
		}
	}

	// Try to acquire leadership by setting label
	if service.Spec.Labels == nil {
		service.Spec.Labels = make(map[string]string)
	}
	service.Spec.Labels[sb.leaderLabel] = sb.taskID
	service.Spec.Labels[sb.timestampLabel] = time.Now().Format(time.RFC3339)

	// Update service with version check (atomic operation)
	updateOpts := types.ServiceUpdateOptions{}
	_, err = sb.dockerClient.ServiceUpdate(
		ctx,
		sb.serviceID,
		service.Version,
		service.Spec,
		updateOpts,
	)
	if err != nil {
		// Update failed, likely due to concurrent update
		log.Printf("[LeaderElection:Swarm] Failed to acquire leadership: %v", err)
		return false, nil
	}

	log.Printf("[LeaderElection:Swarm] Acquired leadership (task: %s, ttl: %s)",
		sb.taskID, sb.config.LeaseDuration)
	return true, nil
}

// Renew renews the leadership lease by updating the timestamp label.
//
// Only succeeds if we are the current leader (label value matches our task ID).
func (sb *swarmBackend) Renew(ctx context.Context) error {
	// Get current service
	service, _, err := sb.dockerClient.ServiceInspectWithRaw(ctx, sb.serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return fmt.Errorf("failed to inspect service: %w", err)
	}

	// Check if we are the leader
	currentLeader, exists := service.Spec.Labels[sb.leaderLabel]
	if !exists || currentLeader != sb.taskID {
		return fmt.Errorf("not the current leader (current: %s, us: %s)", currentLeader, sb.taskID)
	}

	// Update timestamp
	service.Spec.Labels[sb.timestampLabel] = time.Now().Format(time.RFC3339)

	// Update service
	updateOpts := types.ServiceUpdateOptions{}
	_, err = sb.dockerClient.ServiceUpdate(
		ctx,
		sb.serviceID,
		service.Version,
		service.Spec,
		updateOpts,
	)
	if err != nil {
		return fmt.Errorf("failed to renew lease: %w", err)
	}

	return nil
}

// Release releases the leadership lock.
//
// Removes the leader labels from the service.
// Only removes if we are the current leader.
func (sb *swarmBackend) Release(ctx context.Context) error {
	// Get current service
	service, _, err := sb.dockerClient.ServiceInspectWithRaw(ctx, sb.serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return fmt.Errorf("failed to inspect service: %w", err)
	}

	// Check if we are the leader
	currentLeader, exists := service.Spec.Labels[sb.leaderLabel]
	if !exists {
		log.Println("[LeaderElection:Swarm] No leader set, nothing to release")
		return nil
	}

	if currentLeader != sb.taskID {
		log.Printf("[LeaderElection:Swarm] Not the leader (current: %s, us: %s), nothing to release", currentLeader, sb.taskID)
		return nil
	}

	// Remove leader labels
	delete(service.Spec.Labels, sb.leaderLabel)
	delete(service.Spec.Labels, sb.timestampLabel)

	// Update service
	updateOpts := types.ServiceUpdateOptions{}
	_, err = sb.dockerClient.ServiceUpdate(
		ctx,
		sb.serviceID,
		service.Version,
		service.Spec,
		updateOpts,
	)
	if err != nil {
		return fmt.Errorf("failed to release leadership: %w", err)
	}

	log.Printf("[LeaderElection:Swarm] Released leadership (task: %s)", sb.taskID)
	return nil
}

// GetLeader returns the current leader's task ID.
//
// Reads the leader label from the service.
func (sb *swarmBackend) GetLeader(ctx context.Context) (string, error) {
	service, _, err := sb.dockerClient.ServiceInspectWithRaw(ctx, sb.serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to inspect service: %w", err)
	}

	leader, exists := service.Spec.Labels[sb.leaderLabel]
	if !exists {
		return "", nil // No leader
	}

	// Check if lease is still valid
	timestampStr, timestampExists := service.Spec.Labels[sb.timestampLabel]
	if timestampExists {
		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err == nil {
			if time.Since(timestamp) > sb.config.LeaseDuration {
				// Lease expired
				log.Printf("[LeaderElection:Swarm] Leader %s lease expired (age: %v)", leader, time.Since(timestamp))
				return "", nil
			}
		}
	}

	return leader, nil
}

// Close cleans up backend resources.
func (sb *swarmBackend) Close() error {
	// Release leadership if we hold it
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sb.Release(ctx); err != nil {
		log.Printf("[LeaderElection:Swarm] Error releasing leadership: %v", err)
	}

	// Close Docker client
	return sb.dockerClient.Close()
}
