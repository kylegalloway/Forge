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
        "Taylor Swift|Shake it off, shake it off! We're shaking off bugs with"
        "Kendrick Lamar|Sit down, be humble with version"
        "Beyoncé|Who run the world? Version"
        "Radiohead|No surprises here, just smooth sailing to"
        "Pink Floyd|Comfortably numb? Not with the excitement of version"
        "David Bowie|Ground control to Major Tom, we're launching"
        "The Beatles|Here comes the sun (version)"
        "Walter White|I am the one who knocks... with version"
        "Leslie Knope|Treat yo' self to version"
        "Michael Scott|That's what she said about version"
        "Ron Swanson|Give me all the bacon eggs you have... and version"
        "Phoebe Buffay|Smelly cat, smelly cat, what are they feeding version"
        "Kafka's Metamorphosis|Gregor awoke to find himself transformed into version"
        "Vonnegut|So it goes... on to version"
        "Terry Pratchett|The Luggage follows dutifully to version"
        "Sisyphus|One must imagine Sisyphus happy with version"
        "Icarus|Flying too close to the sun? Not with version"
        "Prometheus|Stealing fire from the gods gets you version"
        "Ragnarök|The twilight of the gods brings version"
        "Odysseus|After ten years at sea, we've reached version"
        "Achilles|My only weakness? Not having version"
        "Dante's Inferno|Abandon all hope ye who don't upgrade to version"
        "The Godfather|I'm gonna make them an offer they can't refuse:"
        "Pulp Fiction|Say 'what' again! I dare you! Version is"
        "Fight Club|First rule of Fight Club: always release version"
        "Blade Runner|I've seen things you people wouldn't believe... like version"
        "Eternal Sunshine|Meet me in Montauk with version"
        "Her|Falling in love with an OS? Try falling for version"
        "Dune|The spice must flow, and so must version"
        "Arrested Development|I've made a huge mistake... NOT releasing version"
        "Community|Six seasons and a movie, plus version"
        "The Good Place|Holy forking shirtballs! It's version"
        "Stranger Things|Friends don't lie about version"
        "Succession|You can't make a Tomlette without breaking some Gregs. Version is"
        "The Office (UK)|That's what the version said"
    )

    # Pick a random reference
    local index=$((RANDOM % ${#references[@]}))
    echo "${references[$index]}"
}

# Function to get chart publishing cultural reference
get_chart_reference() {
    local references=(
        "Captain Ahab finally caught the whale:|Chart ${1} is harpooned and ready"
        "Kerouac hit the road:|Chart ${1} is on the move, man"
        "Columbus discovered America:|We discovered chart ${1}"
        "Armstrong walked on the moon:|One small step for chart ${1}, one giant leap"
        "Amelia Earhart took flight:|Chart ${1} soars into the repository"
        "Hemingway finished his manuscript:|Chart ${1} is written and published"
        "Bob Ross painted happy trees:|Chart ${1} is a happy little accident"
        "Julia Child mastered French cooking:|Chart ${1} is perfectly seasoned"
        "Freddie Mercury gave Wembley:|Chart ${1} rocks the gh-pages stage"
        "Bowie changed personas:|Chart ${1} is the thin white duke of releases"
    )

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
    "docs/development/KIND_TESTING_PUBLIC_IMAGES.md"
    "docs/development/KIND_SETUP.md"
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
git pull origin gh-pages
print_success "gh-pages branch synced with remote"

# Copy chart and update index
cp "$CHART_PACKAGE" .
helm repo index . --url https://kylegalloway.github.io/forge
print_success "Helm index updated"

# Commit and push gh-pages
CHART_REF=$(get_chart_reference "${NEW_VERSION}")
IFS='|' read -r chart_title chart_desc <<< "$CHART_REF"
git add "forge-${NEW_VERSION}.tgz" index.yaml
PRE_COMMIT_ALLOW_NO_CONFIG=1 git commit -S -m "$(cat <<EOF
📦 ${chart_title}

${chart_desc}. The gh-pages harbor welcomes this new arrival.
Updated index.yaml points the way for all who seek this version.
Helm repo update will reveal its glory to the masses.
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
