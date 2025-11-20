# Forge User Guide

## Introduction

Forge allows you to manage Zarf packages and UDS bundles using Kubernetes Custom Resources. This guide provides detailed instructions on how to use Forge to build, publish, and deploy your artifacts.

## Installation

Ensure you have a Kubernetes cluster running.

```bash
# 1. Install Custom Resource Definitions
kubectl apply -f config/crd/zarf.dev_zarfpackages.yaml
kubectl apply -f config/crd/uds.io_udsbundles.yaml

# 2. Install the Forge Controller
kubectl apply -f config/manager/deployment.yaml
kubectl apply -f config/rbac/rbac.yaml

# 3. Install the Admission Webhook (Required for policy enforcement)
kubectl apply -f webhook/deploy/
```

## Core Concepts

### ZarfPackage
The primary resource for defining operations on a single Zarf package.

### UDSBundle
The resource for defining operations on a UDS bundle (a collection of Zarf packages).

### Actions
*   **Build**: Creates a Zarf package from source.
*   **Publish**: Uploads a package to a registry (OCI or S3).
*   **Deploy**: Installs a package into a cluster.
*   **Composite Actions**: `BuildPublish`, `BuildDeploy`, `PublishDeploy`, `BuildPublishDeploy`.

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

1.  Create a `ServiceAccount`.
2.  Annotate it with allowed actions and resources.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: restricted-builder
  namespace: default
  annotations:
    forge.zarf.dev/allowed-actions: "Build,Publish"
    forge.zarf.dev/allowed-source-repos: "github.com/myorg/*"
    forge.zarf.dev/allowed-publish-registries: "ghcr.io/myorg/*"
```

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

1.  Check the `ZarfPackage` status:
    ```bash
    kubectl get zarfpackage my-package -o yaml
    ```
2.  Find the failed Job (named `<package-name>-<action>`):
    ```bash
    kubectl get jobs -l forge.zarf.dev/package=my-package
    ```
3.  Check the Job logs:
    ```bash
    # Find the pod
    kubectl get pods -l job-name=<job-name>
    # Get logs
    kubectl logs <pod-name>
    ```

### Webhook Issues

If you cannot create `ZarfPackage` resources:

1.  Check if the webhook pod is running:
    ```bash
    kubectl get pods -n forge-system
    ```
2.  Check webhook logs for validation errors.
