#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Running go test ./..."
GOCACHE="$SCRIPT_DIR/.gocache" go test ./...
rm -rf "$SCRIPT_DIR/.gocache"
