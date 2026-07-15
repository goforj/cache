#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOCACHE_DIR="${GOCACHE:-/tmp/gocache}"
GOMODCACHE_DIR="${GOMODCACHE:-/tmp/gomodcache}"
LOCAL_SIBLINGS="${CACHE_LOCAL_SIBLINGS:-0}"
MANIFEST_FILE="$ROOT_DIR/scripts/module-manifest.txt"

if [[ "$LOCAL_SIBLINGS" != "0" ]] && [[ "$LOCAL_SIBLINGS" != "1" ]]; then
  echo "error: CACHE_LOCAL_SIBLINGS must be 0 or 1" >&2
  exit 1
fi

"$ROOT_DIR/scripts/check-module-manifest.sh"

is_published_module() {
  local wanted="$1"
  local classification module_dir
  while read -r classification module_dir _; do
    if [[ "$classification" == "published" ]] && [[ "$module_dir" == "$wanted" ]]; then
      return 0
    fi
  done < "$MANIFEST_FILE"
  return 1
}

create_local_modfile() {
  local module_dir="$1"
  local module_label="$2"
  local temp_sum classification sibling_dir sibling_path sibling_abs safe_label
  safe_label="${module_label//\//-}"
  safe_label="${safe_label//./root}"
  temp_mod="$(mktemp "${TMPDIR:-/tmp}/cache-staged-${safe_label}.XXXXXX.mod")"
  temp_sum="${temp_mod%.mod}.sum"
  cp "$module_dir/go.mod" "$temp_mod"
  if [[ -f "$module_dir/go.sum" ]]; then
    awk '$1 !~ /^github[.]com\/goforj\/cache(\/|$)/ { print }' "$module_dir/go.sum" > "$temp_sum"
  fi

  while read -r classification sibling_dir _; do
    if [[ "$classification" != "published" ]] || [[ "$sibling_dir" == "$module_label" ]]; then
      continue
    fi
    if [[ "$sibling_dir" == "." ]]; then
      sibling_abs="$ROOT_DIR"
    else
      sibling_abs="$ROOT_DIR/$sibling_dir"
    fi
    sibling_path="$(awk '$1 == "module" { print $2; exit }' "$sibling_abs/go.mod")"
    printf '\nreplace %s => %s\n' "$sibling_path" "$sibling_abs" >> "$temp_mod"
  done < "$MANIFEST_FILE"

}

while IFS= read -r module_file; do
  module_dir="${module_file%/go.mod}"
  module_label="${module_dir#"$ROOT_DIR"}"
  module_label="${module_label#/}"
  if [[ -z "$module_label" ]]; then
    module_manifest_dir="."
    module_label="root"
  else
    module_manifest_dir="$module_label"
  fi
  echo "==> quality $module_label"
  (
    cd "$module_dir"
    temp_mod=""
    trap 'if [[ -n "$temp_mod" ]]; then rm -f "$temp_mod" "${temp_mod%.mod}.sum"; fi' EXIT
    modfile_args=()
    if [[ "$LOCAL_SIBLINGS" == "1" ]] && is_published_module "$module_manifest_dir"; then
      create_local_modfile "$module_dir" "$module_manifest_dir"
      modfile_args=("-modfile=$temp_mod")
    fi
    go_version="$(awk '$1 == "go" { print $2; exit }' go.mod)"
    GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" go mod tidy -diff -go="$go_version" "${modfile_args[@]}"
    packages="$(GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" go list "${modfile_args[@]}" ./...)"
    if [[ -n "$packages" ]]; then
      GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" go vet "${modfile_args[@]}" ./...
      GOWORK=off GOCACHE="$GOCACHE_DIR" GOMODCACHE="$GOMODCACHE_DIR" go test "${modfile_args[@]}" ./...
    else
      echo "No default-build packages; tagged suites run in integration CI."
    fi
  )
done < <(find "$ROOT_DIR" -name go.mod -not -path '*/.git/*' -print | sort)
