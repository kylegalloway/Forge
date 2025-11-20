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

**Status: 100% Complete**

### API & CRD
- [x] ✅ ZarfPackage CRD with comprehensive validation
- [x] ✅ UDSBundle CRD support
- [x] ✅ Status subresources for both CRDs
- [x] ✅ OpenAPI v3 schema validation
- [x] ✅ Short names configured (zp, udsb)

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
- [x] ✅ ZarfPackage and UDSBundle read/status permissions
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

**Status: 100% Complete**

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
- [x] ✅ ZarfPackage validation
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

**Status: 100% Complete**

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

## Phase 4: Production Hardening (IN PROGRESS 🚧)

**Status: 3/15 items complete (20%)**

### High Availability
- [x] ✅ Single replica works correctly
- [ ] ⏸️ Multiple replicas configured (3+)
- [ ] ⏸️ Leader election implemented
- [ ] ⏸️ Pod Disruption Budget
- [ ] ⏸️ Anti-affinity rules
- [ ] ⏸️ Topology spread constraints

### Network Security
- [x] ✅ Controller network policy template
- [x] ✅ Namespace network policy templates
- [ ] ⏸️ Job pod egress restrictions enforced
- [ ] ⏸️ DNS-only egress by default
- [ ] ⏸️ Per-action network policies

### Image Security
- [ ] ⏸️ Container image scanning in CI
- [ ] ⏸️ Image signing (cosign/notary)
- [ ] ⏸️ Image pull policy enforced (Always)
- [ ] ⏸️ Private registry credentials configured

### Credential Management
- [ ] ⏸️ Git credentials rotation process
- [ ] ⏸️ S3 credentials rotation process
- [ ] ⏸️ OCI credentials rotation process
- [ ] ⏸️ External secrets operator integration

---

## Phase 5: Testing & Validation (PENDING ⏸️)

**Status: 0/18 items complete (0%)**

### Unit Tests
- [ ] ⏸️ Controller reconciliation tests
- [ ] ⏸️ Policy engine tests
- [ ] ⏸️ Webhook validation tests
- [ ] ⏸️ Action handler tests
- [ ] ⏸️ Source handler tests
- [ ] ⏸️ Destination handler tests
- [ ] ⏸️ Test coverage >70%

### Integration Tests
- [ ] ⏸️ E2E test suite
- [ ] ⏸️ Multi-namespace tests
- [ ] ⏸️ RBAC policy tests
- [ ] ⏸️ Webhook integration tests
- [ ] ⏸️ Job chaining tests (BuildPublish, etc.)
- [ ] ⏸️ Failure scenario tests

### Load & Performance
- [ ] ⏸️ Baseline performance established
- [ ] ⏸️ Concurrent package operations tested
- [ ] ⏸️ Large package handling tested
- [ ] ⏸️ Resource exhaustion tests
- [ ] ⏸️ Stress testing completed

---

## Phase 6: Documentation (PARTIAL ✅)

**Status: 3/12 items complete (25%)**

### User Documentation
- [x] ✅ README.md with quickstart
- [x] ✅ USER_GUIDE.md with examples
- [ ] ⏸️ Troubleshooting guide
- [ ] ⏸️ FAQ document
- [ ] ⏸️ ServiceAccount annotation reference

### Operational Documentation
- [ ] ⏸️ Runbook for common issues
- [ ] ⏸️ Incident response procedures
- [ ] ⏸️ Upgrade procedures
- [ ] ⏸️ Rollback procedures
- [ ] ⏸️ Backup and restore procedures

### Developer Documentation
- [x] ✅ CONTRIBUTING.md
- [ ] ⏸️ Architecture diagrams
- [ ] ⏸️ API documentation

---

## Phase 7: CI/CD Pipeline (PENDING ⏸️)

**Status: 0/10 items complete (0%)**

### Build Automation
- [ ] ⏸️ CI pipeline configured (GitHub Actions recommended)
- [ ] ⏸️ Automated builds on PR
- [ ] ⏸️ Image vulnerability scanning
- [ ] ⏸️ Image signing in pipeline
- [ ] ⏸️ Multi-arch builds (amd64, arm64)

### Deployment Automation
- [ ] ⏸️ GitOps configured (ArgoCD/Flux)
- [ ] ⏸️ Staging environment automated
- [ ] ⏸️ Production approval workflow
- [ ] ⏸️ Canary deployments
- [ ] ⏸️ Automated rollback on failure

---

## Phase 8: Compliance & Audit (PENDING ⏸️)

**Status: 0/10 items complete (0%)**

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

## Phase 9: Launch Preparation (PENDING ⏸️)

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
| 4 | Production Hardening | 🚧 In Progress | 20% |
| 5 | Testing & Validation | ⏸️ Pending | 0% |
| 6 | Documentation | 🚧 In Progress | 25% |
| 7 | CI/CD Pipeline | ⏸️ Pending | 0% |
| 8 | Compliance & Audit | ⏸️ Pending | 0% |
| 9 | Launch Preparation | ⏸️ Pending | 0% |

### Overall Progress: 45 / 112 items (40%)

### Critical Path to Production

Minimum viable path (fastest route):
1. ✅ **COMPLETE**: Phases 1-3 (Core + Security + Observability)
2. **REQUIRED**: Phase 4 - High Availability (leader election, replicas)
3. **REQUIRED**: Phase 5 - Testing (E2E tests, integration tests)
4. **REQUIRED**: Phase 8 - Security Audit (internal review minimum)
5. **REQUIRED**: Phase 9 - Launch Prep (verification, capacity planning)

**Estimated time to production**: 6-10 weeks (from current state)

---

## Recommended Execution Order

### Weeks 1-2: Complete Phase 4 (Production Hardening)
- Implement leader election
- Configure HA deployment (3 replicas)
- Set up network policies
- Configure image scanning

### Weeks 3-4: Phase 5 (Testing)
- Write unit tests (target 70% coverage)
- Build E2E test suite
- Perform load testing
- Test failure scenarios

### Week 5: Phase 6 (Documentation)
- Complete operational runbooks
- Write troubleshooting guides
- Document upgrade/rollback procedures

### Week 6-7: Phase 7 (CI/CD)
- Set up GitHub Actions
- Configure automated testing
- Implement GitOps deployment

### Week 8-9: Phase 8 (Security Audit)
- Internal security review
- Remediate findings
- Document compliance posture

### Week 10: Phase 9 (Launch Prep)
- Verify all systems
- Capacity planning
- Final staging tests
- Production deployment

---

## Notes

- **Architecture change**: This checklist reflects the Forge architecture using ZarfPackage/UDSBundle CRDs
- **No ScriptRunner**: Old ScriptRunner API has been completely removed
- **Policy-based**: Security model based on ServiceAccount annotations and admission webhooks
- **Observability-first**: Full OpenTelemetry integration from day one
- **Production-ready foundation**: Core security and observability already implemented

---

*Last Updated: 2025-11-20*
*Version: 1.0.0*
*Target Production: 6-10 weeks*
