# Security & Kyverno Policy Compliance Plan

## Overview

This plan outlines security improvements to make Forge deployments compatible with strict Kubernetes security policies (Kyverno, Pod Security Standards, OPA Gatekeeper) while reducing overall security risks.

## Current Security Posture

### Strengths

| Control | Status | Details |
|---------|--------|---------|
| Non-root execution | ✅ | All containers run as UID 1000 (Zarf) or 65532 (UDS/system) |
| Privilege escalation | ✅ | `allowPrivilegeEscalation: false` on all containers |
| Capability dropping | ✅ | All containers drop ALL capabilities |
| Seccomp profiles | ✅ | RuntimeDefault on all containers |
| No hostPath mounts | ✅ | No host filesystem access |
| No host namespaces | ✅ | No hostPID, hostIPC, hostNetwork |
| Resource limits | ✅ | All containers have CPU/memory limits |
| RBAC least privilege | ✅ | Scoped permissions, no cluster-admin |

### Gaps Identified

| Issue | Severity | Impact |
|-------|----------|--------|
| Missing `automountServiceAccountToken: false` | High | Kyverno commonly blocks pods without explicit setting |
| Missing `readOnlyRootFilesystem` on some containers | High | PSS restricted profile requires this |
| Init containers missing seccomp profiles | Medium | Inconsistent with main containers |
| `:latest` image tags on init containers | Medium | Violates image tag policies |
| Network policies disabled by default | Medium | No network segmentation |
| EmptyDir volumes without size limits | Low | Potential DoS vector |
| Inconsistent UID (1000 vs 65532) | Low | May conflict with UID range policies |

---

## Phase 1: Critical Kyverno Blockers (High Priority)

### 1.1 Add `automountServiceAccountToken: false`

Many Kyverno policies require explicit declaration of service account token mounting.

**Affected Resources:**
- Controller Deployment
- Webhook Deployment
- User Job Pods (ZarfPackageJob, UDSBundleJob)
- Cert-generator Job
- CABundle-patcher Job

**Implementation:**

```yaml
# chart/forge/templates/controller/deployment.yaml
spec:
  template:
    spec:
      automountServiceAccountToken: true  # Controller needs API access

# chart/forge/templates/webhook/deployment.yaml
spec:
  template:
    spec:
      automountServiceAccountToken: true  # Webhook needs API access

# chart/forge/templates/webhook/tls-secret.yaml (cert-generator)
spec:
  template:
    spec:
      automountServiceAccountToken: true  # Needs to create secrets

# pkg/actions/job_builder.go - User jobs
# Add to PodSpec:
AutomountServiceAccountToken: ptr.To(serviceAccountName != ""),
```

**Rationale:** Explicit declaration satisfies Kyverno `require-pod-probes` type policies.

---

### 1.2 Add `readOnlyRootFilesystem: true` Everywhere

**Currently Missing On:**

| Container | File | Line |
|-----------|------|------|
| Webhook container | `webhook/deployment.yaml` | ~69 |
| Git init container | `pkg/sources/git.go` | ~89 |
| S3 init container | `pkg/sources/s3.go` | ~141 |
| OCI init container | `pkg/sources/oci.go` | ~79 |
| Cert-generator | `webhook/tls-secret.yaml` | ~93 |
| CABundle-patcher | `webhook/cabundle-patch.yaml` | ~94 |
| User job main container | `pkg/actions/job_builder.go` | ~391 |

**Implementation:**

```go
// pkg/actions/job_builder.go - Update NonRootSecurityContextWithUID
func NonRootSecurityContextWithUID(uid int64) *corev1.SecurityContext {
    return &corev1.SecurityContext{
        RunAsNonRoot:             Ptr(true),
        RunAsUser:                Ptr(uid),
        AllowPrivilegeEscalation: Ptr(false),
        ReadOnlyRootFilesystem:   Ptr(true),  // ADD THIS
        Capabilities: &corev1.Capabilities{
            Drop: []corev1.Capability{"ALL"},
        },
        SeccompProfile: &corev1.SeccompProfile{
            Type: corev1.SeccompProfileTypeRuntimeDefault,
        },
    }
}
```

**Writable Paths Needed:**
- Jobs need `/tmp`, `/home/zarf`, `/home/uds` - mount as emptyDir
- Init containers need `/workspace` - already mounted

**Additional Volume Mounts Required:**

```go
// pkg/actions/job_builder.go - Add temp volume
{
    Name:      "tmp",
    MountPath: "/tmp",
}
// Volume:
{
    Name: "tmp",
    VolumeSource: corev1.VolumeSource{
        EmptyDir: &corev1.EmptyDirVolumeSource{
            SizeLimit: resource.NewQuantity(1*1024*1024*1024, resource.BinarySI), // 1Gi
        },
    },
}
```

---

### 1.3 Add Seccomp to All Init Containers

**Currently Missing On:**
- `pkg/sources/git.go` line 89-96
- `pkg/sources/s3.go` line 141-148
- `pkg/sources/oci.go` line 79-86

**Implementation:**

```go
// Add to each init container SecurityContext:
SeccompProfile: &corev1.SeccompProfile{
    Type: corev1.SeccompProfileTypeRuntimeDefault,
},
```

---

## Phase 2: Image Policy Compliance (Medium Priority)

### 2.1 Replace `:latest` Tags with Pinned Versions

**Current Issues:**

| Image | Location | Fix |
|-------|----------|-----|
| `alpine/git:latest` | `pkg/sources/git.go:74` | Use `alpine/git:v2.43.0` |
| `amazon/aws-cli:latest` | `pkg/sources/s3.go:135` | Use `amazon/aws-cli:2.15.0` |
| `bitnami/oras:latest` | `pkg/sources/oci.go:72` | Use `bitnami/oras:1.1.0` |

**Implementation Options:**

1. **Helm Values (Recommended):**
```yaml
# values.yaml
images:
  gitClone: alpine/git:v2.43.0
  awsCli: amazon/aws-cli:2.15.0
  oras: bitnami/oras:1.1.0
```

2. **Controller ConfigMap:**
```yaml
# Pass via environment variables to controller
FORGE_GIT_CLONE_IMAGE: alpine/git:v2.43.0
FORGE_AWS_CLI_IMAGE: amazon/aws-cli:2.15.0
FORGE_ORAS_IMAGE: bitnami/oras:1.1.0
```

---

### 2.2 Add Image Pull Policy Configuration

**Implementation:**

```yaml
# values.yaml
images:
  pullPolicy: Always  # IfNotPresent for dev, Always for prod

# Or per-image:
controller:
  image:
    pullPolicy: Always
webhook:
  image:
    pullPolicy: Always
```

---

## Phase 3: Network Security (Medium Priority)

### 3.1 Enable Network Policies by Default

**Current:** `networkPolicies.enabled: false`

**Recommendation:** Enable by default, document opt-out

**Implementation:**

```yaml
# values.yaml
networkPolicies:
  enabled: true  # Changed from false

  # New: Configurable egress destinations
  egress:
    allowDNS: true
    allowKubeAPI: true
    allowHTTPS: true
    customRules: []
```

**Additional NetworkPolicy for User Jobs:**

```yaml
# chart/forge/templates/networkpolicy-jobs.yaml
{{- if .Values.networkPolicies.enabled }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "forge.fullname" . }}-jobs
spec:
  podSelector:
    matchLabels:
      app: forge
  policyTypes:
    - Egress
  egress:
    # Allow DNS
    - to:
      - namespaceSelector: {}
      ports:
        - protocol: UDP
          port: 53
    # Allow registry access (configurable)
    {{- range .Values.networkPolicies.allowedRegistries }}
    - to:
      - ipBlock:
          cidr: {{ . }}
      ports:
        - protocol: TCP
          port: 443
    {{- end }}
{{- end }}
```

---

## Phase 4: Pod Security Standards Labels (Medium Priority)

### 4.1 Add PSS Labels to Namespace Template

**Implementation:**

```yaml
# chart/forge/templates/namespace.yaml (new file)
{{- if .Values.createNamespace }}
apiVersion: v1
kind: Namespace
metadata:
  name: {{ include "forge.namespace" . }}
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
{{- end }}
```

```yaml
# values.yaml
createNamespace: false  # Opt-in, user creates with labels
podSecurityStandards:
  enforce: restricted
  audit: restricted
  warn: restricted
```

---

## Phase 5: EmptyDir Size Limits (Low Priority)

### 5.1 Add Size Limits to All EmptyDir Volumes

**Implementation:**

```go
// pkg/actions/job_builder.go
const (
    DefaultTmpSizeLimit       = "1Gi"
    DefaultWorkspaceSizeLimit = "10Gi"
    DefaultArtifactSizeLimit  = "50Gi"
)

// When creating volumes:
{
    Name: "tmp",
    VolumeSource: corev1.VolumeSource{
        EmptyDir: &corev1.EmptyDirVolumeSource{
            SizeLimit: resource.MustParse(DefaultTmpSizeLimit),
        },
    },
}
```

---

## Phase 6: UID Standardization (Low Priority)

### 6.1 Document and Standardize UIDs

**Current State:**
- Zarf jobs: UID 1000
- UDS jobs: UID 65532
- System components: UID 65532

**Options:**

1. **Keep as-is, document clearly** (Recommended)
   - Different images have different user expectations
   - Zarf CLI expects UID 1000
   - UDS CLI works with 65532

2. **Add Helm value for override:**
```yaml
# values.yaml
securityContext:
  runAsUser: 65532  # Override for clusters requiring specific UID
```

---

## Implementation Checklist

### Phase 1 (Critical - Do First)

- [ ] Add `automountServiceAccountToken` to all pods
- [ ] Add `readOnlyRootFilesystem: true` to all containers
- [ ] Add `/tmp` emptyDir mount to job containers
- [ ] Add seccomp profiles to all init containers

### Phase 2 (Before Release)

- [ ] Replace `:latest` tags with pinned versions
- [ ] Add image configuration to values.yaml
- [ ] Add imagePullPolicy configuration

### Phase 3 (Production Hardening)

- [ ] Enable network policies by default
- [ ] Add job-specific network policy
- [ ] Document allowed egress destinations

### Phase 4 (Cluster Integration)

- [ ] Add namespace template with PSS labels
- [ ] Document PSS compatibility
- [ ] Add Kyverno policy exception examples

### Phase 5 (Polish)

- [ ] Add emptyDir size limits
- [ ] Add configurable limits in values.yaml

### Phase 6 (Documentation)

- [ ] Document UID requirements
- [ ] Add UID override option
- [ ] Create Kyverno exception policy examples

---

## Kyverno Policy Exception Examples

For clusters that cannot modify Forge to comply, provide exception policies:

```yaml
# kyverno-exceptions/forge-controller.yaml
apiVersion: kyverno.io/v1
kind: PolicyException
metadata:
  name: forge-controller
  namespace: forge-system
spec:
  exceptions:
  - policyName: require-ro-rootfs
    ruleNames:
    - validate-readOnlyRootFilesystem
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
            app.kubernetes.io/component: controller
```

---

## Testing Plan

1. **Unit Tests:**
   - Verify security contexts in job_builder_test.go
   - Verify init container security contexts

2. **Integration Tests:**
   - Deploy to PSS restricted namespace
   - Verify all pods start successfully
   - Run with Kyverno policies enabled

3. **Kyverno Policy Tests:**
   - Test against common Kyverno policy sets:
     - Pod Security Standards (restricted)
     - require-run-as-non-root
     - disallow-privilege-escalation
     - require-ro-rootfs
     - disallow-latest-tag

---

## Impact on kubectl-forge Debugging

### Current State

The kubectl-forge download and debug pods are **already compliant** with most security requirements:

```go
// pkg/kubectl/client.go - Both pods already have:
ReadOnlyRootFilesystem: true
AllowPrivilegeEscalation: false
Capabilities: Drop ALL
SeccompProfile: RuntimeDefault
```

### Potential Debugging Limitations

| Scenario | Impact | Mitigation |
|----------|--------|------------|
| Exec into job pod with `readOnlyRootFilesystem` | Can't install tools (`apt-get`, `apk add`) | Writable `/tmp`, `/home/zarf`, `/workspace` mounts |
| Debug pod (`--copy-workspace`) | Same as above | Writable volume mounts available |
| Download pod | No impact - only reads files | N/A |

### Required Fixes for kubectl-forge

1. **Pin image versions** in `pkg/kubectl/client.go`:
   - Download pod: `busybox:latest` → `busybox:1.36`
   - Debug pod default: `busybox:latest` → `busybox:1.36`

2. **Add `automountServiceAccountToken: false`** to both pods (they don't need API access)

3. **Add `/tmp` emptyDir** to debug pod for tool caching

### Optional: Debug Mode Flag

For cases where users need a fully writable filesystem for debugging, consider adding:

```bash
kubectl forge debug my-job --writable-root
```

This would create a debug pod without `readOnlyRootFilesystem`, but would require a Kyverno exception.

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| `readOnlyRootFilesystem` | Medium - limits debugging | Writable mounts for /tmp, /home, /workspace |
| EmptyDir size limits | Low - may cause OOM | Set generous limits, make configurable |
| Network policies | Medium - may break external access | Document required egress, make configurable |
| Image pinning | Low - maintenance burden | Automate version updates via Renovate/Dependabot |
