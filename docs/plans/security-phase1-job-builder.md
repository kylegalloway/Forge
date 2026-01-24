# Security Phase 1+6: Job Builder & Init Container Fixes

**Status:** Completed

## Scope

Changes to job creation code for Kyverno/PSS compliance.

## Tasks

### 1. Add `automountServiceAccountToken` to job pods

- [x] `pkg/actions/job_builder.go`: Add to PodSpec in `Build()` method
  - Set to `true` only when `serviceAccountName != ""`
  - Set to `false` when no service account specified

### 2. Add `readOnlyRootFilesystem: true` to main containers

- [x] `pkg/actions/job_builder.go`: Update `NonRootSecurityContextWithUID()`
  - Add `ReadOnlyRootFilesystem: Ptr(true)`

### 3. Add `/tmp` emptyDir mount to job containers

- [x] `pkg/actions/job_builder.go`: Add volume and mount in `Build()`
  - Volume: `tmp` with emptyDir, sizeLimit 1Gi
  - Mount: `/tmp`

### 4. Add seccomp profiles to init containers

- [x] `pkg/sources/git.go`: Add `SeccompProfile: RuntimeDefault` to SecurityContext
- [x] `pkg/sources/s3.go`: Add `SeccompProfile: RuntimeDefault` to SecurityContext
- [x] `pkg/sources/oci.go`: Add `SeccompProfile: RuntimeDefault` to SecurityContext

### 5. Add `readOnlyRootFilesystem` to init containers

- [x] `pkg/sources/git.go`: Add `ReadOnlyRootFilesystem: true`
- [x] `pkg/sources/s3.go`: Add `ReadOnlyRootFilesystem: true`
- [x] `pkg/sources/oci.go`: Add `ReadOnlyRootFilesystem: true`

### 6. Add emptyDir size limits (Phase 6)

- [x] `pkg/actions/job_builder.go`: Add sizeLimit to workspace emptyDir volumes
  - Default: 10Gi for workspace, 10Gi for output, 1Gi for tmp

## Verification

- [x] `go build ./...` passes
- [x] `go test ./pkg/actions/...` passes
- [x] `go test ./pkg/sources/...` passes

## Completion Log

- **Task 1**: Added `automountServiceAccountToken` to PodSpec in `Build()` method. Token is mounted only when a service account is explicitly specified (for RBAC), otherwise disabled for security.
- **Task 2**: Added `ReadOnlyRootFilesystem: Ptr(true)` to `NonRootSecurityContextWithUID()` function.
- **Task 3**: Added `/tmp` emptyDir volume (1Gi limit) and mount in `Build()` method to support containers with read-only root filesystems.
- **Task 4**: Added `SeccompProfile: RuntimeDefault` to security contexts in `git.go`, `s3.go`, and `oci.go` init containers.
- **Task 5**: Added `ReadOnlyRootFilesystem: true` to security contexts in `git.go`, `s3.go`, and `oci.go` init containers.
- **Task 6**: Added size limits to workspace (10Gi) and output (10Gi) emptyDir volumes in `WithWorkspaceVolume()`.

All changes verified with `go build ./...` and `go test ./pkg/actions/...` and `go test ./pkg/sources/...`.
