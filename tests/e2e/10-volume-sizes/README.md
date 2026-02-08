# Test 10: VolumeSizes Configuration

## Description

Tests the VolumeSizes feature, which allows customizing the storage capacity for job volumes (workspace, output, tmp, home).

This validates that:
- Custom workspace sizes are applied
- Custom output sizes are applied
- Custom tmp sizes are applied
- Custom home directory sizes are applied
- Defaults are used when not specified
- Sizes are sufficient for job execution

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- Cluster has sufficient storage capacity
- Service account with pod creation permissions

## Expected Behavior

1. Jobs create with specified volume sizes
2. Volumes are created with correct capacity
3. Jobs can write data up to specified limits
4. Jobs fail gracefully if volumes fill up
5. Default sizes used when not specified
6. Mixed custom and default sizes work correctly

## Test Cases

### Case 1: Custom Workspace Size (large)
For builds that need large workspace:
```bash
kubectl apply -f large-workspace-job.yaml
# Verify: Large workspace available for build artifacts
```

### Case 2: Custom Output Size
For jobs that produce large outputs:
```bash
kubectl apply -f large-output-job.yaml
# Verify: Large output volume for package artifacts
```

### Case 3: Custom Tmp Size
For build steps that need temporary storage:
```bash
kubectl apply -f large-tmp-job.yaml
# Verify: Tmp volume configured for intermediate files
```

### Case 4: All Custom Sizes
For resource-intensive operations:
```bash
kubectl apply -f all-custom-sizes-job.yaml
# Verify: All volumes sized appropriately
```

### Case 5: Default Sizes (not specified)
For simple operations:
```bash
kubectl apply -f default-sizes-job.yaml
# Verify: System defaults applied
```

## Running the Test

```bash
# Setup service account
kubectl apply -f serviceaccount.yaml

# Test 1: Large workspace
kubectl apply -f large-workspace-job.yaml
kubectl get zarfpackagejob test-large-workspace -w

# Test 2: Large output
kubectl apply -f large-output-job.yaml
kubectl get zarfpackagejob test-large-output -w

# Test 3: Large tmp
kubectl apply -f large-tmp-job.yaml
kubectl get zarfpackagejob test-large-tmp -w

# Test 4: All custom sizes
kubectl apply -f all-custom-sizes-job.yaml
kubectl get zarfpackagejob test-all-custom-sizes -w

# Test 5: Default sizes
kubectl apply -f default-sizes-job.yaml
kubectl get zarfpackagejob test-default-sizes -w

# Cleanup
kubectl delete -f all-custom-sizes-job.yaml
kubectl delete -f default-sizes-job.yaml
kubectl delete -f large-tmp-job.yaml
kubectl delete -f large-output-job.yaml
kubectl delete -f large-workspace-job.yaml
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- All jobs execute successfully
- Volumes created with specified sizes
- Jobs have adequate space for operations
- Default sizes applied when not specified
- Volume usage accessible via job logs/stats

## Validation Steps

```bash
# Check volume capacity in pod
kubectl exec -it <pod-name> -- df -h /workspace
kubectl exec -it <pod-name> -- df -h /output
kubectl exec -it <pod-name> -- df -h /tmp
kubectl exec -it <pod-name> -- df -h /home

# Verify job completed with correct resources
kubectl get zarfpackagejob <name> -o yaml | grep -A 5 volumeSizes
```

## Test Scope

This test validates:
- ✅ Workspace volume sizing
- ✅ Output volume sizing
- ✅ Tmp volume sizing
- ✅ Home directory sizing
- ✅ Default fallback behavior
- ✅ Mixed custom/default configurations
- ✅ Sufficient capacity for job execution
