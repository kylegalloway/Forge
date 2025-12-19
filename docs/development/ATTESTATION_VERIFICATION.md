# Forge Attestation Verification Guide

This guide explains how to verify the authenticity and provenance of Forge container images and binaries.

## Overview

Forge images are signed and attested using:

- **Cosign** - Keyless signing with GitHub OIDC
- **SLSA Provenance** - Build provenance metadata
- **SBOM** - Software Bill of Materials (SPDX + CycloneDX)
- **Rekor** - Transparency log for signatures

## Prerequisites

Install verification tools:

```bash
# Install Cosign
brew install cosign  # macOS
# or
curl -O -L "https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64"
sudo mv cosign-linux-amd64 /usr/local/bin/cosign
chmod +x /usr/local/bin/cosign

# Install Syft (for SBOM inspection)
brew install syft  # macOS
# or
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
```

## Verifying Forge Images

### 1. Verify Image Signature

Verify that the image was built by the official Forge GitHub Actions workflow:

```bash
# Verify controller image
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kylegalloway/forge/forge-controller:latest

# Verify webhook image
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kylegalloway/forge/forge-webhook:latest
```

**What this proves:**

- ✅ Image was built by GitHub Actions
- ✅ Built from the official Forge repository
- ✅ Signature recorded in Rekor transparency log
- ✅ Cannot be forged or tampered with

### 2. Verify SLSA Provenance

Download and inspect the SLSA provenance attestation:

```bash
# Download provenance
cosign verify-attestation \
  --certificate-identity-regexp="https://github.com/kylegalloway/forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  --type slsaprovenance \
  ghcr.io/kylegalloway/forge/forge-controller:latest | jq
```

**Provenance contains:**

- Build environment details
- Source repository and commit
- Build inputs and parameters
- Builder identity (GitHub Actions)
- Timestamps

**SLSA Level:** Build Level 3

- ✅ Isolated build environment
- ✅ Signed provenance
- ✅ Non-falsifiable (OIDC identity)
- ✅ Hermetic builds

### 3. Verify and Inspect SBOM

Download the Software Bill of Materials:

```bash
# Download SBOM
cosign download sbom \
  ghcr.io/kylegalloway/forge/forge-controller:latest > sbom.spdx.json

# Inspect with Syft
syft sbom.spdx.json

# Or use jq
cat sbom.spdx.json | jq '.packages[] | select(.name) | {name, version, type}'
```

**SBOM includes:**

- All Go dependencies with versions
- Operating system packages
- License information
- Vulnerability data (when scanned)

### 4. Verify Attestation Bundle

Verify the SBOM attestation:

```bash
cosign verify-attestation \
  --certificate-identity-regexp="https://github.com/kylegalloway/forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  --type spdx \
  ghcr.io/kylegalloway/forge/forge-controller:latest | jq
```

## Verifying Specific Versions

### By Tag

```bash
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/kylegalloway/forge/forge-controller:v0.1.1
```

### By Digest (Most Secure)

```bash
# Get digest
IMAGE_DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' \
  ghcr.io/kylegalloway/forge/forge-controller:latest)

# Verify by digest
cosign verify \
  --certificate-identity-regexp="https://github.com/kylegalloway/forge" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  "${IMAGE_DIGEST}"
```

**Always use digests in production!**

## Admission Policy Enforcement

### Kubernetes Policy Controller

Enforce signature verification at deploy time using Sigstore Policy Controller:

```yaml
apiVersion: policy.sigstore.dev/v1beta1
kind: ClusterImagePolicy
metadata:
  name: forge-images
spec:
  images:
  - glob: "ghcr.io/kylegalloway/forge/*"

  authorities:
  - keyless:

      url: https://fulcio.sigstore.dev
      identities:
      - issuer: https://token.actions.githubusercontent.com

        subjectRegExp: "https://github.com/kylegalloway/forge/.*"
```

This ensures only signed Forge images can be deployed.

### Kyverno Policy

Alternative using Kyverno:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: verify-forge-images
spec:
  validationFailureAction: enforce
  background: false
  webhookTimeoutSeconds: 30
  rules:
  - name: verify-signature

    match:
      any:
      - resources:

          kinds:
          - Pod

    verifyImages:
    - imageReferences:
      - "ghcr.io/kylegalloway/forge/*"

      attestors:
      - entries:
        - keyless:

            subject: "https://github.com/kylegalloway/forge/.github/workflows/attest.yaml@refs/heads/main"
            issuer: "https://token.actions.githubusercontent.com"
            rekor:
              url: https://rekor.sigstore.dev
```

## Transparency Log Verification

All signatures are recorded in Rekor (Sigstore's transparency log):

```bash
# Search Rekor for Forge images
rekor-cli search \
  --artifact ghcr.io/kylegalloway/forge/forge-controller:latest

# Get specific entry
rekor-cli get --uuid <UUID>
```

**What Rekor provides:**

- Immutable audit trail
- Tamper-evident log
- Public verification
- Timestamp authority

## Vulnerability Scanning

Images are scanned with Trivy during build. View results in GitHub Security tab.

### Manual Scan

```bash
# Scan image
trivy image ghcr.io/kylegalloway/forge/forge-controller:latest

# Scan SBOM
trivy sbom sbom.spdx.json
```

## CI/CD Integration

### GitLab CI

```yaml
verify-forge-images:
  image: gcr.io/projectsigstore/cosign:latest
  script:
    - cosign verify

        --certificate-identity-regexp="https://github.com/kylegalloway/forge"
        --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
        ghcr.io/kylegalloway/forge/forge-controller:${CI_COMMIT_TAG}
```

### GitHub Actions

```yaml
- name: Verify Forge image

  run: |
    cosign verify \
      --certificate-identity-regexp="https://github.com/kylegalloway/forge" \
      --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
      ghcr.io/kylegalloway/forge/forge-controller:${{ github.ref_name }}
```

## Troubleshooting

### "Error: no matching signatures"

**Cause:** Image not signed or pulled from wrong registry

**Solution:**

1. Ensure you're using the official registry: `ghcr.io/kylegalloway/forge`
2. Verify the image exists and is signed
3. Check you have the latest Cosign version

### "Error: certificate identity doesn't match"

**Cause:** Image signed with different identity

**Solution:**

1. Verify the `--certificate-identity-regexp` matches the repository
2. For forks, adjust the identity pattern

### "Error: failed to verify signature"

**Cause:** Image tampered with or signature invalid

**Solution:**

1. **DO NOT USE THIS IMAGE**
2. Report to security@forge.dev (if this existed)
3. Pull fresh image from registry
4. Verify digest matches official releases

## Best Practices

### Development

- ✅ Verify images before deploying to dev
- ✅ Use specific tags or digests
- ✅ Inspect SBOMs for known vulnerabilities
- ✅ Test with admission policies enabled

### Production

- ✅ **ALWAYS** verify signatures before deployment
- ✅ **ALWAYS** use digests, never `:latest`
- ✅ Enforce admission policies (Policy Controller or Kyverno)
- ✅ Monitor Rekor for unexpected entries
- ✅ Scan images regularly
- ✅ Pin dependencies in Dockerfiles

### Security Team

- ✅ Audit Rekor logs periodically
- ✅ Review SBOMs for license compliance
- ✅ Track CVEs in dependencies
- ✅ Validate build provenance
- ✅ Verify hermetic builds

## Security Contact

To report security issues with Forge attestations or signatures:

1. **DO NOT** open a public GitHub issue
2. Email: security@kylegalloway.dev (if this existed)
3. Include: image digest, verification error, and steps to reproduce

## References

- [Cosign Documentation](https://docs.sigstore.dev/cosign/overview/)
- [SLSA Framework](https://slsa.dev/)
- [Sigstore Project](https://www.sigstore.dev/)
- [SPDX Specification](https://spdx.dev/)
- [CycloneDX Standard](https://cyclonedx.org/)
- [Rekor Transparency Log](https://docs.sigstore.dev/rekor/overview/)

## Attestation Workflow Details

The attestation workflow (`.github/workflows/attest.yaml`) performs:

1. **Reproducible Build**
   - Disabled CGO
   - Trimmed paths
   - Consistent ldflags

2. **SBOM Generation**
   - SPDX format (industry standard)
   - CycloneDX format (for tools)
   - Both attached to image

3. **Image Build**
   - Multi-arch (amd64, arm64)
   - Provenance auto-generated
   - SBOM auto-attached

4. **Signing**
   - Keyless (GitHub OIDC)
   - Recorded in Rekor
   - Fulcio certificate

5. **Attestation**
   - SBOM attestation
   - SLSA provenance
   - Vulnerability scan results

6. **Verification**
   - Trivy scan
   - SARIF upload to GitHub Security
   - Summary generation

All steps are auditable and verifiable by anyone!

---

*Last Updated: 2025-11-25*
*Forge Version: v0.1.1*
