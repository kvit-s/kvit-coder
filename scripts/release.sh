#!/bin/bash
set -e

# Release build script for kvit-coder
# Builds optimized binary with embedded version info

# Check if git working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Git working directory is not clean"
    echo "Please commit or stash your changes before building a release"
    git status --short
    exit 1
fi

# Get version info
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
COMMIT_HASH=$(git rev-parse --short HEAD)
COMMIT_DATE=$(git log -1 --format=%cd --date=format:%Y%m%d)
BUILD_DATE=$(date -u +%Y-%m-%d)

echo "Building release..."
echo "  Version: ${VERSION}"
echo "  Commit:  ${COMMIT_HASH} (${COMMIT_DATE})"
echo "  Build:   ${BUILD_DATE}"

# Build with optimizations and version info
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commitHash=${COMMIT_HASH} -X main.commitDate=${COMMIT_DATE} -X main.buildDate=${BUILD_DATE}"

go build -ldflags="${LDFLAGS}" -trimpath -o kvit-coder ./cmd/kvit-coder/
go build -ldflags="${LDFLAGS}" -trimpath -o kvit-coder-ui ./cmd/kvit-coder-ui/

echo "Built:"
ls -lh kvit-coder kvit-coder-ui
