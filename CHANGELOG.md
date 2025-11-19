# Changelog

All notable changes to the ScriptRunner project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Health check endpoints**:
  - `/healthz` on port 8081 for liveness probes
  - `/readyz` on port 8081 for readiness probes
  - Automatic readiness tracking based on watcher state
- **Metrics endpoint placeholder**: `/metrics` on port 8080 (implementation pending)
- **RBAC improvements**: Removed unnecessary permissions, following least privilege principle
  - Removed delete/update/patch on Jobs (cleanup via TTL and owner references)
  - Added events create/patch for better observability

### Planned
- Admission webhook implementation for production validation
- Prometheus metrics exporter implementation
- Multi-replica controller support with leader election
- Custom resource status conditions (Ready, JobCreated, Failed)
- Job completion tracking and status updates
- Configurable TTL and timeout values via CRD
- Support for ConfigMap/Secret volume mounts in jobs
- Retry logic for failed jobs

## [0.2.0] - 2024-01-15

### Added
- **scriptRef support**: Reference pre-built scripts in container images instead of inline scripts
- **scriptArgs support**: Pass command-line arguments to referenced scripts
- **Production security hardening**:
  - Job TTL configuration (auto-cleanup after 1 hour)
  - Active deadline for jobs (10-minute timeout)
  - Resource limits on job pods (CPU: 250m-1000m, Memory: 256Mi-1Gi)
  - Pod security context (non-root user, read-only filesystem, dropped capabilities)
  - Seccomp profile enforcement
- **Enhanced controller deployment**:
  - Pod security standards labels on namespace
  - Resource limits on controller (CPU: 250m-1000m, Memory: 256Mi-512Mi)
  - Read-only root filesystem
  - Liveness and readiness probe configuration
  - Prometheus annotations for metrics scraping
- **Structured logging**: Migrated from printf-style to InfoS/ErrorS for better observability
- **Example container image** with pre-built scripts:
  - process-data.sh: Data processing with arguments
  - validate-inputs.sh: Input validation script
  - report-status.py: Python-based status reporting
- **Comprehensive documentation**:
  - docs/PRODUCTION.md: Production deployment guide
  - docs/USER_GUIDE.md: End-user documentation
  - docs/CLIENT_VALIDATION.md: Client-side validation options
  - webhook/README.md: Admission webhook guide
  - CONTRIBUTING.md: Extended with pre-built scripts section
  - QUICKSTART.md: 5-minute getting started guide
- **Client-side validation**:
  - JSON Schema generation script
  - VS Code settings examples
  - IDE integration documentation
- **Development tooling**:
  - scripts/generate-json-schema.sh: Generate schema from CRD
  - Enhanced Makefile with kind integration
  - Podman/Docker auto-detection
  - schemas/ directory with JSON Schema for IDE validation

### Changed
- Controller now uses dynamic client instead of generated clients (simplified deployment)
- Updated CRD with scriptRef and scriptArgs fields
- Improved DeepCopy implementation for new fields
- Controller deployment now enforces restricted pod security standards
- Sample manifests include scriptRef examples

### Fixed
- Controller properly handles both inline scripts and script references
- Job creation validates mutually exclusive script/scriptRef fields

### Documentation
- Added 3 comprehensive documentation guides (PRODUCTION.md, USER_GUIDE.md, CLIENT_VALIDATION.md)
- Created webhook implementation guide
- Extended CONTRIBUTING.md with production patterns
- Added client validation and schema generation documentation
- Included 9 sample manifests demonstrating various use cases

### Developer Experience
- Added kind-* Makefile targets for local development
- Created automated test scripts (quick-test.sh, test-e2e.sh, dev-setup.sh)
- Included JSON Schema for IDE autocomplete and validation
- Added VS Code workspace settings examples

## [0.1.0] - 2024-01-14

### Added
- Initial ScriptRunner CRD (v1alpha1)
- Basic controller implementation
- Support for inline shell scripts
- Input variables passed as environment variables (INPUT_*)
- Job-based execution model
- Owner references for automatic cleanup
- Default busybox:latest image
- Simple controller using dynamic client
- Basic RBAC configuration (ClusterRole, ServiceAccount, ClusterRoleBinding)
- Namespace isolation (scriptrunner-system)
- Sample ScriptRunner manifests
- Makefile for building and deploying
- Dockerfile for controller image
- Basic README with usage examples
- CONTRIBUTING.md with development setup
- LICENSE (Apache 2.0)

### Features
- **Custom Resource**: ScriptRunner CRD with spec.inputs and spec.script
- **Controller**: Watches ScriptRunner resources and creates Jobs
- **Environment Variables**: Inputs automatically prefixed with INPUT_
- **Metadata**: SCRIPTRUNNER_NAME and SCRIPTRUNNER_NAMESPACE available in jobs
- **Owner References**: Jobs cleaned up when ScriptRunner is deleted
- **Status Tracking**: JobName, phase, and message in status
- **Kind Support**: Local development with kind clusters

### Developer Tools
- Go module configuration (v1.21+)
- Multi-stage Dockerfile
- Makefile with build, deploy, test targets
- Config directory structure (crd/, rbac/, manager/, samples/)
- pkg/ directory for API types and controller logic
- DeepCopy code generation

## Version History

### Version Numbering

ScriptRunner follows [Semantic Versioning](https://semver.org/):

- **MAJOR version** (X.0.0): Incompatible API changes
- **MINOR version** (0.X.0): New functionality in a backwards-compatible manner
- **PATCH version** (0.0.X): Backwards-compatible bug fixes

### Upgrade Path

#### From 0.1.0 to 0.2.0

**Breaking Changes**: None - fully backwards compatible

**New Features**:
- You can now use `scriptRef` instead of `script` for pre-built scripts
- Scripts in containers can receive arguments via `scriptArgs`

**Action Required**:
1. Update CRD: `kubectl apply -f config/crd/`
2. Update controller deployment: `kubectl apply -f config/manager/`
3. Update RBAC if needed: `kubectl apply -f config/rbac/`

**Existing Resources**:
- All existing ScriptRunner resources with `script` field continue to work
- No migration needed

**Recommended Actions**:
- Review new security settings in deployment
- Consider building container images with pre-built scripts for production
- Enable Pod Security Standards on user namespaces
- Review production guide (docs/PRODUCTION.md)

### Future Roadmap

#### v0.3.0 (Planned)
- Admission webhook for validation and defaults
- Prometheus metrics
- Health check endpoints
- Job status tracking and updates

#### v0.4.0 (Planned)
- Leader election for multi-replica controllers
- Enhanced status conditions
- Configurable resource limits per ScriptRunner
- Volume mount support

#### v1.0.0 (Planned)
- Production-ready with complete webhook implementation
- Full observability (metrics, traces, structured logs)
- Comprehensive test suite
- Performance benchmarks
- Migration guide from v0.x

## Migration Guides

### Migrating to scriptRef (0.1.0 → 0.2.0)

**Before (inline script)**:
```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
spec:
  script: |
    #!/bin/sh
    echo "Processing: $INPUT_data"
    # ... long script ...
  inputs:
    data: "value"
```

**After (pre-built script)**:
```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
spec:
  image: my-registry/scripts:v1.0.0
  scriptRef: /scripts/process-data.sh
  scriptArgs: ["--verbose"]
  inputs:
    data: "value"
```

**Benefits**:
- Scripts versioned with container images
- Reusable across multiple ScriptRunners
- Tested independently
- Supports complex dependencies

## Deprecation Notices

### Current
- None

### Future
- **v0.3.0**: `spec.script` may be deprecated in favor of `spec.scriptRef` for production use
  - Timeline: Deprecated in v0.3.0, removed in v2.0.0
  - Migration: Build container images with scripts, use `scriptRef`
  - Reason: Better security, versioning, and reusability

## Security Advisories

### Current
- None

### Best Practices
- Always use `scriptRef` with approved container images in production
- Enable Pod Security Standards on all namespaces
- Implement admission webhook for validation
- Set resource quotas on user namespaces
- Use network policies to restrict job pod access
- Scan container images for vulnerabilities

## Support

### Compatibility

| ScriptRunner Version | Kubernetes Version | Go Version |
|---------------------|-------------------|------------|
| 0.2.0               | 1.28+             | 1.21+      |
| 0.1.0               | 1.28+             | 1.21+      |

### Getting Help

- Documentation: See docs/ directory
- Issues: GitHub Issues
- Discussions: GitHub Discussions
- Contributing: See CONTRIBUTING.md

## Contributors

Special thanks to all contributors who have helped make ScriptRunner better!

---

*For detailed information about each release, see the [GitHub Releases](https://github.com/kylegalloway/scriptrunner/releases) page.*
