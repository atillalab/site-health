#!/bin/bash
#
# Bump the site-health version across all files that reference it.
# Usage: scripts/bump-version.sh <major|minor|patch>
#
# This script updates:
#   - internal/version/version.go
#   - README.md
#   - ../homebrew-tap/Formula/site-health.rb (URL only; SHA must be updated manually after release)
#
# Run from the repository root.

set -euo pipefail

BUMP_TYPE="${1:-}"
if [[ -z "$BUMP_TYPE" ]]; then
	echo "Usage: $0 <major|minor|patch>"
	exit 1
fi

VERSION_FILE="internal/version/version.go"
README_FILE="README.md"
HOMEBREW_FILE="../homebrew-tap/Formula/site-health.rb"

CURRENT=$(grep -oE '[0-9]+\.[0-9]+\.[0-9]+' "$VERSION_FILE" | head -1)
if [[ -z "$CURRENT" ]]; then
	echo "Could not determine current version from $VERSION_FILE"
	exit 1
fi

IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT"

case "$BUMP_TYPE" in
	major)
		MAJOR=$((MAJOR + 1))
		MINOR=0
		PATCH=0
		;;
	minor)
		MINOR=$((MINOR + 1))
		PATCH=0
		;;
	patch)
		PATCH=$((PATCH + 1))
		;;
	*)
		echo "Usage: $0 <major|minor|patch>"
		exit 1
		;;
esac

NEW="$MAJOR.$MINOR.$PATCH"

# Update internal/version/version.go
sed -i.bak "s/const Version = \".*\"/const Version = \"$NEW\"/" "$VERSION_FILE"
rm "$VERSION_FILE.bak"

# Update README.md
sed -i.bak \
	-e "s/v$CURRENT/v$NEW/g" \
	-e "s/\"version\": \"$CURRENT\"/\"version\": \"$NEW\"/g" \
	-e "s/Current version:  $CURRENT/Current version:  $NEW/g" \
	-e "s/Latest version:   $CURRENT/Latest version:   $NEW/g" \
	"$README_FILE"
rm "$README_FILE.bak"

# Update Homebrew formula URL and reset SHA placeholder
if [[ -f "$HOMEBREW_FILE" ]]; then
	sed -i.bak \
		-e "s|tags/v$CURRENT\.tar\.gz|tags/v$NEW.tar.gz|" \
		-e "s|sha256 \".*\"|sha256 \"0000000000000000000000000000000000000000000000000000000000000000\"|" \
		-e "s|v$CURRENT GitHub source tarball|v$NEW GitHub source tarball|" \
		"$HOMEBREW_FILE"
	rm "$HOMEBREW_FILE.bak"
else
	echo "Warning: $HOMEBREW_FILE not found; skipping Homebrew formula update."
fi

echo "Bumped version: $CURRENT -> $NEW"
echo "Next steps:"
echo "  1. Review the diff: git diff"
echo "  2. Update the Homebrew formula SHA after tagging and pushing v$NEW"
echo "  3. Run: go test ./... && go vet ./..."
