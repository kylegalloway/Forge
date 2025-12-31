# UDS Bundle User Guide

## Introduction

Forge allows you to manage UDS (Unicorn Delivery Service) bundles using Kubernetes Custom Resources. UDS bundles are collections of multiple Zarf packages deployed together as a cohesive platform. This guide provides detailed instructions on how to use Forge to create, publish, and deploy UDS bundles.

> **For Single Packages**: If you're working with individual Zarf packages (not bundles), see the [Zarf USER_GUIDE.md](USER_GUIDE.md) instead.
>
> **For Developers**: If you're contributing to Forge and need to test local changes, see [KIND_SETUP.md](KIND_SETUP.md) for the developer workflow.

## When to Use UDS vs Zarf

Use **UDS bundles** (`UDSBundleJob`) when:

- Deploying multi-package platforms (e.g., UDS Core with Istio + Keycloak + Grafana)
- Managing complex deployments with interdependencies
- Creating opinionated software distributions
- Bundling multiple Zarf packages for air-gapped delivery

Use **Zarf packages** (`ZarfPackageJob`) when:

- Deploying a single application or service
- Creating standalone packages
- Building individual components
- Testing individual Zarf packages

See the main [USER_GUIDE.md](USER_GUIDE.md) for Zarf package operations.

## Installation

Follow the [Installation instructions](USER_GUIDE.md#installation) in the main User Guide. Forge supports both Zarf packages and UDS bundles out of the box.

## API Versions

Forge provides two API versions for UDS bundles:

### v1alpha2 (Recommended)

**Resource**: `UDSBundleJob`
**Status**: Active, recommended for all new deployments
**Action Field**: `action` (consistent with ZarfPackageJob)
**Actions**: `Create`, `Publish`, `Deploy`, `CreatePublish`, `CreateDeploy`, `PublishDeploy`, `CreatePublishDeploy`

### v1alpha1 (Deprecated)

**Resource**: `UDSBundleJob`
**Status**: Deprecated, will be removed in Forge v0.10.0 (~6 months)
**Action Field**: `bundleAction`
**Actions**: `BundleActionCreate`, `BundleActionPublish`, etc.

**Migration**: See [V1ALPHA2_MIGRATION.md](../operations/V1ALPHA2_MIGRATION.md) for the complete migration guide.

## Core Concepts

### UDS Bundles

A UDS bundle is a collection of multiple Zarf packages defined in a `uds-bundle.yaml` file. Bundles enable:

- **Platform Deployments**: Deploy complete platforms (e.g., DevSecOps stack with GitLab, ArgoCD, monitoring)
- **Dependency Management**: Define package order and dependencies
- **Configuration Overrides**: Set variables and configurations across packages
- **Air-Gapped Distribution**: Bundle everything needed for offline installation

### Actions

UDS bundles support three primary actions (and their combinations):

- **Create**: Builds a UDS bundle from source (equivalent to `uds create`)
- **Publish**: Uploads a bundle to a registry (OCI or S3)
- **Deploy**: Installs a bundle into a cluster (equivalent to `uds deploy`)
- **Composite Actions**: `CreatePublish`, `CreateDeploy`, `PublishDeploy`, `CreatePublishDeploy`

### Sources

UDS bundles can be sourced from:

- **Git**: Clone bundle definition from a Git repository
- **OCI**: Pull existing bundle from an OCI registry
- **S3**: Download bundle from an S3 bucket
- **Local**: Use local bundle (development only, disabled by default)

### Destinations

Published bundles can be stored in:

- **OCI**: Push to OCI registries (GHCR, Harbor, Artifactory, etc.)
- **S3**: Upload to S3-compatible storage

## Examples

### 1. Create a Bundle from Git

Creates a UDS bundle from a public Git repository containing a `uds-bundle.yaml` file.

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: create-uds-core
  namespace: default
spec:
  serviceAccountName: uds-bundle-operator
  action: Create
  source:
    type: Git
    git:
      url: https://github.com/defenseunicorns/uds-core
      ref: v0.25.0
```

Apply with:

```bash
kubectl apply -f create-uds-core.yaml
```

Expected output:

```text
udsbundlejob.forge.dev/create-uds-core created
```

Watch progress:

```bash
kubectl get udsbundlejob create-uds-core -w
```

Expected output:

```text
NAME               PHASE      AGE
create-uds-core    Pending    2s
create-uds-core    Running    10s
create-uds-core    Succeeded  3m45s
```

### 2. Create and Publish to OCI

Creates a bundle and immediately publishes it to an OCI registry.

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: build-publish-bundle
  namespace: default
spec:
  serviceAccountName: uds-bundle-cicd
  action: CreatePublish
  source:
    type: Git
    git:
      url: https://github.com/myorg/my-uds-bundle
      ref: main
  publish:
    destination:
      type: OCI
      oci:
        registry: ghcr.io
        repository: myorg/bundles/my-platform
        tag: v1.0.0
        credentialsSecretRef:
          name: oci-registry-creds
```

Apply with:

```bash
kubectl apply -f build-publish-bundle.yaml
```

Expected output:

```text
udsbundlejob.forge.dev/build-publish-bundle created
```

Check status:

```bash
kubectl get udsbundlejob build-publish-bundle -n default
```

Expected output:

```text
NAME                   PHASE      AGE
build-publish-bundle   Succeeded  5m30s
```

### 3. Deploy from OCI Registry

Deploys an existing UDS bundle from an OCI registry.

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: deploy-uds-core
  namespace: default
spec:
  serviceAccountName: uds-bundle-operator
  action: Deploy
  source:
    type: OCI
    oci:
      ref: ghcr.io/defenseunicorns/packages/uds/bundles/core:0.25.0
      credentialsSecretRef:
        name: oci-registry-creds
  deploy:
    target: InCluster
```

Apply with:

```bash
kubectl apply -f deploy-uds-core.yaml
```

Expected output:

```text
udsbundlejob.forge.dev/deploy-uds-core created
```

Watch deployment progress:

```bash
kubectl get udsbundlejob deploy-uds-core -w
```

Expected output:

```text
NAME              PHASE      AGE
deploy-uds-core   Pending    2s
deploy-uds-core   Running    15s
deploy-uds-core   Succeeded  8m30s
```

### 4. Deploy to Remote Cluster

Deploys a bundle to a different cluster using a kubeconfig Secret.

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: deploy-remote
  namespace: default
spec:
  serviceAccountName: uds-bundle-operator
  action: Deploy
  source:
    type: OCI
    oci:
      ref: ghcr.io/myorg/bundles/platform:latest
  deploy:
    target: RemoteCluster
    kubeconfigSecretRef:
      name: target-cluster-kubeconfig
```

Create the kubeconfig Secret:

```bash
kubectl create secret generic target-cluster-kubeconfig \
  --from-file=kubeconfig=/path/to/target/kubeconfig \
  --namespace default
```

Expected output:

```text
secret/target-cluster-kubeconfig created
```

Apply the UDSBundleJob:

```bash
kubectl apply -f deploy-remote.yaml
```

Expected output:

```text
udsbundlejob.forge.dev/deploy-remote created
```

### 5. Publish to S3

Publishes an existing bundle to an S3 bucket.

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: publish-s3
  namespace: default
spec:
  serviceAccountName: uds-bundle-cicd
  action: Publish
  source:
    type: Git
    git:
      url: https://github.com/myorg/bundle
      ref: v2.0.0
  publish:
    destination:
      type: S3
      s3:
        bucket: my-uds-bundles
        key: platform-v2.0.0.tar.zst
        region: us-east-1
        credentialsSecretRef:
          name: aws-s3-creds
```

Create the AWS credentials Secret:

```bash
kubectl create secret generic aws-s3-creds \
  --from-literal=aws_access_key_id=AKIAIOSFODNN7EXAMPLE \
  --from-literal=aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY \
  --namespace default
```

Expected output:

```text
secret/aws-s3-creds created
```

Apply the publish job:

```bash
kubectl apply -f publish-s3.yaml
```

Expected output:

```text
udsbundlejob.forge.dev/publish-s3 created
```

## Policy Configuration

Forge enforces UDS bundle policies through ServiceAccount annotations. The admission webhook validates operations before they're created, and the controller re-validates before executing Jobs.

### Policy Annotations

All annotations use the prefix `forge.forge.dev/`:

| Annotation | Values | Description |
|------------|--------|-------------|
| `allowed-actions` | `create`, `publish`, `deploy` | Comma-separated list of allowed actions |
| `allowed-git-repos` | `*` or glob patterns | Git repository patterns (e.g., `github.com/myorg/*`) |
| `allowed-oci-registries` | `*` or glob patterns | OCI registry patterns (e.g., `ghcr.io/myorg/*`) |
| `allowed-s3-buckets` | `*` or bucket names | S3 bucket names (comma-separated) |
| `allowed-deploy-namespaces` | `*` or namespace names | Target deployment namespaces (comma-separated) |

### Example ServiceAccounts

Forge provides three reference ServiceAccount configurations for UDS bundles in `examples/policies/uds/`:

#### 1. Permissive (Development)

**File**: `examples/policies/uds/permissive-serviceaccount.yaml`
**Use Case**: Development environments, trusted users, rapid prototyping

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: uds-bundle-operator-permissive
  namespace: forge-system
  annotations:
    forge.forge.dev/allowed-actions: "create,publish,deploy"
    forge.forge.dev/allowed-git-repos: "*"
    forge.forge.dev/allowed-oci-registries: "*"
    forge.forge.dev/allowed-deploy-namespaces: "*"
    forge.forge.dev/allowed-s3-buckets: "*"
```

**Warning**: Not recommended for production. Use only in controlled development environments.

#### 2. Restricted (Production)

**File**: `examples/policies/uds/restricted-serviceaccount.yaml`
**Use Case**: Production environments, compliance requirements, least-privilege principle

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: uds-bundle-operator-restricted
  namespace: forge-system
  annotations:
    forge.forge.dev/allowed-actions: "create,deploy"
    forge.forge.dev/allowed-git-repos: "github.com/cncf/*,github.com/myorg/*"
    forge.forge.dev/allowed-deploy-namespaces: "uds-dev,uds-staging"
    # No OCI or S3 access (internal builds only)
```

#### 3. CI/CD Pipeline

**File**: `examples/policies/uds/ci-cd-serviceaccount.yaml`
**Use Case**: CI/CD pipelines, build automation, artifact publishing

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: uds-bundle-cicd
  namespace: forge-system
  annotations:
    forge.forge.dev/allowed-actions: "create,publish"
    forge.forge.dev/allowed-git-repos: "github.com/myorg/*,gitlab.mycompany.com/*"
    forge.forge.dev/allowed-oci-registries: "registry.mycompany.com/*"
    forge.forge.dev/allowed-s3-buckets: "mycompany-uds-bundles,mycompany-uds-bundles-staging"
    # No deployment permissions (separation of concerns)
```

### Applying Policy

1. Choose the appropriate ServiceAccount template for your use case
2. Customize the annotations to match your security requirements
3. Apply the ServiceAccount:

```bash
kubectl apply -f examples/policies/uds/restricted-serviceaccount.yaml
```

Expected output:

```text
serviceaccount/uds-bundle-operator-restricted created
role.rbac.authorization.k8s.io/uds-bundle-operator-restricted created
rolebinding.rbac.authorization.k8s.io/uds-bundle-operator-restricted created
```

4. Reference in your UDSBundleJob:

```yaml
spec:
  serviceAccountName: uds-bundle-operator-restricted
  # ...
```

For complete policy examples with RBAC configurations and credential Secrets, see:

- `examples/policies/uds/README.md` - Complete policy reference guide
- `examples/policies/uds/permissive-serviceaccount.yaml`
- `examples/policies/uds/restricted-serviceaccount.yaml`
- `examples/policies/uds/ci-cd-serviceaccount.yaml`

## Troubleshooting

### Bundle Creation Failures

If a bundle creation fails, check the following:

1. **Check the UDSBundleJob status**:

    ```bash
    kubectl get udsbundlejob my-bundle -o yaml
    ```

    Expected output (showing failure):

    ```yaml
    status:
      phase: Failed
      message: "Create failed: package validation error in component istio"
      startTime: "2025-12-25T10:00:00Z"
      completionTime: "2025-12-25T10:15:30Z"
    ```

2. **Find the failed Job**:

    ```bash
    kubectl get jobs -l forge.forge.dev/package=my-bundle
    ```

    Expected output:

    ```text
    NAME                  COMPLETIONS   DURATION   AGE
    my-bundle-create      0/1           15m30s     16m
    ```

3. **Check Job logs for specific errors**:

    ```bash
    kubectl logs -l job-name=my-bundle-create --tail=100
    ```

    Common errors:
    - `package not found`: Zarf package specified in bundle doesn't exist
    - `validation error`: uds-bundle.yaml syntax or configuration errors
    - `permission denied`: Git/OCI credentials missing or incorrect

### Policy Validation Failures

If the webhook rejects your UDSBundleJob:

```text
Error from server: admission webhook "validate.udsbundlejob.forge.dev" denied the request:
action "create" not allowed by ServiceAccount annotations
```

**Fix**:

1. Check ServiceAccount annotations:

    ```bash
    kubectl get sa -n forge-system uds-bundle-operator-restricted -o yaml
    ```

2. Verify the action is in `allowed-actions`:

    ```yaml
    annotations:
      forge.forge.dev/allowed-actions: "create,deploy"  # ✅ create is allowed
    ```

3. Verify repository matches `allowed-git-repos` pattern:

    ```yaml
    annotations:
      forge.forge.dev/allowed-git-repos: "github.com/myorg/*"  # ✅ pattern matches
    ```

    Example: `github.com/myorg/bundle` matches `github.com/myorg/*`

### Deployment Failures

Common deployment issues:

1. **Namespace doesn't exist**:

    Error: `namespace "my-namespace" not found`

    **Fix**: Create the namespace first:

    ```bash
    kubectl create namespace my-namespace
    ```

2. **Insufficient RBAC permissions**:

    Error: `failed to create resource: forbidden: User cannot create resource`

    **Fix**: Ensure the ServiceAccount has deployment permissions in target namespaces. See `examples/policies/uds/restricted-serviceaccount.yaml` for Role examples.

3. **Resource conflicts**:

    Error: `resource "my-resource" already exists`

    **Fix**: Either delete the existing resource or use UDS overrides to skip/update it.

### OCI Registry Issues

If bundle push/pull fails:

1. **Check credentials Secret exists**:

    ```bash
    kubectl get secret oci-registry-creds -o yaml
    ```

2. **Verify Secret format** (must be `.dockerconfigjson`):

    ```bash
    kubectl get secret oci-registry-creds -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d | jq
    ```

    Expected format:

    ```json
    {
      "auths": {
        "ghcr.io": {
          "username": "myuser",
          "password": "ghp_token123",  // pragma: allowlist secret
          "auth": "base64encodedusername:password"
        }
      }
    }
    ```

3. **Create properly formatted Secret**:

    ```bash
    kubectl create secret docker-registry oci-registry-creds \
      --docker-server=ghcr.io \
      --docker-username=myuser \
      --docker-password=ghp_token123 \  # pragma: allowlist secret
      --namespace default
    ```

### S3 Access Issues

If S3 upload/download fails:

1. **Check credentials Secret**:

    ```bash
    kubectl get secret aws-s3-creds -o yaml
    ```

2. **Verify required fields**:

    ```bash
    kubectl get secret aws-s3-creds -o jsonpath='{.data}' | jq
    ```

    Required fields:
    - `aws_access_key_id`
    - `aws_secret_access_key`
    - Optional: `aws_session_token` (for temporary credentials)

3. **Test bucket access** (from a test pod):

    ```bash
    aws s3 ls s3://my-uds-bundles --region us-east-1
    ```

### Job Stuck in Pending

If the Job pod never starts:

1. **Check pod status**:

    ```bash
    kubectl get pods -l job-name=my-bundle-create
    ```

2. **Check events for scheduling issues**:

    ```bash
    kubectl describe pod <pod-name>
    ```

    Common issues:
    - Insufficient resources (CPU/memory)
    - Image pull errors
    - Node selector mismatch
    - Persistent volume claim not bound

### Controller Issues

If UDSBundleJobs aren't being processed:

1. **Check controller is running**:

    ```bash
    kubectl get pods -n forge-system -l app.kubernetes.io/component=controller
    ```

    Expected output:

    ```text
    NAME                                READY   STATUS    RESTARTS   AGE
    forge-controller-<hash>             1/1     Running   0          1h
    ```

2. **Check controller logs**:

    ```bash
    kubectl logs -n forge-system -l app.kubernetes.io/component=controller --tail=100
    ```

    Look for errors related to:
    - Webhook communication failures
    - RBAC permission errors
    - Resource watch errors

## Differences Between v1alpha1 and v1alpha2

| Aspect | v1alpha1 (Deprecated) | v1alpha2 (Recommended) |
|--------|----------------------|------------------------|
| **Resource Name** | `UDSBundleJob` | `UDSBundleJob` |
| **Action Field** | `bundleAction` | `action` |
| **Action Values** | `BundleActionCreate`, `BundleActionPublish`, etc. | `Create`, `Publish`, `Deploy` |
| **Naming Convention** | Bundle-prefixed actions | Consistent with ZarfPackageJob |
| **API Group** | `forge.dev/v1alpha1` | `forge.dev/v1alpha2` |
| **Deprecation** | Deprecated, removed in v0.10.0 | Active, recommended |

### Migration Example

**v1alpha1 (Old)**:

```yaml
apiVersion: forge.dev/v1alpha1
kind: UDSBundleJob
metadata:
  name: my-bundle
spec:
  bundleAction: BundleActionCreate
  source:
    type: Git
    git:
      url: https://github.com/myorg/bundle
```

**v1alpha2 (New)**:

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: my-bundle
spec:
  action: Create
  source:
    type: Git
    git:
      url: https://github.com/myorg/bundle
```

**Key changes**:

1. `kind: UDSBundleJob` → `kind: UDSBundleJob`
2. `apiVersion: forge.dev/v1alpha1` → `apiVersion: forge.dev/v1alpha2`
3. `bundleAction: BundleActionCreate` → `action: Create`

For a complete migration guide with automated conversion tools, see [V1ALPHA2_MIGRATION.md](../operations/V1ALPHA2_MIGRATION.md).

## Additional Resources

- **Policy Examples**: `examples/policies/uds/` - Complete ServiceAccount templates and RBAC configurations
- **Sample Jobs**: `examples/samples/uds/` - Ready-to-use UDSBundleJob examples
- **Migration Guide**: [V1ALPHA2_MIGRATION.md](../operations/V1ALPHA2_MIGRATION.md) - v1alpha1 → v1alpha2 migration
- **Troubleshooting**: [UDS_TROUBLESHOOTING.md](../operations/UDS_TROUBLESHOOTING.md) - Detailed troubleshooting guide
- **Zarf Guide**: [USER_GUIDE.md](USER_GUIDE.md) - For single Zarf packages
- **Namespace-Scoped Deployment**: [NAMESPACE_SCOPED_DEPLOYMENT.md](../operations/NAMESPACE_SCOPED_DEPLOYMENT.md)
