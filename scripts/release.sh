#!/bin/bash
set -e

# Forge Release Automation Script
# Usage: ./scripts/release.sh [major|minor|patch]

BUMP_TYPE=${1:-patch}
CHART_FILE="chart/forge/Chart.yaml"
VALUES_FILE="chart/forge/values.yaml"
CHANGELOG_FILE="CHANGELOG.md"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
    echo -e "${CYAN}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Function to extract version from Chart.yaml
get_current_version() {
    grep '^version:' "$CHART_FILE" | awk '{print $2}'
}

# Function to bump version based on type
bump_version() {
    local version=$1
    local bump_type=$2

    # Split version into parts
    IFS='.' read -r -a parts <<< "$version"
    local major="${parts[0]}"
    local minor="${parts[1]}"
    local patch="${parts[2]}"

    case "$bump_type" in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
        *)
            print_error "Invalid bump type: $bump_type (must be major, minor, or patch)"
            exit 1
            ;;
    esac

    echo "${major}.${minor}.${patch}"
}

# Array of cultural references for commit messages (rotate through these)
# Format: References are EMBEDDED naturally - no "Artist: description" labels
get_release_title() {
    local version=$1
    local references=(
        "Welcome to the ${version} era (we hope you like it here)"
        "Damn. Version ${version} just dropped"
        "Who run the world? Version ${version}"
        "No surprises, just ${version}"
        "There is no dark side of ${version} really - it's all dark"
        "There's a starman waiting in ${version}"
        "Here comes version ${version}, doo-doo-doo-doo"
        "I am the one who releases ${version}"
        "Treat yo'self to version ${version}"
        "I declare bankruptcy on all versions before ${version}"
        "Give me all the bacon and eggs you have: ${version}"
        "One morning Gregor awoke transformed into version ${version}"
        "So it goes. On to ${version}"
        "The turtle moves, and so does ${version}"
        "One must imagine ${version} happy"
        "The sun melted the wax, but ${version} soars on"
        "Fire stolen from the gods, delivered as ${version}"
        "The old versions fall; ${version} rises from the ashes"
        "Ten years to get here, but ${version} is home"
        "An offer you can't refuse: version ${version}"
        "Now that's a tasty version ${version}"
        "First rule: we release ${version}"
        "Replicants dream of version ${version}"
        "The spice of ${version} must flow"
        "There's always money in version ${version}"
        "Six seasons and a version ${version}"
        "Holy forking shirtballs it's ${version}"
        "Friends don't lie, and neither does ${version}"
        "L to the OG, version ${version}"
        "This is a love story about ${version}"
        "Believe in version ${version}"
        "Ew, David, it's only version ${version}"
    )

    local index=$((RANDOM % ${#references[@]}))
    echo "${references[$index]}"
}

# Function to get chart publishing title
get_chart_title() {
    local version=$1
    local references=(
        "Thar she blows! Chart ${version} harpooned"
        "On the road again with chart ${version}"
        "One small step for helm, one giant chart ${version}"
        "Chart ${version} crosses the Atlantic"
        "Happy little chart ${version}"
        "Bon appétit, chart ${version}"
        "We will rock chart ${version}"
        "Chart ${version} fell to earth"
        "Chart ${version} rides eternal, shiny and chrome"
        "To infinity and chart ${version}"
        "Winter is coming, but so is chart ${version}"
        "I'll be back with chart ${version}"
    )

    local index=$((RANDOM % ${#references[@]}))
    echo "${references[$index]}"
}

# Function to capitalize first letter (bash 3.2 compatible)
capitalize() {
    local str=$1
    local first
    first=$(echo "${str:0:1}" | tr '[:lower:]' '[:upper:]')
    echo "${first}${str:1}"
}

# Function to generate commit message with cultural reference
generate_commit_message() {
    local new_version=$1
    local bump_type=$2
    local cap_bump_type
    cap_bump_type=$(capitalize "$bump_type")

    cat <<EOF
📦 $(get_release_title "$new_version")

${cap_bump_type} version bump: ${CURRENT_VERSION} → ${new_version}

Updated files:
- chart/forge/Chart.yaml (version and appVersion)
- chart/forge/values.yaml (controller and webhook image tags)
- CHANGELOG.md (Unreleased → ${new_version})
- README.md and user documentation
- zarf.yaml package metadata

The tag v${new_version} triggers the release workflow, which builds
and publishes container images for controller, webhook, zarfpackagejob,
and udsbundlejob to ghcr.io.
EOF
}

# Function to update version in a file
update_version_in_file() {
    local file=$1
    local old_version=$2
    local new_version=$3

    # Update version: X.Y.Z references
    sed -i '' "s/version: ${old_version}/version: ${new_version}/g" "$file" 2>/dev/null || \
    sed -i "s/version: ${old_version}/version: ${new_version}/g" "$file"

    # Update --version X.Y.Z references
    sed -i '' "s/--version ${old_version}/--version ${new_version}/g" "$file" 2>/dev/null || \
    sed -i "s/--version ${old_version}/--version ${new_version}/g" "$file"

    # Update vX.Y.Z references
    sed -i '' "s/v${old_version}/v${new_version}/g" "$file" 2>/dev/null || \
    sed -i "s/v${old_version}/v${new_version}/g" "$file"

    # Update :vX.Y.Z tag references
    sed -i '' "s/:v${old_version}/:v${new_version}/g" "$file" 2>/dev/null || \
    sed -i "s/:v${old_version}/:v${new_version}/g" "$file"
}

# Function to update image tags in values.yaml
# Changes tag: "" or tag: "vX.Y.Z" to tag: "vNEW_VERSION" for controller and webhook
update_values_image_tags() {
    local new_version=$1
    local values_file=$2

    # Update controller image tag (handles both empty "" and existing "vX.Y.Z" values)
    # Matches the pattern under controller.image.tag
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS sed - update controller tag
        sed -i '' '/^controller:/,/^[a-z]/ {
            /^  image:/,/^  [a-z]/ {
                s/tag: ".*"/tag: "v'"${new_version}"'"/
            }
        }' "$values_file"
        # Update webhook tag
        sed -i '' '/^webhook:/,/^[a-z]/ {
            /^  image:/,/^  [a-z]/ {
                s/tag: ".*"/tag: "v'"${new_version}"'"/
            }
        }' "$values_file"
    else
        # GNU sed
        sed -i '/^controller:/,/^[a-z]/ {
            /^  image:/,/^  [a-z]/ {
                s/tag: ".*"/tag: "v'"${new_version}"'"/
            }
        }' "$values_file"
        sed -i '/^webhook:/,/^[a-z]/ {
            /^  image:/,/^  [a-z]/ {
                s/tag: ".*"/tag: "v'"${new_version}"'"/
            }
        }' "$values_file"
    fi
}

# Function to update CHANGELOG.md for a new release
# - Moves [Unreleased] content to a new version section with today's date
# - Creates a fresh [Unreleased] section
# - Updates comparison links at the bottom
update_changelog() {
    local old_version=$1
    local new_version=$2
    local changelog_file=$3
    local today
    today=$(date +%Y-%m-%d)

    # Step 1: Replace "## [Unreleased]" with "## [Unreleased]\n\n## [X.Y.Z] - YYYY-MM-DD"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS sed - need to use literal newlines differently
        sed -i '' "s/^## \[Unreleased\]$/## [Unreleased]\\
\\
## [${new_version}] - ${today}/" "$changelog_file"
    else
        # GNU sed
        sed -i "s/^## \[Unreleased\]$/## [Unreleased]\n\n## [${new_version}] - ${today}/" "$changelog_file"
    fi

    # Step 2: Update the [Unreleased] comparison link to point to the new version
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s|\[Unreleased\]: \(.*\)/compare/v${old_version}\.\.\.HEAD|[Unreleased]: \1/compare/v${new_version}...HEAD|" "$changelog_file"
    else
        sed -i "s|\[Unreleased\]: \(.*\)/compare/v${old_version}\.\.\.HEAD|[Unreleased]: \1/compare/v${new_version}...HEAD|" "$changelog_file"
    fi

    # Step 3: Add the new version comparison link after the [Unreleased] link
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "/^\[Unreleased\]:.*HEAD$/a\\
[${new_version}]: https://github.com/kylegalloway/forge/compare/v${old_version}...v${new_version}" "$changelog_file"
    else
        sed -i "/^\[Unreleased\]:.*HEAD$/a [${new_version}]: https://github.com/kylegalloway/forge/compare/v${old_version}...v${new_version}" "$changelog_file"
    fi
}

# Main script starts here
print_step "🚀 Forge Release Automation"
echo ""

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    print_error "Must be on main branch to release (currently on: $CURRENT_BRANCH)"
    exit 1
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    print_error "Uncommitted changes detected. Please commit or stash them first."
    git status --short
    exit 1
fi

# Get current version
CURRENT_VERSION=$(get_current_version)
print_step "Current version: ${BLUE}${CURRENT_VERSION}${NC}"

# Calculate new version
NEW_VERSION=$(bump_version "$CURRENT_VERSION" "$BUMP_TYPE")
print_step "New version (${BUMP_TYPE} bump): ${GREEN}${NEW_VERSION}${NC}"
echo ""

# Confirm with user
read -p "$(echo -e ${YELLOW}Proceed with release ${NEW_VERSION}? [y/N]: ${NC})" -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_warning "Release cancelled"
    exit 0
fi
echo ""

# Step 1: Update Chart.yaml
print_step "Step 1: Updating Chart.yaml"
sed -i '' "s/^version: ${CURRENT_VERSION}/version: ${NEW_VERSION}/" "$CHART_FILE" 2>/dev/null || \
sed -i "s/^version: ${CURRENT_VERSION}/version: ${NEW_VERSION}/" "$CHART_FILE"
sed -i '' "s/^appVersion: \"v${CURRENT_VERSION}\"/appVersion: \"v${NEW_VERSION}\"/" "$CHART_FILE" 2>/dev/null || \
sed -i "s/^appVersion: \"v${CURRENT_VERSION}\"/appVersion: \"v${NEW_VERSION}\"/" "$CHART_FILE"
print_success "Chart.yaml updated"

# Step 1b: Update values.yaml image tags
print_step "Step 1b: Updating values.yaml image tags"
update_values_image_tags "$NEW_VERSION" "$VALUES_FILE"
print_success "values.yaml image tags updated (controller and webhook → v${NEW_VERSION})"

# Step 2: Update documentation files
print_step "Step 2: Updating documentation"
DOC_FILES=(
    "README.md"
    "docs/getting-started/USER_GUIDE.md"
    "docs/development/KIND_TESTING_PUBLIC_IMAGES.md"
    "docs/development/KIND_SETUP.md"
)

for file in "${DOC_FILES[@]}"; do
    if [ -f "$file" ]; then
        update_version_in_file "$file" "$CURRENT_VERSION" "$NEW_VERSION"
        print_success "Updated $file"
    fi
done

# Update zarf.yaml
if [ -f "zarf.yaml" ]; then
    # Update metadata version (quoted)
    sed -i '' "s/version: \"${CURRENT_VERSION}\"/version: \"${NEW_VERSION}\"/g" "zarf.yaml" 2>/dev/null || \
    sed -i "s/version: \"${CURRENT_VERSION}\"/version: \"${NEW_VERSION}\"/g" "zarf.yaml"
    # Update chart version reference
    sed -i '' "s/version: ${CURRENT_VERSION}/version: ${NEW_VERSION}/g" "zarf.yaml" 2>/dev/null || \
    sed -i "s/version: ${CURRENT_VERSION}/version: ${NEW_VERSION}/g" "zarf.yaml"
    # Update image tags (with 'v' prefix - images are tagged vX.Y.Z)
    sed -i '' "s/:v${CURRENT_VERSION}/:v${NEW_VERSION}/g" "zarf.yaml" 2>/dev/null || \
    sed -i "s/:v${CURRENT_VERSION}/:v${NEW_VERSION}/g" "zarf.yaml"
    print_success "Updated zarf.yaml"
fi

# Step 2b: Update CHANGELOG.md
print_step "Step 2b: Updating CHANGELOG.md"
if [ -f "$CHANGELOG_FILE" ]; then
    update_changelog "$CURRENT_VERSION" "$NEW_VERSION" "$CHANGELOG_FILE"
    print_success "CHANGELOG.md updated (Unreleased → ${NEW_VERSION})"
else
    print_warning "CHANGELOG.md not found, skipping"
fi
echo ""

# Step 3: Commit changes
print_step "Step 3: Creating commit"
git add "$CHART_FILE" "$VALUES_FILE" "$CHANGELOG_FILE" "${DOC_FILES[@]}" zarf.yaml
COMMIT_MSG=$(generate_commit_message "$NEW_VERSION" "$BUMP_TYPE")
git commit -S -m "$COMMIT_MSG"
print_success "Changes committed"
echo ""

# Step 4: Create and push tags
print_step "Step 4: Creating tags"
git tag -s "v${NEW_VERSION}" -m "Release v${NEW_VERSION}

Release of Forge v${NEW_VERSION}
"
print_success "Created tag v${NEW_VERSION}"

# Update latest tag
git tag -d latest 2>/dev/null || true
git tag -s latest -m "Latest stable release (v${NEW_VERSION})"
print_success "Updated latest tag"
echo ""

# Step 5: Push to origin
print_step "Step 5: Pushing to origin"
git push origin main
git push origin "v${NEW_VERSION}"
git push origin latest --force
print_success "Pushed main branch and tags"
echo ""

# Step 6: Package and publish Helm chart
print_step "Step 6: Packaging Helm chart"
TMP_DIR=$(mktemp -d)
helm package "chart/forge" -d "$TMP_DIR"
CHART_PACKAGE="${TMP_DIR}/forge-${NEW_VERSION}.tgz"
print_success "Chart packaged: forge-${NEW_VERSION}.tgz"
echo ""

# Step 7: Update gh-pages
print_step "Step 7: Publishing to gh-pages"
git stash --include-untracked 2>/dev/null || true
git checkout gh-pages
git pull origin gh-pages
print_success "gh-pages branch synced with remote"

# Copy chart and update index
cp "$CHART_PACKAGE" .
helm repo index . --url https://kylegalloway.github.io/forge
print_success "Helm index updated"

# Commit and push gh-pages
git add "forge-${NEW_VERSION}.tgz" index.yaml
PRE_COMMIT_ALLOW_NO_CONFIG=1 git commit -S -m "$(cat <<EOF
📦 $(get_chart_title "${NEW_VERSION}")

Helm chart ${NEW_VERSION} now available via:
  helm repo add forge https://kylegalloway.github.io/forge
  helm install forge forge/forge --version ${NEW_VERSION}

Packaged chart added to gh-pages with updated index.yaml for
automatic discovery on helm repo update.
EOF
)"
git push origin gh-pages
print_success "Published to gh-pages"

# Cleanup and return to main
git checkout main
git stash pop 2>/dev/null || true
rm -rf "$TMP_DIR"
echo ""

# Summary
print_step "🎉 Release Complete!"
echo ""
echo -e "${GREEN}Version ${NEW_VERSION} has been released!${NC}"
echo ""
echo "Summary:"
echo "  • Version bumped: ${CURRENT_VERSION} → ${NEW_VERSION}"
echo "  • Commit created with signed GPG signature"
echo "  • Tags created: v${NEW_VERSION}, latest"
echo "  • Helm chart published to gh-pages"
echo ""
echo "Next steps:"
echo "  1. GitHub Actions will build and push container images for v${NEW_VERSION}"
echo "  2. Users can install with: helm install forge forge/forge --version ${NEW_VERSION}"
echo "  3. Images will be available at:"
echo "     - ghcr.io/kylegalloway/forge/forge-controller:v${NEW_VERSION}"
echo "     - ghcr.io/kylegalloway/forge/forge-webhook:v${NEW_VERSION}"
echo "     - ghcr.io/kylegalloway/forge/zarfpackagejob:v${NEW_VERSION}"
echo "     - ghcr.io/kylegalloway/forge/udsbundlejob:v${NEW_VERSION}"
echo ""
print_success "Release automation complete! 🚀"
