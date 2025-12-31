# UDS Bundle Policy Examples

This directory contains example ServiceAccount configurations for UDS bundle operations with varying permission levels.

## Policy Enforcement

Forge enforces UDS bundle policies through ServiceAccount annotations. The admission webhook validates operations before they're created, and the controller re-validates before executing Jobs.

## Available Examples

### 1. Permissive ServiceAccount (`permissive-serviceaccount.yaml`)

**Use Case**: Development environments, trusted users, rapid prototyping

**Permissions**:
- ✅ All actions: create, publish, deploy
- ✅ All Git repositories: `*`
- ✅ All OCI registries: `*`
- ✅ All S3 buckets: `*`
- ✅ Deploy to any namespace: `*`

**Warning**: This configuration is NOT recommended for production. Use only in controlled development environments.

### 2. Restricted ServiceAccount (`restricted-serviceaccount.yaml`)

**Use Case**: Production environments, compliance requirements, least-privilege principle

**Permissions**:
- ✅ Actions: create, deploy only (no external publishing)
- ✅ Git repositories: `github.com/cncf/*`, `github.com/myorg/*`
- ❌ OCI registries: None (internal builds only)
- ❌ S3 buckets: None (local storage only)
- ✅ Deploy namespaces: `uds-dev`, `uds-staging` only

**Features**:
- Read-only RBAC for sensitive resources
- Namespace-scoped deployment permissions
- No external publishing capabilities

### 3. CI/CD ServiceAccount (`ci-cd-serviceaccount.yaml`)

**Use Case**: CI/CD pipelines, build automation, artifact publishing

**Permissions**:
- ✅ Actions: create, publish (no deployment)
- ✅ Git repositories: Company internal only
- ✅ OCI registries: Company registry only
- ✅ S3 buckets: Company buckets only
- ❌ Deploy namespaces: None (separation of concerns)

**Features**:
- Includes example Secrets for OCI, S3, and Git credentials
- Separation of build/publish from deployment
- Suitable for automated pipelines

## Policy Annotations Reference

All annotations use the prefix `forge.forge.dev/`:

| Annotation | Values | Description |
|------------|--------|-------------|
| `allowed-actions` | `create`, `publish`, `deploy` | Comma-separated list of allowed actions |
| `allowed-git-repos` | `*` or glob patterns | Git repository patterns (e.g., `github.com/myorg/*`) |
| `allowed-oci-registries` | `*` or glob patterns | OCI registry patterns (e.g., `registry.example.com/*`) |
| `allowed-s3-buckets` | `*` or bucket names | S3 bucket names (comma-separated) |
| `allowed-deploy-namespaces` | `*` or namespace names | Target deployment namespaces (comma-separated) |

## Glob Pattern Matching

Glob patterns support wildcards:
- `*` matches any characters within a path segment
- `github.com/myorg/*` matches all repos under myorg
- `*.mycompany.com` matches all subdomains
- `registry.example.com/team-*/*` matches all images in team namespaces

## Quick Start

1. **Choose the appropriate policy** for your use case
2. **Customize the annotations** to match your security requirements
3. **Apply the ServiceAccount**:
   ```bash
   kubectl apply -f permissive-serviceaccount.yaml
   ```
4. **Reference in UDSBundleJob**:
   ```yaml
   apiVersion: forge.dev/v1alpha1
   kind: UDSBundleJob
   metadata:
     name: my-bundle
   spec:
     serviceAccountName: uds-bundle-operator-permissive
     action: create
     source:
       type: git
       git:
         url: https://github.com/grafana/grafana
         ref: v10.0.0
   ```

## RBAC Requirements

### Minimum Required Permissions (Namespace-scoped)

```yaml
rules:
  # Create and manage Jobs
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["create", "get", "list", "watch"]

  # Access Secrets for credentials
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get"]

  # Manage ConfigMaps for attestations
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create", "get", "list", "update"]
```

### Deployment Permissions (Cluster-scoped or Namespace-scoped)

For UDS bundle deployment, additional permissions are required in target namespaces. See individual examples for specific RBAC configurations.

## Security Best Practices

1. **Principle of Least Privilege**: Grant only the permissions needed for the specific use case
2. **Separate Concerns**: Use different ServiceAccounts for build, publish, and deploy
3. **Namespace Isolation**: Restrict deployment to specific namespaces
4. **Credential Management**: Use Kubernetes Secrets for sensitive data, rotate regularly
5. **Audit Trail**: Monitor UDSBundleJob resources and Job executions
6. **Validation**: Enable the admission webhook to enforce policies before execution

## Troubleshooting

### Policy Validation Failures

If the webhook rejects your UDSBundleJob:

```bash
# Check ServiceAccount annotations
kubectl get sa -n forge-system uds-bundle-operator-restricted -o yaml

# Verify the action is allowed
# Example: "create" must be in allowed-actions annotation

# Check repository pattern matching
# Example: Git URL must match allowed-git-repos glob pattern
```

### RBAC Permission Errors

If Jobs fail with permission errors:

```bash
# Check ServiceAccount permissions
kubectl auth can-i create jobs --as=system:serviceaccount:forge-system:uds-bundle-cicd -n forge-system

# View Role/RoleBinding
kubectl get role,rolebinding -n forge-system | grep uds-bundle

# Check Job logs for specific permission errors
kubectl logs -n forge-system job/<bundle-name>-create
```

## Migrating from v1alpha1 to v1alpha2

The v1alpha2 API uses the same policy annotations and RBAC model. Only the resource name changes:

- v1alpha1: `UDSBundleJob`
- v1alpha2: `UDSBundleJob`

ServiceAccount configurations remain compatible across both API versions.

## Additional Resources

- [ServiceAccount Reference](../../docs/development/SERVICEACCOUNT_REFERENCE.md)
- [Policy Engine Documentation](../../docs/development/)
- [UDS User Guide](../../docs/getting-started/UDS_GUIDE.md)
