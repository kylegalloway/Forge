# Git Build and Deploy Example

This example demonstrates building a Zarf package from a public Git repository and deploying it directly to your Kubernetes cluster.

## What It Does

1. **Fetches** the Zarf package definition from this Git repository
2. **Builds** the Zarf package (downloads Headlamp Helm chart and container images)
3. **Deploys** the package to the `headlamp` namespace in your cluster

## What Gets Deployed

[Headlamp](https://headlamp.dev/) - A user-friendly Kubernetes UI that provides a simple way to view and manage your cluster resources.

## How to Use

### Prerequisites

- Forge controller running in your cluster (`make kind-setup`)
- kubectl configured to access your cluster

### Deploy the Example

```bash
kubectl apply -f examples/samples/zarf/03-git-build-deploy/zarfpackagejob.yaml
```

### Monitor Progress

```bash
# Watch the ZarfPackageJob
kubectl get zarfpackagejob git-build-deploy-example -w

# View Job logs
kubectl logs -l forge.dev/package=git-build-deploy-example -f

# Check deployed resources
kubectl get pods -n headlamp
```

### Access Headlamp

```bash
# Port forward to access the UI
kubectl port-forward -n headlamp svc/headlamp 8080:80

# Open http://localhost:8080 in your browser
```

### Cleanup

```bash
# Remove the ZarfPackageJob
kubectl delete zarfpackagejob git-build-deploy-example

# Remove deployed resources
kubectl delete namespace headlamp
```

## Files in This Example

- `zarf.yaml` - Zarf package definition that deploys Headlamp
- `values.yaml` - Helm values for Headlamp configuration
- `zarfpackagejob.yaml` - Forge CRD resource that orchestrates the build and deploy

## Key Features Demonstrated

- **Public Git Source**: No credentials required, works with any public repository
- **BuildDeploy Action**: Single operation that builds and deploys in one step
- **RBAC Policy**: ServiceAccount annotations control allowed actions and namespaces
- **External Helm Chart**: Zarf downloads the chart from the official Headlamp repository
- **Container Images**: Automatically pulled and included in the package
