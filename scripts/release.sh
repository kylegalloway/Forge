#!/bin/bash
set -e

# Forge Release Automation Script
# Usage: ./scripts/release.sh [major|minor|patch]

BUMP_TYPE=${1:-patch}
CHART_FILE="chart/forge/Chart.yaml"

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
# Format: each entry is a complete title (version number appended at end)
get_release_title() {
    local version=$1
    local references=(
        "Taylor Swift's Eras Tour: welcome to the ${version} era"
        "Kendrick drops a classic: damn. version ${version}"
        "Beyoncé runs the world with version ${version}"
        "Radiohead: no surprises, just ${version}"
        "Pink Floyd's Dark Side of version ${version}"
        "Bowie's Starman brought us ${version}"
        "The Beatles: here comes version ${version}"
        "Walter White knocks with version ${version}"
        "Leslie Knope treats herself to version ${version}"
        "Michael Scott declares bankruptcy on old versions, hello ${version}"
        "Ron Swanson approves of version ${version}"
        "Kafka's Gregor awoke transformed into version ${version}"
        "Vonnegut: so it goes, on to ${version}"
        "Pratchett's Luggage carries version ${version}"
        "Sisyphus rolls ${version} uphill (happily)"
        "Icarus flies closer to version ${version}"
        "Prometheus steals fire, delivers ${version}"
        "Ragnarök brings the twilight of ${version}"
        "Odysseus returns home with version ${version}"
        "The Godfather makes an offer: version ${version}"
        "Pulp Fiction: ${version} is a tasty burger"
        "Fight Club: first rule is release ${version}"
        "Blade Runner: replicants dream of ${version}"
        "Dune: the spice of version ${version} must flow"
        "Arrested Development: there's always money in ${version}"
        "Community: six seasons and version ${version}"
        "The Good Place: holy forking shirtballs it's ${version}"
        "Stranger Things: friends don't lie about ${version}"
        "Succession: L to the OG, version ${version}"
        "Fleabag breaks the fourth wall for ${version}"
        "Ted Lasso believes in version ${version}"
        "Schitt's Creek: ew, David, it's ${version}"
    )

    local index=$((RANDOM % ${#references[@]}))
    echo "${references[$index]}"
}

# Function to get chart publishing title
get_chart_title() {
    local version=$1
    local references=(
        "Ahab harpoons chart ${version}"
        "Kerouac hits the road with chart ${version}"
        "Armstrong lands chart ${version} on gh-pages"
        "Amelia Earhart flies chart ${version} to the repo"
        "Bob Ross paints happy chart ${version}"
        "Julia Child serves chart ${version}"
        "Freddie Mercury rocks chart ${version}"
        "Bowie's Ziggy delivers chart ${version}"
    )

    local index=$((RANDOM % ${#references[@]}))
    echo "${references[$index]}"
}

# Function to generate commit message with cultural reference
generate_commit_message() {
    local new_version=$1
    local bump_type=$2

    cat <<EOF
📦 $(get_release_title "$new_version")

Bumped ${bump_type} version from ${CURRENT_VERSION} to ${new_version}.
Chart.yaml updated with new version and appVersion. All documentation
references updated to reflect the new release.
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

# Update zarf.yaml (uses different tag format - no 'v' prefix for image tags)
if [ -f "zarf.yaml" ]; then
    # Update metadata version (quoted)
    sed -i '' "s/version: \"${CURRENT_VERSION}\"/version: \"${NEW_VERSION}\"/g" "zarf.yaml" 2>/dev/null || \
    sed -i "s/version: \"${CURRENT_VERSION}\"/version: \"${NEW_VERSION}\"/g" "zarf.yaml"
    # Update chart version reference
    sed -i '' "s/version: ${CURRENT_VERSION}/version: ${NEW_VERSION}/g" "zarf.yaml" 2>/dev/null || \
    sed -i "s/version: ${CURRENT_VERSION}/version: ${NEW_VERSION}/g" "zarf.yaml"
    # Update image tags (no 'v' prefix - images are tagged X.Y.Z not vX.Y.Z)
    sed -i '' "s/:${CURRENT_VERSION}/:${NEW_VERSION}/g" "zarf.yaml" 2>/dev/null || \
    sed -i "s/:${CURRENT_VERSION}/:${NEW_VERSION}/g" "zarf.yaml"
    print_success "Updated zarf.yaml"
fi
echo ""

# Step 3: Commit changes
print_step "Step 3: Creating commit"
git add "$CHART_FILE" "${DOC_FILES[@]}" zarf.yaml
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

Published Helm chart ${NEW_VERSION} to gh-pages repository.
Updated index.yaml for helm repo update discovery.
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
echo "     - ghcr.io/kylegalloway/forge/forge-controller:${NEW_VERSION}"
echo "     - ghcr.io/kylegalloway/forge/forge-webhook:${NEW_VERSION}"
echo "     - ghcr.io/kylegalloway/forge/zarf-cli:v0.68.1"
echo ""
print_success "Release automation complete! 🚀"
