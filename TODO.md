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

### ~~Missing Source Handler Integration~~ ✅ COMPLETED

* ~~**Integrate UDS actions with shared source handlers** ✅ COMPLETED - UDS now uses shared source handlers via `pkg/sources/uds_adapters.go` and `pkg/sources/git_common.go`~~
  * ~~Created `BuildGitInitContainer()` in `git_common.go` to eliminate duplication between Zarf and UDS~~
  * ~~Created `GetUDSInitContainer()` adapter in `uds_adapters.go` to convert UDS types to common config~~
  * ~~Refactored both Zarf and UDS to use shared Git init container logic with appropriate UIDs (1000 for Zarf, 65532 for UDS)~~
  * ~~Updated `pkg/actions/uds/create.go` to use `sources.GetUDSInitContainer()` instead of inline Git implementation~~
  * ~~Removed 79 lines of duplicated Git cloning logic from create.go (buildInitContainers)~~
  * ~~All tests passing: sources tests 100%, UDS actions tests 80.7% coverage~~

### Missing Defensive Checks

* ~~**Add container bounds checking to zarf/build.go** ✅ COMPLETED~~
* ~~**Add container bounds checking to uds/create.go** ✅ NOT NEEDED - No array access that requires defensive checks~~

**Note**: All action handlers now have proper defensive checks where needed.

---

## High Priority (Configuration & Code Quality)

### Constants Not Being Used

* ~~**Move CLI image constants to pkg/constants/config.go** ✅ COMPLETED - ZarfCLIImage and UDSCLIImage now in constants package~~

* ~~**Replace hardcoded UIDs with constants** ✅ COMPLETED - All Zarf (1000→DefaultZarfUID) and UDS (65532→DefaultUDSUID) actions updated~~

### ~~Timeout Handling Inconsistencies~~ ✅ COMPLETED

* ~~**Make Build action timeout configurable** ✅ COMPLETED - Added BuildConfig with Timeout field, handlers use time.ParseDuration~~
  * ~~Added `BuildConfig` struct to Zarf CRD with `Timeout` field~~
  * ~~Updated `pkg/actions/zarf/build.go` to check `pkg.Spec.Build.Timeout` with fallback to `constants.DefaultBuildTimeout`~~
  * ~~Uses `time.ParseDuration` for parsing with graceful fallback on invalid input~~

* ~~**Make Publish action timeout configurable** ✅ COMPLETED - Added Timeout field to PublishConfig~~
  * ~~Added `Timeout` field to both Zarf and UDS `PublishConfig` structs~~
  * ~~Updated `pkg/actions/zarf/publish.go` and `pkg/actions/uds/publish.go` to use configurable timeout~~
  * ~~Falls back to `constants.DefaultPublishTimeout` if not specified~~

* ~~**Make UDS Create action timeout configurable** ✅ COMPLETED - Added BundleCreateConfig~~
  * ~~Added `BundleCreateConfig` struct with `Timeout` field to UDS CRD~~
  * ~~Updated `pkg/actions/uds/create.go` to check `bundle.Spec.Create.Timeout`~~
  * ~~Falls back to `constants.DefaultCreateTimeout` (7200 seconds / 2 hours)~~

* ~~**Make UDS Publish action timeout configurable** ✅ COMPLETED~~
  * ~~Added `Timeout` field to `BundlePublishConfig` (default "1h")~~
  * ~~Updated handler to use time.ParseDuration with fallback to constants~~

* ~~**Standardize timeout parsing to use time.ParseDuration** ✅ COMPLETED~~
  * ~~Replaced custom `parseTimeout()` function in `pkg/actions/uds/deploy.go` with `time.ParseDuration`~~
  * ~~Removed 37 lines of manual parsing logic~~
  * ~~All action handlers now use consistent timeout parsing pattern~~
  * ~~Deleted TestParseTimeout since we now use standard library (well-tested)~~

### ~~Resource Requirement Inconsistencies~~ ✅ COMPLETED

* ~~**Fix Zarf Publish CPU request** ✅ COMPLETED - Changed from 250m to 200m for consistency with Build~~

* ~~**Convert UDS resource limits to millicore notation** ✅ COMPLETED - All UDS actions now use explicit millicore notation ("1000m", "2000m")~~

* ~~**Standardize resource requirements across similar operations** ✅ COMPLETED~~
  * ~~Build/Create now standardized at 500m/2000m CPU, 1Gi/4Gi memory (both create artifacts)~~
  * ~~Publish already standardized at 200m/1000m CPU, 512Mi/2Gi memory (network I/O focused)~~
  * ~~Deploy now standardized at 500m/2000m CPU, 1Gi/4Gi memory (both deploy to clusters)~~
  * ~~All handlers now include documentation comments explaining resource allocation reasoning~~
  * ~~Updated test expectations in uds_actions_test.go to match new standards~~

### Code Duplication

* ~~**Create shared helpers package and consolidate ActionResult** ✅ COMPLETED~~
  * ~~ActionResult moved to `pkg/actions/common/types.go` (shared by both Zarf and UDS)~~
  * ~~Ptr() and MustParseQuantity() already existed in common, consolidated all usages~~
  * ~~Removed duplicate types files: zarf/types.go, uds/types.go~~
  * ~~All Zarf and UDS handlers now use common.ActionResult~~
  * ~~All action handlers, source handlers, and tests updated to use common package~~

---

## Medium Priority (Testing & Documentation)

### Test Coverage Gaps

* **Improve UDS controller test coverage**
  * Location: `pkg/controller/uds_controller_test.go` (712 lines)
  * Issue: 21% fewer tests than Zarf controller (902 lines)
  * Add: Tests for edge cases, error conditions, and policy enforcement
  * Target: Match Zarf controller coverage

### Missing UDS Documentation

* ~~**Create dedicated UDS user guide** ✅ COMPLETED~~
  * ~~Created `docs/getting-started/UDS_GUIDE.md` with 738 lines of comprehensive documentation~~
  * ~~Complete examples for Create, Publish, Deploy operations (Git, OCI, S3 sources)~~
  * ~~Policy configuration with references to examples/policies/uds/ templates~~
  * ~~Troubleshooting section covering bundle creation, deployment, OCI, S3, and controller issues~~
  * ~~Side-by-side v1alpha1 vs v1alpha2 comparison table with migration examples~~
  * ~~Follows same structure and style as existing USER_GUIDE.md~~

* **Create UDS-specific troubleshooting guide**
  * Location: Create `docs/operations/UDS_TROUBLESHOOTING.md`
  * Include: Common error messages and solutions
  * Include: Debugging failed UDS jobs
  * Include: Policy validation failures
  * Include: Bundle creation issues

* ~~**Add UDS policy configuration examples** ✅ COMPLETED~~
  * ~~Created `examples/policies/uds/` directory with comprehensive examples~~
  * ~~permissive-serviceaccount.yaml - Development use, all permissions~~
  * ~~restricted-serviceaccount.yaml - Production use, least-privilege model~~
  * ~~ci-cd-serviceaccount.yaml - CI/CD pipeline, build & publish only~~
  * ~~README.md - Complete guide with policy annotations, RBAC requirements, troubleshooting~~
  * ~~Includes example Secrets for OCI, S3, and Git credentials~~

* ~~**Update CLAUDE.md with UDS vs Zarf guidance** ✅ COMPLETED~~
  * ~~Added comprehensive "UDS vs Zarf: When to Use Which" section to both CLAUDE.md and CLAUDE.local.md~~
  * ~~ASCII decision tree for choosing between UDS and Zarf based on single package vs bundle~~
  * ~~Complete examples for both ZarfPackageJob and UDSPackageJob (v1alpha2 + v1alpha1)~~
  * ~~Key differences table comparing 9 aspects (actions, CLI, resources, UIDs, etc.)~~
  * ~~API version guidance: v1alpha1 deprecated, v1alpha2 recommended~~
  * ~~Common patterns with complete YAML examples~~
  * ~~Implementation notes for developers (handler locations, policy schema)~~

### Naming Inconsistencies

* ~~**Standardize app label values** ✅ COMPLETED~~
  * ~~All jobs now use `"app": "forge"` for consistency~~
  * ~~Added `"resource-type": "zarfpackagejob"` to Zarf jobs~~
  * ~~Added `"resource-type": "udspackagejob"` to UDS jobs~~
  * ~~Updated all 6 action handlers (build, publish, deploy for both Zarf and UDS)~~
  * ~~Jobs can now be selected by `app=forge` for all jobs, or filtered by resource-type~~

* ~~**Align controller receiver variable names** ✅ COMPLETED~~
  * ~~Standardized all controller receivers to use `ctrl` for Go convention and brevity~~
  * ~~Updated Zarf controller files: `pkg/controller/controller.go` and `pkg/controller/job_monitor.go`~~
  * ~~UDS controller already used `ctrl`, so no changes needed there~~
  * ~~All method receivers and member access now consistent across both controllers~~

---

## Low Priority (Logging & Conventions)

### Logging Inconsistencies

* **Standardize metric naming**
  * Issue: UDS uses `RecordBundleCreateStarted`, Zarf uses `RecordBuildStarted`
  * Decision needed: Either add "Package" prefix to Zarf OR drop "Bundle" from UDS
  * Files: `pkg/telemetry/metrics.go` and all action handlers
  * Recommendation: `RecordPackageBuildStarted` and `RecordBundleCreateStarted` for clarity

* ~~**Standardize log message format** ✅ COMPLETED~~
  * ~~All log messages now include resource type for clarity~~
  * ~~Zarf: "Executing Zarf Package Build action", "Zarf package build job created"~~
  * ~~UDS: "Executing UDS Bundle Create action", "Bundle create job created"~~
  * ~~Consistent pattern across all 6 action handlers (build, publish, deploy for both)~~

* ~~**Document klog verbosity level conventions** ✅ COMPLETED~~
  * ~~Created comprehensive `docs/development/LOGGING.md` with full logging standards~~
  * ~~Defined: V(0) = essential, V(2) = informational, V(4) = debug, V(6+) = reserved~~
  * ~~Documented structured logging patterns, field naming conventions, and best practices~~
  * ~~Included examples, troubleshooting guidance, and kubectl filtering techniques~~
  * ~~Current usage already follows conventions (V(2) for reuse, V(4) for debug)~~

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
* ✅ ActionResult type consolidated in `pkg/actions/common/types.go` (eliminates duplication)
* ✅ CLI image constants moved to `pkg/constants/config.go`
* ✅ All helper functions (Ptr, MustParseQuantity) consolidated in common package

**Next Steps**:
1. Complete UDS source handler integration (use `pkg/sources` instead of inline Git)
2. Make all timeouts configurable via CRD fields
3. Standardize timeout parsing to use time.ParseDuration
4. Standardize resource requirements across similar operations
