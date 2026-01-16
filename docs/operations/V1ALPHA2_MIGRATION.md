# UDSBundleJob API Migration Guide: v1alpha1 to v1alpha2

This guide covers migrating UDSBundleJob resources from API version `v1alpha1` to `v1alpha2`.

## Overview

The `v1alpha2` API version for UDSBundleJob introduces several improvements:

- Better alignment with ZarfPackageJob API patterns
- Enhanced PVC configuration options (`useArtifactPVC`, `retainArtifactPVC`)
- Improved source and destination specifications

## Timeline

- **v1alpha1**: Deprecated, will be removed in Forge v0.10.0
- **v1alpha2**: Current stable API version

## Key Changes

### 1. API Version Update

```yaml
# Before (v1alpha1)
apiVersion: forge.dev/v1alpha1
kind: UDSBundleJob

# After (v1alpha2)
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
```

### 2. New PVC Configuration Fields

The `v1alpha2` API adds explicit control over artifact PVC behavior:

```yaml
apiVersion: forge.dev/v1alpha2
kind: UDSBundleJob
metadata:
  name: my-bundle
spec:
  serviceAccountName: bundle-builder
  action: Create

  # NEW: Control whether a PVC is created for artifact storage
  # Defaults to true - PVC is created for all create jobs
  useArtifactPVC: true

  # NEW: Control whether PVC is retained after job completion
  # Defaults to true - PVC is kept for debugging/reuse
  retainArtifactPVC: false

  source:
    type: Git
    git:
      url: https://github.com/example/bundle
      ref: main
```

### 3. Field Mappings

| v1alpha1 Field | v1alpha2 Field | Notes |
|----------------|----------------|-------|
| `apiVersion: forge.dev/v1alpha1` | `apiVersion: forge.dev/v1alpha2` | Required change |
| `spec.action` | `spec.action` | No change |
| `spec.source` | `spec.source` | No change |
| `spec.publish` | `spec.publish` | No change |
| `spec.deploy` | `spec.deploy` | No change |
| N/A | `spec.useArtifactPVC` | New field (default: true) |
| `spec.retainArtifactPVC` | `spec.retainArtifactPVC` | No change |

## Migration Steps

### Step 1: Identify v1alpha1 Resources

```bash
# List all v1alpha1 UDSBundleJobs
kubectl get udsbundlejobs.v1alpha1.forge.dev -A
```

### Step 2: Export and Convert

```bash
# Export a resource
kubectl get udsbundlejob my-bundle -n default -o yaml > my-bundle.yaml

# Update the apiVersion
sed -i 's|apiVersion: forge.dev/v1alpha1|apiVersion: forge.dev/v1alpha2|g' my-bundle.yaml
```

### Step 3: Apply Updated Resource

```bash
# Delete old resource
kubectl delete udsbundlejob my-bundle -n default

# Apply new version
kubectl apply -f my-bundle.yaml
```

## Automated Conversion

For bulk migration, use this script:

```bash
#!/bin/bash
# migrate-udsbundlejobs.sh

NAMESPACE=${1:-default}

for job in $(kubectl get udsbundlejobs.v1alpha1.forge.dev -n "$NAMESPACE" -o name 2>/dev/null); do
  name=$(basename "$job")
  echo "Migrating $name..."

  # Export, convert, and re-apply
  kubectl get "$job" -n "$NAMESPACE" -o yaml | \
    sed 's|apiVersion: forge.dev/v1alpha1|apiVersion: forge.dev/v1alpha2|g' | \
    kubectl apply -f -

  # Delete old version
  kubectl delete "$job" -n "$NAMESPACE"
done
```

## Validation

After migration, verify resources:

```bash
# Check v1alpha2 resources exist
kubectl get udsbundlejobs.v1alpha2.forge.dev -A

# Verify no v1alpha1 resources remain
kubectl get udsbundlejobs.v1alpha1.forge.dev -A
# Should return: "No resources found"
```

## Rollback

If issues occur, you can temporarily revert:

```bash
# Convert back to v1alpha1 (not recommended for production)
sed -i 's|apiVersion: forge.dev/v1alpha2|apiVersion: forge.dev/v1alpha1|g' my-bundle.yaml
kubectl apply -f my-bundle.yaml
```

## FAQ

### Q: Do I need to migrate ZarfPackageJob resources?

No. ZarfPackageJob has always been at `v1alpha1` and remains stable. Only UDSBundleJob has a v1alpha2 migration.

### Q: What happens if I don't migrate?

The v1alpha1 API will be removed in Forge v0.10.0. After that release, v1alpha1 resources will not be accepted.

### Q: Are there breaking changes in behavior?

No. The v1alpha2 API is backward compatible. New fields (`useArtifactPVC`) have sensible defaults that match previous behavior.

## Related Documentation

- [User Guide](../getting-started/USER_GUIDE.md) - Complete usage examples
- [UDS Troubleshooting](UDS_TROUBLESHOOTING.md) - Common issues and solutions
