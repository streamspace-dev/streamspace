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

	// HibernationEvents tracks hibernation events
	HibernationEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "streamspace_hibernation_events_total",
			Help: "Total number of session hibernation events",
		},
		[]string{"namespace", "reason"},
	)

	// WakeEvents tracks session wake events
	WakeEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "streamspace_wake_events_total",
			Help: "Total number of session wake events",
		},
		[]string{"namespace"},
	)

	// SessionIdleDuration tracks how long sessions have been idle
	SessionIdleDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "streamspace_session_idle_duration_seconds",
			Help:    "Duration of session idle time before hibernation in seconds",
			Buckets: []float64{60, 300, 600, 1800, 3600, 7200}, // 1m, 5m, 10m, 30m, 1h, 2h
		},
		[]string{"namespace"},
	)

	// ResourceUsage tracks session resource consumption
	ResourceUsageCPU = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "streamspace_session_cpu_usage_millicores",
			Help: "CPU usage of sessions in millicores",
		},
		[]string{"session", "namespace"},
	)

	ResourceUsageMemory = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "streamspace_session_memory_usage_bytes",
			Help: "Memory usage of sessions in bytes",
		},
		[]string{"session", "namespace"},
	)

	// SessionDuration tracks how long sessions have been running
	SessionDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "streamspace_session_duration_seconds",
			Help: "Duration of active sessions in seconds",
		},
		[]string{"session", "namespace"},
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
		HibernationEvents,
		WakeEvents,
		SessionIdleDuration,
		ResourceUsageCPU,
		ResourceUsageMemory,
		SessionDuration,
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

// RecordHibernation records a session hibernation event
func RecordHibernation(namespace, reason string) {
	HibernationEvents.WithLabelValues(namespace, reason).Inc()
}

// RecordWake records a session wake event
func RecordWake(namespace string) {
	WakeEvents.WithLabelValues(namespace).Inc()
}

// ObserveIdleDuration records the idle duration before hibernation
func ObserveIdleDuration(namespace string, duration float64) {
	SessionIdleDuration.WithLabelValues(namespace).Observe(duration)
}

// RecordResourceUsage records CPU and memory usage for a session
func RecordResourceUsage(session, namespace string, cpuMillicores, memoryBytes float64) {
	ResourceUsageCPU.WithLabelValues(session, namespace).Set(cpuMillicores)
	ResourceUsageMemory.WithLabelValues(session, namespace).Set(memoryBytes)
}

// RecordSessionDuration records how long a session has been active
func RecordSessionDuration(session, namespace string, durationSeconds float64) {
	SessionDuration.WithLabelValues(session, namespace).Set(durationSeconds)
}
