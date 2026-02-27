#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/tag-all-modules.sh <version> [--push] [--remote <name>] [--dry-run] [--allow-dirty] [--skip-existing] [--exclude <module-dir>]

Examples:
  scripts/tag-all-modules.sh v0.1.3 --dry-run
  scripts/tag-all-modules.sh v0.1.3 --push
  scripts/tag-all-modules.sh v0.1.3 --push --exclude docs --exclude examples
  scripts/tag-all-modules.sh v0.1.3 --exclude driver

Behavior:
  - Tags root module as: vX.Y.Z
  - Tags each submodule as: <relative/module/path>/vX.Y.Z
  - Uses the current HEAD commit for all tags
  - --exclude supports exact module dirs and prefixes (for example: driver excludes all driver/* modules)
USAGE
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

version=""
push=0
remote="origin"
dry_run=0
allow_dirty=0
skip_existing=0
excludes=()

normalize_module_dir() {
  local dir="$1"
  dir="${dir#./}"
  dir="${dir%/}"
  if [[ -z "$dir" ]]; then
    dir="."
  fi
  printf '%s\n' "$dir"
}

module_is_excluded() {
  local dir="$1"
  local ex
  for ex in "${excludes[@]-}"; do
    if [[ "$dir" == "$ex" ]] || [[ "$dir" == "$ex/"* ]]; then
      return 0
    fi
  done
  return 1
}

remote_tag_exists() {
  local remote_name="$1"
  local tag="$2"
  git ls-remote --exit-code --tags --refs "$remote_name" "refs/tags/$tag" >/dev/null 2>&1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --push)
      push=1
      shift
      ;;
    --remote)
      remote="${2:-}"
      if [[ -z "$remote" ]]; then
        echo "error: --remote requires a value" >&2
        exit 1
      fi
      shift 2
      ;;
    --dry-run)
      dry_run=1
      shift
      ;;
    --allow-dirty)
      allow_dirty=1
      shift
      ;;
    --skip-existing)
      skip_existing=1
      shift
      ;;
    --exclude)
      mod="${2:-}"
      if [[ -z "$mod" ]]; then
        echo "error: --exclude requires a module directory value" >&2
        exit 1
      fi
      excludes+=("$(normalize_module_dir "$mod")")
      shift 2
      ;;
    v*)
      if [[ -n "$version" ]]; then
        echo "error: multiple versions provided" >&2
        exit 1
      fi
      version="$1"
      shift
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$version" ]]; then
  echo "error: version is required (example: v0.1.3)" >&2
  exit 1
fi

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: version must look like vX.Y.Z (optionally with -prerelease and/or +build suffix)" >&2
  exit 1
fi

root="$(git rev-parse --show-toplevel)"
cd "$root"

if [[ ! -f go.mod ]]; then
  echo "error: must run inside a Go module repository" >&2
  exit 1
fi

if [[ "$allow_dirty" -eq 0 ]] && [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree is dirty. commit/stash or pass --allow-dirty" >&2
  exit 1
fi

module_dirs=()
while IFS= read -r dir; do
  dir="$(normalize_module_dir "$dir")"
  module_dirs+=("$dir")
done <<EOF_MODULES
$(find . -name go.mod -type f \
  -not -path './.git/*' \
  -not -path './*/.git/*' \
  -not -path './*/vendor/*' \
  -exec dirname {} \; | sed 's#^\./##' | sort)
EOF_MODULES

if [[ ${#module_dirs[@]} -eq 0 ]]; then
  echo "error: no modules discovered" >&2
  exit 1
fi

tags_to_create=()
tags_to_push=()
for dir in "${module_dirs[@]}"; do
  if module_is_excluded "$dir"; then
    continue
  fi

  if [[ "$dir" == "." ]]; then
    tag="$version"
  else
    tag="$dir/$version"
  fi

  if ! git check-ref-format "refs/tags/$tag" >/dev/null 2>&1; then
    echo "error: computed invalid tag ref: $tag (from module dir: $dir)" >&2
    exit 1
  fi

  local_exists=0
  remote_exists=0

  if git rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1; then
    local_exists=1
  fi

  if [[ "$push" -eq 1 ]] && remote_tag_exists "$remote" "$tag"; then
    remote_exists=1
  fi

  if [[ "$local_exists" -eq 1 ]] || [[ "$remote_exists" -eq 1 ]]; then
    if [[ "$skip_existing" -eq 1 ]]; then
      if [[ "$local_exists" -eq 1 ]] && [[ "$remote_exists" -eq 0 ]] && [[ "$push" -eq 1 ]]; then
        echo "reuse local tag for push: $tag"
        tags_to_push+=("$tag")
      else
        echo "skip existing: $tag"
      fi
      continue
    fi

    if [[ "$local_exists" -eq 1 ]]; then
      echo "error: local tag already exists: $tag" >&2
    else
      echo "error: remote tag already exists on $remote: $tag" >&2
    fi
    exit 1
  fi

  tags_to_create+=("$tag")
  if [[ "$push" -eq 1 ]]; then
    tags_to_push+=("$tag")
  fi
done

if [[ ${#tags_to_create[@]} -eq 0 ]] && [[ ${#tags_to_push[@]} -eq 0 ]]; then
  echo "nothing to do"
  exit 0
fi

echo "repo: $root"
echo "head: $(git rev-parse --short HEAD)"
echo "version: $version"
if [[ ${#excludes[@]} -gt 0 ]]; then
  echo "excluded modules: ${excludes[*]}"
fi
if [[ ${#tags_to_create[@]} -gt 0 ]]; then
  echo "create tags (${#tags_to_create[@]}):"
  for t in "${tags_to_create[@]}"; do
    echo "  - $t"
  done
fi
if [[ "$push" -eq 1 ]] && [[ ${#tags_to_push[@]} -gt 0 ]]; then
  echo "push tags (${#tags_to_push[@]}):"
  for t in "${tags_to_push[@]}"; do
    echo "  - $t"
  done
fi

if [[ "$dry_run" -eq 1 ]]; then
  echo "dry-run: no tags created"
  exit 0
fi

if [[ ${#tags_to_create[@]} -gt 0 ]]; then
  for t in "${tags_to_create[@]}"; do
    git tag -a "$t" -m "release $t"
  done
fi

if [[ ${#tags_to_create[@]} -gt 0 ]]; then
  echo "created ${#tags_to_create[@]} tags"
fi

if [[ "$push" -eq 1 ]]; then
  git push "$remote" "${tags_to_push[@]}"
  echo "pushed ${#tags_to_push[@]} tags to $remote"
else
  echo "not pushed (use --push)"
fi
