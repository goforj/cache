#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GOCACHE_DIR="${GOCACHE_DIR:-/tmp/cache-gocache}"

# integration/root suite owns root-backed drivers only by default.
ROOT_INTEGRATION_DRIVERS="${ROOT_INTEGRATION_DRIVERS:-memory,file,null}"

# Aggregator integration matrix owns container-backed optional-driver integration coverage.
# Defaults to a no-container subset plus sqlitecache sqlite.
# Set to "all" or include "rediscache"/"memcachedcache"/"natscache"/"dynamocache"/"postgrescache"/"mysqlcache"
# to exercise testcontainers-backed paths.
ALL_INTEGRATION_DRIVERS="${ALL_INTEGRATION_DRIVERS:-memory,file,null,sqlitecache}"

RUN_ROOT_UNIT="${RUN_ROOT_UNIT:-1}"
RUN_ROOT_INTEGRATION="${RUN_ROOT_INTEGRATION:-1}"
RUN_AGGREGATOR_INTEGRATION="${RUN_AGGREGATOR_INTEGRATION:-1}"
RUN_REDISCACHE_UNIT="${RUN_REDISCACHE_UNIT:-1}"
RUN_MEMCACHEDCACHE_UNIT="${RUN_MEMCACHEDCACHE_UNIT:-1}"
RUN_NATSCACHE_UNIT="${RUN_NATSCACHE_UNIT:-1}"
RUN_SQLCORE_UNIT="${RUN_SQLCORE_UNIT:-1}"
RUN_SQLITECACHE_UNIT="${RUN_SQLITECACHE_UNIT:-1}"
RUN_POSTGRESCACHE_UNIT="${RUN_POSTGRESCACHE_UNIT:-1}"
RUN_MYSQLCACHE_UNIT="${RUN_MYSQLCACHE_UNIT:-1}"
RUN_SQLITECACHE_INTEGRATION="${RUN_SQLITECACHE_INTEGRATION:-1}"
RUN_DYNAMOCACHE_UNIT="${RUN_DYNAMOCACHE_UNIT:-1}"

run() {
  echo
  echo "==> $*"
  GOCACHE="$GOCACHE_DIR" "$@"
}

if [[ "$RUN_ROOT_UNIT" == "1" ]]; then
  run go test ./...
fi

if [[ "$RUN_REDISCACHE_UNIT" == "1" ]]; then
  run go test ./driver/rediscache
fi

if [[ "$RUN_MEMCACHEDCACHE_UNIT" == "1" ]]; then
  run go test ./driver/memcachedcache
fi

if [[ "$RUN_NATSCACHE_UNIT" == "1" ]]; then
  run go test ./driver/natscache
fi

if [[ "$RUN_SQLCORE_UNIT" == "1" ]]; then
  run go test ./driver/sqlcore
fi

if [[ "$RUN_SQLITECACHE_UNIT" == "1" ]]; then
  run go test ./driver/sqlitecache
fi

if [[ "$RUN_POSTGRESCACHE_UNIT" == "1" ]]; then
  run go test ./driver/postgrescache
fi

if [[ "$RUN_MYSQLCACHE_UNIT" == "1" ]]; then
  run go test ./driver/mysqlcache
fi

if [[ "$RUN_DYNAMOCACHE_UNIT" == "1" ]]; then
  run go test ./driver/dynamocache
fi

if [[ "$RUN_ROOT_INTEGRATION" == "1" ]]; then
  echo
  echo "==> integration/root (INTEGRATION_DRIVER=$ROOT_INTEGRATION_DRIVERS)"
  (
    cd integration
    GOWORK=off GOCACHE="$GOCACHE_DIR" INTEGRATION_DRIVER="$ROOT_INTEGRATION_DRIVERS" \
      go test -tags=integration ./root -run '^TestStoreContract_AllDrivers$'
  )
fi

if [[ "$RUN_AGGREGATOR_INTEGRATION" == "1" ]]; then
  echo
  echo "==> integration/all matrix (INTEGRATION_DRIVER=$ALL_INTEGRATION_DRIVERS)"
  (
    cd integration
    GOWORK=off GOCACHE="$GOCACHE_DIR" INTEGRATION_DRIVER="$ALL_INTEGRATION_DRIVERS" \
      go test -tags=integration ./all -run '^TestStoreContract_AllDrivers$'
  )
fi

if [[ "$RUN_SQLITECACHE_INTEGRATION" == "1" ]]; then
  echo
  echo "==> sqlitecache integration (INTEGRATION_DRIVER=sqlitecache)"
  GOCACHE="$GOCACHE_DIR" INTEGRATION_DRIVER="sqlitecache" \
    go test -tags=integration ./driver/sqlitecache -run '^TestStoreContract_IntegrationSQLite$'
fi

echo
echo "All selected test groups passed."
