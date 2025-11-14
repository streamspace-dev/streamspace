package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// SessionsTotal tracks the total number of sessions by state
	SessionsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "streamspace_sessions_total",
			Help: "Total number of StreamSpace sessions by state",
		},
		[]string{"state", "namespace"},
	)

	// SessionsByUser tracks sessions per user
	SessionsByUser = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "streamspace_sessions_by_user",
			Help: "Number of StreamSpace sessions by user",
		},
		[]string{"user", "namespace"},
	)

	// SessionsByTemplate tracks sessions per template
	SessionsByTemplate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "streamspace_sessions_by_template",
			Help: "Number of StreamSpace sessions by template",
		},
		[]string{"template", "namespace"},
	)

	// SessionReconciliations tracks reconciliation count and status
	SessionReconciliations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "streamspace_session_reconciliations_total",
			Help: "Total number of session reconciliations",
		},
		[]string{"namespace", "result"},
	)

	// SessionReconciliationDuration tracks reconciliation latency
	SessionReconciliationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "streamspace_session_reconciliation_duration_seconds",
			Help:    "Duration of session reconciliations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace"},
	)

	// TemplateValidations tracks template validation results
	TemplateValidations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "streamspace_template_validations_total",
			Help: "Total number of template validations",
		},
		[]string{"namespace", "result"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		SessionsTotal,
		SessionsByUser,
		SessionsByTemplate,
		SessionReconciliations,
		SessionReconciliationDuration,
		TemplateValidations,
	)
}

// RecordSessionState records the current state of a session
func RecordSessionState(state, namespace string, count float64) {
	SessionsTotal.WithLabelValues(state, namespace).Set(count)
}

// RecordSessionByUser records sessions for a user
func RecordSessionByUser(user, namespace string, count float64) {
	SessionsByUser.WithLabelValues(user, namespace).Set(count)
}

// RecordSessionByTemplate records sessions for a template
func RecordSessionByTemplate(template, namespace string, count float64) {
	SessionsByTemplate.WithLabelValues(template, namespace).Set(count)
}

// RecordReconciliation records a reconciliation event
func RecordReconciliation(namespace, result string) {
	SessionReconciliations.WithLabelValues(namespace, result).Inc()
}

// ObserveReconciliationDuration records reconciliation duration
func ObserveReconciliationDuration(namespace string, duration float64) {
	SessionReconciliationDuration.WithLabelValues(namespace).Observe(duration)
}

// RecordTemplateValidation records a template validation
func RecordTemplateValidation(namespace, result string) {
	TemplateValidations.WithLabelValues(namespace, result).Inc()
}
