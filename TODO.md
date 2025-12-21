# TODO

## Existing Issues

### Testing Infrastructure
* **make test-validation** - YAML validation tests need review
  - Location: `pkg/validation/`
  - Action: Review and ensure all config files are properly validated

* **make e2e-test** - E2E tests need completion
  - Location: `scripts/test-e2e.sh`
  - Action: Complete end-to-end test coverage for all workflows

* **OTLP test skips** - Some tests skipped for OTLP, might be removed
  - Location: `pkg/telemetry/` tests
  - Action: Review skipped telemetry tests and decide to fix or remove

* **Job state bouncing** - Job bouncing between Running/Completed
  - Location: `pkg/controller/job_monitor.go`, `pkg/controller/uds_job_monitor.go`
  - Action: Investigate and fix job state reconciliation logic

* **Cleanup/Consolidate Examples** - Examples folder is messy
  - Location: `examples`
  - Action: Investigate, cleanup, order, and consolidate examples

* **Cleanup/Consolidate Makefile** - Makefile is messy
  - Location: `Makefile`
  - Action: Investigate, cleanup, order, and consolidate Makefile

---

## Critical Inconsistencies (Fix Immediately)

### Missing Source Handler Integration
* **Integrate UDS actions with shared source handlers**
  - Location: `pkg/actions/uds/create.go:204-241`
  - Issue: UDS actions implement inline Git cloning instead of using `pkg/sources`
  - Pattern: Use `sources.New()` and `sourceHandler.GetInitContainer()` like Zarf does
  - Files to update: `create.go`, `publish.go`, `deploy.go` in `pkg/actions/uds/`
  - Remove: Inline Git clone implementation from init containers

### Missing Defensive Checks
* **Add container bounds checking to build.go**
  - Location: `pkg/actions/build.go`
  - Add: Check `len(job.Spec.Template.Spec.Containers) == 0` before accessing `Containers[0]`
  - Pattern: Match defensive checks in `deploy.go:322-326`

* **Add container bounds checking to uds/create.go**
  - Location: `pkg/actions/uds/create.go`
  - Add: Check `len(job.Spec.Template.Spec.Containers) == 0` before accessing `Containers[0]`
  - Pattern: Match defensive checks in `uds/deploy.go:244-248`

---

## High Priority (Configuration & Code Quality)

### Constants Not Being Used
* **Move CLI image constants to pkg/constants/config.go**
  - Current: `ZarfCLIImage` in `pkg/actions/build.go:24-25`
  - Current: `UDSCLIImage` in `pkg/actions/uds/types.go:13-14`
  - Target: `pkg/constants/config.go`
  - Update all references in action handlers

* **Replace hardcoded UIDs with constants**
  - Replace: `RunAsUser: 1000` → `constants.DefaultZarfUID` in all Zarf actions
    - Files: `build.go:149`, `publish.go:142`, `deploy.go:144`
  - Replace: `RunAsUser: 65532` → `constants.DefaultUDSUID` in all UDS actions
    - Files: `uds/create.go:137`, `uds/publish.go:128`, `uds/deploy.go:137`

### Timeout Handling Inconsistencies
* **Make Build action timeout configurable**
  - Location: `pkg/actions/build.go:100`
  - Current: Hardcoded 3600s
  - Add: Support for `Spec.Build.Timeout` field in CRD
  - Use: `constants.DefaultBuildTimeout` as default

* **Make Publish action timeout configurable**
  - Location: `pkg/actions/publish.go:97`
  - Current: Hardcoded 1800s
  - Add: Support for `Spec.Publish.Timeout` field in CRD
  - Use: `constants.DefaultPublishTimeout` as default

* **Make UDS Create action timeout configurable**
  - Location: `pkg/actions/uds/create.go:87`
  - Current: Hardcoded 7200s
  - Add: Support for `Spec.Create.Timeout` field in CRD
  - Use: `constants.DefaultUDSBundleCreateTimeout` as default

* **Make UDS Publish action timeout configurable**
  - Location: `pkg/actions/uds/publish.go:84`
  - Current: Hardcoded 3600s
  - Add: Support for `Spec.Publish.Timeout` field in CRD
  - Use: `constants.DefaultUDSBundlePublishTimeout` as default

* **Standardize timeout parsing to use time.ParseDuration**
  - Location: `pkg/actions/uds/deploy.go:282-318`
  - Issue: Custom parsing logic instead of standard library
  - Replace: With `time.ParseDuration` like `pkg/actions/deploy.go:89`

### Resource Requirement Inconsistencies
* **Fix Zarf Publish CPU request**
  - Location: `pkg/actions/publish.go:263`
  - Current: 250m (inconsistent with Build's 200m)
  - Change: 250m → 200m
  - Fix: Comment on line 254 that incorrectly says "slightly less than build"

* **Convert UDS resource limits to millicore notation**
  - Location: `pkg/actions/uds/create.go:256`, `publish.go:324`, `deploy.go:332`
  - Current: Shorthand notation ("1", "2")
  - Change: "1" → "1000m", "2" → "2000m"
  - Reason: Consistency with Zarf actions

* **Standardize resource requirements across similar operations**
  - Build/Create should have same resources (both create artifacts)
  - Publish should have same resources across Zarf and UDS
  - Deploy should have same resources across Zarf and UDS
  - Document reasoning in comments

### Code Duplication
* **Create shared helpers package**
  - Create: `pkg/util/helpers.go`
  - Move: `ptr[T any](v T) *T` function (duplicated in 3 places)
  - Move: `mustParseQuantity(quantityStr string) resource.Quantity` function

* **Remove ptr() duplication from pkg/actions/types.go**
  - Location: `pkg/actions/types.go:38-42`
  - Replace: With import from `pkg/util`

* **Remove ptr() duplication from pkg/actions/uds/types.go**
  - Location: `pkg/actions/uds/types.go:47-51`
  - Replace: With import from `pkg/util`

* **Remove ptr() duplication from pkg/sources/types.go**
  - Location: `pkg/sources/types.go:32-34`
  - Replace: With import from `pkg/util`

* **Remove mustParseQuantity() duplication**
  - Location: `pkg/actions/types.go:44-47` and `pkg/actions/uds/types.go:53-56`
  - Replace: With import from `pkg/util`

* **Consolidate ActionResult type**
  - Location: Duplicated in `pkg/actions/types.go:8-33` and `pkg/actions/uds/types.go:17-42`
  - Options:
    1. Share single type in `pkg/util/types.go`
    2. Use generic type with package-specific aliases
    3. Keep separate but document why

---

## Medium Priority (Testing & Documentation)

### Test Coverage Gaps
* **Improve UDS controller test coverage**
  - Location: `pkg/controller/uds_controller_test.go` (712 lines)
  - Issue: 21% fewer tests than Zarf controller (902 lines)
  - Add: Tests for edge cases, error conditions, and policy enforcement
  - Target: Match Zarf controller coverage

### Missing UDS Documentation
* **Create dedicated UDS user guide**
  - Location: Create `docs/getting-started/UDS_GUIDE.md`
  - Include: Complete examples of Create, Publish, Deploy operations
  - Include: Policy configuration for UDS bundles
  - Include: Troubleshooting common issues

* **Create UDS-specific troubleshooting guide**
  - Location: Create `docs/operations/UDS_TROUBLESHOOTING.md`
  - Include: Common error messages and solutions
  - Include: Debugging failed UDS jobs
  - Include: Policy validation failures

* **Add UDS policy configuration examples**
  - Location: `examples/policies/uds/`
  - Create: Example ServiceAccount annotations for UDS bundles
  - Create: Example RBAC configurations
  - Create: Example restricted vs permissive policies

* **Update CLAUDE.local.md with UDS vs Zarf guidance**
  - Location: `CLAUDE.local.md:31`
  - Add: Clear decision tree for when to use UDS vs Zarf
  - Add: UDS-specific examples and patterns
  - Add: UDS bundle structure and requirements

### Naming Inconsistencies
* **Standardize app label values**
  - Current: Zarf jobs use `"app": "forge"`, UDS jobs use `"app": "forge-uds"`
  - Target: Both use `"app": "forge"` with additional `"resource-type": "zarfpackagejob"` or `"resource-type": "udsbundlejob"`
  - Files: All action handlers in `pkg/actions/` and `pkg/actions/uds/`

* **Align controller receiver variable names**
  - Location: `pkg/controller/controller.go` (uses `controller`) vs `pkg/controller/uds_controller.go` (uses `ctrl`)
  - Decision: Standardize on one (prefer `ctrl` for brevity)
  - Update: All method receivers and local variables

---

## Low Priority (Logging & Conventions)

### Logging Inconsistencies
* **Standardize metric naming**
  - Issue: UDS uses `RecordBundleCreateStarted`, Zarf uses `RecordBuildStarted`
  - Decision: Either add "Package" prefix to Zarf OR drop "Bundle" from UDS
  - Files: `pkg/telemetry/metrics.go` and all action handlers

* **Standardize log message format**
  - Issue: UDS prefixes with "UDS Bundle", Zarf doesn't use "Zarf Package"
  - Decision: Consistent prefix strategy across both
  - Files: All `klog.InfoS()` calls in action handlers

* **Document klog verbosity level conventions**
  - Location: Add to `docs/development/LOGGING.md` (create if needed)
  - Define: V(2) = info, V(4) = debug, V(6) = trace
  - Update: All klog calls to follow convention
  - Current usage: V(2) for "reusing job", V(4) for timeout errors (inconsistent)

---

## Future Enhancements (Not Inconsistencies)

* **Support for additional source types** (Azure DevOps, GitLab, Bitbucket)
* **Support for additional destination types** (Artifactory, Nexus)
* **Configurable resource requirements** via CRD (not just hardcoded)
* **Job retry logic** for transient failures
* **Progress reporting** for long-running operations
* **Pause/Resume** functionality for jobs
* **Multi-architecture** bundle builds
