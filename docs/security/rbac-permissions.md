# RBAC Permissions Audit

This document provides a detailed audit of Forge's RBAC permissions, demonstrating adherence to least privilege principles.

## Controller Permissions

The Forge controller requires the following permissions to manage ZarfPackageJobs and UDSBundleJobs:

### Custom Resource Permissions

| Resource | Verbs | Justification | Scope |
|----------|-------|---------------|-------|
| `zarfpackagejobs` | `get`, `list`, `watch` | Monitor job resources for reconciliation | Read-only |
| `zarfpackagejobs/status` | `get`, `update`, `patch` | Update job status (phase, message, operation status) | Status subresource only |
| `udsbundlejobs` | `get`, `list`, `watch` | Monitor bundle job resources for reconciliation | Read-only |
| `udsbundlejobs/status` | `get`, `update`, `patch` | Update bundle job status | Status subresource only |

**Why no `create` or `delete`?** Users create jobs via kubectl/API. Controller only manages existing jobs.

### Core Resource Permissions

| Resource | Verbs | Justification | Notes |
|----------|-------|---------------|-------|
| `serviceaccounts` | `get`, `list`, `watch` | Validate ServiceAccount annotations for RBAC policy enforcement | Read-only, no modifications |
| `secrets` | `get`, `list` | Validate credentials exist (Git, OCI registry, S3) | Read-only, secrets managed externally |
| `jobs` | `create`, `get`, `list`, `watch` | Create Kubernetes Jobs for build/publish/deploy actions | No `delete` - Jobs cleaned up via TTL |
| `pods` | `get`, `list`, `watch` | Monitor pod status for job progress tracking | Read-only |
| `events` | `create`, `patch` | Emit Kubernetes events for audit trail and debugging | Write-only |
| `persistentvolumeclaims` | `create`, `get`, `list`, `watch`, `delete` | Manage artifact PVCs for multi-action jobs | Delete added for `retainArtifactPVC` feature |

### Leader Election

| Resource | Verbs | Justification |
|----------|-------|---------------|
| `leases` (coordination.k8s.io) | `create`, `get`, `list`, `update` | Required for HA controller deployments with leader election |

## Webhook Permissions

The Forge validating webhook requires minimal permissions:

| Resource | Verbs | Justification |
|----------|-------|---------------|
| `serviceaccounts` | `get`, `list` | Validate ServiceAccount exists and has required annotations |
| `zarfpackagejobs` | `get`, `list` | Validate job spec against RBAC policies |

**Why so minimal?** Webhooks validate requests but don't modify cluster state.

## Least Privilege Analysis

### ✅ Permissions We DO NOT Have

- ❌ No `delete` on Jobs (cleaned up via `ttlSecondsAfterFinished`)
- ❌ No `update` on ServiceAccounts (read-only policy validation)
- ❌ No `create/delete` on Secrets (users manage credentials)
- ❌ No cluster-admin or wildcard permissions
- ❌ No access to ConfigMaps (except for scanning config)
- ❌ No access to Deployments, StatefulSets, etc.
- ❌ No access to Nodes, ClusterRoles, or other cluster-level resources

### ✅ Why We Need Each Permission

1. **PVC `delete`**: Added for `retainArtifactPVC=false` feature
2. **Secrets `get`**: Validate credentials exist before job creation (fail fast)
3. **ServiceAccounts `get`**: RBAC policy enforcement via annotations
4. **Jobs `create`**: Core functionality - execute Zarf/UDS operations
5. **Events `create`**: Audit trail and debugging
6. **Leases `update`**: HA controller leader election

## Namespace-Scoped Option

By default, Forge uses ClusterRole for cross-namespace operation. For tighter security, deploy with namespace-scoped permissions:

```yaml
rbac:
  create: true
  clusterWide: false  # Use Role instead of ClusterRole
  namespace: forge-jobs  # Limit to specific namespace
```

This creates `Role` and `RoleBinding` instead of `ClusterRole` and `ClusterRoleBinding`.

## Security Recommendations

### 1. Namespace Isolation

For multi-tenant environments, deploy separate Forge instances per tenant namespace:

```bash
helm install forge-team-a forge/forge --namespace team-a --set rbac.clusterWide=false
helm install forge-team-b forge/forge --namespace team-b --set rbac.clusterWide=false
```

### 2. ServiceAccount Annotations

Control user permissions via ServiceAccount annotations:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: limited-user
  annotations:
    forge.dev/allowed-actions: "Build"  # Only allow builds
    forge.dev/allowed-source-repos: "https://github.com/myorg/*"
    forge.dev/allowed-deploy-targets: "InCluster"
```

### 3. Audit RBAC Changes

Monitor RBAC changes with kubectl:

```bash
kubectl get clusterroles -l app.kubernetes.io/name=forge -o yaml
kubectl describe clusterrolebinding forge-controller-rolebinding
```

### 4. Regular Permission Audits

Use tools like `kubectl-who-can` to audit effective permissions:

```bash
kubectl who-can create jobs --all-namespaces
kubectl who-can delete persistentvolumeclaims
```

## Compliance

Forge's RBAC configuration aligns with:

- **CIS Kubernetes Benchmark**: Section 5.1 (RBAC and Service Accounts)
- **NSA/CISA Kubernetes Hardening Guide**: Minimize permissions
- **SOC 2**: Principle of least privilege

## Future Enhancements

From `TODO.md`:

- [ ] Per-action RBAC (separate roles for build vs deploy)
- [ ] Namespace-scoped Role support (implemented via values.yaml)
- [ ] RBAC escalation prevention (restrict ServiceAccount creation)
- [ ] Custom RBAC roles per tenant
- [ ] Permission documentation per action type

## Permission Matrix

| Action | ServiceAccounts | Secrets | Jobs | PVCs | CRDs |
|--------|----------------|---------|------|------|------|
| Build Package | ✓ (read) | ✓ (read) | ✓ (create) | ✓ (create) | ✓ (status update) |
| Publish Package | ✓ (read) | ✓ (read) | ✓ (create) | ✓ (read) | ✓ (status update) |
| Deploy Package | ✓ (read) | ✓ (read) | ✓ (create) | ✓ (read) | ✓ (status update) |
| Validate (Webhook) | ✓ (read) | - | - | - | ✓ (read) |
| Cleanup PVCs | - | - | - | ✓ (delete) | - |

## References

- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [RBAC Good Practices](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
- [Principle of Least Privilege](https://en.wikipedia.org/wiki/Principle_of_least_privilege)
