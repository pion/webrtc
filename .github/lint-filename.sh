#!/usr/bin/env bash
set -e

SCRIPT_PATH=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
GO_REGEX="^[a-zA-Z_]+\.go$"

find  "$SCRIPT_PATH/.." -name "*.go" | while read fullpath; do
  filename=$(basename -- "$fullpath")

  if ! [[ $filename =~ $GO_REGEX ]]; then
      echo "$filename is not a valid filename for Go code, only alpha and underscores are supported"
      exit 1
  fi
done
