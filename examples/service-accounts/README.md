# ServiceAccount Policy Examples

This directory contains example ServiceAccounts demonstrating Forge's policy enforcement through Kubernetes RBAC and annotations.

## Overview

Forge uses ServiceAccount annotations to enforce fine-grained access control for package operations. The admission webhook validates all ZarfPackageJob and UDSBundleJob resources against the policies defined in the ServiceAccount annotations before allowing creation.

## Available Examples

### simple-test-sa.yaml

**Use Case:** Quick testing in Kind or development environments

A permissive ServiceAccount configured for local development and testing:

- **Namespace:** default
- **Actions:** Build, Publish
- **Repositories:** forge repo and defenseunicorns/zarf-public-test
- **Registries:** ghcr.io/* (wildcard for testing)
- **Deploy Targets:** InCluster
- **Special:** Allows local sources (non-production feature)

**Quick Start:**
```bash
kubectl apply -f simple-test-sa.yaml
kubectl apply -f ../samples/zarf/01-git-to-oci/zarfpackagejob.yaml
```

### service-account-example.yaml

**Use Case:** Production-like multi-team configuration

Three ServiceAccounts demonstrating different policy levels for different teams/environments:

#### 1. Developer ServiceAccount (default namespace)

- **Actions:** Build, Publish
- **Repositories:** Organization repos + public Zarf repo
- **Registries:** Organization-specific OCI registries
- **S3 Buckets:** Artifact buckets with prefix matching
- **Deploy Targets:** InCluster only

#### 2. Production ServiceAccount (production namespace)

- **Actions:** Deploy only (no building in production)
- **Source:** Pre-approved production registries only
- **Deploy Targets:** ExternalCluster only (not in-cluster)
- **Namespaces:** Restricted deployment namespace list

#### 3. Platform Team ServiceAccount (platform-team namespace)

- **Actions:** All actions (Build, Publish, Deploy)
- **Repositories:** All organization repositories
- **Registries:** Full control over organization registries
- **Deploy Targets:** Both InCluster and ExternalCluster

**Prerequisites:**
```bash
# Create namespaces first
kubectl create namespace production
kubectl create namespace platform-team

# Apply ServiceAccounts
kubectl apply -f service-account-example.yaml
```

## Policy Annotation Reference

### Common Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `forge.dev/allowed-actions` | Comma-separated list of allowed actions | `Build,Publish,Deploy` |
| `forge.dev/allowed-source-repos` | Git repositories allowed as sources (glob patterns) | `https://github.com/myorg/*` |
| `forge.dev/allowed-source-registries` | OCI registries allowed as sources | `ghcr.io/myorg/*` |
| `forge.dev/allowed-source-buckets` | S3 buckets allowed as sources | `my-packages-*` |
| `forge.dev/allowed-publish-registries` | OCI registries for publishing | `ghcr.io/myorg/prod/*` |
| `forge.dev/allowed-publish-buckets` | S3 buckets for publishing | `artifacts-prod-*` |
| `forge.dev/allowed-deploy-targets` | Deployment targets | `InCluster,ExternalCluster` |
| `forge.dev/allowed-deploy-namespaces` | Kubernetes namespaces for deployment | `production,staging` |

### Development-Only Annotations

| Annotation | Description | Production Use |
|------------|-------------|----------------|
| `forge.dev/allow-local-sources` | Allow local filesystem sources | ❌ Never in production |

## Glob Pattern Support

Many annotations support glob patterns for flexible matching:

```yaml
# Match all repos in an organization
forge.dev/allowed-source-repos: "https://github.com/myorg/*"

# Match specific registry paths
forge.dev/allowed-publish-registries: "ghcr.io/myorg/dev/*,ghcr.io/myorg/staging/*"

# Match bucket prefixes
forge.dev/allowed-publish-buckets: "artifacts-*,packages-dev-*"
```

## UDS Bundle Annotations

For UDS bundle operations, use the `allowed-bundle-actions` annotation:

```yaml
annotations:
  # UDS-specific actions
  forge.dev/allowed-bundle-actions: "Create,Publish,Deploy"

  # Source and destination annotations work the same
  forge.dev/allowed-source-repos: "https://github.com/myorg/bundles/*"
  forge.dev/allowed-publish-registries: "ghcr.io/myorg/bundles/*"
```

## Testing Policy Enforcement

### Test Allowed Operation

```bash
# Should succeed
kubectl apply -f simple-test-sa.yaml
kubectl apply -f ../samples/zarf/01-git-to-oci/zarfpackagejob.yaml
```

### Test Policy Violation

```bash
# Create restrictive ServiceAccount
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: restricted-sa
  namespace: default
  annotations:
    forge.dev/allowed-actions: "Build"
    forge.dev/allowed-source-repos: "https://github.com/approved-only/*"
EOF

# Try to use unapproved repo (should be rejected by webhook)
cat <<EOF | kubectl apply -f -
apiVersion: forge.dev/v1alpha1
kind: ZarfPackageJob
metadata:
  name: policy-violation-test
  namespace: default
spec:
  serviceAccountName: restricted-sa
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/unapproved/repo  # Not in allowed repos
      ref: main
EOF
```

Expected result: Webhook rejects with policy violation message

## Deployment Modes

Forge supports two RBAC deployment modes. See [docs/operations/NAMESPACE_SCOPED_DEPLOYMENT.md](../../docs/operations/NAMESPACE_SCOPED_DEPLOYMENT.md) for details.

### Cluster-Wide Mode (default)

- ZarfPackageJobs can be created in any namespace
- ServiceAccounts can be in any namespace
- More flexible, requires broader cluster permissions

### Namespace-Scoped Mode

- All ZarfPackageJobs must be in `forge-system` namespace
- All ServiceAccounts must be in `forge-system` namespace
- More restrictive, better for security-focused deployments

## Documentation

For complete policy configuration details, see:

- **[ServiceAccount Reference](../../docs/development/SERVICEACCOUNT_REFERENCE.md)** - Complete annotation reference
- **[User Guide](../../docs/getting-started/USER_GUIDE.md)** - End-to-end workflows
- **[Namespace-Scoped Deployment](../../docs/operations/NAMESPACE_SCOPED_DEPLOYMENT.md)** - Deployment mode comparison

## Best Practices

1. **Principle of Least Privilege**: Grant only the minimum permissions needed
2. **Use Glob Patterns Carefully**: Overly broad patterns (`*`) reduce security
3. **Separate Environments**: Different ServiceAccounts for dev/staging/prod
4. **Never Use Local Sources in Production**: Only for Kind/dev testing
5. **Test Policy Before Deploying**: Verify policies work as expected in dev first
6. **Document Team Policies**: Clearly communicate what each SA can do
