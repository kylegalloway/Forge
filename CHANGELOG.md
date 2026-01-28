# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.11.2] - 2026-01-28

### Fixed
- Release script now updates `zarfCLI` and `udsCLI` image tags in values.yaml during version bumps, ensuring all four Forge images (controller, webhook, zarfpackagejob, udsbundlejob) are versioned consistently

## [0.11.1] - 2026-01-28

### Added
- Complete API reference YAML files in `examples/reference/` documenting all available fields:
  - `zarfpackagejob-reference.yaml` - All ZarfPackageJob spec fields with valid values
  - `udsbundlejob-reference.yaml` - All UDSBundleJob spec fields with valid values
  - `serviceaccount-reference.yaml` - All policy annotations with examples

### Fixed
- UDS example YAML files now use correct PascalCase for action types (`CreatePublish`, `CreateDeploy`) and source types (`Git`, `Local`, `S3`, `OCI`) to match API enum definitions
- UDS policy ServiceAccount examples now use correct annotation prefix `forge.dev/` instead of erroneous `forge.forge.dev/`
- USER_GUIDE.md documentation corrected: annotation prefix fixed, external cluster config field changed from `kubeconfig:` to `externalCluster:`, removed outdated v1alpha1/v1alpha2 migration section

### Changed
- Removed completed plan files from `docs/plans/`: debug-mode.md, cli-flags-passthrough.md, security-kyverno-compliance.md, Kubeconfig-create.md (all implemented)
- Updated `docs/README.md` with complete documentation index including Security section, Planning section, and missing development docs (GITEA_TESTING.md, STRUCTURED-LOGGING.md)

## [0.11.0] - 2026-01-28

### Fixed
- OCI source init containers for Zarf jobs now run as UID 1000 (DefaultZarfUID) instead of UID 65532 (DefaultUDSUID). Previously, files extracted by the OCI init container were owned by a different UID than the main Zarf container, which could cause permission issues if the main container needed to modify source files.
- UDS bundle create command no longer passes unsupported `--output-directory` flag to UDS CLI. The bundle is created in the workspace and moved to the target directory.
- UDS bundle sample `03-git-build-deploy/uds-bundle.yaml` now uses `path` instead of `repository` for local zarf package references, matching UDS CLI requirements.
- In-cluster kubeconfig generation now embeds the service account token directly and uses `KUBERNETES_SERVICE_HOST`/`KUBERNETES_SERVICE_PORT` environment variables for the API server endpoint. This fixes deploy jobs failing in clusters with restrictive DNS configurations or when `tokenFile` paths have permission issues.

### Added
- E2E tests for UDS bundle create and deploy operations (`04-uds-create`, `05-uds-deploy`).

- Per-job `debugMode` field in ZarfPackageJobSpec and UDSBundleJobSpec CRDs
- Per-action `debugActions` field for fine-grained control in chained workflows (e.g., debug only `build` in a `BuildPublish` job)
- `GetDebugMode()` and `GetDebugActions()` methods on PackageResource interface for unified debug mode access
- `ShouldDebugAction()` helper function to determine if a specific action should run in debug mode
- Debug completion marker pattern (`/tmp/debug-complete`) allowing debug pods to exit gracefully and continue chained workflows
- Comprehensive debug logging in GenericJobMonitor with correlation IDs, timing, and detailed status information
- Unit tests for JobBuilder debug mode behavior and `ShouldDebugAction` logic in job_builder_test.go
- Debug mode documentation in USER_GUIDE.md with examples for `debugMode` and `debugActions`
- Debug mode troubleshooting section in TROUBLESHOOTING.md
- Debug mode precedence: `debugActions` > `spec.debugMode` > global `FORGE_DEBUG_MODE` env var

### Changed
- Debug pods now use completion marker script instead of `sleep infinity`, enabling chained workflow continuation
- Action handlers now use `ShouldDebugAction()` for per-action debug control
- JobBuilder sets TTLSecondsAfterFinished to 1 hour (3600s) when debug mode is enabled
- Webhook validators now emit detailed debug logs at V(4) with correlation IDs, timing, and policy decisions
- GenericController now emits debug logs with correlation IDs, timing, and action dispatch decisions
- GenericJobMonitor now emits debug logs for job status checks, retry policy evaluation, action chaining, and cleanup operations

## [0.10.0] - 2026-01-27

### Added
- GitHub artifact attestations via `actions/attest-build-provenance@v2` for all container images and binaries. Attestations are stored in GitHub's attestation store and verifiable with `gh attestation verify`. This complements existing Cosign signatures with GitHub-native supply chain security.
- AWS CLI added to zarfpackagejob and udsbundlejob container images, enabling S3 publishing destinations. Jobs can now publish packages and bundles directly to S3 buckets using `aws s3 cp`.
- kubectl added to zarfpackagejob and udsbundlejob container images for in-cluster kubeconfig generation and debugging.
- In-cluster kubeconfig generation for deploy jobs. Zarf/UDS CLIs don't auto-detect in-cluster mode, so deploy handlers now generate a kubeconfig from the pod's service account token.
- AWS credentials documentation (`docs/operations/AWS_CREDENTIALS.md`) covering static credentials and IRSA configuration for S3 access.
- CI pipeline now validates generated code (CRDs and deepcopy) is up-to-date via `verify-codegen` job. PRs will fail if `make manifests generate` produces changes not committed to the repository. Generation runs goimports to normalize import formatting.

### Fixed
- Container image tags now consistently use "v" prefix (e.g., `v0.9.13` instead of `0.9.13`) across all release artifacts. The release workflow's docker/metadata-action was stripping the "v" prefix, causing helm charts and zarf.yaml to reference non-existent image tags.
- In-cluster deployments now work correctly. Previously, deploy jobs would fail with kubeconfig errors because no KUBECONFIG was set for in-cluster targets.

## [0.9.13] - 2026-01-27

### Added
- PascalCase spec action constants (`SpecActionBuild`, `SpecActionPublish`, `SpecActionDeploy`, etc.) in `pkg/constants/actions.go` for matching CRD enum values
- Comprehensive unit tests for `determineNextAction()` and `isMultiActionJob()` functions covering all compound action chains (Zarf: BuildPublish, BuildDeploy, BuildPublishDeploy; UDS: CreatePublish, CreateDeploy, CreatePublishDeploy; Shared: PublishDeploy)
- Debug mode enhancement plan document outlining per-job debug flag, enhanced webhook/controller logging, and debug pod behavior improvements
- Release script now updates `chart/forge/values.yaml` image tags (controller and webhook) to explicit versions instead of empty strings
- Release script now manages CHANGELOG.md automatically (moves Unreleased to new version section, updates comparison links)
- Git hooks tracked in `scripts/hooks/` for reproducible developer setup:
  - `commit-msg`: Validates commit messages (emoji required, body required, no boring prefixes, no AI attribution)
  - `pre-commit`: Portable pre-commit framework invocation (no hardcoded paths)
  - `prepare-commit-msg`: Enforces signed commits
- CONTRIBUTING.md updated with "Install Git Hooks" section and commit message requirements

### Fixed
- Action chaining case sensitivity bug where compound actions like `BuildPublish` (PascalCase from spec.action) failed to chain because they were compared against string concatenations like `buildPublish` (lowercase + PascalCase). All action dispatch and chaining logic now uses typed constants ensuring consistent comparison.
- Commit-msg hook now accepts both 3-byte (`\xE2`) and 4-byte (`\xF0`) UTF-8 emojis, allowing ✨ ♻️ ⚡ ⬆️ ⬇️ alongside 🔧 🐛 📝 🚀

## [0.9.12] - 2026-01-26

### Fixed
- Corrected StatusKeyState constant in `pkg/constants/status.go` from "state" to "phase" to match the OperationStatus struct JSON field definition. This resolves action chaining failures where Kubernetes was rejecting field updates with "unknown field status.buildStatus.state" errors. All status update operations now properly reference the correct field name across all action handlers.

## [0.9.11] - 2026-01-26

### Added
- Debug mode support for job pods with configurable `debugMode` Helm value (default: false)
- FORGE_DEBUG_MODE environment variable passed to controller deployment
- WithDebugMode() method added to JobBuilder that replaces command args with "sleep infinity"
- WithUserConfig() helper method that consolidates UID, home directory, and security context setup in JobBuilder

### Changed
- Deprecated WithHomeDir() method in favor of new WithUserConfig() which provides more comprehensive user configuration
- Home directory now added as emptyDir volume mount with 1Gi size limit

### Fixed
- Job pod debugging is now possible - pods sleep indefinitely instead of terminating immediately after execution, allowing users to `kubectl exec` for inspection

## [0.9.10] - 2026-01-26

### Fixed
- Corrected Helm template variable reference in controller and webhook deployments from `.Chart.AppVersion` to `.Chart.version`. The AppVersion field is separate from the version field that triggers releases, causing containers to fail version detection. This was a template field mapping error where the code and release tags were using different version sources.

## [0.9.9] - 2026-01-26

### Fixed
- Corrected Helm template variable reference in controller and webhook deployments from `.Chart.AppVersion` to `.Chart.version`. The templates were looking at the wrong metadata field, preventing containers from identifying their own version

## [0.9.8] - 2026-01-26

### Added
- Added /tmp emptyDir volume mount to all init containers (git, s3, oci) for temporary file storage

### Fixed
- Init containers now have writable /tmp directories for credential setup (removing ReadOnlyRootFilesystem: true from git, s3, and oci init containers)
- Init containers can now write SSH keys (~/.ssh/id_rsa) and git credentials (~/.git-credentials) without filesystem restrictions
- Kyverno policy compatibility improved by properly mounting emptyDir volumes for all writable paths

## [0.9.7] - 2026-01-23

### Added
- Variable support in Zarf and UDS package operations via CreateConfig and DeployConfig
- CLI flags passthrough to `zarf create` and `uds create` commands via --set flag support
- ExtraArgs array support in CreateConfig and DeployConfig for unsupported flags
- Flavor, Architecture, and SkipSBOM options added to CreateConfig for advanced package creation
- Insecure TLS skip option and Retries configuration added to DeployConfig
- Comprehensive validation for flag formats and shell metacharacter protection (ShellCheck integration)
- CRD updates for v1alpha3 with new fields

### Changed
- Action handlers now properly support --set flag passthrough
- Variable validation implemented with comprehensive test coverage

## [0.9.6] - 2026-01-23

### Added
- Init container image version constants with automatic versioning support
- Size limits for workspace and output emptyDir volumes (10Gi each) for Kyverno compliance
- Size limit for /tmp emptyDir volume (1Gi) for Kyverno and Pod Security Standards compliance
- Kyverno and PSS (Pod Security Standards) compliance documentation
- Conditional automountServiceAccountToken to disable by default and only enable when service account specified

### Changed
- ReadOnlyRootFilesystem: true enforced in NonRootSecurityContextWithUID() for security hardening
- kubectl-forge debug pod security compliance improvements
- Security context enhancements for Helm templates

### Fixed
- The gitignore was a little too enthusiastic about kubectl-forge - removed .gitignore rule that prevented kubectl-forge binary from being packaged in charts
- S3 destinations now support multiple authentication methods beyond AWS (credential polymorphism)

## [0.9.5] - 2026-01-23

### Added
- Comprehensive kubectl-forge CLI enhancement package: cancel, colors, debug, diagnose, get_events, get_job, get_logs, get_pods, get, list, logs, retry, status, and util commands
- kubectl-forge diagnose command with comprehensive job troubleshooting capabilities:
  - Automatic problem detection (OOMKilled, CrashLoopBackOff, ImagePullBackOff, scheduling issues)
  - Event aggregation and filtering with warning detection
  - Container log collection and display
  - Interactive troubleshooting suggestions
  - Multiple output formats (table, JSON, YAML)
- kubectl-forge retry command for re-running failed jobs
- kubectl-forge cancel command for stopping running jobs
- Comprehensive structured logging support across controller and webhook
- Kubernetes pod security standards documentation and implementation
- RBAC permissions reference guide for cluster-wide and namespace-scoped deployments
- Example demonstrating retainArtifactPVC functionality
- Credential examples for diverse authentication methods

### Changed
- Job pods implement Kyverno appeasement security context with readonly root filesystem
- Release workflow improved to 'latest' tag handling
- kubectl-forge now fully integrated as kubectl plugin

### Fixed
- kubectl-forge binary properly included in Helm chart with security compliance

## [0.9.4] - 2026-01-22

### Added
- Variable support in Zarf and UDS action operations with Variables map in BuildConfig, CreateConfig, PublishConfig, and DeployConfig
- Variables passed to zarf and uds CLI commands via --set KEY=VALUE flags
- Comprehensive test coverage for variable handling across UDS and Zarf actions
- CRD field additions for variable support (v1alpha3 types)

## [0.9.3] - 2026-01-21

### Added
- Comprehensive Gitea testing documentation and examples
- Root Cause Analysis documentation for git credential mounting failures
- Enhanced credential example showcase with diverse authentication methods
- S3 destination support for multiple credential types and regions
- Support for credential polymorphism across different backend types

### Changed
- Git credential handling now supports any git server (GitHub, GitLab, Bitbucket, private instances)
- Git URL parsing enhanced to extract both scheme and host for proper credential routing
- Credential manager improvements with support for multiple authentication schemes:
  - SSH key authentication for SSH URLs
  - Token/password authentication for HTTPS URLs
  - OAuth2 token support with proper username handling
  - URL encoding of special characters in credentials
- CRD updates with credential field enhancements (v1alpha3 types)
- Pre-commit hooks configuration improvements

### Fixed
- Git credentials no longer assume everyone uses SSH keys (OAuth-flavored prison escaped)
- Git URLs now properly support HTTP/HTTPS authentication

## [0.9.2] - 2026-01-21

### Added
- Enhanced git credential mounting with support for multiple git servers
- extractGitHost() function for parsing git URLs (both HTTPS and SSH formats)
- URL scheme extraction for proper credential routing
- Test coverage for git URL parsing and credential handling

### Changed
- Git credential configuration now dynamically reads the host from the repository URL instead of hardcoding github.com/gitlab.com
- Improved shell script templating for credential setup

### Fixed
- Git credentials no longer hardcoded to specific servers - now routes to actual host

## [0.9.1] - 2026-01-21

### Added
- WithHomeDir() method to JobBuilder for setting HOME environment variable
- Home directory path constants (HomePathZarf, HomePathUDS, HomePathTmp) in constants/config.go
- Credential mounting support across all source and destination handlers (git, s3, oci)
- HOME environment variable configuration for all action types (build, deploy, publish, create)

### Changed
- All action handlers now explicitly set HOME directory for credential access
- Improved credential routing across git, OCI, and S3 handlers

### Fixed
- Credential mounts delivered to correct addresses at last
- Action handlers now properly configure HOME directory during initialization
- Non-root users can now write configuration files to proper home directories

## [0.9.0] - 2026-01-20

### Added
- Credential volume mounting infrastructure for all source types (git, s3, oci)
- GetGitCredentialVolume() function for proper credential secret mounting
- Volume name and mount path constants for credential handling
- Multi-environment build support with proper credential routing
- Comprehensive test coverage for credential volumes

### Changed
- All action handlers now support credential references with proper volume mounting
- Release workflow significantly refactored (681 lines changed in release.yaml)
- Builds can now access credentials from mounted secrets across all source handlers
- UDS create and deploy handlers enhanced with credential support

### Fixed
- Missing credential volumes added to all source handlers (git, oci, s3)
- Credential secret references now properly mounted in init containers
- Multi-action workflows can now access credentials stored in Kubernetes secrets

## [0.8.1] - 2026-01-20

### Added
- Comprehensive credential showcase examples for both Zarf and UDS:
  - 04-credentials-showcase for UDS (467 lines of example configs)
  - 05-credentials-showcase for Zarf (412 lines of example configs)
  - Full documentation of credential management patterns
- AWS credential polymorphism with multiple loading methods:
  - EnvVar: Load credentials from secret keys as environment variables
  - File: Mount AWS credentials file from secret
  - Node: Use node-level credentials (IRSA, instance profile)
- AWSCredentialRef type with configurable credential type and namespace support
- S3 destination enhanced with AWS credential flexibility
- UDS adapters improved for credential handling
- Source handlers (git, oci, s3) updated with credential support

### Changed
- All action handlers (build, deploy, publish, create) now support AWS credentials
- S3 destination significantly refactored to support multiple credential types
- Credential reference types expanded to support diverse authentication patterns

## [0.8.0] - 2026-01-20

### Added
- Centralized constants package for action names, status values, and configuration:
  - ActionBuild, ActionPublish, ActionDeploy, ActionCreate constants
  - Status state constants with standardized naming (Running, Succeeded, Failed, etc.)
  - ServiceAccount annotation keys, job/pod labels
  - API group versions for dynamic client operations
  - Container image and configuration constants
- Running status consensus - everyone agrees what "Running" means now
- Shared resource grouping infrastructure

### Changed
- All controllers, handlers, webhooks, and monitoring now use centralized constants
- Job status handling standardized across all action types
- Significantly refactored JobBuilder to support new patterns
- UDS create and deploy handlers refactored with new action framework
- v1alpha3 API validation and label handling improved
- CRD definitions optimized with schema improvements
- V1alpha2 migration guide removed (v1alpha3 is now standard)

### Fixed
- Action handlers unmask their true labels with proper constant usage
- Controllers, webhook, and CLI all speak the common tongue of constants
- Status updates consistent across all component types

## [0.7.3] - 2026-01-16

### Changed
- Three AWS dialects walk into a bar, leave speaking the same language

## [0.7.2] - 2026-01-16

### Fixed
- macOS bash 3.2 compatibility improvements

## [0.7.1] - 2026-01-16

### Added
- Release workflow improvements and forensic capabilities
- kubectl-forge debugging enhancements

## [0.7.0] - 2026-01-15

### Added
- UDS CLI container image support with dedicated Dockerfile
- PVC (PersistentVolumeClaim) support and comprehensive examples
- UDS CLI versioning and auto-detection infrastructure
- Bundle CLI feature alongside Zarf CLI support
- update-tool-versions.sh script for automated tool version updates (updates Dockerfile ARGs only)
- V1alpha2 migration guide documentation for API users
- V1alpha2 and V1alpha1 API types for backward compatibility

### Changed
- Helm chart repository documentation improved
- Container image version management automated
- Generic controller and monitor improvements with better configuration handling
- kubectl client enhanced for image pulling and version detection
- Release workflow updated for multi-version package support
- Deployment documentation improved with additional deployment patterns

### Fixed
- Dockerfile versioning automation
- Version detection for multiple container runtimes
- PVC retention policy handling

## [0.6.2] - 2026-01-15

### Changed
- Release workflow enhancements for better commit message handling
- gh-pages deployment process improvements

### Fixed
- gh-pages commit enhancements with proper body content
- Release commit message formatting

## [0.6.1] - 2026-01-15

### Changed
- zarf.yaml packaging now integrated into release factory workflow

## [0.6.0] - 2026-01-15

### Added
- Zarf CLI container image support with automated building in CI/CD pipeline
- Container image versioning and registry management (ghcr.io)
- Automated image publishing to container registries

### Changed
- Registry changed to ghcr.io for improved accessibility
- Release workflow updated to handle container images alongside Helm charts

## [0.5.0] - 2026-01-07

### Added
- Unified attestation workflow for consistent artifact signing and verification

### Changed
- gh-pages release process improved and consolidated
- Attestation generation consolidated into single workflow

## [0.4.6] - 2026-01-06

### Added
- Node selector, affinity, and tolerations support for pod scheduling control
- Ephemeral storage configuration and resource scheduling
- kubectl-forge CLI tool with multiple commands:
  - debug: Debug pods from jobs
  - download: Download artifacts from jobs
  - list: List all Forge jobs
  - And more subcommands for job management
- kubectl-forge README and comprehensive documentation
- Extensive CRD updates with scheduler field support
- Advanced Kubernetes scheduling capabilities

### Changed
- JobBuilder enhanced with scheduling support
- GoMod dependencies updated with new tooling support
- CI/CD workflow improvements

### Fixed
- Pod scheduling now supports node affinity and pod anti-affinity rules

### Fixed
- Apollo 13 moment: we have a link problem (now solved)

## [0.4.5] - 2026-01-05

### Added
- Webhook high availability configuration for production deployment
- Production-ready webhook setup with multiple replicas
- Structured logging implementation across controller and webhook
- Comprehensive security documentation
- PVC (PersistentVolumeClaim) retention policy with optional automatic cleanup after jobs
- Trivy and Grype image scanning integration for vulnerability scanning
- RBAC documentation for both cluster-wide and namespace-scoped deployments
- Validator enhancements for forensic evidence collection

### Changed
- Webhook deployment refactored for HA setup
- Structured logging adopted throughout system
- Resource adoption now fully idempotent
- Logging improvements with structured output support

### Fixed
- Webhook now supports multiple replicas for high availability
- Resource adoption idempotency improved

## [0.4.4] - 2026-01-04

### Added
- CRD allowDangerousTypes field support for flexible object handling
- Apollo 13 problem solving for complex field compatibility issues

### Fixed
- Action type recognition properly implemented across all handlers
- Fixed unrecognized action type errors in controllers

## [0.4.3] - 2026-01-02

### Changed
- Release script refactored for documentation discovery in new repository structure
- Test suite significantly refactored
- Documentation reorganized by reader context and use case
- Schrödinger's controller finally observed in tests with error handling validation

### Fixed
- Release script now finds docs in reorganized structure

## [0.4.2] - 2025-12-30

### Added
- Retry policies integrated into API with exponential backoff configuration
- Resource adoption capabilities with proper Kubernetes ownership semantics
- Advanced retry configuration with configurable delays and attempt limits

### Changed
- Generics implementation reducing codebase complexity
- DRY principle applied to eliminate code duplication across handlers
- k8s v0.35 upgrade from EOL v0.28
- golangci-lint v2 format migration with updated configuration

### Fixed
- Kubernetes client updated to supported version
- Linting configuration modernized for v2.x compatibility

## [0.4.1] - 2025-12-26

### Changed
- Banished "common" packages from the codebase (package restructuring)
- golangci-lint upgraded to latest version
- Code organization improvements

### Fixed
- Helm's trust issues with directories vs files (proper path handling)

## [0.4.0] - 2025-12-26

### Added
- Testing infrastructure with validation and e2e test frameworks
- UDS troubleshooting guide for production support
- Unified action architecture eliminating code duplication
- ActionResult unification across all action handlers
- CRD regeneration with enhanced capabilities and v1alpha3 support

### Changed
- Testing phoenix rises from the ashes (validation and e2e tests reborn)
- Comprehensive tech debt discovery and cataloging
- Code architecture refactored for consistency and maintainability
- v1alpha1 API deprecated in favor of v1alpha2 migration

### Fixed
- Zarf version upgrade (v0.66.0 → v0.68.1)
- Kubernetes client initialization improvements

## [0.3.0] - 2025-12-25

### Added
- Security improvements with RBAC patches and authentication enhancements
- Comprehensive tech debt discovery, cataloging, and remediation
- Testing and validation infrastructure (both unit and integration)
- Pod security standards documentation

### Changed
- High-priority consistency improvements across codebase
- Security audit completion with documented findings

### Fixed
- Multiple RBAC security vulnerabilities addressed
- Security context properly implemented in controllers and webhooks

## [0.2.0] - 2025-12-18

### Added
- UDS controller support and integration
- Helm chart publication workflow (v0.1.1 and beyond)
- Initial Helm repository infrastructure on GitHub Pages
- Webhook support for validation

## [0.1.2] - 2025-12-07

### Added
- RBAC revelation and UDS controller infrastructure rise
- Webhook unification framework
- Initial architectural planning documents

## [0.1.1] - 2025-12-18

### Changed
- Minor improvements and fixes
- Helm repository enhancements

## [0.1.0] - 2025-11-20

### Added
- Initial Forge project creation with Kubernetes controller framework
- GitHub Actions and GitLab CI/CD automation pipelines
- Pre-commit hooks infrastructure for code quality
- Multi-architecture Dockerfile support (amd64, arm64 detection)
- Kubernetes controller infrastructure with reconciliation logic
- Basic webhook support with validation capabilities
- High availability deployment patterns
- Helm chart for easy deployment to Kubernetes clusters
- Custom Resource Definition (CRD) support for ZarfPackageJob and UDSBundleJob

### Changed
- Initial project setup and build infrastructure
- CI/CD pipeline configuration for automated testing and releases

[Unreleased]: https://github.com/kylegalloway/forge/compare/v0.11.2...HEAD
[0.11.2]: https://github.com/kylegalloway/forge/compare/v0.11.1...v0.11.2
[0.11.1]: https://github.com/kylegalloway/forge/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/kylegalloway/forge/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/kylegalloway/forge/compare/v0.9.13...v0.10.0
[0.9.13]: https://github.com/kylegalloway/forge/compare/v0.9.12...v0.9.13
[0.9.12]: https://github.com/kylegalloway/forge/compare/v0.9.11...v0.9.12
[0.9.11]: https://github.com/kylegalloway/forge/compare/v0.9.10...v0.9.11
[0.9.10]: https://github.com/kylegalloway/forge/compare/v0.9.9...v0.9.10
[0.9.9]: https://github.com/kylegalloway/forge/compare/v0.9.8...v0.9.9
[0.9.8]: https://github.com/kylegalloway/forge/compare/v0.9.7...v0.9.8
[0.9.7]: https://github.com/kylegalloway/forge/compare/v0.9.6...v0.9.7
[0.9.6]: https://github.com/kylegalloway/forge/compare/v0.9.5...v0.9.6
[0.9.5]: https://github.com/kylegalloway/forge/compare/v0.9.4...v0.9.5
[0.9.4]: https://github.com/kylegalloway/forge/compare/v0.9.3...v0.9.4
[0.9.3]: https://github.com/kylegalloway/forge/compare/v0.9.2...v0.9.3
[0.9.2]: https://github.com/kylegalloway/forge/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/kylegalloway/forge/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/kylegalloway/forge/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/kylegalloway/forge/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/kylegalloway/forge/compare/v0.7.3...v0.8.0
[0.7.3]: https://github.com/kylegalloway/forge/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/kylegalloway/forge/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/kylegalloway/forge/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/kylegalloway/forge/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/kylegalloway/forge/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/kylegalloway/forge/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/kylegalloway/forge/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/kylegalloway/forge/compare/v0.4.6...v0.5.0
[0.4.6]: https://github.com/kylegalloway/forge/compare/v0.4.5...v0.4.6
[0.4.5]: https://github.com/kylegalloway/forge/compare/v0.4.4...v0.4.5
[0.4.4]: https://github.com/kylegalloway/forge/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/kylegalloway/forge/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/kylegalloway/forge/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/kylegalloway/forge/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/kylegalloway/forge/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/kylegalloway/forge/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/kylegalloway/forge/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/kylegalloway/forge/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/kylegalloway/forge/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/kylegalloway/forge/releases/tag/v0.1.0
