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

// HibernationReconciler handles automatic hibernation of idle sessions
type HibernationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	CheckInterval   time.Duration
	DefaultIdleTime time.Duration
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
		Complete(r)
}
