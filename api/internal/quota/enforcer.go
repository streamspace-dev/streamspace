// Package quota provides resource quota enforcement for StreamSpace users and groups.
//
// The quota system prevents resource exhaustion and enforces fair usage limits.
// It supports per-user quotas, per-group quotas, and platform-wide defaults.
//
// Quota types:
//   - Session count: Maximum concurrent sessions
//   - CPU per session: Maximum CPU cores per individual session
//   - Memory per session: Maximum RAM per individual session
//   - Total CPU: Maximum total CPU across all sessions
//   - Total memory: Maximum total RAM across all sessions
//   - Storage: Maximum persistent storage per user
//   - GPU: Maximum GPU count per session
//
// Quota hierarchy (most restrictive wins):
//  1. User-specific quotas (user_quotas table)
//  2. Group quotas (all groups user belongs to)
//  3. Platform defaults (defined in code)
//
// Example limits:
//   - Free tier: 5 sessions, 2 CPU/session, 4 GiB/session, 50 GiB storage
//   - Pro tier: 20 sessions, 4 CPU/session, 8 GiB/session, 500 GiB storage
//   - Enterprise: Custom limits per organization
//
// Enforcement points:
//   - Session creation (CheckSessionCreation)
//   - Resource requests (ValidateResourceRequest)
//   - Storage allocation (checked separately)
//
// Example usage:
//
//	enforcer := quota.NewEnforcer(userDB, groupDB)
//
//	// Check if user can create session
//	err := enforcer.CheckSessionCreation(ctx, "user1", 1000, 2048, 0, currentUsage)
//	if quota.IsQuotaExceeded(err) {
//	    return errors.New("quota exceeded")
//	}
package quota

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/streamspace-dev/streamspace/api/internal/db"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Limits represents resource quotas for a user or group.
//
// Limits can be set at multiple levels:
//   - Per-user: Stored in user_quotas table
//   - Per-group: Stored in group_quotas table
//   - Platform default: Defined in GetUserLimits()
//
// When a user belongs to multiple groups, the most restrictive
// limit is applied for each resource type.
//
// Units:
//   - CPU: Millicores (1000m = 1 CPU core)
//   - Memory: MiB (mebibytes, 1024 MiB = 1 GiB)
//   - Storage: GiB (gibibytes)
//   - GPU: Integer count
//
// Example:
//
//	limits := &Limits{
//	    MaxSessions: 10,
//	    MaxCPUPerSession: 2000,      // 2 CPU cores
//	    MaxMemoryPerSession: 4096,   // 4 GiB
//	    MaxTotalCPU: 8000,           // 8 CPU cores total
//	    MaxTotalMemory: 16384,       // 16 GiB total
//	    MaxStorage: 100,             // 100 GiB
//	    MaxGPUPerSession: 1,
//	}
type Limits struct {
	// MaxSessions is the maximum number of concurrent sessions.
	// Example: 5 (free tier), 20 (pro tier), 100 (enterprise)
	MaxSessions int `json:"max_sessions"`

	// MaxCPUPerSession is the maximum CPU per individual session (millicores).
	// Example: 2000 (2 CPU cores)
	MaxCPUPerSession int64 `json:"max_cpu_per_session"`

	// MaxMemoryPerSession is the maximum memory per individual session (MiB).
	// Example: 4096 (4 GiB)
	MaxMemoryPerSession int64 `json:"max_memory_per_session"`

	// MaxTotalCPU is the maximum total CPU across all sessions (millicores).
	// Example: 8000 (8 CPU cores total across all sessions)
	MaxTotalCPU int64 `json:"max_total_cpu"`

	// MaxTotalMemory is the maximum total memory across all sessions (MiB).
	// Example: 16384 (16 GiB total across all sessions)
	MaxTotalMemory int64 `json:"max_total_memory"`

	// MaxStorage is the maximum persistent storage per user (GiB).
	// Example: 50 (50 GiB for home directory)
	MaxStorage int64 `json:"max_storage"`

	// MaxGPUPerSession is the maximum GPU count per individual session.
	// Example: 1 (one GPU per session), 0 (no GPU access)
	MaxGPUPerSession int `json:"max_gpu_per_session"`
}

// Usage represents current resource consumption for a user.
//
// This is calculated from:
//   - Running Kubernetes pods (for sessions, CPU, memory, GPU)
//   - Persistent volume claims (for storage)
//
// Usage is checked before creating new sessions to enforce quotas.
//
// Example:
//
//	usage := &Usage{
//	    ActiveSessions: 3,
//	    TotalCPU: 6000,       // 6 CPU cores in use
//	    TotalMemory: 12288,   // 12 GiB in use
//	    TotalStorage: 45,     // 45 GiB in use
//	    TotalGPU: 2,          // 2 GPUs in use
//	}
type Usage struct {
	// ActiveSessions is the count of currently running sessions.
	// Calculated from pods with status.phase == Running
	ActiveSessions int `json:"active_sessions"`

	// TotalCPU is the total CPU requests across all sessions (millicores).
	// Sum of container.resources.requests.cpu
	TotalCPU int64 `json:"total_cpu"`

	// TotalMemory is the total memory requests across all sessions (MiB).
	// Sum of container.resources.requests.memory
	TotalMemory int64 `json:"total_memory"`

	// TotalStorage is the total persistent storage in use (GiB).
	// Sum of PVC sizes
	TotalStorage int64 `json:"total_storage"`

	// TotalGPU is the total GPU count across all sessions.
	// Sum of container.resources.requests["nvidia.com/gpu"]
	TotalGPU int `json:"total_gpu"`
}

// Enforcer enforces resource quotas for users and groups.
//
// The enforcer:
//   - Retrieves quotas from database (user and group tables)
//   - Calculates effective limits (most restrictive)
//   - Validates resource requests against limits
//   - Checks current usage before allowing new sessions
//
// Thread safety:
//   - Enforcer is stateless and safe for concurrent use
//   - Database queries may run concurrently
//
// Example:
//
//	enforcer := NewEnforcer(userDB, groupDB)
//	limits, _ := enforcer.GetUserLimits(ctx, "user1")
//	fmt.Printf("Max sessions: %d\n", limits.MaxSessions)
type Enforcer struct {
	// userDB provides access to user quota data.
	userDB *db.UserDB

	// groupDB provides access to group quota data.
	groupDB *db.GroupDB
}

// NewEnforcer creates a new quota enforcer instance.
//
// The enforcer is stateless and can be shared across goroutines.
//
// Example:
//
//	enforcer := NewEnforcer(userDB, groupDB)
//	err := enforcer.CheckSessionCreation(ctx, username, cpu, memory, gpu, usage)
func NewEnforcer(userDB *db.UserDB, groupDB *db.GroupDB) *Enforcer {
	return &Enforcer{
		userDB:  userDB,
		groupDB: groupDB,
	}
}

// GetUserLimits retrieves the resource limits for a user
// It combines user-specific limits with group limits (taking the most restrictive)
func (e *Enforcer) GetUserLimits(ctx context.Context, username string) (*Limits, error) {
	// Get user from database
	user, err := e.userDB.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Start with default limits (for free tier users)
	limits := &Limits{
		MaxSessions:         5,
		MaxCPUPerSession:    2000,  // 2 CPU cores
		MaxMemoryPerSession: 4096,  // 4 GiB
		MaxTotalCPU:         4000,  // 4 CPU cores total
		MaxTotalMemory:      8192,  // 8 GiB total
		MaxStorage:          50,    // 50 GiB
		MaxGPUPerSession:    0,     // No GPU by default
	}

	// Override with user-specific limits if set
	if user.Quota != nil {
		if user.Quota.MaxSessions > 0 {
			limits.MaxSessions = user.Quota.MaxSessions
		}
		if user.Quota.MaxCPU != "" {
			// Parse MaxCPU as total CPU (both per-session and total)
			cpu, err := ParseResourceQuantity(user.Quota.MaxCPU, "cpu")
			if err == nil && cpu > 0 {
				limits.MaxCPUPerSession = cpu
				limits.MaxTotalCPU = cpu
			}
		}
		if user.Quota.MaxMemory != "" {
			// Parse MaxMemory as total memory (both per-session and total)
			memory, err := ParseResourceQuantity(user.Quota.MaxMemory, "memory")
			if err == nil && memory > 0 {
				limits.MaxMemoryPerSession = memory
				limits.MaxTotalMemory = memory
			}
		}
		if user.Quota.MaxStorage != "" {
			// Parse MaxStorage
			storage, err := ParseResourceQuantity(user.Quota.MaxStorage, "memory")
			if err == nil && storage > 0 {
				limits.MaxStorage = storage
			}
		}
	}

	// Check group limits and apply the most restrictive
	if len(user.Groups) > 0 {
		for _, groupName := range user.Groups {
			group, err := e.groupDB.GetGroupByName(ctx, groupName)
			if err != nil {
				continue // Skip groups that don't exist
			}

			if group.Quota != nil {
				// Apply most restrictive limits
				if group.Quota.MaxSessions > 0 && group.Quota.MaxSessions < limits.MaxSessions {
					limits.MaxSessions = group.Quota.MaxSessions
				}
				if group.Quota.MaxCPU != "" {
					cpu, err := ParseResourceQuantity(group.Quota.MaxCPU, "cpu")
					if err == nil && cpu > 0 && cpu < limits.MaxCPUPerSession {
						limits.MaxCPUPerSession = cpu
					}
					if err == nil && cpu > 0 && cpu < limits.MaxTotalCPU {
						limits.MaxTotalCPU = cpu
					}
				}
				if group.Quota.MaxMemory != "" {
					memory, err := ParseResourceQuantity(group.Quota.MaxMemory, "memory")
					if err == nil && memory > 0 && memory < limits.MaxMemoryPerSession {
						limits.MaxMemoryPerSession = memory
					}
					if err == nil && memory > 0 && memory < limits.MaxTotalMemory {
						limits.MaxTotalMemory = memory
					}
				}
				if group.Quota.MaxStorage != "" {
					storage, err := ParseResourceQuantity(group.Quota.MaxStorage, "memory")
					if err == nil && storage > 0 && storage < limits.MaxStorage {
						limits.MaxStorage = storage
					}
				}
			}
		}
	}

	return limits, nil
}

// CheckSessionCreation validates if a user can create a new session with the requested resources
func (e *Enforcer) CheckSessionCreation(ctx context.Context, username string, requestedCPU, requestedMemory int64, requestedGPU int, currentUsage *Usage) error {
	limits, err := e.GetUserLimits(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to get user limits: %w", err)
	}

	// Check session count
	if currentUsage.ActiveSessions >= limits.MaxSessions {
		return fmt.Errorf("session quota exceeded: %d/%d sessions active", currentUsage.ActiveSessions, limits.MaxSessions)
	}

	// Check CPU per session
	if requestedCPU > limits.MaxCPUPerSession {
		return fmt.Errorf("CPU quota exceeded: requested %dm, limit is %dm per session", requestedCPU, limits.MaxCPUPerSession)
	}

	// Check memory per session
	if requestedMemory > limits.MaxMemoryPerSession {
		return fmt.Errorf("memory quota exceeded: requested %dMi, limit is %dMi per session", requestedMemory, limits.MaxMemoryPerSession)
	}

	// Check total CPU
	totalCPU := currentUsage.TotalCPU + requestedCPU
	if totalCPU > limits.MaxTotalCPU {
		return fmt.Errorf("total CPU quota exceeded: would use %dm, limit is %dm", totalCPU, limits.MaxTotalCPU)
	}

	// Check total memory
	totalMemory := currentUsage.TotalMemory + requestedMemory
	if totalMemory > limits.MaxTotalMemory {
		return fmt.Errorf("total memory quota exceeded: would use %dMi, limit is %dMi", totalMemory, limits.MaxTotalMemory)
	}

	// Check GPU per session
	if requestedGPU > limits.MaxGPUPerSession {
		return fmt.Errorf("GPU quota exceeded: requested %d, limit is %d per session", requestedGPU, limits.MaxGPUPerSession)
	}

	return nil
}

// CalculateUsage calculates current resource usage from a list of pods
func (e *Enforcer) CalculateUsage(pods []corev1.Pod) *Usage {
	usage := &Usage{}

	for _, pod := range pods {
		// Only count running pods
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		usage.ActiveSessions++

		// Sum up resource requests from all containers
		for _, container := range pod.Spec.Containers {
			// CPU
			if cpu := container.Resources.Requests[corev1.ResourceCPU]; !cpu.IsZero() {
				usage.TotalCPU += cpu.MilliValue()
			}

			// Memory (convert to MiB)
			if memory := container.Resources.Requests[corev1.ResourceMemory]; !memory.IsZero() {
				usage.TotalMemory += memory.Value() / (1024 * 1024)
			}

			// GPU (nvidia.com/gpu)
			if gpu := container.Resources.Requests["nvidia.com/gpu"]; !gpu.IsZero() {
				usage.TotalGPU += int(gpu.Value())
			}
		}
	}

	return usage
}

// ParseResourceQuantity parses a Kubernetes resource quantity string (e.g., "2000m", "4Gi")
func ParseResourceQuantity(quantity string, resourceType string) (int64, error) {
	q, err := resource.ParseQuantity(quantity)
	if err != nil {
		return 0, fmt.Errorf("invalid resource quantity: %w", err)
	}

	switch resourceType {
	case "cpu":
		// Return millicores
		return q.MilliValue(), nil
	case "memory":
		// Return MiB
		return q.Value() / (1024 * 1024), nil
	default:
		return q.Value(), nil
	}
}

// FormatResourceQuantity formats a resource value back to Kubernetes format
func FormatResourceQuantity(value int64, resourceType string) string {
	switch resourceType {
	case "cpu":
		// Convert millicores to string
		return fmt.Sprintf("%dm", value)
	case "memory":
		// Convert MiB to string
		return fmt.Sprintf("%dMi", value)
	default:
		return fmt.Sprintf("%d", value)
	}
}

// ValidateResourceRequest validates that a resource request is within acceptable bounds
func (e *Enforcer) ValidateResourceRequest(cpuStr, memoryStr string) (cpu, memory int64, err error) {
	// Parse CPU
	if cpuStr != "" {
		cpu, err = ParseResourceQuantity(cpuStr, "cpu")
		if err != nil {
			return 0, 0, fmt.Errorf("invalid CPU quantity: %w", err)
		}

		// Minimum 100m (0.1 CPU)
		if cpu < 100 {
			return 0, 0, fmt.Errorf("CPU request too low: minimum 100m")
		}

		// Maximum 64 CPUs (64000m)
		if cpu > 64000 {
			return 0, 0, fmt.Errorf("CPU request too high: maximum 64000m")
		}
	}

	// Parse memory
	if memoryStr != "" {
		memory, err = ParseResourceQuantity(memoryStr, "memory")
		if err != nil {
			return 0, 0, fmt.Errorf("invalid memory quantity: %w", err)
		}

		// Minimum 128Mi
		if memory < 128 {
			return 0, 0, fmt.Errorf("memory request too low: minimum 128Mi")
		}

		// Maximum 512Gi (524288Mi)
		if memory > 524288 {
			return 0, 0, fmt.Errorf("memory request too high: maximum 512Gi")
		}
	}

	return cpu, memory, nil
}

// GetDefaultResources returns default resource requests based on template category
func GetDefaultResources(category string) (cpu, memory string) {
	switch strings.ToLower(category) {
	case "browsers", "web browsers":
		return "1000m", "2048Mi" // 1 CPU, 2 GiB
	case "development", "ide":
		return "2000m", "4096Mi" // 2 CPUs, 4 GiB
	case "design", "graphics":
		return "2000m", "8192Mi" // 2 CPUs, 8 GiB
	case "gaming", "emulation":
		return "2000m", "4096Mi" // 2 CPUs, 4 GiB
	case "productivity", "office":
		return "1000m", "2048Mi" // 1 CPU, 2 GiB
	case "media", "video editing":
		return "4000m", "8192Mi" // 4 CPUs, 8 GiB
	case "ai", "machine learning":
		return "4000m", "16384Mi" // 4 CPUs, 16 GiB
	default:
		return "1000m", "2048Mi" // 1 CPU, 2 GiB (default)
	}
}

// QuotaExceededError represents a quota exceeded error
type QuotaExceededError struct {
	Message string
	Limit   interface{}
	Current interface{}
}

func (e *QuotaExceededError) Error() string {
	return e.Message
}

// IsQuotaExceeded checks if an error is a quota exceeded error
func IsQuotaExceeded(err error) bool {
	_, ok := err.(*QuotaExceededError)
	return ok
}

// ParseGPURequest parses a GPU request from a string
func ParseGPURequest(gpuStr string) (int, error) {
	if gpuStr == "" || gpuStr == "0" {
		return 0, nil
	}

	gpu, err := strconv.Atoi(gpuStr)
	if err != nil {
		return 0, fmt.Errorf("invalid GPU count: %w", err)
	}

	if gpu < 0 {
		return 0, fmt.Errorf("GPU count cannot be negative")
	}

	if gpu > 8 {
		return 0, fmt.Errorf("GPU count too high: maximum 8")
	}

	return gpu, nil
}
