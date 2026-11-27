<p align="center">
  <img src="https://raw.githubusercontent.com/goforj/cache/main/docs/images/logo.png" width="300" alt="cache logo">
</p>

<p align="center">
    One cache API for Go, from in-process memory to shared backends.
</p>

<p align="center">
    <a href="https://pkg.go.dev/github.com/goforj/cache"><img src="https://pkg.go.dev/badge/github.com/goforj/cache.svg" alt="Go Reference"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <a href="https://golang.org"><img src="https://img.shields.io/badge/go-1.27+-blue?logo=go" alt="Go version"></a>
    <img src="https://img.shields.io/github/v/tag/goforj/cache?label=version&sort=semver" alt="Latest tag">
    <a href="https://codecov.io/gh/goforj/cache"><img src="https://codecov.io/gh/goforj/cache/graph/badge.svg" alt="Codecov"></a>
<!-- test-count:embed:start -->
    <img src="https://img.shields.io/badge/unit_tests-447-brightgreen" alt="Unit tests (passing count)">
    <img src="https://img.shields.io/badge/local_integration_tests-109-blue" alt="Local integration tests (passing count)">
<!-- test-count:embed:end -->
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> ·
  <a href="#drivers">Drivers</a> ·
  <a href="#behavior-semantics">Behavior</a> ·
  <a href="#api-index">API</a>
</p>

## Quick Start

<a id="installation"></a>

In a Go 1.27+ module, install cache, save either example as `main.go`, and run `go run .`.

```sh
go get github.com/goforj/cache@latest
```

<!-- quick-start:embed:start -->

```go
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/goforj/cache"
)

// main loads and caches a price in cents.
func main() {
	// Create an in-memory cache store.
	c := cache.NewCache(cache.NewMemoryStore(context.Background()))

	// Load the price on a cache miss and keep it for one minute.
	price, err := c.Remember("product:42:price", time.Minute, func() (int, error) {
		return strconv.Atoi("4900")
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Desk lamp costs %d cents\n", price)
	// Output: Desk lamp costs 4900 cents
}
```

<!-- quick-start:embed:end -->

### Get, Set, and Delete

<!-- quick-start-operations:embed:start -->

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/goforj/cache"
)

// main shows direct cache operations for a piece of product inventory.
func main() {
	// Create an in-memory cache store.
	c := cache.NewCache(cache.NewMemoryStore(context.Background()))
	const key = "stock:desk-lamp"

	// Cache the stock count for one minute.
	if err := c.Set(key, 12, time.Minute); err != nil {
		log.Fatal(err)
	}

	// Read the count as an int. A cache miss sets found to false.
	stock, found, err := c.Get[int](key)
	if err != nil || !found {
		log.Fatalf("read stock: found=%t, error=%v", found, err)
	}
	fmt.Printf("Cached stock: %d\n", stock)
	// Output: Cached stock: 12

	// Delete the entry and verify the next read misses.
	if err := c.Delete(key); err != nil {
		log.Fatal(err)
	}
	_, found, err = c.Get[int](key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Cached after delete: %t\n", found)
	// Output: Cached after delete: false
}
```

<!-- quick-start-operations:embed:end -->

## Drivers

Memory, file, and null are included in the root module. Install only the additional driver modules you use.

| Backend | Use and limits |
| :--- | :--- |
| <a href="#newmemorystore"><img src="https://img.shields.io/badge/memory-5c5c5c?logo=cachet&logoColor=white" alt="Memory"></a> | In-process cache. Values, locks, and counters are local to each process. |
| <a href="#newfilestore"><img src="https://img.shields.io/badge/file-3f51b5?logo=files&logoColor=white" alt="File"></a> | Local files that survive restarts. Use a dedicated directory. Locks are process-local. |
| <a href="#newnullstore"><img src="https://img.shields.io/badge/null-9e9e9e?logo=probot&logoColor=white" alt="Null"></a> | Disable caching. Drops writes and admits every lock. |
| <a href="#driver-config-rediscache"><img src="https://img.shields.io/badge/redis-%23DC382D?logo=redis&logoColor=white" alt="Redis"></a> | Shared cache with native counters. Counter expiry has one-second resolution. |
| <a href="#driver-config-memcachedcache"><img src="https://img.shields.io/badge/memcached-0198c4?logo=buffer&logoColor=white" alt="Memcached"></a> | Shared, ephemeral cache. Keys route to a stable peer. TTL resolution is one second. |
| <a href="#driver-config-natscache"><img src="https://img.shields.io/badge/nats-27AAE1?logo=natsdotio&logoColor=white" alt="NATS JetStream KV"></a> | Uses an existing JetStream KV bucket with per-key or bucket-wide TTL. |
| <a href="#driver-config-dynamocache"><img src="https://img.shields.io/badge/dynamodb-4053D6?logo=amazon-dynamodb&logoColor=white" alt="DynamoDB"></a> | Shared cache using a DynamoDB table. Item limits include encoded values and metadata. |
| <a href="#driver-config-sqlitecache"><img src="https://img.shields.io/badge/sqlite-003B57?logo=sqlite&logoColor=white" alt="SQLite"></a> | Embedded SQL cache. Use a dedicated file or database. |
| <a href="#driver-config-postgrescache"><img src="https://img.shields.io/badge/postgres-336791?logo=postgresql&logoColor=white" alt="PostgreSQL"></a> | Shared SQL cache backed by PostgreSQL. Requires access to a cache table. |
| <a href="#driver-config-mysqlcache"><img src="https://img.shields.io/badge/mysql-4479A1?logo=mysql&logoColor=white" alt="MySQL"></a> | Shared SQL cache backed by MySQL. Requires access to a cache table. |

```sh
go get github.com/goforj/cache/driver/rediscache@latest
go get github.com/goforj/cache/driver/memcachedcache@latest
go get github.com/goforj/cache/driver/natscache@latest
go get github.com/goforj/cache/driver/dynamocache@latest
go get github.com/goforj/cache/driver/sqlitecache@latest
go get github.com/goforj/cache/driver/postgrescache@latest
go get github.com/goforj/cache/driver/mysqlcache@latest
```

<a id="driver-constructor-quick-examples"></a>

See [Driver Configs](#driver-configs) for construction examples and the [Production Guide](docs/production-guide.md#driver-connections-and-rollout) for connection ownership and rollout.

## Supported Go versions

| Modules | Minimum Go version |
| :--- | :--- |
| Root `cache` | 1.27.0 |
| `cachecore`, `cachetest`, `driver/sqlcore` | 1.24.4 |
| DynamoDB, Memcached, MySQL, and Redis drivers | 1.24.4 |
| PostgreSQL and SQLite drivers | 1.25.0 |
| NATS driver | 1.26.0 |

The generic method API requires Go 1.27 or newer. Projects on Go 1.26 can pin the root at v0.4.0 and select independently published drivers that support their toolchain. Documentation, examples, and integration tooling use Go 1.27.

## Config Options

Cache uses explicit config structs throughout, with shared fields embedded via `cachecore.BaseConfig`.

Shared config (embedded by root stores and optional drivers):

```go
type BaseConfig struct {
	DefaultTTL    time.Duration
	Prefix        string
	Compression   CompressionCodec
	MaxValueBytes int
	EncryptionKey []byte
}
```

Root-backed stores use `cache.StoreConfig`:

```go
type StoreConfig struct {
	cachecore.BaseConfig
	MemoryCleanupInterval time.Duration
	FileDir               string
}
```

Typical root constructor usage:

```go
store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	MemoryCleanupInterval: time.Minute,
})
```

Optional backends use driver-local config types that embed the same `cachecore.BaseConfig` plus backend-specific fields.

Example shapes:

```go
// rediscache.Config (abridged)
type Config struct {
	cachecore.BaseConfig
	Client rediscache.Client
}
```

```go
// sqlitecache.Config (abridged)
type Config struct {
	cachecore.BaseConfig
	DSN   string
	Table string
}
```

See the [API Index](#api-index) `Driver Configs` section for per-driver defaults and compile-checked examples for:
`rediscache`, `memcachedcache`, `natscache`, `dynamocache`, `sqlitecache`, `postgrescache`, `mysqlcache`, and `sqlcore`.

## Value Shaping And Rollout

Every root and optional driver honors the shared value-shaping fields:

- `CompressionGzip` writes a versioned `CMP1` envelope.
- `EncryptionKey` writes an AES-GCM `ENC1` envelope and requires a 16, 24, or 32 byte key.
- `MaxValueBytes` bounds both writes and reads, including decompressed output; zero disables it and negative values are invalid.
- Backend-native counters remain raw so `Increment` and `Decrement` stay atomic.

When compression and encryption are both enabled, the preserved on-disk/wire format is
`CMP1(ENC1(plaintext))`: encryption happens first and gzip is the outer envelope. Readers pass
unmarked legacy values through unchanged, so upgraded applications can read existing raw entries.

Optional drivers before this quality pass accepted these fields but did not apply them. Once an
upgraded process writes a shaped value, an older process cannot decode it. Upgrade all readers
together (or stop writes during a rolling upgrade) before enabling shaped writes. See
[Migration to v0.4](docs/migration-v0.4.md) for the full compatibility checklist.

## Behavior Semantics

For precise runtime semantics, see [Behavior Semantics](docs/behavior-semantics.md):

- TTL/default-TTL matrix by operation/helper
- stale and refresh-ahead behavior and edge cases
- lock and rate-limit guarantees (process-local vs distributed scope)

## Production Guidance

<a id="payload-size-caps-effective-bytes-written"></a>

For deployment defaults and operational patterns, see [Production Guide](docs/production-guide.md):

- recommended defaults and tuning
- key naming/versioning conventions
- [payload size limits by backend](docs/production-guide.md#payload-size-limits)
- TTL jitter and miss-storm mitigation
- observability instrumentation patterns

## Memoized reads

Create a `NewMemoStore` per request when a short-lived read snapshot is useful.

```go
memoStore := cache.NewMemoStore(store)
memoRepo := cache.NewCache(memoStore)
```

Memoization holds up to 256 hits or misses for up to one second. Use the backing store directly when you need its exact TTL or immediate visibility of writes made through another handle.

<a id="module-layout"></a>

## Testing

<a id="integration-coverage"></a>

For contributors, the [module manifest](scripts/module-manifest.txt) lists the published drivers and repository tooling.

Unit tests cover the public helpers. See [Integration coverage](docs/integration-coverage.md) for the shared store contract, driver-specific checks, and suite scope. Run the cross-driver suite from the `integration` module:

```bash
cd integration
go test -tags=integration ./all
```

Use `INTEGRATION_DRIVER=sqlitecache` (comma-separated) to select which fixtures run, or use the repo helper:

```bash
bash scripts/test-all-modules.sh
```

`scripts/check-quality.sh` runs tidy-diff, vet, and tests independently in every module. CI also
enforces a 97.0% root package statement-coverage floor with `scripts/check-coverage.sh`. The quick start runs in CI and its output is checked.

## Benchmarks

The charts show a recorded benchmark snapshot. Rerun the benchmarks on the release and backend versions you plan to deploy.

```bash
cd docs
go test -tags benchrender ./bench -run TestRenderBenchmarks -count=1 -v
```

Note: NATS numbers can look slower than Redis/memory because the NATS driver preserves per-operation TTL semantics by storing per-key expiry metadata (envelope encode/decode) and may do extra compare/update steps for some operations.
Generic helper benchmarks (`Get[T]` / `Set[T]`) use the default JSON codec, so compare them against `GetBytes` / `SetBytes` (and `GetString` / `SetString`) when evaluating convenience vs raw-path performance.

<!-- bench:embed:start -->

Note: DynamoDB is intentionally omitted from these local charts because emulator-based numbers are not representative of real AWS latency.

NATS variants in these charts:

- `nats`: per-key TTL semantics using a binary envelope (`magic/expiresAt/value`). This preserves per-key expiry parity with other drivers, with modest metadata overhead.
- `nats_bucket_ttl`: bucket-level TTL mode (`natscache.Config.BucketTTL`), raw value path; faster but different expiry semantics.

### Latency (ns/op)

![Cache benchmark latency chart](docs/bench/benchmarks_ns.svg)

### Iterations (N)

![Cache benchmark iteration chart](docs/bench/benchmarks_ops.svg)

### Allocated Bytes (B/op)

![Cache benchmark bytes chart](docs/bench/benchmarks_bytes.svg)

### Allocations (allocs/op)

![Cache benchmark allocs chart](docs/bench/benchmarks_allocs.svg)

<!-- bench:embed:end -->

## API reference

The API section below is autogenerated; do not edit between the markers.
Bind request context with `c.WithContext(ctx)`. Package-level typed helpers remain available alongside generic Cache methods.

<!-- api:embed:start -->

## API Index

| Group | Functions |
|------:|:-----------|
| **Constructors** | [NewFileStore](#newfilestore) · [NewFileStoreWithConfig](#newfilestorewithconfig) · [NewMemoryStore](#newmemorystore) · [NewMemoryStoreWithConfig](#newmemorystorewithconfig) · [NewNullStore](#newnullstore) · [NewNullStoreWithConfig](#newnullstorewithconfig) |
| **Core** | [Cache.WithContext](#cache-withcontext) · [Driver](#cache-driver) · [Inspector](#cache-inspector) · [NewCache](#newcache) · [NewCacheWithTTL](#newcachewithttl) · [Ready](#cache-ready) · [Store](#cache-store) |
| **Driver Configs** | [Shared BaseConfig](#driver-configs-shared-baseconfig) · [DynamoDB Config](#driver-config-dynamocache) · [Memcached Config](#driver-config-memcachedcache) · [MySQL Config](#driver-config-mysqlcache) · [NATS Config](#driver-config-natscache) · [Postgres Config](#driver-config-postgrescache) · [Redis Config](#driver-config-rediscache) · [SQL Core Config](#driver-config-sqlcore) · [SQLite Config](#driver-config-sqlitecache) |
| **Invalidation** | [Cache.Pull](#cache-pull) · [Delete](#cache-delete) · [DeleteMany](#cache-deletemany) · [Flush](#cache-flush) · [PullBytes](#cache-pullbytes) |
| **Locking** | [Acquire](#lockhandle-acquire) · [Block](#lockhandle-block) · [Lock](#cache-lock) · [NewLockHandle](#cache-newlockhandle) · [Release](#lockhandle-release) · [TryLock](#cache-trylock) · [Unlock](#cache-unlock) |
| **Memoization** | [NewMemoStore](#newmemostore) |
| **Observability** | [OnCacheOp](#observerfunc-oncacheop) · [WithObserver](#cache-withobserver) |
| **Other** |  |
| **Rate Limiting** | [RateLimit](#cache-ratelimit) |
| **Read Through** | [Cache.Remember](#cache-remember) · [Cache.RememberStale](#cache-rememberstale) · [RememberBytes](#cache-rememberbytes) · [RememberStaleBytes](#cache-rememberstalebytes) |
| **Reads** | [BatchGetBytes](#cache-batchgetbytes) · [Cache.Get](#cache-get) · [Cache.GetJSON](#cache-getjson) · [GetBytes](#cache-getbytes) · [GetString](#cache-getstring) · [ListPage](#listpage) |
| **Refresh Ahead** | [Cache.RefreshAhead](#cache-refreshahead) · [Cache.RefreshAheadValueWithCodec](#cache-refreshaheadvaluewithcodec) · [RefreshAheadBytes](#cache-refreshaheadbytes) |
| **Testing Helpers** | [AssertCalled](#fake-assertcalled) · [AssertNotCalled](#fake-assertnotcalled) · [AssertTotal](#fake-asserttotal) · [Cache](#fake-cache) · [Count](#fake-count) · [New](#new) · [Reset](#fake-reset) · [Total](#fake-total) |
| **Writes** | [Add](#cache-add) · [BatchSetBytes](#cache-batchsetbytes) · [Cache.Set](#cache-set) · [Cache.SetJSON](#cache-setjson) · [Decrement](#cache-decrement) · [Increment](#cache-increment) · [SetBytes](#cache-setbytes) · [SetString](#cache-setstring) |


_Examples assume `ctx := context.Background()` and `c := cache.NewCache(cache.NewMemoryStore(ctx))` unless shown otherwise._

## Constructors

### <a id="newfilestore"></a>NewFileStore

NewFileStore is a convenience for a filesystem-backed store.

```go
ctx := context.Background()
store := cache.NewFileStore(ctx, "/tmp/my-cache")
fmt.Println(store.Driver()) // file
```

### <a id="newfilestorewithconfig"></a>NewFileStoreWithConfig

NewFileStoreWithConfig builds a filesystem-backed store using explicit root config.

```go
ctx := context.Background()
store := cache.NewFileStoreWithConfig(ctx, cache.StoreConfig{
	BaseConfig: cachecore.BaseConfig{
		EncryptionKey: []byte("01234567890123456789012345678901"),
		MaxValueBytes: 4096,
		Compression:   cache.CompressionGzip,
	},
	FileDir: "/tmp/my-cache",
})
fmt.Println(store.Driver()) // file
```

### <a id="newmemorystore"></a>NewMemoryStore

NewMemoryStore is a convenience for an in-process store using defaults.

```go
ctx := context.Background()
store := cache.NewMemoryStore(ctx)
fmt.Println(store.Driver()) // memory
```

### <a id="newmemorystorewithconfig"></a>NewMemoryStoreWithConfig

NewMemoryStoreWithConfig builds an in-process store using explicit root config.

```go
ctx := context.Background()
store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL:  30 * time.Second,
		Compression: cache.CompressionGzip,
	},
	MemoryCleanupInterval: 5 * time.Minute,
})
fmt.Println(store.Driver()) // memory
```

### <a id="newnullstore"></a>NewNullStore

NewNullStore is a no-op store useful for tests where caching should be disabled.

```go
ctx := context.Background()
store := cache.NewNullStore(ctx)
fmt.Println(store.Driver()) // null
```

### <a id="newnullstorewithconfig"></a>NewNullStoreWithConfig

NewNullStoreWithConfig builds a null store with shared wrappers (compression/encryption/limits).

```go
ctx := context.Background()
store := cache.NewNullStoreWithConfig(ctx, cache.StoreConfig{
	BaseConfig: cachecore.BaseConfig{
		Compression:   cache.CompressionGzip,
		MaxValueBytes: 1024,
	},
})
fmt.Println(store.Driver()) // null
```

## Core

### <a id="cache-withcontext"></a>Cache.WithContext

WithContext returns a derived cache handle that binds ctx to subsequent operations.

### <a id="cache-driver"></a>Driver

Driver reports the underlying store driver.

### <a id="cache-inspector"></a>Inspector

Inspector returns the optional browsing interface for the underlying store.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
inspector, ok := c.Inspector()
fmt.Println(ok, inspector.Capabilities().CanList) // true true
```

### <a id="newcache"></a>NewCache

NewCache creates a cache facade bound to a concrete store.
NewCache panics when store is nil because a cache cannot operate without its required backend.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
fmt.Println(c.Driver()) // memory
```

### <a id="newcachewithttl"></a>NewCacheWithTTL

NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.
NewCacheWithTTL panics when store is nil because accepting invalid wiring would defer failure to the first operation.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCacheWithTTL(s, 2*time.Minute)
fmt.Println(c.Driver(), c != nil) // memory true
```

### <a id="cache-ready"></a>Ready

Ready checks whether the underlying store is ready to serve requests.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Ready() == nil) // true
```

### <a id="cache-store"></a>Store

Store returns the underlying store implementation.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Store().Driver()) // memory
```

## <a id="driver-configs"></a>Driver Configs

Optional backend config examples (compile-checked from generated examples and driver `New(...)` docs).

### <a id="driver-configs-shared-baseconfig"></a>Shared `cachecore.BaseConfig`

Shared fields are embedded via `cachecore.BaseConfig` on every driver config:

- `DefaultTTL`: defaults to `5*time.Minute` when zero in all optional drivers
- `Prefix`: defaults to `"app"` when empty in all optional drivers
- `Compression`: gzip writes a versioned CMP1 envelope; the zero value leaves bytes uncompressed
- `MaxValueBytes`: bounds writes and decoded reads; `0` disables the limit
- `EncryptionKey`: AES-GCM writes a versioned ENC1 envelope; `nil` disables encryption
- `Increment` and `Decrement`: bypass shaping so backend-native counters remain atomic

### <a id="driver-config-dynamocache"></a>DynamoDB

Defaults:
- Region: "us-east-1" when empty
- Table: "cache_entries" when empty
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- Client: auto-created when nil (uses Region and optional Endpoint)
- Endpoint: empty by default (normal AWS endpoint resolution)

```go
ctx := context.Background()
store, err := dynamocache.New(ctx, dynamocache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	Region: "us-east-1",
	Table:  "cache_entries",
})
if err != nil {
	panic(err)
}
fmt.Println(store.Driver()) // dynamo
```

### <a id="driver-config-memcachedcache"></a>Memcached

release them through io.Closer. Addresses are treated as a fixed peer set.
A failed peer returns an error for its keys so locks never fail over to another peer.

Defaults:
- Addresses: []string{"127.0.0.1:11211"} when empty
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- AllowServerFlush: false, unless every server is dedicated to this cache

```go
store := memcachedcache.New(memcachedcache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	Addresses: []string{"127.0.0.1:11211"},
})
fmt.Println(store.Driver()) // memcached
```

### <a id="driver-config-mysqlcache"></a>MySQL

Defaults:
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- Table: "cache_entries" when empty
- DSN: required

```go
store, err := mysqlcache.New(mysqlcache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	DSN:   "user:pass@tcp(127.0.0.1:3306)/app?parseTime=true",
	Table: "cache_entries",
})
if err != nil {
	panic(err)
}
fmt.Println(store.Driver()) // sql
```

### <a id="driver-config-natscache"></a>NATS

Defaults:
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- BucketTTL: false (TTL enforced in value envelope metadata)
- KeyValue: required for real operations (nil allowed, operations return errors)

```go
var kv natscache.KeyValue // provided by your NATS setup
store := natscache.New(natscache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	KeyValue:  kv,
	BucketTTL: false,
})
fmt.Println(store.Driver()) // nats
```

### <a id="driver-config-postgrescache"></a>Postgres

Defaults:
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- Table: "cache_entries" when empty
- DSN: required

```go
store, err := postgrescache.New(postgrescache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	DSN:   "postgres://user:pass@localhost:5432/app?sslmode=disable",
	Table: "cache_entries",
})
if err != nil {
	panic(err)
}
fmt.Println(store.Driver()) // sql
```

### <a id="driver-config-rediscache"></a>Redis

Defaults:
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- Addr: empty by default (no client auto-created unless Addr is set)
- Client: optional advanced override (takes precedence when set)
- Client overrides must also implement Eval to use ownership-safe locking
- If neither Client nor Addr is set, operations return errors until a client is provided

```go
store := rediscache.New(rediscache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	Addr: "127.0.0.1:6379",
})
fmt.Println(store.Driver()) // redis
```

### <a id="driver-config-sqlcore"></a>SQL Core (advanced/shared implementation)

the store and can release its database pool through io.Closer.

Defaults:
- Table: "cache_entries" when empty
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- DriverName: required
- DSN: required

```go
store, err := sqlcore.New(sqlcore.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	DriverName: "sqlite",
	DSN:        "file::memory:?cache=shared",
	Table:      "cache_entries",
})
if err != nil {
	panic(err)
}
fmt.Println(store.Driver()) // sql
```

### <a id="driver-config-sqlitecache"></a>SQLite

Defaults:
- DefaultTTL: 5*time.Minute when zero
- Prefix: "app" when empty
- Table: "cache_entries" when empty
- DSN: required

```go
store, err := sqlitecache.New(sqlitecache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 5 * time.Minute,
		Prefix:     "app",
	},
	DSN:   "file::memory:?cache=shared",
	Table: "cache_entries",
})
if err != nil {
	panic(err)
}
fmt.Println(store.Driver()) // sql
```


## Invalidation

### <a id="cache-pull"></a>Cache.Pull

Pull reads a typed JSON value and then deletes its key. The operations are separate.

```go
type Token struct { Value string `json:"value"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.Set("preview:banner:42", Token{Value: "abc"}, time.Minute)
tok, ok, err := c.Pull[Token]("preview:banner:42")
fmt.Println(err == nil, ok, tok.Value) // true true abc
```

### <a id="cache-delete"></a>Delete

Delete removes a single key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.SetBytes("a", []byte("1"), time.Minute)
fmt.Println(c.Delete("a") == nil) // true
```

### <a id="cache-deletemany"></a>DeleteMany

DeleteMany removes multiple keys.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.DeleteMany("a", "b") == nil) // true
```

### <a id="cache-flush"></a>Flush

Flush clears all keys for this store scope.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.SetBytes("a", []byte("1"), time.Minute)
fmt.Println(c.Flush() == nil) // true
```

### <a id="pull"></a>Pull

Pull reads a typed JSON value and then deletes its key. The operations are separate.
It remains available for compatibility; new code can use Cache.Pull.

### <a id="cache-pullbytes"></a>PullBytes

PullBytes reads a value and then deletes its key. The operations are separate.
Concurrent readers can receive the same value.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.SetString("preview:banner:42", "abc", time.Minute)
body, ok, _ := c.PullBytes("preview:banner:42")
fmt.Println(ok, string(body)) // true abc
```

## Locking

### <a id="lockhandle-acquire"></a>Acquire

Acquire attempts to acquire the lock once (non-blocking).

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
lock := c.NewLockHandle("job:sync", 10*time.Second)
locked, err := lock.Acquire()
fmt.Println(err == nil, locked) // true true
```

### <a id="lockhandle-block"></a>Block

Block waits up to timeout to acquire the lock, runs fn if acquired, then releases.

retryInterval <= 0 falls back to the cache default lock retry interval.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
lock := c.NewLockHandle("job:sync", 10*time.Second)
locked, err := lock.Block(500*time.Millisecond, 25*time.Millisecond, func() error {
	// do protected work
	return nil
})
fmt.Println(err == nil, locked) // true true
```

### <a id="cache-lock"></a>Lock

Lock waits until this Cache instance and the backend can begin a lock lifecycle or timeout elapses.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, err := c.Lock("job:sync", 10*time.Second, time.Second)
fmt.Println(err == nil, locked) // true true
if locked {
	_ = c.Unlock("job:sync")
}
```

### <a id="lockhandle-get"></a>LockHandle.Get

Get acquires the lock once, runs fn if acquired, then releases automatically.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
lock := c.NewLockHandle("job:sync", 10*time.Second)
locked, err := lock.Get(func() error {
	// do protected work
	return nil
})
fmt.Println(err == nil, locked) // true true
```

### <a id="cache-newlockhandle"></a>NewLockHandle

NewLockHandle creates a reusable lock handle for a key/ttl pair.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
lock := c.NewLockHandle("job:sync", 10*time.Second)
locked, err := lock.Acquire()
fmt.Println(err == nil, locked) // true true
if locked {
	_ = lock.Release()
}
```

### <a id="lockhandle-release"></a>Release

Release unlocks the key if this handle previously acquired and still owns it.

It is safe to call multiple times; repeated calls become no-ops after the first
successful release.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
lock := c.NewLockHandle("job:sync", 10*time.Second)
locked, _ := lock.Acquire()
if locked {
	_ = lock.Release()
}
```

### <a id="cache-trylock"></a>TryLock

TryLock acquires a short-lived lock key when this Cache instance has no active lifecycle for it.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, _ := c.TryLock("job:sync", 10*time.Second)
fmt.Println(locked) // true
if locked {
	_ = c.Unlock("job:sync")
}
```

### <a id="cache-unlock"></a>Unlock

Unlock closes this Cache instance's lifecycle and releases the key only while it still owns the backend lock.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, _ := c.TryLock("job:sync", 10*time.Second)
if locked {
	_ = c.Unlock("job:sync")
}
```

## Memoization

### <a id="newmemostore"></a>NewMemoStore

NewMemoStore decorates a backing store with a short-lived local read snapshot.
Create one per request or unit of work. It holds up to 256 hits or misses, each
for at most one second from the start of its backing read. Those reads can
outlive the value's backing TTL or miss a write made through another handle.
Use the backing store directly when you need backend TTL or immediate visibility.
Writes through the decorator invalidate the memoized keys. The caller retains
ownership of the backing store. NewMemoStore panics when the store is nil.

```go
ctx := context.Background()
base := cache.NewMemoryStore(ctx)
memo := cache.NewMemoStore(base)
c := cache.NewCache(memo)
fmt.Println(c.Driver()) // memory
```

## Observability

### <a id="observerfunc-oncacheop"></a>OnCacheOp

OnCacheOp implements Observer.

```go
obs := cache.ObserverFunc(func(ctx context.Context, event cache.CacheOpEvent) {
	fmt.Println(event.Operation, event.Key, event.Hit, event.Err == nil, event.Driver)
	_ = ctx
	_ = event.Duration
})
obs.OnCacheOp(context.Background(), cache.CacheOpEvent{Operation: "get", Key: "user:42", Hit: true, Duration: time.Millisecond, Driver: cachecore.DriverMemory})
```

### <a id="cache-withobserver"></a>WithObserver

WithObserver attaches an observer to receive operation events.
WithObserver mutates c for backward compatibility and must be called during construction,
before c is used concurrently. The observer must be safe for concurrent callbacks.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
c = c.WithObserver(cache.ObserverFunc(func(ctx context.Context, event cache.CacheOpEvent) {
	// See docs/production-guide.md for a real metrics recipe.
	fmt.Println(event.Operation, event.Driver, event.Hit, event.Err == nil)
	_ = ctx
	_ = event.Key
	_ = event.Duration
}))
_, _, _ = c.GetBytes("profile:42")
```

## Other

### <a id="lockhandle-withcontext"></a>LockHandle.WithContext

WithContext returns a derived handle that shares lock ownership state with l.

## Rate Limiting

### <a id="cache-ratelimit"></a>RateLimit

RateLimit increments a fixed-window counter and returns allowance metadata.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
res, err := c.RateLimit("rl:api:ip:1.2.3.4", 100, time.Minute)
fmt.Println(err == nil, res.Allowed, res.Count, res.Remaining, !res.ResetAt.IsZero())
// Output: true true 1 99 true
```

## Read Through

### <a id="cache-remember"></a>Cache.Remember

Remember is the ergonomic, typed remember helper using JSON encoding by default.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, err := c.Remember("profile:42", time.Minute, func() (Profile, error) {
	return Profile{Name: "Ada"}, nil
})
fmt.Println(err == nil, profile.Name) // true Ada
```

### <a id="cache-rememberstale"></a>Cache.RememberStale

RememberStale returns a typed value with stale fallback semantics using JSON encoding by default.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, usedStale, err := c.RememberStale("profile:42", time.Minute, 10*time.Minute, func() (Profile, error) {
	return Profile{Name: "Ada"}, nil
})
fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
```

### <a id="remember"></a>Remember

Remember is the ergonomic, typed remember helper using JSON encoding by default.
It remains available for compatibility; new code can use Cache.Remember.

### <a id="cache-rememberbytes"></a>RememberBytes

RememberBytes returns a cached value or computes and stores it on a miss.
Concurrent misses can each run the callback.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
data, err := c.RememberBytes("dashboard:summary", time.Minute, func() ([]byte, error) {
	return []byte("payload"), nil
})
fmt.Println(err == nil, string(data)) // true payload
```

### <a id="rememberstale"></a>RememberStale

RememberStale returns a typed value with stale fallback semantics using JSON encoding by default.
It remains available for compatibility; new code can use Cache.RememberStale.

### <a id="cache-rememberstalebytes"></a>RememberStaleBytes

RememberStaleBytes returns a fresh value when available, otherwise computes and caches it.
If computing fails and a stale value exists, it returns the stale value.
The returned bool is true when a stale fallback was used.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
body, usedStale, err := c.RememberStaleBytes("profile:42", time.Minute, 10*time.Minute, func() ([]byte, error) {
	return []byte(`{"name":"Ada"}`), nil
})
fmt.Println(err == nil, usedStale, len(body) > 0)
```

## Reads

### <a id="cache-batchgetbytes"></a>BatchGetBytes

BatchGetBytes returns all found values for the provided keys.
Missing keys are omitted from the returned map. Reads run individually.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.SetBytes("a", []byte("1"), time.Minute)
_ = c.SetBytes("b", []byte("2"), time.Minute)
values, err := c.BatchGetBytes("a", "b", "missing")
fmt.Println(err == nil, string(values["a"]), string(values["b"])) // true 1 2
```

### <a id="cache-get"></a>Cache.Get

Get returns a typed value for key using the default codec (JSON) when present.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.Set("profile:42", Profile{Name: "Ada"}, time.Minute)
_ = c.Set("settings:mode", "dark", time.Minute)
profile, ok, err := c.Get[Profile]("profile:42")
mode, ok2, err2 := c.Get[string]("settings:mode")
fmt.Println(err == nil, ok, profile.Name, err2 == nil, ok2, mode) // true true Ada true true dark
```

### <a id="cache-getjson"></a>Cache.GetJSON

GetJSON decodes a JSON value into T when key exists.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.SetJSON("profile:42", Profile{Name: "Ada"}, time.Minute)
profile, ok, err := c.GetJSON[Profile]("profile:42")
fmt.Println(err == nil, ok, profile.Name) // true true Ada
```

### <a id="get"></a>Get

Get returns a typed value for key using the default codec (JSON) when present.
It remains available for compatibility; new code can use Cache.Get.

### <a id="cache-getbytes"></a>GetBytes

GetBytes returns raw bytes for key when present.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
_ = c.SetBytes("user:42", []byte("Ada"), 0)
value, ok, _ := c.GetBytes("user:42")
fmt.Println(ok, string(value)) // true Ada
```

### <a id="getjson"></a>GetJSON

GetJSON decodes a JSON value into T when key exists.
It remains available for compatibility; new code can use Cache.GetJSON.

### <a id="cache-getstring"></a>GetString

GetString returns a UTF-8 string value for key when present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.SetString("user:42:name", "Ada", 0)
name, ok, _ := c.GetString("user:42:name")
fmt.Println(ok, name) // true Ada
```

### <a id="listpage"></a>ListPage

ListPage lists cache entries from an inspector-capable store.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
_ = c.SetString("profile:1", "Ada", time.Minute)
_ = c.SetString("profile:2", "Grace", time.Minute)
page, _ := cache.ListPage(ctx, c, cachecore.ListPageOptions{
	Query: "profile:",
	Limit: 10,
})
fmt.Println(len(page.Entries), page.Entries[0].Key) // 2 profile:1
```

## Refresh Ahead

### <a id="cache-refreshahead"></a>Cache.RefreshAhead

RefreshAhead returns a typed value and refreshes asynchronously when near expiry.

```go
type Summary struct { Text string `json:"text"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
s, err := c.RefreshAhead("dashboard:summary", time.Minute, 10*time.Second, func() (Summary, error) {
	return Summary{Text: "ok"}, nil
})
fmt.Println(err == nil, s.Text) // true ok
```

### <a id="cache-refreshaheadvaluewithcodec"></a>Cache.RefreshAheadValueWithCodec

RefreshAheadValueWithCodec allows custom encoding/decoding for typed refresh-ahead operations.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
codec := cache.ValueCodec[string]{
	Encode: func(value string) ([]byte, error) { return []byte(value), nil },
	Decode: func(body []byte) (string, error) { return string(body), nil },
}
value, err := c.RefreshAheadValueWithCodec("status", time.Minute, 10*time.Second, func() (string, error) {
	return "ready", nil
}, codec)
fmt.Println(err == nil, value)
// true ready
```

### <a id="refreshahead"></a>RefreshAhead

RefreshAhead returns a typed value and refreshes asynchronously when near expiry.
It remains available for compatibility; new code can use Cache.RefreshAhead.

### <a id="cache-refreshaheadbytes"></a>RefreshAheadBytes

RefreshAheadBytes returns cached value immediately and refreshes asynchronously when near expiry.
On miss, it computes and stores synchronously.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
body, err := c.RefreshAheadBytes("dashboard:summary", time.Minute, 10*time.Second, func() ([]byte, error) {
	return []byte("payload"), nil
})
fmt.Println(err == nil, len(body) > 0) // true true
```

### <a id="refreshaheadvaluewithcodec"></a>RefreshAheadValueWithCodec

RefreshAheadValueWithCodec allows custom encoding/decoding for typed refresh-ahead operations.
It remains available for compatibility; new code can use Cache.RefreshAheadValueWithCodec.

## Testing Helpers

### <a id="fake-assertcalled"></a>AssertCalled

AssertCalled verifies key was touched by op the expected number of times.

```go
f := cachefake.New()
c := f.Cache()
_ = c.SetString("settings:mode", "dark", 0)
t := &testing.T{}
f.AssertCalled(t, cachefake.OpSet, "settings:mode", 1)
```

### <a id="fake-assertnotcalled"></a>AssertNotCalled

AssertNotCalled ensures key was never touched by op.

```go
f := cachefake.New()
t := &testing.T{}
f.AssertNotCalled(t, cachefake.OpDelete, "settings:mode")
```

### <a id="fake-asserttotal"></a>AssertTotal

AssertTotal ensures the total call count for an op matches times.

```go
f := cachefake.New()
c := f.Cache()
_ = c.Delete("a")
_ = c.Delete("b")
t := &testing.T{}
f.AssertTotal(t, cachefake.OpDelete, 2)
```

### <a id="fake-cache"></a>Cache

Cache returns the cache facade to inject into code under test.

```go
f := cachefake.New()
c := f.Cache()
_, _, _ = c.GetBytes("settings:mode")
```

### <a id="fake-count"></a>Count

Count returns calls for op+key.

```go
f := cachefake.New()
c := f.Cache()
_ = c.SetString("settings:mode", "dark", 0)
n := f.Count(cachefake.OpSet, "settings:mode")
_ = n
```

### <a id="new"></a>New

New creates a Fake using an in-memory store.

```go
f := cachefake.New()
c := f.Cache()
_ = c.SetString("settings:mode", "dark", 0)
```

### <a id="fake-reset"></a>Reset

Reset clears recorded counts.

```go
f := cachefake.New()
_ = f.Cache().SetString("settings:mode", "dark", 0)
f.Reset()
```

### <a id="fake-total"></a>Total

Total returns total calls for an op across keys.

```go
f := cachefake.New()
c := f.Cache()
_ = c.Delete("a")
_ = c.Delete("b")
n := f.Total(cachefake.OpDelete)
_ = n
```

## Writes

### <a id="cache-add"></a>Add

Add writes value only when key is not already present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
created, _ := c.Add("boot:seeded", []byte("1"), time.Hour)
fmt.Println(created) // true
```

### <a id="cache-batchsetbytes"></a>BatchSetBytes

BatchSetBytes writes each key individually using a shared TTL.
A write failure can leave an arbitrary subset of keys updated.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
err := c.BatchSetBytes(map[string][]byte{
	"a": []byte("1"),
	"b": []byte("2"),
}, time.Minute)
fmt.Println(err == nil) // true
```

### <a id="cache-set"></a>Cache.Set

Set encodes value with the default codec (JSON) and writes it to key.

```go
type Settings struct { Enabled bool `json:"enabled"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
err := c.Set("settings:alerts", Settings{Enabled: true}, time.Minute)
err2 := c.Set("settings:mode", "dark", time.Minute)
fmt.Println(err == nil, err2 == nil) // true true
```

### <a id="cache-setjson"></a>Cache.SetJSON

SetJSON encodes value as JSON and writes it to key.

```go
type Settings struct { Enabled bool `json:"enabled"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
err := c.SetJSON("settings:alerts", Settings{Enabled: true}, time.Minute)
fmt.Println(err == nil) // true
```

### <a id="cache-decrement"></a>Decrement

Decrement decrements a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Decrement("rate:login:42", 1, time.Minute)
fmt.Println(val) // -1
```

### <a id="cache-increment"></a>Increment

Increment increments a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Increment("rate:login:42", 1, time.Minute)
fmt.Println(val) // 1
```

### <a id="set"></a>Set

Set encodes value with the default codec (JSON) and writes it to key.
It remains available for compatibility; new code can use Cache.Set.

### <a id="cache-setbytes"></a>SetBytes

SetBytes writes raw bytes to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.SetBytes("token", []byte("abc"), time.Minute) == nil) // true
```

### <a id="setjson"></a>SetJSON

SetJSON encodes value as JSON and writes it to key.
It remains available for compatibility; new code can use Cache.SetJSON.

### <a id="cache-setstring"></a>SetString

SetString writes a string value to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.SetString("user:42:name", "Ada", time.Minute) == nil) // true
```
<!-- api:embed:end -->

## Development

Use `make test` for root-module tests, `make vet` for static checks, `make generate` to refresh generated documentation, and `make test-integration` for the separate integration module. Pass a driver such as `make test-integration sqlitecache` to narrow the matrix. Integration tests may require local services. Driver, docs, examples, and integration directories are independent Go modules; test each changed module from its directory.

### README updates

README content is a mix of generated sections and manual sections.

- API reference (`<!-- api:embed:start --> ... <!-- api:embed:end -->`) is generated.
- Test badges are updated separately.
- Driver notes are maintained in the README.

### Update generated API docs

```bash
go run ./docs/readme/main.go
```

### Update test badges

Static counts (fast, watcher-friendly; counts top-level `Test*` funcs):

```bash
go run ./docs/readme/main.go
```

Executed counts (runs tests and counts real `go test -json` test/subtest starts):

```bash
CACHE_LOCAL_SIBLINGS=1 go run ./docs/readme/testcounts/main.go
```

The local-sibling mode uses `go.work` while a coordinated release is still
untagged. Omit it to verify the published module boundary after all sibling tags
are available.

### Watch mode

```bash
make docs-watch
```

Notes:

- The badge watcher runs real tests, so it is slower than API/example regeneration.
