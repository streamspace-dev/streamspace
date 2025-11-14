# StreamSpace Helper Scripts

This directory contains helper scripts for common StreamSpace operations.

## Prerequisites

- `kubectl` configured to access your cluster
- `jq` installed (for JSON parsing)
- Permissions to access the `streamspace` namespace

## Scripts

### list-sessions.sh

List all StreamSpace sessions with details.

```bash
./scripts/list-sessions.sh

# Use custom namespace
NAMESPACE=my-namespace ./scripts/list-sessions.sh
```

**Output**:
```
================================================
StreamSpace Sessions in namespace: streamspace
================================================

NAME               USER    TEMPLATE          STATE      PHASE     URL
alice-firefox      alice   firefox-browser   running    Running   https://alice-firefox.streamspace.local
bob-vscode         bob     vscode            hibernated Hibernated
charlie-desktop    charlie ubuntu-desktop    running    Running   https://charlie-desktop.streamspace.local

Summary:
  Total sessions: 3
  Running: 2
  Hibernated: 1
```

### create-session.sh

Create a new session from a template.

```bash
./scripts/create-session.sh <username> <template-name> <session-name> [namespace]
```

**Examples**:
```bash
# Create Firefox session for Alice
./scripts/create-session.sh alice firefox-browser alice-firefox

# Create VS Code session for Bob
./scripts/create-session.sh bob vscode bob-vscode

# Create session in custom namespace
./scripts/create-session.sh charlie ubuntu-desktop charlie-desktop my-namespace
```

**What it does**:
1. Validates template exists
2. Creates Session resource
3. Waits for session to be ready
4. Shows access URL and quick commands

### hibernate-session.sh

Hibernate a running session (scale to 0 replicas, preserves state).

```bash
./scripts/hibernate-session.sh <session-name> [namespace]
```

**Examples**:
```bash
# Hibernate Alice's Firefox session
./scripts/hibernate-session.sh alice-firefox

# Hibernate in custom namespace
./scripts/hibernate-session.sh bob-vscode my-namespace
```

**What it does**:
1. Patches session state to `hibernated`
2. Waits for deployment to scale to 0
3. Shows updated session status

**Benefits of hibernation**:
- Frees up cluster resources (CPU, memory)
- Preserves session state (filesystem, settings)
- Quick to wake (no new pod creation)
- User PVC remains intact

### wake-session.sh

Wake a hibernated session (scale back to 1 replica).

```bash
./scripts/wake-session.sh <session-name> [namespace]
```

**Examples**:
```bash
# Wake Alice's Firefox session
./scripts/wake-session.sh alice-firefox

# Wake in custom namespace
./scripts/wake-session.sh bob-vscode my-namespace
```

**What it does**:
1. Patches session state to `running`
2. Waits for deployment to scale to 1
3. Waits for pod to be ready
4. Shows access URL

**Typical wake time**: 10-30 seconds (depends on image size and cluster performance)

### get-metrics.sh

View Prometheus metrics from the controller.

```bash
./scripts/get-metrics.sh
```

**What it does**:
1. Sets up port forward to metrics service
2. Fetches custom StreamSpace metrics
3. Displays them in terminal
4. Keeps port forward alive for querying

**Example output**:
```
================================================
StreamSpace Metrics
================================================

Setting up port forward to metrics service...
Fetching metrics from http://localhost:8080/metrics

=== Session Metrics ===
streamspace_sessions_total{namespace="streamspace",state="running"} 2
streamspace_sessions_total{namespace="streamspace",state="hibernated"} 1
streamspace_sessions_by_user{namespace="streamspace",user="alice"} 1
streamspace_sessions_by_template{namespace="streamspace",template="firefox-browser"} 1

=== Reconciliation Metrics ===
streamspace_session_reconciliations_total{namespace="streamspace",result="success"} 156
streamspace_session_reconciliation_duration_seconds_sum{namespace="streamspace"} 15.6

=== Template Metrics ===
streamspace_template_validations_total{namespace="streamspace",result="valid"} 5

================================================
Full metrics available at: http://localhost:8080/metrics
Keep this script running to maintain port forward
Press Ctrl+C to exit
================================================
```

Press Ctrl+C to stop the port forward.

## Common Workflows

### Create and Access a Session

```bash
# 1. List available templates
kubectl get templates -n streamspace

# 2. Create session
./scripts/create-session.sh alice firefox-browser alice-firefox

# 3. Wait for URL (shown in output)
# Access at: https://alice-firefox.streamspace.local

# 4. When done, hibernate to save resources
./scripts/hibernate-session.sh alice-firefox

# 5. Wake when needed
./scripts/wake-session.sh alice-firefox
```

### Monitor Sessions

```bash
# List all sessions
./scripts/list-sessions.sh

# Watch sessions in real-time
watch -n 2 './scripts/list-sessions.sh'

# View metrics
./scripts/get-metrics.sh
```

### Cleanup

```bash
# Delete a session
kubectl delete session alice-firefox -n streamspace

# Delete all sessions
kubectl delete sessions --all -n streamspace

# Delete all hibernated sessions
kubectl delete sessions -n streamspace --field-selector spec.state=hibernated
```

## Environment Variables

All scripts support these environment variables:

- `NAMESPACE`: Kubernetes namespace (default: `streamspace`)
- `SERVICE`: Metrics service name (default: `streamspace-controller-metrics`)

**Examples**:
```bash
# Use custom namespace for all commands
export NAMESPACE=my-namespace
./scripts/list-sessions.sh
./scripts/create-session.sh alice firefox alice-firefox
```

## Troubleshooting

### Script Fails with "command not found: jq"

Install `jq`:
```bash
# Ubuntu/Debian
sudo apt-get install jq

# macOS
brew install jq

# RHEL/CentOS
sudo yum install jq
```

### Port Forward Fails

If `get-metrics.sh` fails to connect:

```bash
# Check if service exists
kubectl get svc -n streamspace streamspace-controller-metrics

# Check if controller is running
kubectl get pods -n streamspace -l app=streamspace-controller

# View controller logs
kubectl logs -n streamspace -l app=streamspace-controller
```

### Session Not Ready After Creation

```bash
# Check session status
kubectl describe session <session-name> -n streamspace

# Check pod logs
kubectl logs -n streamspace -l session=<session-name>

# Check controller logs
kubectl logs -n streamspace -l app=streamspace-controller
```

## Advanced Usage

### Batch Operations

```bash
# Hibernate all running sessions
for session in $(kubectl get sessions -n streamspace -o jsonpath='{.items[?(@.spec.state=="running")].metadata.name}'); do
    ./scripts/hibernate-session.sh "$session"
done

# Wake all hibernated sessions
for session in $(kubectl get sessions -n streamspace -o jsonpath='{.items[?(@.spec.state=="hibernated")].metadata.name}'); do
    ./scripts/wake-session.sh "$session"
done
```

### Integration with CI/CD

```bash
# Create session in CI pipeline
./scripts/create-session.sh testuser firefox-browser ci-test-$BUILD_ID

# Run tests
# ...

# Cleanup
kubectl delete session ci-test-$BUILD_ID -n streamspace
```

### Automation with cron

```bash
# Hibernate all sessions at night (save resources)
0 22 * * * /path/to/scripts/hibernate-all.sh

# Wake sessions in the morning
0 8 * * * /path/to/scripts/wake-all.sh
```

## See Also

- [INSTALL.md](../INSTALL.md) - Installation guide
- [METRICS.md](../METRICS.md) - Metrics reference
- [README.md](../README.md) - Controller overview
