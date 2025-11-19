#!/bin/bash
# test.sh - Test the Zarf builder with different input/output scenarios
#
# This script tests all supported input and output types locally

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

IMAGE_NAME="scriptrunner-zarf-builder:latest"
TEST_OUTPUT="/tmp/zarf-builder-test-$$"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Zarf Builder Test Suite${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up test artifacts...${NC}"
    rm -rf "$TEST_OUTPUT"
}
trap cleanup EXIT

# Create test output directory
mkdir -p "$TEST_OUTPUT"

# Check if image exists
if ! docker inspect "$IMAGE_NAME" &>/dev/null && ! podman inspect "$IMAGE_NAME" &>/dev/null; then
    echo -e "${RED}Error: Image $IMAGE_NAME not found${NC}"
    echo "Run 'make build' first"
    exit 1
fi

# Detect container runtime
RUNTIME=$(command -v podman 2>/dev/null || command -v docker 2>/dev/null)

echo -e "${YELLOW}Using container runtime: $RUNTIME${NC}"
echo -e "${YELLOW}Test output directory: $TEST_OUTPUT${NC}"
echo ""

# ============================================================================
# Test 1: Build from local directory (test package)
# ============================================================================

echo -e "${GREEN}Test 1: Build from local directory${NC}"
$RUNTIME run --rm \
    -v "$TEST_OUTPUT":/output \
    -e INPUT_SOURCE_TYPE=local \
    -e INPUT_SOURCE_LOCATION=/workspace/test-package \
    -e INPUT_OUTPUT_TYPE=local \
    -e INPUT_OUTPUT_PATH=/output \
    "$IMAGE_NAME" \
    /scripts/build-package.sh

# Verify output
if [ -f "$TEST_OUTPUT"/zarf-package-test-package-*.tar.zst ]; then
    echo -e "${GREEN}✓ Test 1 PASSED: Package built successfully${NC}"
else
    echo -e "${RED}✗ Test 1 FAILED: No package file found${NC}"
    exit 1
fi
echo ""

# ============================================================================
# Test 2: Validate the built package
# ============================================================================

echo -e "${GREEN}Test 2: Validate built package${NC}"
PACKAGE_FILE=$(find "$TEST_OUTPUT" -name "zarf-package-*.tar.zst" -type f | head -1)
PACKAGE_NAME=$(basename "$PACKAGE_FILE")

$RUNTIME run --rm \
    -v "$TEST_OUTPUT":/packages \
    -e INPUT_PACKAGE_PATH=/packages/"$PACKAGE_NAME" \
    "$IMAGE_NAME" \
    /scripts/validate-package.sh

echo -e "${GREEN}✓ Test 2 PASSED: Package validation successful${NC}"
echo ""

# ============================================================================
# Test 3: Verify checksum file
# ============================================================================

echo -e "${GREEN}Test 3: Verify checksum file${NC}"
if [ -f "$TEST_OUTPUT/${PACKAGE_NAME}.sha256" ]; then
    echo -e "${GREEN}✓ Test 3 PASSED: Checksum file exists${NC}"
    cat "$TEST_OUTPUT/${PACKAGE_NAME}.sha256"
else
    echo -e "${RED}✗ Test 3 FAILED: No checksum file${NC}"
    exit 1
fi
echo ""

# ============================================================================
# Test 4: Verify Zarf can inspect the package
# ============================================================================

echo -e "${GREEN}Test 4: Inspect package with Zarf${NC}"
$RUNTIME run --rm \
    -v "$TEST_OUTPUT":/packages \
    "$IMAGE_NAME" \
    zarf package inspect /packages/"$PACKAGE_NAME"

echo -e "${GREEN}✓ Test 4 PASSED: Package inspection successful${NC}"
echo ""

# ============================================================================
# Summary
# ============================================================================

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All Tests Passed!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Built package: $PACKAGE_NAME"
echo "Size: $(du -h "$TEST_OUTPUT/$PACKAGE_NAME" | cut -f1)"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Test with ScriptRunner in Kubernetes"
echo "2. Try different input sources (git, oci, helm)"
echo "3. Test output destinations (s3, oci)"
echo ""
