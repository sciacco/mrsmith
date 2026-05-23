#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -euo pipefail

# URL of the source specification
URL="https://gw-int.cdlan.net/dist.yaml"

# Target paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET_FILE="$WORKSPACE_DIR/docs/mistra-dist.yaml"
TEMP_FILE="$TARGET_FILE.tmp"

echo "Downloading Mistra API specification from $URL..."

# Ensure docs directory exists
mkdir -p "$(dirname "$TARGET_FILE")"

# Perform download using curl with fallback to wget
if command -v curl >/dev/null 2>&1; then
  curl -s -S -f -L -o "$TEMP_FILE" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$TEMP_FILE" "$URL"
else
  echo "Error: Neither curl nor wget is installed." >&2
  exit 1
fi

# Basic validation: ensure the file is not empty and is valid OpenAPI spec
if [ ! -s "$TEMP_FILE" ]; then
  echo "Error: Downloaded file is empty." >&2
  rm -f "$TEMP_FILE"
  exit 1
fi

if ! grep -q -E "openapi:|swagger:" "$TEMP_FILE"; then
  echo "Error: Downloaded file does not seem to be a valid OpenAPI/Swagger specification." >&2
  rm -f "$TEMP_FILE"
  exit 1
fi

# Replace target file
mv "$TEMP_FILE" "$TARGET_FILE"
echo "Success! Mistra specification updated at: docs/mistra-dist.yaml"
