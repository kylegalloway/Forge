#!/bin/bash
# build-package.sh - Build a Zarf package with multiple input/output options
#
# This script supports building Zarf packages from various sources and
# publishing to different destinations.
#
# Input Sources:
#   GIT    - Git repository containing zarf.yaml
#   OCI    - OCI artifact containing package definition
#   HELM   - Helm chart to package
#   LOCAL  - Local directory (for testing)
#
# Output Destinations:
#   LOCAL  - Local filesystem (default)
#   S3     - AWS S3 bucket
#   OCI    - OCI registry
#
# Environment Variables:
#   INPUT_SOURCE_TYPE     - Source type: git|oci|helm|local (default: git)
#   INPUT_SOURCE_LOCATION - Source location (repo URL, OCI ref, helm repo, or path)
#   INPUT_SOURCE_REF      - Git ref, chart version, or OCI tag (default: main/latest)
#
#   INPUT_OUTPUT_TYPE     - Output type: local|s3|oci (default: local)
#   INPUT_OUTPUT_PATH     - Local path or S3 bucket name (default: /output)
#   INPUT_OUTPUT_REGISTRY - OCI registry URL (for OCI output)
#
#   INPUT_PACKAGE_NAME    - Override package name (optional)
#   INPUT_BUILD_ARGS      - Additional zarf create arguments (optional)
#
# S3 Configuration (when OUTPUT_TYPE=s3):
#   INPUT_AWS_REGION      - AWS region (default: us-east-1)
#   INPUT_S3_PREFIX       - S3 key prefix (optional)
#   AWS_ACCESS_KEY_ID     - AWS credentials (from secret)
#   AWS_SECRET_ACCESS_KEY - AWS credentials (from secret)
#
# OCI Configuration (when OUTPUT_TYPE=oci):
#   INPUT_OCI_USERNAME    - Registry username (optional, for auth)
#   INPUT_OCI_PASSWORD    - Registry password (from secret)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() { echo -e "${BLUE}ℹ${NC} $1"; }
log_success() { echo -e "${GREEN}✓${NC} $1"; }
log_warn() { echo -e "${YELLOW}⚠${NC} $1"; }
log_error() { echo -e "${RED}✗${NC} $1"; }

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Zarf Package Builder (Multi-Source)${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Set defaults
SOURCE_TYPE="${INPUT_SOURCE_TYPE:-git}"
SOURCE_LOCATION="${INPUT_SOURCE_LOCATION}"
SOURCE_REF="${INPUT_SOURCE_REF:-main}"
OUTPUT_TYPE="${INPUT_OUTPUT_TYPE:-local}"
OUTPUT_PATH="${INPUT_OUTPUT_PATH:-/output}"
WORK_DIR="/workspace/build-$(date +%s)"
BUILD_ARGS="${INPUT_BUILD_ARGS}"

# Validate inputs
if [ -z "$SOURCE_LOCATION" ]; then
    log_error "INPUT_SOURCE_LOCATION is required"
    echo "Examples:"
    echo "  git:  INPUT_SOURCE_LOCATION=https://github.com/org/repo"
    echo "  oci:  INPUT_SOURCE_LOCATION=ghcr.io/org/package"
    echo "  helm: INPUT_SOURCE_LOCATION=https://charts.example.com"
    echo "  local: INPUT_SOURCE_LOCATION=/path/to/package"
    exit 1
fi

log_info "Configuration:"
echo "  Source Type: $SOURCE_TYPE"
echo "  Source Location: $SOURCE_LOCATION"
echo "  Source Ref: $SOURCE_REF"
echo "  Output Type: $OUTPUT_TYPE"
echo "  Output Path: $OUTPUT_PATH"
if [ -n "$INPUT_OUTPUT_REGISTRY" ]; then
    echo "  OCI Registry: $INPUT_OUTPUT_REGISTRY"
fi
echo ""

# Create work directory
mkdir -p "$WORK_DIR"
mkdir -p "$OUTPUT_PATH"
cd "$WORK_DIR"

# ============================================================================
# PHASE 1: Fetch Source
# ============================================================================

log_info "Phase 1: Fetching source..."

case "$SOURCE_TYPE" in
    git)
        log_info "Cloning Git repository: $SOURCE_LOCATION (ref: $SOURCE_REF)"
        git clone --depth 1 --branch "$SOURCE_REF" "$SOURCE_LOCATION" source
        cd source
        ;;

    oci)
        log_info "Pulling OCI artifact: $SOURCE_LOCATION:$SOURCE_REF"
        # Use zarf to pull OCI package
        zarf package pull "oci://$SOURCE_LOCATION:$SOURCE_REF" --output-directory ./source
        cd source
        ;;

    helm)
        log_info "Fetching Helm chart: $SOURCE_LOCATION"
        mkdir -p source
        cd source

        # Add helm repo if it's a repo URL
        if [[ "$SOURCE_LOCATION" =~ ^https?:// ]]; then
            CHART_NAME="${INPUT_PACKAGE_NAME:-chart}"
            helm repo add temp-repo "$SOURCE_LOCATION"
            helm pull temp-repo/"$CHART_NAME" --version "$SOURCE_REF" --untar
            cd "$CHART_NAME"
        else
            # Assume it's a chart reference
            helm pull "$SOURCE_LOCATION" --version "$SOURCE_REF" --untar
            CHART_DIR=$(find . -maxdepth 1 -type d ! -name . | head -1)
            cd "$CHART_DIR"
        fi

        # Create zarf.yaml for Helm chart if it doesn't exist
        if [ ! -f "zarf.yaml" ]; then
            log_warn "No zarf.yaml found, creating one for Helm chart"
            CHART_NAME=$(basename "$PWD")
            cat > zarf.yaml <<EOF
kind: ZarfPackageConfig
metadata:
  name: ${INPUT_PACKAGE_NAME:-$CHART_NAME}
  description: "Zarf package for Helm chart: $CHART_NAME"
  version: "$SOURCE_REF"

components:
  - name: ${CHART_NAME}
    required: true
    charts:
      - name: ${CHART_NAME}
        localPath: .
        namespace: default
EOF
        fi
        ;;

    local)
        log_info "Using local directory: $SOURCE_LOCATION"
        if [ ! -d "$SOURCE_LOCATION" ]; then
            log_error "Local directory not found: $SOURCE_LOCATION"
            exit 1
        fi
        cp -r "$SOURCE_LOCATION" source
        cd source
        ;;

    *)
        log_error "Unknown source type: $SOURCE_TYPE"
        echo "Valid types: git, oci, helm, local"
        exit 1
        ;;
esac

# Verify zarf.yaml exists
if [ ! -f "zarf.yaml" ]; then
    log_error "zarf.yaml not found in source"
    exit 1
fi

log_success "Source fetched successfully"

# ============================================================================
# PHASE 2: Build Package
# ============================================================================

log_info "Phase 2: Building Zarf package..."
echo ""

# Display package metadata
log_info "Package Information:"
zarf package inspect . --no-color || true
echo ""

# Build the package
log_info "Running: zarf package create . --confirm $BUILD_ARGS"
zarf package create . --confirm --output-directory "$OUTPUT_PATH" $BUILD_ARGS

# Find the built package
PACKAGE_FILE=$(find "$OUTPUT_PATH" -name "zarf-package-*.tar.zst" -type f | head -1)

if [ -z "$PACKAGE_FILE" ]; then
    log_error "No package file found after build"
    exit 1
fi

PACKAGE_NAME=$(basename "$PACKAGE_FILE")
log_success "Package built: $PACKAGE_NAME"

# Calculate checksum
cd "$OUTPUT_PATH"
CHECKSUM=$(sha256sum "$PACKAGE_NAME" | awk '{print $1}')
echo "$CHECKSUM  $PACKAGE_NAME" > "${PACKAGE_NAME}.sha256"
log_success "Checksum: $CHECKSUM"

# ============================================================================
# PHASE 3: Publish/Upload
# ============================================================================

log_info "Phase 3: Publishing package..."

case "$OUTPUT_TYPE" in
    local)
        log_info "Package saved locally: $OUTPUT_PATH/$PACKAGE_NAME"
        ls -lh "$OUTPUT_PATH"/$PACKAGE_NAME*
        log_success "Local output complete"
        ;;

    s3)
        if ! command -v aws &> /dev/null; then
            log_error "AWS CLI not installed (required for S3 output)"
            exit 1
        fi

        S3_BUCKET="$OUTPUT_PATH"
        S3_REGION="${INPUT_AWS_REGION:-us-east-1}"
        S3_PREFIX="${INPUT_S3_PREFIX}"

        if [ -n "$S3_PREFIX" ]; then
            S3_KEY="${S3_PREFIX}/${PACKAGE_NAME}"
        else
            S3_KEY="$PACKAGE_NAME"
        fi

        log_info "Uploading to S3: s3://${S3_BUCKET}/${S3_KEY}"
        aws s3 cp "$PACKAGE_NAME" "s3://${S3_BUCKET}/${S3_KEY}" --region "$S3_REGION"
        aws s3 cp "${PACKAGE_NAME}.sha256" "s3://${S3_BUCKET}/${S3_KEY}.sha256" --region "$S3_REGION"

        # Generate presigned URL (valid for 1 hour)
        PRESIGNED_URL=$(aws s3 presign "s3://${S3_BUCKET}/${S3_KEY}" --region "$S3_REGION" --expires-in 3600)

        log_success "Uploaded to S3"
        echo "S3 URI: s3://${S3_BUCKET}/${S3_KEY}"
        echo "Presigned URL (1h): $PRESIGNED_URL"
        ;;

    oci)
        if [ -z "$INPUT_OUTPUT_REGISTRY" ]; then
            log_error "INPUT_OUTPUT_REGISTRY required for OCI output"
            echo "Example: INPUT_OUTPUT_REGISTRY=ghcr.io/org/repo"
            exit 1
        fi

        # Login to registry if credentials provided
        if [ -n "$INPUT_OCI_USERNAME" ] && [ -n "$INPUT_OCI_PASSWORD" ]; then
            log_info "Logging into OCI registry..."
            echo "$INPUT_OCI_PASSWORD" | zarf tools registry login "$INPUT_OUTPUT_REGISTRY" \
                -u "$INPUT_OCI_USERNAME" --password-stdin
        fi

        # Extract version from package name
        VERSION=$(echo "$PACKAGE_NAME" | sed -n 's/.*-\(v[0-9]\+\.[0-9]\+\.[0-9]\+\)\.tar\.zst/\1/p')
        if [ -z "$VERSION" ]; then
            VERSION="latest"
        fi

        OCI_REF="${INPUT_OUTPUT_REGISTRY}:${VERSION}"

        log_info "Publishing to OCI: $OCI_REF"
        zarf package publish "$PACKAGE_NAME" "oci://${OCI_REF}"

        log_success "Published to OCI registry"
        echo "OCI Reference: oci://${OCI_REF}"
        ;;

    *)
        log_error "Unknown output type: $OUTPUT_TYPE"
        echo "Valid types: local, s3, oci"
        exit 1
        ;;
esac

# ============================================================================
# Summary
# ============================================================================

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Build Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Package: $PACKAGE_NAME"
echo "SHA256: $CHECKSUM"
echo "Source: $SOURCE_TYPE ($SOURCE_LOCATION)"
echo "Output: $OUTPUT_TYPE"
echo ""

# Cleanup
if [ "$OUTPUT_TYPE" != "local" ]; then
    log_info "Cleaning up local artifacts..."
    rm -f "$OUTPUT_PATH/$PACKAGE_NAME" "$OUTPUT_PATH/${PACKAGE_NAME}.sha256"
fi

log_success "All operations completed successfully"
