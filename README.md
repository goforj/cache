<p align="center">
  <img src="./docs/images/logo.png?v=1" width="420" alt="cache logo">
</p>

<p align="center">
    cache gives your services one cache API with multiple backend options. Swap drivers without refactoring.
</p>

<p align="center">
    <a href="https://pkg.go.dev/github.com/goforj/cache"><img src="https://pkg.go.dev/badge/github.com/goforj/cache.svg" alt="Go Reference"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <a href="https://golang.org"><img src="https://img.shields.io/badge/go-1.24+-blue?logo=go" alt="Go version"></a>
    <img src="https://img.shields.io/github/v/tag/goforj/cache?label=version&sort=semver" alt="Latest tag">
    <a href="https://goreportcard.com/report/github.com/goforj/cache"><img src="https://goreportcard.com/badge/github.com/goforj/cache" alt="Go Report Card"></a>
    <a href="https://codecov.io/gh/goforj/cache"><img src="https://codecov.io/gh/goforj/cache/graph/badge.svg?token=B6ROULLKWU"/></a>
<!-- test-count:embed:start -->
    <img src="https://img.shields.io/badge/tests-264-brightgreen" alt="Tests">
<!-- test-count:embed:end -->
</p>

## What cache is
 
An explicit cache abstraction with a minimal Store interface and ergonomic Cache helpers. Drivers are chosen when you construct the store, so swapping backends is a dependency-injection change instead of a refactor.
 
## Drivers

|                                                                                             Driver / Backend | Mode | Shared | Durable | TTL | Counters | Notes |
|-------------------------------------------------------------------------------------------------------------:| :--- | :---: | :---: | :---: | :---: | :--- |
|                  <img src="https://img.shields.io/badge/null-9e9e9e?logo=probot&logoColor=white" alt="Null"> | No-op | - | - | - | - | Great for tests: cache calls are no-ops and never persist. |
|                   <img src="https://img.shields.io/badge/file-3f51b5?logo=files&logoColor=white" alt="File"> | Local filesystem | - | ✓ | ✓ | ✓ | Simple durability on a single host; point `WithFileDir` to writable disk. |
|              <img src="https://img.shields.io/badge/memory-5c5c5c?logo=cachet&logoColor=white" alt="Memory"> | In-process | - | - | ✓ | ✓ | Fastest; per-process only, best for single-node or short-lived data. |
|        <img src="https://img.shields.io/badge/memcached-0198c4?logo=buffer&logoColor=white" alt="Memcached"> | Networked | ✓ | - | ✓ | ✓ | Millisecond access; TTL resolution is 1s; use multiple nodes via `WithMemcachedAddresses`. |
|              <img src="https://img.shields.io/badge/redis-%23DC382D?logo=redis&logoColor=white" alt="Redis"> | Networked | ✓ | - | ✓ | ✓ | Full feature set; supports prefixing and counters with per-key TTL refresh. |
|                <img src="https://img.shields.io/badge/nats-27AAE1?logo=natsdotio&logoColor=white" alt="NATS"> | Networked | ✓ | - | ✓ | ✓ | JetStream KeyValue-backed driver; inject an existing bucket via `WithNATSKeyValue`. |
| <img src="https://img.shields.io/badge/dynamodb-4053D6?logo=amazon-dynamodb&logoColor=white" alt="DynamoDB"> | Networked | ✓ | ✓ | ✓ | ✓ | Backed by DynamoDB (supports localstack/dynamodb-local). |
|    <img src="https://img.shields.io/badge/sql-336791?logo=postgresql&logoColor=white" alt="SQL"> | Networked / local | ✓ | ✓ | ✓ | ✓ | Postgres / MySQL / SQLite via database/sql; table schema managed automatically. |

## Installation

```bash
go get github.com/goforj/cache
```

## Quick Start

```go
import (
    "context"
    "time"

    "github.com/goforj/cache"
    "github.com/redis/go-redis/v9"
)

func main() {
    ctx := context.Background()

    store := cache.NewMemoryStore(ctx, cache.WithDefaultTTL(5*time.Minute))
    c := cache.NewCache(store)

    // Remember pattern.
    type Profile struct { Name string `json:"name"` }

    profile, err := cache.Remember[Profile](c, "user:42:profile", time.Minute, func() (Profile, error) {
        return Profile{Name: "Ada"}, nil
    })
    fmt.Println(profile.Name) // Ada

    // Switch to Redis (dependency injection, no code changes below).
    client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
    store = cache.NewRedisStore(ctx, client, cache.WithPrefix("app"), cache.WithDefaultTTL(5*time.Minute))
    c = cache.NewCache(store)
}
```

## StoreConfig

StoreConfig keeps configuration explicit:

- Driver: explicit backend (null, file, memory, memcached, redis, nats, dynamodb, sql)
- DefaultTTL: fallback TTL when a call provides ttl <= 0
- MemoryCleanupInterval: sweep interval for memory driver
- Prefix: key prefix for shared backends
- RedisClient / NATSKeyValue / MemcachedAddresses / DynamoClient / SQLDriverName+DSN: driver-specific inputs
- Compression / MaxValueBytes / EncryptionKey: shaping and security controls

## Behavior Semantics

For precise runtime semantics, see [Behavior Semantics](docs/behavior-semantics.md):

- TTL/default-TTL matrix by operation/helper
- stale and refresh-ahead behavior and edge cases
- lock and rate-limit guarantees (process-local vs distributed scope)

## Cache helpers

Cache wraps a Store with ergonomic helpers (context-free by default, `*Ctx` variants when you need a context). Store stays context-first because drivers perform I/O and should honor deadlines/cancellation; Cache gives you the convenience layer on top:

```go
type CacheAPI interface {
	Get(key string) ([]byte, bool, error)
	GetString(key string) (string, bool, error)
	Set(key string, value []byte, ttl time.Duration) error
	SetString(key string, value string, ttl time.Duration) error
	Add(key string, value []byte, ttl time.Duration) (bool, error)
	Increment(key string, delta int64, ttl time.Duration) (int64, error)
	Decrement(key string, delta int64, ttl time.Duration) (int64, error)
	Pull(key string) ([]byte, bool, error)
	Delete(key string) error
	DeleteMany(keys ...string) error
	Flush() error
	RememberBytes(key string, ttl time.Duration, fn func() ([]byte, error)) ([]byte, error)
	RememberString(key string, ttl time.Duration, fn func() (string, error)) (string, error)
}

// JSON helpers (free functions)
func Remember[T any](cache *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error)
func RememberJSON[T any](cache *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error)
func GetJSON[T any](cache *Cache, key string) (T, bool, error)
func SetJSON[T any](cache *Cache, key string, value T, ttl time.Duration) error
func RememberValueWithCodec[T any](ctx context.Context, cache *Cache, key string, ttl time.Duration, fn func() (T, error), codec ValueCodec[T]) (T, error)
```
ctx-aware twins: GetCtx, SetCtx, RememberCtx (bytes), RememberStringCtx, RememberJSONCtx, RememberValueWithCodec, etc.

To observe cache operations (hits, misses, errors, latency), attach an Observer:

```go
type Observer interface {
	OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cache.Driver)
}

obs := &myObserver{}
c := cache.NewCache(store).WithObserver(obs)
```

Example:

```go
settings, err := cache.RememberJSON[Settings](c, "settings:alerts", 10*time.Minute, func() (Settings, error) {
    return fetchSettings(context.Background())
})
```

## Memoized reads

Wrap any store with `NewMemoStore` to memoize reads within the process; cache is invalidated automatically on write paths.

```go
memoStore := cache.NewMemoStore(store)
memoRepo := cache.NewCache(memoStore)
```

**Staleness note:** memoization is per-process only. Writes that happen in *other* processes (or outside your app) will not invalidate this memo cache. Use it when local staleness is acceptable, or scope it narrowly (e.g., per-request) if multiple writers exist.

## Testing

Unit tests cover the public helpers. Integration tests use `testcontainers-go` to spin up Redis:

```bash
go test -tags=integration ./...
```

Use `INTEGRATION_DRIVER=redis` (comma-separated; defaults to `all`) to select which drivers start containers and run the contract suite.

## Benchmarks

```go
go test ./docs/bench -tags benchrender
```

Note: NATS numbers can look slower than Redis/memory because the NATS driver preserves per-operation TTL semantics by storing per-key expiry metadata (envelope encode/decode) and may do extra compare/update steps for some operations.

<!-- bench:embed:start -->

### Latency (ns/op)

![Cache benchmark latency chart](docs/bench/benchmarks_ns.svg)

### Iterations (N)

![Cache benchmark iteration chart](docs/bench/benchmarks_ops.svg)

### Allocated Bytes (B/op)

![Cache benchmark bytes chart](docs/bench/benchmarks_bytes.svg)

### Allocations (allocs/op)

![Cache benchmark allocs chart](docs/bench/benchmarks_allocs.svg)

<!-- bench:embed:end -->

## Testing helpers

For unit tests that shouldn’t hit real infrastructure, use the in-memory fake with call assertions:

```go
f := cachefake.New()
c := f.Cache()

// exercise code under test
_ = c.SetString("settings:mode", "dark", 0)
_, _, _ = c.Get("settings:mode")
_ = c.Delete("settings:mode")

f.AssertCalled(t, cachefake.OpSet, "settings:mode", 1)
f.AssertCalled(t, cachefake.OpGet, "settings:mode", 1)
f.AssertCalled(t, cachefake.OpDelete, "settings:mode", 1)
f.AssertNotCalled(t, cachefake.OpAdd, "settings:mode")
```

## API reference

The API section below is autogenerated; do not edit between the markers.

<!-- api:embed:start -->

## API Index

| Group | Functions |
|------:|:-----------|
| **Constructors** | [NewDynamoStore](#newdynamostore) [NewFileStore](#newfilestore) [NewMemcachedStore](#newmemcachedstore) [NewMemoryStore](#newmemorystore) [NewNATSStore](#newnatsstore) [NewNullStore](#newnullstore) [NewRedisStore](#newredisstore) [NewSQLStore](#newsqlstore) [NewStore](#newstore) [NewStoreWith](#newstorewith) |
| **Core** | [Driver](#driver) [NewCache](#newcache) [NewCacheWithTTL](#newcachewithttl) [Store](#store) |
| **Invalidation** | [Delete](#delete) [DeleteCtx](#deletectx) [DeleteMany](#deletemany) [DeleteManyCtx](#deletemanyctx) [Flush](#flush) [FlushCtx](#flushctx) [Pull](#pull) [PullCtx](#pullctx) |
| **Locking** | [Lock](#lock) [LockCtx](#lockctx) [TryLock](#trylock) [TryLockCtx](#trylockctx) [Unlock](#unlock) [UnlockCtx](#unlockctx) |
| **Memoization** | [NewMemoStore](#newmemostore) |
| **Observability** | [WithObserver](#withobserver) |
| **Options** | [WithCompression](#withcompression) [WithDefaultTTL](#withdefaultttl) [WithDynamoClient](#withdynamoclient) [WithDynamoEndpoint](#withdynamoendpoint) [WithDynamoRegion](#withdynamoregion) [WithDynamoTable](#withdynamotable) [WithEncryptionKey](#withencryptionkey) [WithFileDir](#withfiledir) [WithMaxValueBytes](#withmaxvaluebytes) [WithMemcachedAddresses](#withmemcachedaddresses) [WithMemoryCleanupInterval](#withmemorycleanupinterval) [WithNATSBucketTTL](#withnatsbucketttl) [WithNATSKeyValue](#withnatskeyvalue) [WithPrefix](#withprefix) [WithRedisClient](#withredisclient) [WithSQL](#withsql) |
| **Rate Limiting** | [RateLimit](#ratelimit) [RateLimitCtx](#ratelimitctx) [RateLimitWithRemaining](#ratelimitwithremaining) [RateLimitWithRemainingCtx](#ratelimitwithremainingctx) |
| **Read Through** | [Remember](#remember) [RememberBytes](#rememberbytes) [RememberCtx](#rememberctx) [RememberJSON](#rememberjson) [RememberJSONCtx](#rememberjsonctx) [RememberStale](#rememberstale) [RememberStaleBytes](#rememberstalebytes) [RememberStaleBytesCtx](#rememberstalebytesctx) [RememberStaleCtx](#rememberstalectx) [RememberStaleValueWithCodec](#rememberstalevaluewithcodec) [RememberString](#rememberstring) [RememberStringCtx](#rememberstringctx) [RememberValue](#remembervalue) [RememberValueWithCodec](#remembervaluewithcodec) |
| **Reads** | [BatchGet](#batchget) [BatchGetCtx](#batchgetctx) [Get](#get) [GetCtx](#getctx) [GetJSON](#getjson) [GetJSONCtx](#getjsonctx) [GetString](#getstring) [GetStringCtx](#getstringctx) |
| **Refresh Ahead** | [RefreshAhead](#refreshahead) [RefreshAheadCtx](#refreshaheadctx) [RefreshAheadValueWithCodec](#refreshaheadvaluewithcodec) |
| **Writes** | [Add](#add) [AddCtx](#addctx) [BatchSet](#batchset) [BatchSetCtx](#batchsetctx) [Decrement](#decrement) [DecrementCtx](#decrementctx) [Increment](#increment) [IncrementCtx](#incrementctx) [Set](#set) [SetCtx](#setctx) [SetJSON](#setjson) [SetJSONCtx](#setjsonctx) [SetString](#setstring) [SetStringCtx](#setstringctx) |


## Constructors

### <a id="newdynamostore"></a>NewDynamoStore

NewDynamoStore is a convenience for a DynamoDB-backed store.

```go
ctx := context.Background()
store := cache.NewDynamoStore(ctx, cache.StoreConfig{DynamoEndpoint: "http://localhost:8000"})
fmt.Println(store.Driver()) // dynamodb
```

### <a id="newfilestore"></a>NewFileStore

NewFileStore is a convenience for a filesystem-backed store.

```go
ctx := context.Background()
store := cache.NewFileStore(ctx, "/tmp/my-cache")
fmt.Println(store.Driver()) // file
```

### <a id="newmemcachedstore"></a>NewMemcachedStore

NewMemcachedStore is a convenience for a memcached-backed store.

```go
ctx := context.Background()
store := cache.NewMemcachedStore(ctx, []string{"127.0.0.1:11211"})
fmt.Println(store.Driver()) // memcached
```

### <a id="newmemorystore"></a>NewMemoryStore

NewMemoryStore is a convenience for an in-process store with optional overrides.

```go
ctx := context.Background()
store := cache.NewMemoryStore(ctx)
fmt.Println(store.Driver()) // memory
```

### <a id="newnatsstore"></a>NewNATSStore

NewNATSStore is a convenience for a NATS JetStream KeyValue-backed store.

```go
ctx := context.Background()
var kv cache.NATSKeyValue // provided by your NATS setup
store := cache.NewNATSStore(ctx, kv, cache.WithPrefix("app"))
fmt.Println(store.Driver()) // nats
```

### <a id="newnullstore"></a>NewNullStore

NewNullStore is a no-op store useful for tests where caching should be disabled.

```go
ctx := context.Background()
store := cache.NewNullStore(ctx)
fmt.Println(store.Driver()) // null
```

### <a id="newredisstore"></a>NewRedisStore

NewRedisStore is a convenience for a redis-backed store. Redis client is required.

```go
ctx := context.Background()
redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store := cache.NewRedisStore(ctx, redisClient, cache.WithPrefix("app"))
fmt.Println(store.Driver()) // redis
```

### <a id="newsqlstore"></a>NewSQLStore

NewSQLStore builds a SQL-backed store (postgres, mysql, sqlite).

_Example: sqlite helper_

```go
ctx := context.Background()
store := cache.NewSQLStore(ctx, "sqlite", "file:cache.db?cache=shared&mode=rwc", "cache_entries")
fmt.Println(store.Driver()) // sql
```

_Example: postgres helper_

```go
dsnPg := "postgres://user:pass@localhost:5432/app?sslmode=disable"
storePg := cache.NewSQLStore(ctx, "pgx", dsnPg, "cache_entries")
fmt.Println(storePg.Driver()) // sql
```

_Example: mysql helper_

```go
dsnMy := "user:pass@tcp(localhost:3306)/app?parseTime=true"
storeMy := cache.NewSQLStore(ctx, "mysql", dsnMy, "cache_entries")
fmt.Println(storeMy.Driver()) // sql
```

### <a id="newstore"></a>NewStore

NewStore returns a concrete store for the requested driver.
Caller is responsible for providing any driver-specific dependencies.

```go
ctx := context.Background()
store := cache.NewStore(ctx, cache.StoreConfig{
	Driver: cache.DriverMemory,
})
fmt.Println(store.Driver()) // memory
```

### <a id="newstorewith"></a>NewStoreWith

NewStoreWith builds a store using a driver and a set of functional options.
Required data (e.g., Redis client) must be provided via options when needed.

_Example: memory store (options)_

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverMemory)
fmt.Println(store.Driver()) // memory
```

_Example: redis store (options)_

```go
redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store = cache.NewStoreWith(ctx, cache.DriverRedis,
	cache.WithRedisClient(redisClient),
	cache.WithPrefix("app"),
	cache.WithDefaultTTL(5*time.Minute),
)
fmt.Println(store.Driver()) // redis
```

## Core

### <a id="driver"></a>Driver

Driver reports the underlying store driver.

### <a id="newcache"></a>NewCache

NewCache creates a cache facade bound to a concrete store.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
fmt.Println(c.Driver()) // memory
```

### <a id="newcachewithttl"></a>NewCacheWithTTL

NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCacheWithTTL(s, 2*time.Minute)
fmt.Println(c.Driver(), c != nil) // memory true
```

### <a id="store"></a>Store

Store returns the underlying store implementation.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Store().Driver()) // memory
```

## Invalidation

### <a id="delete"></a>Delete

Delete removes a single key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Delete("a") == nil) // true
```

### <a id="deletectx"></a>DeleteCtx

DeleteCtx is the context-aware variant of Delete.

### <a id="deletemany"></a>DeleteMany

DeleteMany removes multiple keys.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.DeleteMany("a", "b") == nil) // true
```

### <a id="deletemanyctx"></a>DeleteManyCtx

DeleteManyCtx is the context-aware variant of DeleteMany.

### <a id="flush"></a>Flush

Flush clears all keys for this store scope.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Flush() == nil) // true
```

### <a id="flushctx"></a>FlushCtx

FlushCtx is the context-aware variant of Flush.

### <a id="pull"></a>Pull

Pull returns value and removes it from cache.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
body, ok, _ := c.Pull("reset:token:42")
fmt.Println(ok, string(body)) // true abc
```

### <a id="pullctx"></a>PullCtx

PullCtx is the context-aware variant of Pull.

## Locking

### <a id="lock"></a>Lock

Lock waits until the lock is acquired or timeout elapses.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, err := c.Lock("job:sync", 10*time.Second, time.Second)
fmt.Println(err == nil, locked) // true true
```

### <a id="lockctx"></a>LockCtx

LockCtx retries lock acquisition until success or context cancellation.

### <a id="trylock"></a>TryLock

TryLock acquires a short-lived lock key when not already held.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, _ := c.TryLock("job:sync", 10*time.Second)
fmt.Println(locked) // true
```

### <a id="trylockctx"></a>TryLockCtx

TryLockCtx is the context-aware variant of TryLock.

### <a id="unlock"></a>Unlock

Unlock releases a previously acquired lock key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, _ := c.TryLock("job:sync", 10*time.Second)
if locked {
}
```

### <a id="unlockctx"></a>UnlockCtx

UnlockCtx is the context-aware variant of Unlock.

## Memoization

### <a id="newmemostore"></a>NewMemoStore

NewMemoStore decorates store with per-process read memoization.

Behavior:
- First Get hits the backing store, clones the value, and memoizes it in-process.
- Subsequent Get for the same key returns the memoized clone (no backend call).
- Any write/delete/flush invalidates the memo entry so local reads stay in sync
with changes made through this process.
- Memo data is per-process only; other processes or external writers will not
invalidate it. Use only when that staleness window is acceptable.

```go
ctx := context.Background()
base := cache.NewMemoryStore(ctx)
memo := cache.NewMemoStore(base)
c := cache.NewCache(memo)
fmt.Println(c.Driver()) // memory
```

## Observability

### <a id="withobserver"></a>WithObserver

WithObserver attaches an observer to receive operation events.

## Options

### <a id="withcompression"></a>WithCompression

WithCompression enables value compression using the chosen codec.

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithCompression(cache.CompressionGzip))
fmt.Println(store.Driver()) // memory
```

### <a id="withdefaultttl"></a>WithDefaultTTL

WithDefaultTTL overrides the fallback TTL used when ttl <= 0.

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithDefaultTTL(30*time.Second))
fmt.Println(store.Driver()) // memory
```

### <a id="withdynamoclient"></a>WithDynamoClient

WithDynamoClient injects a pre-built DynamoDB client.

```go
ctx := context.Background()
var client cache.DynamoAPI // assume already configured
store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoClient(client))
fmt.Println(store.Driver()) // dynamodb
```

### <a id="withdynamoendpoint"></a>WithDynamoEndpoint

WithDynamoEndpoint sets the DynamoDB endpoint (useful for local testing).

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoEndpoint("http://localhost:8000"))
fmt.Println(store.Driver()) // dynamodb
```

### <a id="withdynamoregion"></a>WithDynamoRegion

WithDynamoRegion sets the DynamoDB region for requests.

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoRegion("us-west-2"))
fmt.Println(store.Driver()) // dynamodb
```

### <a id="withdynamotable"></a>WithDynamoTable

WithDynamoTable sets the table used by the DynamoDB driver.

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoTable("cache_entries"))
fmt.Println(store.Driver()) // dynamodb
```

### <a id="withencryptionkey"></a>WithEncryptionKey

WithEncryptionKey enables at-rest encryption using the provided AES key (16/24/32 bytes).

```go
ctx := context.Background()
key := []byte("01234567890123456789012345678901")
store := cache.NewStoreWith(ctx, cache.DriverFile, cache.WithEncryptionKey(key))
fmt.Println(store.Driver()) // file
```

### <a id="withfiledir"></a>WithFileDir

WithFileDir sets the directory used by the file driver.

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverFile, cache.WithFileDir("/tmp/cache"))
fmt.Println(store.Driver()) // file
```

### <a id="withmaxvaluebytes"></a>WithMaxValueBytes

WithMaxValueBytes sets a per-entry size limit (0 disables the check).

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithMaxValueBytes(1024))
fmt.Println(store.Driver()) // memory
```

### <a id="withmemcachedaddresses"></a>WithMemcachedAddresses

WithMemcachedAddresses sets memcached server addresses (host:port).

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverMemcached, cache.WithMemcachedAddresses("127.0.0.1:11211"))
fmt.Println(store.Driver()) // memcached
```

### <a id="withmemorycleanupinterval"></a>WithMemoryCleanupInterval

WithMemoryCleanupInterval overrides the sweep interval for the memory driver.

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithMemoryCleanupInterval(5*time.Minute))
fmt.Println(store.Driver()) // memory
```

### <a id="withnatsbucketttl"></a>WithNATSBucketTTL

WithNATSBucketTTL toggles bucket-level TTL mode for DriverNATS.
When enabled, values are stored as raw bytes and per-operation ttl values are ignored.

```go
ctx := context.Background()
var kv cache.NATSKeyValue // provided by your NATS setup
store := cache.NewStoreWith(ctx, cache.DriverNATS,
	cache.WithNATSKeyValue(kv),
	cache.WithNATSBucketTTL(true),
)
fmt.Println(store.Driver()) // nats
```

### <a id="withnatskeyvalue"></a>WithNATSKeyValue

WithNATSKeyValue sets the NATS JetStream KeyValue bucket; required when using DriverNATS.

```go
ctx := context.Background()
var kv cache.NATSKeyValue // provided by your NATS setup
store := cache.NewStoreWith(ctx, cache.DriverNATS, cache.WithNATSKeyValue(kv))
fmt.Println(store.Driver()) // nats
```

### <a id="withprefix"></a>WithPrefix

WithPrefix sets the key prefix for shared backends (e.g., redis).

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverRedis, cache.WithPrefix("svc"))
fmt.Println(store.Driver()) // redis
```

### <a id="withredisclient"></a>WithRedisClient

WithRedisClient sets the redis client; required when using DriverRedis.

```go
ctx := context.Background()
rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
store := cache.NewStoreWith(ctx, cache.DriverRedis, cache.WithRedisClient(rdb))
fmt.Println(store.Driver()) // redis
```

### <a id="withsql"></a>WithSQL

WithSQL configures the SQL driver (driver name + DSN + optional table).

```go
ctx := context.Background()
store := cache.NewStoreWith(ctx, cache.DriverSQL,
	cache.WithSQL("sqlite", "file::memory:?cache=shared", "cache_entries"),
	cache.WithPrefix("svc"),
)
fmt.Println(store.Driver()) // sql
```

## Rate Limiting

### <a id="ratelimit"></a>RateLimit

RateLimit increments key in a fixed window and reports whether requests are allowed.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
allowed, count, _ := c.RateLimit("rl:login:user:42", 5, time.Minute)
fmt.Println(allowed, count >= 1) // true true
```

### <a id="ratelimitctx"></a>RateLimitCtx

RateLimitCtx is the context-aware variant of RateLimit.

### <a id="ratelimitwithremaining"></a>RateLimitWithRemaining

RateLimitWithRemaining increments a fixed-window counter and returns allowance metadata.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
allowed, count, remaining, resetAt, err := c.RateLimitWithRemaining("rl:api:ip:1.2.3.4", 100, time.Minute)
fmt.Println(err == nil, allowed, count >= 1, remaining >= 0, resetAt.After(time.Now()))
```

### <a id="ratelimitwithremainingctx"></a>RateLimitWithRemainingCtx

RateLimitWithRemainingCtx is the context-aware variant of RateLimitWithRemaining.

## Read Through

### <a id="remember"></a>Remember

Remember is the ergonomic, typed remember helper using JSON encoding by default.

### <a id="rememberbytes"></a>RememberBytes

RememberBytes returns key value or computes/stores it when missing.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
data, err := c.RememberBytes("dashboard:summary", time.Minute, func() ([]byte, error) {
	return []byte("payload"), nil
})
fmt.Println(err == nil, string(data)) // true payload
```

### <a id="rememberctx"></a>RememberCtx

RememberCtx is the context-aware variant of RememberBytes.

### <a id="rememberjson"></a>RememberJSON

RememberJSON returns key value or computes/stores JSON when missing.

```go
type Settings struct { Enabled bool `json:"enabled"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
settings, err := cache.RememberJSON[Settings](c, "settings:alerts", time.Minute, func() (Settings, error) {
	return Settings{Enabled: true}, nil
})
fmt.Println(err == nil, settings.Enabled) // true true
```

### <a id="rememberjsonctx"></a>RememberJSONCtx

RememberJSONCtx is the context-aware variant of RememberJSON.

### <a id="rememberstale"></a>RememberStale

RememberStale returns a typed value with stale fallback semantics using JSON encoding by default.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, usedStale, err := cache.RememberStale[Profile](c, "profile:42", time.Minute, 10*time.Minute, func() (Profile, error) {
	return Profile{Name: "Ada"}, nil
})
fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
```

### <a id="rememberstalebytes"></a>RememberStaleBytes

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

### <a id="rememberstalebytesctx"></a>RememberStaleBytesCtx

RememberStaleBytesCtx is the context-aware variant of RememberStaleBytes.

### <a id="rememberstalectx"></a>RememberStaleCtx

RememberStaleCtx returns a typed value with stale fallback semantics using JSON encoding by default.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, usedStale, err := cache.RememberStaleCtx[Profile](ctx, c, "profile:42", time.Minute, 10*time.Minute, func(ctx context.Context) (Profile, error) {
	return Profile{Name: "Ada"}, nil
})
fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
```

### <a id="rememberstalevaluewithcodec"></a>RememberStaleValueWithCodec

RememberStaleValueWithCodec allows custom encoding/decoding for typed stale remember operations.

```go
type Profile struct { Name string }
codec := cache.ValueCodec[Profile]{
	Encode: func(v Profile) ([]byte, error) { return []byte(v.Name), nil },
	Decode: func(b []byte) (Profile, error) { return Profile{Name: string(b)}, nil },
}
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, usedStale, err := cache.RememberStaleValueWithCodec(ctx, c, "profile:42", time.Minute, 10*time.Minute, func() (Profile, error) {
	return Profile{Name: "Ada"}, nil
}, codec)
fmt.Println(err == nil, usedStale, profile.Name) // true false Ada
```

### <a id="rememberstring"></a>RememberString

RememberString returns key value or computes/stores it when missing.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, err := c.RememberString("settings:mode", time.Minute, func() (string, error) {
	return "on", nil
})
fmt.Println(err == nil, val) // true on
```

### <a id="rememberstringctx"></a>RememberStringCtx

RememberStringCtx is the context-aware variant of RememberString.

### <a id="remembervalue"></a>RememberValue

RememberValue returns a typed value or computes/stores it when missing using JSON encoding by default.

### <a id="remembervaluewithcodec"></a>RememberValueWithCodec

RememberValueWithCodec allows custom encoding/decoding for typed remember operations.

## Reads

### <a id="batchget"></a>BatchGet

BatchGet returns all found values for the provided keys.
Missing keys are omitted from the returned map.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
values, err := c.BatchGet("a", "b", "missing")
fmt.Println(err == nil, string(values["a"]), string(values["b"])) // true 1 2
```

### <a id="batchgetctx"></a>BatchGetCtx

BatchGetCtx is the context-aware variant of BatchGet.

### <a id="get"></a>Get

Get returns raw bytes for key when present.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
value, ok, _ := c.Get("user:42")
fmt.Println(ok, string(value)) // true Ada
```

### <a id="getctx"></a>GetCtx

GetCtx is the context-aware variant of Get.

### <a id="getjson"></a>GetJSON

GetJSON decodes a JSON value into T when key exists, using background context.

### <a id="getjsonctx"></a>GetJSONCtx

GetJSONCtx is the context-aware variant of GetJSON.

### <a id="getstring"></a>GetString

GetString returns a UTF-8 string value for key when present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
name, ok, _ := c.GetString("user:42:name")
fmt.Println(ok, name) // true Ada
```

### <a id="getstringctx"></a>GetStringCtx

GetStringCtx is the context-aware variant of GetString.

## Refresh Ahead

### <a id="refreshahead"></a>RefreshAhead

RefreshAhead returns cached value immediately and refreshes asynchronously when near expiry.
On miss, it computes and stores synchronously.

_Example: refresh ahead_

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
body, err := c.RefreshAhead("dashboard:summary", time.Minute, 10*time.Second, func() ([]byte, error) {
	return []byte("payload"), nil
})
fmt.Println(err == nil, len(body) > 0) // true true
```

_Example: refresh ahead typed_

```go
type Summary struct { Text string `json:"text"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
s, err := cache.RefreshAhead[Summary](c, "dashboard:summary", time.Minute, 10*time.Second, func() (Summary, error) {
	return Summary{Text: "ok"}, nil
})
fmt.Println(err == nil, s.Text) // true ok
```

### <a id="refreshaheadctx"></a>RefreshAheadCtx

RefreshAheadCtx is the context-aware variant of RefreshAhead.

### <a id="refreshaheadvaluewithcodec"></a>RefreshAheadValueWithCodec

RefreshAheadValueWithCodec allows custom encoding/decoding for typed refresh-ahead operations.

## Writes

### <a id="add"></a>Add

Add writes value only when key is not already present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
created, _ := c.Add("boot:seeded", []byte("1"), time.Hour)
fmt.Println(created) // true
```

### <a id="addctx"></a>AddCtx

AddCtx is the context-aware variant of Add.

### <a id="batchset"></a>BatchSet

BatchSet writes many key/value pairs using a shared ttl.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
err := c.BatchSet(map[string][]byte{
	"a": []byte("1"),
	"b": []byte("2"),
}, time.Minute)
fmt.Println(err == nil) // true
```

### <a id="batchsetctx"></a>BatchSetCtx

BatchSetCtx is the context-aware variant of BatchSet.

### <a id="decrement"></a>Decrement

Decrement decrements a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Decrement("rate:login:42", 1, time.Minute)
fmt.Println(val) // -1
```

### <a id="decrementctx"></a>DecrementCtx

DecrementCtx is the context-aware variant of Decrement.

### <a id="increment"></a>Increment

Increment increments a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Increment("rate:login:42", 1, time.Minute)
fmt.Println(val) // 1
```

### <a id="incrementctx"></a>IncrementCtx

IncrementCtx is the context-aware variant of Increment.

### <a id="set"></a>Set

Set writes raw bytes to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Set("token", []byte("abc"), time.Minute) == nil) // true
```

### <a id="setctx"></a>SetCtx

SetCtx is the context-aware variant of Set.

### <a id="setjson"></a>SetJSON

SetJSON encodes value as JSON and writes it to key using background context.

### <a id="setjsonctx"></a>SetJSONCtx

SetJSONCtx is the context-aware variant of SetJSON.

### <a id="setstring"></a>SetString

SetString writes a string value to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.SetString("user:42:name", "Ada", time.Minute) == nil) // true
```

### <a id="setstringctx"></a>SetStringCtx

SetStringCtx is the context-aware variant of SetString.
<!-- api:embed:end -->
