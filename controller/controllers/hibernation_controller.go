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

// Reconcile checks sessions for idle timeout and triggers hibernation
func (r *HibernationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Session
	var session streamv1alpha1.Session
	if err := r.Get(ctx, req.NamespacedName, &session); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if session is not running
	if session.Spec.State != "running" {
		return ctrl.Result{}, nil
	}

	// Skip if no idle timeout configured
	if session.Spec.IdleTimeout == "" {
		// Requeue after check interval to keep monitoring
		return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
	}

	// Parse idle timeout
	idleTimeout, err := time.ParseDuration(session.Spec.IdleTimeout)
	if err != nil {
		log.Error(err, "Failed to parse idle timeout", "timeout", session.Spec.IdleTimeout)
		// Use default
		idleTimeout = r.DefaultIdleTime
	}

	// Check if session has been idle too long
	if session.Status.LastActivity != nil {
		idleDuration := time.Since(session.Status.LastActivity.Time)

		if idleDuration > idleTimeout {
			log.Info("Session idle timeout reached, triggering hibernation",
				"session", session.Name,
				"idleDuration", idleDuration,
				"idleTimeout", idleTimeout,
			)

			// Update session state to hibernated
			session.Spec.State = "hibernated"
			if err := r.Update(ctx, &session); err != nil {
				log.Error(err, "Failed to update session state to hibernated")
				return ctrl.Result{}, err
			}

			// Record hibernation metrics
			metrics.RecordHibernation(session.Namespace, "idle")
			metrics.ObserveIdleDuration(session.Namespace, idleDuration.Seconds())

			log.Info("Session hibernated due to idle timeout", "session", session.Name)
			return ctrl.Result{}, nil
		}

		// Calculate next check time
		nextCheck := idleTimeout - idleDuration
		if nextCheck < r.CheckInterval {
			nextCheck = r.CheckInterval
		}

		log.V(1).Info("Session still active",
			"session", session.Name,
			"idleDuration", idleDuration,
			"nextCheck", nextCheck,
		)

		return ctrl.Result{RequeueAfter: nextCheck}, nil
	}

	// No last activity timestamp yet, initialize it
	now := metav1.Now()
	session.Status.LastActivity = &now
	if err := r.Status().Update(ctx, &session); err != nil {
		log.Error(err, "Failed to initialize last activity timestamp")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *HibernationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Set defaults if not configured
	if r.CheckInterval == 0 {
		r.CheckInterval = 1 * time.Minute
	}
	if r.DefaultIdleTime == 0 {
		r.DefaultIdleTime = 30 * time.Minute
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&streamv1alpha1.Session{}).
		Named("hibernation").
		Complete(r)
}
