# TODO

## Existing Issues

### Testing Infrastructure

* **make test-validation** - YAML validation tests need review
  * Location: `pkg/validation/`
  * Action: Review and ensure all config files are properly validated

* **make e2e-test** - E2E tests need completion
  * Location: `scripts/test-e2e.sh`
  * Action: Complete end-to-end test coverage for all workflows

* **OTLP test skips** - Some tests skipped for OTLP, might be removed
  * Location: `pkg/telemetry/` tests
  * Action: Review skipped telemetry tests and decide to fix or remove

* **Job state bouncing** - Job bouncing between Running/Completed
  * Location: `pkg/controller/job_monitor.go`, `pkg/controller/uds_job_monitor.go`
  * Action: Investigate and fix job state reconciliation logic

* **Verify Examples** - Examples folder has examples that haven't been checked to make sure they work
  * Location: `examples/`
  * Action: Investigate and validate all example manifests

---

## Critical Inconsistencies (Fix Immediately)

### Missing Source Handler Integration

* **Integrate UDS actions with shared source handlers**
  * Location: `pkg/actions/uds/create.go:200-249`
  * Issue: UDS actions implement inline Git cloning (`alpine/git:latest`) instead of using `pkg/sources`
  * Pattern: Use `sources.New()` and `sourceHandler.GetInitContainer()` like Zarf does
  * Files to update: `create.go`, `publish.go`, `deploy.go` in `pkg/actions/uds/`
  * Remove: Inline Git clone implementation from init containers
  * Impact: **Architectural inconsistency** - Zarf uses shared source handlers, UDS doesn't

### Missing Defensive Checks

* ~~**Add container bounds checking to zarf/build.go** ✅ COMPLETED~~
* ~~**Add container bounds checking to uds/create.go** ✅ NOT NEEDED - No array access that requires defensive checks~~

**Note**: All action handlers now have proper defensive checks where needed.

---

## High Priority (Configuration & Code Quality)

### Constants Not Being Used

* ~~**Move CLI image constants to pkg/constants/config.go** ✅ COMPLETED - ZarfCLIImage and UDSCLIImage now in constants package~~

* ~~**Replace hardcoded UIDs with constants** ✅ COMPLETED - All Zarf (1000→DefaultZarfUID) and UDS (65532→DefaultUDSUID) actions updated~~

### Timeout Handling Inconsistencies

* **Make Build action timeout configurable**
  * Location: `pkg/actions/zarf/build.go:98`
  * Current: Hardcoded `int64(3600)` (1 hour)
  * Add: Support for `Spec.Build.Timeout` field in CRD
  * Use: `constants.DefaultBuildTimeout` as default

* **Make Publish action timeout configurable**
  * Location: `pkg/actions/zarf/publish.go:105`
  * Current: Hardcoded `int64(1800)` (30 minutes)
  * Add: Support for `Spec.Publish.Timeout` field in CRD
  * Use: `constants.DefaultPublishTimeout` as default

* **Make UDS Create action timeout configurable**
  * Location: `pkg/actions/uds/create.go:87`
  * Current: Hardcoded `int64(7200)` (2 hours)
  * Add: Support for `Spec.Create.Timeout` field in CRD
  * Use: `constants.DefaultCreateTimeout` as default

* **Make UDS Publish action timeout configurable**
  * Location: `pkg/actions/uds/publish.go:84`
  * Current: Hardcoded `int64(3600)` (1 hour)
  * Add: Support for `Spec.Publish.Timeout` field in CRD
  * Use: New constant (create `constants.DefaultUDSPublishTimeout`)

* **Standardize timeout parsing to use time.ParseDuration**
  * Location: `pkg/actions/uds/deploy.go:282-318`
  * Issue: Custom parsing logic instead of standard library
  * Replace: With `time.ParseDuration` like Zarf deploy uses
  * Current: Manual string parsing for "30m", "1h", "2h30m" formats

### Resource Requirement Inconsistencies

* ~~**Fix Zarf Publish CPU request** ✅ COMPLETED - Changed from 250m to 200m for consistency with Build~~

* ~~**Convert UDS resource limits to millicore notation** ✅ COMPLETED - All UDS actions now use explicit millicore notation ("1000m", "2000m")~~

* **Standardize resource requirements across similar operations**
  * Build/Create should have same resources (both create artifacts)
  * Publish should have same resources across Zarf and UDS
  * Deploy should have same resources across Zarf and UDS
  * Document reasoning in comments

### Code Duplication

* ~~**Create shared helpers package** ✅ COMPLETED~~
  * ~~Create: `pkg/util/helpers.go`~~
  * ~~Move: `ptr[T any](v T) *T` function (duplicated in 3 places) → util.Ptr()~~
  * ~~Move: `mustParseQuantity(quantityStr string) resource.Quantity` function (duplicated in 2 places) → util.MustParseQuantity()~~
  * ~~All action handlers, source handlers, and tests updated to use util package~~

* **Consolidate ActionResult type**
  * Location: Duplicated in `pkg/actions/zarf/types.go:8-33` and `pkg/actions/uds/types.go:17-42`
  * Options:
    1. Move to `pkg/actions/common/types.go` (recommended - aligns with common package)
    2. Use generic type with package-specific aliases
    3. Keep separate but document why (if they diverge in the future)

---

## Medium Priority (Testing & Documentation)

### Test Coverage Gaps

* **Improve UDS controller test coverage**
  * Location: `pkg/controller/uds_controller_test.go` (712 lines)
  * Issue: 21% fewer tests than Zarf controller (902 lines)
  * Add: Tests for edge cases, error conditions, and policy enforcement
  * Target: Match Zarf controller coverage

### Missing UDS Documentation

* **Create dedicated UDS user guide**
  * Location: Create `docs/getting-started/UDS_GUIDE.md`
  * Include: Complete examples of Create, Publish, Deploy operations
  * Include: Policy configuration for UDS bundles
  * Include: Troubleshooting common issues
  * Include: Differences between v1alpha1 and v1alpha2 APIs

* **Create UDS-specific troubleshooting guide**
  * Location: Create `docs/operations/UDS_TROUBLESHOOTING.md`
  * Include: Common error messages and solutions
  * Include: Debugging failed UDS jobs
  * Include: Policy validation failures
  * Include: Bundle creation issues

* **Add UDS policy configuration examples**
  * Location: `examples/policies/uds/`
  * Create: Example ServiceAccount annotations for UDS bundles
  * Create: Example RBAC configurations
  * Create: Example restricted vs permissive policies

* **Update CLAUDE.md with UDS vs Zarf guidance**
  * Location: `CLAUDE.md` and `CLAUDE.local.md`
  * Add: Clear decision tree for when to use UDS vs Zarf
  * Add: UDS-specific examples and patterns
  * Add: UDS bundle structure and requirements
  * Add: v1alpha2 migration path

### Naming Inconsistencies

* **Standardize app label values**
  * Current: Zarf jobs use `"app": "forge"`, UDS jobs use `"app": "forge-uds"`
  * Target: Both use `"app": "forge"` with additional labels for differentiation
  * Suggested: Add `"resource-type": "zarfpackagejob"` or `"resource-type": "udspackagejob"`
  * Files affected (all 6 action handlers):
    * `pkg/actions/zarf/build.go:105,120`
    * `pkg/actions/zarf/publish.go:112,127`
    * `pkg/actions/zarf/deploy.go:113,128`
    * `pkg/actions/uds/create.go:94,109`
    * `pkg/actions/uds/publish.go:91,106`
    * `pkg/actions/uds/deploy.go:100,115`

* **Align controller receiver variable names**
  * Location: `pkg/controller/controller.go` vs `pkg/controller/uds_controller.go`
  * Current: Zarf uses `controller`, UDS uses `ctrl`
  * Decision: Standardize on one (prefer `ctrl` for brevity and Go convention)
  * Update: All method receivers and local variables

---

## Low Priority (Logging & Conventions)

### Logging Inconsistencies

* **Standardize metric naming**
  * Issue: UDS uses `RecordBundleCreateStarted`, Zarf uses `RecordBuildStarted`
  * Decision needed: Either add "Package" prefix to Zarf OR drop "Bundle" from UDS
  * Files: `pkg/telemetry/metrics.go` and all action handlers
  * Recommendation: `RecordPackageBuildStarted` and `RecordBundleCreateStarted` for clarity

* **Standardize log message format**
  * Issue: UDS prefixes with "UDS Bundle", Zarf doesn't use "Zarf Package"
  * Decision needed: Consistent prefix strategy across both
  * Files: All `klog.InfoS()` calls in action handlers
  * Recommendation: Always include resource type for clarity

* **Document klog verbosity level conventions**
  * Location: Add to `docs/development/LOGGING.md` (create if needed)
  * Define: V(2) = info, V(4) = debug, V(6) = trace
  * Update: All klog calls to follow convention
  * Current usage: V(2) for "reusing job", V(4) for timeout errors (inconsistent)

---

## Future Enhancements (Not Inconsistencies)

* **Support adopting existing resources** for deploy actions
* **Support for additional source types** (Azure DevOps, GitLab, Bitbucket)
* **Support for additional destination types** (Artifactory, Nexus)
* **Configurable resource requirements** via CRD (not just hardcoded defaults)
* **Job retry logic** for transient failures
* **Progress reporting** for long-running operations
* **Pause/Resume** functionality for jobs
* **Multi-architecture** bundle builds
* **Webhook TLS certificate rotation** without downtime
* **Metrics dashboard templates** for Grafana

---

## Architecture Notes

**Recent Refactoring (Completed)**:

* ✅ Action handlers reorganized into `pkg/actions/zarf/` and `pkg/actions/uds/` packages
* ✅ Common job building utilities extracted to `pkg/actions/common/`
* ✅ Constants package created in `pkg/constants/` with proper organization
* ✅ Defensive container bounds checking added to all handlers that need it
* ✅ Hardcoded UIDs replaced with constants throughout action handlers
* ✅ Resource requirements standardized (CPU millicore notation, consistent values)

**Next Steps**:
1. Complete UDS source handler integration (use `pkg/sources` instead of inline Git)
2. Create `pkg/util/` package for shared helper functions
3. Consolidate ActionResult type into common package
4. Make all timeouts configurable via CRD fields
