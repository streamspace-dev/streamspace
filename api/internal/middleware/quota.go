// Package middleware - quota.go
//
// This file implements resource quota enforcement at the API level.
//
// The quota middleware provides the HTTP layer integration for StreamSpace's
// resource quota system, preventing users from exceeding their allocated
// CPU, memory, GPU, and session limits.
//
// # Why Quota Enforcement is Critical
//
// Without quotas, a single user could:
//   - Consume all cluster resources (DoS to other users)
//   - Launch hundreds of sessions (resource exhaustion)
//   - Request unlimited CPU/memory (cluster instability)
//   - Exceed billing limits (cost overruns)
//
// # Multi-Layered Quota Enforcement
//
// StreamSpace enforces quotas at multiple levels for defense in depth:
//
//	┌─────────────────────────────────────────────────────────┐
//	│  Level 1: API Middleware (This File)                   │
//	│  - Fast rejection before DB writes                     │
//	│  - HTTP 402 (Payment Required) response                │
//	│  - User-friendly error messages                        │
//	└──────────────────────┬──────────────────────────────────┘
//	                       │ Passed
//	                       ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  Level 2: API Handlers (handlers/sessions.go)          │
//	│  - Business logic validation                           │
//	│  - Current usage calculation                           │
//	│  - Quota check with enforcer                           │
//	└──────────────────────┬──────────────────────────────────┘
//	                       │ Passed
//	                       ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  Level 3: Kubernetes Controller                        │
//	│  - Admission webhook validation (future)               │
//	│  - Pod resource limits enforcement                     │
//	│  - Node resource availability check                    │
//	└─────────────────────────────────────────────────────────┘
//
// # Quota Types Enforced
//
// **Per-User Limits**:
//   - MaxSessions: Maximum concurrent sessions (e.g., 10)
//   - MaxCPU: Total CPU across all sessions (e.g., 16 cores)
//   - MaxMemory: Total memory across all sessions (e.g., 64 GB)
//   - MaxGPU: Number of GPU devices (e.g., 2)
//   - MaxStorage: Home directory size (e.g., 100 GB)
//
// **Per-Session Limits**:
//   - MaxCPUPerSession: CPU per session (e.g., 8 cores)
//   - MaxMemoryPerSession: Memory per session (e.g., 32 GB)
//
// # Integration with Quota Enforcer
//
// This middleware is a thin wrapper around quota.Enforcer:
//   - Enforcer contains the core quota logic
//   - Enforcer queries database for user limits
//   - Enforcer calculates current resource usage
//   - Enforcer performs quota math and validation
//
// This middleware just:
//   1. Extracts username from auth context
//   2. Injects enforcer into request context
//   3. Provides helper functions for handlers
//
// # Error Response Format
//
// When quota is exceeded, return HTTP 402 (Payment Required):
//
//	{
//	  "error": "quota_exceeded",
//	  "message": "CPU quota exceeded: requested 4000m, limit 8000m, current usage 5000m",
//	  "quota": {
//	    "limit": "8000m",
//	    "current": "5000m",
//	    "requested": "4000m",
//	    "available": "3000m"
//	  }
//	}
//
// # Usage Pattern
//
// Middleware is applied globally, enforcement is selective:
//
//	// In main.go
//	quotaMiddleware := middleware.NewQuotaMiddleware(enforcer)
//	router.Use(quotaMiddleware.Middleware())
//
//	// In session creation handler
//	err := middleware.EnforceSessionCreation(c, cpu, memory, gpu, currentUsage)
//	if err != nil {
//	    c.JSON(402, gin.H{"error": err.Error()})
//	    return
//	}
//
// # Known Limitations
//
//  1. **Race conditions**: Two concurrent requests might both pass quota check
//     - Solution: Database-level locking in enforcer
//  2. **Stale usage data**: Usage is cached briefly for performance
//     - Solution: Short cache TTL (5 seconds) in enforcer
//  3. **No GPU accounting yet**: GPU quota exists but usage tracking incomplete
//     - Solution: Implement GPU usage tracking in controller
//
// See also:
//   - api/internal/quota/enforcer.go: Core quota enforcement logic
//   - api/internal/handlers/sessions.go: Session creation with quota checks
//   - controller/internal/controllers/session_controller.go: Resource limit enforcement
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/quota"
)

// QuotaMiddleware enforces resource quotas at the API level.
//
// This middleware integrates with quota.Enforcer to provide HTTP-layer
// quota enforcement. It extracts user identity from the request context
// and makes the quota enforcer available to downstream handlers.
//
// **Responsibilities**:
//   - Extract username from auth middleware (c.Get("username"))
//   - Inject quota enforcer into request context
//   - Provide helper functions for quota enforcement
//
// **Non-Responsibilities**:
//   - Does NOT automatically reject requests (handlers decide what to check)
//   - Does NOT calculate current usage (enforcer does that)
//   - Does NOT store quota limits (database does that)
//
// Thread safety: Safe for concurrent use (enforcer is thread-safe)
type QuotaMiddleware struct {
	enforcer *quota.Enforcer
}

// NewQuotaMiddleware creates a new quota middleware instance.
//
// The enforcer parameter contains all the quota enforcement logic including:
//   - Database queries for user limits
//   - Current usage calculation
//   - Quota validation math
//   - Error message generation
//
// This middleware is just a thin HTTP wrapper around the enforcer.
//
// Parameters:
//   - enforcer: The quota enforcer instance (required, must not be nil)
//
// Returns:
//   - QuotaMiddleware ready to be added to Gin router
//
// Example usage:
//
//	enforcer := quota.NewEnforcer(database, k8sClient)
//	quotaMiddleware := middleware.NewQuotaMiddleware(enforcer)
//	router.Use(quotaMiddleware.Middleware())
func NewQuotaMiddleware(enforcer *quota.Enforcer) *QuotaMiddleware {
	return &QuotaMiddleware{
		enforcer: enforcer,
	}
}

// Middleware provides the Gin middleware handler for quota enforcement.
//
// This middleware runs on EVERY request but does not automatically enforce quotas.
// It only prepares the context for downstream handlers to perform quota checks.
//
// # What This Middleware Does
//
// 1. **Extract Username**: Get username from auth middleware context
// 2. **Inject Enforcer**: Store enforcer in request context for handlers
// 3. **Skip Unauthenticated**: Pass through requests without username
//
// # What This Middleware Does NOT Do
//
// - Does NOT reject requests automatically
// - Does NOT query database (deferred to handlers)
// - Does NOT calculate usage (deferred to handlers)
// - Does NOT apply quotas to GET requests (read-only operations)
//
// # Design Rationale: Why Not Auto-Enforce?
//
// **Option 1: Auto-enforce all requests** (rejected):
//   - Problem: Read operations don't consume resources
//   - Problem: Not all requests need quota checks
//   - Problem: Would slow down every request
//
// **Option 2: Middleware just sets up context** (chosen):
//   - Benefit: Fast (no DB queries for reads)
//   - Benefit: Selective (only check when needed)
//   - Benefit: Flexible (handlers decide what to check)
//
// # Context Values Set
//
// The middleware stores these values in Gin context:
//   - "quota_enforcer": The enforcer instance
//   - "quota_username": The authenticated username
//
// Handlers retrieve these with:
//
//	enforcer := c.Get("quota_enforcer").(*quota.Enforcer)
//	username := c.Get("quota_username").(string)
//
// # Performance Characteristics
//
// - Execution time: <0.1ms (just context operations)
// - No database queries
// - No network calls
// - No blocking operations
//
// # Integration with Auth Middleware
//
// This middleware must run AFTER authentication middleware:
//
//	router.Use(middleware.JWTAuth())      // Sets "username"
//	router.Use(quotaMiddleware.Middleware()) // Reads "username"
//
// If auth middleware doesn't set "username", this middleware does nothing
// (allows unauthenticated requests to pass through to auth enforcement layer).
//
// See also:
//   - EnforceSessionCreation(): Helper for quota enforcement in handlers
//   - api/internal/quota/enforcer.go: Core quota logic
func (q *QuotaMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get username from context (set by auth middleware)
		username, exists := c.Get("username")
		if !exists {
			// Skip quota check for unauthenticated requests
			// Auth middleware will reject if authentication is required
			c.Next()
			return
		}

		// Store enforcer in context for handlers to use
		c.Set("quota_enforcer", q.enforcer)
		c.Set("quota_username", username)

		c.Next()
	}
}

// EnforceSessionCreation enforces quotas for session creation requests.
//
// This helper function should be called from session creation handlers to
// validate that the user has sufficient quota to launch the requested session.
//
// # When to Call This
//
// Call this BEFORE creating any Kubernetes resources:
//
//	// ❌ WRONG: Creates session first, then checks quota
//	session := createSession(...)
//	if err := middleware.EnforceSessionCreation(...); err != nil {
//	    deleteSession(session)  // Wasteful
//	}
//
//	// ✅ CORRECT: Checks quota first, then creates session
//	if err := middleware.EnforceSessionCreation(...); err != nil {
//	    return c.JSON(402, gin.H{"error": err.Error()})
//	}
//	session := createSession(...)
//
// # Parameters
//
// **requestedCPU** (string):
//   - CPU request in Kubernetes format (e.g., "2000m", "2", "0.5")
//   - Validates format and converts to millicores
//   - Common values: "1000m" (1 core), "2000m" (2 cores), "500m" (0.5 cores)
//
// **requestedMemory** (string):
//   - Memory request in Kubernetes format (e.g., "2Gi", "512Mi", "1G")
//   - Validates format and converts to bytes
//   - Common values: "2Gi" (2 GB), "4Gi" (4 GB), "512Mi" (512 MB)
//
// **requestedGPU** (int):
//   - Number of GPU devices requested (0 for none)
//   - Each GPU counts as 1 unit
//   - Example: 0 (no GPU), 1 (one GPU), 2 (two GPUs)
//
// **currentUsage** (*quota.Usage):
//   - User's current resource usage across all sessions
//   - If nil, enforcer will query database (slower)
//   - If provided, uses cached value (faster, may be slightly stale)
//
// # Return Value
//
// Returns error if quota check fails:
//   - nil: Quota check passed, proceed with session creation
//   - error: Quota exceeded or validation failed, return HTTP 402
//
// Error message format:
//   "CPU quota exceeded: requested 4000m, limit 8000m, current 5000m"
//   "Invalid CPU format: must be like '1000m' or '2'"
//   "Session limit reached: 10/10 sessions active"
//
// # Quota Check Algorithm
//
// The enforcer performs these checks in order:
//
//  1. **Format validation**: Ensure CPU/memory strings are valid
//  2. **Per-session limits**: Check if request exceeds per-session max
//  3. **Session count**: Check if user has too many active sessions
//  4. **Aggregate CPU**: Check if total CPU (current + requested) exceeds limit
//  5. **Aggregate Memory**: Check if total memory (current + requested) exceeds limit
//  6. **GPU count**: Check if GPU request exceeds limit
//
// If any check fails, returns detailed error with quota information.
//
// # Graceful Degradation
//
// If quota enforcement is not configured, this function allows the request:
//   - No enforcer in context → Allow (quota enforcement disabled)
//   - No username in context → Allow (unauthenticated, auth layer will handle)
//
// This prevents quota failures from breaking the platform if quota feature
// is not configured or temporarily unavailable.
//
// # Performance Considerations
//
// - Database query: 1 query to get user limits (~5ms)
// - If currentUsage provided: No additional queries
// - If currentUsage nil: 1 query to calculate usage (~10ms)
// - Total latency: 5-15ms (acceptable for session creation)
//
// # Example Usage
//
// **In session creation handler**:
//
//	func CreateSession(c *gin.Context) {
//	    var req CreateSessionRequest
//	    if err := c.ShouldBindJSON(&req); err != nil {
//	        c.JSON(400, gin.H{"error": err.Error()})
//	        return
//	    }
//
//	    // Check quota BEFORE creating resources
//	    err := middleware.EnforceSessionCreation(
//	        c,
//	        req.CPU,      // "2000m"
//	        req.Memory,   // "4Gi"
//	        req.GPU,      // 0
//	        nil,          // Let enforcer query current usage
//	    )
//	    if err != nil {
//	        c.JSON(402, gin.H{
//	            "error": "quota_exceeded",
//	            "message": err.Error(),
//	        })
//	        return
//	    }
//
//	    // Quota check passed, proceed with session creation
//	    session := createKubernetesSession(req)
//	    c.JSON(200, session)
//	}
//
// See also:
//   - api/internal/quota/enforcer.go: Core quota enforcement logic
//   - api/internal/handlers/sessions.go: Example usage in session creation
func EnforceSessionCreation(c *gin.Context, requestedCPU, requestedMemory string, requestedGPU int, currentUsage *quota.Usage) error {
	enforcer, exists := c.Get("quota_enforcer")
	if !exists {
		// No enforcer configured, allow request
		// This allows the platform to work without quota enforcement
		return nil
	}

	username, exists := c.Get("quota_username")
	if !exists {
		// No username in context, allow request
		// Auth middleware will reject if authentication is required
		return nil
	}

	quotaEnforcer := enforcer.(*quota.Enforcer)
	usernameStr := username.(string)

	// Parse and validate resource requests
	// This converts "2000m" → 2000, "4Gi" → 4294967296
	cpu, memory, err := quotaEnforcer.ValidateResourceRequest(requestedCPU, requestedMemory)
	if err != nil {
		return err
	}

	// Check quotas against user limits
	// Returns detailed error if any quota is exceeded
	return quotaEnforcer.CheckSessionCreation(c.Request.Context(), usernameStr, cpu, memory, requestedGPU, currentUsage)
}

// GetUserQuota returns a Gin handler that retrieves user quota information.
//
// This handler is typically mounted at GET /api/quotas/me to allow users
// to view their resource limits and current usage.
//
// # Response Format
//
// Returns HTTP 200 with quota information:
//
//	{
//	  "limits": {
//	    "max_sessions": 10,
//	    "max_cpu": "16000m",
//	    "max_memory": "64Gi",
//	    "max_gpu": 2,
//	    "max_storage": "100Gi",
//	    "max_cpu_per_session": "8000m",
//	    "max_memory_per_session": "32Gi",
//	    "current": {
//	      "sessions": 3,
//	      "cpu": "6000m",
//	      "memory": "12Gi",
//	      "gpu": 1,
//	      "storage": "45Gi"
//	    },
//	    "available": {
//	      "sessions": 7,
//	      "cpu": "10000m",
//	      "memory": "52Gi",
//	      "gpu": 1,
//	      "storage": "55Gi"
//	    }
//	  }
//	}
//
// # Error Responses
//
// - HTTP 401 Unauthorized: No username in context (not authenticated)
// - HTTP 500 Internal Server Error: Database error fetching limits
//
// # Authentication
//
// This handler requires authentication (expects "username" in context).
// If username is not present, returns 401 Unauthorized.
//
// # Performance
//
// - Database queries: 2 queries (user limits + current usage)
// - Latency: 10-20ms (typical)
// - Caching: Enforcer may cache limits for 5 seconds
//
// # Example Usage
//
// **Register handler**:
//
//	router.GET("/api/quotas/me", middleware.GetUserQuota(enforcer))
//
// **Frontend usage**:
//
//	fetch('/api/quotas/me')
//	  .then(res => res.json())
//	  .then(data => {
//	    console.log(`Sessions: ${data.limits.current.sessions}/${data.limits.max_sessions}`)
//	    console.log(`CPU: ${data.limits.current.cpu}/${data.limits.max_cpu}`)
//	  })
//
// See also:
//   - api/internal/quota/enforcer.go: GetUserLimits() implementation
func GetUserQuota(enforcer *quota.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, exists := c.Get("username")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		usernameStr := username.(string)

		// Get user limits and current usage from enforcer
		limits, err := enforcer.GetUserLimits(c.Request.Context(), usernameStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to get quota limits",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"limits": limits,
		})
	}
}
