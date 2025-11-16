# Deployment Troubleshooting Guide

This guide covers common issues you might encounter when deploying StreamSpace and their solutions.

## Table of Contents

- [Helm Chart Validation Issues](#helm-chart-validation-issues)
- [Image Pull Issues](#image-pull-issues)
- [Database Connection Issues](#database-connection-issues)
- [CRD Installation Issues](#crd-installation-issues)

---

## Helm Chart Validation Issues

### Issue: "Chart.yaml file is missing" error during helm lint

**Symptoms:**
```bash
==> Linting /path/to/streamspace/chart
[ERROR] templates/: Chart.yaml file is missing
[ERROR] : unable to load chart
	Chart.yaml file is missing

Error: 1 chart(s) linted, 1 chart(s) failed
```

**Affected Versions:**
- Helm v3.19.0 and possibly later versions
- Primarily observed on macOS

**Root Cause:**
This appears to be a regression or change in behavior in Helm v3.19.0 where the chart linting process incorrectly reports that Chart.yaml is missing even though the file exists and is valid.

**Solutions:**

#### Option 1: Skip Validation (Quick Fix)
Use the `SKIP_LINT` environment variable to bypass validation:
```bash
SKIP_LINT=true ./scripts/local-deploy.sh
```

The deployment script will now automatically try an alternative validation method (`helm template`) if `helm lint` fails.

#### Option 2: Downgrade Helm (Recommended for Stability)
Downgrade to Helm v3.18.0 or earlier:

**On macOS (using Homebrew):**
```bash
# Uninstall current version
brew uninstall helm

# Install specific version
brew install helm@3.18.0

# Or use asdf for version management
asdf install helm 3.18.0
asdf global helm 3.18.0
```

**On Linux:**
```bash
# Download specific version
wget https://get.helm.sh/helm-v3.18.0-linux-amd64.tar.gz
tar -zxvf helm-v3.18.0-linux-amd64.tar.gz
sudo mv linux-amd64/helm /usr/local/bin/helm
```

#### Option 3: Use helm template for Validation
Manually validate the chart using `helm template` instead of `helm lint`:
```bash
helm template streamspace ./chart \
  --set controller.image.tag=local \
  --set api.image.tag=local \
  --set ui.image.tag=local \
  --set postgresql.enabled=true \
  --validate > /dev/null
```

If this command succeeds, your chart is valid and you can proceed with installation.

#### Option 4: Direct Installation (Skip lint entirely)
Modify the deployment script to skip the lint step, or install directly with Helm:
```bash
helm install streamspace ./chart \
  --namespace streamspace \
  --create-namespace \
  --set controller.image.tag=local \
  --set controller.image.pullPolicy=Never \
  --set api.image.tag=local \
  --set api.image.pullPolicy=Never \
  --set ui.image.tag=local \
  --set ui.image.pullPolicy=Never \
  --set postgresql.enabled=true \
  --set postgresql.auth.password=streamspace
```

**Verification:**
After installation, verify the deployment is working:
```bash
# Check pod status
kubectl get pods -n streamspace

# Check helm release
helm status streamspace -n streamspace

# Check logs
kubectl logs -n streamspace -l app.kubernetes.io/component=controller -f
```

---

## Image Pull Issues

### Issue: ImagePullBackOff for local images

**Symptoms:**
```
NAME                                      READY   STATUS             RESTARTS   AGE
streamspace-controller-xxxxx              0/1     ImagePullBackOff   0          2m
```

**Cause:**
Kubernetes is trying to pull the image from a registry instead of using the local Docker image.

**Solution:**

1. **Verify images exist locally:**
```bash
docker images | grep streamspace
```

You should see:
```
streamspace/streamspace-controller   local   ...
streamspace/streamspace-api          local   ...
streamspace/streamspace-ui           local   ...
```

2. **Ensure `imagePullPolicy` is set to `Never`:**

The deployment script should set this automatically, but you can verify:
```bash
kubectl get deployment streamspace-controller -n streamspace -o jsonpath='{.spec.template.spec.containers[0].imagePullPolicy}'
```

Should output: `Never`

3. **For Docker Desktop Kubernetes:**

Make sure you're using the same Docker context:
```bash
# Check current context
docker context list

# If needed, switch to default
docker context use default
```

4. **Manual fix if needed:**
```bash
helm upgrade streamspace ./chart \
  --namespace streamspace \
  --reuse-values \
  --set controller.image.pullPolicy=Never \
  --set api.image.pullPolicy=Never \
  --set ui.image.pullPolicy=Never
```

---

## Database Connection Issues

### Issue: API or Controller can't connect to PostgreSQL

**Symptoms:**
```
Error: failed to connect to postgres: dial tcp: lookup streamspace-postgres: no such host
```

**Solutions:**

1. **Verify PostgreSQL is running:**
```bash
kubectl get pods -n streamspace -l app.kubernetes.io/component=database
```

2. **Check PostgreSQL service:**
```bash
kubectl get svc -n streamspace -l app.kubernetes.io/component=database
```

3. **Verify connection from a test pod:**
```bash
kubectl run -it --rm debug --image=postgres:15 --restart=Never -n streamspace -- \
  psql -h streamspace-postgres -U streamspace -d streamspace
```

4. **Check PostgreSQL logs:**
```bash
kubectl logs -n streamspace -l app.kubernetes.io/component=database
```

5. **Verify password secret:**
```bash
kubectl get secret streamspace-secrets -n streamspace -o jsonpath='{.data.postgres-password}' | base64 -d
```

---

## CRD Installation Issues

### Issue: CRDs not found

**Symptoms:**
```
Error from server (NotFound): the server could not find the requested resource (get sessions.stream.streamspace.io)
```

**Solutions:**

1. **Manually install CRDs:**
```bash
kubectl apply -f ./chart/crds/
```

2. **Verify CRDs are installed:**
```bash
kubectl get crds | grep streamspace
```

Expected output:
```
sessions.stream.streamspace.io
templates.stream.streamspace.io
templaterepositories.stream.streamspace.io
connections.stream.streamspace.io
```

3. **Check CRD details:**
```bash
kubectl get crd sessions.stream.streamspace.io -o yaml
```

4. **Reinstall if needed:**
```bash
kubectl delete crd sessions.stream.streamspace.io templates.stream.streamspace.io
kubectl apply -f ./chart/crds/
```

---

## Controller Issues

### Issue: Controller not starting or crash looping

**Symptoms:**
```
NAME                                      READY   STATUS             RESTARTS   AGE
streamspace-controller-xxxxx              0/1     CrashLoopBackOff   5          5m
```

**Debugging Steps:**

1. **Check controller logs:**
```bash
kubectl logs -n streamspace -l app.kubernetes.io/component=controller --tail=100
```

2. **Check for RBAC issues:**
```bash
kubectl auth can-i get deployments --as=system:serviceaccount:streamspace:streamspace-controller -n streamspace
```

3. **Verify service account exists:**
```bash
kubectl get serviceaccount streamspace-controller -n streamspace
```

4. **Check resource limits:**
```bash
kubectl describe pod -n streamspace -l app.kubernetes.io/component=controller
```

5. **Increase verbosity:**
```bash
helm upgrade streamspace ./chart \
  --namespace streamspace \
  --reuse-values \
  --set controller.args.verbosity=2
```

---

## Additional Resources

- **Helm Documentation:** https://helm.sh/docs/
- **Kubernetes Debugging:** https://kubernetes.io/docs/tasks/debug/
- **StreamSpace Architecture:** [ARCHITECTURE.md](./ARCHITECTURE.md)
- **GitHub Issues:** https://github.com/streamspace/streamspace/issues

For further assistance, please open an issue on GitHub with:
1. Output of `kubectl version`
2. Output of `helm version`
3. Relevant logs from affected components
4. Steps to reproduce the issue
