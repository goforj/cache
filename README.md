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
<!-- test-count:embed:start -->
    <img src="https://img.shields.io/badge/tests-14-brightgreen" alt="Tests">
<!-- test-count:embed:end -->
</p>

<p align="center">
  <img src="https://img.shields.io/badge/drivers-memory%20%E2%9C%85%20|%20redis%20%E2%9C%85-brightgreen" alt="Drivers">
  <img src="https://img.shields.io/badge/options-ttl%20|%20prefix%20|%20counters%20|%20add%20|%20memo-brightgreen" alt="Options">
</p>

## What cache is

An explicit cache abstraction with a minimal `Store` interface and ergonomic `Repository` helpers. Drivers are chosen when you construct the store (no env coupling), so swapping backends is a dependency-injection change instead of a refactor.

## Drivers

| Driver | Mode | Shared | Durable | TTL | Counters |
| ---: | :--- | :---: | :---: | :---: | :---: |
| <img src="https://img.shields.io/badge/memory-5c5c5c?logo=cachet&logoColor=white" alt="Memory"> | In-process | - | - | ✓ | ✓ |
| <img src="https://img.shields.io/badge/redis-%23DC382D?logo=redis&logoColor=white" alt="Redis"> | Networked | ✓ | - | ✓ | ✓ |

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

    store := cache.NewStore(ctx, cache.StoreConfig{
        Driver:               cache.DriverMemory, // or DriverRedis
        DefaultTTL:           5 * time.Minute,
        MemoryCleanupInterval: 10 * time.Minute,
    })
    repo := cache.NewRepository(store)

    // Remember pattern.
    profile, err := repo.Remember(ctx, "user:42:profile", time.Minute, func(context.Context) ([]byte, error) {
        return []byte(`{"name":"ada"}`), nil
    })
    _ = profile

    // Switch to Redis (dependency injection, no code changes below).
    client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
    store = cache.NewStore(ctx, cache.StoreConfig{
        Driver:      cache.DriverRedis,
        Prefix:      "app",
        DefaultTTL:  5 * time.Minute,
        RedisClient: client,
    })
    repo = cache.NewRepository(store)
}
```

## StoreConfig

`StoreConfig` keeps configuration explicit:

- `Driver`: `DriverMemory` (default) or `DriverRedis`
- `DefaultTTL`: fallback TTL when a call provides `ttl <= 0`
- `MemoryCleanupInterval`: sweep interval for memory driver
- `Prefix`: key prefix for shared backends
- `RedisClient`: required when using the Redis driver

## Repository helpers

`Repository` wraps a `Store` with ergonomic helpers:

- `Remember`, `RememberString`, `RememberJSON`
- `Get`, `GetString`, `GetJSON`
- `Set`, `SetString`, `SetJSON`
- `Add`, `Increment`, `Decrement`
- `Pull`, `Delete`, `DeleteMany`, `Flush`

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
memoRepo := cache.NewRepository(memoStore)
```

## Testing

Unit tests cover the public helpers. Integration tests use `testcontainers-go` to spin up Redis:

```bash
go test -tags=integration ./...
```

Use `INTEGRATION_DRIVER=redis` (comma-separated; defaults to `all`) to select which drivers start containers and run the contract suite.
