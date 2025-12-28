# Troubleshooting: Job State Bouncing

## Symptom

When watching ZarfPackageJob or UDSBundleJob resources with `-w`, you observe the status phase bouncing between states like:
- Running → Completed → Running → Completed
- Pending → Running → Pending

This is especially noticeable with multi-action workflows (BuildPublish, BuildDeploy, CreatePublishDeploy, etc.).

## Root Cause

### Expected Behavior for Multi-Action Workflows

For jobs with chained actions (e.g., `BuildPublish`), the controller intentionally updates the status multiple times:

1. **Build phase starts**: Status set to "Running" with action "build"
2. **Build completes**: Status set to "Completed", build artifacts available
3. **Publish phase starts**: Status changes to "Running" with action "publish" (THIS LOOKS LIKE BOUNCING)
4. **Publish completes**: Status set to "Completed"

This is **expected behavior** and indicates that action chaining is working correctly.

### How Action Chaining Works

See `pkg/controller/job_monitor.go:184-189`:

```go
// Handle action chaining: if this job succeeded and is part of a chained workflow,
// trigger the next action
if phase == "Completed" {
    return ctrl.handleActionChaining(ctx, unstrObj, action, artifactLocation)
}
```

When a job completes successfully:
1. Controller marks the resource status as "Completed"
2. Controller checks if more actions are queued (via `handleActionChaining`)
3. If yes, controller creates a new Job for the next action
4. Status updates to "Running" for the new action

### Monitoring Interval

The job monitor runs every 10 seconds (`constants.JobMonitorInterval`):
- Checks all Jobs with label `app=forge`
- Updates ZarfPackageJob/UDSBundleJob status based on Job completion
- Triggers action chaining if applicable

## Diagnosis

### Check if Multi-Action Workflow

```bash
# Get the action from the resource
kubectl get zarfpackagejob <name> -o jsonpath='{.spec.action}'

# Multi-action workflows:
# - BuildPublish
# - BuildDeploy
# - PublishDeploy
# - BuildPublishDeploy
# - CreatePublish (UDS)
# - CreateDeploy (UDS)
# - PublishDeploy (UDS)
# - CreatePublishDeploy (UDS)
```

If the action contains multiple steps (e.g., "BuildPublish"), status bouncing is **expected**.

### Watch Detailed Status

Instead of watching just the phase, watch the full status to see which action is currently running:

```bash
# Watch full status
kubectl get zarfpackagejob <name> -o yaml -w | grep -A 10 "^status:"

# Watch with detailed fields
kubectl get zarfpackagejob <name> -o custom-columns=\
NAME:.metadata.name,\
ACTION:.spec.action,\
PHASE:.status.phase,\
BUILD:.status.buildStatus.state,\
PUBLISH:.status.publishStatus.state,\
DEPLOY:.status.deployStatus.state \
-w
```

### Check Job History

```bash
# List all Jobs for this package
kubectl get jobs -l forge.dev/package=<name>

# For multi-action workflows, you should see multiple jobs:
# <name>-build
# <name>-publish
# <name>-deploy
```

### View Controller Logs

```bash
# Filter for action chaining logs
kubectl logs -n forge-system -l app=forge-controller | grep -i "chaining\|completed"

# Expected output for BuildPublish:
# "Job status changed" job="<name>-build" package="<name>" phase="Completed"
# "Chaining to Publish after Build" package="<name>"
# "Executing Publish action" name="<name>" namespace="default"
# "Job status changed" job="<name>-publish" package="<name>" phase="Completed"
```

## When It's a Real Problem

Status bouncing is **NOT normal** when:

1. **Single-action workflow bounces**
   - Example: `action: Build` bounces between Running/Completed
   - This indicates controller reconciliation issues

2. **Bounces back to earlier action**
   - Example: BuildPublish goes Build → Publish → Build again
   - This should not happen

3. **Bounces without Job changes**
   - Jobs are not completing but status still bounces
   - Check `kubectl get jobs -l forge.dev/package=<name>` for active jobs

### Debugging Real Problems

```bash
# 1. Check if Jobs are actually completing
kubectl get jobs -l forge.dev/package=<name> -o wide

# 2. Check for controller errors
kubectl logs -n forge-system -l app=forge-controller --tail=100 | grep -i error

# 3. Check for reconciliation loops
kubectl logs -n forge-system -l app=forge-controller | grep "Reconciling\|reconcilePackage" | tail -20

# 4. Check resource versions (should increment)
kubectl get zarfpackagejob <name> -o jsonpath='{.metadata.resourceVersion}' -w
```

## Workarounds

### For Single-Action Workflows

If you see bouncing on single-action workflows (which shouldn't happen):

```bash
# Add generation tracking to prevent re-processing
# This is already implemented via observedGeneration field
kubectl get zarfpackagejob <name> -o jsonpath='{.status.observedGeneration}'
kubectl get zarfpackagejob <name> -o jsonpath='{.metadata.generation}'
# These should match when status is up-to-date
```

### For Better Visibility

Use custom columns to track per-action status:

```bash
kubectl get zarfpackagejobs \
  -o custom-columns=\
NAME:.metadata.name,\
ACTION:.spec.action,\
PHASE:.status.phase,\
BUILD:.status.buildStatus.state,\
PUBLISH:.status.publishStatus.state,\
MESSAGE:.status.message
```

## Prevention (Future Enhancements)

Potential improvements to reduce confusion:

1. **Add currentAction field to status**
   ```yaml
   status:
     phase: Running
     currentAction: publish  # Which action is currently executing
   ```

2. **Use more granular phases**
   ```yaml
   status:
     phase: BuildRunning | BuildCompleted | PublishRunning | PublishCompleted
   ```

3. **Add phase transition timestamps**
   ```yaml
   status:
     phaseTransitions:
     - from: Pending
       to: Running
       timestamp: "2024-01-01T00:00:00Z"
   ```

4. **Skip status update if already correct**
   - Check current status before updating
   - Only update if phase/message actually changed

## Related Files

- `pkg/controller/job_monitor.go` - Zarf job monitoring and status updates
- `pkg/controller/uds_job_monitor.go` - UDS job monitoring and status updates
- `pkg/constants/config.go` - JobMonitorInterval (10 seconds)
- `pkg/controller/controller.go:handleActionChaining()` - Triggers next action

## Summary

**TL;DR**: If you're using multi-action workflows (BuildPublish, CreateDeploy, etc.), seeing status bounce between "Running" and "Completed" is **normal and expected**. Each action in the chain causes the status to update.

To verify this is normal:
1. Check `kubectl get jobs -l forge.dev/package=<name>` - should see multiple jobs
2. Check controller logs for "Chaining to..." messages
3. Watch per-action status fields (buildStatus, publishStatus, deployStatus)

If the bouncing happens on single-action workflows or loops indefinitely, that's a bug - please file an issue with controller logs.
