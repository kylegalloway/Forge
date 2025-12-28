# Zarf vs UDS Implementation Unification Plan

**Goal**: Reduce cognitive load for developers by eliminating duplication and unifying common patterns between Zarf and UDS implementations.

**Current State**: 3,000+ lines of duplicated controller/monitoring logic across parallel implementations.

---

## Executive Summary

The Zarf and UDS implementations follow nearly identical patterns but maintain separate codebases with 80-90% duplication. This creates:

- **High cognitive load**: Developers must learn two parallel systems
- **Maintenance burden**: Bug fixes must be applied twice
- **Inconsistency risk**: Features added to one may not reach the other
- **Test duplication**: 1,850+ lines of test code testing the same patterns

**Proposed Solution**: Generic controller pattern with type parameters and shared infrastructure.

**Estimated Impact**:

- Reduce controller code by ~60% (2,000+ lines eliminated)
- Single mental model for developers
- Unified test infrastructure
- Consistent behavior across both resource types

---

## Current Architecture Analysis

### Duplication Breakdown

| Component | Zarf Lines | UDS Lines | Duplication % | Notes |
|-----------|------------|-----------|---------------|-------|
| **Controllers** | 416 | 470 | 85% | Nearly identical reconciliation logic |
| **Job Monitors** | 288 | 245 | 90% | Same monitoring patterns, different labels |
| **Build/Create Handlers** | 279 | 261 | 75% | Different CLI commands, same structure |
| **Publish Handlers** | 260 | 311 | 70% | UDS has extra env var logic |
| **Deploy Handlers** | 300 | 272 | 80% | Different kubeconfig handling |
| **Controller Tests** | 966 | 906 | 90% | Same test patterns, different types |
| **Handler Tests** | 652 | 944 | 85% | Different fixtures, same test logic |
| **Total** | **3,161** | **3,409** | **~83%** | **6,570 lines, 5,450+ duplicated** |

### Key Structural Differences

#### 1. Resource Types (Legitimate)

- **Zarf**: `ZarfPackageJob` - manages single Zarf packages
- **UDS**: `UDSBundleJob` - manages UDS bundles (collections of Zarf packages)

#### 2. Action Names (Semantic)

- **Zarf**: Build, Publish, Deploy
- **UDS**: Create, Publish, Deploy

#### 3. Handler Signatures (Design Inconsistency)

```go
// Zarf - passes artifact location
Publish.Execute(ctx, pkg, artifactPath string, pvcName string)

// UDS - no artifact tracking
Publish.Execute(ctx, bundle)
```

**Why this matters**: Zarf supports artifact PVC sharing for multi-action workflows; UDS doesn't. This is a feature gap, not a legitimate difference.

#### 4. Status Field Names (Needless Difference)

- **Zarf**: `buildStatus`, `publishStatus`, `deployStatus`
- **UDS**: `createStatus`, `publishStatus`, `deployStatus`

#### 5. Job Labels (Configuration)

- **Zarf**: `app=forge`
- **UDS**: `app=forge-uds`

#### 6. Metrics Naming (Duplication)

- **Zarf**: `RecordPackageBuild*`, `RecordZarfPackageJob*`
- **UDS**: `RecordBundleCreate*`, `RecordUDSBundleJob*`

---

## Root Causes of Duplication

### 1. No Generic Controller Pattern

Go 1.18+ supports generics, but Forge predates this or didn't adopt them. Result: copy-paste controllers.

### 2. Tight Coupling to Type Names

Controllers are coupled to specific type names (`ZarfPackageJob` vs `UDSBundleJob`) instead of interfaces.

### 3. Action Name Differences Propagated Everywhere

The Build→Create rename cascades through:

- Handler names (`BuildHandler` → `CreateHandler`)
- Status field names (`buildStatus` → `createStatus`)
- Metrics names (`RecordPackageBuild*` → `RecordBundleCreate*`)
- Test function names

### 4. Missing Abstraction Layers

No shared interfaces for:

- Package/Bundle resources
- Action handlers
- Job monitoring

### 5. Independent Evolution

UDS was likely created by copying Zarf and modifying, then evolved separately. Features added to one (like action chaining improvements) don't automatically reach the other.

---

## Proposed Unification Strategy

### Phase 1: Shared Abstractions (Low Risk, High Value)

**Goal**: Extract common interfaces and types without changing existing code.

#### 1.1 Package Resource Interface

```go
// pkg/controller/types.go
type PackageResource interface {
    GetName() string
    GetNamespace() string
    GetServiceAccountName() string
    GetAction() string
    GetSourceConfig() SourceConfig
    GetPublishConfig() *PublishConfig
    GetDeployConfig() *DeployConfig
}

// Implement for both types
func (z *ZarfPackageJob) GetAction() string { return z.Spec.Action }
func (u *UDSBundleJob) GetAction() string { return u.Spec.Action }
```

**Impact**: Enables writing functions that work with both types.

#### 1.2 Action Handler Interface

```go
// pkg/actions/common/handler.go
type ActionHandler[T PackageResource] interface {
    Execute(ctx context.Context, resource T, opts ExecuteOptions) (*ActionResult, error)
}

type ExecuteOptions struct {
    ArtifactPath    string
    ArtifactPVCName string
}
```

**Impact**: Unified handler signature, extensible options.

#### 1.3 Job Monitor Interface

```go
// pkg/controller/monitor.go
type JobMonitor[T PackageResource] interface {
    ProcessJobStatus(ctx context.Context, job *batchv1.Job) error
    CheckJobStatuses(ctx context.Context) error
    HandleActionChaining(ctx context.Context, resource T, completedAction, artifactPath string) error
}
```

**Impact**: Shared monitoring logic.

### Phase 2: Generic Controller (Medium Risk, High Impact)

**Goal**: Single controller implementation that works with both resource types.

```go
// pkg/controller/generic_controller.go
type GenericController[T PackageResource] struct {
    kubeClient     kubernetes.Interface
    dynamicClient  dynamic.Interface
    namespace      string
    resourceGVR    schema.GroupVersionResource
    metrics        *telemetry.Metrics
    tracer         *telemetry.Tracer
    policyEngine   *policy.Engine

    // Generic handlers
    primaryHandler ActionHandler[T]  // Build or Create
    publishHandler ActionHandler[T]
    deployHandler  ActionHandler[T]

    // Config
    config ControllerConfig
}

type ControllerConfig struct {
    ResourceType    string  // "ZarfPackageJob" or "UDSBundleJob"
    PrimaryAction   string  // "Build" or "Create"
    JobLabelApp     string  // "forge" or "forge-uds"
    SupportsPVC     bool    // true for Zarf, false for UDS
}
```

**Usage**:

```go
// Zarf controller
zarfCtrl := NewGenericController[*ZarfPackageJob](
    kubeClient, dynamicClient, namespace,
    constants.ZarfPackageJobGVR,
    ControllerConfig{
        ResourceType:  "ZarfPackageJob",
        PrimaryAction: "Build",
        JobLabelApp:   "forge",
        SupportsPVC:   true,
    },
)

// UDS controller
udsCtrl := NewGenericController[*UDSBundleJob](
    kubeClient, dynamicClient, namespace,
    constants.UDSBundleJobGVR,
    ControllerConfig{
        ResourceType:  "UDSBundleJob",
        PrimaryAction: "Create",
        JobLabelApp:   "forge-uds",
        SupportsPVC:   false,
    },
)
```

**Impact**:

- Eliminates 800+ lines of duplicated controller code
- Single reconciliation logic
- Unified bug fixes and features

### Phase 3: Unified Job Monitoring (Medium Risk, High Impact)

**Goal**: Single job monitoring implementation using generics.

```go
// pkg/controller/generic_monitor.go
type GenericJobMonitor[T PackageResource] struct {
    kubeClient    kubernetes.Interface
    dynamicClient dynamic.Interface
    namespace     string
    resourceGVR   schema.GroupVersionResource
    metrics       MetricsRecorder[T]
    config        MonitorConfig
}

type MetricsRecorder[T PackageResource] interface {
    RecordJobCreated(ctx context.Context, namespace, name, action string)
    RecordJobCompleted(ctx context.Context, namespace, name, action string)
    RecordJobFailed(ctx context.Context, namespace, name, action string)
    RecordActionDuration(ctx context.Context, namespace, name, action string, duration float64, status string)
}
```

**Impact**:

- Eliminates 500+ lines of duplicated monitoring code
- Consistent behavior
- Single place to add features (like detailed failure tracking)

### Phase 4: Handler Unification (Higher Risk, High Value)

**Goal**: Unify Build and Create handlers using shared infrastructure.

#### Current Problem

```go
// Zarf
buildHandler.Execute(ctx, pkg, pvcName)
// UDS
createHandler.Execute(ctx, bundle)
```

#### Proposed Solution

```go
// Unified signature
type BuildCreateHandler[T PackageResource] struct {
    config BuildCreateConfig
}

type BuildCreateConfig struct {
    CLICommand      string  // "zarf package create" or "uds create"
    OutputPath      string  // "/workspace/package.tar.zst" or "/workspace/bundle.tar.zst"
    SupportsPVC     bool
    DefaultTimeout  time.Duration
}

func (h *BuildCreateHandler[T]) Execute(ctx context.Context, resource T, opts ExecuteOptions) (*ActionResult, error) {
    // Unified logic with config-driven differences
}
```

**Impact**:

- Eliminates 500+ lines of duplicated handler code
- Single command builder
- Unified error handling

### Phase 5: Test Infrastructure Unification (Low Risk, High Value)

**Goal**: Shared test utilities and fixtures.

```go
// pkg/controller/testing/controller_test_helpers.go
type ControllerTestFixture[T PackageResource] struct {
    kubeClient    *fake.Clientset
    dynamicClient *dynamicfake.FakeDynamicClient
    controller    *GenericController[T]
}

func NewZarfControllerFixture() *ControllerTestFixture[*ZarfPackageJob] { ... }
func NewUDSControllerFixture() *ControllerTestFixture[*UDSBundleJob] { ... }

// Shared test functions
func TestGenericControllerReconciliation[T PackageResource](t *testing.T, fixture *ControllerTestFixture[T]) {
    // Tests that work for both types
}
```

**Impact**:

- Eliminates 1,500+ lines of duplicated test code
- Consistent test coverage
- Faster test development

---

## Migration Strategy

### Approach: Gradual Extraction (Safe, Low Risk)

**Principle**: Keep existing code working while building new infrastructure alongside.

#### Step 1: Add Abstractions (Week 1)

1. Create `pkg/controller/types.go` with interfaces
2. Implement interfaces on existing types
3. Add generic monitor in `pkg/controller/generic_monitor.go`
4. Add tests for new code
5. **No changes to existing controllers yet**

**Risk**: Zero - only additive changes.

#### Step 2: Migrate Job Monitoring (Week 2)

1. Create `GenericJobMonitor` implementation
2. Add feature flag: `USE_GENERIC_MONITOR=true`
3. Switch Zarf controller to use generic monitor behind flag
4. Switch UDS controller to use generic monitor behind flag
5. Run parallel (both monitors) in test environment
6. Compare metrics/behavior
7. Enable flag in production
8. Delete old monitors after 1 week

**Risk**: Low - feature-flagged, reversible.

#### Step 3: Migrate Controllers (Week 3-4)

1. Create `GenericController` implementation
2. Add feature flag: `USE_GENERIC_CONTROLLER=true`
3. Instantiate both old and new controllers
4. Run both in parallel (both reconcile)
5. Compare metrics/behavior
6. Switch flag to only use new
7. Delete old controllers after verification

**Risk**: Medium - but mitigated by parallel operation.

#### Step 4: Migrate Handlers (Week 5-6)

1. Create unified `BuildCreateHandler`, `PublishHandler`, `DeployHandler`
2. Instantiate in new controllers
3. Test in parallel
4. Switch over
5. Delete old handlers

**Risk**: Medium - handlers are complex, need thorough testing.

#### Step 5: Cleanup (Week 7)

1. Delete old controller files
2. Delete old monitor files
3. Delete old handler files
4. Update documentation
5. Update architecture diagrams

**Risk**: Low - just deletion.

---

## Benefits Analysis

### Quantitative Benefits

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| **Controller Code** | 3,161 lines | ~1,200 lines | 62% reduction |
| **Test Code** | 1,872 lines | ~600 lines | 68% reduction |
| **Files to Understand** | 14 files | 6 files | 57% reduction |
| **Duplicated Patterns** | 8 major patterns | 0 patterns | 100% elimination |
| **Mental Models** | 2 (Zarf + UDS) | 1 (Generic) | 50% reduction |

### Qualitative Benefits

1. **Onboarding**: New developers learn one pattern, apply to both
2. **Bug Fixes**: Fix once, both benefit
3. **Feature Development**: Implement once, both get it
4. **Code Review**: Review one implementation, not two
5. **Testing**: Write one test suite, covers both
6. **Documentation**: Document one system, covers both

### Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking Zarf | Low | High | Feature flags, parallel operation |
| Breaking UDS | Low | High | Feature flags, parallel operation |
| Performance regression | Low | Medium | Benchmark both before/after |
| Generic code harder to read | Medium | Low | Good documentation, examples |
| Migration takes longer | Medium | Low | Time-boxed phases |

---

## Detailed Refactoring Recommendations

### 1. Unify Status Field Names

**Problem**: `buildStatus` vs `createStatus` - needless difference.

**Solution**: Use action-agnostic field names.

```go
// Current (Zarf)
type ZarfPackageJobStatus struct {
    BuildStatus   *OperationStatus
    PublishStatus *OperationStatus
    DeployStatus  *OperationStatus
}

// Current (UDS)
type UDSBundleJobStatus struct {
    CreateStatus  *OperationStatus
    PublishStatus *OperationStatus
    DeployStatus  *OperationStatus
}

// Proposed (both)
type PackageJobStatus struct {
    PrimaryStatus  *OperationStatus  // Build or Create
    PublishStatus  *OperationStatus
    DeployStatus   *OperationStatus
}
```

**Migration**: Add `PrimaryStatus` field, populate both old and new, deprecate old after 1 release.

### 2. Unify Handler Signatures

**Problem**: Zarf handlers take artifactPath, UDS handlers don't.

**Solution**: Use options struct for extensibility.

```go
// Current (inconsistent)
Zarf:  Execute(ctx, pkg, artifactPath, pvcName)
UDS:   Execute(ctx, bundle)

// Proposed (unified)
type ExecuteOptions struct {
    ArtifactPath    string
    ArtifactPVCName string
    // Future extensibility
    DryRun          bool
    Variables       map[string]string
}

Execute(ctx context.Context, resource PackageResource, opts ExecuteOptions) (*ActionResult, error)
```

### 3. Unify Metrics Recording

**Problem**: Parallel metric functions for each type.

**Solution**: Generic metrics recorder with type parameter.

```go
// Current
metrics.RecordPackageBuildStarted(ctx, namespace, name)
metrics.RecordBundleCreateStarted(ctx, namespace, name)

// Proposed
metrics.RecordActionStarted(ctx, resourceType, namespace, name, action)
// Where: resourceType = "zarf" or "uds"
//        action = "build" or "create"
```

**Better Alternative**: Make metrics agnostic to resource type.

```go
metrics.RecordPrimaryActionStarted(ctx, namespace, name)  // works for both Build and Create
```

### 4. Unify Job Labels

**Problem**: Different label selectors make shared utilities harder.

**Solution**: Use resource-type label instead of app label difference.

```go
// Current
Zarf: app=forge
UDS:  app=forge-uds

// Proposed (both)
Labels: {
    "app": "forge",
    "forge.dev/resource-type": "zarf",  // or "uds"
    "forge.dev/package": packageName,
    "forge.dev/action": action,
}

// Selector becomes:
app=forge,forge.dev/resource-type=zarf
```

### 5. Add PVC Support to UDS

**Problem**: UDS doesn't support artifact PVCs, limiting multi-action efficiency.

**Solution**: Extend UDS to use same PVC pattern as Zarf.

**Benefit**: After unification, both automatically get this feature.

### 6. Unify Deploy Kubeconfig Handling

**Problem**: Different kubeconfig mounting patterns.

```go
// Zarf
type ExternalClusterConfig struct {
    KubeconfigSecretRef *SecretReference
}

// UDS
type KubeconfigReference struct {
    SecretRef *corev1.SecretReference
}
```

**Solution**: Standardize on simpler pattern.

```go
// Unified
type ExternalCluster struct {
    KubeconfigSecretRef corev1.SecretReference
}
```

---

## Implementation Checklist

### Phase 1: Shared Abstractions (1 week)

- [ ] Create `pkg/controller/types.go` with `PackageResource` interface
- [ ] Create `pkg/actions/common/handler.go` with `ActionHandler` interface
- [ ] Create `pkg/controller/monitor.go` with `JobMonitor` interface
- [ ] Implement interfaces on `ZarfPackageJob`
- [ ] Implement interfaces on `UDSBundleJob`
- [ ] Add tests for interface implementations
- [ ] Document new abstractions

### Phase 2: Generic Monitor (1 week)

- [ ] Create `pkg/controller/generic_monitor.go`
- [ ] Implement `GenericJobMonitor` with type parameters
- [ ] Add feature flag `USE_GENERIC_MONITOR`
- [ ] Add comprehensive tests
- [ ] Enable for Zarf behind flag
- [ ] Enable for UDS behind flag
- [ ] Run both monitors in parallel for 1 week
- [ ] Compare metrics and behavior
- [ ] Enable flag by default
- [ ] Delete old monitor files

### Phase 3: Generic Controller (2 weeks)

- [ ] Create `pkg/controller/generic_controller.go`
- [ ] Implement `GenericController[T]`
- [ ] Add feature flag `USE_GENERIC_CONTROLLER`
- [ ] Add comprehensive tests
- [ ] Run old and new controllers in parallel
- [ ] Compare reconciliation behavior
- [ ] Enable flag by default
- [ ] Delete old controller files

### Phase 4: Unified Handlers (2 weeks)

- [ ] Create unified `BuildCreateHandler[T]`
- [ ] Create unified `PublishHandler[T]`
- [ ] Create unified `DeployHandler[T]`
- [ ] Add comprehensive tests
- [ ] Integrate with generic controller
- [ ] Run in parallel with old handlers
- [ ] Verify behavior matches
- [ ] Switch over completely
- [ ] Delete old handler files

### Phase 5: Unified Tests (1 week)

- [ ] Create `pkg/controller/testing/` package
- [ ] Implement generic test fixtures
- [ ] Convert Zarf tests to use generic fixtures
- [ ] Convert UDS tests to use generic fixtures
- [ ] Verify coverage maintained
- [ ] Delete old test duplicates

### Phase 6: Cleanup & Documentation (1 week)

- [ ] Delete all old files
- [ ] Update `ARCHITECTURE.md`
- [ ] Update `CLAUDE.md` with new patterns
- [ ] Update examples
- [ ] Update API documentation
- [ ] Write migration guide for external users
- [ ] Create before/after comparison doc

---

## Success Criteria

### Must Have

1. ✅ All existing tests pass
2. ✅ No behavior changes for end users
3. ✅ Metrics continue to work
4. ✅ Code reduction of >50%
5. ✅ Single reconciliation logic for both types

### Should Have

1. ✅ Test code reduction of >60%
2. ✅ Generic controller performance ≈ current
3. ✅ Documentation updated
4. ✅ Migration completed in 8 weeks

### Nice to Have

1. ⭐ UDS gains PVC support automatically
2. ⭐ Shared test utilities speed up future test development
3. ⭐ Generic pattern extensible to future resource types

---

## Alternatives Considered

### Alternative 1: Keep Separate (Status Quo)

**Pros**: No migration risk
**Cons**: Continued duplication, high cognitive load
**Verdict**: ❌ Not recommended - technical debt grows

### Alternative 2: Delete UDS, Merge Into Zarf

**Pros**: Simplest unification
**Cons**: Loses UDS-specific semantics, breaks existing users
**Verdict**: ❌ Not recommended - too disruptive

### Alternative 3: Code Generation

**Pros**: No generics needed
**Cons**: Doesn't reduce mental model complexity, still 2 codebases
**Verdict**: ❌ Not recommended - doesn't solve core problem

### Alternative 4: Shared Base Class

**Pros**: Familiar pattern
**Cons**: Go doesn't have inheritance, requires composition
**Verdict**: ⚠️ Possible but generics are cleaner

### Alternative 5: Generic Controller (Recommended)

**Pros**: Single implementation, type-safe, modern Go
**Cons**: Requires Go 1.18+, migration effort
**Verdict**: ✅ **RECOMMENDED** - best long-term solution

---

## Open Questions

1. **API Versioning**: Should we create a unified v2 API or keep separate versions?
   - **Recommendation**: Keep separate - UDSBundleJob and ZarfPackageJob have different semantics

2. **Metrics Naming**: Should we rename all metrics to be resource-agnostic?
   - **Recommendation**: Yes, but with deprecation period for Prometheus compatibility

3. **PVC for UDS**: Should we add PVC support to UDS as part of unification?
   - **Recommendation**: Yes - it's a feature, not just unification

4. **Deployment Mode**: Generic controller in same binary or separate binaries?
   - **Recommendation**: Same binary, instantiate both controllers

5. **Backwards Compatibility**: How long to support old metric names?
   - **Recommendation**: 2 releases (6 months) with both old and new

---

## Appendix: File Structure After Unification

```text
pkg/
├── controller/
│   ├── generic_controller.go        # NEW: Unified controller
│   ├── generic_monitor.go           # NEW: Unified job monitor
│   ├── types.go                      # NEW: Shared interfaces
│   ├── pvc.go                        # UPDATED: Works with both types
│   ├── controller_test.go           # REMOVED: Merged into generic tests
│   ├── uds_controller.go            # REMOVED: Replaced by generic
│   ├── uds_controller_test.go       # REMOVED: Merged into generic tests
│   ├── job_monitor.go               # REMOVED: Replaced by generic
│   ├── uds_job_monitor.go           # REMOVED: Replaced by generic
│   └── testing/
│       ├── fixtures.go              # NEW: Generic test fixtures
│       └── generic_controller_test.go  # NEW: Unified tests
│
├── actions/
│   └── common/
│       ├── handler.go               # NEW: Generic handler interface
│       ├── build_create_handler.go  # NEW: Unified build/create
│       ├── publish_handler.go       # NEW: Unified publish
│       ├── deploy_handler.go        # NEW: Unified deploy
│       ├── job_builder.go           # KEPT: Already shared
│       └── types.go                 # KEPT: Already shared
│   ├── zarf/                         # REMOVED: Merged into common
│   └── uds/                          # REMOVED: Merged into common
```

**Result**: 14 files → 8 files (43% reduction)

---

## Conclusion

The current Zarf/UDS duplication represents significant technical debt:

- **5,450+ lines of duplicated code** (83% duplication rate)
- **2 mental models** developers must learn
- **2× maintenance cost** for every bug fix and feature

The proposed generic controller pattern eliminates this duplication while:

- ✅ Maintaining type safety through generics
- ✅ Preserving semantic differences (Build vs Create)
- ✅ Enabling shared innovation (features benefit both)
- ✅ Reducing cognitive load (single pattern to learn)
- ✅ Minimizing migration risk (gradual, feature-flagged approach)

**Recommendation**: Proceed with unification using the phased approach outlined above.

**Timeline**: 8 weeks from start to cleanup complete.

**Effort**: ~2-3 weeks of focused development work spread across 8 weeks.

**ROI**: Every future feature and bug fix saves 50% development time.
