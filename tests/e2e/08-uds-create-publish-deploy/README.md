# Test 08: UDS CreatePublishDeploy Action Chain

## Description

Tests the complete UDS workflow with full action chaining:
- **Phase 1 (Create):** Create UDS bundle from packages
- **Phase 2 (Publish):** Publish bundle to registry
- **Phase 3 (Deploy):** Deploy bundle to cluster

This is the most complex action chain test, validating all three actions work together seamlessly.

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- UDS CLI available in cluster
- OCI registry accessible (local or cloud)
- Cluster has resources for bundle deployment
- Service accounts with appropriate RBAC
- Target cluster kubeconfig (for external deployments)

## Expected Behavior

1. Job creates successfully with 3 actions queued
2. Status transitions:
   - `createStatus`: Pending → Running → Completed
   - `publishStatus`: Pending → Running → Completed
   - `deployStatus`: Pending → Running → Completed
3. Bundle created from package sources
4. Bundle artifacts passed to publish phase
5. Published bundle deployed to cluster
6. All phases complete without errors
7. Deployed components accessible in cluster

## Running the Test

```bash
# Apply service account
kubectl apply -f serviceaccount.yaml

# Run the full action chain test
kubectl apply -f udsbundlejob.yaml

# Watch progress (all three phases)
kubectl get udsbundlejobs test-create-publish-deploy -w

# Check individual action status
kubectl get udsbundlejob test-create-publish-deploy -o jsonpath='{.status.createStatus}'
kubectl get udsbundlejob test-create-publish-deploy -o jsonpath='{.status.publishStatus}'
kubectl get udsbundlejob test-create-publish-deploy -o jsonpath='{.status.deployStatus}'

# View logs for each phase
kubectl logs -l forge.dev/bundle=test-create-publish-deploy,forge.dev/action=Create
kubectl logs -l forge.dev/bundle=test-create-publish-deploy,forge.dev/action=Publish
kubectl logs -l forge.dev/bundle=test-create-publish-deploy,forge.dev/action=Deploy

# Cleanup
kubectl delete -f udsbundlejob.yaml
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- UDSBundleJob `status.phase == "Completed"`
- All three action statuses completed:
  - `status.createStatus == "Completed"`
  - `status.publishStatus == "Completed"`
  - `status.deployStatus == "Completed"`
- No error messages in any action logs
- Bundle components deployed and accessible

## Validation Steps

```bash
# Verify bundle was created
kubectl exec -it <pod-name> -- ls -la /workspace/
# Should see bundle.tar.zst or similar

# Verify bundle deployed successfully
# Check for deployed components (e.g., Headlamp if using default example)
kubectl get pods -n headlamp
kubectl get pods -n zarf

# Verify connectivity to deployed services
kubectl get svc -n headlamp
kubectl port-forward -n headlamp svc/headlamp 3000:80
# Visit http://localhost:3000
```

## Troubleshooting

### Create Phase Fails
- Verify source packages accessible from Git
- Check UDS CLI version compatibility
- Verify service account has necessary permissions
- Check bundle YAML syntax if custom bundle

### Publish Phase Fails
- Verify OCI registry is accessible
- Check registry credentials in service account
- Verify registry has sufficient storage
- Check destination configuration

### Deploy Phase Fails
- Check cluster resources available for bundle
- Verify target cluster kubeconfig (if external)
- Check RBAC permissions for deployment
- Verify bundle compatibility with cluster version

### All Phases Timeout
- Increase job TTL: `spec.ttlSecondsAfterFinished`
- Increase timeout: `spec.timeout`
- Check controller logs for action chaining logic
- Monitor resource utilization during execution

## Test Scope

This test validates:
- ✅ Complete UDS action chaining with 3 phases
- ✅ Bundle creation, publication, and deployment
- ✅ Artifact passing between all three actions
- ✅ Status updates throughout job lifecycle
- ✅ Deploy target configuration (in-cluster vs external)
- ✅ Job completion and resource cleanup
- ✅ Deployed component accessibility
