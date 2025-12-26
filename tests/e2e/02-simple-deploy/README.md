# Test 02: Simple Deploy

## Description

Tests basic Zarf package deployment functionality using:
- Pre-built package from public OCI registry (no credentials needed)
- In-cluster deployment target
- Permissive service account (minimal RBAC configuration)

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- Permissive service account created (see serviceaccount.yaml)
- Cluster has sufficient resources for deployment

## Expected Behavior

1. Job creates successfully
2. Status transitions: Pending → Running → Completed
3. Package components deployed to cluster
4. No errors in Job logs

## Running the Test

```bash
# Apply service account
kubectl apply -f serviceaccount.yaml

# Run the test
kubectl apply -f zarfpackagejob.yaml

# Watch progress
kubectl get zarfpackagejobs -w

# Check deployed resources (example assumes hello-world package)
kubectl get pods -n zarf

# Check job status
kubectl get job -l forge.dev/package=test-simple-deploy

# View logs
kubectl logs -l forge.dev/package=test-simple-deploy

# Cleanup
kubectl delete -f zarfpackagejob.yaml
kubectl delete namespace zarf --ignore-not-found=true
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- ZarfPackageJob status.phase == "Completed"
- Job succeeded (1/1)
- Package components deployed (check with `kubectl get pods -n zarf`)
- No error messages in logs
