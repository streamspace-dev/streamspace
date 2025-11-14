package quota

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/streamspace/streamspace/api/internal/db"
	"github.com/streamspace/streamspace/api/internal/models"
)

// Enforcer handles quota enforcement logic
type Enforcer struct {
	userDB  *db.UserDB
	groupDB *db.GroupDB
}

// NewEnforcer creates a new quota enforcer
func NewEnforcer(userDB *db.UserDB, groupDB *db.GroupDB) *Enforcer {
	return &Enforcer{
		userDB:  userDB,
		groupDB: groupDB,
	}
}

// SessionRequest represents a session creation request for quota checking
type SessionRequest struct {
	UserID   string
	Memory   string // e.g., "2Gi", "512Mi"
	CPU      string // e.g., "1000m", "2"
	Storage  string // e.g., "50Gi"
}

// QuotaCheckResult contains the result of a quota check
type QuotaCheckResult struct {
	Allowed       bool
	Reason        string
	CurrentUsage  *QuotaUsage
	RequestedUsage *QuotaUsage
	AvailableQuota *QuotaUsage
}

// QuotaUsage represents resource usage in normalized units
type QuotaUsage struct {
	Sessions int
	CPUMilli int64  // CPU in millicores (1000m = 1 core)
	MemoryMB int64  // Memory in MB
	StorageGB int64 // Storage in GB
}

// CheckSessionQuota verifies if a user can create a session within their quota
func (e *Enforcer) CheckSessionQuota(ctx context.Context, req *SessionRequest) (*QuotaCheckResult, error) {
	// Get user quota
	userQuota, err := e.userDB.GetUserQuota(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user quota: %w", err)
	}

	// Parse requested resources
	requestedCPU, err := parseResourceCPU(req.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU value: %w", err)
	}

	requestedMemory, err := parseResourceMemory(req.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory value: %w", err)
	}

	requestedStorage, err := parseResourceStorage(req.Storage)
	if err != nil {
		return nil, fmt.Errorf("invalid storage value: %w", err)
	}

	// Parse current usage
	usedCPU, _ := parseResourceCPU(userQuota.UsedCPU)
	usedMemory, _ := parseResourceMemory(userQuota.UsedMemory)
	usedStorage, _ := parseResourceStorage(userQuota.UsedStorage)

	// Parse quota limits
	maxCPU, _ := parseResourceCPU(userQuota.MaxCPU)
	maxMemory, _ := parseResourceMemory(userQuota.MaxMemory)
	maxStorage, _ := parseResourceStorage(userQuota.MaxStorage)

	// Build result
	result := &QuotaCheckResult{
		Allowed: true,
		CurrentUsage: &QuotaUsage{
			Sessions:  userQuota.UsedSessions,
			CPUMilli:  usedCPU,
			MemoryMB:  usedMemory,
			StorageGB: usedStorage,
		},
		RequestedUsage: &QuotaUsage{
			Sessions:  1,
			CPUMilli:  requestedCPU,
			MemoryMB:  requestedMemory,
			StorageGB: requestedStorage,
		},
		AvailableQuota: &QuotaUsage{
			Sessions:  userQuota.MaxSessions,
			CPUMilli:  maxCPU,
			MemoryMB:  maxMemory,
			StorageGB: maxStorage,
		},
	}

	// Check session count
	if userQuota.UsedSessions+1 > userQuota.MaxSessions {
		result.Allowed = false
		result.Reason = fmt.Sprintf("session quota exceeded: using %d/%d sessions",
			userQuota.UsedSessions, userQuota.MaxSessions)
		return result, nil
	}

	// Check CPU quota
	if usedCPU+requestedCPU > maxCPU {
		result.Allowed = false
		result.Reason = fmt.Sprintf("CPU quota exceeded: requesting %dm would use %dm/%dm",
			requestedCPU, usedCPU+requestedCPU, maxCPU)
		return result, nil
	}

	// Check memory quota
	if usedMemory+requestedMemory > maxMemory {
		result.Allowed = false
		result.Reason = fmt.Sprintf("memory quota exceeded: requesting %dMB would use %dMB/%dMB",
			requestedMemory, usedMemory+requestedMemory, maxMemory)
		return result, nil
	}

	// Check storage quota
	if usedStorage+requestedStorage > maxStorage {
		result.Allowed = false
		result.Reason = fmt.Sprintf("storage quota exceeded: requesting %dGB would use %dGB/%dGB",
			requestedStorage, usedStorage+requestedStorage, maxStorage)
		return result, nil
	}

	return result, nil
}

// UpdateSessionQuota updates user quota usage when a session is created
func (e *Enforcer) UpdateSessionQuota(ctx context.Context, userID string, memory, cpu, storage string, increment bool) error {
	cpuMilli, err := parseResourceCPU(cpu)
	if err != nil {
		return err
	}

	memoryMB, err := parseResourceMemory(memory)
	if err != nil {
		return err
	}

	storageGB, err := parseResourceStorage(storage)
	if err != nil {
		return err
	}

	sessionDelta := 1
	if !increment {
		sessionDelta = -1
		cpuMilli = -cpuMilli
		memoryMB = -memoryMB
		storageGB = -storageGB
	}

	// Update usage
	return e.userDB.UpdateQuotaUsage(ctx, userID, sessionDelta,
		formatResourceCPU(cpuMilli),
		formatResourceMemory(memoryMB),
		formatResourceStorage(storageGB))
}

// parseResourceCPU parses CPU values like "1000m", "2", "500m" to millicores
func parseResourceCPU(cpu string) (int64, error) {
	if cpu == "" || cpu == "0" {
		return 0, nil
	}

	cpu = strings.TrimSpace(cpu)

	// Handle millicores (e.g., "1000m")
	if strings.HasSuffix(cpu, "m") {
		cpuStr := strings.TrimSuffix(cpu, "m")
		return strconv.ParseInt(cpuStr, 10, 64)
	}

	// Handle cores (e.g., "2" = 2000m)
	cores, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU format: %s", cpu)
	}

	return int64(cores * 1000), nil
}

// parseResourceMemory parses memory values like "2Gi", "512Mi", "1G" to MB
func parseResourceMemory(memory string) (int64, error) {
	if memory == "" || memory == "0" {
		return 0, nil
	}

	memory = strings.TrimSpace(memory)

	// Parse Kubernetes-style memory (Gi, Mi, Gi, Ki, G, M, K)
	var multiplier int64 = 1
	var valueStr string

	if strings.HasSuffix(memory, "Gi") {
		multiplier = 1024 // GiB to MiB
		valueStr = strings.TrimSuffix(memory, "Gi")
	} else if strings.HasSuffix(memory, "Mi") {
		multiplier = 1 // MiB to MiB
		valueStr = strings.TrimSuffix(memory, "Mi")
	} else if strings.HasSuffix(memory, "Ki") {
		multiplier = 1 / 1024 // KiB to MiB (will be fractional)
		valueStr = strings.TrimSuffix(memory, "Ki")
	} else if strings.HasSuffix(memory, "G") {
		multiplier = 1000 // GB to MB (decimal)
		valueStr = strings.TrimSuffix(memory, "G")
	} else if strings.HasSuffix(memory, "M") {
		multiplier = 1 // MB to MB
		valueStr = strings.TrimSuffix(memory, "M")
	} else if strings.HasSuffix(memory, "K") {
		multiplier = 1 / 1000 // KB to MB (will be fractional)
		valueStr = strings.TrimSuffix(memory, "K")
	} else {
		// Assume bytes
		value, err := strconv.ParseInt(memory, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory format: %s", memory)
		}
		return value / (1024 * 1024), nil // bytes to MiB
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory format: %s", memory)
	}

	return int64(value * float64(multiplier)), nil
}

// parseResourceStorage parses storage values like "50Gi", "100G" to GB
func parseResourceStorage(storage string) (int64, error) {
	if storage == "" || storage == "0" {
		return 0, nil
	}

	storage = strings.TrimSpace(storage)

	var multiplier float64 = 1
	var valueStr string

	if strings.HasSuffix(storage, "Ti") {
		multiplier = 1024 // TiB to GiB
		valueStr = strings.TrimSuffix(storage, "Ti")
	} else if strings.HasSuffix(storage, "Gi") {
		multiplier = 1 // GiB to GiB
		valueStr = strings.TrimSuffix(storage, "Gi")
	} else if strings.HasSuffix(storage, "Mi") {
		multiplier = 1.0 / 1024 // MiB to GiB
		valueStr = strings.TrimSuffix(storage, "Mi")
	} else if strings.HasSuffix(storage, "T") {
		multiplier = 1000 // TB to GB (decimal)
		valueStr = strings.TrimSuffix(storage, "T")
	} else if strings.HasSuffix(storage, "G") {
		multiplier = 1 // GB to GB
		valueStr = strings.TrimSuffix(storage, "G")
	} else if strings.HasSuffix(storage, "M") {
		multiplier = 1.0 / 1000 // MB to GB
		valueStr = strings.TrimSuffix(storage, "M")
	} else {
		// Assume bytes
		value, err := strconv.ParseInt(storage, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid storage format: %s", storage)
		}
		return value / (1024 * 1024 * 1024), nil // bytes to GiB
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid storage format: %s", storage)
	}

	return int64(value * multiplier), nil
}

// formatResourceCPU formats millicores to string (e.g., 1000 -> "1000m")
func formatResourceCPU(milli int64) string {
	if milli == 0 {
		return "0"
	}
	return fmt.Sprintf("%dm", milli)
}

// formatResourceMemory formats MB to string (e.g., 1024 -> "1Gi")
func formatResourceMemory(mb int64) string {
	if mb == 0 {
		return "0"
	}
	if mb >= 1024 && mb%1024 == 0 {
		return fmt.Sprintf("%dGi", mb/1024)
	}
	return fmt.Sprintf("%dMi", mb)
}

// formatResourceStorage formats GB to string (e.g., 50 -> "50Gi")
func formatResourceStorage(gb int64) string {
	if gb == 0 {
		return "0"
	}
	if gb >= 1024 && gb%1024 == 0 {
		return fmt.Sprintf("%dTi", gb/1024)
	}
	return fmt.Sprintf("%dGi", gb)
}

// CheckGroupQuota verifies if a group can accommodate a session within their quota
func (e *Enforcer) CheckGroupQuota(ctx context.Context, groupID string, req *SessionRequest) (*QuotaCheckResult, error) {
	// Get group quota
	groupQuota, err := e.groupDB.GetGroupQuota(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group quota: %w", err)
	}

	// Parse requested resources
	requestedCPU, err := parseResourceCPU(req.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU value: %w", err)
	}

	requestedMemory, err := parseResourceMemory(req.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory value: %w", err)
	}

	requestedStorage, err := parseResourceStorage(req.Storage)
	if err != nil {
		return nil, fmt.Errorf("invalid storage value: %w", err)
	}

	// Parse current usage
	usedCPU, _ := parseResourceCPU(groupQuota.UsedCPU)
	usedMemory, _ := parseResourceMemory(groupQuota.UsedMemory)
	usedStorage, _ := parseResourceStorage(groupQuota.UsedStorage)

	// Parse quota limits
	maxCPU, _ := parseResourceCPU(groupQuota.MaxCPU)
	maxMemory, _ := parseResourceMemory(groupQuota.MaxMemory)
	maxStorage, _ := parseResourceStorage(groupQuota.MaxStorage)

	result := &QuotaCheckResult{
		Allowed: true,
		CurrentUsage: &QuotaUsage{
			Sessions:  groupQuota.UsedSessions,
			CPUMilli:  usedCPU,
			MemoryMB:  usedMemory,
			StorageGB: usedStorage,
		},
		RequestedUsage: &QuotaUsage{
			Sessions:  1,
			CPUMilli:  requestedCPU,
			MemoryMB:  requestedMemory,
			StorageGB: requestedStorage,
		},
		AvailableQuota: &QuotaUsage{
			Sessions:  groupQuota.MaxSessions,
			CPUMilli:  maxCPU,
			MemoryMB:  maxMemory,
			StorageGB: maxStorage,
		},
	}

	// Check session count
	if groupQuota.UsedSessions+1 > groupQuota.MaxSessions {
		result.Allowed = false
		result.Reason = fmt.Sprintf("group session quota exceeded: using %d/%d sessions",
			groupQuota.UsedSessions, groupQuota.MaxSessions)
		return result, nil
	}

	// Check CPU quota
	if usedCPU+requestedCPU > maxCPU {
		result.Allowed = false
		result.Reason = fmt.Sprintf("group CPU quota exceeded: requesting %dm would use %dm/%dm",
			requestedCPU, usedCPU+requestedCPU, maxCPU)
		return result, nil
	}

	// Check memory quota
	if usedMemory+requestedMemory > maxMemory {
		result.Allowed = false
		result.Reason = fmt.Sprintf("group memory quota exceeded: requesting %dMB would use %dMB/%dMB",
			requestedMemory, usedMemory+requestedMemory, maxMemory)
		return result, nil
	}

	// Check storage quota
	if usedStorage+requestedStorage > maxStorage {
		result.Allowed = false
		result.Reason = fmt.Sprintf("group storage quota exceeded: requesting %dGB would use %dGB/%dGB",
			requestedStorage, usedStorage+requestedStorage, maxStorage)
		return result, nil
	}

	return result, nil
}
