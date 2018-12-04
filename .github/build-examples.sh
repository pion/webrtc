#!/usr/bin/env bash
set -e

SCRIPT_PATH=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
TEMP_DIR=$(mktemp -d)

cp -r "$SCRIPT_PATH/../examples" "$TEMP_DIR"

# Remove go.mod files libraries have already been installed
find $TEMP_DIR -name 'go.mod' | while read fname; do
    rm "$fname"
done

# Build every example out of tree, ensuring we don't accidentally use non-portable constructs
find $TEMP_DIR -name 'main.go' | while read fname; do
  BUILD_DIR=$(dirname "$fname")
  cd $BUILD_DIR

  BUILD_OUTPUT=$(go build 2>&1 >/dev/null || true)
  if [ -n "$BUILD_OUTPUT" ]; then
    echo "Failed to build $BUILD_DIR"
    echo "$BUILD_OUTPUT"
    exit 1;
  fi
done

rm -rf "$TEMP_DIR"
