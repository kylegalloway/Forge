# Test 05: UDS Bundle Deploy

This test validates that Forge can create and deploy a UDS bundle from a Git source.

## What It Tests

- UDS bundle creation from Git repository
- Action chaining (Create → Deploy)
- In-cluster deployment via service account
- Deployed components are running in cluster

## Resources Created

- `ServiceAccount/test-uds-deployer` - Service account with permissive policy
- `UDSBundleJob/test-uds-deploy` - Creates and deploys bundle from examples/samples/uds/03-git-build-deploy

## What Gets Deployed

The bundle deploys Headlamp (a Kubernetes UI) to the `headlamp` namespace.

## Expected Behavior

1. Controller detects new UDSBundleJob
2. Creates Job with init container to clone Git repo
3. Main container runs `uds create` to build the bundle
4. On success, controller chains to Deploy action
5. Deploy Job runs `uds deploy` to install the bundle
6. UDSBundleJob status shows `Completed`
7. Headlamp pods are running in `headlamp` namespace

## Duration

Approximately 3-5 minutes (depends on network speed and image pull times).

## Manual Testing

```bash
kubectl apply -f serviceaccount.yaml
kubectl apply -f udsbundlejob.yaml
kubectl get udsbundlejobs -w

# After completion, verify deployment
kubectl get pods -n headlamp
```

## Cleanup

The test runner cleans up the UDSBundleJob resources. The deployed Headlamp
remains in the cluster unless manually removed:

```bash
kubectl delete namespace headlamp
```
