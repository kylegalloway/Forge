# Forge Architecture

This document explains key architectural decisions and design patterns in Forge.

## Table of Contents

- [Handler Signature Divergence](#handler-signature-divergence)
- [Multi-Action Job Patterns](#multi-action-job-patterns)
- [Source Handler Pattern](#source-handler-pattern)

---

## Handler Signature Divergence

### Decision

Zarf and UDS action handlers intentionally use different signatures:

**Zarf Handlers:**
```go
Build:   Execute(ctx, pkg *ZarfPackageJob, artifactPVCName string)
Publish: Execute(ctx, pkg *ZarfPackageJob, artifactPath, artifactPVCName string)
Deploy:  Execute(ctx, pkg *ZarfPackageJob, artifactPath, artifactPVCName string)
```

**UDS Handlers:**
```go
Create:  Execute(ctx, bundle *UDSBundleJob)
Publish: Execute(ctx, bundle *UDSBundleJob)
Deploy:  Execute(ctx, bundle *UDSBundleJob)
```

### Rationale

The signature difference reflects fundamentally different approaches to multi-action job orchestration:

#### Zarf: Artifact Sharing via PersistentVolumeClaims

Zarf implements **stateful multi-action jobs** where actions share artifacts through a PersistentVolumeClaim:

1. **BuildPublish**: Build outputs package to `/artifacts` PVC → Publish reads from `/artifacts` PVC
2. **BuildDeploy**: Build outputs package to `/artifacts` PVC → Deploy reads from `/artifacts` PVC
3. **BuildPublishDeploy**: Three-stage pipeline sharing the same PVC

The `artifactPVCName` parameter enables this sharing:
- When empty: Action uses ephemeral storage (EmptyDir)
- When set: Action mounts the PVC and reads/writes artifacts

This approach **minimizes redundant work** - packages are built once and reused across Publish and Deploy actions without rebuilding.

#### UDS: Independent Job Execution

UDS implements **stateless multi-action jobs** where each action runs independently:

1. **CreatePublish**: Create builds bundle → Publish re-fetches source and builds
2. **CreateDeploy**: Create builds bundle → Deploy re-fetches source and builds
3. **CreatePublishDeploy**: Each action re-fetches and rebuilds independently

No artifact sharing occurs. Each action:
- Fetches source independently (Git clone, S3 download, OCI pull)
- Runs `uds create/publish/deploy` with fresh source
- Produces output independently

This approach **trades performance for simplicity** - no PVC management, no shared state, cleaner Job lifecycle.

### Trade-offs

| Aspect | Zarf (Artifact Sharing) | UDS (Independent) |
|--------|------------------------|-------------------|
| **Performance** | ✅ Faster - build once | ❌ Slower - rebuild per action |
| **Complexity** | ❌ More complex - PVC lifecycle | ✅ Simpler - no shared state |
| **Reliability** | ❌ PVC dependency | ✅ No shared state to corrupt |
| **Storage** | ❌ Requires PVC | ✅ Uses ephemeral storage |
| **Idempotency** | ❌ Stateful pipeline | ✅ Each action standalone |

### Why Not Unify?

Several options were considered:

1. **Add PVC support to UDS**: Rejected - UDS bundles can be multi-GB, making artifact sharing expensive and complex
2. **Remove PVC support from Zarf**: Rejected - Zarf packages are smaller and benefit significantly from artifact reuse
3. **Generic interface with optional PVC**: Rejected - forces complexity on all handlers for optional feature
4. **Current approach**: Accepted - signatures match their use cases

### Future Considerations

If multi-action job patterns converge in the future, consider:
- Optional PVC support in UDS (performance optimization)
- Builder pattern for Job construction (reduce signature divergence)
- Shared base handler with template methods

---

## Multi-Action Job Patterns

### Zarf Multi-Action Pipeline

```yaml
apiVersion: forge.dev/v1alpha3
kind: ZarfPackageJob
metadata:
  name: build-publish-deploy
spec:
  action: BuildPublishDeploy
  # ...
```

**Controller orchestration:**
1. Create PVC for artifact sharing
2. Execute Build → wait for completion
3. Execute Publish with PVC → wait for completion
4. Execute Deploy with PVC → wait for completion
5. Clean up PVC after pipeline completes

**Implementation:** See `pkg/controller/controller.go` - `handleObject()` method creates PVC and dispatches first action. Job monitoring (`pkg/controller/job_monitor.go`) detects completion and triggers next action.

### UDS Multi-Action Pipeline

```yaml
apiVersion: forge.dev/v1alpha3
kind: UDSBundleJob
metadata:
  name: create-publish-deploy
spec:
  action: createpublishdeploy
  # ...
```

**Controller orchestration:**
1. Execute Create → wait for completion
2. Execute Publish (re-fetch source) → wait for completion
3. Execute Deploy (re-fetch source) → wait for completion

**Implementation:** See `pkg/controller/uds_controller.go` - `handleUDSBundleJob()` method dispatches first action. Job monitoring (`pkg/controller/uds_job_monitor.go`) detects completion and triggers next action.

---

## Source Handler Pattern

Both Zarf and UDS use a common source handler pattern with adapter functions:

### Common Builders

Located in `pkg/sources/`:
- `BuildGitInitContainer(config, runAsUser)` - Shared Git clone logic
- `BuildS3InitContainer(config, runAsUser)` - Shared S3 download logic
- `BuildOCIInitContainer(config, runAsUser)` - Shared OCI pull logic

### Adapters

**Zarf**: `pkg/sources/types.go` - Interface-based with type-specific handlers
**UDS**: `pkg/sources/uds_adapters.go` - Adapter function converts UDS types to common config

Both call the same builder functions with different UIDs:
- Zarf: `DefaultZarfUID = 1000`
- UDS: `DefaultUDSUID = 65532`

This pattern achieves:
- ✅ Code reuse across Zarf and UDS
- ✅ Type safety through adapters
- ✅ Correct security contexts (different UIDs)
- ✅ Single source of truth for init container logic

See `pkg/sources/README.md` for detailed source handler documentation.

---

## Contributing

When making architectural changes:
1. Update this document with rationale
2. Update affected package documentation
3. Consider backward compatibility
4. Add tests for new patterns
