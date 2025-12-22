# UDS Git Build and Deploy Example

This example demonstrates creating a UDS bundle from a public Git repository and deploying it directly to your Kubernetes cluster.

## What It Does

1. **Fetches** the UDS bundle definition from this Git repository
2. **Creates** the UDS bundle (builds the Zarf package, downloads Headlamp Helm chart and images)
3. **Deploys** the bundle to the `headlamp` namespace in your cluster

## What Gets Deployed

[Headlamp](https://headlamp.dev/) - A user-friendly Kubernetes UI packaged as a UDS bundle.

## How to Use

### Prerequisites

- Forge controller running in your cluster (`make kind-setup`)
- kubectl configured to access your cluster

### Deploy the Example

```bash
kubectl apply -f examples/samples/uds/03-git-build-deploy/udsbundlejob.yaml
```

### Monitor Progress

```bash
# Watch the UDSBundleJob
kubectl get udsbundlejob git-build-deploy-bundle-example -w

# View Job logs
kubectl logs -l forge.dev/bundle=git-build-deploy-bundle-example -f

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
# Remove the UDSBundleJob
kubectl delete udsbundlejob git-build-deploy-bundle-example

# Remove deployed resources
kubectl delete namespace headlamp
```

## Files in This Example

- `uds-bundle.yaml` - UDS bundle definition
- `zarf-package/zarf.yaml` - Zarf package definition for Headlamp
- `zarf-package/values.yaml` - Helm values for Headlamp configuration
- `udsbundlejob.yaml` - Forge CRD resource that orchestrates the create and deploy

## Key Features Demonstrated

- **Public Git Source**: No credentials required, works with any public repository
- **CreateDeploy Action**: Single operation that creates the bundle and deploys in one step
- **RBAC Policy**: ServiceAccount annotations control allowed actions and namespaces
- **Bundle Structure**: UDS bundle containing a Zarf package
- **External Helm Chart**: Zarf downloads the chart from the official Headlamp repository
- **Container Images**: Automatically pulled and included in the package
