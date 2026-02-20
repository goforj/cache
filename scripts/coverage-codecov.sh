#!/usr/bin/env bash
set -euo pipefail

# Generates a merged Go coverage profile suitable for Codecov upload.
# Default output matches current CI convention: coverage.txt

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUTPUT_FILE="${COVERAGE_OUTPUT:-coverage.txt}"
TMP_ROOT="${COVERAGE_TMP_DIR:-/tmp/cache-coverage}"
GOCACHE_DIR="${GOCACHE_DIR:-/tmp/go-build-cache}"
UNIT_PKGS="${UNIT_PKGS:-./...}"
INTEGRATION_PKGS="${INTEGRATION_PKGS:-./...}"
INTEGRATION_TAGS="${INTEGRATION_TAGS:-integration}"

UNIT_DIR="$TMP_ROOT/unit"
INT_DIR="$TMP_ROOT/integration"
MERGED_DIR="$TMP_ROOT/merged"

rm -rf "$TMP_ROOT"
mkdir -p "$UNIT_DIR" "$INT_DIR" "$MERGED_DIR"

echo "==> Unit coverage collection"
GOCACHE="$GOCACHE_DIR" \
go test -cover -coverpkg=./... $UNIT_PKGS -args -test.gocoverdir="$UNIT_DIR"

echo "==> Integration coverage collection"
GOCACHE="$GOCACHE_DIR" \
go test -cover -tags="$INTEGRATION_TAGS" -coverpkg=./... $INTEGRATION_PKGS -args -test.gocoverdir="$INT_DIR"

echo "==> Merging coverage data"
go tool covdata merge -i="$UNIT_DIR,$INT_DIR" -o="$MERGED_DIR"

mkdir -p "$(dirname "$OUTPUT_FILE")"
go tool covdata textfmt -i="$MERGED_DIR" -o="$OUTPUT_FILE"

echo "==> Combined coverage written to $OUTPUT_FILE"
go tool covdata percent -i="$MERGED_DIR"
