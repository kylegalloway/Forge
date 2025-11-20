# Image Security for ScriptRunner

This document describes the container image security strategy for ScriptRunner deployments.

## Overview

ScriptRunner executes untrusted code in Kubernetes Jobs. Container images are a critical attack surface:
- Vulnerable base images can be exploited
- Unsigned images may be tampered with
- Public registries can serve malicious images
- Stale images miss critical security patches

## Security Goals

1. **Vulnerability-free images**: Scan all images for CVEs
2. **Image provenance**: Verify images are from trusted sources
3. **Supply chain security**: Detect tampering and unauthorized modifications
4. **Registry access control**: Restrict to approved registries
5. **Patch management**: Keep images up-to-date

## Strategy Overview

```
┌─────────────────────────────────────────────────────────────┐
│              Image Security Layers                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. Admission Webhook                                        │
│     ↓ Validates image registry prefix                        │
│     ↓ Enforces approved registries only                      │
│                                                               │
│  2. Image Scanning (CI/CD)                                   │
│     ↓ Scans for CVEs before push                             │
│     ↓ Blocks images with HIGH/CRITICAL vulnerabilities       │
│                                                               │
│  3. Image Signing                                            │
│     ↓ Signs images with cosign/notary                        │
│     ↓ Policy controller verifies signatures                  │
│                                                               │
│  4. Pull Policy Enforcement                                  │
│     ↓ Always pull from registry (no stale local images)      │
│     ↓ Or IfNotPresent for immutable tags                     │
│                                                               │
│  5. Registry Authentication                                  │
│     ↓ ImagePullSecrets for private registries                │
│     ↓ Scoped to user namespaces                              │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Implementation

### 1. Image Registry Validation (Webhook)

**Status**: ✅ Implemented in Batch 2

The admission webhook validates images against an approved registry list:

```go
// pkg/webhook/validator.go
func (v *Validator) validateImage(image string) error {
    if v.config.AllowedRegistries != nil && len(v.config.AllowedRegistries) > 0 {
        allowed := false
        for _, registry := range v.config.AllowedRegistries {
            if strings.HasPrefix(image, registry) {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("image %s is not from an approved registry", image)
        }
    }
    return nil
}
```

**Configuration** (`webhook/deploy/webhook-config.yaml`):
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: scriptrunner-webhook-config
  namespace: scriptrunner-system
data:
  config.json: |
    {
      "allowedRegistries": [
        "myregistry.io/scriptrunner/",
        "ghcr.io/myorg/",
        "registry.example.com/approved/"
      ]
    }
```

**Update allowed registries**:
```bash
kubectl edit configmap scriptrunner-webhook-config -n scriptrunner-system
# Restart webhook to reload config
kubectl rollout restart deployment scriptrunner-webhook -n scriptrunner-system
```

### 2. Image Vulnerability Scanning

**Status**: 📋 Documentation (CI/CD implementation in Batch 6)

#### Option A: Trivy (Recommended)

**Installation**:
```bash
# Install trivy
brew install aquasecurity/trivy/trivy  # macOS
# or
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -
echo "deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/trivy.list
sudo apt-get update && sudo apt-get install trivy
```

**Scan image**:
```bash
trivy image --severity HIGH,CRITICAL myregistry.io/scriptrunner/process-data:v1.0.0

# Exit with error if vulnerabilities found
trivy image --exit-code 1 --severity CRITICAL myregistry.io/scriptrunner/process-data:v1.0.0
```

**CI/CD Integration** (GitHub Actions):
```yaml
name: Build and Scan

on:
  push:
    branches: [ main ]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3

    - name: Build image
      run: docker build -t myregistry.io/scriptrunner/process-data:${{ github.sha }} .

    - name: Scan with Trivy
      uses: aquasecurity/trivy-action@master
      with:
        image-ref: myregistry.io/scriptrunner/process-data:${{ github.sha }}
        severity: 'CRITICAL,HIGH'
        exit-code: '1'  # Fail build if vulnerabilities found

    - name: Push image (only if scan passes)
      run: |
        echo "${{ secrets.REGISTRY_PASSWORD }}" | docker login myregistry.io -u "${{ secrets.REGISTRY_USERNAME }}" --password-stdin
        docker push myregistry.io/scriptrunner/process-data:${{ github.sha }}
```

#### Option B: Clair

**Installation**:
```bash
# Deploy Clair to Kubernetes
kubectl apply -f https://raw.githubusercontent.com/quay/clair/main/contrib/k8s/clair-kubernetes.yaml
```

**Scan image**:
```bash
# Use clairctl
clairctl analyze myregistry.io/scriptrunner/process-data:v1.0.0
clairctl report myregistry.io/scriptrunner/process-data:v1.0.0
```

#### Option C: Cloud Provider Solutions

**AWS ECR Image Scanning**:
```bash
# Enable scanning on push
aws ecr put-image-scanning-configuration \
    --repository-name scriptrunner/process-data \
    --image-scanning-configuration scanOnPush=true

# View scan results
aws ecr describe-image-scan-findings \
    --repository-name scriptrunner/process-data \
    --image-id imageTag=v1.0.0
```

**GCP Artifact Analysis**:
```bash
# Enable vulnerability scanning
gcloud container images scan myregistry.io/scriptrunner/process-data:v1.0.0

# View results
gcloud container images list-tags myregistry.io/scriptrunner/process-data \
    --format="get(name,vulnerabilities)"
```

### 3. Image Signing and Verification

**Status**: 📋 Documentation (CI/CD implementation in Batch 6)

#### Cosign (Recommended for Kubernetes)

**Installation**:
```bash
# Install cosign
brew install sigstore/tap/cosign  # macOS
# or
wget https://github.com/sigstore/cosign/releases/download/v2.0.0/cosign-linux-amd64
chmod +x cosign-linux-amd64 && sudo mv cosign-linux-amd64 /usr/local/bin/cosign
```

**Generate signing keys**:
```bash
# Generate key pair (store private key securely!)
cosign generate-key-pair

# Or use keyless signing (recommended)
# Uses OIDC identity (GitHub, Google, etc.)
export COSIGN_EXPERIMENTAL=1
```

**Sign images**:
```bash
# With key pair
cosign sign --key cosign.key myregistry.io/scriptrunner/process-data:v1.0.0

# Keyless (uses OIDC)
cosign sign myregistry.io/scriptrunner/process-data:v1.0.0
```

**Verify signatures**:
```bash
# With public key
cosign verify --key cosign.pub myregistry.io/scriptrunner/process-data:v1.0.0

# Keyless
cosign verify myregistry.io/scriptrunner/process-data:v1.0.0
```

**Kubernetes Policy Controller**:

Install Sigstore Policy Controller to enforce signatures:

```bash
kubectl apply -f https://github.com/sigstore/policy-controller/releases/latest/download/policy-controller.yaml
```

Create ClusterImagePolicy:
```yaml
apiVersion: policy.sigstore.dev/v1beta1
kind: ClusterImagePolicy
metadata:
  name: scriptrunner-image-policy
spec:
  images:
  - glob: "myregistry.io/scriptrunner/**"
  authorities:
  - keyless:
      url: https://fulcio.sigstore.dev
      identities:
      - issuer: https://github.com/login/oauth
        subject: https://github.com/myorg/scriptrunner
```

Now unsigned images from `myregistry.io/scriptrunner/**` will be rejected.

#### Notary (Docker Content Trust)

**Enable Docker Content Trust**:
```bash
export DOCKER_CONTENT_TRUST=1
export DOCKER_CONTENT_TRUST_SERVER=https://notary.myregistry.io
```

**Sign on push**:
```bash
docker push myregistry.io/scriptrunner/process-data:v1.0.0
# Will prompt for signing key passphrase
```

**Verify**:
```bash
docker pull myregistry.io/scriptrunner/process-data:v1.0.0
# Fails if signature invalid
```

### 4. Image Pull Policy Enforcement

**Current Implementation**:

The controller sets `imagePullPolicy` when creating Jobs:

```go
// pkg/controller/controller.go
container := corev1.Container{
    Name:            "script-executor",
    Image:           image,
    ImagePullPolicy: corev1.PullIfNotPresent,  // Current default
    // ...
}
```

**Recommended Policies**:

| Tag Pattern | Pull Policy | Rationale |
|-------------|-------------|-----------|
| `latest` | Always | Latest tag is mutable |
| Semver (v1.2.3) | IfNotPresent | Immutable, can cache |
| SHA digest | IfNotPresent | Immutable by definition |
| Branch names | Always | Mutable, rebuild on push |

**Update controller** to enforce policy:

```go
func getImagePullPolicy(image string) corev1.PullPolicy {
    // If image uses :latest tag, always pull
    if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
        return corev1.PullAlways
    }

    // If image uses SHA digest, can cache
    if strings.Contains(image, "@sha256:") {
        return corev1.PullIfNotPresent
    }

    // If image uses semver tag (v1.2.3), can cache
    if matched, _ := regexp.MatchString(`:[v]?\d+\.\d+\.\d+$`, image); matched {
        return corev1.PullIfNotPresent
    }

    // Default: always pull (conservative)
    return corev1.PullAlways
}
```

**Configuration Option** (via CRD):

```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
spec:
  image: myregistry.io/scripts:v1.0.0
  imagePullPolicy: IfNotPresent  # Allow user to override
  scriptRef: /scripts/process-data.sh
```

### 5. Private Registry Authentication

**Create ImagePullSecret**:

```bash
# Docker Hub
kubectl create secret docker-registry regcred \
  --docker-server=https://index.docker.io/v1/ \
  --docker-username=myusername \
  --docker-password=mypassword \
  --docker-email=myemail@example.com \
  -n user-alice

# Private registry
kubectl create secret docker-registry myregistry-secret \
  --docker-server=myregistry.io \
  --docker-username=myusername \
  --docker-password=mypassword \
  -n user-alice
```

**Option 1: Per-namespace default** (Recommended):

Add to namespace template:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: scriptrunner-job-sa
  namespace: user-{{ .Username }}
imagePullSecrets:
- name: myregistry-secret
```

Update controller to use ServiceAccount:
```go
job := &batchv1.Job{
    Spec: batchv1.JobSpec{
        Template: corev1.PodTemplateSpec{
            Spec: corev1.PodSpec{
                ServiceAccountName: "scriptrunner-job-sa",  // Add this
                Containers: []corev1.Container{...},
            },
        },
    },
}
```

**Option 2: Per-ScriptRunner**:

Allow users to specify in CRD:
```yaml
apiVersion: scriptrunner.io/v1alpha1
kind: ScriptRunner
metadata:
  name: my-task
spec:
  image: myregistry.io/scripts:v1.0.0
  imagePullSecrets:
  - name: myregistry-secret
  scriptRef: /scripts/process-data.sh
```

Update controller to use it:
```go
var imagePullSecrets []corev1.LocalObjectReference
if len(scriptRunner.Spec.ImagePullSecrets) > 0 {
    imagePullSecrets = scriptRunner.Spec.ImagePullSecrets
}

job := &batchv1.Job{
    Spec: batchv1.JobSpec{
        Template: corev1.PodTemplateSpec{
            Spec: corev1.PodSpec{
                ImagePullSecrets: imagePullSecrets,
                // ...
            },
        },
    },
}
```

## Best Practices

### Image Tagging

✅ **DO**:
- Use semantic versioning: `v1.2.3`, `v2.0.0-beta.1`
- Use SHA digests: `myimage@sha256:abc123...`
- Tag with build number: `myimage:build-1234`
- Include metadata: `myimage:v1.2.3-alpine-20240115`

❌ **DON'T**:
- Use `latest` in production
- Reuse tags (leads to cache issues)
- Use mutable tags (`dev`, `staging`) in production
- Omit tags (defaults to `latest`)

### Base Image Selection

**Recommended Base Images**:

1. **Distroless** (Minimal attack surface):
   ```dockerfile
   FROM gcr.io/distroless/static-debian11
   COPY --from=builder /app/binary /app
   ```

2. **Alpine** (Small, but has shell):
   ```dockerfile
   FROM alpine:3.19
   RUN apk add --no-cache bash curl
   ```

3. **Wolfi** (Modern, secure):
   ```dockerfile
   FROM cgr.dev/chainguard/wolfi-base:latest
   ```

**Avoid**:
- ❌ Ubuntu/Debian full images (too large, many CVEs)
- ❌ CentOS (deprecated)
- ❌ Random images from Docker Hub

### Scanning Frequency

| Image Type | Scan Frequency | Reason |
|------------|----------------|---------|
| Base images | Daily | New CVEs discovered constantly |
| Build images | On every push | Catch issues before deployment |
| Production | Weekly + on new CVE | Detect newly-disclosed vulnerabilities |
| Deprecated | Never/Manual | Don't waste resources |

### Vulnerability Thresholds

**Recommended**:
- **CRITICAL**: Block deployment, require immediate fix
- **HIGH**: Block deployment OR require security review + exception
- **MEDIUM**: Warn, require fix within 30 days
- **LOW**: Log, fix opportunistically

**Implementation**:
```yaml
# CI/CD
trivy image --severity CRITICAL --exit-code 1 myimage:latest
trivy image --severity HIGH,CRITICAL --exit-code 0 --format json myimage:latest > scan-results.json
```

## Troubleshooting

### Image Pull Errors

**Symptom**: Jobs fail with `ErrImagePull` or `ImagePullBackOff`

**Diagnosis**:
```bash
kubectl describe pod <pod-name> -n user-alice
# Look for "Failed to pull image" error
```

**Common Causes**:

1. **Invalid registry** → Check webhook allowed registries
2. **No credentials** → Create ImagePullSecret
3. **Invalid tag** → Verify image exists: `docker pull <image>`
4. **Network policy** → Verify egress to registry IPs
5. **Rate limiting** → Docker Hub has pull limits (use authenticated pulls)

### Signature Verification Failures

**Symptom**: Policy controller rejects unsigned images

**Diagnosis**:
```bash
kubectl get events -A | grep "failed signature verification"
```

**Fix**:
1. Sign images: `cosign sign --key cosign.key <image>`
2. Or disable policy for namespace: Add label to exempt
3. Or update ClusterImagePolicy to allow unsigned images temporarily

### Scan Results Not Available

**Symptom**: ECR/GCR scans don't show results

**Diagnosis**:
```bash
# AWS
aws ecr describe-image-scan-findings --repository-name scriptrunner/my-image --image-id imageTag=v1.0.0

# GCP
gcloud container images describe <image> --show-package-vulnerability
```

**Fix**:
- Wait a few minutes (scanning takes time)
- Verify scanning is enabled: `aws ecr get-repository-policy`
- Check IAM permissions for scan results

## Security Considerations

### Threat Model

| Threat | Mitigation |
|--------|------------|
| Malicious image from public registry | Registry whitelist (webhook) |
| Vulnerable base image | Vulnerability scanning (Trivy/Clair) |
| Tampered image | Image signing (cosign/notary) |
| Stale image with known CVE | Pull policy (Always) + scanning |
| Credential theft | ImagePullSecrets + RBAC |
| Supply chain attack | SBOM + provenance verification |

### Known Limitations

1. **Scanning isn't perfect**: Zero-day vulnerabilities won't be detected
2. **Signatures can be forged**: If signing keys are compromised
3. **Pull policy can be bypassed**: If image already cached on node
4. **Registry whitelist can be disabled**: Requires cluster-admin to modify webhook
5. **ImagePullSecrets can leak**: If users have access to namespace secrets

### Defense-in-Depth

Image security is ONE layer. Also implement:
- ✅ Pod Security Standards (restrict privileged containers)
- ✅ NetworkPolicy (limit egress from containers)
- ✅ RBAC (limit who can create ScriptRunners)
- ✅ ResourceQuota (limit resource consumption)
- ✅ Admission webhook (validate inputs)

## Compliance

### PCI-DSS Requirements

- **Req 6.2**: Scan images for vulnerabilities before deployment
- **Req 10.2**: Log all image pull events (enable audit logging)

### HIPAA Requirements

- **§ 164.308(a)(1)**: Risk analysis of container images
- **§ 164.312(a)(1)**: Unique image identification (SHA digests)

### SOC 2 Requirements

- **CC6.1**: Vulnerability management (scanning)
- **CC7.1**: Change management (signed images)

## References

- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [Cosign Quickstart](https://docs.sigstore.dev/cosign/overview/)
- [Kubernetes ImagePullPolicy](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy)
- [Sigstore Policy Controller](https://docs.sigstore.dev/policy-controller/overview/)
- [NIST Container Security Guide](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-190.pdf)
