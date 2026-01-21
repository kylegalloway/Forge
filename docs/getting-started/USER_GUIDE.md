# Forge User Guide

## Introduction

Forge allows you to manage Zarf packages and UDS bundles using Kubernetes Custom Resources. This guide provides detailed instructions on how to use Forge to build, publish, and deploy your artifacts.

> **For Developers**: If you're contributing to Forge and need to test local changes, see [KIND_SETUP.md](KIND_SETUP.md) for the developer workflow

## When to Use What

### Use Zarf Packages (`ZarfPackageJob`)

- Deploying a single application or service
- Creating standalone packages
- Building individual components
- Testing individual Zarf packages

### Use UDS Bundles (`UDSBundleJob`)

- Deploying multi-package platforms (e.g., UDS Core with Istio + Keycloak + Grafana)
- Managing complex deployments with interdependencies
- Creating opinionated software distributions
- Bundling multiple Zarf packages for air-gapped delivery

## Installation

### Prerequisites

- Kubernetes cluster (1.24+)
- Helm 3.8+
- kubectl configured for your cluster

### Install from Helm Repository

Add the Forge Helm repository and install:

```bash
# Add Helm repository
helm repo add forge https://kylegalloway.github.io/Forge
helm repo update

# Install Forge
helm install forge forge/forge \
  --namespace forge-system \
  --create-namespace \
  --version 0.9.1
```

Expected output:

```text
"forge" has been added to your repositories
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "forge" chart repository
Update Complete. ⎈Happy Helming!⎈

NAME: forge
LAST DEPLOYED: Thu Dec 19 10:00:00 2025
NAMESPACE: forge-system
STATUS: deployed
REVISION: 1
```

**Container Images Used**:

- Controller: `ghcr.io/kylegalloway/forge/forge-controller:latest` (or `:v0.9.1` for specific version)
- Webhook: `ghcr.io/kylegalloway/forge/forge-webhook:latest` (or `:v0.9.1` for specific version)

### Installation Options

**Default Installation** (production-ready):

```bash
helm install forge forge/forge \
  --version 0.9.1 \
  --namespace forge-system \
  --create-namespace
```

**With Custom Configuration**:

```bash
helm install forge forge/forge \
  --version 0.9.1 \
  --namespace forge-system \
  --create-namespace \
  --set controller.replicaCount=2 \
  --set webhook.replicaCount=3
```

### Verify Installation

```bash
# Check that Forge is running
kubectl get pods -n forge-system

# Expected output:
# NAME                                 READY   STATUS    RESTARTS   AGE
# forge-controller-<hash>              1/1     Running   0          1m
# forge-webhook-<hash>                 1/1     Running   0          1m
```

For additional deployment options and configurations, see [DEPLOYMENT.md](DEPLOYMENT.md).

### Deployment Modes

Forge supports two deployment modes depending on your cluster permissions and security requirements.

**Cluster-Wide Deployment (Default)**:

- Watches all namespaces
- ZarfPackageJobs and UDSBundleJobs can be created in any namespace
- ServiceAccounts can be in any namespace
- Suitable for platform teams

**Namespace-Scoped Deployment**:

- Watches only forge-system namespace
- All resources must be in forge-system
- Minimal permissions (Role, not ClusterRole)
- Suitable for restricted clusters, individual teams

📖 **Detailed Guide**: See [NAMESPACE_SCOPED_DEPLOYMENT.md](../operations/NAMESPACE_SCOPED_DEPLOYMENT.md) for complete instructions.

## Core Concepts

### ZarfPackageJob

The primary resource for defining operations on a single Zarf package.

### UDSBundleJob

For UDS bundles, Forge provides a unified API with consistent naming.

#### API Versions

- **v1alpha1** (deprecated): `UDSBundleJob` with `BundleAction` types
- **v1alpha2** (recommended): `UDSBundleJob` with simplified `Action` types matching Zarf

The v1alpha2 API uses the same action names as Zarf (`Create`, `Publish`, `Deploy`) instead of the v1alpha1 prefixed names (`BundleActionCreate`, etc.). This makes the API consistent across both package types.

**Migration**: v1alpha1 will be supported until Forge v0.10.0 (~6 months). See [V1ALPHA2_MIGRATION.md](../operations/V1ALPHA2_MIGRATION.md) for the complete migration guide.

#### What is a UDS Bundle?

A UDS bundle is a collection of multiple Zarf packages defined in a `uds-bundle.yaml` file. Bundles enable:

- **Platform Deployments**: Deploy complete platforms (e.g., DevSecOps stack with GitLab, ArgoCD, monitoring)
- **Dependency Management**: Define package order and dependencies
- **Configuration Overrides**: Set variables and configurations across packages
- **Air-Gapped Distribution**: Bundle everything needed for offline installation

### Actions

**For Zarf Packages**:
- **Build**: Creates a Zarf package from source
- **Publish**: Uploads a package to a registry (OCI or S3)
- **Deploy**: Installs a package into a cluster
- **Composite Actions**: `BuildPublish`, `BuildDeploy`, `PublishDeploy`, `BuildPublishDeploy`

**For UDS Bundles** (v1alpha2):
- **Create**: Builds a UDS bundle from source (equivalent to `uds create`)
- **Publish**: Uploads a bundle to a registry (OCI or S3)
- **Deploy**: Installs a bundle into a cluster (equivalent to `uds deploy`)
- **Composite Actions**: `CreatePublish`, `CreateDeploy`, `PublishDeploy`, `CreatePublishDeploy`

### Sources

Both Zarf packages and UDS bundles can be sourced from:

- **Git**: Clone from a Git repository
- **OCI**: Pull existing artifact from an OCI registry
- **S3**: Download from an S3 bucket
- **Local**: Use local files (development only, disabled by default)

### Destinations

Published artifacts can be stored in:

- **OCI**: Push to OCI registries (GHCR, Harbor, Artifactory, etc.)
- **S3**: Upload to S3-compatible storage

## Zarf Package Examples

### 1. Build a Package from Git

Builds a package from a public Git repository.

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-example
  namespace: default
spec:
  serviceAccountName: default
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/stefanprodan/podinfo.git
      ref: 6.7.0
      path: charts/podinfo
```

Apply with:

```bash
kubectl apply -f build-example.yaml
```

Expected output:

```text
zarfpackagejob.forge.dev/build-example created
```

Watch progress:

```bash
kubectl get zarfpackagejob build-example -w
```

Expected output:

```text
NAME            PHASE      AGE
build-example   Pending    2s
build-example   Running    5s
build-example   Succeeded  45s
```

### 2. Build and Publish to OCI

Builds a package and immediately publishes it to an OCI registry.

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-publish-oci
  namespace: default
spec:
  serviceAccountName: default
  action: BuildPublish
  source:
    type: Git
    git:
      url: https://github.com/stefanprodan/podinfo.git
      ref: 6.7.0
      path: charts/podinfo
  publish:
    destination:
      type: OCI
      oci:
        registry: ghcr.io
        repository: myuser/dos-games
        tag: 1.0.0
        credentialRef:
          name: oci-creds # Secret containing .dockerconfigjson
```

Apply with:

```bash
kubectl apply -f build-publish-oci.yaml
```

Expected output:

```text
zarfpackagejob.forge.dev/build-publish-oci created
```

Watch progress:

```bash
kubectl get zarfpackagejob build-publish-oci -n default
```

Expected output:

```text
NAME                 PHASE      AGE
build-publish-oci    Succeeded  2m15s
```

### 3. Deploy from S3

Deploys a package stored in an S3 bucket.

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: deploy-s3
  namespace: default
spec:
  serviceAccountName: default
  action: Deploy
  source:
    type: S3
    s3:
      bucket: my-zarf-packages
      key: dos-games-v1.0.0.tar.zst
      region: us-east-1
      credentialRef:
        name: aws-creds # Secret with 'access-key-id' and 'secret-access-key' keys
  deploy:
    target: InCluster
    namespace: games
```

Apply with:

```bash
kubectl apply -f deploy-s3.yaml
```

Expected output:

```text
zarfpackagejob.forge.dev/deploy-s3 created
```

Check deployment status:

```bash
kubectl describe zarfpackagejob deploy-s3 -n default
```

Expected output (relevant sections):

```text
Name:         deploy-s3
Namespace:    default
Status:
  Phase:             Succeeded
  Completion Time:   2025-12-19T10:05:30Z
  Message:          Package deployed successfully to namespace games
```

## UDS Bundle Examples

### 1. Create a Bundle from Git

Creates a UDS bundle from a public Git repository containing a `uds-bundle.yaml` file.

```yaml
apiVersion: forge.dev/v1alpha3
kind: UDSBundleJob
metadata:
  name: create-bundle
  namespace: default
spec:
  serviceAccountName: uds-bundle-operator
  action: Create
  source:
    type: Git
    git:
      url: https://github.com/prometheus/prometheus
      ref: v2.45.0
```

Apply with:

```bash
kubectl apply -f create-bundle.yaml
```

Expected output:

```text
udsbundlejob.forge.dev/create-bundle created
```

Watch progress:

```bash
kubectl get udsbundlejob create-bundle -w
```

Expected output:

```text
NAME             PHASE      AGE
create-bundle    Pending    2s
create-bundle    Running    10s
create-bundle    Succeeded  3m45s
```

### 2. Create and Publish to OCI

Creates a bundle and immediately publishes it to an OCI registry.

```yaml
apiVersion: forge.dev/v1alpha3
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
        credentialRef:
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
apiVersion: forge.dev/v1alpha3
kind: UDSBundleJob
metadata:
  name: deploy-bundle
  namespace: default
spec:
  serviceAccountName: uds-bundle-operator
  action: Deploy
  source:
    type: OCI
    oci:
      reference: ghcr.io/myorg/bundles/app-bundle:1.0.0
      credentialRef:
        name: oci-registry-creds
  deploy:
    target: InCluster
```

Apply with:

```bash
kubectl apply -f deploy-bundle.yaml
```

Expected output:

```text
udsbundlejob.forge.dev/deploy-bundle created
```

Watch deployment progress:

```bash
kubectl get udsbundlejob deploy-bundle -w
```

Expected output:

```text
NAME            PHASE      AGE
deploy-bundle   Pending    2s
deploy-bundle   Running    15s
deploy-bundle   Succeeded  8m30s
```

### 4. Deploy to External Cluster

Deploys a bundle to a different cluster using a kubeconfig Secret.

```yaml
apiVersion: forge.dev/v1alpha3
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
      reference: ghcr.io/myorg/bundles/platform:latest
  deploy:
    target: ExternalCluster
    kubeconfig:
      secretRef:
        name: target-cluster-kubeconfig
        namespace: default
      key: kubeconfig
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
apiVersion: forge.dev/v1alpha3
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
        credentialRef:
          name: aws-s3-creds
```

Create the AWS credentials Secret:

```bash
# S3 sources and destinations use consistent key names
kubectl create secret generic aws-s3-creds \
  --from-literal=access-key-id=AKIAIOSFODNN7EXAMPLE \
  --from-literal=secret-access-key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY \
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

## Policy Enforcement

Forge uses `ServiceAccount` annotations to enforce policies for both Zarf packages and UDS bundles.

### Setup

1. Create a `ServiceAccount`.
2. Annotate it with allowed actions and resources.

**For Zarf Packages**:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: restricted-builder
  namespace: default  # cluster-wide mode
  # namespace: forge-system  # namespace-scoped mode (all SAs must be here)
  annotations:
    forge.dev/allowed-actions: "Build,Publish"
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"
```

**For UDS Bundles**:

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

Apply with:

```bash
kubectl apply -f restricted-builder-sa.yaml
```

Expected output:

```text
serviceaccount/restricted-builder created
```

**Note**: In namespace-scoped deployments, all ServiceAccounts must be created in the `forge-system` namespace.

### UDS Bundle Policy Annotations

All annotations use the prefix `forge.forge.dev/`:

| Annotation | Values | Description |
|------------|--------|-------------|
| `allowed-actions` | `create`, `publish`, `deploy` | Comma-separated list of allowed actions |
| `allowed-git-repos` | `*` or glob patterns | Git repository patterns (e.g., `github.com/myorg/*`) |
| `allowed-oci-registries` | `*` or glob patterns | OCI registry patterns (e.g., `ghcr.io/myorg/*`) |
| `allowed-s3-buckets` | `*` or bucket names | S3 bucket names (comma-separated) |
| `allowed-deploy-namespaces` | `*` or namespace names | Target deployment namespaces (comma-separated) |

### Example ServiceAccount Configurations

Forge provides reference ServiceAccount configurations in `examples/policies/`:

- **Zarf**: `examples/policies/zarf/` - Configurations for ZarfPackageJob
- **UDS**: `examples/policies/uds/` - Configurations for UDSBundleJob

Each directory contains three templates:

#### 1. Permissive (Development)

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

4. Reference in your job:

```yaml
spec:
  serviceAccountName: uds-bundle-operator-restricted
  # ...
```

### Usage

Reference the ServiceAccount in your `ZarfPackageJob` or `UDSBundleJob`:

```yaml
spec:
  serviceAccountName: restricted-builder
  # ...
```

If the job tries to use a disallowed source or action, the controller will reject it.

## Troubleshooting

### Job Failures

Forge creates Kubernetes Jobs for each operation. If an operation fails:

1. Check the job status (ZarfPackageJob or UDSBundleJob):

    ```bash
    kubectl get ZarfPackageJob my-package -o yaml
    # or
    kubectl get UDSBundleJob my-bundle -o yaml
    ```

    Expected output (showing failed status):

    ```text
    status:
      phase: Failed
      message: "Build failed: package validation error"
      startTime: "2025-12-19T10:00:00Z"
      completionTime: "2025-12-19T10:02:30Z"
    ```

2. Find the failed Job (named `<package-name>-<action>`):

    ```bash
    kubectl get jobs -l forge.dev/package=my-package
    # or for UDS bundles
    kubectl get jobs -l forge.forge.dev/package=my-bundle
    ```

    Expected output:

    ```text
    NAME                  COMPLETIONS   DURATION   AGE
    my-package-build      0/1           2m30s      3m
    ```

3. Check the Job logs:

    ```bash
    # Find the pod
    kubectl get pods -l job-name=<job-name>
    # Get logs
    kubectl logs <pod-name>
    ```

### UDS Bundle Creation Failures

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

If the webhook rejects your job:

**Zarf Package Example**:
```text
Error from server: admission webhook "validate.zarfpackagejob.forge.dev" denied the request:
action "Build" not allowed by ServiceAccount annotations
```

**UDS Bundle Example**:
```text
Error from server: admission webhook "validate.udsbundlejob.forge.dev" denied the request:
action "create" not allowed by ServiceAccount annotations
```

**Fix**:

1. Check ServiceAccount annotations:

    ```bash
    kubectl get sa -n forge-system restricted-builder -o yaml
    ```

2. Verify the action is in `allowed-actions`:

    ```yaml
    annotations:
      forge.dev/allowed-actions: "Build,Deploy"  # ✅ Build is allowed
      # or for UDS
      forge.forge.dev/allowed-actions: "create,deploy"  # ✅ create is allowed
    ```

3. Verify repository matches the allowed pattern:

    ```yaml
    annotations:
      forge.dev/allowed-source-repos: "https://github.com/myorg/*"  # ✅ pattern matches
      # or for UDS
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

    **Fix**: Ensure the ServiceAccount has deployment permissions in target namespaces. See `examples/policies/` for Role examples.

3. **Resource conflicts**:

    Error: `resource "my-resource" already exists`

    **Fix**: Either delete the existing resource or use overrides to skip/update it.

### OCI Registry Issues

If bundle/package push/pull fails:

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

If jobs aren't being processed:

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

### Webhook Issues

If you cannot create job resources:

1. Check if the webhook pod is running:

    ```bash
    kubectl get pods -n forge-system
    ```

2. Check webhook logs for validation errors.

## UDS Bundle API Migration

### Differences Between v1alpha1 and v1alpha2

| Aspect | v1alpha1 (Deprecated) | v1alpha2 (Recommended) |
|--------|----------------------|------------------------|
| **Resource Name** | `UDSBundleJob` | `UDSBundleJob` |
| **Action Field** | `bundleAction` | `action` |
| **Action Values** | `BundleActionCreate`, `BundleActionPublish`, etc. | `Create`, `Publish`, `Deploy` |
| **Naming Convention** | Bundle-prefixed actions | Consistent with ZarfPackageJob |
| **API Group** | `forge.dev/v1alpha3` | `forge.dev/v1alpha3` |
| **Deprecation** | Deprecated, removed in v0.10.0 | Active, recommended |

### Migration Example

**v1alpha1 (Old)**:

```yaml
apiVersion: forge.dev/v1alpha3
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
apiVersion: forge.dev/v1alpha3
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

1. `apiVersion: forge.dev/v1alpha3` → `apiVersion: forge.dev/v1alpha3`
2. `bundleAction: BundleActionCreate` → `action: Create`

For a complete migration guide with automated conversion tools, see [V1ALPHA2_MIGRATION.md](../operations/V1ALPHA2_MIGRATION.md).

## Additional Resources

### Documentation

- **Deployment Guide**: [DEPLOYMENT.md](DEPLOYMENT.md) - Helm installation and configuration
- **Developer Guide**: [KIND_SETUP.md](KIND_SETUP.md) - Local development with KIND
- **Namespace-Scoped Deployment**: [NAMESPACE_SCOPED_DEPLOYMENT.md](../operations/NAMESPACE_SCOPED_DEPLOYMENT.md)
- **Migration Guide**: [V1ALPHA2_MIGRATION.md](../operations/V1ALPHA2_MIGRATION.md) - v1alpha1 → v1alpha2 migration

### Examples

- **Zarf Policy Examples**: `examples/policies/zarf/` - ServiceAccount templates and RBAC configurations
- **UDS Policy Examples**: `examples/policies/uds/` - ServiceAccount templates and RBAC configurations
- **Sample Jobs**: `examples/samples/` - Ready-to-use job examples

### External Resources

- [Zarf Documentation](https://zarf.dev) - Official Zarf package documentation
- [UDS Documentation](https://uds.defenseunicorns.com) - UDS bundle documentation
