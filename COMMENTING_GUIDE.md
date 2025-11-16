# StreamSpace Code Commenting Guide

**Version:** 1.0
**Last Updated:** 2025-11-16
**Purpose:** Comprehensive guide for maintaining high-quality code comments throughout the StreamSpace codebase

---

## Table of Contents

- [Philosophy](#philosophy)
- [Go Code Comments](#go-code-comments)
- [TypeScript/React Comments](#typescriptreact-comments)
- [YAML Comments](#yaml-comments)
- [Python Comments](#python-comments)
- [Examples from Codebase](#examples-from-codebase)
- [Common Patterns](#common-patterns)
- [What NOT to Comment](#what-not-to-comment)
- [Commenting Checklist](#commenting-checklist)

---

## Philosophy

### Core Principles

1. **Explain WHY, not WHAT**: Code shows what it does; comments explain why it's needed and how it solves the problem
2. **Think of Future You**: Write comments as if you'll be debugging this code at 2 AM six months from now
3. **Teach, Don't Repeat**: Comments should teach the reader about the domain, not just restate the code
4. **Comprehensive Over Concise**: Better to over-explain complex logic than leave future maintainers guessing
5. **Examples Over Explanations**: Show concrete examples whenever possible

### When to Add Comments

**ALWAYS comment:**
- Package-level overview explaining the file's purpose
- Complex algorithms (sorting, scheduling, conflict detection)
- Security-critical code (authentication, authorization, input validation)
- Non-obvious business logic (quota calculations, resource allocation)
- Public API functions and methods
- Workarounds for bugs or limitations
- Performance optimizations that aren't immediately obvious

**CONSIDER commenting:**
- Function parameters that aren't self-explanatory
- Return values with special meanings
- Edge cases and error handling
- Database queries with complex joins
- Regular expressions

**DON'T comment:**
- Obvious code (`i++` doesn't need a comment)
- Auto-generated code
- Code that's already self-documenting through clear naming
- Temporary debug code (remove it instead)

---

## Go Code Comments

### Package-Level Documentation

Every Go package should start with a comprehensive package comment explaining:
- What the package does (high-level overview)
- Key concepts and terminology
- How it fits into the larger system
- Important usage notes or gotchas

**Example Pattern** (from `api/internal/handlers/scheduling.go`):

```go
// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements session scheduling and calendar integration features.
//
// SCHEDULING SYSTEM OVERVIEW:
//
// The scheduling system allows users to create sessions that start automatically
// at specific times or on recurring schedules. This is useful for:
// - Regular team meetings or training sessions
// - Pre-warming environments before work hours
// - Demo environments that start/stop on a schedule
// - Resource optimization by scheduling sessions during off-peak hours
//
// SUPPORTED SCHEDULE TYPES:
//
// 1. One-Time (once): Session starts at a specific date/time, runs once
//    - Example: Demo session on Friday at 2 PM
//    - Requires: start_time field
//
// [... continue with detailed explanation ...]
package handlers
```

### Function/Method Documentation

Every exported function should have a comment block explaining:
- What the function does (one-line summary)
- Detailed explanation of the algorithm/logic
- Parameter descriptions
- Return value meanings
- Error conditions
- Example usage
- Security considerations (if applicable)

**Example Pattern** (from `api/internal/handlers/scheduling.go`):

```go
// calculateNextRun calculates when a schedule will next trigger.
//
// This is the core scheduling algorithm that determines when a session should
// be created based on the schedule configuration. The algorithm handles different
// schedule types and properly accounts for timezones.
//
// TIMEZONE HANDLING:
//
// All schedule calculations are performed in the user's specified timezone,
// then converted to UTC for storage. This ensures:
// - 9 AM in New York is always 9 AM local time, even across DST changes
// - Schedules work correctly for users in different timezones
// - Database stores normalized UTC timestamps for consistency
//
// ALGORITHM BY SCHEDULE TYPE:
//
// 1. ONE-TIME ("once"):
//    - Simply returns the start_time field
//    - No calculation needed
//    - Schedule will only run once at that exact time
//
// 2. DAILY ("daily"):
//    - Parses time_of_day (e.g., "09:30" -> 9 hours, 30 minutes)
//    - Creates timestamp for TODAY at that time
//    - If that time has already passed today, schedules for TOMORROW
//    - Example: If now is 2 PM and schedule is 9 AM, next run is tomorrow 9 AM
//
// [... continue with other schedule types ...]
//
// RETURN VALUES:
//
// - time.Time: Next occurrence of the schedule (in user's timezone)
// - error: If schedule cannot be calculated (e.g., invalid cron expression)
//
// EXAMPLES:
//
//	// Daily at 9 AM in New York timezone
//	calculateNextRun(&ScheduleConfig{Type: "daily", TimeOfDay: "09:00"}, "America/New_York")
//	// Returns: tomorrow 9 AM EST if it's after 9 AM today
func (h *Handler) calculateNextRun(schedule *ScheduleConfig, timezone string) (time.Time, error) {
	// Implementation...
}
```

### Inline Comments for Complex Logic

Within functions, add inline comments for:
- Algorithm steps (STEP 1, STEP 2, etc.)
- Complex calculations
- Edge cases
- Security checks
- Performance considerations

**Example Pattern**:

```go
func (h *Handler) checkSchedulingConflicts(userID string, schedule ScheduleConfig, timezone string, terminateAfterMinutes int) ([]int64, error) {
	// STEP 1: Calculate when the proposed schedule will next run
	// This gives us the start time for conflict detection
	proposedStart, err := h.calculateNextRun(&schedule, timezone)
	if err != nil {
		return nil, err
	}

	// STEP 2: Determine session duration
	// Default to 8 hours if not specified (conservative estimate)
	// This prevents conflicts from long-running sessions
	defaultDuration := 8 * time.Hour

	proposedDuration := defaultDuration
	if terminateAfterMinutes > 0 {
		proposedDuration = time.Duration(terminateAfterMinutes) * time.Minute
	}

	// STEP 3: Query all enabled schedules for this user
	// Excludes disabled schedules since they won't actually run
	query := `
		SELECT id, schedule, timezone, terminate_after, next_run_at
		FROM scheduled_sessions
		WHERE user_id = $1 AND enabled = true
	`

	// Implementation continues...
}
```

### Struct Field Comments

Document struct fields, especially:
- Non-obvious field purposes
- Valid value ranges
- Required vs optional fields
- JSON tags and their meanings

**Example Pattern**:

```go
// ScheduledSession represents a scheduled workspace session that starts automatically.
//
// This struct defines a session that will be created at specific times based on
// the configured schedule. Unlike on-demand sessions, scheduled sessions are
// managed by a background scheduler process that monitors next_run_at timestamps.
//
// Lifecycle:
// 1. User creates scheduled session via API
// 2. System calculates next_run_at based on schedule configuration
// 3. Scheduler daemon checks for due schedules every minute
// 4. When next_run_at is reached, system creates actual Session resource
// 5. After session is created, next_run_at is recalculated for recurring schedules
// 6. System optionally terminates session after terminate_after minutes
//
// Example use cases:
// - Development environment that starts at 9 AM and terminates at 6 PM
// - Weekly demo session every Friday at 2 PM
// - Training environment that pre-warms 15 minutes before scheduled time
type ScheduledSession struct {
	ID               int64           `json:"id"`                  // Unique identifier
	UserID           string          `json:"user_id"`             // Owner of this schedule
	TemplateID       string          `json:"template_id"`         // Which template to instantiate
	Name             string          `json:"name"`                // User-friendly name
	Description      string          `json:"description,omitempty"`
	Timezone         string          `json:"timezone"`            // IANA timezone (e.g., "America/New_York")
	Schedule         ScheduleConfig  `json:"schedule"`            // When to run (see ScheduleConfig)
	Resources        ResourceConfig  `json:"resources"`           // CPU/memory allocation
	AutoTerminate    bool            `json:"auto_terminate"`      // Terminate after duration?
	TerminateAfter   int             `json:"terminate_after_minutes,omitempty"` // Minutes after start
	PreWarm          bool            `json:"pre_warm"`            // Start before scheduled time?
	PreWarmMinutes   int             `json:"pre_warm_minutes,omitempty"`
	PostCleanup      bool            `json:"post_cleanup"`        // Cleanup after termination
	Enabled          bool            `json:"enabled"`             // Is this schedule active?
	NextRunAt        time.Time       `json:"next_run_at,omitempty"` // When will it next trigger (UTC)
	LastRunAt        time.Time       `json:"last_run_at,omitempty"` // When did it last run
	LastSessionID    string          `json:"last_session_id,omitempty"` // ID of session created
	LastRunStatus    string          `json:"last_run_status,omitempty"` // "success" or "failed"
	Metadata         map[string]interface{} `json:"metadata,omitempty"` // Custom metadata
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}
```

### Security Comments

Always add `// SECURITY:` comments for security-critical code:

```go
// SECURITY: Force userID to authenticated user (prevent creating schedules for others)
req.UserID = userID

// SECURITY: Validate webhook URL to prevent SSRF attacks
if err := h.validateWebhookURL(webhook.URL); err != nil {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":   "Invalid webhook URL",
		"message": err.Error(),
	})
	return
}

// SECURITY: Returns "not found" whether webhook doesn't exist OR user lacks permission
// This prevents attackers from enumerating which resources exist
var result sql.Result
if role == "admin" {
	result, err = h.DB.Exec("DELETE FROM webhooks WHERE id = $1", webhookID)
} else {
	result, err = h.DB.Exec("DELETE FROM webhooks WHERE id = $1 AND created_by = $2", webhookID, userID)
}
```

---

## TypeScript/React Comments

### Component Documentation (JSDoc Style)

Use JSDoc for React components:

```tsx
/**
 * SessionCard displays a session in the session list or dashboard.
 *
 * This component shows:
 * - Session name and template
 * - Current state (running, hibernated, etc.)
 * - Resource usage (CPU, memory)
 * - Quick actions (open, hibernate, terminate)
 *
 * The card updates in real-time via WebSocket when session state changes.
 *
 * @param props - Component props
 * @param props.session - Session object from API
 * @param props.onOpen - Callback when user clicks to open session
 * @param props.onHibernate - Callback when user hibernates session
 * @param props.onTerminate - Callback when user terminates session
 * @param props.showActions - Whether to show action buttons (default: true)
 *
 * @example
 * <SessionCard
 *   session={sessionData}
 *   onOpen={(id) => navigate(`/sessions/${id}`)}
 *   onHibernate={handleHibernate}
 *   onTerminate={handleTerminate}
 * />
 */
export const SessionCard: React.FC<SessionCardProps> = ({
  session,
  onOpen,
  onHibernate,
  onTerminate,
  showActions = true
}) => {
  // Implementation...
}
```

### Hook Documentation

```tsx
/**
 * useWebSocket establishes a WebSocket connection for real-time updates.
 *
 * This hook:
 * 1. Connects to the WebSocket server on mount
 * 2. Automatically reconnects if connection is lost (exponential backoff)
 * 3. Handles authentication via JWT token
 * 4. Provides send() method for bidirectional communication
 * 5. Cleans up connection on unmount
 *
 * RECONNECTION LOGIC:
 * - First retry: 1 second delay
 * - Second retry: 2 second delay
 * - Third retry: 4 second delay
 * - Max retry: 30 second delay
 * - Stops retrying after 10 consecutive failures
 *
 * @param url - WebSocket URL (e.g., "wss://api.streamspace.local/ws")
 * @param options - Configuration options
 * @param options.autoConnect - Connect immediately on mount (default: true)
 * @param options.onMessage - Callback for incoming messages
 * @param options.onError - Callback for connection errors
 * @param options.onReconnect - Callback when reconnection succeeds
 *
 * @returns WebSocket state and methods
 * @returns returns.connected - Whether WebSocket is currently connected
 * @returns returns.send - Function to send messages to server
 * @returns returns.disconnect - Function to manually close connection
 * @returns returns.reconnect - Function to manually trigger reconnection
 *
 * @example
 * const { connected, send } = useWebSocket('wss://api.example.com/ws', {
 *   onMessage: (data) => console.log('Received:', data),
 *   onError: (error) => console.error('WS Error:', error)
 * });
 *
 * // Send a message
 * send({ type: 'subscribe', channel: 'sessions' });
 */
export function useWebSocket(url: string, options: WebSocketOptions) {
  // Implementation...
}
```

### Complex Logic Comments

```tsx
const QuotaStatus: React.FC<QuotaStatusProps> = ({ userId }) => {
  const [quotaData, setQuotaData] = useState(null);

  useEffect(() => {
    // STEP 1: Fetch quota data from API
    // We fetch on mount and every 30 seconds to keep usage current
    const fetchQuota = async () => {
      try {
        const response = await api.get(`/quotas/users/${userId}/status`);
        setQuotaData(response.data);
      } catch (error) {
        console.error('Failed to fetch quota:', error);
      }
    };

    // STEP 2: Initial fetch
    fetchQuota();

    // STEP 3: Set up polling interval
    // Polling every 30s provides good balance between real-time updates
    // and server load. For critical quota monitoring, WebSocket updates
    // are sent immediately when crossing 80% or 100% thresholds.
    const interval = setInterval(fetchQuota, 30000);

    // STEP 4: Cleanup on unmount
    return () => clearInterval(interval);
  }, [userId]);

  // Calculate color based on quota percentage
  // Green (ok): < 80%
  // Yellow (warning): 80-100%
  // Red (exceeded): > 100%
  const getStatusColor = (percent: number): string => {
    if (percent > 100) return 'red';
    if (percent > 80) return 'yellow';
    return 'green';
  };

  // Implementation continues...
}
```

---

## YAML Comments

### Kubernetes Manifests

```yaml
# ==============================================================================
# StreamSpace Session CRD
# ==============================================================================
#
# This Custom Resource Definition defines the Session resource, which represents
# a user's containerized workspace. Sessions are the core resource in StreamSpace.
#
# IMPORTANT: This is a cluster-wide resource. Changes to this CRD affect ALL
# StreamSpace installations in the cluster.
#
# FIELDS:
# - spec.user: User ID who owns this session (required)
# - spec.template: Template name to instantiate (required)
# - spec.state: Desired state (running|hibernated|terminated)
# - spec.resources: CPU and memory limits
# - spec.persistentHome: Whether to mount user's persistent home directory
#
# LIFECYCLE:
# 1. User creates Session via API or kubectl
# 2. Controller watches for new Sessions
# 3. Controller creates Deployment, Service, PVC
# 4. Controller updates Session.status with pod info
# 5. Session runs until user terminates or auto-hibernates
#
# See docs/ARCHITECTURE.md for detailed explanation
#
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: sessions.stream.space
spec:
  group: stream.space
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                # User who owns this session (enforced by admission webhook)
                user:
                  type: string
                  description: "Username of session owner"
                  minLength: 1
                  maxLength: 253

                # Template to instantiate (must exist in same namespace)
                template:
                  type: string
                  description: "Name of Template to use for this session"
                  minLength: 1

                # Desired session state drives reconciliation logic
                # - running: Deployment scaled to 1 replica, pod scheduled
                # - hibernated: Deployment scaled to 0, state preserved in PVC
                # - terminated: All resources deleted (irreversible)
                state:
                  type: string
                  description: "Desired session state"
                  enum: [running, hibernated, terminated]
                  default: running

                # Resource limits (overrides template defaults)
                # CPU is measured in millicores (1000m = 1 core)
                # Memory is measured in Mi (megabytes)
                resources:
                  type: object
                  properties:
                    memory:
                      type: string
                      pattern: '^[0-9]+(Mi|Gi)$'
                      description: "Memory limit (e.g., '2Gi', '512Mi')"
                      default: "2Gi"
                    cpu:
                      type: string
                      pattern: '^[0-9]+m?$'
                      description: "CPU limit in millicores (e.g., '1000m', '2')"
                      default: "1000m"

                # Whether to mount user's persistent home directory at /config
                # If true, creates PVC named "home-{username}" if it doesn't exist
                persistentHome:
                  type: boolean
                  description: "Mount persistent home directory"
                  default: true

                # Auto-hibernation after idle time (minutes)
                # Controller checks lastActivity timestamp and hibernates if exceeded
                idleTimeout:
                  type: string
                  description: "Idle duration before auto-hibernate (e.g., '30m', '2h')"
                  default: "30m"

            status:
              type: object
              # ... continue with status fields
```

---

## Python Comments

### Module-Level Docstrings

```python
"""
Template Generator for LinuxServer.io Container Images

This script generates StreamSpace Template YAML files for all supported
LinuxServer.io container images. It fetches the container catalog from the
LinuxServer.io API and creates Template CRDs for each image.

USAGE:
    # Generate all templates
    python scripts/generate-templates.py

    # Generate specific category
    python scripts/generate-templates.py --category "Web Browsers"

    # List available categories
    python scripts/generate-templates.py --list-categories

OUTPUT:
    Generated YAML files are written to manifests/templates-generated/
    Organized by category (browsers/, development/, design/, etc.)

TEMPLATE STRUCTURE:
    Each generated Template includes:
    - Metadata: name, labels, annotations
    - Spec: baseImage, ports, env vars, volume mounts
    - VNC config: port 3000 for LinuxServer.io images
    - Resource defaults: 2Gi memory, 1000m CPU

LSIO API:
    Fetches catalog from: https://fleet.linuxserver.io/api/v1/containers
    API returns: name, description, category, image URL, icon

For more details, see docs/TEMPLATE_GENERATION.md
"""

import requests
import yaml
import os
from pathlib import Path
from typing import Dict, List, Optional

# ... rest of the code
```

### Function Docstrings

```python
def generate_template(container: Dict) -> Dict:
    """
    Generate a StreamSpace Template YAML from LinuxServer.io container metadata.

    This function creates a Template CRD with proper VNC configuration for GUI
    applications. LinuxServer.io containers expose VNC on port 3000 by default.

    Args:
        container: Container metadata from LinuxServer.io API
            Required keys: name, image, category
            Optional keys: description, icon, env

    Returns:
        dict: Template manifest ready to serialize to YAML
            Structure matches stream.space/v1alpha1 Template CRD

    Example:
        >>> container = {
        ...     "name": "firefox",
        ...     "image": "lscr.io/linuxserver/firefox:latest",
        ...     "category": "Web Browsers",
        ...     "description": "Firefox web browser"
        ... }
        >>> template = generate_template(container)
        >>> print(template['metadata']['name'])
        firefox-browser

    Note:
        - Template name is sanitized (lowercase, hyphens)
        - VNC port defaults to 3000 for LinuxServer.io images
        - Resource defaults: 2Gi memory, 1000m CPU
        - All templates include PUID=1000, PGID=1000 env vars
    """
    # Sanitize container name for Kubernetes
    # Replace underscores with hyphens, lowercase, remove special chars
    name = container['name'].lower().replace('_', '-')
    name = ''.join(c for c in name if c.isalnum() or c == '-')

    # Create Template manifest
    template = {
        'apiVersion': 'stream.space/v1alpha1',
        'kind': 'Template',
        'metadata': {
            'name': f"{name}-browser",
            'labels': {
                'category': container.get('category', 'Other'),
                'source': 'linuxserver.io'
            }
        },
        'spec': {
            'displayName': container['name'].title(),
            'description': container.get('description', f"{container['name']} from LinuxServer.io"),
            'category': container.get('category', 'Other'),
            'baseImage': container['image'],
            # ... continue with full template structure
        }
    }

    return template
```

---

## Examples from Codebase

### Excellent Examples to Follow

These files demonstrate the commenting standards established:

1. **`api/internal/handlers/scheduling.go`** - Package-level overview + detailed algorithm documentation
   - Lines 1-66: Comprehensive package documentation
   - Lines 150-212: CreateScheduledSession with validation steps
   - Lines 810-918: validateSchedule with per-type documentation
   - Lines 920-1120: calculateNextRun with detailed algorithm explanation

2. **`api/internal/handlers/quotas.go`** - Quota system documentation
   - Lines 1-87: Complete quota system overview
   - Explains hierarchy, enforcement, calculation methods

3. **`api/internal/handlers/integrations.go`** - Security-focused comments
   - Lines 1-40: Security features and fixes documented
   - Lines 69-117: Input validation with security notes
   - SECURITY comments throughout explain threat models

### Files Still Needing Comments

**High Priority** (complex logic, minimal comments):
- `api/internal/handlers/loadbalancing.go` - Node selection algorithms
- `api/internal/auth/jwt.go` - Token generation/validation
- `api/internal/auth/saml.go` - SAML XML parsing
- `api/internal/middleware/ratelimit.go` - Rate limiting algorithm
- `controller/controllers/session_controller.go` - Reconciliation logic
- `controller/controllers/hibernation_controller.go` - Idle detection

**Medium Priority** (moderate complexity):
- All remaining handler files in `api/internal/handlers/`
- Plugin system files in `api/internal/plugins/`
- All 26 plugin implementations in `plugins/`
- React components in `ui/src/components/`
- React pages in `ui/src/pages/`

---

## Common Patterns

### 1. The "STEP" Pattern

For functions with multiple logical steps:

```go
func complexFunction() error {
	// STEP 1: Validate input
	// Ensure all required fields are present before processing
	if err := validateInput(); err != nil {
		return err
	}

	// STEP 2: Fetch data from database
	// Query returns nil if no record found (not an error)
	data, err := fetchData()
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// STEP 3: Transform data
	// Convert API format to internal representation
	result := transformData(data)

	// STEP 4: Store result
	// ACID transaction ensures consistency
	if err := store(result); err != nil {
		return fmt.Errorf("store failed: %w", err)
	}

	return nil
}
```

### 2. The "WHY" Comment

Explain non-obvious decisions:

```go
// Use exponential backoff instead of fixed delay
// This prevents thundering herd when many clients reconnect simultaneously
// First retry: 1s, second: 2s, third: 4s, etc.
delay := time.Duration(math.Pow(2, float64(retryCount))) * time.Second
```

### 3. The "Example" Comment

Show concrete examples:

```go
// parseTimeOfDay converts "HH:MM" string to hours and minutes
// Examples:
//   "09:30" -> 9 hours, 30 minutes
//   "14:00" -> 14 hours, 0 minutes
//   "00:00" -> 0 hours, 0 minutes (midnight)
func parseTimeOfDay(s string) (int, int, error) {
	// Implementation...
}
```

### 4. The "Security" Comment

Always mark security-critical code:

```go
// SECURITY: Hash passwords with bcrypt (cost factor 12)
// Never store plaintext passwords in database
// Cost factor 12 provides good balance between security and performance
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
```

### 5. The "TODO/FIXME/NOTE" Comment

```go
// TODO(username): Implement caching layer for frequently accessed data
// This query runs on every request and could benefit from Redis cache
// Estimated improvement: 200ms -> 10ms response time

// FIXME: Race condition when multiple goroutines access this map
// Need to add mutex or use sync.Map
// Issue #123

// NOTE: This timeout is intentionally short (5s) to fail fast
// Longer timeouts would cause cascading failures during outages
```

---

## What NOT to Comment

### 1. Obvious Code

**Bad:**
```go
// Set i to 0
i := 0

// Increment counter
counter++

// Check if name is empty
if name == "" {
	return ErrEmptyName
}
```

**Good:**
```go
// Reset retry counter for new connection attempt
i := 0

counter++

if name == "" {
	return ErrEmptyName
}
```

### 2. Self-Documenting Code

If the code is already clear, don't add redundant comments:

**Bad:**
```go
// Function to validate email
func ValidateEmail(email string) bool {
	// Check if email matches regex
	return emailRegex.MatchString(email)
}
```

**Good:**
```go
func ValidateEmail(email string) bool {
	return emailRegex.MatchString(email)
}
```

### 3. Change History

Don't use comments for version history (use git instead):

**Bad:**
```go
// 2025-11-01: Changed timeout from 10s to 30s
// 2025-10-15: Added retry logic
// 2025-10-01: Initial implementation
const timeout = 30 * time.Second
```

**Good:**
```go
// Timeout increased to 30s to accommodate slow external APIs
// Most APIs respond within 10s, but some take up to 25s
const timeout = 30 * time.Second
```

---

## Commenting Checklist

Before committing code, verify:

### Package Level
- [ ] Package comment explains what the package does
- [ ] Package comment describes key concepts
- [ ] Package comment includes usage examples (if complex)
- [ ] Package comment notes any important gotchas

### Functions/Methods
- [ ] Exported functions have doc comments
- [ ] Complex algorithms are explained step-by-step
- [ ] Non-obvious parameters are documented
- [ ] Return values are explained
- [ ] Error conditions are documented
- [ ] Example usage is provided (for complex functions)

### Inline Comments
- [ ] Security-critical code has `// SECURITY:` comments
- [ ] Complex logic is broken into steps
- [ ] Edge cases are explained
- [ ] Performance optimizations are justified
- [ ] Workarounds are documented with issue references

### Structs/Types
- [ ] Exported types have doc comments
- [ ] Field purposes are explained (especially non-obvious ones)
- [ ] Valid value ranges are documented
- [ ] Lifecycle/usage patterns are described

### Overall Quality
- [ ] Comments explain WHY, not just WHAT
- [ ] Comments include examples where helpful
- [ ] Comments are up-to-date with code
- [ ] Comments are grammatically correct
- [ ] Comments use consistent formatting

---

## Quick Reference

### Go Comment Syntax

```go
// Single-line comment

/*
Multi-line comment
Block style
*/

// Function doc comment (must be immediately before declaration)
func MyFunc() {}

// godoc formatting:
//   - Indent code examples
//   - Use blank comment line for paragraphs
//   - Start with function name: "MyFunc does X"
```

### JSDoc Comment Syntax

```tsx
/**
 * Component documentation
 *
 * @param props - Props description
 * @param props.field - Field description
 * @returns Return value description
 *
 * @example
 * <Component field="value" />
 */
```

### YAML Comment Syntax

```yaml
# Single-line comment

# Multi-line comment
# Continues on next line
# Still part of same comment block

key: value  # Inline comment
```

---

## Conclusion

High-quality comments are an investment in the future of the codebase. They:
- Reduce onboarding time for new developers
- Prevent bugs by documenting edge cases and gotchas
- Make debugging faster by explaining the "why"
- Serve as inline documentation
- Preserve knowledge when team members leave

When in doubt, err on the side of over-commenting. It's easier to remove verbose comments than to figure out what uncommented code does six months later.

**Remember:** You're not writing comments for yourself today. You're writing them for the developer (maybe future you!) who will maintain this code at 2 AM during a production incident.

Happy commenting! 🎉
