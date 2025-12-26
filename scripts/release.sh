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
get_cultural_reference() {
    local references=(
        "Gandalf|You shall not pass... without a version bump"
        "Marty McFly|Great Scott! We're going back to the future with"
        "Neo|Red pill or blue pill? We chose version"
        "Dorothy|There's no place like home, there's no place like"
        "Luke Skywalker|Use the Force, Luke. The Force says bump to"
        "Frodo|One does not simply walk into Mordor. One releases"
        "Dory|Just keep swimming, just keep versioning to"
        "HAL 9000|I'm sorry Dave, I can't do that... without releasing"
        "Ferris Bueller|Life moves pretty fast. So does version"
        "The Dude|The Dude abides, and so does version"
        "Maximus|Are you not entertained?! By version"
        "Beetlejuice|Say it three times: version version version"
        "Tony Stark|I am Iron Man. I am also version"
        "Morpheus|What if I told you... the new version is"
        "Hamlet|To be or not to be version"
        "Rocky|Yo Adrian! We got version"
        "Indiana Jones|It belongs in a museum! Version"
        "Jack Sparrow|Why is the rum always gone? Because we're at version"
        "Yoda|Do or do not, there is no try. There is only version"
        "Forrest Gump|Life is like a box of chocolates, you get version"
    )

    # Pick a random reference
    local index=$((RANDOM % ${#references[@]}))
    echo "${references[$index]}"
}

# Function to generate commit message with cultural reference
generate_commit_message() {
    local new_version=$1
    local bump_type=$2

    local ref_line
    ref_line=$(get_cultural_reference)
    IFS='|' read -r character quote <<< "$ref_line"

    cat <<EOF
📦 ${character}: ${quote} ${new_version}

Bumped ${bump_type} version from ${CURRENT_VERSION} to ${new_version}.
Chart.yaml updated with new version and appVersion. All documentation
references updated to reflect the new release. The prophecy foretold
this day would come, and here we are with a shiny new version number
ready to conquer the Kubernetes landscape.
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
    "docs/getting-started/KIND_TESTING_PUBLIC_IMAGES.md"
    "docs/getting-started/KIND_SETUP.md"
)

for file in "${DOC_FILES[@]}"; do
    if [ -f "$file" ]; then
        update_version_in_file "$file" "$CURRENT_VERSION" "$NEW_VERSION"
        print_success "Updated $file"
    fi
done
echo ""

# Step 3: Commit changes
print_step "Step 3: Creating commit"
git add "$CHART_FILE" "${DOC_FILES[@]}"
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

# Copy chart and update index
cp "$CHART_PACKAGE" .
helm repo index . --url https://kylegalloway.github.io/forge
print_success "Helm index updated"

# Commit and push gh-pages
git add "forge-${NEW_VERSION}.tgz" index.yaml
PRE_COMMIT_ALLOW_NO_CONFIG=1 git commit -S -m "$(cat <<EOF
📦 Chart ${NEW_VERSION} sails into the harbor like Odysseus

The ${NEW_VERSION} chart has completed its epic journey and now rests
safely in the gh-pages harbor. Updated index.yaml points the way for
all who seek this version. Helm repo update will reveal its glory to
the masses. The sirens sang, but we stayed the course.
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
echo ""
print_success "Release automation complete! 🚀"
