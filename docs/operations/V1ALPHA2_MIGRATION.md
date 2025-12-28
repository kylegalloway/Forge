# v1alpha1 UDS API Removed - v1alpha2 is the Only Supported Version

## Overview

The v1alpha1 `UDSBundleJob` API has been **completely removed** from Forge. Only the v1alpha2 `UDSPackageJob` API is supported. This change unifies UDS naming conventions with Zarf for consistency across both package types.

**If you were using v1alpha1**: You need to update your resources to v1alpha2. There is no backward compatibility or conversion webhook - v1alpha1 is gone.

## What Changed?

### API Version and Kind

```yaml
# v1alpha1 (REMOVED)
apiVersion: forge.dev/v1alpha1
kind: UDSBundleJob

# v1alpha2 (ONLY VERSION)
apiVersion: forge.dev/v1alpha2
kind: UDSPackageJob
```

### Resource Names

| v1alpha1 (Removed) | v1alpha2 (Current) |
|--------------------|-------------------|
| `udsbundlejobs` | `udspackagejobs` |
| Short name: `ubj` | Short name: `upj` |

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
# v1alpha1 (removed)
action: BundleActionCreate
action: BundleActionPublish
action: BundleActionDeploy

# v1alpha2 (simplified)
action: Create
action: Publish
action: Deploy
action: CreatePublish
action: CreateDeploy
```

### ServiceAccount Annotations

v1alpha2 uses the same annotation keys as Zarf (no "bundle" prefix):

```yaml
# v1alpha1 (removed)
annotations:
  forge.dev/allowed-bundle-actions: "Create,Deploy"
  forge.dev/allowed-bundle-source-repos: "https://github.com/*"

# v1alpha2 (current)
annotations:
  forge.dev/allowed-actions: "Create,Deploy"
  forge.dev/allowed-source-repos: "https://github.com/*"
```

## Migration Timeline

- **v0.5.0**: v1alpha1 marked as deprecated
- **v0.6.0**: v1alpha1 completely removed
- **Current**: Only v1alpha2 is supported

## How to Update Your Resources

### 1. Check for v1alpha1 Resources

```bash
# This will fail if v1alpha1 is no longer in the cluster
kubectl get udsbundlejobs.v1alpha1.forge.dev --all-namespaces

# Use v1alpha2 instead
kubectl get udspackagejobs --all-namespaces
```

### 2. Update Resource Manifests

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
  name: my-bundle
spec:
  serviceAccountName: my-sa-v2
  action: CreateDeploy
  source:
    type: Git
    git:
      url: https://github.com/example/repo
```

### 3. Update ServiceAccounts

Remove "bundle" prefix from annotation keys:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa-v2
  annotations:
    # Remove "bundle" from annotation keys
    forge.dev/allowed-actions: "Create,Deploy"
    forge.dev/allowed-source-repos: "https://github.com/*"
    forge.dev/allowed-deploy-namespaces: "default"
```

### 4. Update kubectl Commands

```bash
# v1alpha1 (no longer works)
kubectl get ubj
kubectl get udsbundlejobs

# v1alpha2 (current)
kubectl get upj
kubectl get udspackagejobs
```

## Examples

See `examples/samples/uds/` for v1alpha2 examples:
- `01-git-to-oci/` - Create bundle from Git and publish to OCI
- `02-local-to-s3/` - Create from local source and publish to S3
- `03-git-build-deploy/` - Complete Git → Create → Deploy workflow

## Common Issues

### "no matches for kind UDSBundleJob"

**Cause**: Trying to use removed v1alpha1 API

**Fix**: Update to v1alpha2:
```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSPackageJob
```

### "udsbundlejobs" resource not found

**Cause**: Resource name changed

**Fix**: Use `udspackagejobs` instead:
```bash
kubectl get udspackagejobs
```

### ServiceAccount permission denied

**Cause**: Using old annotation keys with "bundle" prefix

**Fix**: Update ServiceAccount annotations to remove "bundle":
```yaml
# Old (doesn't work with v1alpha2)
forge.dev/allowed-bundle-actions: "Create"

# New (correct)
forge.dev/allowed-actions: "Create"
```

## Benefits of v1alpha2

1. **Consistency**: Same naming as Zarf makes learning curve easier
2. **Simplicity**: Shorter action names (`Create` vs `BundleActionCreate`)
3. **Unified Annotations**: Same ServiceAccount annotations for both package types
4. **Cleaner CRDs**: No need to support multiple versions

## Comparison Table

| Feature | v1alpha1 (Removed) | v1alpha2 (Current) |
|---------|-------------------|-------------------|
| Kind | UDSBundleJob | UDSPackageJob |
| Resource | udsbundlejobs | udspackagejobs |
| Action Type | BundleAction | Action |
| Action Values | BundleActionCreate | Create |
| Shortname | ubj | upj |
| Annotations | forge.dev/allowed-bundle-* | forge.dev/allowed-* |
| Status | ❌ Removed | ✅ Supported |

## Getting Help

- Documentation: `docs/getting-started/UDS_GUIDE.md`
- Examples: `examples/samples/uds/`
- Issues: https://github.com/kylegalloway/forge/issues
