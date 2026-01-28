#!/bin/bash
# Script to update the underlying tool versions used by Forge images
# Usage: ./scripts/update-tool-versions.sh
#
# This script fetches the latest upstream tool releases and updates only
# the Dockerfile ARGs (ZARF_VERSION and UDS_VERSION) in the Forge image
# Dockerfiles: images/zarfpackagejob/Dockerfile and images/udsbundlejob/Dockerfile

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Fetch latest release version from GitHub
get_latest_github_release() {
    local repo="$1"
    local version

    version=$(curl -sL "https://api.github.com/repos/${repo}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

    if [[ -z "$version" ]]; then
        log_error "Failed to fetch latest release for ${repo}"
        return 1
    fi

    echo "$version"
}

# Update Zarf tool version ARG in images/zarfpackagejob/Dockerfile
update_zarf_version() {
    log_info "Fetching latest Zarf release..."
    local new_version
    new_version=$(get_latest_github_release "zarf-dev/zarf")
    log_info "Latest Zarf release: $new_version"

    local dockerfile="$PROJECT_ROOT/images/zarfpackagejob/Dockerfile"
    if [[ -f "$dockerfile" ]]; then
        sed -i.bak "s/ARG ZARF_VERSION=v[0-9]*\.[0-9]*\.[0-9]*/ARG ZARF_VERSION=${new_version}/" "$dockerfile"
        rm -f "${dockerfile}.bak"
        log_info "Updated: $dockerfile"
    else
        log_warn "$dockerfile not found; skipping"
    fi

    echo "$new_version"
}

# Update UDS tool version ARG in images/udsbundlejob/Dockerfile
update_uds_version() {
    log_info "Fetching latest UDS release..."
    local new_version
    new_version=$(get_latest_github_release "defenseunicorns/uds-cli")
    log_info "Latest UDS release: $new_version"

    local dockerfile="$PROJECT_ROOT/images/udsbundlejob/Dockerfile"
    if [[ -f "$dockerfile" ]]; then
        sed -i.bak "s/ARG UDS_VERSION=v[0-9]*\.[0-9]*\.[0-9]*/ARG UDS_VERSION=${new_version}/" "$dockerfile"
        rm -f "${dockerfile}.bak"
        log_info "Updated: $dockerfile"
    else
        log_warn "$dockerfile not found; skipping"
    fi

    echo "$new_version"
}

# Main execution
main() {
    log_info "Starting CLI version updates..."
    echo ""

    local zarf_version uds_version

    zarf_version=$(update_zarf_version)
    echo ""

    uds_version=$(update_uds_version)
    echo ""

    log_info "Version update complete!"
    echo ""
    echo "Summary:"
    echo "  Zarf CLI: ${zarf_version}"
    echo "  UDS CLI:  ${uds_version}"
    echo ""
    echo "Next steps:"
    echo "  1. Review the changes: git diff"
    echo "  2. Build and test: go build ./..."
    echo "  3. Optionally build local images for testing:"
    echo "     docker build -t zarfpackagejob:test images/zarfpackagejob/"
    echo "     docker build -t udsbundlejob:test images/udsbundlejob/"
}

main "$@"
