# Migrating from v1alpha1 to v1alpha2 UDS API

## Overview

The v1alpha2 API unifies UDS naming conventions with Zarf for consistency across both package types. This guide helps you migrate your existing UDSBundleJob resources to the new UDSPackageJob API.

## What Changed?

### API Version

```yaml
# v1alpha1
apiVersion: forge.dev/v1alpha1
kind: UDSBundleJob

# v1alpha2
apiVersion: forge.dev/v1alpha2
kind: UDSPackageJob
```

### Type Names

| v1alpha1 | v1alpha2 | Reason |
|----------|----------|--------|
| `UDSBundleJob` | `UDSPackageJob` | Parallels `ZarfPackageJob` |
| `BundleAction` | `Action` | Matches Zarf action type |
| `BundleSourceType` | `SourceType` | Consistent naming |
| `BundleDestinationType` | `DestinationType` | Consistent naming |
| `BundleDeployTargetType` | `DeployTargetType` | Consistent naming |

### Action Constants

```yaml
# v1alpha1
action: BundleActionCreate
action: BundleActionPublish
action: BundleActionDeploy
action: BundleActionCreatePublish
action: BundleActionCreateDeploy

# v1alpha2 - Simplified
action: Create
action: Publish
action: Deploy
action: CreatePublish
action: CreateDeploy
```

### ServiceAccount Annotations

v1alpha2 uses the same annotation keys as Zarf (no "bundle" prefix):

```yaml
# v1alpha1
annotations:
  forge.dev/allowed-bundle-actions: "Create,Deploy"
  forge.dev/allowed-bundle-source-repos: "https://github.com/*"
  forge.dev/allowed-bundle-deploy-namespaces: "default"

# v1alpha2 - Unified with Zarf
annotations:
  forge.dev/allowed-actions: "Create,Deploy"
  forge.dev/allowed-source-repos: "https://github.com/*"
  forge.dev/allowed-deploy-namespaces: "default"
```

### Field Names

Most field names remain the same, but some struct names changed:

```yaml
# v1alpha1
spec:
  source:
    type: Git  # BundleSourceType
  publish:
    destination:
      type: S3  # BundleDestinationType
  deploy:
    target: InCluster  # BundleDeployTargetType

# v1alpha2 - Same field names, different type names
spec:
  source:
    type: Git  # SourceType
  publish:
    destination:
      type: S3  # DestinationType
  deploy:
    target: InCluster  # DeployTargetType
```

## Migration Timeline

- **v1alpha1**: Supported until Forge v0.10.0 (estimated 6 months)
- **v1alpha1**: Deprecated as of Forge v0.5.0
- **v1alpha2**: Recommended for all new workloads
- **Conversion Webhook**: Automatic conversion between versions (future)

## Migration Steps

### 1. Review Your Current Resources

```bash
# List all v1alpha1 UDSBundleJob resources
kubectl get udsbundlejobs.v1alpha1.forge.dev --all-namespaces
```

### 2. Create v1alpha2 Versions

For each UDSBundleJob, create a v1alpha2 equivalent:

**Before (v1alpha1):**
```yaml
apiVersion: forge.dev/v1alpha1
kind: UDSBundleJob
metadata:
  name: my-bundle
spec:
  serviceAccountName: my-sa
  action: CreateDeploy
  source:
    type: Git
    git:
      url: https://github.com/example/repo
```

**After (v1alpha2):**
```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSPackageJob
metadata:
  name: my-bundle-v2
spec:
  serviceAccountName: my-sa-v2
  action: CreateDeploy
  source:
    type: Git
    git:
      url: https://github.com/example/repo
```

### 3. Update ServiceAccounts

Update annotation keys to remove "bundle" prefix:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa-v2
  annotations:
    # Remove "bundle" from annotation keys
    forge.dev/allowed-actions: "Create,Deploy"  # was allowed-bundle-actions
    forge.dev/allowed-source-repos: "https://github.com/*"  # was allowed-bundle-source-repos
```

### 4. Test in Non-Production

```bash
# Apply v1alpha2 resources to test cluster
kubectl apply -f my-bundle-v2.yaml

# Verify it works
kubectl get udspackagejobs my-bundle-v2 -o yaml
kubectl describe udspackagejob my-bundle-v2
```

### 5. Migrate Production

Once tested:
1. Apply v1alpha2 resources to production
2. Monitor for any issues
3. Delete old v1alpha1 resources after verification

```bash
# Apply new version
kubectl apply -f my-bundle-v2.yaml

# Verify it's running
kubectl get udspackagejob my-bundle-v2

# Delete old version (after verification)
kubectl delete udsbundlejob my-bundle
```

## Examples

See `examples/samples/uds/` for v1alpha2 examples:
- `03-git-build-deploy/udspackagejob-v1alpha2.yaml` - Create and deploy from Git

## Automated Migration (Future)

Future versions of Forge will include:
- **Conversion Webhook**: Automatic conversion between v1alpha1 and v1alpha2
- **Migration Tool**: `kubectl convert` support for batch migrations

## Troubleshooting

### "Invalid API version" error

**Cause**: Forge CRD doesn't include v1alpha2 schema

**Fix**: Upgrade Forge Helm chart to v0.5.0+
```bash
helm upgrade forge chart/forge
```

### ServiceAccount permissions denied

**Cause**: Using old annotation keys with v1alpha2

**Fix**: Update ServiceAccount to use new annotation keys (without "bundle" prefix)

### Can't find UDSPackageJob resources

**Cause**: Using wrong kubectl command

**Fix**: Use the correct shortname
```bash
# v1alpha1
kubectl get ubj  # UDSBundleJob

# v1alpha2
kubectl get upj  # UDSPackageJob
```

## Benefits of v1alpha2

1. **Consistency**: Same naming as Zarf makes learning curve easier
2. **Simplicity**: Shorter action names (`Create` vs `BundleActionCreate`)
3. **Unified Annotations**: Same ServiceAccount annotations for both package types
4. **Future-Proof**: Foundation for merging Zarf and UDS into unified package system

## Getting Help

- Documentation: `docs/getting-started/USER_GUIDE.md`
- Examples: `examples/samples/uds/`
- Issues: https://github.com/kylegalloway/forge/issues

## Comparison Table

| Feature | v1alpha1 | v1alpha2 |
|---------|----------|----------|
| Kind | UDSBundleJob | UDSPackageJob |
| Action Type | BundleAction | Action |
| Action Values | BundleActionCreate | Create |
| Shortname | ubj | upj |
| Annotations | forge.dev/allowed-bundle-* | forge.dev/allowed-* |
| Status | Deprecated | Recommended |
| Support Until | v0.10.0 | Current + future |
