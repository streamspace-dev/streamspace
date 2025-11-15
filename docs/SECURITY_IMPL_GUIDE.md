# Security Implementation Guide - Phase 4 Enhancements

This guide provides ready-to-deploy configurations for all Phase 4 security enhancements.

**Last Updated**: 2025-11-14
**Status**: Phase 4 Implementation

---

## Table of Contents

1. [Runtime Security Monitoring (Falco)](#runtime-security-monitoring-falco)
2. [Security Monitoring Dashboard (Grafana)](#security-monitoring-dashboard-grafana)
3. [Secrets Rotation Automation](#secrets-rotation-automation)
4. [SBOM Generation and Signing](#sbom-generation-and-signing)
5. [File Upload Security](#file-upload-security)
6. [Service Mesh Deployment (Istio)](#service-mesh-deployment-istio)
7. [Web Application Firewall (ModSecurity)](#web-application-firewall-modsecurity)

---

## Runtime Security Monitoring (Falco)

### What is Falco?

Falco is a runtime security tool that detects unexpected behavior in containers and Kubernetes. It provides real-time threat detection for:
- Privilege escalation attempts
- Unexpected network connections
- Filesystem modifications
- Shell spawning in containers
- Sensitive file access

### Deployment

**File**: `manifests/security/falco-deployment.yaml`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: falco
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: falco
  namespace: falco
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: falco
rules:
  - apiGroups: [""]
    resources: ["pods", "namespaces", "nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "daemonsets", "replicasets"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: falco
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: falco
subjects:
  - kind: ServiceAccount
    name: falco
    namespace: falco
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: falco-config
  namespace: falco
data:
  falco.yaml: |
    # Custom rules for StreamSpace
    rules_file:
      - /etc/falco/falco_rules.yaml
      - /etc/falco/falco_rules.local.yaml
      - /etc/falco/rules.d
    
    # Enable JSON output for better parsing
    json_output: true
    json_include_output_property: true
    
    # Logging
    log_stderr: true
    log_syslog: false
    log_level: info
    
    # Output channels
    stdout_output:
      enabled: true
    
    # Falco alerting
    program_output:
      enabled: true
      keep_alive: false
      program: "jq '{text: .output}' | curl -d @- -X POST https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
  
  streamspace_rules.yaml: |
    - rule: Unauthorized Process in StreamSpace Session
      desc: Detect unauthorized processes in session pods
      condition: >
        container.image contains "streamspace"
        and spawned_process
        and not proc.name in (firefox, chromium, code, bash, sh)
      output: >
        Unauthorized process in StreamSpace session
        (user=%user.name command=%proc.cmdline container=%container.name image=%container.image)
      priority: WARNING
      tags: [streamspace, process]
    
    - rule: StreamSpace Privilege Escalation Attempt
      desc: Detect privilege escalation in StreamSpace containers
      condition: >
        container.image contains "streamspace"
        and (proc.name in (sudo, su) or proc.cmdline contains "chmod +s")
      output: >
        Privilege escalation attempt in StreamSpace
        (user=%user.name command=%proc.cmdline container=%container.name)
      priority: CRITICAL
      tags: [streamspace, privilege_escalation]
    
    - rule: Suspicious Network Connection from Session
      desc: Detect unexpected outbound connections
      condition: >
        container.image contains "streamspace"
        and fd.type=ipv4
        and fd.sip != "0.0.0.0"
        and not fd.dip in (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
      output: >
        Suspicious external connection from session
        (connection=%fd.name user=%user.name container=%container.name)
      priority: WARNING
      tags: [streamspace, network]
    
    - rule: Sensitive File Access in Session
      desc: Detect access to sensitive files
      condition: >
        container.image contains "streamspace"
        and (fd.name startswith /etc/passwd or
             fd.name startswith /etc/shadow or
             fd.name contains "id_rsa" or
             fd.name contains "authorized_keys")
      output: >
        Sensitive file access detected
        (file=%fd.name user=%user.name command=%proc.cmdline container=%container.name)
      priority: HIGH
      tags: [streamspace, filesystem]
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: falco
  namespace: falco
  labels:
    app: falco
spec:
  selector:
    matchLabels:
      app: falco
  template:
    metadata:
      labels:
        app: falco
    spec:
      serviceAccountName: falco
      hostNetwork: true
      hostPID: true
      tolerations:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
      containers:
        - name: falco
          image: falcosecurity/falco:0.36.2
          securityContext:
            privileged: true
          args:
            - /usr/bin/falco
            - --cri
            - /run/containerd/containerd.sock
            - -K
            - /var/run/secrets/kubernetes.io/serviceaccount/token
            - -k
            - https://kubernetes.default
            - -pk
          volumeMounts:
            - mountPath: /host/var/run/docker.sock
              name: docker-socket
            - mountPath: /host/run/containerd/containerd.sock
              name: containerd-socket
            - mountPath: /host/dev
              name: dev-fs
            - mountPath: /host/proc
              name: proc-fs
              readOnly: true
            - mountPath: /host/boot
              name: boot-fs
              readOnly: true
            - mountPath: /host/lib/modules
              name: lib-modules
              readOnly: true
            - mountPath: /host/usr
              name: usr-fs
              readOnly: true
            - mountPath: /etc/falco
              name: config-volume
      volumes:
        - name: docker-socket
          hostPath:
            path: /var/run/docker.sock
        - name: containerd-socket
          hostPath:
            path: /run/containerd/containerd.sock
        - name: dev-fs
          hostPath:
            path: /dev
        - name: proc-fs
          hostPath:
            path: /proc
        - name: boot-fs
          hostPath:
            path: /boot
        - name: lib-modules
          hostPath:
            path: /lib/modules
        - name: usr-fs
          hostPath:
            path: /usr
        - name: config-volume
          configMap:
            name: falco-config
```

### Installation

```bash
# Deploy Falco
kubectl apply -f manifests/security/falco-deployment.yaml

# Verify installation
kubectl get pods -n falco

# View Falco logs
kubectl logs -n falco -l app=falco -f

# Test with a security event
kubectl exec -it <some-pod> -- bash
# Falco should alert on unexpected shell access
```

---

## Security Monitoring Dashboard (Grafana)

### Dashboard Configuration

**File**: `manifests/monitoring/grafana-dashboard-security.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboard-security
  namespace: observability
  labels:
    grafana_dashboard: "1"
data:
  security-dashboard.json: |
    {
      "dashboard": {
        "title": "StreamSpace Security Monitoring",
        "panels": [
          {
            "title": "Failed Authentication Attempts (Last Hour)",
            "type": "graph",
            "targets": [{
              "expr": "sum(rate(streamspace_auth_failures_total[5m])) by (reason)"
            }],
            "alert": {
              "conditions": [{
                "evaluator": {"params": [10], "type": "gt"},
                "query": {"params": ["A", "5m", "now"]},
                "type": "query"
              }]
            }
          },
          {
            "title": "Rate Limit Violations",
            "type": "stat",
            "targets": [{
              "expr": "sum(increase(streamspace_rate_limit_exceeded_total[1h]))"
            }]
          },
          {
            "title": "Authorization Failures by Endpoint",
            "type": "table",
            "targets": [{
              "expr": "topk(10, sum by (endpoint, user) (streamspace_authz_failures_total))"
            }]
          },
          {
            "title": "Suspicious API Access Patterns",
            "type": "graph",
            "targets": [{
              "expr": "sum(rate(streamspace_api_requests_total{status=~\"4..\"}[5m])) by (endpoint, method)"
            }]
          },
          {
            "title": "Active Sessions by User",
            "type": "bargauge",
            "targets": [{
              "expr": "sum(streamspace_active_sessions) by (user)"
            }]
          },
          {
            "title": "CSRF Token Validations (Success/Failure)",
            "type": "piechart",
            "targets": [{
              "expr": "sum by (result) (streamspace_csrf_validations_total)"
            }]
          },
          {
            "title": "Security Scan Failures (CI/CD)",
            "type": "stat",
            "targets": [{
              "expr": "github_workflow_run_conclusion{workflow=\"Security Scanning\",conclusion=\"failure\"}"
            }]
          },
          {
            "title": "Certificate Expiration (Days Remaining)",
            "type": "gauge",
            "targets": [{
              "expr": "(cert_exporter_not_after - time()) / 86400"
            }],
            "alert": {
              "conditions": [{
                "evaluator": {"params": [30], "type": "lt"}
              }]
            }
          },
          {
            "title": "Falco Security Alerts",
            "type": "logs",
            "targets": [{
              "expr": "{app=\"falco\"} |= \"priority\""
            }]
          },
          {
            "title": "User Quota Exceeded Events",
            "type": "table",
            "targets": [{
              "expr": "topk(20, sum by (user, quota_type) (streamspace_quota_exceeded_total))"
            }]
          }
        ]
      }
    }
```

---

## Secrets Rotation Automation

### Rotation Script

**File**: `scripts/security/rotate-secrets.sh`

```bash
#!/bin/bash
# StreamSpace Secrets Rotation Script
# Run this monthly/quarterly to rotate all secrets

set -euo pipefail

NAMESPACE="streamspace"
DRY_RUN="${DRY_RUN:-false}"

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

rotate_jwt_secret() {
    log "Rotating JWT secret..."
    
    # Generate new JWT secret
    NEW_JWT_SECRET=$(openssl rand -base64 32)
    
    if [ "$DRY_RUN" = "true" ]; then
        log "DRY RUN: Would update JWT_SECRET"
        return
    fi
    
    # Update Kubernetes secret
    kubectl create secret generic streamspace-api-secrets \
        --from-literal=JWT_SECRET="$NEW_JWT_SECRET" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Restart API pods to pick up new secret
    kubectl rollout restart deployment/streamspace-api -n "$NAMESPACE"
    
    log "JWT secret rotated successfully"
}

rotate_database_password() {
    log "Rotating database password..."
    
    # Generate new password
    NEW_DB_PASSWORD=$(openssl rand -base64 32)
    
    if [ "$DRY_RUN" = "true" ]; then
        log "DRY RUN: Would update DB password"
        return
    fi
    
    # Update PostgreSQL password
    kubectl exec -n "$NAMESPACE" deployment/streamspace-postgres -- \
        psql -U postgres -c "ALTER USER streamspace PASSWORD '$NEW_DB_PASSWORD';"
    
    # Update Kubernetes secret
    kubectl create secret generic streamspace-db-secrets \
        --from-literal=DB_PASSWORD="$NEW_DB_PASSWORD" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Restart API pods
    kubectl rollout restart deployment/streamspace-api -n "$NAMESPACE"
    
    log "Database password rotated successfully"
}

rotate_webhook_secret() {
    log "Rotating webhook secret..."
    
    NEW_WEBHOOK_SECRET=$(openssl rand -hex 32)
    
    if [ "$DRY_RUN" = "true" ]; then
        log "DRY RUN: Would update WEBHOOK_SECRET"
        return
    fi
    
    kubectl create secret generic streamspace-webhook-secrets \
        --from-literal=WEBHOOK_SECRET="$NEW_WEBHOOK_SECRET" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    kubectl rollout restart deployment/streamspace-api -n "$NAMESPACE"
    
    log "Webhook secret rotated successfully"
    log "IMPORTANT: Update webhook secret in external systems!"
}

verify_rotation() {
    log "Verifying rotation..."
    
    # Wait for rollout to complete
    kubectl rollout status deployment/streamspace-api -n "$NAMESPACE" --timeout=5m
    
    # Check if API is healthy
    kubectl wait --for=condition=ready pod -l app=streamspace-api -n "$NAMESPACE" --timeout=5m
    
    log "Rotation verified successfully"
}

main() {
    log "Starting secrets rotation for StreamSpace"
    log "Namespace: $NAMESPACE"
    log "Dry run: $DRY_RUN"
    
    rotate_jwt_secret
    rotate_database_password
    rotate_webhook_secret
    
    if [ "$DRY_RUN" != "true" ]; then
        verify_rotation
    fi
    
    log "Secrets rotation completed successfully!"
    log "Next rotation due: $(date -d '+90 days' +'%Y-%m-%d')"
}

main "$@"
```

### Automated Rotation with CronJob

**File**: `manifests/security/secrets-rotation-cronjob.yaml`

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: secrets-rotation
  namespace: streamspace
spec:
  # Run every 90 days (quarterly)
  schedule: "0 2 1 */3 *"
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: secrets-rotator
          containers:
            - name: rotate
              image: bitnami/kubectl:latest
              command:
                - /bin/bash
                - /scripts/rotate-secrets.sh
              volumeMounts:
                - name: scripts
                  mountPath: /scripts
          volumes:
            - name: scripts
              configMap:
                name: rotation-scripts
          restartPolicy: OnFailure
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: secrets-rotator
  namespace: streamspace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secrets-rotator
  namespace: streamspace
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "create", "update", "patch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "patch"]
  - apiGroups: [""]
    resources: ["pods", "pods/exec"]
    verbs: ["get", "list", "create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: secrets-rotator
  namespace: streamspace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: secrets-rotator
subjects:
  - kind: ServiceAccount
    name: secrets-rotator
    namespace: streamspace
```

---

## SBOM Generation and Signing

### SBOM Workflow

**File**: `.github/workflows/sbom.yml`

```yaml
name: SBOM Generation and Signing

on:
  push:
    branches: [main]
    tags: ['v*']
  release:
    types: [published]

permissions:
  contents: read
  packages: write
  id-token: write  # For Cosign signing

jobs:
  generate-sbom:
    name: Generate and Sign SBOM
    runs-on: ubuntu-latest
    strategy:
      matrix:
        component: [api, ui, controller]
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Build container image
        run: |
          docker build -t stream space-${{ matrix.component }}:sbom ./${{ matrix.component }}
      
      - name: Install Syft
        run: |
          curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
      
      - name: Generate SBOM with Syft
        run: |
          syft streamspace-${{ matrix.component }}:sbom \
            -o spdx-json=sbom-${{ matrix.component }}.spdx.json \
            -o cyclonedx-json=sbom-${{ matrix.component }}.cyclonedx.json
      
      - name: Install Cosign
        uses: sigstore/cosign-installer@v3
      
      - name: Sign SBOM with Cosign
        run: |
          cosign sign-blob \
            --yes \
            sbom-${{ matrix.component }}.spdx.json \
            --output-signature sbom-${{ matrix.component }}.spdx.json.sig \
            --output-certificate sbom-${{ matrix.component }}.spdx.json.pem
      
      - name: Upload SBOM artifacts
        uses: actions/upload-artifact@v4
        with:
          name: sbom-${{ matrix.component }}
          path: |
            sbom-${{ matrix.component }}.*.json
            sbom-${{ matrix.component }}.*.sig
            sbom-${{ matrix.component }}.*.pem
          retention-days: 90
      
      - name: Attach SBOM to container image
        if: github.event_name == 'release'
        run: |
          cosign attach sbom \
            --sbom sbom-${{ matrix.component }}.spdx.json \
            ghcr.io/${{ github.repository }}/streamspace-${{ matrix.component }}:${{ github.ref_name }}
```

---

## File Upload Security

### Upload Security Middleware

**File**: `api/internal/middleware/uploadsecurity.go`

```go
package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
)

// UploadSecurity handles secure file upload validation
type UploadSecurity struct {
	maxFileSize    int64
	allowedTypes   map[string]bool
	scanWithClamAV bool
}

// NewUploadSecurity creates a new upload security validator
func NewUploadSecurity(maxFileSize int64, allowedExtensions []string) *UploadSecurity {
	allowed := make(map[string]bool)
	for _, ext := range allowedExtensions {
		allowed[strings.ToLower(ext)] = true
	}

	return &UploadSecurity{
		maxFileSize:    maxFileSize,
		allowedTypes:   allowed,
		scanWithClamAV: false, // Enable if ClamAV is available
	}
}

// Middleware validates uploaded files
func (us *UploadSecurity) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only process multipart form requests
		if !strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
			c.Next()
			return
		}

		// Parse multipart form
		err := c.Request.ParseMultipartForm(us.maxFileSize)
		if err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "File too large",
				"message": "Uploaded file exceeds maximum size limit",
			})
			c.Abort()
			return
		}

		// Validate each uploaded file
		if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
			for _, files := range c.Request.MultipartForm.File {
				for _, fileHeader := range files {
					// Validate file
					if err := us.validateFile(fileHeader); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{
							"error":    "Invalid file",
							"message":  err.Error(),
							"filename": fileHeader.Filename,
						})
						c.Abort()
						return
					}
				}
			}
		}

		c.Next()
	}
}

// validateFile performs comprehensive file validation
func (us *UploadSecurity) validateFile(fileHeader *multipart.FileHeader) error {
	// 1. Size validation
	if fileHeader.Size > us.maxFileSize {
		return fmt.Errorf("file size %d exceeds maximum %d bytes", fileHeader.Size, us.maxFileSize)
	}

	// 2. Filename sanitization
	filename := filepath.Base(fileHeader.Filename)
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("invalid filename: path traversal detected")
	}

	// 3. Extension validation
	ext := strings.ToLower(filepath.Ext(filename))
	if !us.allowedTypes[ext] {
		return fmt.Errorf("file type not allowed: %s", ext)
	}

	// 4. Magic byte validation (check actual file type, not just extension)
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read first 261 bytes for magic byte detection
	buffer := make([]byte, 261)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect file type from content
	kind, err := filetype.Match(buffer[:n])
	if err != nil {
		return fmt.Errorf("failed to detect file type: %w", err)
	}

	// Verify file type matches extension
	expectedExt := "." + kind.Extension
	if kind != filetype.Unknown && expectedExt != ext {
		return fmt.Errorf("file type mismatch: extension is %s but content is %s", ext, kind.Extension)
	}

	// 5. Scan for malware (if ClamAV enabled)
	if us.scanWithClamAV {
		// Reset file pointer
		file.Seek(0, 0)
		if err := us.scanWithClamAV(file); err != nil {
			return fmt.Errorf("malware detected: %w", err)
		}
	}

	return nil
}

// scanFile scans file with ClamAV (placeholder - implement if needed)
func (us *UploadSecurity) scanFile(file io.Reader) error {
	// Implement ClamAV scanning here
	// Example: use github.com/dutchcoders/go-clamd
	return nil
}
```

---

**This guide continues with more implementations...**

For the complete implementations of:
- Service Mesh (Istio) deployment
- WAF (ModSecurity) configuration
- Incident response procedures

Would you like me to continue with these remaining sections, or shall we commit what we have and create a follow-up implementation plan?

