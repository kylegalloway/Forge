# ServiceAccount Annotations Reference

Complete reference for Forge ServiceAccount policy annotations.

## Overview

Forge uses ServiceAccount annotations to define fine-grained permissions for ZarfPackageJob operations. This enables cluster administrators to control what actions users can perform and which resources they can access.

## Security Model

1. **Deny by default**: If an annotation is missing, the operation is denied
2. **Explicit allow**: Each permission must be explicitly granted
3. **Glob pattern matching**: Supports wildcards for flexible policies
4. **Admission-time validation**: Webhook enforces policies before resource creation

## Annotation Reference

### forge.dev/allowed-actions

**Purpose:** Controls which actions a ServiceAccount can perform.

**Format:** Comma-separated list of action names

**Valid Values:**

- `Build` - Build Zarf packages from source
- `Publish` - Publish built packages to registries/buckets
- `Deploy` - Deploy packages to clusters
- `BuildPublish` - Chain Build + Publish
- `BuildDeploy` - Chain Build + Deploy
- `PublishDeploy` - Chain Publish + Deploy
- `BuildPublishDeploy` - Chain all three
- `*` - Allow all actions (use with caution)

**Examples:**

```yaml
# Development team - can only build
annotations:
  forge.dev/allowed-actions: "Build"

# CI/CD pipeline - can build and publish
annotations:
  forge.dev/allowed-actions: "Build,Publish"

# Production deployer - can only deploy pre-built packages
annotations:
  forge.dev/allowed-actions: "Deploy"

# Platform team - full permissions
annotations:
  forge.dev/allowed-actions: "*"
```text

**Error Example:**

```text
action Deploy is not allowed (allowed actions: [Build,Publish]) for ServiceAccount dev-sa
```text

---

### forge.dev/allowed-source-repos

**Purpose:** Controls which Git repositories can be used as package sources.

**Format:** Comma-separated glob patterns

**Pattern Syntax:**

- Exact match: `https://github.com/myorg/myrepo`
- Wildcard: `https://github.com/myorg/*` (matches all repos under myorg)
- All: `*` (matches everything - not recommended)

**Examples:**

```yaml
# Single repository
annotations:
  forge.dev/allowed-source-repos: "https://github.com/stefanprodan/podinfo"

# Organization-wide
annotations:
  forge.dev/allowed-source-repos: "https://github.com/myorg/*"

# Multiple organizations
annotations:
  forge.dev/allowed-source-repos: "https://github.com/myorg/*,https://github.com/platform/*"

# Private GitLab
annotations:
  forge.dev/allowed-source-repos: "https://gitlab.company.com/infra/*"
```text

**Required For:** `source.type: Git`

**Error Example:**

```text
Git repo https://github.com/other/repo is not allowed (allowed repos: [https://github.com/myorg/*])
```text

---

### forge.dev/allowed-source-buckets

**Purpose:** Controls which S3 buckets can be used as package sources.

**Format:** Comma-separated glob patterns

**Pattern Syntax:**

- Exact: `my-artifacts-bucket`
- Prefix match: `my-artifacts-*`
- All: `*`

**Examples:**

```yaml
# Single bucket
annotations:
  forge.dev/allowed-source-buckets: "prod-zarf-packages"

# Environment-based
annotations:
  forge.dev/allowed-source-buckets: "dev-packages-*,staging-packages-*"

# All buckets with prefix
annotations:
  forge.dev/allowed-source-buckets: "zarf-*"
```text

**Required For:** `source.type: S3`

**Error Example:**

```text
S3 bucket prod-bucket is not allowed (allowed buckets: [dev-*,staging-*])
```text

---

### forge.dev/allowed-source-registries

**Purpose:** Controls which OCI registries can be used as package sources.

**Format:** Comma-separated glob patterns

**Pattern Syntax:**

- Exact: `ghcr.io/myorg/package`
- Repository match: `ghcr.io/myorg/*`
- Registry-wide: `ghcr.io/*`

**Examples:**

```yaml
# Specific package
annotations:
  forge.dev/allowed-source-registries: "localhost/packages/zarf"

# Organization packages
annotations:
  forge.dev/allowed-source-registries: "ghcr.io/myorg/*"

# Multiple registries
annotations:
  forge.dev/allowed-source-registries: "ghcr.io/myorg/*,registry.company.com/*"

# Docker Hub
annotations:
  forge.dev/allowed-source-registries: "docker.io/myorg/*"
```text

**Required For:** `source.type: OCI`

**Error Example:**

```text
OCI image ghcr.io/other/package is not allowed (allowed registries: [ghcr.io/myorg/*])
```text

---

### forge.dev/allowed-publish-buckets

**Purpose:** Controls which S3 buckets packages can be published to.

**Format:** Comma-separated glob patterns

**Examples:**

```yaml
# Development artifacts
annotations:
  forge.dev/allowed-publish-buckets: "dev-artifacts-*"

# Production and staging
annotations:
  forge.dev/allowed-publish-buckets: "prod-artifacts,staging-artifacts"

# Customer-specific
annotations:
  forge.dev/allowed-publish-buckets: "customer-*-packages"
```text

**Required For:** `publish.destination.type: S3`

**Error Example:**

```text
S3 bucket wrong-bucket is not allowed for publishing (allowed buckets: [artifacts-*])
```text

---

### forge.dev/allowed-publish-registries

**Purpose:** Controls which OCI registries packages can be published to.

**Format:** Comma-separated glob patterns

**Examples:**

```yaml
# Organization registry
annotations:
  forge.dev/allowed-publish-registries: "ghcr.io/myorg/*"

# Multi-registry
annotations:
  forge.dev/allowed-publish-registries: "ghcr.io/myorg/*,harbor.company.com/packages/*"

# Environment-specific
annotations:
  forge.dev/allowed-publish-registries: "registry.company.com/dev/*,registry.company.com/staging/*"
```text

**Required For:** `publish.destination.type: OCI`

**Error Example:**

```text
OCI registry ghcr.io/other/* is not allowed for publishing (allowed registries: [ghcr.io/myorg/*])
```text

---

### forge.dev/allowed-deploy-targets

**Purpose:** Controls where packages can be deployed.

**Format:** Comma-separated list of deployment targets

**Valid Values:**

- `InCluster` - Deploy to the same cluster where Forge runs
- `ExternalCluster` - Deploy to external clusters (requires kubeconfig)
- `*` - Allow both (use with caution)

**Examples:**

```yaml
# Local deployments only
annotations:
  forge.dev/allowed-deploy-targets: "InCluster"

# External deployments only (production patterns)
annotations:
  forge.dev/allowed-deploy-targets: "ExternalCluster"

# Both
annotations:
  forge.dev/allowed-deploy-targets: "InCluster,ExternalCluster"
```text

**Required For:** `deploy.target: InCluster` or `ExternalCluster`

**Error Example:**

```text
deploy target ExternalCluster is not allowed (allowed targets: [InCluster])
```text

---

### forge.dev/allow-local-sources

**Purpose:** Enables local filesystem sources (development/testing only).

**Format:** String boolean (`"true"` or `"false"`)

**Security Warning:** ⚠️ Only enable for development/testing. Never in production.

**Examples:**

```yaml
# Enable local sources (DEV ONLY)
annotations:
  forge.dev/allow-local-sources: "true"

# Explicitly deny (default behavior)
annotations:
  forge.dev/allow-local-sources: "false"
```text

**Required For:** `source.type: Local` or `publish.destination.type: Local`

**Error Example:**

```text
local sources are not allowed (set annotation forge.dev/allow-local-sources: true for dev mode)
```text

---

## Common Patterns

### Developer ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: developer-sa
  namespace: dev-team
  annotations:
    # Can only build packages
    forge.dev/allowed-actions: "Build"

    # Can use team repositories
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"

    # Can publish to dev registry
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/dev/*"

    # No deployment permissions
```text

### CI/CD Pipeline ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cicd-pipeline-sa
  namespace: cicd
  annotations:
    # Build and publish
    forge.dev/allowed-actions: "BuildPublish"

    # Organization repos
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"

    # Publish to specific registries
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/packages/*"

    # Publish to staging bucket
    forge.dev/allowed-publish-buckets: "staging-artifacts"
```text

### Production Deployer ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prod-deployer-sa
  namespace: production
  annotations:
    # Only deploy, no build
    forge.dev/allowed-actions: "Deploy"

    # Only from production registry
    forge.dev/allowed-source-registries: "ghcr.io/myorg/prod/*"

    # Only to external clusters
    forge.dev/allowed-deploy-targets: "ExternalCluster"
```text

### Platform Team ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: platform-team-sa
  namespace: platform
  annotations:
    # Full permissions
    forge.dev/allowed-actions: "*"
    forge.dev/allowed-source-repos: "*"
    forge.dev/allowed-source-buckets: "*"
    forge.dev/allowed-source-registries: "*"
    forge.dev/allowed-publish-buckets: "*"
    forge.dev/allowed-publish-registries: "*"
    forge.dev/allowed-deploy-targets: "*"
```text

### Testing/Development ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dev-testing-sa
  namespace: dev
  annotations:
    # All actions
    forge.dev/allowed-actions: "Build,Publish,Deploy"

    # Any repository (development)
    forge.dev/allowed-source-repos: "*"

    # Enable local sources for testing
    forge.dev/allow-local-sources: "true"

    # Dev registry
    forge.dev/allowed-publish-registries: "ghcr.io/myorg/dev/*"

    # In-cluster deployments
    forge.dev/allowed-deploy-targets: "InCluster"
```text

---

## Validation Workflow

1. **User creates ZarfPackageJob** with `spec.serviceAccountName: my-sa`

2. **Webhook validates** (admission time):
   - ServiceAccount exists
   - Has required annotations for the action
   - Source/destination match patterns

3. **Controller validates** (reconciliation time):
   - Re-checks ServiceAccount permissions
   - Validates credentials if needed

4. **Job executes** with proper credentials and permissions

---

## Best Practices

### Principle of Least Privilege

✅ **DO:**

- Grant minimal required permissions
- Use specific patterns over wildcards
- Separate ServiceAccounts by role/team
- Regular audit of permissions

❌ **DON'T:**

- Use `*` wildcard unless absolutely necessary
- Share ServiceAccounts across teams
- Grant local source access in production
- Use overly broad patterns

### Pattern Design

**Good Patterns:**

```yaml
# Specific organization
forge.dev/allowed-source-repos: "https://github.com/myorg/*"

# Environment-based
forge.dev/allowed-publish-buckets: "dev-artifacts-*,staging-artifacts-*"

# Team-scoped
forge.dev/allowed-source-registries: "ghcr.io/myorg/team-platform/*"
```text

**Avoid:**

```yaml
# Too permissive
forge.dev/allowed-source-repos: "*"
forge.dev/allowed-source-repos: "https://*/*"

# Overly specific (limits flexibility)
forge.dev/allowed-source-repos: "https://github.com/myorg/one-specific-repo"
```text

### Multi-Environment Strategy

**Development:**

- Broad source access (team repos)
- Dev-only publish destinations
- Local sources allowed
- In-cluster deployment only

**Staging:**

- Specific source patterns
- Staging registries/buckets
- No local sources
- External cluster deployment

**Production:**

- Deploy-only (no build)
- Production registries only
- Strict validation
- External clusters only

---

## Troubleshooting

See [TROUBLESHOOTING.md](../operations/TROUBLESHOOTING.md) for common policy-related issues and solutions.

---

*Last Updated: 2025-11-20*
*Version: 1.0.0*
