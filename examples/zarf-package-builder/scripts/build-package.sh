#!/bin/bash
# build-package.sh - Build a Zarf package from a zarf.yaml specification
#
# This script is designed to run in a ScriptRunner Job to build Zarf packages
# Environment variables:
#   INPUT_SOURCE_REPO - Git repository URL containing zarf.yaml
#   INPUT_SOURCE_REF  - Git ref (branch/tag) to build from (default: main)
#   INPUT_PACKAGE_NAME - Name of the output package (optional)
#   INPUT_OUTPUT_PATH - Where to store the built package (default: /output)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Zarf Package Builder${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Check required environment variables
if [ -z "$INPUT_SOURCE_REPO" ]; then
    echo -e "${RED}Error: INPUT_SOURCE_REPO is required${NC}"
    echo "Example: INPUT_SOURCE_REPO=https://github.com/org/repo"
    exit 1
fi

# Set defaults
SOURCE_REF="${INPUT_SOURCE_REF:-main}"
OUTPUT_PATH="${INPUT_OUTPUT_PATH:-/output}"
WORK_DIR="/workspace/build"

echo "Configuration:"
echo "  Source Repository: $INPUT_SOURCE_REPO"
echo "  Source Ref: $SOURCE_REF"
echo "  Output Path: $OUTPUT_PATH"
echo ""

# Create work directory
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# Clone repository
echo -e "${YELLOW}Cloning repository...${NC}"
git clone --depth 1 --branch "$SOURCE_REF" "$INPUT_SOURCE_REPO" repo
cd repo

# Verify zarf.yaml exists
if [ ! -f "zarf.yaml" ]; then
    echo -e "${RED}Error: zarf.yaml not found in repository${NC}"
    exit 1
fi

# Display package metadata
echo ""
echo -e "${YELLOW}Package Information:${NC}"
zarf package inspect . --no-color

# Build the package
echo ""
echo -e "${YELLOW}Building Zarf package...${NC}"
zarf package create . --confirm --output-directory "$OUTPUT_PATH"

# List built packages
echo ""
echo -e "${GREEN}Build complete!${NC}"
echo -e "${YELLOW}Built packages:${NC}"
ls -lh "$OUTPUT_PATH"/*.tar.zst 2>/dev/null || ls -lh "$OUTPUT_PATH"/zarf-package-*.tar.zst 2>/dev/null || true

# Calculate checksums
echo ""
echo -e "${YELLOW}Checksums:${NC}"
cd "$OUTPUT_PATH"
for pkg in *.tar.zst 2>/dev/null || zarf-package-*.tar.zst 2>/dev/null; do
    if [ -f "$pkg" ]; then
        sha256sum "$pkg"
    fi
done

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Package build successful!${NC}"
echo -e "${GREEN}========================================${NC}"
