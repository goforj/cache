<p align="center">
  <img src="./docs/images/logo.png?v=1" width="420" alt="cache logo">
</p>

<p align="center">
    cache gives your services one cache API with memory and redis drivers.
</p>

<p align="center">
    <a href="https://pkg.go.dev/github.com/goforj/cache"><img src="https://pkg.go.dev/badge/github.com/goforj/cache.svg" alt="Go Reference"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <a href="https://golang.org"><img src="https://img.shields.io/badge/go-1.24+-blue?logo=go" alt="Go version"></a>
    <img src="https://img.shields.io/github/v/tag/goforj/cache?label=version&sort=semver" alt="Latest tag">
    <a href="https://goreportcard.com/report/github.com/goforj/cache"><img src="https://goreportcard.com/badge/github.com/goforj/cache" alt="Go Report Card"></a>
    <a href="https://codecov.io/gh/goforj/cache"><img src="https://codecov.io/gh/goforj/cache/graph/badge.svg?token=B6ROULLKWU"/></a>
<!-- test-count:embed:start -->
    <img src="https://img.shields.io/badge/tests-209-brightgreen" alt="Tests">
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
    profile, err := c.Remember("user:42:profile", time.Minute, func() ([]byte, error) {
        return []byte(`{"name":"ada"}`), nil
    })
    _ = profile

    // Switch to Redis (dependency injection, no code changes below).
    client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
    store = cache.NewRedisStore(ctx, client, cache.WithPrefix("app"), cache.WithDefaultTTL(5*time.Minute))
    c = cache.NewCache(store)
}
```

## StoreConfig

StoreConfig keeps configuration explicit:

- Driver: explicit backend (null, file, memory, memcached, redis, dynamodb, sql)
- DefaultTTL: fallback TTL when a call provides ttl <= 0
- MemoryCleanupInterval: sweep interval for memory driver
- Prefix: key prefix for shared backends
- RedisClient / MemcachedAddresses / DynamoClient / SQLDriverName+DSN: driver-specific inputs
- Compression / MaxValueBytes / EncryptionKey: shaping and security controls

## Cache helpers

Cache wraps a Store with ergonomic helpers (context-free by default, `*Ctx` variants when you need a context). Store stays context-first because drivers perform I/O and should honor deadlines/cancellation; Cache gives you the convenience layer on top:

```go
Get(key string) ([]byte, bool, error) // returns raw bytes for key when present.
GetString(key string) (string, bool, error) // returns string for key when present.
Set(key string, value []byte, ttl time.Duration) error // writes bytes to key with TTL.
SetString(key string, value string, ttl time.Duration) error // writes string to key with TTL.
Add(key string, value []byte, ttl time.Duration) (bool, error) // writes value only when key is absent.
Increment(key string, delta int64, ttl time.Duration) (int64, error) // increments numeric value and returns the result.
Decrement(key string, delta int64, ttl time.Duration) (int64, error) // decrements numeric value and returns the result.
Pull(key string) ([]byte, bool, error) // returns value and removes it.
Delete(key string) error // removes a single key.
DeleteMany(keys ...string) error // removes multiple keys.
Flush() error // clears all keys for this store scope.
Remember(key string, ttl time.Duration, fn func() ([]byte, error)) ([]byte, error) // returns value or computes/stores it when missing.
RememberString(key string, ttl time.Duration, fn func() (string, error)) (string, error) // string helper for Remember.
RememberJSON[T any](cache *Cache, key string, ttl time.Duration, fn func() (T, error)) (T, error) // JSON helper for Remember.
GetJSON[T any](cache *Cache, key string) (T, bool, error) // decodes JSON into T when key exists.
SetJSON[T any](cache *Cache, key string, value T, ttl time.Duration) error // writes JSON-encoded value to key with TTL.
// ctx-aware variants mirror the same names: GetCtx, SetCtx, RememberCtx, RememberStringCtx, RememberJSONCtx, etc.
```

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

## API reference

The API section below is autogenerated; do not edit between the markers.

<!-- api:embed:start -->

## API Index

| Group | Functions |
|------:|:-----------|
| **Cache** | [Add](#add) [Decrement](#decrement) [Delete](#delete) [DeleteMany](#deletemany) [Driver](#driver) [Flush](#flush) [Get](#get) [GetString](#getstring) [Increment](#increment) [NewCache](#newcache) [NewCacheWithTTL](#newcachewithttl) [Pull](#pull) [Remember](#remember) [RememberString](#rememberstring) [Set](#set) [SetString](#setstring) [Store](#store) |
| **Cache JSON** | [GetJSON](#getjson) [RememberJSON](#rememberjson) [SetJSON](#setjson) |
| **Constructors** | [NewDynamoStore](#newdynamostore) [NewFileStore](#newfilestore) [NewMemcachedStore](#newmemcachedstore) [NewMemoryStore](#newmemorystore) [NewNullStore](#newnullstore) [NewRedisStore](#newredisstore) [NewSQLStore](#newsqlstore) [NewStore](#newstore) [NewStoreWith](#newstorewith) |
| **Memoization** | [NewMemoStore](#newmemostore) |
| **Options** | [WithCompression](#withcompression) [WithDefaultTTL](#withdefaultttl) [WithDynamoClient](#withdynamoclient) [WithDynamoEndpoint](#withdynamoendpoint) [WithDynamoRegion](#withdynamoregion) [WithDynamoTable](#withdynamotable) [WithEncryptionKey](#withencryptionkey) [WithFileDir](#withfiledir) [WithMaxValueBytes](#withmaxvaluebytes) [WithMemcachedAddresses](#withmemcachedaddresses) [WithMemoryCleanupInterval](#withmemorycleanupinterval) [WithPrefix](#withprefix) [WithRedisClient](#withredisclient) [WithSQL](#withsql) |
| **Other** | [GetJSONCtx](#getjsonctx) [SetJSONCtx](#setjsonctx) [WithObserver](#withobserver) |


## Cache

### <a id="add"></a>Add

Add writes value only when key is not already present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
created, _ := c.Add("boot:seeded", []byte("1"), time.Hour)
fmt.Println(created) // true
```

### <a id="decrement"></a>Decrement

Decrement decrements a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Decrement("rate:login:42", 1, time.Minute)
fmt.Println(val) // -1
```

### <a id="delete"></a>Delete

Delete removes a single key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Delete("a") == nil) // true
```

### <a id="deletemany"></a>DeleteMany

DeleteMany removes multiple keys.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.DeleteMany("a", "b") == nil) // true
```

### <a id="driver"></a>Driver

Driver reports the underlying store driver.

### <a id="flush"></a>Flush

Flush clears all keys for this store scope.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Flush() == nil) // true
```

### <a id="get"></a>Get

Get returns raw bytes for key when present.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
value, ok, _ := c.Get("user:42")
fmt.Println(ok, string(value)) // true Ada
```

### <a id="getstring"></a>GetString

GetString returns a UTF-8 string value for key when present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
name, ok, _ := c.GetString("user:42:name")
fmt.Println(ok, name) // true Ada
```

### <a id="increment"></a>Increment

Increment increments a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Increment("rate:login:42", 1, time.Minute)
fmt.Println(val) // 1
```

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

### <a id="pull"></a>Pull

Pull returns value and removes it from cache.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
body, ok, _ := c.Pull("reset:token:42")
fmt.Println(ok, string(body)) // true abc
```

### <a id="remember"></a>Remember

Remember returns key value or computes/stores it when missing.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
data, err := c.Remember("dashboard:summary", time.Minute, func() ([]byte, error) {
	return []byte("payload"), nil
})
fmt.Println(err == nil, string(data)) // true payload
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

### <a id="set"></a>Set

Set writes raw bytes to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Set("token", []byte("abc"), time.Minute) == nil) // true
```

### <a id="setstring"></a>SetString

SetString writes a string value to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.SetString("user:42:name", "Ada", time.Minute) == nil) // true
```

### <a id="store"></a>Store

Store returns the underlying store implementation.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Store().Driver()) // memory
```

## Cache JSON

### <a id="getjson"></a>GetJSON

GetJSON decodes a JSON value into T when key exists, using background context.

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

### <a id="setjson"></a>SetJSON

SetJSON encodes value as JSON and writes it to key using background context.

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

## Other

### <a id="getjsonctx"></a>GetJSONCtx

GetJSONCtx is the context-aware variant of GetJSON.

### <a id="setjsonctx"></a>SetJSONCtx

SetJSONCtx is the context-aware variant of SetJSON.

### <a id="withobserver"></a>WithObserver

WithObserver attaches an observer to receive operation events.
<!-- api:embed:end -->
