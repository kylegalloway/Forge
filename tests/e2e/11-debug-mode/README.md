# Test 11: Debug Mode Feature

## Description

Tests the debug mode feature, which allows pausing job execution for interactive debugging.

This validates that:
- Debug mode prevents job completion
- Debug marker scripts are created correctly
- TTL configuration applies in debug mode
- Per-action debug control works
- Global vs per-action debug precedence is correct
- Pod remains running for interactive debugging

## Prerequisites

- Forge controller deployed in `forge-system` namespace
- kubectl access to cluster
- Ability to exec into pods

## Expected Behavior

1. Job creates with debug: true
2. Job runs initialization but pauses before completion
3. Pod remains running indefinitely (or until TTL)
4. Debug marker script available in pod
5. User can exec into pod for interactive debugging
6. Job status shows as Pending/Running (not Completed)
7. Cleanup removes job/pod when done

## Test Cases

### Case 1: Global Debug Mode
Job pauses after build execution:
```bash
kubectl apply -f debug-global-job.yaml
# Pod remains running for inspection
```

### Case 2: Per-Action Debug
Debug only the build action:
```bash
kubectl apply -f debug-per-action-job.yaml
# Build action pauses; deploy runs to completion
```

### Case 3: Debug with TTL
Job auto-cleans up after TTL expires:
```bash
kubectl apply -f debug-ttl-job.yaml
# Job deleted after 300 seconds (5 minutes)
```

### Case 4: Action Chain with Debug
First action debugged in chain:
```bash
kubectl apply -f debug-action-chain-job.yaml
# Build pauses; Publish/Deploy queued until manual resume
```

## Running the Test

```bash
# Setup service account
kubectl apply -f serviceaccount.yaml

# Test 1: Global debug
kubectl apply -f debug-global-job.yaml
sleep 5
kubectl get zarfpackagejob test-debug-global
kubectl exec -it <pod-name> -- /bin/bash
# Inside pod: inspect logs, artifacts, etc.
# ctrl+d to exit - pod continues running
# ctrl+c to stop job

# Test 2: Per-action debug
kubectl apply -f debug-per-action-job.yaml
# Only build is paused; deploy runs independently

# Test 3: Debug with TTL
kubectl apply -f debug-ttl-job.yaml
sleep 60
# Job will auto-delete after TTL expires

# Test 4: Action chain debug
kubectl apply -f debug-action-chain-job.yaml
# First action pauses; subsequent actions queued

# Cleanup
kubectl delete -f debug-action-chain-job.yaml --ignore-not-found=true
kubectl delete -f debug-ttl-job.yaml --ignore-not-found=true
kubectl delete -f debug-per-action-job.yaml --ignore-not-found=true
kubectl delete -f debug-global-job.yaml --ignore-not-found=true
kubectl delete -f serviceaccount.yaml
```

## Success Criteria

- Jobs with debug: true do not complete automatically
- Pod remains accessible for interactive debugging
- Debug marker script exists and is executable
- TTL configuration works correctly
- Per-action debug control works independently
- Job status reflects paused state
- No errors in job execution before pause point

## Validation Steps

```bash
# Check job status
kubectl get zarfpackagejob test-debug-global -o jsonpath='{.status.phase}'
# Should show Pending or Running (not Completed)

# Check if pod is running
kubectl get pods -l forge.dev/package=test-debug-global
# Should show pod in Running state

# Exec into pod for debugging
kubectl exec -it <pod-name> -- /bin/bash

# Inside pod:
ls -la /workspace/            # View workspace contents
ls -la /output/               # View build artifacts
cat /proc/1/environ           # View environment variables
ps aux                        # View running processes

# Check for debug marker script
ls -la /.forge-debug-marker
# or look for it in standard location
```

## Troubleshooting

### Pod Doesn't Stay Running
- Verify debug: true is set in spec
- Check controller logs for debug flag processing
- Ensure pod TTL is not set too short

### Can't Exec into Pod
- Verify pod is in Running state
- Check RBAC permissions for exec
- Verify container has shell (/bin/bash or /bin/sh)

### Debug Marker Script Missing
- Check /workspace/ directory
- Check pod logs for marker script creation
- Verify controller version supports debug markers

### Job Doesn't Clean Up After TTL
- Check ttlSecondsAfterFinished value
- Verify job TTL controller is running in cluster
- Check for finalizers preventing deletion

## Test Scope

This test validates:
- ✅ Global debug mode functionality
- ✅ Per-action debug control
- ✅ Debug mode precedence (per-action overrides global)
- ✅ TTL configuration with debug mode
- ✅ Pod persistence during debug
- ✅ Debug marker script creation
- ✅ Action chaining with debug pausing
- ✅ Interactive debugging capability
