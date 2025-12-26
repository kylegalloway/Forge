# Test 01: Simple Build

## Description

Tests basic Zarf package build functionality using:
- Public Git repository (no credentials needed)
- DevMode enabled (no external registry needed)
- Permissive service account (minimal RBAC configuration)

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- Permissive service account created (see serviceaccount.yaml)

## Expected Behavior

1. Job creates successfully
2. Status transitions: Pending → Running → Completed
3. Build artifacts stored in `/workspace/package.tar.zst`
4. No errors in Job logs

## Running the Test

```bash
# Apply service account
kubectl apply -f serviceaccount.yaml

# Run the test
kubectl apply -f zarfpackagejob.yaml

# Watch progress
kubectl get zarfpackagejobs -w

# Check job status
kubectl get job -l forge.dev/package=test-simple-build

# View logs
kubectl logs -l forge.dev/package=test-simple-build

# Cleanup
kubectl delete -f zarfpackagejob.yaml
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- ZarfPackageJob status.phase == "Completed"
- Job succeeded (1/1)
- No error messages in logs
