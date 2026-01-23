# kubectl-forge Improvement Plan

## Status: COMPLETED

All planned improvements have been implemented.

| Item | Status |
|------|--------|
| UDS jobs not listed | Done |
| Naive PVC discovery | Done |
| Code duplication | Done |
| Hardcoded TTY size | Done |
| `diagnose` command | Done |
| `status` command | Done |
| `logs controller\|webhook` command | Done |
| `cancel` command | Done |
| `retry` command | Done |
| `--watch` flag on list | Done |
| Multi-pod debug support (`--all-pods`) | Done |
| Color output for phase status | Done |
| Shell completion | Provided by Cobra |

---

## Overview

The kubectl-forge CLI tool has become stale due to changes in the main project. This plan outlines bugs to fix, improvements to make, and new auto-debug commands to add.

---

## Phase 1: Critical Bug Fixes

### 1.1 UDS Jobs Not Listed

**Location:** `pkg/kubectl/client.go:483`

**Problem:** The `ListJobs()` function uses label selector `"app=forge"` which only matches Zarf jobs. UDS jobs use label `"app=forge-uds"` and are completely invisible to the CLI.

**Impact:**
- `kubectl forge list` won't show UDS bundle jobs
- `debug`, `download`, and all `get` commands won't work for UDS jobs
- Tool is effectively broken for half of its use cases

**Fix:** Change selector from `"app=forge"` to `"app in (forge,forge-uds)"`

---

## Phase 2: Medium Priority Fixes

### 2.1 Naive PVC Discovery

**Location:** `pkg/kubectl/client.go:94-108`

**Problem:** PVC detection only finds first PVC with "artifact" in the name:
```go
if strings.Contains(volume.PersistentVolumeClaim.ClaimName, "artifact") {
    return volume.PersistentVolumeClaim.ClaimName, nil
}
```

**Impact:** May fail with different PVC naming schemes or multiple PVCs.

**Fix:** Use the `forge.dev/artifact-storage=true` label that the controller applies to artifact PVCs.

### 2.2 Code Duplication

**Location:** `cmd/kubectl-forge/get_events.go:165-183`

**Problem:** `formatDuration()` function is defined in `get_events.go` but also called from `get_job.go`. Should be in a shared location.

**Fix:** Move `formatDuration()` to `pkg/kubectl/output.go` or a new `cmd/kubectl-forge/util.go`.

---

## Phase 3: Low Priority / UX Issues

### 3.1 Hardcoded TTY Size

**Location:** `pkg/kubectl/client.go:656-677`

**Problem:** TTY implementation returns fixed 80x24 terminal size regardless of actual terminal dimensions. No resize support.

**Impact:** Poor UX for `debug` command - interactive shells always use 80x24.

**Fix:** Detect actual terminal size using `golang.org/x/term` and handle SIGWINCH for resize events.

### 3.2 Unused CRD Imports

**Location:** `pkg/kubectl/client.go:23-24, 62-67`

**Problem:** CRD types are imported and registered with scheme but never used. The client only queries Kubernetes Job objects, not the CRDs directly.

**Fix:** Either remove unused imports or implement direct CRD queries for richer information.

---

## Phase 4: New Auto-Debug Commands

### 4.1 `kubectl forge diagnose <job>`

**Purpose:** Automatically surface all problems with a job for users debugging failed builds/deploys.

**Usage:**
```bash
kubectl forge diagnose my-package-build
kubectl forge diagnose my-package-build --verbose --logs-lines 50
```

**Checks to Perform:**

1. **Job Status**
   - Phase, message, retry count
   - Last failure reason from CRD status
   - Time in current state

2. **Pod Health**
   - Find all pods for the job
   - Detect common issues:
     - `ImagePullBackOff` / `ErrImagePull`
     - `CrashLoopBackOff`
     - `OOMKilled`
     - `ContainerCreating` stuck
     - Init container failures
     - Termination reasons and exit codes

3. **Events**
   - Show warning events sorted by time
   - Highlight scheduling failures, resource issues

4. **Logs**
   - Extract last N lines from failed/erroring containers
   - Auto-detect which container failed

5. **PVC Status**
   - Check if artifact PVC is bound
   - Check for capacity issues

6. **Resource Quotas**
   - Check if namespace has quota issues blocking pod creation

**Output Format:**
```text
Job: my-package-build (ZarfPackageJob)
Namespace: forge-system
Phase: Failed
Message: Build failed after 2 retries

━━━ Problems Found ━━━
✗ Pod my-package-build-xyz: OOMKilled (exit code 137)
  Container 'build' exceeded memory limit (512Mi)

✗ Warning Event: FailedScheduling
  0/3 nodes available: insufficient memory

━━━ Recent Logs (build container) ━━━
[last 20 lines of error output]

━━━ Suggestions ━━━
• Increase memory limit in job spec
• Check if cluster has available resources
```

**Flags:**
- `--verbose` / `-v` - Show all events, not just warnings
- `--logs-lines N` - Number of log lines to show (default 20)
- `--output json|yaml` - Machine-readable output for scripting

---

### 4.2 `kubectl forge status`

**Purpose:** Overall system health check for operators managing Forge installations.

**Usage:**
```bash
kubectl forge status
kubectl forge status --watch
kubectl forge status --output json
```

**Checks to Perform:**

1. **Controller Health**
   - Deployment exists and has ready replicas
   - Pod status (Running, restarts, age)
   - Recent error logs (last 5 minutes)
   - Metrics endpoint reachable (`/healthz`, `/readyz`)

2. **Webhook Health**
   - Deployment exists and has ready replicas
   - TLS certificate validity (expiry check)
   - ValidatingWebhookConfiguration exists and registered
   - Pod anti-affinity working (pods on different nodes)

3. **CRD Status**
   - ZarfPackageJob and UDSBundleJob CRDs installed
   - Correct API versions available (v1alpha3)

4. **Cluster-wide Job Summary**
   - Count of jobs by phase (Pending/Running/Completed/Failed/Retrying)
   - Any stuck jobs (Running > 1 hour)
   - Any jobs in retry loop (retryCount > 3)

5. **RBAC Check**
   - Controller ServiceAccount has required permissions
   - Webhook ServiceAccount has required permissions

**Output Format:**
```text
Forge System Status
Namespace: forge-system

━━━ Controller ━━━
✓ Deployment: forge-controller (1/1 ready)
✓ Pod: forge-controller-abc123 (Running, 0 restarts, 2d)
✓ Health: /healthz OK, /readyz OK
✗ Recent Errors: 2 errors in last 5 minutes
  • "Watch error, restarting in 5 seconds" (3m ago)
  • "Failed to update status" (2m ago)

━━━ Webhook ━━━
✓ Deployment: forge-webhook (2/2 ready)
✓ Pods distributed across 2 nodes
✓ TLS Certificate: Valid (expires in 89 days)
✓ ValidatingWebhookConfiguration: Registered

━━━ CRDs ━━━
✓ zarfpackagejobs.forge.dev (v1alpha3)
✓ udsbundlejobs.forge.dev (v1alpha3)

━━━ Jobs Summary ━━━
  Pending:   2
  Running:   5
  Completed: 47
  Failed:    3
  Retrying:  1

⚠ Warnings:
  • 1 job running > 1 hour: long-build-xyz
  • 1 job in retry loop (attempt 4): flaky-deploy-abc
```

**Flags:**
- `--namespace` / `-n` - Check specific namespace only (default: all)
- `--watch` / `-w` - Continuously refresh status
- `--output json|yaml` - Machine-readable output

---

### 4.3 `kubectl forge logs controller|webhook`

**Purpose:** Quick access to operator component logs without remembering label selectors.

**Usage:**
```bash
kubectl forge logs controller
kubectl forge logs controller --errors --since 5m
kubectl forge logs webhook --follow
```

**Features:**
- Auto-discovers controller/webhook pods by label (`app=forge-controller`, `app=forge-webhook`)
- Works in any namespace (discovers `forge-system` automatically)

**Flags:**
- `--errors` / `-e` - Filter to error-level logs only
- `--since` - Time-based filtering (e.g., `5m`, `1h`)
- `--follow` / `-f` - Stream logs in real-time
- `--tail N` - Number of lines to show (default 100)

---

## Phase 5: Feature Enhancements

### 5.1 Watch Support

Add `--watch` flag to `list` and `get pods` commands for real-time monitoring of job progress.

### 5.2 Cancel Command

```bash
kubectl forge cancel <job> [--delete-pvc]
```

Delete a running or pending job and optionally clean up associated PVCs.

### 5.3 Direct CRD Queries

Query ZarfPackageJob/UDSBundleJob CRDs directly instead of just Kubernetes Jobs to get richer information:
- Full spec details
- Detailed status conditions
- Retry configuration
- Action chain progress

### 5.4 Shell Completion

Add bash/zsh/fish completion scripts for better CLI UX:
- Complete job names
- Complete command and flag names
- Context-aware suggestions

---

## Implementation Order

| Priority | Task | Effort | Impact |
|----------|------|--------|--------|
| 1 | Fix UDS label selector | Small | Critical - tool half-broken |
| 2 | Add `diagnose` command | Medium | High user value |
| 3 | Add `status` command | Medium | Operator visibility |
| 4 | Fix PVC discovery with labels | Small | Reliability |
| 5 | Add `logs controller\|webhook` | Small | Operator convenience |
| 6 | Refactor code duplication | Small | Code quality |
| 7 | Add `cancel` command | Small | User convenience |
| 8 | Improve TTY handling | Medium | Debug UX |
| 9 | Add `--watch` support | Medium | Monitoring UX |
| 10 | Shell completion | Medium | CLI UX |

---

## Technical Notes

### Controller/Webhook Discovery

The operator components use these labels:
- Controller: `app=forge-controller`, `app.kubernetes.io/component=controller`
- Webhook: `app=forge-webhook`, `app.kubernetes.io/component=webhook`

Default namespace: `forge-system`

### Health Endpoints

| Component | Endpoint | Port |
|-----------|----------|------|
| Controller | `/healthz` | 8081 |
| Controller | `/readyz` | 8081 |
| Controller | `/metrics` | 8080 |
| Webhook | `/healthz` | 8443 (HTTPS) |
| Webhook | `/readyz` | 8443 (HTTPS) |
| Webhook | `/metrics` | 8080 |

### Job Labels

Jobs created by the controller have these labels:
- `app=forge` (Zarf) or `app=forge-uds` (UDS)
- `forge.dev/package=<name>`
- `forge.dev/action=<Build|Publish|Deploy|Create>`
- `forge.dev/job-type=<type>`

### CRD Status Fields

Key status fields for diagnostics:
- `status.phase` - Pending/Running/Completed/Failed/Retrying
- `status.message` - Human-readable status
- `status.buildStatus.lastFailureReason` - Error details
- `status.buildStatus.retryCount` - Retry attempts
- `status.buildStatus.nextRetryTime` - When retry will occur
