# CRD Naming Issue and Refactoring Plan

## Problem Statement

The current CRD naming is **confusing and misleading**:

### Current Names (WRONG)

- `ZarfPackageJob` - Sounds like a Zarf package definition
- `UDSBundle` - Sounds like a UDS bundle definition

### Why This Is Bad

1. **Namespace Pollution**: Conflicts with actual Zarf/UDS terminology
2. **User Confusion**: Users might think they're defining Zarf packages, when they're actually defining **Forge job specifications**
3. **Conceptual Error**: These CRDs describe what Forge should DO with packages, not the packages themselves

### Actual Zarf/UDS Resources

- Zarf packages are defined in `zarf.yaml` files
- UDS bundles are defined in `uds-bundle.yaml` files containing lists of packages
- Our CRDs should clearly be **job specs for Forge operations**, not package definitions

## Recommended Solution

### New Names (CORRECT)

- `ZarfPackageJob` - Clearly a Forge job spec for Zarf operations
- `UDSBundleJob` - Clearly a Forge job spec for UDS operations (stub/incomplete)

### Benefits

1. **Clear Intent**: Obvious these are job specifications
2. **No Namespace Collision**: Can't be confused with actual Zarf/UDS resources
3. **Follows Kubernetes Patterns**: Like `CronJob`, `Job`, etc.

## Current Status (PARTIAL)

### ✅ Completed

- `pkg/apis/zarf/v1alpha1/types.go` - Renamed to `ZarfPackageJob`
- `pkg/apis/zarf/v1alpha1/register.go` - Updated
- `pkg/apis/zarf/v1alpha1/zz_generated.deepcopy.go` - Updated
- Shortname changed: `zp` → `zpj`

### ❌ TODO (Remaining Work)

1. **Controller Files**
   - `pkg/controller/controller.go` - 50+ references
   - `pkg/controller/job_monitor.go` - 20+ references

2. **Action Handlers**
   - `pkg/actions/build.go`
   - `pkg/actions/publish.go`
   - `pkg/actions/deploy.go`

3. **Policy Engine**
   - `pkg/policy/engine.go`

4. **CRD YAML Manifests**
   - `config/crd/forge.dev_zarfpackagejobs.yaml` → `forge.dev_zarfpackagejobs.yaml`
   - Resource name: `zarfpackagejobs` → `zarfpackagejobs`
   - Kind: `ZarfPackageJob` → `ZarfPackageJob`

5. **Sample Files** (all in `config/samples/v1alpha1/`)
   - `build-only-git.yaml`
   - `build-publish-deploy-git.yaml`
   - `publish-deploy-s3.yaml`
   - etc.

6. **Documentation**
   - `README.md` - All examples and explanations
   - `docs/USER_GUIDE.md`
   - `docs/SERVICEACCOUNT_REFERENCE.md`
   - `docs/NAMESPACE_SCOPED_DEPLOYMENT.md`
   - All other docs

7. **Webhook**
   - `webhook/pkg/webhook/webhook.go`

8. **Tests** (when created)

## UDS Bundle Issue (CRITICAL)

### Current State: **ARCHITECTURALLY WRONG**

The `UDSBundle` CRD is currently implemented as a 1:1 copy of `ZarfPackageJob`, which is **fundamentally incorrect**.

### The Real Problem

**UDS Bundles ≠ Zarf Packages**

From <https://github.com/defenseunicorns/uds-cli>:

```yaml
# UDS Bundle (REAL)
kind: UDSBundle
metadata:
  name: my-bundle
packages:
  - name: init
    repository: ghcr.io/defenseunicorns/packages
    ref: v0.4.0
  - name: podinfo
    repository: ghcr.io/defenseunicorns/packages
    ref: v6.4.0
```

**vs**

```go
// Our current WRONG implementation
type UDSBundleSpec struct {
    ServiceAccountName string
    Action             zarfv1alpha1.Action  // WRONG: bundles have different actions
    Source             zarfv1alpha1.PackageSource // WRONG: bundles reference packages
    // ...
}
```

### What UDS Bundles Actually Are

1. **Collections of Zarf Packages** - A bundle contains multiple package references
2. **Different Operations** - `uds create`, `uds publish`, `uds deploy` (not `zarf`)
3. **Package References** - Bundles point to existing packages, don't build them

### Current Code That's Wrong

**`pkg/controller/controller.go:238-251`**

```go
// handleUDSBundle reconciles a UDSBundle resource
func (c *Controller) handleUDSBundle(ctx context.Context, unstrObj *unstructured.Unstructured) error {
    // ...

    // Convert to ZarfPackageJob for processing  ← THIS IS WRONG!
    pkg := &zarfv1alpha1.ZarfPackageJob{
        ObjectMeta: bundle.ObjectMeta,
        Spec: zarfv1alpha1.ZarfPackageJobSpec{
            ServiceAccountName: bundle.Spec.ServiceAccountName,
            Action:             bundle.Spec.Action,
            Source:             bundle.Spec.Source,
            // ...
        },
    }

    return c.reconcilePackage(ctx, unstrObj, pkg)  ← WRONG! Bundles need different logic
}
```

### Recommended Approach for UDS Bundles

**Option 1: Remove Entirely (RECOMMENDED)**

- Remove `pkg/apis/uds/v1alpha1/`
- Remove UDS handling from controller
- Remove `config/crd/uds.io_udsbundles.yaml`
- Focus on Zarf packages first, add UDS later when properly designed

**Option 2: Mark as Explicitly Incomplete**

- Keep the CRD stub
- Make `handleUDSBundle` return `fmt.Errorf("UDS Bundle support not implemented")`
- Add big warning comments everywhere
- Create `docs/UDS_BUNDLE_TODO.md` with proper design

**Option 3: Implement Properly (HUGE WORK)**

- Redesign API types to match real UDS bundle structure
- Create separate `uds` action handlers
- Use `uds` CLI instead of `zarf` CLI
- Would require weeks of work

## Migration Strategy

### For ZarfPackageJob Rename

This is a **breaking change** requiring:

1. **CRD Update**: New CRD must be installed
2. **Resource Migration**: Existing resources must be recreated
3. **Documentation**: All docs must be updated

### Migration Steps

```bash
# 1. Export existing resources
kubectl get zarfpackagejobs -A -o yaml > backup.yaml

# 2. Install new CRD
kubectl delete crd zarfpackagejobs.forge.dev
kubectl apply -f config/crd/forge.dev_zarfpackagejobs.yaml

# 3. Update controller
kubectl apply -f config/manager/deployment.yaml

# 4. Migrate resources
# Edit backup.yaml: s/ZarfPackageJob/ZarfPackageJob/g, s/zarfpackagejobs/zarfpackagejobs/g
kubectl apply -f backup.yaml
```

## Refactoring Script

A complete refactoring would involve running:

```bash
# Bulk rename in all Go files
find . -name "*.go" -type f -not -path "*/vendor/*" -exec sed -i '' \
  -e 's/ZarfPackageJob/ZarfPackageJob/g' \
  -e 's/zarfpackagejobs/zarfpackagejobs/g' \
  {} +

# Update CRD files
mv config/crd/forge.dev_zarfpackagejobs.yaml config/crd/forge.dev_zarfpackagejobs.yaml
sed -i '' 's/ZarfPackageJob/ZarfPackageJob/g' config/crd/forge.dev_zarfpackagejobs.yaml

# Update samples
find config/samples -name "*.yaml" -exec sed -i '' 's/ZarfPackageJob/ZarfPackageJob/g' {} +

# Update docs
find docs -name "*.md" -exec sed -i '' 's/ZarfPackageJob/ZarfPackageJob/g' {} +
sed -i '' 's/ZarfPackageJob/ZarfPackageJob/g' README.md
```

## Decision Required

**You must decide:**

1. **Complete the ZarfPackageJob rename now?** (Big effort, but cleaner going forward)
2. **Leave as-is for now?** (Technical debt, confusing for users)
3. **Remove UDS Bundle support entirely?** (Cleanest for v1)
4. **Mark UDS as incomplete stub?** (Documents the issue)

## Recommendation

**Immediate Action (This PR):**

1. Complete `ZarfPackageJob` rename (already started)
2. Remove UDS Bundle CRD entirely
3. Update all docs to reflect changes
4. Add migration guide for existing users

**Future Work:**

1. Design UDS Bundle support properly (separate feature)
2. Implement with correct architecture
3. Release as v1alpha2 or v1beta1

This keeps Forge focused, correct, and maintainable.

---

**Status**: Documentation only - refactoring not completed yet
**Author**: AI Assistant
**Date**: 2025-01-21
