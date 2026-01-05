# Pod Security Standards Compliance

Forge implements Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) to ensure secure execution of workloads.

## Enforcement

Pod Security Standards are enforced at the namespace level using Pod Security admission labels:

```yaml
pod-security.kubernetes.io/enforce: restricted
pod-security.kubernetes.io/audit: restricted
pod-security.kubernetes.io/warn: restricted
```

These labels are automatically applied to the `forge-system` namespace when deployed via Helm chart.

## Compliance Matrix

| Control | Requirement | Forge Implementation | Status |
|---------|------------|---------------------|--------|
| **Running as Non-root** | Containers must run as non-root user | `runAsNonRoot: true`, `runAsUser: 1000` (Zarf) or `65532` (UDS) | ✅ Compliant |
| **Privilege Escalation** | `allowPrivilegeEscalation` must be false | `allowPrivilegeEscalation: false` | ✅ Compliant |
| **Capabilities** | Must drop ALL capabilities | `capabilities.drop: [ALL]` | ✅ Compliant |
| **Seccomp Profile** | Must use RuntimeDefault or Localhost | `seccompProfile.type: RuntimeDefault` | ✅ Compliant |
| **Host Namespaces** | Cannot use host namespaces (PID, IPC, Network) | Not used | ✅ Compliant |
| **Privileged Containers** | Cannot run privileged containers | `privileged: false` (default) | ✅ Compliant |
| **Host Ports** | Cannot use host ports | Not used | ✅ Compliant |
| **AppArmor** | Must use runtime/default or custom profile | Uses runtime default | ✅ Compliant |
| **SELinux** | Must use allowed types | Uses defaults | ✅ Compliant |
| **Volumes** | Limited volume types allowed | Uses PersistentVolumeClaims, EmptyDir, ConfigMap, Secret | ✅ Compliant |
| **Read-only Root Filesystem** | Recommended but not required | Not enforced (requires writable /tmp) | ⚠️ Optional |

## Job Pod Security Context

All Forge-created job pods use the following security context:

### Pod-Level Security Context

```go
SecurityContext: &corev1.PodSecurityContext{
    RunAsNonRoot: true,
    RunAsUser:    1000,  // or 65532 for UDS
    FSGroup:      1000,  // or 65532 for UDS
    SeccompProfile: &corev1.SeccompProfile{
        Type: corev1.SeccompProfileTypeRuntimeDefault,
    },
}
```

### Container-Level Security Context

```go
SecurityContext: &corev1.SecurityContext{
    RunAsNonRoot:             true,
    RunAsUser:                1000,  // or 65532 for UDS
    AllowPrivilegeEscalation: false,
    Capabilities: &corev1.Capabilities{
        Drop: []corev1.Capability{"ALL"},
    },
    SeccompProfile: &corev1.SeccompProfile{
        Type: corev1.SeccompProfileTypeRuntimeDefault,
    },
}
```

## Controller and Webhook Security

The Forge controller and webhook deployments also comply with the restricted Pod Security Standard:

- Run as non-root (UID 65532)
- Drop all capabilities
- Use RuntimeDefault seccomp profile
- Disable privilege escalation
- Set FSGroup for volume permissions

## Configuration

Pod Security Standards enforcement can be configured in the Helm chart:

```yaml
podSecurityStandards:
  enforce: restricted  # restricted, baseline, or privileged
  audit: restricted
  warn: restricted
```

## Best Practices

1. **Never override security contexts** - The default security contexts meet strict security requirements
2. **Use volume mounts for writable space** - Instead of relaxing read-only root filesystem
3. **Monitor PSS violations** - Check `kubectl get events` for Pod Security warnings
4. **Test in audit mode first** - Use `audit: restricted` before `enforce: restricted` in new environments

## References

- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/)
- [NSA/CISA Kubernetes Hardening Guide](https://media.defense.gov/2022/Aug/29/2003066362/-1/-1/0/CTR_KUBERNETES_HARDENING_GUIDANCE_1.2_20220829.PDF)
