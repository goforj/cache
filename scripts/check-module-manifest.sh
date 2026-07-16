#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST_FILE="$ROOT_DIR/scripts/module-manifest.txt"
EXPECTED_VERSION="${1:-}"
MODULE_PREFIX="github.com/goforj/cache"

if [[ -n "$EXPECTED_VERSION" ]] && [[ ! "$EXPECTED_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: expected version must look like vX.Y.Z or vX.Y.Z-prerelease" >&2
  exit 1
fi

module_dirs=()
published_dirs=()
tooling_dirs=()
failures=0

contains_dir() {
  local wanted="$1"
  shift
  local candidate
  for candidate in "$@"; do
    if [[ "$candidate" == "$wanted" ]]; then
      return 0
    fi
  done
  return 1
}

list_sibling_requirements() {
  local go_mod="$1"
  awk '
    function is_sibling(path) {
      return path == "github.com/goforj/cache" ||
        index(path, "github.com/goforj/cache/") == 1
    }
    $1 == "require" && $2 == "(" {
      in_require = 1
      next
    }
    in_require && $1 == ")" {
      in_require = 0
      next
    }
    in_require && is_sibling($1) {
      print $1, $2
      next
    }
    $1 == "require" && is_sibling($2) {
      print $2, $3
    }
  ' "$go_mod"
}

list_sibling_replacements() {
  local go_mod="$1"
  awk '
    function is_sibling(path) {
      return path == "github.com/goforj/cache" ||
        index(path, "github.com/goforj/cache/") == 1
    }
    $1 == "replace" && $2 == "(" {
      in_replace = 1
      next
    }
    in_replace && $1 == ")" {
      in_replace = 0
      next
    }
    in_replace && is_sibling($1) {
      print NR ":" $0
      next
    }
    $1 == "replace" && is_sibling($2) {
      print NR ":" $0
    }
  ' "$go_mod"
}

while read -r classification module_dir extra; do
  if [[ -z "$classification" ]] || [[ "$classification" == \#* ]]; then
    continue
  fi
  if [[ -n "${extra:-}" ]]; then
    echo "error: malformed manifest entry: $classification $module_dir $extra" >&2
    failures=1
    continue
  fi
  if [[ "$module_dir" == /* ]] || [[ "$module_dir" == *".."* ]]; then
    echo "error: module directory must stay within the repository: $module_dir" >&2
    failures=1
    continue
  fi
  if contains_dir "$module_dir" "${module_dirs[@]-}"; then
    echo "error: duplicate module manifest entry: $module_dir" >&2
    failures=1
    continue
  fi

  case "$classification" in
    published)
      published_dirs+=("$module_dir")
      ;;
    tooling)
      tooling_dirs+=("$module_dir")
      ;;
    *)
      echo "error: unknown module classification '$classification' for $module_dir" >&2
      failures=1
      continue
      ;;
  esac
  module_dirs+=("$module_dir")

  if [[ "$module_dir" == "." ]]; then
    go_mod="$ROOT_DIR/go.mod"
    expected_path="$MODULE_PREFIX"
  else
    go_mod="$ROOT_DIR/$module_dir/go.mod"
    expected_path="$MODULE_PREFIX/$module_dir"
  fi
  if [[ ! -f "$go_mod" ]]; then
    echo "error: manifest entry has no go.mod: $module_dir" >&2
    failures=1
    continue
  fi
  actual_path="$(awk '$1 == "module" { print $2; exit }' "$go_mod")"
  if [[ "$actual_path" != "$expected_path" ]]; then
    echo "error: $module_dir declares '$actual_path'; expected '$expected_path'" >&2
    failures=1
  fi
done < "$MANIFEST_FILE"

while IFS= read -r go_mod; do
  module_dir="${go_mod%/go.mod}"
  module_dir="${module_dir#"$ROOT_DIR"}"
  module_dir="${module_dir#/}"
  if [[ -z "$module_dir" ]]; then
    module_dir="."
  fi
  if ! contains_dir "$module_dir" "${module_dirs[@]-}"; then
    echo "error: unclassified module: $module_dir" >&2
    failures=1
  fi
done < <(find "$ROOT_DIR" -name go.mod -not -path "$ROOT_DIR/.git/*" -print | sort)

sibling_version=""
for module_dir in "${published_dirs[@]}"; do
  if [[ "$module_dir" == "." ]]; then
    go_mod="$ROOT_DIR/go.mod"
  else
    go_mod="$ROOT_DIR/$module_dir/go.mod"
  fi

  replacements="$(list_sibling_replacements "$go_mod")"
  if [[ -n "$replacements" ]]; then
    echo "error: published module $module_dir contains sibling replacement(s):" >&2
    echo "$replacements" >&2
    failures=1
  fi

  while read -r dependency version; do
    if [[ -z "${dependency:-}" ]]; then
      continue
    fi
    if [[ "$version" == "v0.0.0" ]]; then
      echo "error: published module $module_dir requires $dependency at placeholder v0.0.0" >&2
      failures=1
    fi
    if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
      echo "error: published module $module_dir requires $dependency at non-release version $version" >&2
      failures=1
    fi
    if [[ -z "$sibling_version" ]]; then
      sibling_version="$version"
    elif [[ "$version" != "$sibling_version" ]]; then
      echo "error: coordinated sibling versions differ: $dependency is $version, expected $sibling_version" >&2
      failures=1
    fi
    if [[ -n "$EXPECTED_VERSION" ]] && [[ "$version" != "$EXPECTED_VERSION" ]]; then
      echo "error: $module_dir requires $dependency at $version, expected $EXPECTED_VERSION" >&2
      failures=1
    fi
  done < <(list_sibling_requirements "$go_mod")
done

if [[ "$failures" -ne 0 ]]; then
  exit 1
fi

version_note=""
if [[ -n "$sibling_version" ]]; then
  version_note="; sibling version $sibling_version"
fi
echo "module manifest OK: ${#published_dirs[@]} published, ${#tooling_dirs[@]} tooling$version_note"
