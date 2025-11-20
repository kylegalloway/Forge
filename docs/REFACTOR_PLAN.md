# ScriptRunner → Zarf/UDS Controller Refactor Plan

## Overview

This document outlines the architectural refactor to transform ScriptRunner from a generic script execution controller into a purpose-built Zarf Package and UDS Bundle deployment controller with restricted, policy-driven actions.

## Current State (v1alpha1)

**What it does:**
- Executes arbitrary scripts in containers
- Users provide: image, script (inline or ref), inputs
- No restrictions on what scripts can do
- Generic and open-ended

**Problems:**
- Too permissive - users can run anything
- No semantic understanding of operations
- Difficult to enforce policies
- No built-in Zarf/UDS knowledge

## Target State (v1alpha2)

**What it will do:**
- Build, publish, and deploy Zarf packages and UDS bundles
- Declarative actions with semantic meaning
- Policy-driven RBAC for actions and resources
- Built-in Zarf/UDS tooling and conventions

---

## Architecture Design

### 1. Resource Types

Two new CRD kinds (or one CRD with type field):

#### Option A: Two CRDs
```yaml
# ZarfPackage CRD
apiVersion: zarf.io/v1alpha2
kind: ZarfPackage
metadata:
  name: my-package
spec:
  actions: [build, publish, deploy]  # Which actions to perform
  source:
    type: git | s3 | oci | local
    # ... source-specific fields
  publish:
    destination:
      type: s3 | oci | local
      # ... destination-specific fields
  deploy:
    target: in-cluster | external-cluster
    # ... deploy-specific fields
  rbacPolicy:  # Policy restrictions for this resource
    allowedActions: [build, publish]
    allowedUsers: [user1, user2]
```

```yaml
# UDSBundle CRD
apiVersion: uds.io/v1alpha2
kind: UDSBundle
# ... similar structure
```

#### Option B: Single CRD with type field
```yaml
apiVersion: zarf.io/v1alpha2
kind: ZarfResource
spec:
  type: package | bundle
  # ... rest of spec
```

**Recommendation: Option A** - Clearer separation, easier to extend independently

### 2. Action Types

| Action | Description | Input | Output |
|--------|-------------|-------|--------|
| **build** | Build Zarf package/bundle from source | Source location | Package artifact |
| **publish** | Publish built artifact to registry/storage | Artifact | Published location |
| **deploy** | Deploy package/bundle to cluster | Artifact | Deployed resources |
| **build+publish** | Build and immediately publish | Source | Published location |
| **build+deploy** | Build and immediately deploy | Source | Deployed resources |
| **build+publish+deploy** | Full pipeline | Source | Deployed resources |
| **publish+deploy** | Publish pre-built + deploy | Artifact | Deployed resources |

### 3. Source Types

| Type | Use Cases | Authentication | Restrictions |
|------|-----------|----------------|--------------|
| **local** | Dev/testing only | None | Must be marked as dev-only |
| **git** | Source code repos | SSH key, token | Allowed branches/tags |
| **s3** | Pre-built artifacts | IAM role, access key | Allowed buckets |
| **oci** | OCI registries | Registry credentials | Allowed registries |

**Source Schema:**
```yaml
source:
  type: git
  git:
    repo: https://github.com/org/repo
    ref: main  # branch, tag, or commit
    path: /path/to/zarf.yaml  # Optional: path within repo
    auth:
      secretRef: git-credentials  # Secret with SSH key or token
  # OR
  type: s3
  s3:
    bucket: my-bucket
    key: path/to/package.tar.zst
    region: us-west-2
    auth:
      secretRef: s3-credentials
  # OR
  type: oci
  oci:
    image: ghcr.io/org/package:v1.0.0
    auth:
      secretRef: oci-credentials
  # OR
  type: local
  local:
    path: /tmp/package  # Only for dev/testing
    devMode: true  # Required flag
```

### 4. Publish Destinations

| Type | Use Cases | Authentication |
|------|-----------|----------------|
| **local** | Testing only | None |
| **s3** | Artifact storage | IAM role, access key |
| **oci** | Container registries | Registry credentials |

**Publish Schema:**
```yaml
publish:
  destination:
    type: s3
    s3:
      bucket: artifacts-bucket
      keyPrefix: packages/
      region: us-west-2
      auth:
        secretRef: s3-publish-creds
  # OR
  destination:
    type: oci
    oci:
      registry: ghcr.io/org/packages
      tag: "{{.Version}}"  # Template support
      auth:
        secretRef: oci-publish-creds
```

### 5. Deploy Targets

| Type | Description | Authentication |
|------|-------------|----------------|
| **in-cluster** | Deploy to same cluster as controller | ServiceAccount |
| **external-cluster** | Deploy to different cluster | Kubeconfig secret |

**Deploy Schema:**
```yaml
deploy:
  target: in-cluster
  # OR
  target: external-cluster
  externalCluster:
    kubeconfigSecretRef: target-cluster-kubeconfig
    context: production  # Optional: specific context

  # Common fields
  namespace: default
  timeout: 30m
  skipCRDs: false
  options:
    - --adopt-existing-resources
    - --skip-webwait
```

### 6. RBAC Policy Engine

**Policy Fields in CRD:**
```yaml
spec:
  rbacPolicy:
    # Who can use this resource
    allowedUsers:
      - user:alice@example.com
      - group:developers

    # Which actions are permitted
    allowedActions: [build, publish]  # Cannot deploy

    # Source restrictions
    allowedSources:
      - type: git
        repos: [github.com/myorg/*]
      - type: s3
        buckets: [dev-artifacts]

    # Destination restrictions
    allowedDestinations:
      - type: oci
        registries: [ghcr.io/myorg/*]

    # Deploy restrictions (if action=deploy allowed)
    allowedDeployTargets:
      - in-cluster
      # External clusters require higher permissions
```

**Webhook Validation:**
- Check if user (from admission review) is in `allowedUsers`
- Verify requested actions are in `allowedActions`
- Validate source matches `allowedSources` patterns
- Validate destination matches `allowedDestinations`

**Multi-level Policies:**
1. **CRD-level policy** - in the resource spec (self-service)
2. **Namespace-level policy** - ConfigMap/annotation (admin-managed)
3. **Cluster-level policy** - Webhook default policy (platform team)

### 7. Validation Matrix

| Action | Local Source | Git Source | S3 Source | OCI Source |
|--------|--------------|------------|-----------|------------|
| build | ✅ (dev only) | ✅ | ❌ | ❌ |
| publish | ❌ | ❌ | ✅ (copy) | ✅ (copy) |
| deploy | ❌ | ❌ | ✅ | ✅ |
| build+publish | ✅ (dev) | ✅ | ❌ | ❌ |
| build+deploy | ✅ (dev) | ✅ | ❌ | ❌ |

**Validation Rules:**
- `build` requires source type `git` or `local`
- `publish` without `build` requires source type `s3` or `oci` (artifact already exists)
- `deploy` without `build` requires source type `s3` or `oci`
- `local` source requires `devMode: true` and triggers warnings

---

## Implementation Plan

### Phase 1: API Design & Validation (Week 1)
**Goal: Define and validate new API schema**

1. ✅ Create this refactor plan document
2. ⏸️ Design complete v1alpha2 CRD schema
3. ⏸️ Write validation matrix and policy rules
4. ⏸️ Create sample manifests for all use cases
5. ⏸️ Review with stakeholders

**Deliverables:**
- `pkg/apis/zarf/v1alpha2/types.go`
- `config/crd/zarf.io_zarfpackages.yaml`
- `config/crd/uds.io_udsbundles.yaml`
- `config/samples/v1alpha2/*.yaml`
- Updated `docs/REFACTOR_PLAN.md`

### Phase 2: Controller Refactor (Week 2-3)
**Goal: Implement action handlers and source/destination logic**

1. ⏸️ Create action handler interface
2. ⏸️ Implement Build action (Zarf build command)
3. ⏸️ Implement Publish action (upload to S3/OCI)
4. ⏸️ Implement Deploy action (Zarf deploy command)
5. ⏸️ Implement combined action handlers
6. ⏸️ Create source handlers (git clone, S3 download, OCI pull)
7. ⏸️ Create destination handlers (S3 upload, OCI push)
8. ⏸️ Add credential management (Secret refs)

**Deliverables:**
- `pkg/actions/` - Action handler implementations
- `pkg/sources/` - Source type handlers
- `pkg/destinations/` - Destination handlers
- Updated `pkg/controller/controller.go`
- `pkg/credentials/` - Secret management

### Phase 3: Policy Enforcement (Week 3-4)
**Goal: Implement RBAC policy engine**

1. ⏸️ Design policy evaluation engine
2. ⏸️ Update admission webhook for policy checks
3. ⏸️ Implement user/group matching
4. ⏸️ Implement action authorization
5. ⏸️ Implement source/destination pattern matching
6. ⏸️ Add audit logging for policy decisions
7. ⏸️ Create namespace-level policy ConfigMap

**Deliverables:**
- `pkg/policy/` - Policy evaluation engine
- Updated `pkg/webhook/validator.go`
- `config/rbac/namespace-policy-template.yaml`
- Policy decision audit logs

### Phase 4: Observability Updates (Week 4)
**Goal: Update metrics, alerts, and dashboards**

1. ⏸️ Add metrics for new action types
2. ⏸️ Add metrics for policy decisions (allowed/denied)
3. ⏸️ Update Grafana dashboard with new panels
4. ⏸️ Add alerts for policy violations
5. ⏸️ Add alerts for failed builds/deploys
6. ⏸️ Update tracing with action-specific spans

**Deliverables:**
- Updated `pkg/telemetry/metrics.go`
- Updated `config/grafana/scriptrunner-dashboard.json`
- Updated `config/prometheus/alerts/scriptrunner-alerts.yaml`

### Phase 5: Documentation & Migration (Week 5)
**Goal: Document new functionality and migration path**

1. ⏸️ Write v1alpha1 → v1alpha2 migration guide
2. ⏸️ Update USER_GUIDE.md for new API
3. ⏸️ Update CONTRIBUTING.md with new architecture
4. ⏸️ Create policy configuration guide
5. ⏸️ Create troubleshooting guide
6. ⏸️ Update README with new capabilities
7. ⏸️ Create conversion webhook (optional - for dual-version support)

**Deliverables:**
- `docs/MIGRATION_v1alpha2.md`
- Updated `docs/USER_GUIDE.md`
- Updated `CONTRIBUTING.md`
- `docs/POLICY_GUIDE.md`
- `docs/TROUBLESHOOTING_v1alpha2.md`

### Phase 6: Testing (Week 5-6)
**Goal: Comprehensive testing of new functionality**

1. ⏸️ Unit tests for action handlers
2. ⏸️ Unit tests for policy engine
3. ⏸️ Integration tests for each action type
4. ⏸️ Integration tests for policy enforcement
5. ⏸️ E2E tests for complete workflows
6. ⏸️ Policy violation tests
7. ⏸️ Load testing with new resource types

**Deliverables:**
- `pkg/actions/*_test.go`
- `pkg/policy/*_test.go`
- `test/integration/v1alpha2/`
- `test/e2e/v1alpha2/`
- Test coverage report (>70%)

---

## Breaking Changes

### API Changes (v1alpha1 → v1alpha2)

**Removed:**
- `spec.script` - No longer arbitrary scripts
- `spec.image` - Image determined by action type
- `spec.scriptRef` - Replaced with action/source model
- `spec.scriptArgs` - Replaced with action-specific config
- `spec.inputs` - Replaced with source/destination config

**Added:**
- `spec.type` - "package" or "bundle"
- `spec.actions` - Array of actions to perform
- `spec.source` - Source configuration
- `spec.publish` - Publish configuration
- `spec.deploy` - Deploy configuration
- `spec.rbacPolicy` - Policy restrictions

**Migration Strategy:**
1. No automatic conversion - breaking change
2. Deprecate v1alpha1 (mark as deprecated in CRD)
3. Support both versions for 2 release cycles
4. Create conversion webhook for dual-version support (optional)
5. Provide migration tool/script

### Controller Changes

**Job Creation:**
- Jobs now use Zarf CLI container image
- Command determined by action type
- Credentials mounted as volumes (not env vars)
- No more arbitrary script execution

**RBAC:**
- Controller needs permissions to read Secrets (for credentials)
- Controller needs permissions to create external cluster connections (if enabled)

### Webhook Changes

**Validation:**
- New validation rules for v1alpha2 schema
- Policy enforcement on create/update
- Source/destination pattern validation
- User/group authorization checks

**Mutation:**
- Set defaults for optional fields
- Inject namespace-level policies
- Add resource labels for policy tracking

---

## Security Considerations

### Credential Management

**Secrets Structure:**
```yaml
# Git credentials
apiVersion: v1
kind: Secret
metadata:
  name: git-credentials
type: Opaque
data:
  ssh-key: <base64>
  # OR
  token: <base64>

# S3 credentials
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials
type: Opaque
data:
  access-key-id: <base64>
  secret-access-key: <base64>

# OCI credentials
apiVersion: v1
kind: Secret
metadata:
  name: oci-credentials
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <base64>
```

**Secret Access:**
- Controller reads secrets via RBAC
- Secrets mounted into Job pods as files (not env vars)
- Secret references must be in same namespace as resource
- Audit logging when secrets are accessed

### Policy Enforcement

**Defense in Depth:**
1. **Webhook validation** - First line of defense
2. **Controller checks** - Runtime validation
3. **RBAC** - Kubernetes-native authz
4. **Network policies** - Limit job pod network access
5. **Audit logs** - Track all policy decisions

**Attack Vectors:**
- User tries to access unauthorized registry → Blocked by webhook policy check
- User tries to deploy to prod cluster → Blocked by `allowedDeployTargets`
- User tries to read unauthorized S3 bucket → Blocked by `allowedSources`
- User modifies policy in resource → Blocked by namespace-level policy override

---

## Rollout Strategy

### Development (Weeks 1-6)
- Implement in feature branch
- Extensive testing with staging cluster
- Internal review and feedback

### Alpha Release (Week 7)
- Deploy v1alpha2 alongside v1alpha1 (both supported)
- Mark v1alpha1 as deprecated
- Documentation for migration
- Early adopter testing

### Beta Release (Week 10)
- v1alpha2 stable, v1alpha1 still supported
- Migration tools available
- Most users migrated

### GA Release (Week 13)
- v1alpha1 removed
- v1alpha2 only
- Production-ready

---

## Success Metrics

### Functionality
- ✅ All actions (build, publish, deploy) working
- ✅ All source types (git, s3, oci, local) working
- ✅ All destination types working
- ✅ Policy enforcement working
- ✅ >70% test coverage

### Security
- ✅ Policy violations blocked 100% of the time
- ✅ No credential leakage in logs
- ✅ Audit trail for all operations
- ✅ Penetration testing passed

### Performance
- ✅ Build action completes in <5min (typical package)
- ✅ Publish action completes in <2min
- ✅ Deploy action completes in <10min
- ✅ Policy evaluation adds <100ms latency

### Usability
- ✅ Migration guide clear and accurate
- ✅ Sample manifests for all use cases
- ✅ Policy configuration straightforward
- ✅ Error messages actionable

---

## Open Questions

1. **CRD Design**: Single CRD with type field vs separate ZarfPackage/UDSBundle CRDs?
   - Recommendation: Separate CRDs for clarity

2. **Policy Hierarchy**: How do namespace and cluster policies override resource policies?
   - Recommendation: Cluster > Namespace > Resource (most restrictive wins)

3. **Versioning**: Support v1alpha1 and v1alpha2 simultaneously?
   - Recommendation: Yes, for 2 release cycles (6 months)

4. **Conversion**: Automatic v1alpha1 → v1alpha2 conversion?
   - Recommendation: No - too different, require manual migration

5. **External Clusters**: Allow deploying to arbitrary external clusters?
   - Recommendation: Yes, but with strict RBAC policy

6. **Build Caching**: Cache build artifacts for faster rebuilds?
   - Recommendation: Phase 2 feature, not MVP

7. **Multi-stage Actions**: Support conditional actions (build if changed, always deploy)?
   - Recommendation: Phase 2 feature, start simple

8. **Rollback**: Support automated rollback on failed deploy?
   - Recommendation: Phase 2 feature

---

## Next Steps

1. **Review this plan** with team and stakeholders
2. **Decide on open questions** (especially CRD design)
3. **Create detailed API spec** for v1alpha2
4. **Start Phase 1** (API design) with first CRD draft
5. **Set up feature branch** for development
6. **Create tracking issues** for each phase

---

*Last Updated: 2025-11-19*
*Author: Architecture Team*
*Status: DRAFT - Awaiting Review*
