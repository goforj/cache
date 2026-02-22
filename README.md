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
    <img src="https://img.shields.io/badge/unit_tests-353-brightgreen" alt="Unit tests (executed count)">
    <img src="https://img.shields.io/badge/integration_tests-435-blue" alt="Integration tests (executed count)">
<!-- test-count:embed:end -->
</p>

## What cache is
 
An explicit cache abstraction with a minimal Store interface and ergonomic Cache helpers. Drivers are chosen when you construct the store, so swapping backends is a dependency-injection change instead of a refactor.
 
## Drivers

|                                                                                             Driver / Backend | Mode | Shared | Durable | TTL | Counters | Locks | RateLimit | Prefix | Batch | Shaping | Notes |
|-------------------------------------------------------------------------------------------------------------:| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
|                  <img src="https://img.shields.io/badge/null-9e9e9e?logo=probot&logoColor=white" alt="Null"> | No-op | - | - | - | - | No-op | No-op | ✓ | ✓ | ✓ | Great for tests: cache calls are no-ops and never persist. |
|                   <img src="https://img.shields.io/badge/file-3f51b5?logo=files&logoColor=white" alt="File"> | Local filesystem | - | ✓ | ✓ | ✓ | Local | Local | - | ✓ | ✓ | Simple durability on a single host; point `WithFileDir` to writable disk. |
|              <img src="https://img.shields.io/badge/memory-5c5c5c?logo=cachet&logoColor=white" alt="Memory"> | In-process | - | - | ✓ | ✓ | Local | Local | - | ✓ | ✓ | Fastest; per-process only, best for single-node or short-lived data. |
|        <img src="https://img.shields.io/badge/memcached-0198c4?logo=buffer&logoColor=white" alt="Memcached"> | Networked | ✓ | - | ✓ | ✓ | Shared | Shared | ✓ | ✓ | ✓ | TTL resolution is 1s; use multiple nodes via `WithMemcachedAddresses`. |
|              <img src="https://img.shields.io/badge/redis-%23DC382D?logo=redis&logoColor=white" alt="Redis"> | Networked | ✓ | - | ✓ | ✓ | Shared | Shared | ✓ | ✓ | ✓ | Full feature set; counters refresh TTL (Redis counter TTL granularity currently 1s). |
|                <img src="https://img.shields.io/badge/nats-27AAE1?logo=natsdotio&logoColor=white" alt="NATS"> | Networked | ✓ | - | ✓ | ✓ | Shared | Shared | ✓ | ✓ | ✓ | JetStream KV-backed driver; inject an existing bucket via `WithNATSKeyValue`. |
| <img src="https://img.shields.io/badge/dynamodb-4053D6?logo=amazon-dynamodb&logoColor=white" alt="DynamoDB"> | Networked | ✓ | ✓ | ✓ | ✓ | Shared | Shared | ✓ | ✓ | ✓ | Backed by DynamoDB (supports localstack/dynamodb-local). |
|    <img src="https://img.shields.io/badge/sql-336791?logo=postgresql&logoColor=white" alt="SQL"> | Networked / local | ✓ | ✓ | ✓ | ✓ | Shared | Shared | ✓ | ✓ | ✓ | Postgres / MySQL / SQLite via database/sql; table schema managed automatically. |

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

## Production Guidance

For deployment defaults and operational patterns, see [Production Guide](docs/production-guide.md):

- recommended defaults and tuning
- key naming/versioning conventions
- TTL jitter and miss-storm mitigation
- observability instrumentation patterns

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

Note: DynamoDB is intentionally omitted from these local charts because emulator-based numbers are not representative of real AWS latency.

NATS variants in these charts:

- `nats`: per-key TTL semantics using a binary envelope (`magic/expiresAt/value`). This preserves per-key expiry parity with other drivers, with modest metadata overhead.
- `nats_bucket_ttl`: bucket-level TTL mode (`WithNATSBucketTTL(true)`), raw value path; faster but different expiry semantics.

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
| **Core** | [Driver](#cache-driver) [NewCache](#newcache) [NewCacheWithTTL](#newcachewithttl) [Store](#cache-store) |
| **Invalidation** | [Delete](#cache-delete) [DeleteCtx](#cache-deletectx) [DeleteMany](#cache-deletemany) [DeleteManyCtx](#cache-deletemanyctx) [Flush](#cache-flush) [FlushCtx](#cache-flushctx) [Pull](#pull) [PullBytes](#cache-pullbytes) [PullBytesCtx](#cache-pullbytesctx) [PullCtx](#pullctx) |
| **Locking** | [Acquire](#lockhandle-acquire) [AcquireCtx](#lockhandle-acquirectx) [Block](#lockhandle-block) [BlockCtx](#lockhandle-blockctx) [Lock](#cache-lock) [LockCtx](#cache-lockctx) [LockHandle.Get](#lockhandle-get) [LockHandle.GetCtx](#lockhandle-getctx) [NewLockHandle](#cache-newlockhandle) [Release](#lockhandle-release) [ReleaseCtx](#lockhandle-releasectx) [TryLock](#cache-trylock) [TryLockCtx](#cache-trylockctx) [Unlock](#cache-unlock) [UnlockCtx](#cache-unlockctx) |
| **Memoization** | [NewMemoStore](#newmemostore) |
| **Observability** | [WithObserver](#cache-withobserver) |
| **Options** | [WithCompression](#withcompression) [WithDefaultTTL](#withdefaultttl) [WithDynamoClient](#withdynamoclient) [WithDynamoEndpoint](#withdynamoendpoint) [WithDynamoRegion](#withdynamoregion) [WithDynamoTable](#withdynamotable) [WithEncryptionKey](#withencryptionkey) [WithFileDir](#withfiledir) [WithMaxValueBytes](#withmaxvaluebytes) [WithMemcachedAddresses](#withmemcachedaddresses) [WithMemoryCleanupInterval](#withmemorycleanupinterval) [WithNATSBucketTTL](#withnatsbucketttl) [WithNATSKeyValue](#withnatskeyvalue) [WithPrefix](#withprefix) [WithRedisClient](#withredisclient) [WithSQL](#withsql) |
| **Rate Limiting** | [RateLimit](#cache-ratelimit) [RateLimitCtx](#cache-ratelimitctx) |
| **Read Through** | [Remember](#remember) [RememberBytes](#cache-rememberbytes) [RememberBytesCtx](#cache-rememberbytesctx) [RememberJSON](#rememberjson) [RememberJSONCtx](#rememberjsonctx) [RememberStale](#rememberstale) [RememberStaleBytes](#cache-rememberstalebytes) [RememberStaleBytesCtx](#cache-rememberstalebytesctx) [RememberStaleCtx](#rememberstalectx) [RememberStaleValueWithCodec](#rememberstalevaluewithcodec) [RememberString](#cache-rememberstring) [RememberStringCtx](#cache-rememberstringctx) [RememberValue](#remembervalue) [RememberValueWithCodec](#remembervaluewithcodec) |
| **Reads** | [BatchGetBytes](#cache-batchgetbytes) [BatchGetBytesCtx](#cache-batchgetbytesctx) [Get](#get) [GetBytes](#cache-getbytes) [GetBytesCtx](#cache-getbytesctx) [GetCtx](#getctx) [GetJSON](#getjson) [GetJSONCtx](#getjsonctx) [GetString](#cache-getstring) [GetStringCtx](#cache-getstringctx) |
| **Refresh Ahead** | [RefreshAhead](#refreshahead) [RefreshAheadBytes](#cache-refreshaheadbytes) [RefreshAheadBytesCtx](#cache-refreshaheadbytesctx) [RefreshAheadCtx](#refreshaheadctx) [RefreshAheadValueWithCodec](#refreshaheadvaluewithcodec) |
| **Writes** | [Add](#cache-add) [AddCtx](#cache-addctx) [BatchSetBytes](#cache-batchsetbytes) [BatchSetBytesCtx](#cache-batchsetbytesctx) [Decrement](#cache-decrement) [DecrementCtx](#cache-decrementctx) [Increment](#cache-increment) [IncrementCtx](#cache-incrementctx) [Set](#set) [SetBytes](#cache-setbytes) [SetBytesCtx](#cache-setbytesctx) [SetCtx](#setctx) [SetJSON](#setjson) [SetJSONCtx](#setjsonctx) [SetString](#cache-setstring) [SetStringCtx](#cache-setstringctx) |


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

### <a id="cache-driver"></a>Driver

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

### <a id="cache-store"></a>Store

Store returns the underlying store implementation.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Store().Driver()) // memory
```

## Invalidation

### <a id="cache-delete"></a>Delete

Delete removes a single key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Delete("a") == nil) // true
```

### <a id="cache-deletectx"></a>DeleteCtx

DeleteCtx is the context-aware variant of Delete.

### <a id="cache-deletemany"></a>DeleteMany

DeleteMany removes multiple keys.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.DeleteMany("a", "b") == nil) // true
```

### <a id="cache-deletemanyctx"></a>DeleteManyCtx

DeleteManyCtx is the context-aware variant of DeleteMany.

### <a id="cache-flush"></a>Flush

Flush clears all keys for this store scope.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Flush() == nil) // true
```

### <a id="cache-flushctx"></a>FlushCtx

FlushCtx is the context-aware variant of Flush.

### <a id="pull"></a>Pull

Pull returns a typed value for key and removes it, using the default codec (JSON).

```go
type Token struct { Value string `json:"value"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
tok, ok, err := cache.Pull[Token](c, "reset:token:42")
fmt.Println(err == nil, ok, tok.Value) // true true abc
```

### <a id="cache-pullbytes"></a>PullBytes

PullBytes returns value and removes it from cache.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
body, ok, _ := c.PullBytes("reset:token:42")
fmt.Println(ok, string(body)) // true abc
```

### <a id="cache-pullbytesctx"></a>PullBytesCtx

PullBytesCtx is the context-aware variant of PullBytes.

### <a id="pullctx"></a>PullCtx

PullCtx is the context-aware variant of Pull.

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

### <a id="lockhandle-acquirectx"></a>AcquireCtx

AcquireCtx is the context-aware variant of Acquire.

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

### <a id="lockhandle-blockctx"></a>BlockCtx

BlockCtx is the context-aware variant of Block.

### <a id="cache-lock"></a>Lock

Lock waits until the lock is acquired or timeout elapses.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, err := c.Lock("job:sync", 10*time.Second, time.Second)
fmt.Println(err == nil, locked) // true true
```

### <a id="cache-lockctx"></a>LockCtx

LockCtx retries lock acquisition until success or context cancellation.

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

### <a id="lockhandle-getctx"></a>LockHandle.GetCtx

GetCtx is the context-aware variant of Get.

### <a id="cache-newlockhandle"></a>NewLockHandle

NewLockHandle creates a reusable lock handle for a key/ttl pair.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
lock := c.NewLockHandle("job:sync", 10*time.Second)
locked, err := lock.Acquire()
fmt.Println(err == nil, locked) // true true
if locked {
}
```

### <a id="lockhandle-release"></a>Release

Release unlocks the key if this handle previously acquired it.

It is safe to call multiple times; repeated calls become no-ops after the first
successful release.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
lock := c.NewLockHandle("job:sync", 10*time.Second)
locked, _ := lock.Acquire()
if locked {
}
```

### <a id="lockhandle-releasectx"></a>ReleaseCtx

ReleaseCtx is the context-aware variant of Release.

### <a id="cache-trylock"></a>TryLock

TryLock acquires a short-lived lock key when not already held.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, _ := c.TryLock("job:sync", 10*time.Second)
fmt.Println(locked) // true
```

### <a id="cache-trylockctx"></a>TryLockCtx

TryLockCtx is the context-aware variant of TryLock.

### <a id="cache-unlock"></a>Unlock

Unlock releases a previously acquired lock key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
locked, _ := c.TryLock("job:sync", 10*time.Second)
if locked {
}
```

### <a id="cache-unlockctx"></a>UnlockCtx

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

### <a id="cache-withobserver"></a>WithObserver

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

### <a id="cache-ratelimit"></a>RateLimit

RateLimit increments a fixed-window counter and returns allowance metadata.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
res, err := c.RateLimit("rl:api:ip:1.2.3.4", 100, time.Minute)
fmt.Println(err == nil, res.Allowed, res.Count, res.Remaining, !res.ResetAt.IsZero())
// Output: true true 1 99 true
```

### <a id="cache-ratelimitctx"></a>RateLimitCtx

RateLimitCtx is the context-aware variant of RateLimit.

## Read Through

### <a id="remember"></a>Remember

Remember is the ergonomic, typed remember helper using JSON encoding by default.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, err := cache.Remember[Profile](c, "profile:42", time.Minute, func() (Profile, error) {
	return Profile{Name: "Ada"}, nil
})
fmt.Println(err == nil, profile.Name) // true Ada
```

### <a id="cache-rememberbytes"></a>RememberBytes

RememberBytes returns key value or computes/stores it when missing.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
data, err := c.RememberBytes("dashboard:summary", time.Minute, func() ([]byte, error) {
	return []byte("payload"), nil
})
fmt.Println(err == nil, string(data)) // true payload
```

### <a id="cache-rememberbytesctx"></a>RememberBytesCtx

RememberBytesCtx is the context-aware variant of RememberBytes.

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

### <a id="cache-rememberstalebytesctx"></a>RememberStaleBytesCtx

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

### <a id="cache-rememberstring"></a>RememberString

RememberString returns key value or computes/stores it when missing.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, err := c.RememberString("settings:mode", time.Minute, func() (string, error) {
	return "on", nil
})
fmt.Println(err == nil, val) // true on
```

### <a id="cache-rememberstringctx"></a>RememberStringCtx

RememberStringCtx is the context-aware variant of RememberString.

### <a id="remembervalue"></a>RememberValue

RememberValue returns a typed value or computes/stores it when missing using JSON encoding by default.

```go
type Summary struct { Text string `json:"text"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
s, err := cache.RememberValue[Summary](c, "dashboard:summary", time.Minute, func() (Summary, error) {
	return Summary{Text: "ok"}, nil
})
fmt.Println(err == nil, s.Text) // true ok
```

### <a id="remembervaluewithcodec"></a>RememberValueWithCodec

RememberValueWithCodec allows custom encoding/decoding for typed remember operations.

## Reads

### <a id="cache-batchgetbytes"></a>BatchGetBytes

BatchGetBytes returns all found values for the provided keys.
Missing keys are omitted from the returned map.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
values, err := c.BatchGetBytes("a", "b", "missing")
fmt.Println(err == nil, string(values["a"]), string(values["b"])) // true 1 2
```

### <a id="cache-batchgetbytesctx"></a>BatchGetBytesCtx

BatchGetBytesCtx is the context-aware variant of BatchGetBytes.

### <a id="get"></a>Get

Get returns a typed value for key using the default codec (JSON) when present.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, ok, err := cache.Get[Profile](c, "profile:42")
fmt.Println(err == nil, ok, profile.Name) // true true Ada
```

### <a id="cache-getbytes"></a>GetBytes

GetBytes returns raw bytes for key when present.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
value, ok, _ := c.GetBytes("user:42")
fmt.Println(ok, string(value)) // true Ada
```

### <a id="cache-getbytesctx"></a>GetBytesCtx

GetBytesCtx is the context-aware variant of GetBytes.

### <a id="getctx"></a>GetCtx

GetCtx is the context-aware variant of Get.

### <a id="getjson"></a>GetJSON

GetJSON decodes a JSON value into T when key exists, using background context.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, ok, err := cache.GetJSON[Profile](c, "profile:42")
fmt.Println(err == nil, ok, profile.Name) // true true Ada
```

### <a id="getjsonctx"></a>GetJSONCtx

GetJSONCtx is the context-aware variant of GetJSON.

### <a id="cache-getstring"></a>GetString

GetString returns a UTF-8 string value for key when present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
name, ok, _ := c.GetString("user:42:name")
fmt.Println(ok, name) // true Ada
```

### <a id="cache-getstringctx"></a>GetStringCtx

GetStringCtx is the context-aware variant of GetString.

## Refresh Ahead

### <a id="refreshahead"></a>RefreshAhead

RefreshAhead returns a typed value and refreshes asynchronously when near expiry.

```go
type Summary struct { Text string `json:"text"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
s, err := cache.RefreshAhead[Summary](c, "dashboard:summary", time.Minute, 10*time.Second, func() (Summary, error) {
	return Summary{Text: "ok"}, nil
})
fmt.Println(err == nil, s.Text) // true ok
```

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

### <a id="cache-refreshaheadbytesctx"></a>RefreshAheadBytesCtx

RefreshAheadBytesCtx is the context-aware variant of RefreshAheadBytes.

### <a id="refreshaheadctx"></a>RefreshAheadCtx

RefreshAheadCtx is the context-aware variant of RefreshAhead.

### <a id="refreshaheadvaluewithcodec"></a>RefreshAheadValueWithCodec

RefreshAheadValueWithCodec allows custom encoding/decoding for typed refresh-ahead operations.

## Writes

### <a id="cache-add"></a>Add

Add writes value only when key is not already present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
created, _ := c.Add("boot:seeded", []byte("1"), time.Hour)
fmt.Println(created) // true
```

### <a id="cache-addctx"></a>AddCtx

AddCtx is the context-aware variant of Add.

### <a id="cache-batchsetbytes"></a>BatchSetBytes

BatchSetBytes writes many key/value pairs using a shared ttl.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
err := c.BatchSetBytes(map[string][]byte{
	"a": []byte("1"),
	"b": []byte("2"),
}, time.Minute)
fmt.Println(err == nil) // true
```

### <a id="cache-batchsetbytesctx"></a>BatchSetBytesCtx

BatchSetBytesCtx is the context-aware variant of BatchSetBytes.

### <a id="cache-decrement"></a>Decrement

Decrement decrements a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Decrement("rate:login:42", 1, time.Minute)
fmt.Println(val) // -1
```

### <a id="cache-decrementctx"></a>DecrementCtx

DecrementCtx is the context-aware variant of Decrement.

### <a id="cache-increment"></a>Increment

Increment increments a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Increment("rate:login:42", 1, time.Minute)
fmt.Println(val) // 1
```

### <a id="cache-incrementctx"></a>IncrementCtx

IncrementCtx is the context-aware variant of Increment.

### <a id="set"></a>Set

Set encodes value with the default codec (JSON) and writes it to key.

```go
type Settings struct { Enabled bool `json:"enabled"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
err := cache.Set(c, "settings:alerts", Settings{Enabled: true}, time.Minute)
fmt.Println(err == nil) // true
```

### <a id="cache-setbytes"></a>SetBytes

SetBytes writes raw bytes to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.SetBytes("token", []byte("abc"), time.Minute) == nil) // true
```

### <a id="cache-setbytesctx"></a>SetBytesCtx

SetBytesCtx is the context-aware variant of SetBytes.

### <a id="setctx"></a>SetCtx

SetCtx is the context-aware variant of Set.

### <a id="setjson"></a>SetJSON

SetJSON encodes value as JSON and writes it to key using background context.

```go
type Settings struct { Enabled bool `json:"enabled"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
err := cache.SetJSON(c, "settings:alerts", Settings{Enabled: true}, time.Minute)
fmt.Println(err == nil) // true
```

### <a id="setjsonctx"></a>SetJSONCtx

SetJSONCtx is the context-aware variant of SetJSON.

### <a id="cache-setstring"></a>SetString

SetString writes a string value to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.SetString("user:42:name", "Ada", time.Minute) == nil) // true
```

### <a id="cache-setstringctx"></a>SetStringCtx

SetStringCtx is the context-aware variant of SetString.
<!-- api:embed:end -->

### Payload size caps (effective bytes written)

| Driver | Hard / default cap | Configurable | Notes |
| ---: | :--- | :---: | :--- |
| **Null** | N/A | N/A | No persistence. |
| **Memory** | Process memory | - | No backend hard cap. |
| **File** | Disk / filesystem | - | No backend hard cap. |
| **Redis** | Backend practical (memory/SLO) | Server-side | No commonly hit low per-value hard cap in app use. |
| **NATS** | Server/bucket payload limits | Server-side | Depends on NATS/JetStream config. |
| **Memcached** | ~1 MiB per item (default) | ✓ (server `-I`) | Backend-enforced item limit. |
| **DynamoDB** | 400 KB item hard cap | No | Includes key/metadata overhead, so usable value bytes are lower. |
| **SQL** | DB/engine config dependent | Server-side | Blob/row/packet limits vary by engine and deployment. |

`WithMaxValueBytes` is the only uniform application-level cap across all drivers, and it applies to post-shaping bytes (after compression/encryption overhead).

## Integration Coverage

| Area | What is validated | Scope |
| :--- | :--- | :--- |
| Core store contract | `Set/Get`, TTL expiry, `Add`, counters, `Delete/DeleteMany`, `Flush`, typed `Remember` | All drivers |
| Option contracts | `prefix`, `compression`, `encryption`, `prefix+compression+encryption`, `WithMaxValueBytes`, `WithDefaultTTL` | All drivers (per option case) |
| Locking | single-winner contention, timeout/cancel, TTL expiry reacquire, unlock safety | All drivers |
| Rate limiting | monotonic counts, `remaining >= 0`, window rollover reset | All drivers |
| Refresh-ahead | miss/hit behavior, async refresh success/error, malformed metadata handling | All drivers |
| Remember stale | stale fallback semantics, TTL interactions, stale/fresh independent expiry, joined errors | All drivers |
| Batch ops | partial misses, empty input behavior, default TTL application | All drivers |
| Counter semantics | signed deltas, zero delta, TTL refresh extension | All drivers |
| Context cancellation | `GetCtx/SetCtx/LockCtx/RefreshAheadCtx/Remember*Ctx` prompt return + driver-aware cancel semantics | All drivers (driver-aware assertions) |
| Latency / transient faults | injected slow `Get/Add/Increment`, timeout propagation, no hidden retries for `RefreshAhead/Remember*/LockCtx/RateLimit*` | All drivers (integration wrappers over real stores) |
| Prefix isolation | `Delete/Flush` isolation + helper-generated keys (`__lock:`, `:__refresh_exp`, `:__stale`, rate-limit buckets) | Shared/prefixed backends |
| Payload shaping / corruption | compression+encryption round-trips, corrupted compressed/encrypted payload errors | Shared/persistent backends |
| Payload size limits | large binary payload round-trips; backend-specific near/over-limit checks (Memcached, DynamoDB) | Driver-specific where meaningful |
| Cross-store scope | shared vs local semantics across store instances (e.g. rate-limit counters) | Driver-specific expectations |
| Backend fault / recovery | backend restart mid-suite, outage errors, post-recovery round-trip/lock/refresh/stale flows | Container-backed drivers (`INTEGRATION_FAULT=1`) |
| Observer metadata | op names, hit/miss flags, propagated errors, driver labels | Unit contract tests (integration helper paths exercise emissions indirectly) |
| Memo store caveats | per-process memoization, local-only invalidation, cross-process staleness behavior | Unit tests |

Default integration runs cover the contract suite above. Fault/recovery restart tests are opt-in because they restart shared testcontainers and are slower/flakier by nature.

## Contributing (README updates)

README content is a mix of generated sections and manual sections.

- API reference (`<!-- api:embed:start --> ... <!-- api:embed:end -->`) is generated.
- Test badges are updated separately.
- Sections like driver notes and the integration coverage table are manual.

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
go run ./docs/readme/testcounts/main.go
```

### Watch mode

```bash
./docs/watcher.sh
```

Notes:

- The badge watcher runs real tests, so it is slower than API/example regeneration.
- Fault/recovery integration tests are opt-in (`INTEGRATION_FAULT=1`).
