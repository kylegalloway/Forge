# Test 04: UDS Bundle Create

This test validates that Forge can create a UDS bundle from a Git source.

## What It Tests

- UDS bundle creation from Git repository
- UDS controller job management
- Artifact generation (bundle stored in workspace)

## Resources Created

- `ServiceAccount/test-uds-creator` - Service account with permissive policy
- `UDSBundleJob/test-uds-create` - Creates bundle from examples/samples/uds/03-git-build-deploy

## Expected Behavior

1. Controller detects new UDSBundleJob
2. Creates Job with init container to clone Git repo
3. Main container runs `uds create` to build the bundle
4. Job completes successfully
5. UDSBundleJob status shows `Completed`

## Duration

Approximately 2-3 minutes (depends on network speed for Git clone and image pulls).

## Manual Testing

```bash
kubectl apply -f serviceaccount.yaml
kubectl apply -f udsbundlejob.yaml
kubectl get udsbundlejobs -w
```
