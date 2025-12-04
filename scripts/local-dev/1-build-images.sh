#!/bin/bash
set -e

echo "🔨 Building Forge images..."

# Set defaults
VERSION="${VERSION:-dev-$(date +%s)}"
ZARF_VERSION="${ZARF_VERSION:-v0.66.0}"

echo "📦 Forge version: ${VERSION}"
echo "📦 Zarf CLI version: ${ZARF_VERSION}"
echo "🐋 Container runtime: podman"

# Build controller image directly with podman
echo "Building Forge controller..."
podman build -t forge-controller:${VERSION} .

# Build Zarf CLI image (Zarf doesn't publish container images)
echo "Building Zarf CLI image..."
podman build -t localhost/zarf:${ZARF_VERSION} images/zarf-cli/

echo ""
echo "✅ Images built successfully:"
podman images | grep -E 'forge-controller|zarf'

# Save version to file for other scripts
echo "${VERSION}" > .forge-version
echo ""
echo "📝 Version saved to .forge-version"
echo "📝 Next: Load images into Kind cluster with script ./scripts/local-dev/2-setup-kind.sh"
