# Forge User Guide

## Introduction

Forge allows you to manage Zarf packages and UDS bundles using Kubernetes Custom Resources. This guide provides detailed instructions on how to use Forge to build, publish, and deploy your artifacts.

## Installation

Forge supports two deployment modes depending on your cluster permissions and security requirements.

### Option 1: Cluster-Wide Deployment (Recommended)

For platform teams managing multi-tenant environments with full cluster access:

```bash
# 1. Install Custom Resource Definitions (requires cluster-admin)
kubectl apply -f config/crd/zarf.dev_zarfpackages.yaml
kubectl apply -f config/crd/uds.io_udsbundles.yaml

# 2. Install the Forge Controller with ClusterRole
kubectl apply -f config/rbac/rbac.yaml
kubectl apply -f config/manager/deployment.yaml

# 3. Install the Admission Webhook (Required for policy enforcement)
kubectl apply -f webhook/deploy/
```

**Features**:

- Watches all namespaces
- ZarfPackages can be created in any namespace
- ServiceAccounts can be in any namespace
- Suitable for platform teams

### Option 2: Namespace-Scoped Deployment (Restricted)

For restricted environments where ClusterRole permissions aren't available:

```bash
# 1. Install CRDs (requires cluster-admin - one-time setup)
kubectl apply -f config/crd/zarf.dev_zarfpackages.yaml
kubectl apply -f config/crd/uds.io_udsbundles.yaml

# 2. Create namespace
kubectl create namespace forge-system

# 3. Install Forge with Role (namespace-only permissions)
kubectl apply -f config/namespace-scoped/rbac.yaml
kubectl apply -f config/namespace-scoped/deployment.yaml
```

**Features**:

- Watches only forge-system namespace
- All resources must be in forge-system
- Minimal permissions (Role, not ClusterRole)
- Suitable for restricted clusters, individual teams

**Important**: In namespace-scoped mode, all ZarfPackages, ServiceAccounts, and Secrets must be created in the `forge-system` namespace.

📖 **Detailed Guide**: See [NAMESPACE_SCOPED_DEPLOYMENT.md](./NAMESPACE_SCOPED_DEPLOYMENT.md) for complete instructions, migration paths, and multi-tenant patterns.

## Core Concepts

### ZarfPackage

The primary resource for defining operations on a single Zarf package.

### UDSBundle

The resource for defining operations on a UDS bundle (a collection of Zarf packages).

### Actions

- **Build**: Creates a Zarf package from source.
- **Publish**: Uploads a package to a registry (OCI or S3).
- **Deploy**: Installs a package into a cluster.
- **Composite Actions**: `BuildPublish`, `BuildDeploy`, `PublishDeploy`, `BuildPublishDeploy`.

## Examples

### 1. Build a Package from Git

Builds a package from a public Git repository.

```yaml
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackage
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
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackage
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
apiVersion: zarf.dev/v1alpha1
kind: ZarfPackage
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

### 4. UDS Bundle Deploy

Deploys a UDS bundle from an OCI registry.

```yaml
apiVersion: uds.io/v1alpha1
kind: UDSBundle
metadata:
  name: deploy-bundle
  namespace: default
spec:
  serviceAccountName: default
  action: Deploy
  source:
    type: OCI
    oci:
      image: ghcr.io/defenseunicorns/packages/uds/bundle:0.1.0
  deploy:
    target: InCluster
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
    forge.zarf.dev/allowed-actions: "Build,Publish"
    forge.zarf.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.zarf.dev/allowed-publish-registries: "ghcr.io/myorg/*"
```

**Note**: In namespace-scoped deployments, all ServiceAccounts must be created in the `forge-system` namespace.

### Usage

Reference this ServiceAccount in your `ZarfPackage`:

```yaml
spec:
  serviceAccountName: restricted-builder
  # ...
```

If the `ZarfPackage` tries to use a disallowed source or action, the controller will reject it.

## Troubleshooting

### Job Failures

Forge creates Kubernetes Jobs for each operation. If an operation fails:

1. Check the `ZarfPackage` status:

    ```bash
    kubectl get zarfpackage my-package -o yaml
    ```

2. Find the failed Job (named `<package-name>-<action>`):

    ```bash
    kubectl get jobs -l forge.zarf.dev/package=my-package
    ```

3. Check the Job logs:

    ```bash
    # Find the pod
    kubectl get pods -l job-name=<job-name>
    # Get logs
    kubectl logs <pod-name>
    ```

### Webhook Issues

If you cannot create `ZarfPackage` resources:

1. Check if the webhook pod is running:

    ```bash
    kubectl get pods -n forge-system
    ```

2. Check webhook logs for validation errors.
