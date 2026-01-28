# Debug Mode for Webhook and Controller

**Status:** Proposed

## Problem

When troubleshooting issues with Forge jobs, operators lack visibility into:
- What validation rules are being applied and why jobs are accepted/rejected
- How the controller processes events and makes decisions
- Detailed timing information for operations
- Policy evaluation results
- Why a particular action handler was selected

Currently, the only debug capability is a global `FORGE_DEBUG_MODE` environment variable that makes pods run `sleep infinity` instead of actual commands. This is useful for exec-ing into pods but doesn't provide observability into the webhook/controller decision-making process.

### Current State

| Component | Debug Capability | Limitation |
|-----------|------------------|------------|
| Job Pods | `FORGE_DEBUG_MODE` env var → `sleep infinity` | Global only, no per-job control |
| Webhook | Basic klog at V(5) for requests | No detailed validation tracing |
| Controller | Basic klog for reconciliation | No detailed event/handler tracing |
| Logging | Structured logger with correlation IDs | Not widely used in controller |

### User Pain Points

1. **"Why was my job rejected?"** - Webhook denies jobs but doesn't explain which policy check failed
2. **"Why isn't my job starting?"** - Controller logic is opaque, hard to trace event flow
3. **"I need to inspect the pod environment"** - Must set global env var, affects all jobs
4. **"What's taking so long?"** - No timing information for operations

## Proposed Solution

Implement a comprehensive debug mode with three layers:

1. **Per-Job Debug Flag** - CRD spec field to enable debug mode on individual jobs
2. **Enhanced Debug Logging** - Detailed tracing throughout webhook and controller
3. **Debug Pod Behavior** - Skip cleanup, run `sleep infinity`, extended TTL

### Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────┐
│                     Debug Mode Hierarchy                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Environment Variable (FORGE_DEBUG_MODE=true)                   │
│       │                                                          │
│       ▼  Global default (fallback)                              │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  CRD Spec: spec.debugMode: true                             ││
│  │       │                                                      ││
│  │       ▼  Per-job override (takes precedence)                ││
│  │  ┌───────────────────────────────────────────────────────┐  ││
│  │  │  Effects:                                              │  ││
│  │  │  • Pod runs "sleep infinity"                          │  ││
│  │  │  • Skip automatic cleanup (TTL = 1 hour)              │  ││
│  │  │  • Enhanced logging at V(4) for this job              │  ││
│  │  │  • Correlation ID propagated through all logs         │  ││
│  │  └───────────────────────────────────────────────────────┘  ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### API Changes

```go
// pkg/apis/zarf/v1alpha3/types.go

type ZarfPackageJobSpec struct {
    // ... existing fields ...

    // DebugMode enables debugging capabilities for this job.
    // When enabled:
    // - Job pods run "sleep infinity" instead of actual commands
    // - Automatic pod/job cleanup is skipped (TTL set to 1 hour)
    // - Enhanced debug logging is emitted for this job's operations
    // This allows operators to exec into pods and inspect the environment.
    // Can also be enabled globally via FORGE_DEBUG_MODE environment variable.
    // +optional
    DebugMode bool `json:"debugMode,omitempty"`
}
```

Similar changes for `UDSBundleJobSpec`.

### Webhook Debug Logging

Add detailed tracing to validation flow:

```go
// pkg/webhook/zarfpackage_validator.go

func (v *ZarfPackageJobValidator) Validate(ctx context.Context, pkg *zarfv1alpha3.ZarfPackageJob) *ValidationResult {
    v.logger.Debug(ctx, "Starting validation",
        "resource", pkg.Name,
        "namespace", pkg.Namespace,
        "serviceAccount", pkg.Spec.ServiceAccountName,
    )

    // Check allowed actions
    v.logger.Debug(ctx, "Checking allowed actions",
        "requestedAction", pkg.Spec.Action,
        "allowedActions", saAnnotations.AllowedActions,
    )

    if !isActionAllowed(pkg.Spec.Action, saAnnotations.AllowedActions) {
        v.logger.Debug(ctx, "Action not allowed",
            "requestedAction", pkg.Spec.Action,
            "allowedActions", saAnnotations.AllowedActions,
            "decision", "DENY",
        )
        return &ValidationResult{
            Allowed: false,
            Reason:  fmt.Sprintf("action %q not in allowed list", pkg.Spec.Action),
        }
    }
    v.logger.Debug(ctx, "Action allowed", "action", pkg.Spec.Action, "decision", "ALLOW")

    // Check source policies
    v.logger.Debug(ctx, "Checking source policy",
        "sourceType", pkg.Spec.Source.Type,
        "sourceURL", pkg.Spec.Source.Git.URL,
        "allowedSources", saAnnotations.AllowedSources,
    )

    // ... similar for each validation step ...

    v.logger.Debug(ctx, "Validation complete",
        "resource", pkg.Name,
        "allowed", true,
        "duration", time.Since(startTime),
    )

    return &ValidationResult{Allowed: true}
}
```

### Controller Debug Logging

Add detailed tracing to reconciliation and monitoring:

```go
// pkg/controller/generic_controller.go

func (c *GenericController[T]) handleObject(ctx context.Context, obj T) error {
    startTime := time.Now()
    meta := obj.GetObjectMeta()

    // Set up logging context
    ctx = logging.WithCorrelationID(ctx, logging.GenerateCorrelationID(meta.GetNamespace(), meta.GetName(), string(c.getAction(obj))))
    ctx = logging.WithJobName(ctx, meta.GetName())
    ctx = logging.WithNamespace(ctx, meta.GetNamespace())
    ctx = logging.WithAction(ctx, string(c.getAction(obj)))

    c.logger.Debug(ctx, "Handling object event",
        "eventType", "reconcile",
        "generation", meta.GetGeneration(),
        "resourceVersion", meta.GetResourceVersion(),
    )

    // Check terminal state
    if isTerminal(obj) {
        c.logger.Debug(ctx, "Skipping terminal resource",
            "status", obj.GetStatus().Phase,
            "reason", "already_completed",
        )
        return nil
    }

    // Determine action
    action := c.getAction(obj)
    c.logger.Debug(ctx, "Determined action",
        "action", action,
        "debugMode", c.isDebugModeEnabled(obj),
    )

    // Dispatch to handler
    c.logger.Debug(ctx, "Dispatching to handler",
        "handler", c.handlerName(action),
    )

    err := c.dispatchToHandler(ctx, obj, action)

    c.logger.Debug(ctx, "Handler completed",
        "duration", time.Since(startTime),
        "error", err,
    )

    return err
}
```

### Job Monitor Debug Logging

```go
// pkg/controller/generic_monitor.go

func (m *GenericJobMonitor[T]) checkJobStatus(ctx context.Context, monitoredJob *MonitoredJob) {
    m.logger.Debug(ctx, "Checking job status",
        "job", monitoredJob.JobName,
        "pollCount", monitoredJob.PollCount,
        "elapsedTime", time.Since(monitoredJob.StartTime),
    )

    job, err := m.kubeClient.BatchV1().Jobs(monitoredJob.Namespace).Get(ctx, monitoredJob.JobName, metav1.GetOptions{})
    if err != nil {
        m.logger.Debug(ctx, "Failed to get job",
            "job", monitoredJob.JobName,
            "error", err,
        )
        return
    }

    m.logger.Debug(ctx, "Job status retrieved",
        "active", job.Status.Active,
        "succeeded", job.Status.Succeeded,
        "failed", job.Status.Failed,
        "conditions", len(job.Status.Conditions),
    )

    // ... continue with status evaluation ...
}
```

### Debug Mode Activation Logic

```go
// pkg/controller/debug.go

// IsDebugModeEnabled returns true if debug mode should be enabled for the given resource.
// Per-job spec.debugMode takes precedence over global FORGE_DEBUG_MODE environment variable.
func IsDebugModeEnabled[T ForgeResource](obj T) bool {
    // Per-job override takes precedence
    if spec := obj.GetSpec(); spec != nil && spec.DebugMode {
        return true
    }
    // Fall back to global environment variable
    return constants.DebugMode
}
```

### Job Builder Updates

```go
// pkg/actions/job_builder.go

func (b *JobBuilder) Build() *batchv1.Job {
    // ... existing code ...

    if b.debugMode {
        containerArgs = []string{"sleep infinity"}
        klog.InfoS("Debug mode enabled, job will run sleep infinity instead of actual command",
            "job", b.job.Name,
            "originalCommand", b.command,
            "originalArgs", b.args,
        )

        // Set extended TTL for debug pods (1 hour instead of immediate cleanup)
        ttlSeconds := int32(3600)
        b.job.Spec.TTLSecondsAfterFinished = &ttlSeconds
    }

    // ... rest of build ...
}
```

## Implementation Checklist

### Phase 1: CRD Spec Field ✓ COMPLETED

Phase 1 has been implemented:
- Added `debugMode bool` field to both `ZarfPackageJobSpec` and `UDSBundleJobSpec`
- Added `GetDebugMode()` method to the `PackageResource` interface
- Updated all action handlers to use per-job debug flag with global fallback
- Updated `JobBuilder` to set extended TTL when debug mode enabled
- Regenerated CRDs with `make manifests`
- Added unit tests for debug mode in job_builder_test.go

### Phase 2: Enhanced Webhook Logging ✓ COMPLETED

Phase 2 has been implemented:
- Added correlation ID to all webhook request contexts via `logging.WithCorrelationID()`
- Added debug logging at each validation step (action, source, extraArgs, publish, deploy)
- Logged policy evaluation inputs and outputs (requested action vs allowed actions, etc.)
- Logged ServiceAccount annotations when fetched
- Logged final validation decision with detailed reason and duration
- Added timing information via `time.Since(startTime)`
- Existing unit tests verify debug log paths work correctly

### Phase 3: Enhanced Controller Logging ✓ COMPLETED

Phase 3 has been implemented:
- Added logger field to GenericController using `pkg/logging.Logger`
- Added correlation ID propagation through handleObject via `logging.WithCorrelationID()`
- Added debug logging for object event handling (generation, resourceVersion, debugMode)
- Added debug logging for action dispatch decisions (isPrimaryAction, handler selection)
- Added debug logging for PVC creation and terminal state skipping
- Added timing information for reconciliation duration via `time.Since(startTime)`
- Existing unit tests verify the controller behavior works correctly

### Phase 4: Enhanced Job Monitor Logging ✓ COMPLETED

Phase 4 has been implemented:
- Added logger field to GenericJobMonitor using `pkg/logging.Logger`
- Added correlation ID propagation through processJobStatus via `logging.WithCorrelationID()`
- Added debug logging for job status checks (`checkJobStatuses`, `processJobStatus`)
- Added debug logging for job/pod condition evaluation (each condition logged)
- Added debug logging for retry policy evaluation and decisions
- Added debug logging for action chaining (`handleActionChaining`, `determineNextAction`)
- Added debug logging for cleanup operations (`cleanupArtifactPVCIfNeeded`, `deleteFailedJob`)
- Added debug logging for resource adoption (`adoptDeployedResources`)
- Added timing information for job status check duration via `time.Since(startTime)`
- Existing unit tests verify the monitor behavior works correctly

### Phase 5: Debug Pod Behavior ✓ COMPLETED

Phase 5 has been implemented:
- [x] Set `TTLSecondsAfterFinished = 3600` when debugMode enabled (done in Phase 1)
- [x] Added `debugActions []string` field to specify which actions to debug in chained workflows
- [x] Changed debug pod behavior from `sleep infinity` to completion marker script
- [x] Debug pods now wait for `/tmp/debug-complete` marker file before exiting
- [x] When marker is created, pod exits successfully, allowing action chaining to continue
- [x] Added `ShouldDebugAction()` helper to check if specific action should be debugged
- [x] Updated all action handlers to use per-action debug checking
- [x] Regenerated CRDs with new `debugActions` field
- [x] Added comprehensive unit tests for `ShouldDebugAction` logic

### Phase 6: Documentation & Testing

- [ ] Document debug mode in README or operator guide
- [ ] Add troubleshooting guide with debug mode examples
- [ ] Integration tests for debug mode behavior
- [ ] Test debug mode precedence (spec overrides env var)

## Logging Levels Reference

The implementation uses klog verbosity levels consistently:

| Level | Method | Use Case |
|-------|--------|----------|
| V(0) | `Info()`, `Error()` | Essential operational info, errors |
| V(2) | `Warning()` | Non-critical warnings, retries |
| V(4) | `Debug()` | Detailed troubleshooting, validation steps |
| V(5) | `Trace()` | Very detailed trace (request bodies, etc.) |

To enable debug logging, run the controller/webhook with `-v=4` or `-v=5`.

## Example Usage

### Per-Job Debug Mode (All Actions)

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: debug-my-build
  namespace: forge-jobs
spec:
  debugMode: true  # Enable debug mode for ALL actions
  action: Build
  source:
    type: Git
    git:
      url: https://github.com/example/zarf-package
      ref: main
  build:
    flavor: "slim"
```

### Per-Action Debug Mode (Chained Workflows)

For chained actions like `BuildPublish`, use `debugActions` to debug specific steps:

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: debug-build-only
  namespace: forge-jobs
spec:
  action: BuildPublish
  debugActions:
    - build  # Only debug the build step, publish runs normally after completion
  source:
    type: Git
    git:
      url: https://github.com/example/zarf-package
      ref: main
  build:
    flavor: "slim"
  publish:
    destination:
      type: OCI
      oci:
        url: ghcr.io/example/packages
```

### Debug Workflow

```bash
# 1. Create job with debug mode
kubectl apply -f debug-job.yaml

# 2. Wait for pod to start (shows debug instructions)
kubectl get pods -n forge-jobs -w

# 3. Exec into the pod to inspect environment
kubectl exec -it -n forge-jobs debug-my-build-xxxxx -- /bin/sh

# 4. Inside pod: inspect workspace, run commands manually
ls -la /workspace
cat /workspace/zarf.yaml
zarf package create . --confirm

# 5. Signal debug completion to continue to next action (or finish)
touch /tmp/debug-complete

# The pod will exit successfully, and if this is a chained workflow,
# the next action (e.g., publish) will start automatically.
```

### Viewing Debug Logs

```bash
# Webhook debug logs
kubectl logs -n forge-system deployment/forge-webhook -f | grep -E 'correlationID|validation'

# Controller debug logs
kubectl logs -n forge-system deployment/forge-controller -f | grep -E 'correlationID|handler|dispatch'

# Filter by specific job
kubectl logs -n forge-system deployment/forge-controller -f | grep 'job="debug-my-build"'
```

## Alternatives Considered

### 1. Annotation-Based Debug Mode

**Pros:** No CRD schema change, flexible
**Cons:** Less discoverable, no validation, not in kubectl explain output

### 2. Debug CRD (DebugZarfPackageJob)

**Pros:** Clear separation of concerns
**Cons:** Duplicates entire CRD, maintenance burden, confusing UX

### 3. Spec Field (Recommended)

**Pros:** Discoverable, validated, works with kubectl explain, per-job control
**Cons:** Requires CRD regeneration

## Decision

Implement the **spec field approach** with a `debugMode` boolean:

1. Clear, discoverable API (shows in `kubectl explain`)
2. Per-job control with global fallback
3. Integrates naturally with existing JobBuilder debug mode
4. Enables future enhancements (debug log streaming, artifact preservation)

The enhanced logging throughout webhook and controller provides operational visibility independent of the per-job debug flag - operators can always run with `-v=4` to see detailed logs even without enabling debug mode on jobs.
