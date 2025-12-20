# Forge Troubleshooting Guide

Common issues and their solutions when running Forge.

## Table of Contents

- [ZarfPackageJob Issues](#ZarfPackageJob-issues)
- [Policy & Permission Issues](#policy--permission-issues)
- [Job Failures](#job-failures)
- [Webhook Issues](#webhook-issues)
- [Controller Issues](#controller-issues)
- [Debugging Commands](#debugging-commands)

---

## ZarfPackageJob Issues

### ZarfPackageJob stays in Pending phase

**Symptoms:**

```bash
kubectl get ZarfPackageJob my-package
# STATUS shows "Pending" for extended period
```text

**Possible causes:**

1. **ServiceAccount not found**

   ```bash
   kubectl describe ZarfPackageJob my-package
   # Check Events section for "ServiceAccount not found"
   ```

   **Solution:** Create the ServiceAccount or fix the name in spec.serviceAccountName

2. **Policy validation failure**

   ```bash
   kubectl get events --field-selector involvedObject.name=my-package
   ```

   **Solution:** Check ServiceAccount annotations match your requirements (see [Policy Issues](#policy--permission-issues))

3. **Controller not running**

   ```bash
   kubectl get pods -n forge-system
   ```

   **Solution:** Check controller logs and ensure deployment is healthy

### ZarfPackageJob was denied by webhook

**Symptoms:**

```text
Error from server: admission webhook "validate.forge.dev" denied the request
```text

**Solution:**
Check the webhook validation error message for specific policy violation. Common issues:

- Action not in allowed-actions annotation
- Source repository not in allowed-source-repos
- Missing required ServiceAccount annotations

---

## Policy & Permission Issues

### Action not allowed error

**Symptoms:**

```text
action Deploy is not allowed (allowed actions: [Build,Publish]) for ServiceAccount dev-sa
```text

**Solution:**
Update ServiceAccount annotations to include the action:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dev-sa
  annotations:
    forge.dev/allowed-actions: "Build,Publish,Deploy"  # Add Deploy
```text

### Git repository not allowed

**Symptoms:**

```text
Git repo https://github.com/myorg/repo is not allowed (allowed repos: [github.com/other/*])
```text

**Solution:**
Update ServiceAccount to allow the repository pattern:

```yaml
annotations:
  forge.dev/allowed-source-repos: "https://github.com/myorg/*,https://github.com/other/*"
```text

**Note:** Patterns use glob matching. Use `*` for wildcard, e.g., `https://github.com/myorg/*` matches all repos under myorg.

### S3 bucket not allowed

**Symptoms:**

```text
S3 bucket my-prod-bucket is not allowed (allowed buckets: [my-dev-*])
```text

**Solution:**

```yaml
annotations:
  forge.dev/allowed-source-buckets: "my-dev-*,my-prod-*"
  # or for publish:
  forge.dev/allowed-publish-buckets: "my-artifacts-*"
```text

### Local sources denied

**Symptoms:**

```text
local sources are not allowed (set annotation forge.dev/allow-local-sources: true for dev mode)
```text

**Solution (DEV/TEST ONLY):**

```yaml
annotations:
  forge.dev/allow-local-sources: "true"
```text

**Warning:** Only enable for development/testing. Never in production.

---

## Job Failures

### Job pod fails immediately

**Check job logs:**

```bash
# Find the job
kubectl get jobs -l forge.dev/package=my-package

# Get logs
kubectl logs job/my-package-build-xxxxx
```text

**Common causes:**

1. **Missing credentials**

   ```text
   fatal: could not read Username for 'https://github.com': No such device
   ```

   **Solution:** Create Secret with Git credentials and reference in ZarfPackageJob:

   ```yaml
   source:
     git:
       credentialsSecretRef:
         name: github-token
   ```

2. **Invalid S3 credentials**

   ```text
   InvalidAccessKeyId: The AWS Access Key Id you provided does not exist
   ```

   **Solution:** Verify Secret contains correct access-key-id and secret-access-key

3. **OCI auth failure**

   ```text
   unauthorized: authentication required
   ```

   **Solution:** Create kubernetes.io/dockerconfigjson Secret and reference it

### ImagePullBackOff or ErrImagePull

**Symptoms:**

```bash
kubectl get pods -n default
# Shows: STATUS = ImagePullBackOff or ErrImagePull
```

```text
Failed to pull image "localhost/zarf:v0.66.0":
failed to authorize: failed to fetch anonymous token: 403 Forbidden
```

**Cause:**
The Zarf project does not publish container images - they only distribute binaries. The image `localhost/zarf:v0.66.0` referenced in the code doesn't exist in any public registry.

**Solutions:**

1. **Build the Zarf CLI image (Recommended for testing/Kind):**

   Forge includes a Dockerfile that packages the official Zarf CLI binary:

   ```bash
   # Build the image
   docker build -t localhost/zarf:v0.66.0 images/zarf-cli/

   # For Kind clusters, load it
   kind load docker-image localhost/zarf:v0.66.0 --name <cluster-name>

   # For Podman users:
   podman build -t localhost/zarf:v0.66.0 images/zarf-cli/
   podman save localhost/zarf:v0.66.0 -o /tmp/zarf-cli.tar
   kind load image-archive /tmp/zarf-cli.tar --name <cluster-name>
   rm /tmp/zarf-cli.tar
   ```

2. **Build and push to your registry (Recommended for production):**

   ```bash
   # Build and tag for your registry
   docker build -t your-registry.io/zarf:v0.66.0 images/zarf-cli/
   docker push your-registry.io/zarf:v0.66.0

   # Update pkg/actions/build.go to reference your registry
   # Change: ZarfCLIImage = "localhost/zarf:v0.66.0"
   # To: ZarfCLIImage = "your-registry.io/zarf:v0.66.0"
   ```

3. **Configure imagePullSecrets (if you pushed to a private registry):**

   ```bash
   kubectl create secret docker-registry registry-secret \
     --docker-server=your-registry.io \
     --docker-username=<username> \
     --docker-password=<password> \
     -n <namespace>

   # Add to your namespace's default ServiceAccount
   kubectl patch serviceaccount default \
     -n <namespace> \
     -p '{"imagePullSecrets": [{"name": "registry-secret"}]}'
   ```

### Job exceeds active deadline

**Symptoms:**

```text
Job was active longer than specified deadline
```text

**Solution:**

- Build jobs have 1 hour timeout
- Publish/Deploy have 30 minute timeout
- For larger packages, this may need to be increased (requires code change)

**Workaround:**
Split large operations:

1. Build separately
2. Then Publish from the built artifact
3. Then Deploy

### Job resource limits exceeded

**Symptoms:**

```text
OOMKilled
```text

**Solution:**
Current resource limits per action:

- Build: CPU 2-4, Memory 4Gi-8Gi
- Publish: CPU 1-2, Memory 2Gi-4Gi
- Deploy: CPU 1-2, Memory 2Gi-4Gi

If package requires more resources, this needs code adjustment in pkg/actions/

---

## Webhook Issues

### Webhook certificate errors

**Symptoms:**

```text
x509: certificate signed by unknown authority
```text

**Solution:**

1. Ensure cert-manager is installed
2. Apply webhook certificates:

   ```bash
   kubectl apply -f webhook/deploy/certificate.yaml
   ```

3. Wait for cert-manager to issue certificate:

   ```bash
   kubectl get certificate -n forge-system
   ```

### Webhook not responding

**Symptoms:**

```text
context deadline exceeded
```text

**Solution:**
Check webhook pod status:

```bash
kubectl get pods -n forge-system -l app=forge-webhook
kubectl logs -n forge-system -l app=forge-webhook
```text

Verify webhook service:

```bash
kubectl get svc -n forge-system forge-webhook
```text

### Webhook validation takes too long

**Symptoms:**
Slow ZarfPackageJob creation (>5 seconds)

**Possible causes:**

1. Webhook pod resource constrained
2. Too many concurrent validations
3. ServiceAccount lookups slow

**Solution:**
Check webhook metrics and logs for performance issues.

---

## Controller Issues

### Controller crash loop

**Check logs:**

```bash
kubectl logs -n forge-system -l app=forge-controller --tail=100
```text

**Common causes:**

1. **RBAC permissions missing**

   ```text
   is forbidden: User "system:serviceaccount:forge-system:forge-controller" cannot get resource "zarfpackagejobs"
   ```

   **Solution:** Ensure RBAC is properly configured:

   ```bash
   kubectl apply -f config/rbac/rbac.yaml
   ```

2. **CRD not installed**

   ```text
   no matches for kind "ZarfPackageJob" in version "forge.dev/v1alpha1"
   ```

   **Solution:** Install CRDs:

   ```bash
   kubectl apply -f config/crd/
   ```

### Controller not reconciling

**Symptoms:**
ZarfPackageJob created but no Jobs appear

**Debug:**

```bash
# Check controller logs
kubectl logs -n forge-system -l app=forge-controller -f

# Check if controller is receiving events
kubectl get events -n forge-system
```text

**Solution:**

1. Verify controller is running: `kubectl get pods -n forge-system`
2. Check for errors in logs
3. Verify RBAC permissions for Job creation

### High memory usage

**Symptoms:**
Controller pod OOMKilled or high memory consumption

**Possible causes:**

1. Too many ZarfPackageJobs being monitored
2. Memory leak (report bug if persists)
3. Large job watch cache

**Solution:**

1. Increase memory limits in deployment
2. Check for completed ZarfPackageJobs that can be cleaned up
3. Review metrics for abnormal patterns

---

## Debugging Commands

### Check overall system health

```bash
# Controller status
kubectl get pods -n forge-system
kubectl get deployment -n forge-system forge-controller

# Webhook status
kubectl get pods -n forge-system -l app=forge-webhook
kubectl get validatingwebhookconfiguration forge-webhook

# CRDs installed
kubectl get crd | grep forge.dev
```text

### Inspect ZarfPackageJob

```bash
# Get package details
kubectl get ZarfPackageJob my-package -o yaml

# Get package status
kubectl get ZarfPackageJob my-package -o jsonpath='{.status}' | jq

# Get related events
kubectl get events --field-selector involvedObject.name=my-package

# Get related job
JOB=$(kubectl get ZarfPackageJob my-package -o jsonpath='{.status.buildStatus.jobName}' 2>/dev/null || echo "none")
kubectl get job $JOB -o yaml
```text

### Check ServiceAccount permissions

```bash
# View ServiceAccount
kubectl get sa dev-sa -o yaml

# Check annotations
kubectl get sa dev-sa -o jsonpath='{.metadata.annotations}' | jq
```text

### View controller logs

```bash
# Recent logs
kubectl logs -n forge-system -l app=forge-controller --tail=100

# Follow logs
kubectl logs -n forge-system -l app=forge-controller -f

# Logs for specific package reconciliation
kubectl logs -n forge-system -l app=forge-controller | grep my-package
```text

### Check metrics

```bash
# Port forward to metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller 8080:8080

# Query metrics
curl http://localhost:8080/metrics | grep forge_

# Specific metrics
curl -s http://localhost:8080/metrics | grep forge_zarf_packages_created
curl -s http://localhost:8080/metrics | grep forge_jobs_failed
```text

### Validate webhook configuration

```bash
# Check webhook config
kubectl get validatingwebhookconfiguration forge-webhook -o yaml

# Test webhook (will fail but shows if webhook is reachable)
kubectl create -f - <<EOF
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: test-webhook
spec:
  serviceAccountName: nonexistent
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/test/repo
      ref: main
EOF
# Should see webhook validation error
kubectl delete ZarfPackageJob test-webhook --ignore-not-found
```text

---

## Getting Help

If the issue persists after trying these solutions:

1. **Collect debug information:**

   ```bash
   # Controller logs
   kubectl logs -n forge-system -l app=forge-controller --tail=500 > controller.log

   # Webhook logs
   kubectl logs -n forge-system -l app=forge-webhook --tail=500 > webhook.log

   # ZarfPackageJob details
   kubectl get ZarfPackageJob my-package -o yaml > package.yaml

   # Events
   kubectl get events -A > events.log
   ```

2. **Check metrics for anomalies**

3. **Review recent changes** to ServiceAccounts, policies, or configurations

4. **File an issue** with:
   - Description of the problem
   - Steps to reproduce
   - Logs and debug information
   - Forge version (`kubectl get deployment -n forge-system forge-controller -o jsonpath='{.spec.template.spec.containers[0].image}'`)
