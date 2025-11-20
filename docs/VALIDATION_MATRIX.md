# ZarfPackage Validation Matrix

This document defines all validation rules for the ZarfPackage CRD, including both OpenAPI schema validation (enforced by the API server) and webhook validation (enforced by the admission controller).

## Validation Layers

1. **OpenAPI Schema Validation** (CRD level) - Enforced by kube-apiserver
2. **Webhook Validation** (Admission controller) - Custom business logic
3. **RBAC Policy Validation** (Admission controller) - User/action/resource authorization

---

## Action-Specific Required Fields

| Action | Source | Publish | Deploy |
|--------|--------|---------|--------|
| `Build` | ✅ Required | ❌ Forbidden | ❌ Forbidden |
| `Publish` | ✅ Required | ✅ Required | ❌ Forbidden |
| `Deploy` | ✅ Required | ❌ Forbidden | ✅ Required |
| `BuildPublish` | ✅ Required | ✅ Required | ❌ Forbidden |
| `BuildDeploy` | ✅ Required | ❌ Forbidden | ✅ Required |
| `PublishDeploy` | ✅ Required | ✅ Required | ✅ Required |
| `BuildPublishDeploy` | ✅ Required | ✅ Required | ✅ Required |

**Validation Layer**: Webhook (custom logic)

**Examples**:
```yaml
# ✅ VALID - Build only
spec:
  action: Build
  source: {...}
  # No publish or deploy

# ❌ INVALID - Build with deploy config
spec:
  action: Build
  source: {...}
  deploy: {...}  # Error: deploy not allowed for Build action
```

---

## Source Type Validation

### Git Source

**When**: `source.type: Git`

| Field | Required | Validation | Example |
|-------|----------|------------|---------|
| `git.url` | ✅ | Must match `^https://.*` | `https://github.com/org/repo` |
| `git.ref` | ✅ | Any valid ref | `v1.0.0`, `main`, `abc123` |
| `git.path` | ❌ | Default: `.` | `examples/package` |
| `git.credentialsSecretRef` | ❌ | Must reference existing Secret | `name: git-creds` |

**Validation Layer**: OpenAPI schema + Webhook (secret existence)

**Examples**:
```yaml
# ✅ VALID
source:
  type: Git
  git:
    url: https://github.com/defenseunicorns/zarf
    ref: v0.32.0
    path: examples/big-bang

# ❌ INVALID - SSH URL not allowed
source:
  type: Git
  git:
    url: git@github.com:org/repo.git  # Error: must be HTTPS
    ref: main
```

### S3 Source

**When**: `source.type: S3`

| Field | Required | Validation | Example |
|-------|----------|------------|---------|
| `s3.bucket` | ✅ | Non-empty string | `artifacts-bucket` |
| `s3.key` | ✅ | Non-empty string | `packages/pkg-v1.0.tar.zst` |
| `s3.region` | ✅ | Valid AWS region | `us-west-2` |
| `s3.credentialsSecretRef` | ❌ | Must reference existing Secret | `name: s3-creds` |

**Validation Layer**: OpenAPI schema + Webhook (secret existence)

### OCI Source

**When**: `source.type: OCI`

| Field | Required | Validation | Example |
|-------|----------|------------|---------|
| `oci.image` | ✅ | Valid OCI ref | `ghcr.io/org/pkg:v1.0` |
| `oci.credentialsSecretRef` | ❌ | Must be type `kubernetes.io/dockerconfigjson` | `name: ghcr-pull` |

**Validation Layer**: OpenAPI schema + Webhook (secret type validation)

### Local Source

**When**: `source.type: Local`

| Field | Required | Validation | Example |
|-------|----------|------------|---------|
| `local.path` | ✅ | Non-empty string | `/workspace/package` |
| `local.devMode` | ✅ | Must be `true` | `true` |

**Validation Layer**: OpenAPI schema + Webhook (devMode enforcement)

**Examples**:
```yaml
# ✅ VALID - Dev mode explicitly enabled
source:
  type: Local
  local:
    path: /tmp/my-package
    devMode: true

# ❌ INVALID - Local without devMode
source:
  type: Local
  local:
    path: /tmp/my-package
    # Error: devMode must be true for local sources
```

---

## Publish Destination Validation

### S3 Destination

**When**: `publish.destination.type: S3`

| Field | Required | Validation | Example |
|-------|----------|------------|---------|
| `s3.bucket` | ✅ | Non-empty string | `published-packages` |
| `s3.keyPrefix` | ❌ | Default: empty | `releases/` |
| `s3.region` | ✅ | Valid AWS region | `us-east-1` |
| `s3.credentialsSecretRef` | ❌ | Must reference existing Secret | `name: s3-write-creds` |

### OCI Destination

**When**: `publish.destination.type: OCI`

| Field | Required | Validation | Example |
|-------|----------|------------|---------|
| `oci.registry` | ✅ | Valid registry URL | `ghcr.io` |
| `oci.repository` | ✅ | Non-empty string | `org/packages` |
| `oci.tag` | ✅ | Valid tag | `v1.0.0`, `latest` |
| `oci.credentialsSecretRef` | ❌ | Must be type `kubernetes.io/dockerconfigjson` | `name: registry-push` |

### Local Destination

**When**: `publish.destination.type: Local`

| Field | Required | Validation | Example |
|-------|----------|------------|---------|
| `local.path` | ✅ | Non-empty string | `/tmp/artifacts` |
| `local.devMode` | ✅ | Must be `true` | `true` |

---

## Deploy Target Validation

### InCluster Deploy

**When**: `deploy.target: InCluster`

| Field | Required | Validation | Default |
|-------|----------|------------|---------|
| `namespace` | ❌ | Valid K8s namespace name | `default` |
| `timeout` | ❌ | Duration string | `30m` |
| `components` | ❌ | Array of strings | `[]` (all) |
| `setVariables` | ❌ | Map of string to string | `{}` |

**Validation Layer**: OpenAPI schema

### ExternalCluster Deploy

**When**: `deploy.target: ExternalCluster`

| Field | Required | Validation | Default |
|-------|----------|------------|---------|
| `externalCluster.kubeconfigSecretRef` | ✅ | Must reference existing Secret | `name: prod-kubeconfig` |
| `externalCluster.context` | ❌ | Non-empty string | First context in kubeconfig |
| `namespace` | ❌ | Valid K8s namespace name | `default` |
| `timeout` | ❌ | Duration string | `30m` |

**Validation Layer**: OpenAPI schema + Webhook (secret existence)

**Examples**:
```yaml
# ✅ VALID - External cluster with kubeconfig
deploy:
  target: ExternalCluster
  externalCluster:
    kubeconfigSecretRef:
      name: staging-cluster
    context: staging
  namespace: app-namespace

# ❌ INVALID - External cluster without kubeconfig
deploy:
  target: ExternalCluster
  namespace: app-namespace
  # Error: externalCluster.kubeconfigSecretRef required for ExternalCluster target
```

---

## RBAC Policy Validation

### Policy Fields

All RBAC policy fields are optional. When specified, they restrict the resource usage.

| Field | Type | Validation | Example |
|-------|------|------------|---------|
| `allowedUsers` | `[]string` | Format: `user:email` or `group:name` | `["user:alice@example.com", "group:developers"]` |
| `allowedActions` | `[]Action` | Valid Action enum values | `["Build", "Publish"]` |
| `allowedSourceRepos` | `[]string` | Glob patterns | `["github.com/myorg/*"]` |
| `allowedSourceBuckets` | `[]string` | Glob patterns | `["prod-artifacts-*"]` |
| `allowedSourceRegistries` | `[]string` | Glob patterns | `["ghcr.io/myorg/*"]` |
| `allowedPublishBuckets` | `[]string` | Glob patterns | `["releases-*"]` |
| `allowedPublishRegistries` | `[]string` | Glob patterns | `["registry.example.com/*"]` |
| `allowedDeployTargets` | `[]DeployTargetType` | Valid DeployTargetType enum | `["InCluster"]` |

**Validation Layer**: Webhook (policy engine)

### Policy Hierarchy

Policies are evaluated in order:

1. **Cluster-level policy** (ConfigMap in `forge-system` namespace) - Most restrictive
2. **Namespace-level policy** (ConfigMap in resource namespace) - Overrides resource policy
3. **Resource-level policy** (In ZarfPackage spec) - Least restrictive

**Example Policy Evaluation**:
```yaml
# Cluster policy (most restrictive)
allowedDeployTargets: [InCluster]  # No external deploys allowed cluster-wide

# Namespace policy (namespace admin)
allowedDeployTargets: [InCluster, ExternalCluster]  # Namespace allows both

# Resource policy (user self-service)
allowedDeployTargets: [ExternalCluster]  # User wants external only

# Result: DENIED - Cluster policy forbids ExternalCluster
```

---

## Cross-Field Validation

### Source Type Matching

**Rule**: Exactly one source type config must be provided matching `source.type`

| `source.type` | Required Field | Forbidden Fields |
|---------------|----------------|------------------|
| `Git` | `source.git` | `source.s3`, `source.oci`, `source.local` |
| `S3` | `source.s3` | `source.git`, `source.oci`, `source.local` |
| `OCI` | `source.oci` | `source.git`, `source.s3`, `source.local` |
| `Local` | `source.local` | `source.git`, `source.s3`, `source.oci` |

**Validation Layer**: Webhook

**Examples**:
```yaml
# ✅ VALID
source:
  type: Git
  git:
    url: https://github.com/org/repo
    ref: main

# ❌ INVALID - Type mismatch
source:
  type: Git
  s3:  # Error: s3 config provided but type is Git
    bucket: my-bucket
```

### Publish Destination Type Matching

**Rule**: If `publish` is specified, exactly one destination type config must match `publish.destination.type`

| `destination.type` | Required Field | Forbidden Fields |
|-------------------|----------------|------------------|
| `S3` | `destination.s3` | `destination.oci`, `destination.local` |
| `OCI` | `destination.oci` | `destination.s3`, `destination.local` |
| `Local` | `destination.local` | `destination.s3`, `destination.oci` |

**Validation Layer**: Webhook

---

## Secret Validation

### Secret Existence

**Rule**: All `*SecretRef` fields must reference Secrets that exist in the same namespace (or specified namespace)

**Validation Layer**: Webhook

**Timing**: Validation webhook queries the API server to verify Secret exists

**Examples**:
```yaml
# ✅ VALID - Secret exists
source:
  type: Git
  git:
    url: https://github.com/private/repo
    ref: main
    credentialsSecretRef:
      name: github-token  # Secret exists in same namespace

# ❌ INVALID - Secret doesn't exist
source:
  type: Git
  git:
    url: https://github.com/private/repo
    ref: main
    credentialsSecretRef:
      name: nonexistent-secret  # Error: Secret not found
```

### Secret Type Validation

**Rule**: OCI credential secrets must be type `kubernetes.io/dockerconfigjson`

**Validation Layer**: Webhook

| Secret Reference | Required Type |
|------------------|---------------|
| `git.credentialsSecretRef` | `Opaque` (with `token` or `ssh-key` key) |
| `s3.credentialsSecretRef` | `Opaque` (with `access-key-id` and `secret-access-key`) |
| `oci.credentialsSecretRef` | `kubernetes.io/dockerconfigjson` |
| External cluster kubeconfig | `Opaque` (with `kubeconfig` key) |

---

## DevMode Restrictions

**Rule**: Local sources and destinations require `devMode: true`

**Enforcement**: Webhook validation

**Additional DevMode rules**:
- DevMode must be explicitly enabled (no defaults)
- Recommended: Use namespace labels to mark dev namespaces
- Recommended: Policy can restrict which namespaces allow devMode

**Examples**:
```yaml
# ✅ VALID - DevMode explicitly enabled
source:
  type: Local
  local:
    path: /workspace/pkg
    devMode: true

# ❌ INVALID - DevMode not enabled
source:
  type: Local
  local:
    path: /workspace/pkg
    devMode: false  # Error: devMode must be true for Local sources
```

---

## Validation Error Messages

All validation errors should include:
1. **Field path**: Which field failed validation
2. **Rule violated**: What validation rule was broken
3. **Remediation**: How to fix it

**Example error messages**:

```
Field: spec.deploy
Violation: Deploy configuration not allowed for action 'Build'
Remediation: Remove spec.deploy or change action to 'BuildDeploy' or 'Deploy'

Field: spec.source.git.url
Violation: Git URL must use HTTPS protocol
Remediation: Change URL from 'git@github.com:org/repo.git' to 'https://github.com/org/repo'

Field: spec.source.git.credentialsSecretRef.name
Violation: Secret 'github-token' not found in namespace 'default'
Remediation: Create the Secret or reference an existing Secret

Field: spec.source
Violation: Source type is 'Git' but git configuration is missing
Remediation: Add spec.source.git configuration or change source.type

Field: spec.publish.destination.oci.credentialsSecretRef
Violation: Secret 'registry-creds' must be type 'kubernetes.io/dockerconfigjson' but is 'Opaque'
Remediation: Create a dockerconfigjson Secret using: kubectl create secret docker-registry
```

---

## Validation Implementation Checklist

### CRD OpenAPI Schema (✅ Implemented)

- [x] Action enum validation
- [x] SourceType enum validation
- [x] DestinationType enum validation
- [x] DeployTargetType enum validation
- [x] Required fields marked
- [x] String pattern validation (Git URL HTTPS)
- [x] Field descriptions

### Webhook Validation (❌ TODO)

- [ ] Action-specific required fields (Build requires no publish/deploy, etc.)
- [ ] Source type matching (type=Git requires git config)
- [ ] Publish destination type matching
- [ ] Secret existence validation
- [ ] Secret type validation (dockerconfigjson for OCI)
- [ ] DevMode enforcement for Local sources/destinations
- [ ] RBAC policy evaluation
- [ ] Cross-namespace secret references
- [ ] Glob pattern matching for policy enforcement

### Policy Engine (❌ TODO)

- [ ] User/group extraction from request
- [ ] Cluster-level policy loading
- [ ] Namespace-level policy loading
- [ ] Policy hierarchy evaluation
- [ ] Action authorization
- [ ] Source authorization (repo/bucket/registry patterns)
- [ ] Publish destination authorization
- [ ] Deploy target authorization
- [ ] Audit logging of policy decisions
