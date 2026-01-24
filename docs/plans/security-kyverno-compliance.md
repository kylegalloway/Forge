# Security & Kyverno Policy Compliance

**Status:** ✅ Implemented

## Summary

Forge is now compliant with Kubernetes Pod Security Standards (restricted) and common Kyverno policies.

## Security Controls Implemented

| Control | Status | Details |
|---------|--------|---------|
| Non-root execution | ✅ | UID 1000 (Zarf) or 65532 (UDS/system) |
| `allowPrivilegeEscalation: false` | ✅ | All containers |
| `capabilities: drop ALL` | ✅ | All containers |
| `seccompProfile: RuntimeDefault` | ✅ | All containers and init containers |
| `readOnlyRootFilesystem: true` | ✅ | All containers (with writable /tmp, /workspace) |
| `automountServiceAccountToken` | ✅ | Explicitly set on all pods |
| No hostPath mounts | ✅ | No host filesystem access |
| No host namespaces | ✅ | No hostPID, hostIPC, hostNetwork |
| Resource limits | ✅ | All containers have CPU/memory limits |
| EmptyDir size limits | ✅ | /tmp (1Gi), workspace (10Gi), output (10Gi) |
| Pinned image versions | ✅ | No `:latest` tags |
| Network policies | ✅ | Enabled by default |
| RBAC least privilege | ✅ | Scoped permissions, no cluster-admin |

## Image Versions

All init container images are pinned and configurable via environment variables:

| Image | Default Version | Environment Variable |
|-------|-----------------|---------------------|
| alpine/git | v2.43.0 | `FORGE_GIT_CLONE_IMAGE` |
| amazon/aws-cli | 2.15.0 | `FORGE_AWS_CLI_IMAGE` |
| crane | v0.19.0 | `FORGE_CRANE_IMAGE` |
| busybox | 1.36 | (constants.ImageBusybox) |

## Debugging with readOnlyRootFilesystem

Writable paths available for debugging:

| Path | Size Limit | Purpose |
|------|------------|---------|
| `/tmp` | 1Gi | Temporary files, tool caches |
| `/workspace` | 10Gi | Build artifacts, source code |
| `/home/zarf` or `/home/uds` | via HOME env | User home directory |

## Remaining Optional Items

- [ ] Add `--writable-root` flag to `kubectl forge debug` for edge cases
- [ ] Add job-specific NetworkPolicy template
- [ ] Create Kyverno PolicyException examples for documentation

## Kyverno Policy Exception Example

For clusters with stricter policies that Forge can't satisfy:

```yaml
apiVersion: kyverno.io/v1
kind: PolicyException
metadata:
  name: forge-controller
  namespace: forge-system
spec:
  exceptions:
  - policyName: <policy-name>
    ruleNames:
    - <rule-name>
  match:
    any:
    - resources:
        kinds:
        - Pod
        namespaces:
        - forge-system
        selector:
          matchLabels:
            app.kubernetes.io/name: forge
```
