#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/check-published-release.sh <version>

Downloads every published cache module into a fresh module cache, verifies that
the requested version resolved exactly, and runs that downloaded module's
package tests. Run this only after every dependency-ordered tag layer is public.
USAGE
}

if [[ $# -ne 1 ]]; then
  usage
  exit 1
fi
if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
  usage
  exit 0
fi

version="$1"
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: version must look like vX.Y.Z (optionally with -prerelease and/or +build suffix)" >&2
  exit 1
fi
release_proxy="${CACHE_RELEASE_GOPROXY:-https://proxy.golang.org}"
if [[ "$release_proxy" =~ (^|[,|])direct($|[,|]) ]]; then
  echo "error: CACHE_RELEASE_GOPROXY must not include a direct VCS fallback" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST_FILE="$ROOT_DIR/scripts/module-manifest.txt"
GOCACHE_DIR="${GOCACHE:-/tmp/gocache}"
TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/cache-published-release.XXXXXX")"
GOMODCACHE_DIR="$TEMP_ROOT/modcache"

cleanup() {
  chmod -R u+w "$TEMP_ROOT" 2>/dev/null || true
  rm -rf "$TEMP_ROOT"
}
trap cleanup EXIT

"$ROOT_DIR/scripts/check-module-manifest.sh" "$version"

published_count=0
while read -r classification module_dir _; do
  if [[ "$classification" != "published" ]]; then
    continue
  fi
  if [[ "$module_dir" == "." ]]; then
    go_mod="$ROOT_DIR/go.mod"
  else
    go_mod="$ROOT_DIR/$module_dir/go.mod"
  fi
  module_path="$(awk '$1 == "module" { print $2; exit }' "$go_mod")"
  if [[ -z "$module_path" ]]; then
    echo "error: module path not found for $module_dir" >&2
    exit 1
  fi

  published_count=$((published_count + 1))
  consumer_dir="$TEMP_ROOT/consumer-$published_count"
  mkdir -p "$consumer_dir"
  (
    cd "$consumer_dir"
    GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOPROXY="$release_proxy" GONOPROXY=none \
      go mod init "releasecheck.local/module-$published_count" >/dev/null
    GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOPROXY="$release_proxy" GONOPROXY=none \
      go mod edit -require="$module_path@$version"
    GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOPROXY="$release_proxy" GONOPROXY=none \
      go mod download "$module_path@$version"

    resolved_version="$(
      GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOPROXY="$release_proxy" GONOPROXY=none \
        go list -m -f '{{.Version}}' "$module_path"
    )"
    if [[ "$resolved_version" != "$version" ]]; then
      echo "error: $module_path resolved at $resolved_version, expected $version" >&2
      exit 1
    fi
    published_dir="$(
      GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOPROXY="$release_proxy" GONOPROXY=none \
        go list -m -f '{{.Dir}}' "$module_path"
    )"
    if [[ -z "$published_dir" ]] || [[ "$published_dir" != "$GOMODCACHE_DIR/"* ]]; then
      echo "error: $module_path resolved outside the fresh module cache: $published_dir" >&2
      exit 1
    fi

    echo "==> published $module_path@$resolved_version"
    (
      cd "$published_dir"
      GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" GOPROXY="$release_proxy" GONOPROXY=none \
        go test -count=1 ./...
    )
  )
done < "$MANIFEST_FILE"

if [[ "$published_count" -eq 0 ]]; then
  echo "error: no published modules found" >&2
  exit 1
fi
echo "published release OK: $published_count modules at $version"
