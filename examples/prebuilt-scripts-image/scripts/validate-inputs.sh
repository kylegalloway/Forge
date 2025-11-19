#!/bin/bash
set -e

# Pre-built script for validating inputs
# This demonstrates validation logic that can be reused across multiple ScriptRunners

echo "=================================="
echo "Input Validation Script"
echo "=================================="
echo ""

# Required inputs (from environment variables)
REQUIRED_VARS=("INPUT_environment" "INPUT_version")

# Check for required inputs
MISSING=()
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        MISSING+=("$var")
    fi
done

if [ ${#MISSING[@]} -ne 0 ]; then
    echo "ERROR: Missing required inputs:"
    for var in "${MISSING[@]}"; do
        echo "  - ${var#INPUT_}"
    done
    exit 1
fi

echo "✓ All required inputs present"
echo ""

# Validate environment
ENVIRONMENT="${INPUT_environment}"
VALID_ENVS=("dev" "staging" "production")

if [[ ! " ${VALID_ENVS[@]} " =~ " ${ENVIRONMENT} " ]]; then
    echo "ERROR: Invalid environment: $ENVIRONMENT"
    echo "Valid options: ${VALID_ENVS[*]}"
    exit 1
fi

echo "✓ Environment is valid: $ENVIRONMENT"
echo ""

# Validate version format (semantic versioning)
VERSION="${INPUT_version}"
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "ERROR: Invalid version format: $VERSION"
    echo "Expected format: X.Y.Z (e.g., 1.2.3)"
    exit 1
fi

echo "✓ Version format is valid: $VERSION"
echo ""

# Show all validated inputs
echo "Validated Inputs:"
env | grep "^INPUT_" | sort | sed 's/INPUT_/  /'
echo ""

echo "=================================="
echo "✓ All Validations Passed!"
echo "=================================="
