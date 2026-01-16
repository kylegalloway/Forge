#!/bin/bash
# Script to update Zarf CLI and UDS CLI versions from GitHub releases
# Usage: ./scripts/update-cli-versions.sh
#
# This script fetches the latest release versions from GitHub and updates:
# - Dockerfiles (images/zarf-cli/, images/uds-cli/)
# - Go constants (pkg/constants/config.go)
# - Helm values (chart/forge/values.yaml)
# - Zarf package config (zarf.yaml)

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

# Update Zarf CLI version
update_zarf_version() {
    log_info "Fetching latest Zarf CLI version..."
    local new_version
    new_version=$(get_latest_github_release "zarf-dev/zarf")
    log_info "Latest Zarf version: $new_version"

    # Update Dockerfile ARG
    local dockerfile="$PROJECT_ROOT/images/zarf-cli/Dockerfile"
    if [[ -f "$dockerfile" ]]; then
        sed -i.bak "s/ARG ZARF_VERSION=v[0-9]*\.[0-9]*\.[0-9]*/ARG ZARF_VERSION=${new_version}/" "$dockerfile"
        rm -f "${dockerfile}.bak"
        log_info "Updated: $dockerfile"
    fi

    # Update Go constants
    local config_go="$PROJECT_ROOT/pkg/constants/config.go"
    if [[ -f "$config_go" ]]; then
        sed -i.bak "s|ghcr.io/kylegalloway/forge/zarf-cli:v[0-9]*\.[0-9]*\.[0-9]*|ghcr.io/kylegalloway/forge/zarf-cli:${new_version}|g" "$config_go"
        rm -f "${config_go}.bak"
        log_info "Updated: $config_go"
    fi

    # Update Helm values - find zarf-cli section and update tag
    local values_yaml="$PROJECT_ROOT/chart/forge/values.yaml"
    if [[ -f "$values_yaml" ]]; then
        # Use awk for more reliable multi-line updates
        awk -v ver="$new_version" '
            /repository:.*zarf-cli$/ { in_zarf=1 }
            in_zarf && /tag:/ { sub(/tag: v[0-9]+\.[0-9]+\.[0-9]+/, "tag: " ver); in_zarf=0 }
            { print }
        ' "$values_yaml" > "${values_yaml}.tmp" && mv "${values_yaml}.tmp" "$values_yaml"
        log_info "Updated: $values_yaml (zarfCLI)"
    fi

    # Update zarf.yaml
    local zarf_yaml="$PROJECT_ROOT/zarf.yaml"
    if [[ -f "$zarf_yaml" ]]; then
        sed -i.bak "s|ghcr.io/kylegalloway/forge/zarf-cli:v[0-9]*\.[0-9]*\.[0-9]*|ghcr.io/kylegalloway/forge/zarf-cli:${new_version}|g" "$zarf_yaml"
        rm -f "${zarf_yaml}.bak"
        log_info "Updated: $zarf_yaml"
    fi

    log_info "Zarf CLI updated to ${new_version}"
    echo "$new_version"
}

# Update UDS CLI version
update_uds_version() {
    log_info "Fetching latest UDS CLI version..."
    local new_version
    new_version=$(get_latest_github_release "defenseunicorns/uds-cli")
    log_info "Latest UDS CLI version: $new_version"

    # Update Dockerfile ARG
    local dockerfile="$PROJECT_ROOT/images/uds-cli/Dockerfile"
    if [[ -f "$dockerfile" ]]; then
        sed -i.bak "s/ARG UDS_VERSION=v[0-9]*\.[0-9]*\.[0-9]*/ARG UDS_VERSION=${new_version}/" "$dockerfile"
        rm -f "${dockerfile}.bak"
        log_info "Updated: $dockerfile"
    fi

    # Update Go constants
    local config_go="$PROJECT_ROOT/pkg/constants/config.go"
    if [[ -f "$config_go" ]]; then
        sed -i.bak "s|ghcr.io/defenseunicorns/uds-cli:v[0-9]*\.[0-9]*\.[0-9]*|ghcr.io/defenseunicorns/uds-cli:${new_version}|g" "$config_go"
        rm -f "${config_go}.bak"
        log_info "Updated: $config_go"
    fi

    # Update Helm values - find uds-cli section and update tag
    local values_yaml="$PROJECT_ROOT/chart/forge/values.yaml"
    if [[ -f "$values_yaml" ]]; then
        awk -v ver="$new_version" '
            /repository:.*uds-cli$/ { in_uds=1 }
            in_uds && /tag:/ { sub(/tag: v[0-9]+\.[0-9]+\.[0-9]+/, "tag: " ver); in_uds=0 }
            { print }
        ' "$values_yaml" > "${values_yaml}.tmp" && mv "${values_yaml}.tmp" "$values_yaml"
        log_info "Updated: $values_yaml (udsCLI)"
    fi

    # Update zarf.yaml
    local zarf_yaml="$PROJECT_ROOT/zarf.yaml"
    if [[ -f "$zarf_yaml" ]]; then
        sed -i.bak "s|ghcr.io/kylegalloway/forge/uds-cli:v[0-9]*\.[0-9]*\.[0-9]*|ghcr.io/kylegalloway/forge/uds-cli:${new_version}|g" "$zarf_yaml"
        rm -f "${zarf_yaml}.bak"
        log_info "Updated: $zarf_yaml"
    fi

    log_info "UDS CLI updated to ${new_version}"
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
    echo "  3. Build images:"
    echo "     docker build -t ghcr.io/kylegalloway/forge/zarf-cli:${zarf_version} images/zarf-cli/"
    echo "     docker build -t ghcr.io/kylegalloway/forge/uds-cli:${uds_version} images/uds-cli/"
}

main "$@"
