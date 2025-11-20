# Forge Troubleshooting Guide

Common issues and their solutions when running Forge.

## Table of Contents

- [ZarfPackage Issues](#zarfpackage-issues)
- [Policy & Permission Issues](#policy--permission-issues)
- [Job Failures](#job-failures)
- [Webhook Issues](#webhook-issues)
- [Controller Issues](#controller-issues)
- [Debugging Commands](#debugging-commands)

---

## ZarfPackage Issues

### ZarfPackage stays in Pending phase

**Symptoms:**
```bash
kubectl get zarfpackage my-package
# STATUS shows "Pending" for extended period
```

**Possible causes:**

1. **ServiceAccount not found**
   ```bash
   kubectl describe zarfpackage my-package
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

### ZarfPackage was denied by webhook

**Symptoms:**
```
Error from server: admission webhook "validate.zarf.dev" denied the request
```

**Solution:**
Check the webhook validation error message for specific policy violation. Common issues:
- Action not in allowed-actions annotation
- Source repository not in allowed-source-repos
- Missing required ServiceAccount annotations

---

## Policy & Permission Issues

### Action not allowed error

**Symptoms:**
```
action Deploy is not allowed (allowed actions: [Build,Publish]) for ServiceAccount dev-sa
```

**Solution:**
Update ServiceAccount annotations to include the action:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dev-sa
  annotations:
    forge.zarf.dev/allowed-actions: "Build,Publish,Deploy"  # Add Deploy
```

### Git repository not allowed

**Symptoms:**
```
Git repo https://github.com/myorg/repo is not allowed (allowed repos: [github.com/other/*])
```

**Solution:**
Update ServiceAccount to allow the repository pattern:
```yaml
annotations:
  forge.zarf.dev/allowed-source-repos: "https://github.com/myorg/*,https://github.com/other/*"
```

**Note:** Patterns use glob matching. Use `*` for wildcard, e.g., `https://github.com/myorg/*` matches all repos under myorg.

### S3 bucket not allowed

**Symptoms:**
```
S3 bucket my-prod-bucket is not allowed (allowed buckets: [my-dev-*])
```

**Solution:**
```yaml
annotations:
  forge.zarf.dev/allowed-source-buckets: "my-dev-*,my-prod-*"
  # or for publish:
  forge.zarf.dev/allowed-publish-buckets: "my-artifacts-*"
```

### Local sources denied

**Symptoms:**
```
local sources are not allowed (set annotation forge.zarf.dev/allow-local-sources: true for dev mode)
```

**Solution (DEV/TEST ONLY):**
```yaml
annotations:
  forge.zarf.dev/allow-local-sources: "true"
```

**Warning:** Only enable for development/testing. Never in production.

---

## Job Failures

### Job pod fails immediately

**Check job logs:**
```bash
# Find the job
kubectl get jobs -l forge.zarf.dev/package=my-package

# Get logs
kubectl logs job/my-package-build-xxxxx
```

**Common causes:**

1. **Missing credentials**
   ```
   fatal: could not read Username for 'https://github.com': No such device
   ```
   **Solution:** Create Secret with Git credentials and reference in ZarfPackage:
   ```yaml
   source:
     git:
       credentialsSecretRef:
         name: github-token
   ```

2. **Invalid S3 credentials**
   ```
   InvalidAccessKeyId: The AWS Access Key Id you provided does not exist
   ```
   **Solution:** Verify Secret contains correct access-key-id and secret-access-key

3. **OCI auth failure**
   ```
   unauthorized: authentication required
   ```
   **Solution:** Create kubernetes.io/dockerconfigjson Secret and reference it

### Job exceeds active deadline

**Symptoms:**
```
Job was active longer than specified deadline
```

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
```
OOMKilled
```

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
```
x509: certificate signed by unknown authority
```

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
```
context deadline exceeded
```

**Solution:**
Check webhook pod status:
```bash
kubectl get pods -n forge-system -l app=forge-webhook
kubectl logs -n forge-system -l app=forge-webhook
```

Verify webhook service:
```bash
kubectl get svc -n forge-system forge-webhook
```

### Webhook validation takes too long

**Symptoms:**
Slow ZarfPackage creation (>5 seconds)

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
```

**Common causes:**

1. **RBAC permissions missing**
   ```
   is forbidden: User "system:serviceaccount:forge-system:forge-controller" cannot get resource "zarfpackages"
   ```
   **Solution:** Ensure RBAC is properly configured:
   ```bash
   kubectl apply -f config/rbac/rbac.yaml
   ```

2. **CRD not installed**
   ```
   no matches for kind "ZarfPackage" in version "zarf.dev/v1alpha1"
   ```
   **Solution:** Install CRDs:
   ```bash
   kubectl apply -f config/crd/
   ```

### Controller not reconciling

**Symptoms:**
ZarfPackage created but no Jobs appear

**Debug:**
```bash
# Check controller logs
kubectl logs -n forge-system -l app=forge-controller -f

# Check if controller is receiving events
kubectl get events -n forge-system
```

**Solution:**
1. Verify controller is running: `kubectl get pods -n forge-system`
2. Check for errors in logs
3. Verify RBAC permissions for Job creation

### High memory usage

**Symptoms:**
Controller pod OOMKilled or high memory consumption

**Possible causes:**
1. Too many ZarfPackages being monitored
2. Memory leak (report bug if persists)
3. Large job watch cache

**Solution:**
1. Increase memory limits in deployment
2. Check for completed ZarfPackages that can be cleaned up
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
kubectl get crd | grep zarf.dev
```

### Inspect ZarfPackage

```bash
# Get package details
kubectl get zarfpackage my-package -o yaml

# Get package status
kubectl get zarfpackage my-package -o jsonpath='{.status}' | jq

# Get related events
kubectl get events --field-selector involvedObject.name=my-package

# Get related job
JOB=$(kubectl get zarfpackage my-package -o jsonpath='{.status.buildStatus.jobName}' 2>/dev/null || echo "none")
kubectl get job $JOB -o yaml
```

### Check ServiceAccount permissions

```bash
# View ServiceAccount
kubectl get sa dev-sa -o yaml

# Check annotations
kubectl get sa dev-sa -o jsonpath='{.metadata.annotations}' | jq
```

### View controller logs

```bash
# Recent logs
kubectl logs -n forge-system -l app=forge-controller --tail=100

# Follow logs
kubectl logs -n forge-system -l app=forge-controller -f

# Logs for specific package reconciliation
kubectl logs -n forge-system -l app=forge-controller | grep my-package
```

### Check metrics

```bash
# Port forward to metrics endpoint
kubectl port-forward -n forge-system svc/forge-controller 8080:8080

# Query metrics
curl http://localhost:8080/metrics | grep forge_

# Specific metrics
curl -s http://localhost:8080/metrics | grep forge_zarf_packages_created
curl -s http://localhost:8080/metrics | grep forge_jobs_failed
```

### Validate webhook configuration

```bash
# Check webhook config
kubectl get validatingwebhookconfiguration forge-webhook -o yaml

# Test webhook (will fail but shows if webhook is reachable)
kubectl create -f - <<EOF
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackage
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
kubectl delete zarfpackage test-webhook --ignore-not-found
```

---

## Getting Help

If the issue persists after trying these solutions:

1. **Collect debug information:**
   ```bash
   # Controller logs
   kubectl logs -n forge-system -l app=forge-controller --tail=500 > controller.log

   # Webhook logs
   kubectl logs -n forge-system -l app=forge-webhook --tail=500 > webhook.log

   # ZarfPackage details
   kubectl get zarfpackage my-package -o yaml > package.yaml

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
