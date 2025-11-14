# StreamSpace Metrics Guide

This document describes the Prometheus metrics exposed by the StreamSpace controller.

## Metrics Endpoint

The controller exposes Prometheus metrics at:
- **Port**: 8080
- **Path**: `/metrics`

```bash
# Port forward to access metrics
kubectl port-forward -n streamspace svc/streamspace-controller-metrics 8080:8080

# Query metrics
curl http://localhost:8080/metrics | grep streamspace
```

## Custom Metrics

### Session Metrics

#### `streamspace_sessions_total`
**Type**: Gauge
**Description**: Total number of StreamSpace sessions by state
**Labels**:
- `state`: Session state (running, hibernated, terminated)
- `namespace`: Kubernetes namespace

**Example**:
```
streamspace_sessions_total{state="running",namespace="streamspace"} 5
streamspace_sessions_total{state="hibernated",namespace="streamspace"} 2
streamspace_sessions_total{state="terminated",namespace="streamspace"} 0
```

**Use Cases**:
- Monitor active sessions
- Alert on high session counts
- Track hibernation effectiveness

#### `streamspace_sessions_by_user`
**Type**: Gauge
**Description**: Number of StreamSpace sessions by user
**Labels**:
- `user`: Username
- `namespace`: Kubernetes namespace

**Example**:
```
streamspace_sessions_by_user{user="alice",namespace="streamspace"} 3
streamspace_sessions_by_user{user="bob",namespace="streamspace"} 1
```

**Use Cases**:
- Per-user session tracking
- Identify power users
- Enforce user quotas

#### `streamspace_sessions_by_template`
**Type**: Gauge
**Description**: Number of StreamSpace sessions by template
**Labels**:
- `template`: Template name
- `namespace`: Kubernetes namespace

**Example**:
```
streamspace_sessions_by_template{template="firefox-browser",namespace="streamspace"} 4
streamspace_sessions_by_template{template="chrome-browser",namespace="streamspace"} 2
```

**Use Cases**:
- Popular template analytics
- Resource planning
- Template usage optimization

### Reconciliation Metrics

#### `streamspace_session_reconciliations_total`
**Type**: Counter
**Description**: Total number of session reconciliations
**Labels**:
- `namespace`: Kubernetes namespace
- `result`: Reconciliation result (success, error)

**Example**:
```
streamspace_session_reconciliations_total{namespace="streamspace",result="success"} 156
streamspace_session_reconciliations_total{namespace="streamspace",result="error"} 3
```

**Use Cases**:
- Controller health monitoring
- Error rate tracking
- Troubleshooting reconciliation issues

#### `streamspace_session_reconciliation_duration_seconds`
**Type**: Histogram
**Description**: Duration of session reconciliations in seconds
**Labels**:
- `namespace`: Kubernetes namespace

**Buckets**: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10

**Example**:
```
streamspace_session_reconciliation_duration_seconds_bucket{namespace="streamspace",le="0.1"} 142
streamspace_session_reconciliation_duration_seconds_bucket{namespace="streamspace",le="0.5"} 153
streamspace_session_reconciliation_duration_seconds_sum{namespace="streamspace"} 15.6
streamspace_session_reconciliation_duration_seconds_count{namespace="streamspace"} 156
```

**Use Cases**:
- Performance monitoring
- Identify slow reconciliations
- Optimize controller performance

### Template Metrics

#### `streamspace_template_validations_total`
**Type**: Counter
**Description**: Total number of template validations
**Labels**:
- `namespace`: Kubernetes namespace
- `result`: Validation result (valid, invalid)

**Example**:
```
streamspace_template_validations_total{namespace="streamspace",result="valid"} 12
streamspace_template_validations_total{namespace="streamspace",result="invalid"} 1
```

**Use Cases**:
- Template quality monitoring
- Catch configuration errors
- Template catalog health

## Standard Controller-Runtime Metrics

In addition to custom metrics, the controller exposes standard controller-runtime metrics:

### `controller_runtime_reconcile_total`
Reconciliation attempts per controller

### `controller_runtime_reconcile_errors_total`
Reconciliation errors per controller

### `controller_runtime_reconcile_time_seconds`
Reconciliation latency per controller

### `workqueue_*`
Work queue metrics (depth, latency, etc.)

## Prometheus Integration

### ServiceMonitor (Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: streamspace-controller
  namespace: streamspace
  labels:
    app: streamspace-controller
spec:
  selector:
    matchLabels:
      app: streamspace-controller
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

### Prometheus Scrape Config (Manual)

```yaml
scrape_configs:
  - job_name: 'streamspace-controller'
    kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names:
        - streamspace
    relabel_configs:
    - source_labels: [__meta_kubernetes_service_label_app]
      regex: streamspace-controller
      action: keep
    - source_labels: [__meta_kubernetes_endpoint_port_name]
      regex: metrics
      action: keep
```

## Example PromQL Queries

### Active Sessions by State
```promql
streamspace_sessions_total{state="running"}
```

### Session Error Rate
```promql
rate(streamspace_session_reconciliations_total{result="error"}[5m])
```

### Average Reconciliation Duration
```promql
rate(streamspace_session_reconciliation_duration_seconds_sum[5m])
/
rate(streamspace_session_reconciliation_duration_seconds_count[5m])
```

### Top Users by Session Count
```promql
topk(10, sum by(user) (streamspace_sessions_by_user))
```

### Template Popularity
```promql
topk(5, sum by(template) (streamspace_sessions_by_template))
```

### Template Validation Failure Rate
```promql
rate(streamspace_template_validations_total{result="invalid"}[5m])
/
rate(streamspace_template_validations_total[5m])
```

## Grafana Dashboards

### Key Panels

1. **Active Sessions Gauge**
   - Query: `sum(streamspace_sessions_total{state="running"})`
   - Type: Stat panel

2. **Sessions by State**
   - Query: `streamspace_sessions_total`
   - Type: Pie chart

3. **Session Error Rate**
   - Query: `rate(streamspace_session_reconciliations_total{result="error"}[5m])`
   - Type: Graph

4. **Reconciliation Duration**
   - Query: `histogram_quantile(0.95, rate(streamspace_session_reconciliation_duration_seconds_bucket[5m]))`
   - Type: Graph

5. **Top Users**
   - Query: `topk(10, streamspace_sessions_by_user)`
   - Type: Table

6. **Template Usage**
   - Query: `streamspace_sessions_by_template`
   - Type: Bar gauge

## Alerting Rules

### Example Alerts

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: streamspace-alerts
  namespace: streamspace
spec:
  groups:
  - name: streamspace
    interval: 30s
    rules:
    # High error rate
    - alert: StreamSpaceHighErrorRate
      expr: |
        rate(streamspace_session_reconciliations_total{result="error"}[5m]) > 0.1
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "High session reconciliation error rate"
        description: "Session reconciliation error rate is {{ $value }} errors/sec in namespace {{ $labels.namespace }}"

    # Too many active sessions
    - alert: StreamSpaceTooManySessions
      expr: |
        sum(streamspace_sessions_total{state="running"}) > 100
      for: 10m
      labels:
        severity: warning
      annotations:
        summary: "Too many active sessions"
        description: "There are {{ $value }} active sessions, which may impact cluster resources"

    # Slow reconciliations
    - alert: StreamSpaceSlowReconciliations
      expr: |
        histogram_quantile(0.95,
          rate(streamspace_session_reconciliation_duration_seconds_bucket[5m])
        ) > 5
      for: 10m
      labels:
        severity: warning
      annotations:
        summary: "Slow session reconciliations"
        description: "P95 reconciliation duration is {{ $value }}s in namespace {{ $labels.namespace }}"

    # Template validation failures
    - alert: StreamSpaceTemplateValidationFailures
      expr: |
        rate(streamspace_template_validations_total{result="invalid"}[5m]) > 0
      for: 5m
      labels:
        severity: info
      annotations:
        summary: "Template validation failures detected"
        description: "Templates are failing validation in namespace {{ $labels.namespace }}"
```

## Monitoring Best Practices

1. **Set Up Alerts**: Configure alerts for high error rates, resource exhaustion, and performance degradation

2. **Track Trends**: Monitor session growth, template popularity, and user behavior over time

3. **Performance Baselines**: Establish baseline reconciliation duration and track deviations

4. **Capacity Planning**: Use session metrics to forecast resource needs

5. **User Quotas**: Leverage per-user metrics to enforce and monitor quotas

6. **Template Optimization**: Identify unused or problematic templates

## Troubleshooting

### Metrics Not Appearing

```bash
# Check controller logs
kubectl logs -n streamspace deployment/streamspace-controller | grep metrics

# Verify metrics endpoint
kubectl port-forward -n streamspace deployment/streamspace-controller 8080:8080
curl http://localhost:8080/metrics

# Check ServiceMonitor (if using Prometheus Operator)
kubectl get servicemonitor -n streamspace
kubectl describe servicemonitor streamspace-controller -n streamspace
```

### High Memory Usage

Custom metrics with high cardinality (many label combinations) can increase memory usage. Monitor:
- Number of unique users
- Number of unique templates
- Namespace count

Consider using metric relabeling to drop high-cardinality labels if needed.

## Next Steps

- Deploy Grafana dashboard (coming soon)
- Set up alert rules for your environment
- Integrate with existing monitoring stack
- Create custom dashboards for your use cases
