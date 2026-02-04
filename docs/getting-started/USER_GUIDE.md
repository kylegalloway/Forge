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
  --version 0.11.17
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

- Controller: `ghcr.io/kylegalloway/forge/forge-controller:latest` (or `:v0.11.17` for specific version)
- Webhook: `ghcr.io/kylegalloway/forge/forge-webhook:latest` (or `:v0.11.17` for specific version)

### Installation Options

**Default Installation** (production-ready):

```bash
helm install forge forge/forge \
  --version 0.11.17 \
  --namespace forge-system \
  --create-namespace
```

**With Custom Configuration**:

```bash
helm install forge forge/forge \
  --version 0.11.17 \
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

### kubectl-forge CLI Plugin

The `kubectl-forge` plugin provides developer-friendly commands for working with Forge jobs. Install it to simplify common workflows like listing jobs, debugging failures, and downloading artifacts.

**Installation**:

```bash
# Build from source
make build-kubectl-forge

# Copy to your PATH
cp bin/kubectl-forge /usr/local/bin/
```

**Quick Reference**:

| Command | Description |
|---------|-------------|
| `kubectl forge status` | Check Forge system health (controller, webhook, CRDs) |
| `kubectl forge list` | List all Forge jobs |
| `kubectl forge list --watch` | Watch jobs with live updates |
| `kubectl forge diagnose <job>` | Diagnose problems with a job |
| `kubectl forge get job <job>` | Get detailed job information |
| `kubectl forge get logs <job>` | Get logs from a job's pods |
| `kubectl forge get pods <job>` | List pods for a job |
| `kubectl forge get events <job>` | Get events for a job |
| `kubectl forge download <job>` | Download artifacts from a completed job |
| `kubectl forge debug <job>` | Debug a failed or running job |
| `kubectl forge debug <job> --all-pods` | Debug all pods in a multi-pod job |
| `kubectl forge cancel <job>` | Cancel a running job |
| `kubectl forge retry <job>` | Retry a failed job |
| `kubectl forge retry --all-failed` | Retry all failed jobs in namespace |
| `kubectl forge logs controller` | Get controller logs (for operators) |
| `kubectl forge logs webhook` | Get webhook logs (for operators) |

See [kubectl-forge Reference](#kubectl-forge-reference) for detailed usage.

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

For UDS bundles, Forge provides a unified API with consistent naming that mirrors the ZarfPackageJob API.

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

**For UDS Bundles**:
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
- **S3**: Upload to S3-compatible storage (AWS S3, MinIO, Ceph, etc.)

### Variables

Both Zarf packages and UDS bundles support passing variables during build/create and deploy operations. Variables are passed as `--set KEY=VALUE` flags to the underlying CLI commands.

#### Zarf Package Variables

**Build Variables** (`spec.build.variables`):
Variables passed to `zarf package create` during the build phase.

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-with-vars
spec:
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/myorg/my-package
      ref: main
  build:
    variables:
      IMAGE_TAG: "v1.2.3"
      REGISTRY: "ghcr.io/myorg"
```

**Deploy Variables** (`spec.deploy.variables`):
Variables passed to `zarf package deploy` during the deploy phase.

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: deploy-with-vars
spec:
  action: Deploy
  source:
    type: OCI
    oci:
      reference: ghcr.io/myorg/my-package:1.0.0
  deploy:
    target: InCluster
    variables:
      REPLICAS: "3"
      LOG_LEVEL: "debug"
```

#### UDS Bundle Variables

**Deploy Variables** (`spec.deploy.variables`):
Variables passed to `uds deploy` during the bundle deploy phase.

```yaml
apiVersion: forge.dev/v1alpha3
kind: UDSBundleJob
metadata:
  name: deploy-with-vars
spec:
  action: Deploy
  source:
    type: OCI
    oci:
      reference: ghcr.io/myorg/my-bundle:1.0.0
  deploy:
    target: InCluster
    variables:
      DOMAIN: "example.com"
      ENABLE_MONITORING: "true"
```

### Credential Configuration

Forge supports credentials for private repositories and registries. All credential types work with any host, not just major public providers.

#### Git Credentials

| Server Type | Secret Keys | Example |
|-------------|-------------|---------|
| GitHub, GitLab.com | `token` | OAuth-style authentication |
| Gitea, GitLab self-hosted, Bitbucket Server | `username` + `token` | Basic auth with token |
| Self-hosted (password auth) | `username` + `password` | Basic auth with password |
| Any server with SSH | `ssh-key` | SSH key authentication |

**Example: Private Git repository with basic auth (Gitea, GitLab self-hosted)**

```yaml
# Secret for self-hosted git server
apiVersion: v1
kind: Secret
metadata:
  name: gitea-creds
type: Opaque
stringData:
  username: "myuser"
  token: "my-access-token"  # or use 'password' key
---
# ZarfPackageJob referencing the credentials
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-from-gitea
spec:
  serviceAccountName: default
  action: Build
  source:
    type: Git
    git:
      url: http://gitea.internal:3000/myorg/repo.git
      ref: main
      credentialRef:
        name: gitea-creds
```

#### S3 Credentials

S3 sources and destinations support custom endpoints for S3-compatible storage (MinIO, Ceph, etc.):

```yaml
source:
  type: S3
  s3:
    bucket: my-bucket
    key: packages/app.tar.zst
    region: us-east-1
    endpoint: http://minio.internal:9000  # Optional: for S3-compatible storage
    credentialRef:
      name: s3-creds
```

See `examples/samples/zarf/05-credentials-showcase/` for complete credential examples.

## Zarf Package Examples

### 1. Build a Package from Git

Builds a package from a public Git repository with build-time variables.

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
  # Optional: Pass variables to zarf package create
  build:
    variables:
      IMAGE_TAG: "6.7.0"
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

Creates a UDS bundle from a public Git repository containing a `uds-bundle.yaml` file with create-time variables.

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
    externalCluster:
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
    forge.dev/allowed-actions: "Create,Deploy"
    forge.dev/allowed-source-repos: "github.com/cncf/*,github.com/myorg/*"
    forge.dev/allowed-deploy-targets: "uds-dev,uds-staging"
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

All annotations use the prefix `forge.dev/`:

| Annotation | Values | Description |
|------------|--------|-------------|
| `allowed-actions` | `Create`, `Publish`, `Deploy` | Comma-separated list of allowed actions |
| `allowed-source-repos` | `*` or glob patterns | Source Git repository patterns (e.g., `github.com/myorg/*`) |
| `allowed-source-registries` | `*` or glob patterns | Source OCI registry patterns (e.g., `ghcr.io/myorg/*`) |
| `allowed-publish-registries` | `*` or glob patterns | Publish OCI registry patterns (e.g., `ghcr.io/myorg/*`) |
| `allowed-source-buckets` | `*` or bucket names | Source S3 bucket names (comma-separated) |
| `allowed-publish-buckets` | `*` or bucket names | Publish S3 bucket names (comma-separated) |
| `allowed-deploy-targets` | `*` or namespace names | Target deployment namespaces (comma-separated) |

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
    forge.dev/allowed-actions: "Create,Publish,Deploy"
    forge.dev/allowed-source-repos: "*"
    forge.dev/allowed-source-registries: "*"
    forge.dev/allowed-publish-registries: "*"
    forge.dev/allowed-deploy-targets: "*"
    forge.dev/allowed-source-buckets: "*"
    forge.dev/allowed-publish-buckets: "*"
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
    forge.dev/allowed-actions: "Create,Deploy"
    forge.dev/allowed-source-repos: "github.com/cncf/*,github.com/myorg/*"
    forge.dev/allowed-deploy-targets: "uds-dev,uds-staging"
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
    forge.dev/allowed-actions: "Create,Publish"
    forge.dev/allowed-source-repos: "github.com/myorg/*,gitlab.mycompany.com/*"
    forge.dev/allowed-source-registries: "registry.mycompany.com/*"
    forge.dev/allowed-publish-registries: "registry.mycompany.com/*"
    forge.dev/allowed-publish-buckets: "mycompany-uds-bundles,mycompany-uds-bundles-staging"
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

## Debug Mode

Debug mode allows operators to inspect job pods before they complete, enabling manual inspection and troubleshooting of the execution environment.

### Enabling Debug Mode

There are three ways to enable debug mode, with increasing granularity:

#### Global Debug Mode (Environment Variable)

Set `FORGE_DEBUG_MODE=true` on the controller to enable debug mode for all jobs:

```bash
# Via Helm
helm upgrade forge forge/forge \
  --set controller.debugMode=true

# Or set the environment variable directly
kubectl set env deployment/forge-controller -n forge-system FORGE_DEBUG_MODE=true
```

#### Per-Job Debug Mode

Enable debug mode for a specific job using `spec.debugMode`:

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: debug-my-build
spec:
  debugMode: true  # Debug ALL actions in this job
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/example/zarf-package
      ref: main
```

#### Per-Action Debug Mode (Chained Workflows)

For chained actions (e.g., `BuildPublish`, `CreatePublishDeploy`), use `spec.debugActions` to debug only specific steps:

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: debug-build-only
spec:
  action: BuildPublish
  debugActions:
    - build  # Only debug the build step; publish runs normally after
  source:
    type: Git
    git:
      url: https://github.com/example/zarf-package
      ref: main
```

This is useful when you want to inspect the build environment without also pausing the publish step.

### Debug Mode Behavior

When debug mode is enabled for an action:

1. **Pod waits for completion marker**: Instead of running the actual command, the pod displays debug instructions and waits for a marker file
2. **Extended TTL**: Job cleanup is delayed (TTL set to 1 hour) to give time for inspection
3. **Enhanced logging**: Debug logs at V(4) include correlation IDs, timing, and detailed decision information

### Debug Workflow

```bash
# 1. Create job with debug mode
kubectl apply -f debug-job.yaml

# 2. Wait for pod to start
kubectl get pods -l forge.dev/package=debug-my-build -w

# 3. Exec into the pod to inspect environment
kubectl exec -it debug-my-build-build-xxxxx -- /bin/sh

# 4. Inside pod: inspect workspace and run commands manually
ls -la /workspace
cat /workspace/zarf.yaml
zarf package create . --confirm

# 5. Signal debug completion to continue (or finish)
touch /tmp/debug-complete

# The pod exits successfully. For chained workflows, the next action starts.
```

### Debug Mode Precedence

Debug mode follows this precedence (highest to lowest):

1. `spec.debugActions` - If specified, only listed actions are debugged
2. `spec.debugMode` - If true and `debugActions` is empty, all actions are debugged
3. `FORGE_DEBUG_MODE` env var - Global fallback for all jobs

### Viewing Debug Logs

Enable verbose logging on the controller/webhook to see detailed debug information:

```bash
# Run controller with debug verbosity
kubectl patch deployment forge-controller -n forge-system \
  --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "-v=4"}]'

# View webhook debug logs
kubectl logs -n forge-system -l app.kubernetes.io/component=webhook -f | grep -E 'correlationID|validation'

# View controller debug logs
kubectl logs -n forge-system -l app.kubernetes.io/component=controller -f | grep -E 'correlationID|handler|dispatch'

# Filter by specific job
kubectl logs -n forge-system -l app.kubernetes.io/component=controller -f | grep 'job="debug-my-build"'
```

## Extra Mounts

Forge allows you to mount ConfigMaps and Secrets into job containers using `extraMounts`. Mounts can be defined at the spec level (applied to all actions) or at the per-action level (applied only to that action). Per-action mounts are merged with spec-level mounts; if mount paths conflict, per-action mounts take precedence.

### Spec-Level Extra Mounts

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-with-mounts
spec:
  serviceAccountName: default
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/myorg/my-package
      ref: main
  extraMounts:
    - configMapRef:
        name: shared-config
      mountPath: /etc/shared-config
      readOnly: true
    - secretRef:
        name: signing-key
      mountPath: /etc/signing/key.pem
      subPath: key.pem
      readOnly: true
```

### Per-Action Extra Mounts

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-publish-with-mounts
spec:
  serviceAccountName: default
  action: BuildPublish
  source:
    type: Git
    git:
      url: https://github.com/myorg/my-package
      ref: main
  build:
    extraMounts:
      - configMapRef:
          name: build-config
        mountPath: /etc/build-config
  publish:
    extraMounts:
      - secretRef:
          name: publish-creds
        mountPath: /etc/publish-creds
        readOnly: true
```

### Mount Rules

- Exactly one of `configMapRef` or `secretRef` must be set per mount
- `mountPath` must be an absolute path
- Reserved paths cannot be used: `/workspace`, `/output`, `/artifacts`, `/tmp`, `/home/zarf`, `/home/uds`, and others used by Forge internally
- No duplicate mount paths within the same scope
- `readOnly` defaults to `true`

## Volume Sizes

Forge job pods use EmptyDir volumes for workspace, output, tmp, and home directories. You can customize the size limits using `volumeSizes` at the spec level.

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: large-build
spec:
  serviceAccountName: default
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/myorg/large-package
      ref: main
  volumeSizes:
    workspace: 20Gi  # default: 10Gi
    output: 15Gi     # default: 10Gi
    tmp: 2Gi         # default: 1Gi
    home: 2Gi        # default: 1Gi
```

All fields are optional. Unset fields use the defaults shown above.

## Pre-Tasks (UDS Only)

UDS bundle jobs support `preTasks` that run UDS runner tasks before the main action. Pre-tasks are available on `create` and `deploy` actions. Each pre-task executes `uds run -f <name>` with optional `--set KEY=VALUE` flags.

```yaml
apiVersion: forge.dev/v1alpha3
kind: UDSBundleJob
metadata:
  name: create-with-pretasks
spec:
  serviceAccountName: uds-bundle-operator
  action: Create
  source:
    type: Git
    git:
      url: https://github.com/myorg/my-bundle
      ref: main
  create:
    preTasks:
      - name: setup-env
        variables:
          REGISTRY: "ghcr.io/myorg"
      - name: pull-deps
```

Pre-tasks execute sequentially in order before the main action begins. Task names must not contain shell metacharacters (`;`, `|`, `&`, `$`, etc.) for security.

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
    kubectl get jobs -l forge.dev/package=my-bundle
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
    kubectl get jobs -l forge.dev/package=my-bundle
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
      forge.dev/allowed-actions: "Create,Deploy"  # ✅ create is allowed
    ```

3. Verify repository matches the allowed pattern:

    ```yaml
    annotations:
      forge.dev/allowed-source-repos: "https://github.com/myorg/*"  # ✅ pattern matches
      # or for UDS
      forge.dev/allowed-source-repos: "github.com/myorg/*"  # ✅ pattern matches
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
    - `access-key-id`
    - `secret-access-key`
    - Optional: `session-token` (for temporary credentials)

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

## kubectl-forge Reference

The kubectl-forge CLI plugin provides commands for managing and debugging Forge jobs.

### System Commands

#### `kubectl forge status`

Check the overall health of the Forge system.

```bash
# Check system status
kubectl forge status

# Output in JSON for scripting
kubectl forge status --output json
```

**Output includes**:
- Controller deployment status and pod health
- Webhook deployment status and TLS certificate expiry
- CRD installation status
- Jobs summary (pending, running, completed, failed, retrying)
- Warnings about stuck or problematic jobs

#### `kubectl forge logs controller|webhook`

Get logs from Forge system components.

```bash
# Get controller logs
kubectl forge logs controller

# Get webhook logs with error filtering
kubectl forge logs webhook --errors

# Follow logs in real-time
kubectl forge logs controller --follow

# Get logs from last 10 minutes
kubectl forge logs controller --since 10m

# Get logs from all replicas
kubectl forge logs webhook --all
```

### Job Management Commands

#### `kubectl forge list`

List all Forge jobs.

```bash
# List jobs in current namespace
kubectl forge list

# List jobs across all namespaces
kubectl forge list --all-namespaces

# Filter by job type
kubectl forge list --type zarf
kubectl forge list --type uds

# Watch for changes
kubectl forge list --watch
```

#### `kubectl forge cancel`

Cancel a running or pending job.

```bash
# Cancel a job
kubectl forge cancel my-package-build

# Cancel and delete the artifact PVC
kubectl forge cancel my-package-build --delete-pvc

# Skip confirmation
kubectl forge cancel my-package-build --force
```

#### `kubectl forge retry`

Retry a failed job by triggering a new execution.

```bash
# Retry a specific failed job
kubectl forge retry my-package-build

# Retry all failed jobs in the namespace
kubectl forge retry --all-failed

# Skip confirmation
kubectl forge retry my-package-build --force
```

### Diagnostic Commands

#### `kubectl forge diagnose`

Automatically diagnose problems with a job.

```bash
# Diagnose a job
kubectl forge diagnose my-package-build

# Show all events (not just warnings)
kubectl forge diagnose my-package-build --verbose

# Show more log lines
kubectl forge diagnose my-package-build --logs-lines 50

# Output for scripting
kubectl forge diagnose my-package-build --output json
```

**Checks performed**:
- Job status and phase
- Pod health (OOMKilled, CrashLoopBackOff, ImagePullBackOff, etc.)
- Warning events
- Container logs from failed pods
- Scheduling issues and resource constraints

**Example output**:

```text
Job: my-package-build (zarfpackagejob)
Namespace: default
Action: Build
Phase: Failed
Age: 5m

--- Problems Found ---
X Pod/my-package-build-abc123 Container/build: OOMKilled
  Container was killed due to out of memory

--- Warning Events ---
! [2m] OOMKilled: Container build exceeded memory limit

--- Container Logs ---
==> my-package-build-abc123/build <==
Building package...
Error: out of memory while compiling large component

--- Suggestions ---
* Increase memory limit in the job spec or reduce memory usage in the build
```

#### `kubectl forge get job`

Get detailed information about a job.

```bash
# Get job details
kubectl forge get job my-package-build

# Output in JSON or YAML
kubectl forge get job my-package-build --output json
kubectl forge get job my-package-build --output yaml
```

#### `kubectl forge get logs`

Get logs from a job's pods.

```bash
# Get logs
kubectl forge get logs my-package-build

# Follow logs in real-time
kubectl forge get logs my-package-build --follow

# Get logs from a specific container
kubectl forge get logs my-package-build --container zarf-build

# Get logs from all containers
kubectl forge get logs my-package-build --all-containers

# Get last 100 lines
kubectl forge get logs my-package-build --tail 100

# Get logs from last 5 minutes
kubectl forge get logs my-package-build --since 300

# Save logs to a file
kubectl forge get logs my-package-build --save build.log
```

#### `kubectl forge get pods`

List pods associated with a job.

```bash
# List pods
kubectl forge get pods my-package-build

# Show all pods (including completed)
kubectl forge get pods my-package-build --show-all

# Output in JSON or YAML
kubectl forge get pods my-package-build --output json
```

#### `kubectl forge get events`

Get Kubernetes events for a job.

```bash
# Get warning events
kubectl forge get events my-package-build

# Get all events (including Normal)
kubectl forge get events my-package-build --all

# Output in JSON or YAML
kubectl forge get events my-package-build --output json
```

### Artifact Commands

#### `kubectl forge download`

Download artifacts from a completed job.

```bash
# Download artifacts to current directory
kubectl forge download my-package-build

# Download to specific directory
kubectl forge download my-package-build --output-dir ./artifacts

# Download all files (not just final artifacts)
kubectl forge download my-package-build --all
```

### Debug Commands

#### `kubectl forge debug`

Debug a failed or running job by exec'ing into the pod.

```bash
# Debug a running job
kubectl forge debug my-package-build

# Debug a failed job (creates debug pod with workspace)
kubectl forge debug my-package-build --failed

# Use a specific shell
kubectl forge debug my-package-build --shell /bin/bash

# Use a custom debug image
kubectl forge debug my-package-build --debug-image ubuntu:22.04

# Keep the debug pod after exit
kubectl forge debug my-package-build --preserve-pod
```

## Additional Resources

### Documentation

- **Deployment Guide**: [DEPLOYMENT.md](DEPLOYMENT.md) - Helm installation and configuration
- **Developer Guide**: [KIND_SETUP.md](KIND_SETUP.md) - Local development with KIND
- **Namespace-Scoped Deployment**: [NAMESPACE_SCOPED_DEPLOYMENT.md](../operations/NAMESPACE_SCOPED_DEPLOYMENT.md)

### Examples

- **Zarf Policy Examples**: `examples/policies/zarf/` - ServiceAccount templates and RBAC configurations
- **UDS Policy Examples**: `examples/policies/uds/` - ServiceAccount templates and RBAC configurations
- **Sample Jobs**: `examples/samples/` - Ready-to-use job examples

### External Resources

- [Zarf Documentation](https://zarf.dev) - Official Zarf package documentation
- [UDS Documentation](https://uds.defenseunicorns.com) - UDS bundle documentation
