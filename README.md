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
    <img src="https://img.shields.io/badge/tests-90-brightgreen" alt="Tests">
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

    store := cache.NewStoreWith(ctx, cache.DriverMemory,
        cache.WithDefaultTTL(5*time.Minute),
        cache.WithMemoryCleanupInterval(10*time.Minute),
    )
    repo := cache.NewCache(store)

    // Remember pattern.
    profile, err := repo.Remember(ctx, "user:42:profile", time.Minute, func(context.Context) ([]byte, error) {
        return []byte(`{"name":"ada"}`), nil
    })
    _ = profile

    // Switch to Redis (dependency injection, no code changes below).
    client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
    store = cache.NewStoreWith(ctx, cache.DriverRedis,
        cache.WithRedisClient(client),
        cache.WithPrefix("app"),
        cache.WithDefaultTTL(5*time.Minute),
    )
    repo = cache.NewCache(store)
}
```

## StoreConfig

StoreConfig keeps configuration explicit:

- Driver: DriverMemory (default) or DriverRedis
- DefaultTTL: fallback TTL when a call provides ttl <= 0
- MemoryCleanupInterval: sweep interval for memory driver
- Prefix: key prefix for shared backends
- RedisClient: required when using the Redis driver

## Cache helpers

Cache wraps a Store with ergonomic helpers:

```go
type CacheHelpers interface {
	Remember(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) ([]byte, error)) ([]byte, error)                  // returns value or computes/stores it when missing.
	RememberString(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) (string, error)) (string, error)            // returns string value or computes/stores it when missing.
	RememberJSON[T any](ctx context.Context, cache *Cache, key string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error)   // returns JSON value or computes/stores it when missing.
	Get(ctx context.Context, key string) ([]byte, bool, error)                                                                              // returns raw bytes for key when present.
	GetString(ctx context.Context, key string) (string, bool, error)                                                                        // returns string for key when present.
	GetJSON[T any](ctx context.Context, cache *Cache, key string) (T, bool, error)                                                          // decodes JSON into T when key exists.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error                                                             // writes bytes to key with TTL.
	SetString(ctx context.Context, key string, value string, ttl time.Duration) error                                                       // writes string to key with TTL.
	SetJSON[T any](ctx context.Context, cache *Cache, key string, value T, ttl time.Duration) error                                         // writes JSON-encoded value to key with TTL.
	Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)                                                     // writes value only when key is not present.
	Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)                                               // increments numeric value and returns the result.
	Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)                                               // decrements numeric value and returns the result.
	Pull(ctx context.Context, key string) ([]byte, bool, error)                                                                             // returns value and removes it.
	Delete(ctx context.Context, key string) error                                                                                           // removes a single key.
	DeleteMany(ctx context.Context, keys ...string) error                                                                                   // removes multiple keys.
	Flush(ctx context.Context) error                                                                                                        // clears all keys for this store scope.
}
```

Example:

```go
settings, err := cache.RememberJSON[Settings](ctx, repo, "settings:alerts", 10*time.Minute, func(context.Context) (Settings, error) {
    return fetchSettings(ctx)
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
| **Other** | [WithDefaultTTL](#withdefaultttl) [WithDynamoClient](#withdynamoclient) [WithDynamoEndpoint](#withdynamoendpoint) [WithDynamoRegion](#withdynamoregion) [WithDynamoTable](#withdynamotable) [WithFileDir](#withfiledir) [WithMemcachedAddresses](#withmemcachedaddresses) [WithMemoryCleanupInterval](#withmemorycleanupinterval) [WithPrefix](#withprefix) [WithRedisClient](#withredisclient) [WithSQL](#withsql) |


## Cache

### <a id="add"></a>Add

Add writes value only when key is not already present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
created, _ := c.Add(ctx, "boot:seeded", []byte("1"), time.Hour)
fmt.Println(created) // true
```

### <a id="decrement"></a>Decrement

Decrement decrements a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Decrement(ctx, "rate:login:42", 1, time.Minute)
fmt.Println(val) // -1
```

### <a id="delete"></a>Delete

Delete removes a single key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Delete(ctx, "a") == nil) // true
```

### <a id="deletemany"></a>DeleteMany

DeleteMany removes multiple keys.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.DeleteMany(ctx, "a", "b") == nil) // true
```

### <a id="driver"></a>Driver

Driver reports the underlying store driver.

### <a id="flush"></a>Flush

Flush clears all keys for this store scope.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Flush(ctx) == nil) // true
```

### <a id="get"></a>Get

Get returns raw bytes for key when present.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
value, ok, _ := c.Get(ctx, "user:42")
fmt.Println(ok, string(value)) // true Ada
```

### <a id="getstring"></a>GetString

GetString returns a UTF-8 string value for key when present.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
name, ok, _ := c.GetString(ctx, "user:42:name")
fmt.Println(ok, name) // true Ada
```

### <a id="increment"></a>Increment

Increment increments a numeric value and returns the result.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, _ := c.Increment(ctx, "rate:login:42", 1, time.Minute)
fmt.Println(val) // 1
```

### <a id="newcache"></a>NewCache

NewCache creates a cache facade bound to a concrete store.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCache(s)
fmt.Println(c.Driver()) // DriverMemory
```

### <a id="newcachewithttl"></a>NewCacheWithTTL

NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.

```go
ctx := context.Background()
s := cache.NewMemoryStore(ctx)
c := cache.NewCacheWithTTL(s, 2*time.Minute)
fmt.Println(c.Driver(), c != nil) // DriverMemory true
```

### <a id="pull"></a>Pull

Pull returns value and removes it from cache.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
body, ok, _ := c.Pull(ctx, "reset:token:42")
fmt.Println(ok, string(body)) // true abc
```

### <a id="remember"></a>Remember

Remember returns key value or computes/stores it when missing.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
data, err := c.Remember(ctx, "dashboard:summary", time.Minute, func(context.Context) ([]byte, error) {
	return []byte("payload"), nil
})
fmt.Println(err == nil, string(data)) // true payload
```

### <a id="rememberstring"></a>RememberString

RememberString returns key value or computes/stores it when missing.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
val, err := c.RememberString(ctx, "settings:mode", time.Minute, func(context.Context) (string, error) {
	return "on", nil
})
fmt.Println(err == nil, val) // true on
```

### <a id="set"></a>Set

Set writes raw bytes to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Set(ctx, "token", []byte("abc"), time.Minute) == nil) // true
```

### <a id="setstring"></a>SetString

SetString writes a string value to key.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.SetString(ctx, "user:42:name", "Ada", time.Minute) == nil) // true
```

### <a id="store"></a>Store

Store returns the underlying store implementation.

```go
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(c.Store().Driver()) // DriverMemory
```

## Cache JSON

### <a id="getjson"></a>GetJSON

GetJSON decodes a JSON value into T when key exists.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
profile, ok, _ := cache.GetJSON[Profile](ctx, c, "profile:42")
fmt.Println(ok, profile.Name) // true Ada
```

### <a id="rememberjson"></a>RememberJSON

RememberJSON returns key value or computes/stores JSON when missing.

```go
type Settings struct { Enabled bool `json:"enabled"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
settings, err := cache.RememberJSON[Settings](ctx, c, "settings:alerts", time.Minute, func(context.Context) (Settings, error) {
	return Settings{Enabled: true}, nil
})
fmt.Println(err == nil, settings.Enabled) // true true
```

### <a id="setjson"></a>SetJSON

SetJSON encodes value as JSON and writes it to key.

```go
type Profile struct { Name string `json:"name"` }
ctx := context.Background()
c := cache.NewCache(cache.NewMemoryStore(ctx))
fmt.Println(cache.SetJSON(ctx, c, "profile:42", Profile{Name: "Ada"}, time.Minute) == nil) // true
```

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

## Other

### <a id="withdefaultttl"></a>WithDefaultTTL

WithDefaultTTL overrides the fallback TTL used when ttl <= 0.

### <a id="withdynamoclient"></a>WithDynamoClient

WithDynamoClient injects a pre-built DynamoDB client.

### <a id="withdynamoendpoint"></a>WithDynamoEndpoint

WithDynamoEndpoint sets the DynamoDB endpoint (useful for local testing).

### <a id="withdynamoregion"></a>WithDynamoRegion

WithDynamoRegion sets the DynamoDB region for requests.

### <a id="withdynamotable"></a>WithDynamoTable

WithDynamoTable sets the table used by the DynamoDB driver.

### <a id="withfiledir"></a>WithFileDir

WithFileDir sets the directory used by the file driver.

### <a id="withmemcachedaddresses"></a>WithMemcachedAddresses

WithMemcachedAddresses sets memcached server addresses (host:port).

### <a id="withmemorycleanupinterval"></a>WithMemoryCleanupInterval

WithMemoryCleanupInterval overrides the sweep interval for the memory driver.

### <a id="withprefix"></a>WithPrefix

WithPrefix sets the key prefix for shared backends (e.g., redis).

### <a id="withredisclient"></a>WithRedisClient

WithRedisClient sets the redis client; required when using DriverRedis.

### <a id="withsql"></a>WithSQL

WithSQL configures the SQL driver (driver name + DSN + optional table).
<!-- api:embed:end -->
