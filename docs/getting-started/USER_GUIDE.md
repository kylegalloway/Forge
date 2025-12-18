# Forge User Guide

## Introduction

Forge allows you to manage Zarf packages using Kubernetes Custom Resources. This guide provides detailed instructions on how to use Forge to build, publish, and deploy your artifacts.

> **For Developers**: If you're contributing to Forge and need to test local changes, see [KIND_SETUP.md](KIND_SETUP.md) for the developer workflow

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
  --version 0.1.0
```

**Container Images Used**:

- Controller: `ghcr.io/kylegalloway/forge/forge-controller:v0.1.0`
- Webhook: `ghcr.io/kylegalloway/forge/forge-webhook:v0.1.0`

### Installation Options

**Default Installation** (minimal, production-ready):

```bash
helm install forge forge/forge \
  --version 0.1.0 \
  --namespace forge-system \
  --create-namespace
```

**Mature Cluster** (existing Prometheus/Grafana):

```bash
helm install forge forge/forge \
  --version 0.1.0 \
  --values https://raw.githubusercontent.com/kylegalloway/Forge/main/chart/forge/values-mature-cluster.yaml \
  --namespace forge-system \
  --create-namespace
```

**New Cluster** (includes full observability stack):

```bash
helm install forge forge/forge \
  --version 0.1.0 \
  --values https://raw.githubusercontent.com/kylegalloway/Forge/main/chart/forge/values-new-cluster.yaml \
  --namespace forge-system \
  --create-namespace
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

For additional deployment options and configurations, see [DEPLOYMENT.md](../../DEPLOYMENT.md).

### Deployment Modes

Forge supports two deployment modes depending on your cluster permissions and security requirements.

**Cluster-Wide Deployment (Default)**:

- Watches all namespaces
- ZarfPackageJobs can be created in any namespace
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

### Actions

- **Build**: Creates a Zarf package from source.
- **Publish**: Uploads a package to a registry (OCI or S3).
- **Deploy**: Installs a package into a cluster.
- **Composite Actions**: `BuildPublish`, `BuildDeploy`, `PublishDeploy`, `BuildPublishDeploy`.

## Examples

### 1. Build a Package from Git

Builds a package from a public Git repository.

```yaml
apiVersion: forge.dev/v1alpha1
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
      url: https://github.com/defenseunicorns/zarf-public-test.git
      ref: main
      path: packages/dos-games
```

### 2. Build and Publish to OCI

Builds a package and immediately publishes it to an OCI registry.

```yaml
apiVersion: forge.dev/v1alpha1
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
      url: https://github.com/defenseunicorns/zarf-public-test.git
      ref: main
      path: packages/dos-games
  publish:
    destination:
      type: OCI
      oci:
        registry: ghcr.io
        repository: myuser/dos-games
        tag: 1.0.0
        credentialsSecretRef:
          name: oci-creds # Secret containing .dockerconfigjson
```

### 3. Deploy from S3

Deploys a package stored in an S3 bucket.

```yaml
apiVersion: forge.dev/v1alpha1
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
      credentialsSecretRef:
        name: aws-creds # Secret containing AWS_ACCESS_KEY_ID/SECRET
  deploy:
    target: InCluster
    namespace: games
```

## Policy Enforcement

Forge uses `ServiceAccount` annotations to enforce policies.

### Setup

1. Create a `ServiceAccount`.
2. Annotate it with allowed actions and resources.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: restricted-builder
  namespace: default  # cluster-wide mode
  # namespace: forge-system  # namespace-scoped mode (all SAs must be here)
  annotations:
    forge.forge.dev/allowed-actions: "Build,Publish"
    forge.forge.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"
```

**Note**: In namespace-scoped deployments, all ServiceAccounts must be created in the `forge-system` namespace.

### Usage

Reference this ServiceAccount in your `ZarfPackageJob`:

```yaml
spec:
  serviceAccountName: restricted-builder
  # ...
```

If the `ZarfPackageJob` tries to use a disallowed source or action, the controller will reject it.

## Troubleshooting

### Job Failures

Forge creates Kubernetes Jobs for each operation. If an operation fails:

1. Check the `ZarfPackageJob` status:

    ```bash
    kubectl get ZarfPackageJob my-package -o yaml
    ```

2. Find the failed Job (named `<package-name>-<action>`):

    ```bash
    kubectl get jobs -l forge.forge.dev/package=my-package
    ```

3. Check the Job logs:

    ```bash
    # Find the pod
    kubectl get pods -l job-name=<job-name>
    # Get logs
    kubectl logs <pod-name>
    ```

### Webhook Issues

If you cannot create `ZarfPackageJob` resources:

1. Check if the webhook pod is running:

    ```bash
    kubectl get pods -n forge-system
    ```

2. Check webhook logs for validation errors.
