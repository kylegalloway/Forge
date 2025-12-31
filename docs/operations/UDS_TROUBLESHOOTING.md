# UDS Bundle Troubleshooting Guide

Comprehensive troubleshooting guide for UDS bundle operations in Forge. This guide focuses on UDS-specific issues with bundles, packages, and deployments.

> **For Zarf Package Issues**: See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for Zarf package troubleshooting.

## Table of Contents

- [UDSBundleJob Issues](#udsbundlejob-issues)
- [Bundle Creation Failures](#bundle-creation-failures)
- [Bundle Deployment Failures](#bundle-deployment-failures)
- [Policy Validation Failures](#policy-validation-failures)
- [OCI Registry Issues](#oci-registry-issues)
- [S3 Storage Issues](#s3-storage-issues)
- [Git Source Issues](#git-source-issues)
- [Controller Issues](#controller-issues)
- [Common Error Messages](#common-error-messages)
- [Debugging Commands](#debugging-commands)

---

## UDSBundleJob Issues

### UDSBundleJob stays in Pending phase

**Symptoms:**

```bash
kubectl get udsbundlejob my-bundle
# NAME        PHASE     AGE
# my-bundle   Pending   5m
```

**Possible causes:**

#### 1. ServiceAccount not found

```bash
kubectl describe udsbundlejob my-bundle
# Events:
#   Type     Reason          Message
#   ----     ------          -------
#   Warning  ServiceAccount  ServiceAccount "uds-operator" not found
```

**Solution:** Create the ServiceAccount or fix the name in `spec.serviceAccountName`:

```bash
kubectl apply -f examples/policies/uds/permissive-serviceaccount.yaml
```

#### 2. Policy validation failure (pre-execution)

```bash
kubectl get events --field-selector involvedObject.name=my-bundle
# Type     Reason           Message
# Warning  PolicyViolation  action "create" not allowed by ServiceAccount annotations
```

**Solution:** Update ServiceAccount annotations. See [Policy Validation Failures](#policy-validation-failures).

#### 3. Controller not processing UDS resources

```bash
kubectl get pods -n forge-system -l app.kubernetes.io/component=controller
# NAME                               READY   STATUS    RESTARTS   AGE
# forge-controller-<hash>            0/1     Error     3          5m
```

**Solution:** Check controller logs:

```bash
kubectl logs -n forge-system -l app.kubernetes.io/component=controller --tail=50
```

Look for errors related to:
- CRD registration failures
- RBAC permission errors
- Webhook communication failures

### UDSBundleJob rejected by webhook

**Symptoms:**

```text
Error from server: admission webhook "validate.udsbundlejob.forge.dev" denied the request:
action "create" not allowed by ServiceAccount annotations
```

**Common causes:**

1. **Action not allowed**: `action` not in `forge.forge.dev/allowed-actions`
2. **Repository not allowed**: Git URL doesn't match `forge.forge.dev/allowed-git-repos` pattern
3. **Registry not allowed**: OCI registry doesn't match `forge.forge.dev/allowed-oci-registries` pattern
4. **Namespace not allowed**: Deploy namespace doesn't match `forge.forge.dev/allowed-deploy-namespaces`

**Solution:** Check ServiceAccount annotations:

```bash
kubectl get sa -n forge-system uds-bundle-operator -o yaml | grep -A 10 annotations
```

Verify the annotations include the required permissions. See [Policy Validation Failures](#policy-validation-failures).

### UDSBundleJob (v1alpha1) no longer supported

**Symptoms:**

```text
Error: no matches for kind "UDSBundleJob" in version "forge.dev/v1alpha1"
```

**Solution:** The v1alpha1 API has been completely removed. Migrate to v1alpha2 API. See [API Version Migration Issues](#api-version-migration-issues) and [V1ALPHA2_MIGRATION.md](V1ALPHA2_MIGRATION.md).

---

## Bundle Creation Failures

### Package not found error

**Symptoms:**

```bash
kubectl logs job/my-bundle-create
# Error: package "myapp" not found in bundle definition
# Failed to pull package from oci://ghcr.io/myorg/packages/myapp:1.0.0
```

**Possible causes:**

#### 1. Package OCI reference incorrect

**Check bundle definition**:

```bash
# If using Git source
kubectl get udsbundlejob my-bundle -o jsonpath='{.spec.source.git}'

# Clone and check uds-bundle.yaml
git clone <repo-url>
cat uds-bundle.yaml
```

Look for incorrect OCI references in package list:

```yaml
packages:
  - name: myapp
    repository: ghcr.io/myorg/packages/myapp  # Missing tag
    ref: 1.0.0  # Should be part of repository or use 'tag' field
```

**Solution:** Fix the package reference in `uds-bundle.yaml`:

```yaml
packages:
  - name: myapp
    repository: ghcr.io/myorg/packages/myapp
    ref: 1.0.0
```

#### 2. Missing OCI credentials

**Symptoms:**

```text
Error: failed to pull package: unauthorized
```

**Solution:** Create OCI credentials Secret:

```bash
kubectl create secret docker-registry oci-creds \
  --docker-server=ghcr.io \
  --docker-username=myuser \
  --docker-password=ghp_token123 \  # pragma: allowlist secret
  --namespace default
```

Reference in UDSBundleJob:

```yaml
spec:
  source:
    type: OCI
    oci:
      credentialsSecretRef:
        name: oci-creds
```

#### 3. Package doesn't exist in registry

**Verify package exists**:

```bash
docker pull ghcr.io/myorg/packages/myapp:1.0.0
```

If pull fails, check:
- Package name spelling
- Tag/version exists
- Registry permissions

### Bundle validation error

**Symptoms:**

```bash
kubectl logs job/my-bundle-create
# Error: bundle validation failed: package "metrics-server" has invalid configuration
# Field "namespace" is required but not set
```

**Possible causes:**

#### 1. Invalid uds-bundle.yaml syntax

**Solution:** Validate bundle YAML locally:

```bash
# If you have uds CLI installed
uds create --confirm --dry-run

# Or validate with yamllint
yamllint uds-bundle.yaml
```

#### 2. Missing required package fields

**Common missing fields**:

- `name` - Package identifier
- `repository` or `path` - Package location
- `ref` or `tag` - Package version

**Solution:** Add missing fields to bundle definition:

```yaml
packages:
  - name: monitoring
    repository: ghcr.io/myorg/packages/monitoring
    ref: 1.0.0
    # Add any required overrides
    overrides:
      namespace: monitoring
```

### Bundle creation timeout

**Symptoms:**

```bash
kubectl get udsbundlejob my-bundle
# NAME        PHASE    AGE
# my-bundle   Failed   60m

kubectl describe job my-bundle-create
# Pods Statuses:  0 Running / 0 Succeeded / 1 Failed
# Pod:
#   Reason:  DeadlineExceeded
#   Message: Job was active longer than specified deadline
```

**Possible causes:**

#### 1. Default timeout too short for large bundles

**Solution:** Increase timeout in UDSBundleJob spec:

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: large-bundle
spec:
  action: Create
  create:
    timeout: "4h"  # Increase from default 2h
  source:
    type: Git
    git:
      url: https://github.com/myorg/large-bundle
```

#### 2. Slow network pulling packages

**Check Job logs** for download progress:

```bash
kubectl logs job/my-bundle-create --follow
# Look for package download messages
```

**Solution:**
- Increase timeout
- Use faster network
- Pre-cache packages in cluster's local registry

---

## Bundle Deployment Failures

### Component deployment failed

**Symptoms:**

```bash
kubectl logs job/my-bundle-deploy
# Error: failed to deploy component "istio-admin-gateway"
# Error: timed out waiting for the condition
```

**Possible causes:**

#### 1. Insufficient cluster resources

**Check node resources**:

```bash
kubectl top nodes
kubectl describe nodes | grep -A 5 "Allocated resources"
```

**Solution:**
- Scale up cluster
- Reduce bundle resource requirements
- Use node affinity to schedule on appropriate nodes

#### 2. Component dependencies not ready

**UDS bundles have package order dependencies**. If a component fails, subsequent components may fail.

**Check bundle package order**:

```bash
# View bundle definition
kubectl get udsbundlejob my-bundle -o jsonpath='{.spec.source.git.url}'
# Clone and check package order in uds-bundle.yaml
```

**Solution:** Ensure dependencies are deployed in correct order. UDS automatically handles this, but errors in earlier packages will block later ones.

#### 3. Namespace doesn't exist

**Symptoms:**

```text
Error: namespace "istio-system" not found
```

**Solution:** Create namespace before deployment:

```bash
kubectl create namespace istio-system
```

Or use bundle overrides to create namespaces automatically.

### Resource already exists error

**Symptoms:**

```text
Error: resource "istio-ingressgateway" already exists in namespace "istio-system"
```

**Possible causes:**

#### 1. Previous deployment not cleaned up

**Solution:** Delete existing resources:

```bash
kubectl delete deployment istio-ingressgateway -n istio-system
# Or delete entire namespace
kubectl delete namespace istio-system
```

#### 2. Conflicting Helm release

**Check for existing Helm releases**:

```bash
helm list -A | grep istio
```

**Solution:** Uninstall conflicting Helm release:

```bash
helm uninstall istio -n istio-system
```

### Deployment timeout

**Symptoms:**

```bash
kubectl logs job/my-bundle-deploy
# Error: timed out waiting for deployment "istio-ingressgateway" to be ready
```

**Possible causes:**

#### 1. Image pull failures

**Check pod events**:

```bash
kubectl get pods -n istio-system
kubectl describe pod <pod-name> -n istio-system | grep -A 10 Events
```

Look for `ImagePullBackOff` or `ErrImagePull`.

**Solution:**
- Verify image exists and is accessible
- Add image pull secrets if private registry
- Check network policies allow image pulls

#### 2. Default timeout too short

**Solution:** Increase deployment timeout:

```yaml
spec:
  action: Deploy
  deploy:
    timeout: "45m"  # Increase from default 30m
```

---

## Policy Validation Failures

### Action not allowed

**Symptoms:**

```text
Error from server: admission webhook denied the request:
action "create" not allowed by ServiceAccount annotations
```

**Diagnosis:**

```bash
# Check ServiceAccount annotations
kubectl get sa uds-bundle-operator -n forge-system -o yaml

# Look for allowed-actions annotation
annotations:
  forge.forge.dev/allowed-actions: "publish,deploy"  # ❌ "create" missing
```

**Solution:** Add missing action to ServiceAccount:

```bash
kubectl annotate sa uds-bundle-operator \
  -n forge-system \
  forge.forge.dev/allowed-actions="create,publish,deploy" \
  --overwrite
```

### Git repository not allowed

**Symptoms:**

```text
Error: Git repo "https://github.com/external/bundle" is not allowed
(allowed repos: [github.com/myorg/*])
```

**Diagnosis:**

```bash
kubectl get sa uds-bundle-operator -n forge-system -o jsonpath='{.metadata.annotations.forge\.forge\.dev/allowed-git-repos}'
# Output: github.com/myorg/*
```

**Solution:** Either:

1. **Use allowed repository**: Change source to an allowed repo
2. **Update policy**: Add repository pattern to allowed list

```bash
kubectl annotate sa uds-bundle-operator \
  -n forge-system \
  forge.forge.dev/allowed-git-repos="github.com/myorg/*,github.com/external/*" \
  --overwrite
```

### OCI registry not allowed

**Symptoms:**

```text
Error: OCI registry "docker.io" is not allowed
(allowed registries: [ghcr.io/*])
```

**Solution:** Add registry to allowed list:

```bash
kubectl annotate sa uds-bundle-cicd \
  -n forge-system \
  forge.forge.dev/allowed-oci-registries="ghcr.io/*,docker.io/*" \
  --overwrite
```

### Deploy namespace not allowed

**Symptoms:**

```text
Error: deployment to namespace "production" is not allowed
(allowed namespaces: [dev,staging])
```

**Solution:** Either:

1. **Deploy to allowed namespace**: Change target namespace
2. **Update policy**: Add namespace to allowed list

```bash
kubectl annotate sa uds-bundle-operator \
  -n forge-system \
  forge.forge.dev/allowed-deploy-namespaces="dev,staging,production" \
  --overwrite
```

---

## OCI Registry Issues

### Unauthorized error

**Symptoms:**

```text
Error: failed to push bundle: unauthorized: authentication required
```

**Diagnosis:**

1. **Check Secret exists**:

```bash
kubectl get secret oci-registry-creds
```

2. **Verify Secret format**:

```bash
kubectl get secret oci-registry-creds -o jsonpath='{.type}'
# Should be: kubernetes.io/dockerconfigjson
```

3. **Check Secret data**:

```bash
kubectl get secret oci-registry-creds -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d | jq
```

Expected format:

```json
{
  "auths": {
    "ghcr.io": {
      "username": "myuser",
      "password": "ghp_token",  // pragma: allowlist secret
      "auth": "base64(username:password)"
    }
  }
}
```

**Solution:** Create properly formatted Secret:

```bash
kubectl create secret docker-registry oci-registry-creds \
  --docker-server=ghcr.io \
  --docker-username=myuser \
  --docker-password=ghp_token123 \  # pragma: allowlist secret
  --dry-run=client -o yaml | kubectl apply -f -
```

### Image manifest not found

**Symptoms:**

```text
Error: failed to pull bundle: manifest unknown: manifest unknown
```

**Possible causes:**

1. **Tag doesn't exist**: Verify tag exists in registry
2. **Wrong registry**: Check registry URL is correct
3. **Permissions**: User doesn't have read access

**Solution:**

```bash
# Verify bundle exists
docker pull ghcr.io/myorg/bundles/platform:v1.0.0

# If using GitHub Container Registry, verify package visibility
# Go to: https://github.com/orgs/YOURORG/packages
```

### Rate limit exceeded

**Symptoms:**

```text
Error: toomanyrequests: You have reached your pull rate limit
```

**Solution:**

1. **Authenticate to increase limits**:

```bash
kubectl create secret docker-registry docker-hub-creds \
  --docker-server=docker.io \
  --docker-username=myuser \
  --docker-password=mypassword  # pragma: allowlist secret
```

2. **Use alternative registry** (GHCR has higher limits):

```yaml
spec:
  source:
    type: OCI
    oci:
      ref: ghcr.io/myorg/bundle:v1.0.0  # Instead of docker.io
```

---

## S3 Storage Issues

### Access denied error

**Symptoms:**

```text
Error: failed to upload to S3: AccessDenied: Access Denied
```

**Diagnosis:**

1. **Check Secret exists**:

```bash
kubectl get secret aws-s3-creds
```

2. **Verify Secret data**:

```bash
kubectl get secret aws-s3-creds -o jsonpath='{.data}' | jq
# Should have: aws_access_key_id, aws_secret_access_key
```

3. **Decode and verify credentials**:

```bash
kubectl get secret aws-s3-creds -o jsonpath='{.data.aws_access_key_id}' | base64 -d
kubectl get secret aws-s3-creds -o jsonpath='{.data.aws_secret_access_key}' | base64 -d
```

**Solution:** Create Secret with correct credentials:

```bash
kubectl create secret generic aws-s3-creds \
  --from-literal=aws_access_key_id=AKIAIOSFODNN7EXAMPLE \  # pragma: allowlist secret
  --from-literal=aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY  # pragma: allowlist secret
```

### Bucket not found

**Symptoms:**

```text
Error: NoSuchBucket: The specified bucket does not exist
```

**Solution:**

1. **Verify bucket name**:

```bash
aws s3 ls s3://my-uds-bundles --region us-east-1
```

2. **Create bucket if needed**:

```bash
aws s3 mb s3://my-uds-bundles --region us-east-1
```

3. **Update UDSBundleJob with correct bucket name**:

```yaml
spec:
  publish:
    destination:
      type: S3
      s3:
        bucket: my-uds-bundles  # Verify this matches actual bucket name
        region: us-east-1
```

### Invalid region error

**Symptoms:**

```text
Error: InvalidRequest: Invalid region specified
```

**Solution:** Verify bucket region:

```bash
aws s3api get-bucket-location --bucket my-uds-bundles
# Output: {"LocationConstraint": "us-west-2"}
```

Update UDSBundleJob with correct region:

```yaml
spec:
  publish:
    destination:
      type: S3
      s3:
        bucket: my-uds-bundles
        region: us-west-2  # Match bucket region
```

---

## Git Source Issues

### Repository not found

**Symptoms:**

```text
Error: failed to clone repository: repository not found
```

**Possible causes:**

1. **Private repository without credentials**
2. **Repository URL typo**
3. **Repository deleted or moved**

**Solution:**

For private repositories, create Git credentials Secret:

```bash
# For SSH key
kubectl create secret generic git-ssh-key \
  --from-file=id_rsa=/path/to/private/key \
  --from-file=known_hosts=/path/to/known_hosts
```

Reference in UDSBundleJob:

```yaml
spec:
  source:
    type: Git
    git:
      url: git@github.com:myorg/private-bundle.git
      credentialsSecretRef:
        name: git-ssh-key
```

### Authentication failed

**Symptoms:**

```text
Error: authentication failed: invalid credentials
```

**For HTTPS with token**:

```bash
kubectl create secret generic git-token \
  --from-literal=username=git \
  --from-literal=password=ghp_token123  # pragma: allowlist secret
```

**For SSH**:

Verify SSH key has access to repository:

```bash
ssh -T git@github.com -i /path/to/key
# Hi username! You've successfully authenticated...
```

### Branch/tag not found

**Symptoms:**

```text
Error: reference not found: ref 'v1.0.0' not found
```

**Solution:**

1. **Verify ref exists**:

```bash
git ls-remote https://github.com/myorg/bundle
# Look for refs/heads/<branch> or refs/tags/<tag>
```

2. **Update UDSBundleJob with correct ref**:

```yaml
spec:
  source:
    type: Git
    git:
      url: https://github.com/myorg/bundle
      ref: main  # Or correct branch/tag name
```

---

## Controller Issues

### Controller not processing UDS jobs

**Symptoms:**

- UDSBundleJobs stuck in Pending
- No Jobs created for UDSBundleJobs
- Events show no activity

**Diagnosis:**

1. **Check controller is running**:

```bash
kubectl get pods -n forge-system -l app.kubernetes.io/component=controller
```

2. **Check controller logs**:

```bash
kubectl logs -n forge-system -l app.kubernetes.io/component=controller --tail=100
```

**Common errors:**

- `failed to watch UDSBundleJob: forbidden` - RBAC issue
- `failed to connect to webhook` - Webhook communication issue
- `panic: runtime error` - Controller bug (report to developers)

**Solution:**

Restart controller:

```bash
kubectl rollout restart deployment forge-controller -n forge-system
```

### RBAC permission errors

**Symptoms:**

```bash
kubectl logs -n forge-system -l app.kubernetes.io/component=controller
# Error: failed to create job: forbidden: User "system:serviceaccount:forge-system:forge-controller"
# cannot create resource "jobs" in API group "batch" in namespace "default"
```

**Solution:**

Verify controller ServiceAccount has correct RBAC:

```bash
kubectl get clusterrolebinding | grep forge
kubectl describe clusterrolebinding forge-controller

# For namespace-scoped deployments
kubectl get rolebinding -n forge-system | grep forge
```

If missing, reinstall with Helm:

```bash
helm upgrade forge forge/forge \
  --namespace forge-system \
  --reuse-values
```

---

## Common Error Messages

### Error: uds-bundle.yaml not found

**Full message:**

```text
Error: failed to read bundle definition: open uds-bundle.yaml: no such file or directory
```

**Cause:** Git repository doesn't contain `uds-bundle.yaml` in the specified path.

**Solution:**

1. **Verify bundle file exists in repository**:

```bash
git clone <repo-url>
cd <repo>
ls -la uds-bundle.yaml
```

2. **Specify correct path if in subdirectory**:

```yaml
spec:
  source:
    type: Git
    git:
      url: https://github.com/myorg/bundle
      ref: main
      path: bundles/platform  # If uds-bundle.yaml is in bundles/platform/
```

### Error: package checksum mismatch

**Full message:**

```text
Error: package checksum mismatch for "istio"
Expected: sha256:abc123...
Got:      sha256:def456...
```

**Cause:** Package integrity verification failed.

**Possible reasons:**
- Package was modified after bundle creation
- Corrupted download
- Man-in-the-middle attack

**Solution:**

1. **Re-download package**:

```bash
# If using OCI
docker pull ghcr.io/myorg/packages/myapp:1.0.0

# Verify checksum matches bundle definition
```

2. **Update bundle with new checksum** (if package was legitimately updated):

Edit `uds-bundle.yaml` and update package checksum.

### Error: invalid override configuration

**Full message:**

```text
Error: invalid override for package "keycloak": field "replicas" does not exist
```

**Cause:** Override specifies a field that doesn't exist in the package.

**Solution:**

Check package chart values:

```bash
# If Helm-based package
helm show values <chart-name>

# Verify field name is correct
```

Update override with correct field name:

```yaml
packages:
  - name: keycloak
    overrides:
      replicaCount: 2  # Correct field name (not "replicas")
```

---

## Debugging Commands

### Inspect UDSBundleJob Status

```bash
# Get basic status
kubectl get udsbundlejob <name>

# Get full details
kubectl get udsbundlejob <name> -o yaml

# Watch status changes
kubectl get udsbundlejob <name> -w

# Get events
kubectl get events --field-selector involvedObject.name=<name>
```

### Check Job Status

```bash
# List all UDS jobs
kubectl get jobs -l app=forge,resource-type=udsbundlejob

# Find jobs for specific bundle
kubectl get jobs -l forge.forge.dev/package=<bundle-name>

# Get job logs
kubectl logs job/<bundle-name>-create
kubectl logs job/<bundle-name>-deploy --follow

# Check pod status
kubectl get pods -l job-name=<bundle-name>-create
kubectl describe pod <pod-name>
```

### Check ServiceAccount Policy

```bash
# Get ServiceAccount with annotations
kubectl get sa <sa-name> -n forge-system -o yaml

# Check specific annotation
kubectl get sa <sa-name> -n forge-system \
  -o jsonpath='{.metadata.annotations.forge\.forge\.dev/allowed-actions}'

# List all UDS-related ServiceAccounts
kubectl get sa -n forge-system -l app.kubernetes.io/component=uds
```

### Check Controller Health

```bash
# Check controller pod status
kubectl get pods -n forge-system -l app.kubernetes.io/component=controller

# Get controller logs
kubectl logs -n forge-system -l app.kubernetes.io/component=controller --tail=100 --follow

# Check controller resource usage
kubectl top pods -n forge-system -l app.kubernetes.io/component=controller

# Check controller events
kubectl get events -n forge-system --field-selector involvedObject.name=<controller-pod>
```

### Check Webhook Health

```bash
# Check webhook pod status
kubectl get pods -n forge-system -l app.kubernetes.io/component=webhook

# Get webhook logs
kubectl logs -n forge-system -l app.kubernetes.io/component=webhook --tail=50

# Test webhook is responding
kubectl get validatingwebhookconfigurations | grep forge
kubectl describe validatingwebhookconfigurations forge-webhook
```

### Debug Network Issues

```bash
# Check if job can reach OCI registry
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -I https://ghcr.io/v2/

# Check if job can reach S3
kubectl run -it --rm debug --image=amazon/aws-cli --restart=Never -- \
  s3 ls s3://my-bucket

# Check DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup ghcr.io
```

### Collect Diagnostic Information

```bash
#!/bin/bash
# Save this as collect-uds-diagnostics.sh

BUNDLE_NAME=$1
OUTPUT_DIR="diagnostics-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTPUT_DIR"

echo "Collecting diagnostics for bundle: $BUNDLE_NAME"

# UDSBundleJob details
kubectl get udsbundlejob "$BUNDLE_NAME" -o yaml > "$OUTPUT_DIR/udsbundlejob.yaml"
kubectl describe udsbundlejob "$BUNDLE_NAME" > "$OUTPUT_DIR/udsbundlejob-describe.txt"

# Job details
kubectl get jobs -l forge.forge.dev/package="$BUNDLE_NAME" -o yaml > "$OUTPUT_DIR/jobs.yaml"

# Pod logs
for pod in $(kubectl get pods -l forge.forge.dev/package="$BUNDLE_NAME" -o name); do
  kubectl logs "$pod" > "$OUTPUT_DIR/${pod##*/}.log" 2>&1
done

# Events
kubectl get events --field-selector involvedObject.name="$BUNDLE_NAME" > "$OUTPUT_DIR/events.txt"

# Controller logs
kubectl logs -n forge-system -l app.kubernetes.io/component=controller --tail=200 > "$OUTPUT_DIR/controller.log"

# ServiceAccount details
SA_NAME=$(kubectl get udsbundlejob "$BUNDLE_NAME" -o jsonpath='{.spec.serviceAccountName}')
kubectl get sa "$SA_NAME" -o yaml > "$OUTPUT_DIR/serviceaccount.yaml"

echo "Diagnostics saved to: $OUTPUT_DIR"
tar -czf "$OUTPUT_DIR.tar.gz" "$OUTPUT_DIR"
echo "Archive created: $OUTPUT_DIR.tar.gz"
```

Usage:

```bash
chmod +x collect-uds-diagnostics.sh
./collect-uds-diagnostics.sh my-bundle
```

---

## Additional Resources

- **User Guide**: [UDS_GUIDE.md](../getting-started/UDS_GUIDE.md) - Complete UDS bundle guide
- **Policy Examples**: `examples/policies/uds/` - ServiceAccount templates and RBAC
- **General Troubleshooting**: [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Zarf package issues
- **Runbook**: [RUNBOOK.md](RUNBOOK.md) - Operational procedures

**Getting Help**:

1. Check this troubleshooting guide
2. Review [UDS_GUIDE.md](../getting-started/UDS_GUIDE.md) for usage examples
3. Check GitHub Issues: https://github.com/kylegalloway/forge/issues
