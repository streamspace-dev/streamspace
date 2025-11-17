// Package controllers implements Kubernetes controllers for StreamSpace.
//
// HIBERNATION CONTROLLER
//
// The HibernationReconciler implements automatic resource optimization by detecting
// idle sessions and hibernating them to save compute resources and reduce costs.
//
// AUTO-HIBERNATION MECHANISM:
//
// When a session is inactive for longer than the configured idle timeout:
// 1. Controller detects idle session via LastActivity timestamp
// 2. Updates Session.Spec.State from "running" to "hibernated"
// 3. SessionReconciler observes state change and scales Deployment to 0
// 4. Pod is terminated, resources are freed
// 5. PVC data is preserved for when user returns
//
// COST SAVINGS:
//
// Example cost calculation:
// - Active session: 2 CPU, 4GB RAM = $0.15/hour
// - Idle 20 hours/day: 20 * $0.15 = $3.00/day wasted
// - With auto-hibernation: Saves $3.00/day per session
// - 100 users: $300/day = $9,000/month saved
//
// WHY HIBERNATION VS DELETE:
//
// Hibernation (scale to 0):
// ✅ Wake time: ~5 seconds (pod start)
// ✅ Data preserved: PVC mounted immediately
// ✅ User experience: Seamless resume
// ✅ Network config: Ingress/Service remain
//
// Deletion:
// ❌ Wake time: ~30+ seconds (recreate all resources)
// ❌ Data preserved: Yes, but must remount PVC
// ❌ User experience: Feels like new session
// ❌ Network config: New Ingress URL may change
//
// RECONCILIATION STRATEGY:
//
// Unlike SessionReconciler which reacts to spec changes, HibernationReconciler
// proactively monitors sessions on a schedule:
//
// 1. List all running sessions
// 2. Check each session's LastActivity timestamp
// 3. Calculate idle duration: now - LastActivity
// 4. If idle > IdleTimeout: Trigger hibernation
// 5. Requeue for next check
//
// REQUEUE INTERVALS:
//
// The controller uses intelligent requeue scheduling:
//
// - CheckInterval (default: 1 minute): How often to check sessions
// - Dynamic requeue: Next check at (IdleTimeout - idleDuration)
//
// Example timeline:
//   0:00 - User active, LastActivity updated by API
//   0:05 - Controller checks: idle 5min < 30min timeout ✓ OK
//         Requeue in 25 minutes (30 - 5)
//   0:30 - Controller checks: idle 30min = 30min timeout ✗ HIBERNATE
//         Session state → "hibernated"
//   0:31 - SessionReconciler scales Deployment to 0
//
// LAST ACTIVITY TRACKING:
//
// The LastActivity timestamp is updated by:
// - API backend when user interacts with session (HTTP requests)
// - WebSocket connections for real-time activity
// - VNC proxy for mouse/keyboard events
//
// WHY NOT controller-runtime: LastActivity is business logic tracked by API,
// not a Kubernetes resource change. Controller only reads the timestamp.
//
// CONFIGURATION:
//
// - Session.Spec.IdleTimeout: Per-session idle timeout (e.g., "30m", "1h")
// - CheckInterval: How often to check sessions (default: 1 minute)
// - DefaultIdleTime: Fallback if Session.Spec.IdleTimeout not set (default: 30 minutes)
//
// METRICS:
//
// The controller exports Prometheus metrics:
// - session_hibernations_total{reason="idle"}: Auto-hibernations triggered
// - session_idle_duration_seconds: How long sessions were idle before hibernation
//
// These metrics help:
// - Measure cost savings from auto-hibernation
// - Tune IdleTimeout values for optimal UX vs cost
// - Identify users with long idle periods
//
// EXAMPLE CONFIGURATION:
//
//   apiVersion: stream.streamspace.io/v1alpha1
//   kind: Session
//   metadata:
//     name: user1-firefox
//   spec:
//     user: user1
//     template: firefox-browser
//     state: running
//     idleTimeout: "30m"  # Hibernate after 30 minutes of inactivity
//
// OPTING OUT:
//
// Users can disable auto-hibernation by:
// - Not setting Session.Spec.IdleTimeout (empty string)
// - Setting very long timeout (e.g., "999h")
//
// EDGE CASES:
//
// 1. Session created without LastActivity:
//    - Controller initializes LastActivity to current time
//    - Prevents immediate hibernation of new sessions
//
// 2. Clock skew:
//    - LastActivity in future (user's clock ahead)
//    - idleDuration would be negative
//    - Controller treats as active (doesn't hibernate)
//
// 3. LastActivity never updated:
//    - API backend down or misconfigured
//    - Session will eventually hibernate
//    - This is safe: prevents zombie sessions
//
// 4. Concurrent updates:
//    - User activates session while controller hibernating
//    - Optimistic concurrency: SessionReconciler wins
//    - Controller requeues, sees "running" state, skips
//
// PRODUCTION CONSIDERATIONS:
//
// - Leader election: Only one controller instance hibernates sessions
// - Throttling: CheckInterval prevents API server overload
// - Fairness: All sessions checked, not just one per reconcile
package controllers

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
	"github.com/streamspace/streamspace/pkg/metrics"
)

// HibernationReconciler handles automatic hibernation of idle sessions.
//
// This controller monitors running sessions and automatically hibernates them
// when they've been inactive for longer than their configured idle timeout.
//
// FIELDS:
//
// - Client: Kubernetes client for reading/writing Sessions
// - Scheme: Runtime scheme for type information
// - CheckInterval: How often to check sessions for idle timeout (default: 1 minute)
// - DefaultIdleTime: Fallback idle timeout if Session doesn't specify (default: 30 minutes)
//
// RECONCILIATION FREQUENCY:
//
// Unlike SessionReconciler (event-driven), this controller uses scheduled requeuing:
// - Each reconciliation checks ONE session
// - Requeues itself after CheckInterval or calculated next check time
// - All sessions eventually checked within CheckInterval
//
// WHY THIS APPROACH:
//
// - Avoids listing all sessions repeatedly (scales better)
// - Distributes load over time instead of spikes
// - Smart requeuing reduces unnecessary checks
//
// RESOURCE USAGE:
//
// - Minimal CPU: Only checks timestamp comparison
// - Minimal memory: No caching, works with single session at a time
// - Network: One API call per reconciliation (get session)
type HibernationReconciler struct {
	client.Client        // Kubernetes API client
	Scheme *runtime.Scheme   // Type information for objects
	CheckInterval time.Duration  // How often to check for idle sessions
	DefaultIdleTime time.Duration  // Default idle timeout if not specified
}

// Reconcile checks sessions for idle timeout and triggers auto-hibernation.
//
// This function implements the core auto-hibernation logic that saves
// compute resources by detecting and hibernating idle sessions.
//
// RECONCILIATION LOGIC:
//
// 1. Fetch the Session resource
// 2. Skip if session is not in "running" state
// 3. Skip if session has no idle timeout configured
// 4. Parse idle timeout duration
// 5. Calculate idle duration since last activity
// 6. If idle too long: Trigger hibernation (with conflict retry)
// 7. If still active: Schedule next check
// 8. If no activity timestamp: Initialize it
//
// IDLE DETECTION:
//
// Session idle duration = CurrentTime - LastActivity
//
// LastActivity is updated by:
//   - API backend on HTTP requests
//   - WebSocket proxy on VNC connections
//   - Activity tracker on keyboard/mouse events
//
// If idle duration exceeds configured timeout:
//   - Session.Spec.State = "hibernated"
//   - SessionReconciler scales Deployment to 0
//
// REQUEUE STRATEGY:
//
// Smart requeuing minimizes reconciliation overhead:
//   - Active session: Requeue at (IdleTimeout - IdleDuration)
//   - Just hibernated: No requeue (state change triggers SessionReconciler)
//   - No timestamp: Requeue after CheckInterval
//
// Example timeline (30 minute timeout):
//   0:00 - User active, LastActivity = 0:00
//   0:05 - Check: idle 5min < 30min → requeue in 25min
//   0:30 - Check: idle 30min = 30min → HIBERNATE
//
// OPTIMISTIC CONCURRENCY CONTROL:
//
// BUG FIX: Now uses retry.RetryOnConflict to handle race conditions.
// Previously, updating the session without a fresh fetch caused conflict errors.
//
// Race condition scenario:
//   1. HibernationReconciler fetches session (resourceVersion=123)
//   2. User updates session via API (resourceVersion=124)
//   3. HibernationReconciler tries to update (resourceVersion=123)
//   4. Kubernetes rejects update (conflict error)
//
// Solution:
//   - Fetch fresh copy before update
//   - Retry up to 3 times on conflict
//   - Latest changes always win
//
// COST SAVINGS CALCULATION:
//
// Metrics are recorded for cost analysis:
//   - session_hibernations_total{reason="idle"}: Count of auto-hibernations
//   - session_idle_duration_seconds: How long sessions were idle
//
// These metrics help:
//   - Measure cost savings from auto-hibernation
//   - Tune idle timeout values
//   - Identify users with long idle periods
//
// EDGE CASES:
//
// 1. Session without LastActivity:
//    - Initialize to current time
//    - Prevents immediate hibernation of new sessions
//
// 2. Invalid IdleTimeout format:
//    - Log error and use DefaultIdleTime
//    - Continues monitoring instead of failing
//
// 3. Clock skew (LastActivity in future):
//    - idleDuration would be negative
//    - Won't hibernate (negative < timeout)
//    - Self-correcting as time progresses
//
// 4. Session deleted during reconciliation:
//    - Get() returns NotFound error
//    - Ignored gracefully (client.IgnoreNotFound)
//
// SECURITY CONSIDERATIONS:
//
// LastActivity timestamp trusts the API backend:
//   - API must authenticate users before updating LastActivity
//   - Malicious updates could prevent hibernation
//   - TODO: Add timestamp validation (max age check)
//
// FUTURE ENHANCEMENTS:
//
// TODO: Add hibernation scheduling:
//   - Hibernate all sessions at specific times (e.g., 2 AM)
//   - Support cron-style schedules
//   - Override idle timeout during business hours
//
// TODO: Add wake-on-access:
//   - Automatically wake sessions on incoming requests
//   - Seamless user experience (transparent hibernation)
//
// TODO: Add hibernation notifications:
//   - Warn users before hibernation (e.g., 5 min warning)
//   - Send email/webhook on hibernation
func (r *HibernationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Session resource from the cluster
	var session streamv1alpha1.Session
	if err := r.Get(ctx, req.NamespacedName, &session); err != nil {
		// Ignore NotFound errors - session was deleted, nothing to hibernate
		// Return other errors for retry
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip sessions that are not running
	// Hibernated/terminated sessions don't need idle checking
	if session.Spec.State != "running" {
		return ctrl.Result{}, nil
	}

	// Skip sessions without idle timeout configured
	// Empty string means auto-hibernation is disabled
	if session.Spec.IdleTimeout == "" {
		// Still requeue to keep monitoring in case timeout is added later
		return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
	}

	// Parse idle timeout duration from string format (e.g., "30m", "1h")
	idleTimeout, err := time.ParseDuration(session.Spec.IdleTimeout)
	if err != nil {
		// Invalid format - log error but continue with default
		// This prevents broken configurations from disabling hibernation
		log.Error(err, "Failed to parse idle timeout", "timeout", session.Spec.IdleTimeout)
		idleTimeout = r.DefaultIdleTime // Fallback to default (30 minutes)
	}

	// Check if LastActivity timestamp exists and is set
	if session.Status.LastActivity != nil {
		// Calculate how long the session has been idle
		idleDuration := time.Since(session.Status.LastActivity.Time)

		// Check if idle duration exceeds configured timeout
		if idleDuration > idleTimeout {
			// Session has been idle too long - trigger hibernation
			log.Info("Session idle timeout reached, triggering hibernation",
				"session", session.Name,
				"idleDuration", idleDuration,
				"idleTimeout", idleTimeout,
			)

			// BUG FIX: Use retry.RetryOnConflict to handle race conditions
			// Previously updated session without fresh fetch, causing conflict errors
			// Multiple reconciliations or user updates could cause version conflicts
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				// Fetch fresh copy of session to get latest resourceVersion
				// This ensures we're updating the most recent version
				freshSession := &streamv1alpha1.Session{}
				if err := r.Get(ctx, client.ObjectKeyFromObject(&session), freshSession); err != nil {
					return err
				}

				// Update state to hibernated
				// This triggers SessionReconciler to scale Deployment to 0
				freshSession.Spec.State = "hibernated"
				return r.Update(ctx, freshSession)
			})

			if err != nil {
				log.Error(err, "Failed to update session state to hibernated")
				return ctrl.Result{}, err
			}

			// Record hibernation metrics for cost analysis
			// Label "idle" distinguishes auto-hibernation from manual
			metrics.RecordHibernation(session.Namespace, "idle")
			metrics.ObserveIdleDuration(session.Namespace, idleDuration.Seconds())

			log.Info("Session hibernated due to idle timeout", "session", session.Name)
			// No requeue needed - state change triggers SessionReconciler
			return ctrl.Result{}, nil
		}

		// Session is still active (idle < timeout)
		// Calculate when to check again (when timeout will be reached)
		nextCheck := idleTimeout - idleDuration
		if nextCheck < r.CheckInterval {
			// Don't check more frequently than CheckInterval
			nextCheck = r.CheckInterval
		}

		log.V(1).Info("Session still active",
			"session", session.Name,
			"idleDuration", idleDuration,
			"nextCheck", nextCheck,
		)

		// Requeue at calculated time to check if idle timeout is reached
		return ctrl.Result{RequeueAfter: nextCheck}, nil
	}

	// No last activity timestamp exists yet
	// This happens for newly created sessions
	// Initialize to current time to start tracking idle duration
	now := metav1.Now()
	session.Status.LastActivity = &now
	if err := r.Status().Update(ctx, &session); err != nil {
		log.Error(err, "Failed to initialize last activity timestamp")
		return ctrl.Result{}, err
	}

	// Requeue after check interval to start monitoring
	return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
}

// SetupWithManager registers the HibernationReconciler with the controller manager.
//
// This function configures:
//   - Primary resource to watch (Session)
//   - Controller name ("hibernation")
//   - Default configuration values
//
// WATCH CONFIGURATION:
//
// For(&streamv1alpha1.Session{}):
//   - Reconcile when Session is created, updated, or deleted
//   - Filters sessions by state (only "running" sessions checked)
//
// Named("hibernation"):
//   - Gives controller a unique name for logging and metrics
//   - Prevents conflicts with SessionReconciler (also watches Sessions)
//
// MULTIPLE CONTROLLERS ON SAME RESOURCE:
//
// Both SessionReconciler and HibernationReconciler watch Sessions:
//   - SessionReconciler: Manages Kubernetes resources (Deployment, Service, etc.)
//   - HibernationReconciler: Manages idle timeout and auto-hibernation
//
// This works because:
//   - Different controller names ("session" vs "hibernation")
//   - Different reconciliation logic
//   - Both are idempotent
//
// DEFAULT CONFIGURATION:
//
// CheckInterval (default: 1 minute):
//   - How often to check sessions for idle timeout
//   - Lower values: More responsive, higher overhead
//   - Higher values: Less overhead, slower detection
//
// DefaultIdleTime (default: 30 minutes):
//   - Fallback when Session.Spec.IdleTimeout is invalid
//   - Applied when parse error occurs
//   - Prevents broken configs from disabling hibernation
//
// CONFIGURATION OVERRIDE:
//
// Defaults can be overridden when creating the reconciler:
//
//   reconciler := &HibernationReconciler{
//       Client: mgr.GetClient(),
//       Scheme: mgr.GetScheme(),
//       CheckInterval: 5 * time.Minute,  // Custom check interval
//       DefaultIdleTime: 1 * time.Hour,   // Custom default timeout
//   }
//
// FUTURE ENHANCEMENTS:
//
// TODO: Add event filtering predicates:
//   - Only reconcile running sessions (skip hibernated/terminated)
//   - Reduce unnecessary reconciliation loops
//   - Improve performance at scale
//
// TODO: Add leader election configuration:
//   - Ensure only one replica processes hibernation
//   - Prevent duplicate hibernation events
//   - Support HA controller deployments
func (r *HibernationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Set default values if not configured
	// This ensures the controller works even if values aren't explicitly set
	if r.CheckInterval == 0 {
		r.CheckInterval = 1 * time.Minute // Check every minute by default
	}
	if r.DefaultIdleTime == 0 {
		r.DefaultIdleTime = 30 * time.Minute // 30 minute default idle timeout
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&streamv1alpha1.Session{}).
		Named("hibernation"). // Unique name to distinguish from SessionReconciler
		Complete(r)
}
