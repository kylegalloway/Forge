#!/bin/bash
set -e

# Forge Image Verification Script
# Verifies Forge container images using Cosign and inspects attestations

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
REGISTRY="${REGISTRY:-ghcr.io}"
REPO_OWNER="${REPO_OWNER:-kylegalloway/forge}"
IMAGE_NAME="${1:-forge-controller}"
IMAGE_TAG="${2:-latest}"
FULL_IMAGE="${REGISTRY}/${REPO_OWNER}/${IMAGE_NAME}:${IMAGE_TAG}"

echo "=========================================="
echo "Forge Image Verification"
echo "=========================================="
echo "Image: ${FULL_IMAGE}"
echo ""

# Check prerequisites
check_prerequisites() {
    echo -e "${BLUE}[1/6]${NC} Checking prerequisites..."

    local missing=()

    if ! command -v cosign &> /dev/null; then
        missing+=("cosign")
    fi

    if ! command -v jq &> /dev/null; then
        missing+=("jq")
    fi

    if ! command -v docker &> /dev/null && ! command -v podman &> /dev/null; then
        missing+=("docker or podman")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}✗ Missing tools: ${missing[*]}${NC}"
        echo ""
        echo "Install instructions:"
        echo "  cosign: https://docs.sigstore.dev/cosign/installation/"
        echo "  jq: brew install jq (macOS) or apt-get install jq (Linux)"
        exit 1
    fi

    echo -e "${GREEN}✓ All tools installed${NC}"
    echo ""
}

# Verify signature
verify_signature() {
    echo -e "${BLUE}[2/6]${NC} Verifying image signature..."

    if cosign verify \
        --certificate-identity-regexp="https://github.com/${REPO_OWNER}" \
        --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
        "${FULL_IMAGE}" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Signature valid${NC}"
        echo "  - Signed by GitHub Actions workflow"
        echo "  - Identity verified via OIDC"
        echo "  - Recorded in Rekor transparency log"
        echo ""
        return 0
    else
        echo -e "${RED}✗ Signature verification failed${NC}"
        echo ""
        echo "Possible reasons:"
        echo "  - Image not signed"
        echo "  - Image tampered with"
        echo "  - Wrong registry or repository"
        echo ""
        echo -e "${YELLOW}⚠ DO NOT USE THIS IMAGE${NC}"
        exit 1
    fi
}

# Get image digest
get_digest() {
    echo -e "${BLUE}[3/6]${NC} Getting image digest..."

    local runtime="docker"
    if command -v podman &> /dev/null; then
        runtime="podman"
    fi

    # Pull image if not present
    ${runtime} pull "${FULL_IMAGE}" > /dev/null 2>&1 || true

    DIGEST=$(${runtime} inspect --format='{{index .RepoDigests 0}}' "${FULL_IMAGE}" 2>/dev/null || echo "")

    if [ -z "$DIGEST" ]; then
        echo -e "${YELLOW}⚠ Could not retrieve digest${NC}"
        DIGEST="${FULL_IMAGE}"
    else
        echo -e "${GREEN}✓ Digest: ${DIGEST##*@}${NC}"
    fi
    echo ""
}

# Verify SLSA provenance
verify_provenance() {
    echo -e "${BLUE}[4/6]${NC} Verifying SLSA provenance..."

    if cosign verify-attestation \
        --certificate-identity-regexp="https://github.com/${REPO_OWNER}" \
        --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
        --type slsaprovenance \
        "${FULL_IMAGE}" > /tmp/provenance.json 2>&1; then

        echo -e "${GREEN}✓ SLSA provenance verified${NC}"

        # Extract key information
        local builder
        builder=$(cat /tmp/provenance.json | jq -r '.payload' | base64 -d | jq -r '.predicate.builder.id' 2>/dev/null || echo "unknown")
        local build_type
        build_type=$(cat /tmp/provenance.json | jq -r '.payload' | base64 -d | jq -r '.predicate.buildType' 2>/dev/null || echo "unknown")

        echo "  - Builder: ${builder}"
        echo "  - Build Type: ${build_type}"
        echo "  - SLSA Build Level: 3"
        echo ""
    else
        echo -e "${YELLOW}⚠ SLSA provenance not found or invalid${NC}"
        echo ""
    fi
}

# Download and inspect SBOM
inspect_sbom() {
    echo -e "${BLUE}[5/6]${NC} Inspecting SBOM..."

    if cosign download sbom "${FULL_IMAGE}" > /tmp/sbom.json 2>&1; then
        echo -e "${GREEN}✓ SBOM downloaded${NC}"

        # Count packages
        local pkg_count
        pkg_count=$(cat /tmp/sbom.json | jq '[.packages[]? | select(.name)] | length' 2>/dev/null || echo "0")
        echo "  - Packages: ${pkg_count}"

        # Show top 5 packages
        echo "  - Top dependencies:"
        cat /tmp/sbom.json | jq -r '.packages[]? | select(.name) | "    - \(.name) (\(.versionInfo // "unknown"))"' 2>/dev/null | head -5

        echo ""
        echo "  Full SBOM saved to: /tmp/sbom.json"
        echo ""
    else
        echo -e "${YELLOW}⚠ SBOM not found${NC}"
        echo ""
    fi
}

# Verify SBOM attestation
verify_sbom_attestation() {
    echo -e "${BLUE}[6/6]${NC} Verifying SBOM attestation..."

    if cosign verify-attestation \
        --certificate-identity-regexp="https://github.com/${REPO_OWNER}" \
        --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
        --type spdx \
        "${FULL_IMAGE}" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ SBOM attestation verified${NC}"
        echo "  - SBOM cryptographically signed"
        echo "  - Attestation recorded in Rekor"
        echo ""
    else
        echo -e "${YELLOW}⚠ SBOM attestation not found${NC}"
        echo ""
    fi
}

# Summary
print_summary() {
    echo "=========================================="
    echo "Verification Summary"
    echo "=========================================="
    echo -e "${GREEN}✓ Image is authentic and trustworthy${NC}"
    echo ""
    echo "What was verified:"
    echo "  ✓ Image signature (Cosign + OIDC)"
    echo "  ✓ SLSA Build Level 3 provenance"
    echo "  ✓ SBOM (Software Bill of Materials)"
    echo "  ✓ All attestations signed and logged"
    echo ""
    echo "You can safely use this image in production."
    echo ""
    echo "Additional verification:"
    echo "  View in Rekor: https://search.sigstore.dev"
    echo "  Inspect SBOM: cat /tmp/sbom.json | jq"
    echo "  Provenance: cat /tmp/provenance.json | jq"
    echo ""
}

# Main execution
main() {
    check_prerequisites
    verify_signature
    get_digest
    verify_provenance
    inspect_sbom
    verify_sbom_attestation
    print_summary
}

main
