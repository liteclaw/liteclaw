#!/bin/bash
# Build script for LiteClaw

set -e

VERSION=${VERSION:-"dev"}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X github.com/liteclaw/liteclaw/internal/version.Version=$VERSION"
LDFLAGS="$LDFLAGS -X github.com/liteclaw/liteclaw/internal/version.Commit=$COMMIT"
LDFLAGS="$LDFLAGS -X github.com/liteclaw/liteclaw/internal/version.BuildDate=$BUILD_DATE"

echo "Building LiteClaw..."
echo "  Version: $VERSION"
echo "  Commit:  $COMMIT"
echo "  Date:    $BUILD_DATE"

# Build for current platform
CGO_ENABLED=1 go build -ldflags="$LDFLAGS" -o liteclaw ./cmd/liteclaw

echo "Build complete: ./liteclaw"
