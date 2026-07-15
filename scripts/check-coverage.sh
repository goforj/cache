#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOCACHE_DIR="${GOCACHE:-/tmp/gocache}"
GOMODCACHE_DIR="${GOMODCACHE:-/tmp/gomodcache}"
COVERAGE_MIN="${COVERAGE_MIN:-89.5}"
PROFILE="$(mktemp /tmp/cache-root-coverage.XXXXXX)"
trap 'rm -f "$PROFILE"' EXIT

cd "$ROOT_DIR"
GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" \
  go test -coverprofile="$PROFILE" .

coverage="$(go tool cover -func="$PROFILE" | awk '$1 == "total:" { gsub(/%/, "", $3); print $3 }')"
if [[ -z "$coverage" ]]; then
  echo "coverage gate could not read total coverage" >&2
  exit 1
fi

if ! awk -v actual="$coverage" -v minimum="$COVERAGE_MIN" 'BEGIN { exit !(actual + 0 >= minimum + 0) }'; then
  echo "root coverage ${coverage}% is below the ${COVERAGE_MIN}% floor" >&2
  exit 1
fi

echo "root coverage ${coverage}% meets the ${COVERAGE_MIN}% floor"
