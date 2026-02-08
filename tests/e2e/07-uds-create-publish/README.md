# Test 07: UDS CreatePublish Action Chain

## Description

Tests the UDS bundle action chaining workflow:
- **Phase 1 (Create):** Create UDS bundle from source packages
- **Phase 2 (Publish):** Publish bundle to OCI registry

This validates that UDS action chaining works correctly, with the Create action building the bundle and the Publish action distributing it.

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- UDS CLI available in cluster
- Local OCI registry or cloud registry credentials configured
- Sufficient cluster resources for bundle creation
- Service accounts with appropriate RBAC

## Expected Behavior

1. Job creates successfully with 2 actions queued
2. Status transitions:
   - `createStatus`: Pending → Running → Completed
   - `publishStatus`: Pending → Running → Completed
3. Bundle created from package sources
4. Bundle artifacts passed to publish phase
5. Bundle published to destination registry
6. Both phases complete without errors

## Running the Test

```bash
# Apply service account
kubectl apply -f serviceaccount.yaml

# Run the action chain test
kubectl apply -f udsbundlejob.yaml

# Watch progress (both phases)
kubectl get udsbundlejobs test-create-publish -w

# Check action status
kubectl get udsbundlejob test-create-publish -o jsonpath='{.status.createStatus}'
kubectl get udsbundlejob test-create-publish -o jsonpath='{.status.publishStatus}'

# View logs for each phase
kubectl logs -l forge.dev/bundle=test-create-publish,forge.dev/action=Create
kubectl logs -l forge.dev/bundle=test-create-publish,forge.dev/action=Publish

# Cleanup
kubectl delete -f udsbundlejob.yaml
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- UDSBundleJob `status.phase == "Completed"`
- Both action statuses completed:
  - `status.createStatus == "Completed"`
  - `status.publishStatus == "Completed"`
- No error messages in action logs
- Bundle successfully published to registry

## Validation Steps

```bash
# Verify bundle was created
kubectl exec -it <pod-name> -- ls -la /workspace/
# Should see bundle.tar.zst or similar

# Verify publish succeeded
# Check OCI registry for published bundle
oras pull <registry>/test-bundles/latest
```

## Troubleshooting

### Create Phase Fails
- Verify source packages accessible
- Check UDS CLI version compatibility
- Verify service account permissions for bundle creation

### Publish Phase Fails
- Verify OCI registry is accessible
- Check registry credentials in service account
- Verify destination configuration in spec

### Both Phases Timeout
- Increase job TTL: `spec.ttlSecondsAfterFinished`
- Increase timeout: `spec.timeout`
- Check controller logs for action chaining issues

## Test Scope

This test validates:
- ✅ UDS action chaining orchestration
- ✅ Bundle creation from multiple packages
- ✅ Bundle artifact passing between actions
- ✅ Status updates for multiple actions
- ✅ Publish destination handling
- ✅ Job completion when all UDS actions succeed
