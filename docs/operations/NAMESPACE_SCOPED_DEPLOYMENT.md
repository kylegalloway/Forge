# Namespace-Scoped Deployment Guide

Deploy Forge with namespace-scoped permissions instead of cluster-wide access.

## Overview

**Default deployment:** Forge uses ClusterRole/ClusterRoleBinding to watch all namespaces.

**Namespace-scoped deployment:** Forge uses Role/RoleBinding to watch only its own namespace. This is ideal for:

- Restricted clusters where ClusterRole permissions are not allowed
- Multi-tenant environments where each team gets their own Forge instance
- Security-conscious deployments requiring minimal permissions
- Development/testing environments

## Differences from Cluster-Wide Deployment

| Aspect | Cluster-Wide | Namespace-Scoped |
|--------|--------------|------------------|
| **RBAC** | ClusterRole | Role |
| **Scope** | All namespaces | Single namespace (forge-system) |
| **Permissions** | Cluster-wide | Namespace-only |
| **Use Case** | Platform teams | Individual teams |
| **ZarfPackageJobs** | Can be created anywhere | Only in forge-system |
| **ServiceAccounts** | Any namespace | Only forge-system |
| **Jobs** | Created in source namespace | Only in forge-system |

## Architecture

```text
┌─────────────────────────────────────────┐
│         forge-system namespace          │
├─────────────────────────────────────────┤
│ ┌────────────────────────────────────┐  │
│ │  Forge Controller                  │  │
│ │  - Watches: forge-system only      │  │
│ │  - Permissions: Role (not Cluster) │  │
│ └────────────────────────────────────┘  │
│                                          │
│ ┌────────────────────────────────────┐  │
│ │  ZarfPackageJobs (CRDs)               │  │
│ │  - Created in: forge-system        │  │
│ └────────────────────────────────────┘  │
│                                          │
│ ┌────────────────────────────────────┐  │
│ │  Jobs (build/publish/deploy)       │  │
│ │  - Run in: forge-system            │  │
│ └────────────────────────────────────┘  │
│                                          │
│ ┌────────────────────────────────────┐  │
│ │  ServiceAccounts (policies)        │  │
│ │  - Defined in: forge-system        │  │
│ └────────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

## Prerequisites

- Helm 3.8+
- kubectl configured
- Namespace creation permission (or pre-created namespace)

## Installation

Forge is deployed using Helm charts. The namespace-scoped mode is enabled via Helm values.

### Quick Install

```bash
helm upgrade --install forge ./chart/forge \
  --namespace forge-system \
  --create-namespace \
  --set controller.namespaceScope=true
```

**Note**: The above command:

- Creates CRDs automatically (included in Helm chart)
- Deploys controller with namespace-scoped RBAC (Role instead of ClusterRole)
- Controller watches only the forge-system namespace

### Custom Values File

Create a values file for namespace-scoped deployment:

```yaml
# values-namespace-scoped.yaml
controller:
  namespaceScope: true
  replicaCount: 1

networkPolicies:
  enabled: true
```

Install with custom values:

```bash
helm upgrade --install forge ./chart/forge \
  -f values-namespace-scoped.yaml \
  --namespace forge-system \
  --create-namespace
```

### Manual Steps (if needed)

**Verify Installation:**

```bash
# Check controller is running
kubectl get pods -n forge-system

# Check RBAC
kubectl get role,rolebinding -n forge-system

# Check logs
kubectl logs -n forge-system -l app=forge-controller
```

You should see:

```text
Watching namespace: forge-system
Leader election disabled - running as single instance
Starting Forge controller
```

## Usage

### Creating ZarfPackageJobs

All ZarfPackageJobs **must be created in the forge-system namespace**:

```yaml
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: my-package
  namespace: forge-system  # REQUIRED
spec:
  serviceAccountName: my-sa  # Must exist in forge-system
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/myorg/myrepo
      ref: main
```

### Creating ServiceAccounts

ServiceAccounts with policies must be in forge-system:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
  namespace: forge-system  # REQUIRED
  annotations:
    forge.dev/allowed-actions: "Build,Publish"
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
```

### Creating Secrets

Credential secrets must be in forge-system:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-token
  namespace: forge-system  # REQUIRED
type: Opaque
stringData:
  token: ghp_xxxxxxxxxxxx
```

## High Availability (Optional)

Enable HA in namespace-scoped mode:

**1. Update Deployment:**

```yaml
spec:
  replicas: 3  # Increase from 1
  template:
    spec:
      containers:
      - name: controller

        args:
        - -v=2
        - --namespace=forge-system
        - --enable-leader-election=true  # Add this

```

**2. Add PodDisruptionBudget:**

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: forge-controller-pdb
  namespace: forge-system
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: forge-controller
```

**3. Apply:**

```bash
kubectl apply -f config/namespace-scoped/deployment.yaml
```

Leader election will work namespace-scoped (Lease in forge-system).

## Multi-Tenant Deployment

Deploy separate Forge instances per team:

```bash
# Team A
kubectl create namespace team-a-forge
kubectl apply -f config/namespace-scoped/rbac.yaml -n team-a-forge
kubectl apply -f config/namespace-scoped/deployment.yaml -n team-a-forge

# Team B
kubectl create namespace team-b-forge
kubectl apply -f config/namespace-scoped/rbac.yaml -n team-b-forge
kubectl apply -f config/namespace-scoped/deployment.yaml -n team-b-forge
```

Each team gets isolated Forge instance with own policies.

## Monitoring

### Metrics

Port-forward to access metrics:

```bash
kubectl port-forward -n forge-system svc/forge-controller 8080:8080

# Access metrics
curl http://localhost:8080/metrics
```

### Logs

```bash
# Follow controller logs
kubectl logs -n forge-system -l app=forge-controller -f

# Get recent logs
kubectl logs -n forge-system -l app=forge-controller --tail=100
```

### Health Checks

```bash
# Health endpoint
kubectl port-forward -n forge-system svc/forge-controller 8081:8081
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

## Limitations

When running namespace-scoped, be aware of these constraints:

### ❌ Cannot Do

1. **Watch other namespaces:**
   - ZarfPackageJobs in other namespaces will be ignored
   - Controller only sees forge-system

2. **Access ServiceAccounts in other namespaces:**
   - All ServiceAccounts must be in forge-system
   - Cannot reference secrets from other namespaces

3. **Create jobs in other namespaces:**
   - All jobs run in forge-system
   - Network policies apply to forge-system only

### ✅ Can Do

1. **All ZarfPackageJob operations:**
   - Build, Publish, Deploy work normally
   - Just scoped to forge-system namespace

2. **Policy enforcement:**
   - ServiceAccount-based policies work
   - Admission webhook validates (if deployed)

3. **External resources:**
   - Can still access external Git/S3/OCI
   - Can deploy to external clusters

## Migration

### From Cluster-Wide to Namespace-Scoped

```bash
# 1. Delete cluster-wide deployment
kubectl delete clusterrolebinding forge-controller-rolebinding
kubectl delete clusterrole forge-controller-role
kubectl delete deployment forge-controller -n forge-system

# 2. Migrate ZarfPackageJobs to forge-system
kubectl get zarfpackagejobs -A -o yaml > all-packages.yaml
# Edit to change all namespaces to forge-system
kubectl apply -f all-packages.yaml

# 3. Migrate ServiceAccounts to forge-system
kubectl get sa -A -o yaml | grep "forge.dev/" > all-sa.yaml
# Edit to change namespaces to forge-system
kubectl apply -f all-sa.yaml

# 4. Install namespace-scoped
kubectl apply -f config/namespace-scoped/
```

### From Namespace-Scoped to Cluster-Wide

```bash
# 1. Delete namespace-scoped deployment
kubectl delete rolebinding forge-controller-rolebinding -n forge-system
kubectl delete role forge-controller-role -n forge-system
kubectl delete deployment forge-controller -n forge-system

# 2. Install cluster-wide
kubectl apply -f config/rbac/rbac.yaml
kubectl apply -f config/manager/deployment.yaml
```

## Security Considerations

### Advantages

✅ **Least privilege:** Only namespace-level permissions
✅ **Isolation:** Cannot affect other namespaces
✅ **Multi-tenancy:** Safe to run multiple instances
✅ **Audit:** Easier to audit single-namespace access

### Disadvantages

⚠️ **CRDs still cluster-wide:** Cannot install CRDs without cluster admin
⚠️ **All resources in one namespace:** Potential resource contention
⚠️ **Less flexible:** Users must use forge-system namespace

## Troubleshooting

### Controller not seeing ZarfPackageJobs

**Problem:** Created ZarfPackageJob but controller doesn't process it.

**Solution:** Ensure ZarfPackageJob is in forge-system:

```bash
kubectl get zarfpackagejobs -n forge-system
```

### Permission denied errors

**Problem:** Controller cannot create jobs.

**Solution:** Verify Role permissions:

```bash
kubectl get role forge-controller-role -n forge-system -o yaml
```

Ensure `jobs` verbs include `create, get, list, watch`.

### ServiceAccount not found

**Problem:** ZarfPackageJob references ServiceAccount in another namespace.

**Solution:** Move ServiceAccount to forge-system:

```bash
kubectl get sa my-sa -n other-namespace -o yaml | \
  sed 's/namespace: other-namespace/namespace: forge-system/' | \
  kubectl apply -f -
```

## Best Practices

### 1. Resource Quotas

Limit resources in forge-system:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: forge-quota
  namespace: forge-system
spec:
  hard:
    count/zarfpackagejobs.forge.dev: "100"
    count/jobs.batch: "50"
    requests.cpu: "10"
    requests.memory: "20Gi"
```

### 2. Network Policies

Apply to forge-system:

```bash
kubectl apply -f config/network/job-network-policy.yaml -n forge-system
```

### 3. Monitoring Labels

Label namespace for monitoring:

```bash
kubectl label namespace forge-system \
  monitoring=enabled \
  team=platform
```

### 4. Backup

Backup namespace resources:

```bash
kubectl get all,zarfpackagejobs,sa,secrets -n forge-system -o yaml > forge-backup.yaml
```

## Examples

See [examples/zarfpackagejobs/](../../examples/zarfpackagejobs/) for examples. All examples work in namespace-scoped mode when applied to forge-system namespace.

---

*Last Updated: 2025-11-20*
*Version: 1.0.0*
