#!/bin/bash
set -e

# Builds a versioned Swagger UI for each of the last 5 release tags into
# docs/public/swagger-ui/<tag>/.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."

BUILD_OUTPUT="/tmp/swagger-ui-build"
rm -rf "$BUILD_OUTPUT"
mkdir -p "$BUILD_OUTPUT"

for TAG in $(git tag --sort=-creatordate | head -n 5); do
  echo "=== $TAG ==="

  # deploy/00crds.yaml is mirrored into this repo on every release, so it exists at
  # every release tag. A tag without it predates the sync and is skipped.
  if ! git show "$TAG:deploy/00crds.yaml" > "/tmp/00crds-$TAG.yaml" 2>/dev/null; then
    echo "$TAG: Skipping (no deploy/00crds.yaml at this tag)"
    rm -f "/tmp/00crds-$TAG.yaml"
    continue
  fi

  echo "$TAG: Generating OpenAPI..."
  chmod +x "$SCRIPT_DIR/generate_openapi.sh"
  "$SCRIPT_DIR/generate_openapi.sh" "$TAG" "$PROJECT_ROOT" "/tmp/00crds-$TAG.yaml"

  OUTPUT_DIR="$BUILD_OUTPUT/$TAG"
  mkdir -p "$OUTPUT_DIR"
  cp -r "$PROJECT_ROOT/swagger-ui-template/"* "$OUTPUT_DIR/"
  cp "$PROJECT_ROOT/docs/swagger-ui/$TAG/openapi.yaml" "$OUTPUT_DIR/openapi.yaml"
  rm -f "/tmp/00crds-$TAG.yaml"
done

rm -rf "$PROJECT_ROOT/docs/public/swagger-ui"
mkdir -p "$PROJECT_ROOT/docs/public/swagger-ui"
if [ -n "$(ls -A "$BUILD_OUTPUT" 2>/dev/null)" ]; then
  cp -r "$BUILD_OUTPUT"/* "$PROJECT_ROOT/docs/public/swagger-ui/"
  echo "All Swagger UI versions copied to docs/public/swagger-ui/"
else
  echo "::warning::No tag produced a Swagger UI build."
fi
