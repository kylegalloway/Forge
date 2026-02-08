# Test 09: ExtraMounts Feature Validation

## Description

Tests the ExtraMounts feature, which allows mounting ConfigMaps and Secrets into job containers.

This validates that:
- ConfigMaps can be mounted as volume sources
- Secrets can be mounted as volume sources
- Multiple mounts work correctly
- Reserved paths are protected
- Duplicate paths are detected and rejected

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- ConfigMap and Secret created in cluster (see manifests)
- Service account with secret read permissions

## Expected Behavior

1. Job creates successfully with ConfigMap/Secret mounts
2. Init containers and main container have correct volumeMounts
3. Volumes section includes ConfigMap and Secret volumes
4. Job executes and accesses mounted data
5. Reserved paths are protected from mounting
6. Duplicate mount paths are rejected

## Test Cases

### Case 1: ConfigMap Mount
```bash
kubectl apply -f test-configmap.yaml
kubectl apply -f configmap-mount-job.yaml
# Verify ConfigMap data accessible in job
```

### Case 2: Secret Mount
```bash
kubectl apply -f test-secret.yaml
kubectl apply -f secret-mount-job.yaml
# Verify Secret data accessible in job
```

### Case 3: Multiple Mounts
```bash
kubectl apply -f multi-mount-job.yaml
# Verify both ConfigMap and Secret mounted
```

### Case 4: Reserved Path Protection (should fail)
```bash
kubectl apply -f reserved-path-job.yaml
# Should be rejected by webhook validation
```

## Running the Test

```bash
# Setup: Create ConfigMap and Secret
kubectl apply -f test-configmap.yaml
kubectl apply -f test-secret.yaml
kubectl apply -f serviceaccount.yaml

# Test ConfigMap mounting
kubectl apply -f configmap-mount-job.yaml
kubectl get zarfpackagejob test-cm-mount -w

# Test Secret mounting
kubectl apply -f secret-mount-job.yaml
kubectl get zarfpackagejob test-secret-mount -w

# Test multiple mounts
kubectl apply -f multi-mount-job.yaml
kubectl get zarfpackagejob test-multi-mount -w

# Cleanup
kubectl delete -f multi-mount-job.yaml
kubectl delete -f secret-mount-job.yaml
kubectl delete -f configmap-mount-job.yaml
kubectl delete -f test-secret.yaml
kubectl delete -f test-configmap.yaml
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- All jobs without reserved path violations execute successfully
- ConfigMap and Secret data accessible in containers
- Reserved path validation prevents invalid mounts
- Multiple mounts work correctly
- Job logs show successful access to mounted data

## Validation Steps

```bash
# Check mounted ConfigMap data
kubectl exec -it <pod-name> -- cat /workspace/config/settings.yaml
# Should output ConfigMap data

# Check mounted Secret data
kubectl exec -it <pod-name> -- cat /workspace/secret/password
# Should output Secret data (masked if sensitive)
```

## Test Scope

This test validates:
- ✅ ConfigMap mounting functionality
- ✅ Secret mounting functionality
- ✅ Multiple mount handling
- ✅ Reserved path protection
- ✅ Duplicate path detection
- ✅ Job execution with mounted data
