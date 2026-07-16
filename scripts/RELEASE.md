# Releasing cache modules

`scripts/module-manifest.txt` is the source of truth for module ownership. Entries
classified as `published` are independently consumable Go modules and receive
release tags. Entries classified as `tooling` are repository-local test,
documentation, or example modules; they use the workspace and are not tagged.

Historical releases tagged `docs`, `examples`, and `integration`. Starting with
`v0.4.0`, those tooling tags are intentionally discontinued.

## Published dependency order

| Layer | Modules | Published sibling dependencies |
| --- | --- | --- |
| 1 | `cachecore` | None |
| 2 | `cachetest`, `driver/sqlcore` | `cachecore` |
| 3 | root, `driver/dynamocache`, `driver/memcachedcache`, `driver/mysqlcache`, `driver/natscache`, `driver/postgrescache`, `driver/rediscache`, `driver/sqlitecache` | Layer 1 and, where applicable, layer 2 |

Published `go.mod` files must require the coordinated release version and must
not contain sibling `replace` directives. Local development resolves siblings
through the committed `go.work`, so a fresh checkout tests the repository's
current sibling source instead of requiring unpublished versions.

## Pre-release validation

Before the new tags exist, the public module proxy cannot resolve the planned
version. Validate the staged source with temporary modfiles instead:

```bash
scripts/check-module-manifest.sh v0.4.0
CACHE_LOCAL_SIBLINGS=1 scripts/check-quality.sh
git diff --check
```

`CACHE_LOCAL_SIBLINGS=1` keeps `GOWORK=off` for every Go command but gives each
published module a temporary modfile containing local sibling replacements.
The temporary files are deleted on exit and never alter committed manifests.

## Dependency-ordered tagging

Run each tag layer from a clean commit. The tag script reads only `published`
entries from the module manifest. Later layers need checksum-only staging
commits because a dependency's `go.sum` entry cannot exist until its earlier
tag is available.

1. Review the complete tag plan:

   ```bash
   scripts/tag-all-modules.sh v0.4.0 --dry-run
   ```

2. Publish layer 1:

   ```bash
   scripts/tag-all-modules.sh v0.4.0 --only cachecore --push
   GOWORK=off go mod download github.com/goforj/cache/cachecore@v0.4.0
   ```

3. Once `cachecore` resolves publicly, write its checksums into layer 2,
   validate, commit that staging change, and publish layer 2:

   ```bash
   (
     cd cachetest
     GOWORK=off go mod tidy
     GOWORK=off go vet ./...
     GOWORK=off go test ./...
   )
   (
     cd driver/sqlcore
     GOWORK=off go mod tidy
     GOWORK=off go vet ./...
     GOWORK=off go test ./...
   )
   git add cachetest/go.mod cachetest/go.sum driver/sqlcore/go.mod driver/sqlcore/go.sum
   git commit -m "chore(release): stage cache v0.4.0 layer 2"
   scripts/tag-all-modules.sh v0.4.0 --only cachetest --only driver/sqlcore --push
   GOWORK=off go mod download github.com/goforj/cache/cachetest@v0.4.0
   GOWORK=off go mod download github.com/goforj/cache/driver/sqlcore@v0.4.0
   ```

   The two layer-2 tags are sent with one atomic push. If either tag is
   rejected, neither tag is updated on the remote.

4. Once layer 2 resolves publicly, write dependency checksums into the remaining
   published modules:

   ```bash
   for module in \
     . \
     driver/dynamocache \
     driver/memcachedcache \
     driver/mysqlcache \
     driver/natscache \
     driver/postgrescache \
     driver/rediscache \
     driver/sqlitecache
   do
     (cd "$module" && GOWORK=off go mod tidy)
   done
   scripts/check-quality.sh
   git add \
     go.mod go.sum \
     driver/dynamocache/go.mod driver/dynamocache/go.sum \
     driver/memcachedcache/go.mod driver/memcachedcache/go.sum \
     driver/mysqlcache/go.mod driver/mysqlcache/go.sum \
     driver/natscache/go.mod driver/natscache/go.sum \
     driver/postgrescache/go.mod driver/postgrescache/go.sum \
     driver/rediscache/go.mod driver/rediscache/go.sum \
     driver/sqlitecache/go.mod driver/sqlitecache/go.sum
   git commit -m "chore(release): stage cache v0.4.0 layer 3"
   scripts/tag-all-modules.sh v0.4.0 --skip-existing --push
   ```

   The remaining layer-3 tags are also sent atomically.
   `--skip-existing` accepts earlier-layer tags only when their commits are
   ancestors of the current release commit and their module trees are unchanged;
   it will not silently reuse a stale or divergent release artifact.

5. Confirm the final tag set, rerun the source boundary check, and validate the
   exact published artifacts:

   ```bash
   git tag --list | rg '(^|/)v0[.]4[.]0$'
   scripts/check-quality.sh
   scripts/check-published-release.sh v0.4.0
   ```

   `check-published-release.sh` is intentionally post-tag and networked. It
   creates a fresh module cache, resolves every one of the 11 published modules
   at the requested version, rejects version or source-directory drift, and
   runs tests from each downloaded module rather than from the working tree.
   Its default public proxy has no direct-VCS fallback; set
   `CACHE_RELEASE_GOPROXY` to another proxy-only endpoint for a private mirror.

If a module proxy has not observed a newly pushed dependency tag yet, wait for
that tag to resolve before moving to the next layer. Do not add a committed
replacement to bypass the ordering.
