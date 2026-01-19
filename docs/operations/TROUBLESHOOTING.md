# Forge Troubleshooting Guide

Common issues and their solutions when running Forge.

## Table of Contents

- [Job Resource Issues](#job-resource-issues)
- [Job Status Bouncing (Multi-Action Workflows)](#job-status-bouncing-multi-action-workflows)
- [Policy & Permission Issues](#policy--permission-issues)
- [Job Failures](#job-failures)
- [Webhook Issues](#webhook-issues)
- [Controller Issues](#controller-issues)
- [Debugging Commands](#debugging-commands)

---

## Job Resource Issues

**Note:** This section applies to both ZarfPackageJob and UDSBundleJob resources. Examples show ZarfPackageJob, but the same troubleshooting steps apply to UDSBundleJob.

### Job stays in Pending phase

**Symptoms:**

```bash
kubectl get ZarfPackageJob my-package
# STATUS shows "Pending" for extended period
```

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

### Job was denied by webhook

**Symptoms:**

```text
Error from server: admission webhook "validate.forge.dev" denied the request
```

**Solution:**
Check the webhook validation error message for specific policy violation. Common issues:

- Action not in allowed-actions annotation
- Source repository not in allowed-source-repos
- Missing required ServiceAccount annotations

---

## Job Status Bouncing (Multi-Action Workflows)

### Symptom

When watching ZarfPackageJob or UDSBundleJob resources with `-w`, you observe the status phase bouncing between states like:
- Running → Completed → Running → Completed
- Pending → Running → Pending

This is especially noticeable with multi-action workflows (BuildPublish, BuildDeploy, CreatePublishDeploy, etc.).

### Root Cause: Expected Behavior

For jobs with chained actions (e.g., `BuildPublish`), the controller intentionally updates the status multiple times:

1. **Build phase starts**: Status set to "Running" with action "build"
2. **Build completes**: Status set to "Completed", build artifacts available
3. **Publish phase starts**: Status changes to "Running" with action "publish" (THIS LOOKS LIKE BOUNCING)
4. **Publish completes**: Status set to "Completed"

This is **expected behavior** and indicates that action chaining is working correctly.

### Diagnosis

**Check if Multi-Action Workflow:**

```bash
# Get the action from the resource
kubectl get zarfpackagejob <name> -o jsonpath='{.spec.action}'

# Multi-action workflows:
# - BuildPublish, BuildDeploy, PublishDeploy, BuildPublishDeploy
# - CreatePublish (UDS), CreateDeploy (UDS), CreatePublishDeploy (UDS)
```

If the action contains multiple steps, status bouncing is **expected**.

**Watch Detailed Status:**

```bash
# Watch with detailed fields
kubectl get zarfpackagejob <name> -o custom-columns=\
NAME:.metadata.name,\
ACTION:.spec.action,\
PHASE:.status.phase,\
BUILD:.status.buildStatus.state,\
PUBLISH:.status.publishStatus.state,\
DEPLOY:.status.deployStatus.state \
-w
```

**Check Job History:**

```bash
# List all Jobs for this package
kubectl get jobs -l forge.dev/package=<name>

# For multi-action workflows, you should see multiple jobs:
# <name>-build
# <name>-publish
# <name>-deploy
```

### When It's a Real Problem

Status bouncing is **NOT normal** when:

1. **Single-action workflow bounces** (e.g., `action: Build` bounces between Running/Completed)
2. **Bounces back to earlier action** (e.g., BuildPublish goes Build → Publish → Build again)
3. **Bounces without Job changes** (Jobs are not completing but status still bounces)

**Debugging Real Problems:**

```bash
# 1. Check if Jobs are actually completing
kubectl get jobs -l forge.dev/package=<name> -o wide

# 2. Check for controller errors
kubectl logs -n forge-system -l app=forge-controller --tail=100 | grep -i error

# 3. Check for reconciliation loops
kubectl logs -n forge-system -l app=forge-controller | grep "Reconciling\|reconcilePackage" | tail -20

# 4. Check resource versions (should increment)
kubectl get zarfpackagejob <name> -o jsonpath='{.metadata.resourceVersion}' -w
```

**TL;DR**: If you're using multi-action workflows (BuildPublish, CreateDeploy, etc.), seeing status bounce between "Running" and "Completed" is **normal and expected**. Each action in the chain causes the status to update.

---

## Policy & Permission Issues

### Action not allowed error

**Symptoms:**

```text
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
    forge.dev/allowed-actions: "Build,Publish,Deploy"  # Add Deploy
```

### Git repository not allowed

**Symptoms:**

```text
Git repo https://github.com/myorg/repo is not allowed (allowed repos: [github.com/other/*])
```

**Solution:**
Update ServiceAccount to allow the repository pattern:

```yaml
annotations:
  forge.dev/allowed-source-repos: "https://github.com/myorg/*,https://github.com/other/*"
```

**Note:** Patterns use glob matching. Use `*` for wildcard, e.g., `https://github.com/myorg/*` matches all repos under myorg.

### S3 bucket not allowed

**Symptoms:**

```text
S3 bucket my-prod-bucket is not allowed (allowed buckets: [my-dev-*])
```

**Solution:**

```yaml
annotations:
  forge.dev/allowed-source-buckets: "my-dev-*,my-prod-*"
  # or for publish:
  forge.dev/allowed-publish-buckets: "my-artifacts-*"
```

### Local sources denied

**Symptoms:**

```text
local sources are not allowed (set annotation forge.dev/allow-local-sources: true for dev mode)
```

**Solution (DEV/TEST ONLY):**

```yaml
annotations:
  forge.dev/allow-local-sources: "true"
```

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
```

**Common causes:**

1. **Missing credentials**

   ```text
   fatal: could not read Username for 'https://github.com': No such device
   ```

   **Solution:** Create Secret with Git credentials and reference in ZarfPackageJob:

   ```yaml
   source:
     git:
       credentialRef:
         name: github-token
   ```

2. **Invalid S3 credentials**

   ```text
   InvalidAccessKeyId: The AWS Access Key Id you provided does not exist
   ```

   **Solution:** Verify Secret contains correct `access-key-id` and `secret-access-key` keys

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
Failed to pull image "ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1":
failed to pull and unpack image: pull access denied
```

**Cause:**
The Zarf CLI image cannot be pulled. This could be due to network issues, registry authentication, or (in Kind clusters) the image not being loaded.

**Solutions:**

1. **For Kind clusters - load the image:**

   Kind clusters cannot pull images directly. Load the published image:

   ```bash
   # Using Docker
   docker pull ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1
   kind load docker-image ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1 --name <cluster-name>

   # Using Podman
   podman pull ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1
   podman save ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1 -o /tmp/zarf-cli.tar
   kind load image-archive /tmp/zarf-cli.tar --name <cluster-name>
   rm /tmp/zarf-cli.tar
   ```

2. **For air-gapped environments - use Zarf package:**

   Deploy Forge using the included Zarf package which bundles the image:

   ```bash
   zarf package deploy zarf-package-forge-*.tar.zst --confirm
   ```

3. **Build locally (if needed for custom versions):**

   Forge includes a Dockerfile to build custom versions:

   ```bash
   docker build -t ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1 images/zarf-cli/
   ```

4. **Configure imagePullSecrets (for private registries):**

   ```bash
   kubectl create secret docker-registry registry-secret \
     --docker-server=ghcr.io \
     --docker-username=<username> \
     --docker-password=<token> \
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

```text
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

```text
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

```text
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
Slow job creation (>5 seconds)

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

   ```text
   is forbidden: User "system:serviceaccount:forge-system:forge-controller" cannot get resource "zarfpackagejobs"
   ```

   **Solution:** Ensure RBAC is properly configured:

   ```bash
   kubectl apply -f config/rbac/rbac.yaml
   ```

2. **CRD not installed**

   ```text
   no matches for kind "ZarfPackageJob" in version "forge.dev/v1alpha3"
   ```

   **Solution:** Install CRDs:

   ```bash
   kubectl apply -f config/crd/
   ```

### Controller not reconciling

**Symptoms:**
Job resource created but no Kubernetes Jobs appear

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

1. Too many job resources being monitored
2. Memory leak (report bug if persists)
3. Large job watch cache

**Solution:**

1. Increase memory limits in deployment
2. Check for completed job resources that can be cleaned up
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
```

### Inspect Job Resources

```bash
# Get ZarfPackageJob details
kubectl get ZarfPackageJob my-package -o yaml
kubectl get ZarfPackageJob my-package -o jsonpath='{.status}' | jq

# Get UDSBundleJob details
kubectl get UDSBundleJob my-bundle -o yaml
kubectl get UDSBundleJob my-bundle -o jsonpath='{.status}' | jq

# Get related events
kubectl get events --field-selector involvedObject.name=my-package

# Get related Kubernetes job
JOB=$(kubectl get ZarfPackageJob my-package -o jsonpath='{.status.buildStatus.jobName}' 2>/dev/null || echo "none")
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
apiVersion: forge.dev/v1alpha3
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

   # Job details (ZarfPackageJob or UDSBundleJob)
   kubectl get ZarfPackageJob my-package -o yaml > package.yaml
   kubectl get UDSBundleJob my-bundle -o yaml > bundle.yaml

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
