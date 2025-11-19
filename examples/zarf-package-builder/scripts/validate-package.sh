#!/bin/bash
# validate-package.sh - Validate a Zarf package
#
# Environment variables:
#   INPUT_PACKAGE_PATH - Path to the Zarf package (.tar.zst file)

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Zarf Package Validator${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

if [ -z "$INPUT_PACKAGE_PATH" ]; then
    echo -e "${RED}Error: INPUT_PACKAGE_PATH is required${NC}"
    echo "Example: INPUT_PACKAGE_PATH=/path/to/zarf-package-example-amd64-v1.0.0.tar.zst"
    exit 1
fi

if [ ! -f "$INPUT_PACKAGE_PATH" ]; then
    echo -e "${RED}Error: Package file not found: $INPUT_PACKAGE_PATH${NC}"
    exit 1
fi

echo "Validating package: $INPUT_PACKAGE_PATH"
echo ""

# Verify package integrity
echo -e "${YELLOW}Verifying package integrity...${NC}"
if file "$INPUT_PACKAGE_PATH" | grep -q "Zstandard compressed"; then
    echo -e "${GREEN}✓${NC} Package is a valid Zstandard compressed file"
else
    echo -e "${RED}✗${NC} Package is not a valid Zstandard compressed file"
    exit 1
fi

# Inspect package contents
echo ""
echo -e "${YELLOW}Package contents:${NC}"
zarf package inspect "$INPUT_PACKAGE_PATH"

# Check for required components
echo ""
echo -e "${YELLOW}Checking package structure...${NC}"

# Extract package metadata (this is a simplified check)
METADATA=$(zarf package inspect "$INPUT_PACKAGE_PATH" --no-color)

if echo "$METADATA" | grep -q "Components:"; then
    echo -e "${GREEN}✓${NC} Package contains components"
else
    echo -e "${RED}✗${NC} Package missing components section"
    exit 1
fi

# Verify checksums
echo ""
echo -e "${YELLOW}Package checksum:${NC}"
sha256sum "$INPUT_PACKAGE_PATH"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Package validation successful!${NC}"
echo -e "${GREEN}========================================${NC}"
