# Test 06: Zarf BuildPublishDeploy Action Chain

## Description

Tests the complete Zarf workflow with action chaining:
- **Phase 1 (Build):** Build Zarf package from Git repository
- **Phase 2 (Publish):** Publish package to local OCI registry
- **Phase 3 (Deploy):** Deploy package to in-cluster Zarf installation

This tests the core action chaining logic where multiple actions are executed sequentially within a single ZarfPackageJob.

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- Local OCI registry accessible from cluster (or use OCI destination)
- Sufficient cluster resources for both build and deploy
- Service accounts with appropriate RBAC (see serviceaccount.yaml)

## Expected Behavior

1. Job creates successfully with 3 actions queued
2. Status transitions:
   - `buildStatus`: Pending → Running → Completed
   - `publishStatus`: Pending → Running → Completed
   - `deployStatus`: Pending → Running → Completed
3. Build artifacts processed through publish phase
4. Package deployed to cluster
5. All phases complete without errors

## Running the Test

```bash
# Apply service account
kubectl apply -f serviceaccount.yaml

# Run the action chain test
kubectl apply -f zarfpackagejob.yaml

# Watch progress (all three phases)
kubectl get zarfpackagejobs test-build-publish-deploy -w

# Check individual action status
kubectl get zarfpackagejob test-build-publish-deploy -o jsonpath='{.status.buildStatus}'
kubectl get zarfpackagejob test-build-publish-deploy -o jsonpath='{.status.publishStatus}'
kubectl get zarfpackagejob test-build-publish-deploy -o jsonpath='{.status.deployStatus}'

# View logs for each phase
kubectl logs -l forge.dev/package=test-build-publish-deploy,forge.dev/action=Build
kubectl logs -l forge.dev/package=test-build-publish-deploy,forge.dev/action=Publish
kubectl logs -l forge.dev/package=test-build-publish-deploy,forge.dev/action=Deploy

# Cleanup
kubectl delete -f zarfpackagejob.yaml
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- ZarfPackageJob `status.phase == "Completed"`
- All three action statuses completed:
  - `status.buildStatus == "Completed"`
  - `status.publishStatus == "Completed"`
  - `status.deployStatus == "Completed"`
- No error messages in any action logs
- Package successfully deployed to cluster

## Validation Steps

```bash
# Verify build artifacts were created
kubectl exec -it <pod-name> -- ls -la /workspace/

# Verify publish succeeded
# (Check OCI registry or storage backend for package)

# Verify deployment
kubectl get pods -n zarf
# Should see zarf-injector and other zarf components
```

## Troubleshooting

### Build Phase Fails
- Check Git repository access
- Verify resource requests sufficient for build
- Check service account permissions

### Publish Phase Fails
- Verify OCI registry is accessible
- Check registry credentials in service account
- Verify destination configuration

### Deploy Phase Fails
- Check cluster resources available
- Verify Zarf deployment target permissions
- Check RBAC for in-cluster deployment

### All Phases Timeout
- Increase job TTL: `spec.ttlSecondsAfterFinished`
- Increase timeout: `spec.timeout`
- Check controller logs for action chaining issues

## Test Scope

This test validates:
- ✅ Action chaining orchestration
- ✅ State preservation between actions (artifacts)
- ✅ Status field updates for multiple actions
- ✅ Job completion when all actions succeed
- ✅ Next action determination logic
- ✅ Multi-action job detection and handling
