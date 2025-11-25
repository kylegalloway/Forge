# Forge Production Deployment Checklist

Production readiness checklist for deploying Forge - the Kubernetes-native Zarf package operations controller.

**Current Status**: Foundation complete, ready for production hardening.

## Status Legend

- ✅ **Completed** - Implemented and verified
- 🚧 **In Progress** - Currently being worked on
- ⏸️ **Pending** - Not yet started
- ⚠️ **Blocked** - Waiting on dependencies
- ➖ **Not Applicable** - Not required for your environment

---

## Phase 1: Core Infrastructure (COMPLETED ✅)

*Status: 100% Complete*

### API & CRD

- [x] ✅ ZarfPackageJob CRD with comprehensive validation
- [x] ✅ Status subresources
- [x] ✅ OpenAPI v3 schema validation
- [x] ✅ Short name configured (zpj)

### Controller Security

- [x] ✅ Non-root user (65532:65532)
- [x] ✅ Read-only root filesystem
- [x] ✅ Dropped ALL capabilities
- [x] ✅ Seccomp RuntimeDefault profile
- [x] ✅ Resource limits configured (CPU: 250m-1000m, Memory: 256Mi-512Mi)
- [x] ✅ Namespace enforces Pod Security Standards (restricted)

### RBAC (Least Privilege)

- [x] ✅ ServiceAccount for controller
- [x] ✅ ClusterRole with minimal permissions
- [x] ✅ ZarfPackageJob read/status permissions
- [x] ✅ ServiceAccount read permissions (for policy validation)
- [x] ✅ Secret read permissions (for credentials)
- [x] ✅ Job create/read permissions (no delete)
- [x] ✅ Event create permissions (for observability)

### Health & Readiness

- [x] ✅ /healthz endpoint implemented
- [x] ✅ /readyz endpoint implemented
- [x] ✅ Liveness probe configured
- [x] ✅ Readiness probe configured
- [x] ✅ Graceful shutdown handling

### Build & Dependencies

- [x] ✅ Go 1.25 compatibility
- [x] ✅ Multi-stage Dockerfile
- [x] ✅ Makefile with all targets
- [x] ✅ Dependencies up to date
- [x] ✅ Builds successfully

---

## Phase 2: Policy & Security (COMPLETED ✅)

*Status: 100% Complete*

### Policy Engine

- [x] ✅ ServiceAccount-based authorization
- [x] ✅ Glob pattern matching for repos/registries/buckets
- [x] ✅ Action allowlist enforcement
- [x] ✅ Source type validation
- [x] ✅ Destination type validation
- [x] ✅ Deploy target validation

### Admission Webhook

- [x] ✅ Validating webhook deployed
- [x] ✅ TLS certificate management (cert-manager ready)
- [x] ✅ ZarfPackageJob validation
- [x] ✅ Policy enforcement at admission time
- [x] ✅ Fail-closed configuration
- [x] ✅ Health endpoints (/healthz, /readyz)

### Job Security

- [x] ✅ Jobs run as non-root (1000:1000)
- [x] ✅ Security context enforced
- [x] ✅ TTL configured (1 hour)
- [x] ✅ Active deadline configured (1 hour for build, 30min for others)
- [x] ✅ Resource limits per action type
- [x] ✅ Owner references for cleanup
- [x] ✅ No retry on failure (backoffLimit: 0)

---

## Phase 3: Observability (COMPLETED ✅)

*Status: 100% Complete*

### Metrics (OpenTelemetry)

- [x] ✅ Package creation metrics
- [x] ✅ Job lifecycle metrics (created, completed, failed)
- [x] ✅ Action-specific metrics (build, publish, deploy)
- [x] ✅ Duration histograms
- [x] ✅ Error counters
- [x] ✅ Active resource gauges

### Prometheus Integration

- [x] ✅ Prometheus exporter configured
- [x] ✅ /metrics endpoint exposed
- [x] ✅ ServiceMonitor template provided
- [x] ✅ Prometheus-compatible metric names

### Tracing

- [x] ✅ OpenTelemetry tracer initialized
- [x] ✅ Span creation for operations
- [x] ✅ Context propagation

### Logging

- [x] ✅ Structured logging (klog.InfoS/ErrorS)
- [x] ✅ Log levels configurable via flags
- [x] ✅ Request tracing in logs

### Alerting

- [x] ✅ Alert rules defined
- [x] ✅ Controller down alerts
- [x] ✅ High failure rate alerts
- [x] ✅ Policy violation alerts

---

## Phase 4: Production Hardening (COMPLETED ✅)

**Status: 18/18 items complete (100%)

### High Availability

- [x] ✅ Single replica works correctly
- [x] ✅ Multiple replicas configured (3+)
- [x] ✅ Leader election implemented
- [x] ✅ Pod Disruption Budget
- [x] ✅ Anti-affinity rules
- [x] ✅ Topology spread constraints

### Network Security

- [x] ✅ Controller network policy template
- [x] ✅ Namespace network policy templates
- [x] ✅ Job pod egress restrictions enforced
- [x] ✅ DNS-only egress by default
- [x] ✅ Per-action network policies

### Image Security

- [x] ✅ Container image scanning in CI
- [x] ✅ Image signing (cosign/notary)
- [x] ✅ Image pull policy enforced (Always)
- [x] ✅ Private registry credentials configured

### Credential Management

- [x] ✅ Git credentials rotation process
- [x] ✅ S3 credentials rotation process
- [x] ✅ OCI credentials rotation process
- [x] ✅ External secrets operator integration

### Deployment Options

- [x] ✅ Cluster-wide deployment mode (default)
- [x] ✅ Namespace-scoped deployment mode (restricted environments)
- [x] ✅ Multi-tenant deployment patterns documented

---

## Phase 5: Testing & Validation (COMPLETED ✅)

**Status: 18/18 items complete (100%)

### Unit Tests

- [x] ✅ Controller reconciliation tests
- [x] ✅ Policy engine tests
- [x] ✅ Webhook validation tests
- [x] ✅ Action handler tests
- [x] ✅ Source handler tests
- [x] ✅ Destination handler tests
- [x] ✅ Test coverage >70%

### Integration Tests

- [x] ✅ E2E test suite
- [x] ✅ Multi-namespace tests
- [x] ✅ RBAC policy tests
- [x] ✅ Webhook integration tests
- [x] ✅ Job chaining tests (BuildPublish, etc.)
- [x] ✅ Failure scenario tests

### Load & Performance

- [x] ✅ Baseline performance established
- [x] ✅ Concurrent package operations tested
- [x] ✅ Large package handling tested
- [x] ✅ Resource exhaustion tests
- [x] ✅ Stress testing completed

---

## Phase 6: Documentation (COMPLETED ✅)

**Status: 12/12 items complete (100%)

### User Documentation

- [x] ✅ README.md with quickstart
- [x] ✅ USER_GUIDE.md with examples
- [x] ✅ Troubleshooting guide
- [x] ✅ FAQ document
- [x] ✅ ServiceAccount annotation reference

### Operational Documentation

- [x] ✅ Runbook for common issues
- [x] ✅ Incident response procedures
- [x] ✅ Upgrade procedures
- [x] ✅ Rollback procedures
- [x] ✅ Backup and restore procedures

### Developer Documentation

- [x] ✅ CONTRIBUTING.md
- [x] ✅ Architecture diagrams
- [x] ✅ API documentation

---

## Phase 7: Supply Chain Security & Attestation (IN PROGRESS 🚧)

*Status: 9/20 items complete (45%)*

### Forge Self-Attestation (Controller/Webhook Images)

- [x] ✅ SLSA provenance generation for Forge builds (GitHub Actions)
- [x] ✅ Cosign keyless signing (GitHub OIDC)
- [x] ✅ SBOM generation (SPDX + CycloneDX)
- [x] ✅ SBOM attestation to images
- [x] ✅ Vulnerability scanning (Trivy)
- [x] ✅ Rekor transparency log integration
- [x] ✅ Multi-arch builds (amd64, arm64)
- [x] ✅ Reproducible builds (trimpath, consistent ldflags)
- [x] ✅ Verification documentation

### Package Attestation (Zarf Packages Built BY Forge)

- [x] 🚧 In-toto attestation framework (types implemented)
- [x] 🚧 Build attestation generator (source → artifact)
- [x] 🚧 Publish attestation generator (artifact → registry)
- [x] 🚧 Deploy attestation generator (registry → cluster)
- [ ] ⏸️ Attestation storage (OCI registry implementation)
- [ ] ⏸️ Attestation verification in controller
- [ ] ⏸️ Package signature verification before deploy
- [ ] ⏸️ Policy-based signature requirements
- [ ] ⏸️ Signature validation webhook

### Supply Chain Tracking

- [x] ✅ SBOM generation for Forge images
- [ ] ⏸️ SBOM generation for Zarf packages
- [ ] ⏸️ Supply chain policy enforcement

**Rationale**: Supply chain security is critical for production deployments. Attestations provide cryptographic proof of provenance, enabling auditability and compliance. SLSA framework provides industry-standard approach to supply chain integrity.

**Two-Tier Approach**:
1. **Forge Self-Attestation**: Prove Forge controller/webhook images are authentic
2. **Package Attestation**: Forge generates attestations for packages it builds

**Target Standards**:
- SLSA Build Level 3 for Forge images (✅ ACHIEVED)
- SLSA Level 2 minimum for Zarf packages (🚧 IN PROGRESS)
- In-toto attestation predicates for each operation stage
- Sigstore integration for keyless signing

---

## Phase 8: CI/CD Pipeline (COMPLETED ✅)

*Status: 10/10 items complete (100%)*

### Build Automation

- [x] ✅ CI pipeline configured (GitHub Actions + GitLab CI)
- [x] ✅ Automated builds on PR
- [x] ✅ Image vulnerability scanning (Trivy)
- [x] ✅ Code security scanning (gosec)
- [x] ✅ Multi-arch builds (amd64, arm64, darwin/amd64, darwin/arm64)

### Deployment Automation

- [x] ✅ GitHub Actions release pipeline (triggered on v* tags)
- [x] ✅ GitLab CI complete pipeline (lint, test, build, security, release)
- [x] ✅ Docker images pushed to ghcr.io
- [x] ✅ Coverage reporting (Codecov integration)
- [x] ✅ Pre-commit hooks automated

---

## Phase 9: Compliance & Audit (PENDING ⏸️)

*Status: 0/10 items complete (0%)*

### Security Audit

- [ ] ⏸️ Internal security review completed
- [ ] ⏸️ External security audit (if required)
- [ ] ⏸️ Penetration testing completed
- [ ] ⏸️ Vulnerability remediation complete

### Compliance

- [ ] ⏸️ Compliance requirements identified
- [ ] ⏸️ Audit logging enabled
- [ ] ⏸️ Access logs configured
- [ ] ⏸️ Data classification documented
- [ ] ⏸️ PII handling procedures
- [ ] ⏸️ Data retention policy

---

## Phase 10: Launch Preparation (PENDING ⏸️)

**Status: 0/12 items complete (0%)**

### Pre-Launch Verification

- [ ] ⏸️ All Phase 4 items complete (HA)
- [ ] ⏸️ All Phase 5 items complete (Testing)
- [ ] ⏸️ All Phase 8 items complete (Security)
- [ ] ⏸️ Staging environment tested
- [ ] ⏸️ Monitoring verified working
- [ ] ⏸️ On-call rotation established
- [ ] ⏸️ Rollback plan tested

### Capacity Planning

- [ ] ⏸️ Expected load estimated
- [ ] ⏸️ Resource requirements calculated
- [ ] ⏸️ Scaling strategy defined
- [ ] ⏸️ Cost analysis performed
- [ ] ⏸️ Growth projections documented

---

## Summary Progress

### By Phase

| Phase | Name | Status | Completion |
|-------|------|--------|------------|
| 1 | Core Infrastructure | ✅ Complete | 100% |
| 2 | Policy & Security | ✅ Complete | 100% |
| 3 | Observability | ✅ Complete | 100% |
| 4 | Production Hardening | ✅ Complete | 100% |
| 5 | Testing & Validation | ✅ Complete | 100% |
| 6 | Documentation | ✅ Complete | 100% |
| 7 | Supply Chain Security | 🚧 In Progress | 45% |
| 8 | CI/CD Pipeline | ✅ Complete | 100% |
| 9 | Compliance & Audit | ⏸️ Pending | 0% |
| 10 | Launch Preparation | ⏸️ Pending | 0% |

### Overall Progress: 112 / 135 items (83%)

### Critical Path to Production

Minimum viable path (fastest route):

1. ✅ **COMPLETE**: Phases 1-6, 8 (Core, Security, Observability, Hardening, Testing, Documentation, CI/CD)
2. **RECOMMENDED**: Phase 7 - Supply Chain Security (attestation, signing) - 4-6 weeks
3. **REQUIRED**: Phase 9 - Compliance Audit (internal review minimum) - 2 weeks
4. **REQUIRED**: Phase 10 - Launch Prep (verification, capacity planning) - 1 week

**Estimated time to production**:
- Without supply chain security: 3-4 weeks
- With supply chain security (recommended): 7-9 weeks

**Note**: Phase 7 (Supply Chain Security) is highly recommended for production but not strictly required for initial deployment in controlled environments.

---

## Recommended Execution Order

### ✅ Phases 1-6, 8: COMPLETED
- Core infrastructure, security, observability, hardening complete
- Testing, documentation, and CI/CD pipelines fully implemented

### Weeks 1-4: Phase 7 (Supply Chain Security) - RECOMMENDED

- Week 1-2: SLSA provenance and attestation framework
  - Implement build provenance generation
  - Set up attestation storage (OCI registry)
  - Create attestation predicates for each operation
- Week 3: Signing and verification
  - Integrate Sigstore/Cosign for artifact signing
  - Implement signature verification in controller
  - Add keyless signing support (OIDC)
- Week 4: Supply chain tracking
  - Add SBOM generation
  - Integrate vulnerability scanning
  - Implement supply chain policy enforcement

### Weeks 5-6: Phase 9 (Compliance Audit)

- Internal security review
- Remediate findings
- Document compliance posture
- Validate network policies and attestation flows
- Review credential management and signing keys

### Week 7: Phase 10 (Launch Prep)

- Verify all systems in staging
- Capacity planning and load testing
- Final end-to-end validation with attestation
- Establish on-call rotation
- Production deployment

### Alternative Path (No Supply Chain Security)

If deploying without Phase 7:

- Week 1-2: Phase 9 (Compliance Audit)
- Week 3: Phase 10 (Launch Prep)
- Production deployment in controlled environment
- Plan Phase 7 implementation for future iteration

---

## Notes

- **Architecture change**: This checklist reflects the Forge architecture using ZarfPackageJob CRDs
- **Policy-based**: Security model based on ServiceAccount annotations and admission webhooks
- **Observability-first**: Full OpenTelemetry integration from day one
- **Production-ready foundation**: Core security and observability already implemented
- **Flexible deployment**: Supports both cluster-wide (default) and namespace-scoped (restricted) deployment modes

## Deployment Modes

### Cluster-Wide (Default)

- **RBAC**: ClusterRole with cluster-wide permissions
- **Scope**: Watches all namespaces
- **Use Case**: Platform teams managing multi-tenant deployments
- **Configuration**: Standard RBAC + deployment manifests

### Namespace-Scoped (Restricted)

- **RBAC**: Role with namespace-only permissions
- **Scope**: Watches single namespace (e.g., forge-system)
- **Use Case**: Restricted clusters, individual teams, multi-tenant isolation
- **Configuration**: See [NAMESPACE_SCOPED_DEPLOYMENT.md](./NAMESPACE_SCOPED_DEPLOYMENT.md)
- **Benefits**: Minimal permissions, strong isolation, safe multi-instance deployment

---

*Last Updated: 2025-11-25*
*Version: 1.2.0*
*Target Production: 3-4 weeks (without supply chain security), 7-9 weeks (with supply chain security)*
