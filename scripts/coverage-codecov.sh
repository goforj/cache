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
INTEGRATION_MODULE_DIR="${INTEGRATION_MODULE_DIR:-integration}"
INTEGRATION_MODULE_PKGS="${INTEGRATION_MODULE_PKGS:-./root ./all}"
INTEGRATION_MODULE_COVERPKG="${INTEGRATION_MODULE_COVERPKG:-github.com/goforj/cache/...}"
INTEGRATION_MODULE_DRIVERS="${INTEGRATION_MODULE_DRIVERS:-memory,file,null,sqlitecache}"

UNIT_DIR="$TMP_ROOT/unit"
INT_DIR="$TMP_ROOT/integration"
INT_MOD_DIR="$TMP_ROOT/integration-module"
MERGED_DIR="$TMP_ROOT/merged"

rm -rf "$TMP_ROOT"
mkdir -p "$UNIT_DIR" "$INT_DIR" "$INT_MOD_DIR" "$MERGED_DIR"

echo "==> Unit coverage collection"
GOCACHE="$GOCACHE_DIR" \
go test -cover -coverpkg=./... $UNIT_PKGS -args -test.gocoverdir="$UNIT_DIR"

echo "==> Integration coverage collection"
GOCACHE="$GOCACHE_DIR" \
go test -cover -tags="$INTEGRATION_TAGS" -coverpkg=./... $INTEGRATION_PKGS -args -test.gocoverdir="$INT_DIR"

if [[ -d "$INTEGRATION_MODULE_DIR" ]]; then
  echo "==> Integration module coverage collection ($INTEGRATION_MODULE_DIR)"
  (
    cd "$INTEGRATION_MODULE_DIR"
    GOWORK=off GOCACHE="$GOCACHE_DIR" INTEGRATION_DRIVER="$INTEGRATION_MODULE_DRIVERS" \
    go test -cover -tags="$INTEGRATION_TAGS" -coverpkg="$INTEGRATION_MODULE_COVERPKG" $INTEGRATION_MODULE_PKGS \
      -args -test.gocoverdir="$INT_MOD_DIR"
  )
fi

echo "==> Merging coverage data"
MERGE_INPUTS="$UNIT_DIR,$INT_DIR"
if [[ -d "$INT_MOD_DIR" ]]; then
  MERGE_INPUTS="$MERGE_INPUTS,$INT_MOD_DIR"
fi
go tool covdata merge -i="$MERGE_INPUTS" -o="$MERGED_DIR"

mkdir -p "$(dirname "$OUTPUT_FILE")"
go tool covdata textfmt -i="$MERGED_DIR" -o="$OUTPUT_FILE"

echo "==> Combined coverage written to $OUTPUT_FILE"
go tool covdata percent -i="$MERGED_DIR"
