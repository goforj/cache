# Production Guide

This guide covers practical defaults and operational patterns for running cache in production.

## Recommended Defaults

- Use explicit prefixes for shared backends:
  - example: `cachecore.BaseConfig{Prefix: "billing:v1"}`
- Set a non-zero default TTL at store construction:
  - example: 5 to 15 minutes for read-mostly metadata
- Use backend-specific strengths:
  - memory/file for local or single-node use
  - `driver/rediscache`, `driver/memcachedcache`, `driver/natscache`, `driver/dynamocache`
  - `driver/sqlitecache`, `driver/postgrescache`, `driver/mysqlcache` (with `driver/sqlcore` as the shared SQL implementation)
- Enable shaping controls where needed:
  - `BaseConfig.Compression = cachecore.CompressionGzip` for larger payloads
  - `BaseConfig.MaxValueBytes = ...` to enforce payload budgets
  - `BaseConfig.EncryptionKey = ...` for at-rest value protection

Example optional-driver construction shape:

```go
cfg := rediscache.Config{
	BaseConfig: cachecore.BaseConfig{
		DefaultTTL: 10 * time.Minute,
		Prefix:     "billing:v1",
	},
	Addr: "127.0.0.1:6379",
}
store := rediscache.New(cfg)
c := cache.NewCache(store)
```

## Key Naming And Versioning

Use structured keys so invalidation and migrations are predictable:

- `{service}:{domain}:{entity}:{id}`
- include schema/version segment when payload shape can change:
  - `billing:v2:invoice:12345`

Guidelines:

- Keep keys deterministic and lowercase.
- Avoid unbounded user input directly in key names.
- Use `DeleteMany` for coordinated invalidation of known key sets.
- Bump key version when changing serialized payload semantics.

## TTL And Expiration Strategy

- Use shorter TTLs for frequently changing data.
- Use longer TTLs for stable reference data.
- Prefer explicit operation TTL for critical paths; fallback to default TTL for convenience APIs.

For precise operation semantics, see [Behavior Semantics](behavior-semantics.md).

## Miss-Storm Mitigation

Use layered mitigation rather than a single technique:

- Read-through helpers (`Remember*`) for lazy fill.
- Stale fallback (`RememberStale*`) for degraded upstream periods.
- Refresh-ahead (`RefreshAhead*`) for hot keys near expiry.
- Locking (`TryLock` / `Lock`) around expensive recomputation where needed.

### TTL Jitter Pattern

Add small random jitter to spread expirations for similarly written keys:

```go
base := 5 * time.Minute
jitter := time.Duration(rand.Int63n(int64(30 * time.Second)))
ttl := base + jitter
_ = c.Set("catalog:item:42", payload, ttl)
```

Guideline:

- Keep jitter bounded (for example 5-15% of base TTL).
- Use the same jitter strategy for batch writes of related keys.

## Rate Limiting Guidance

- Current helpers use fixed-window counters.
- On shared backends, counters are shared across instances.
- On local backends (for example memory), counters are process-local.

For client-facing APIs:

- Use `RateLimit` for header-friendly metadata:
  - `remaining`
  - `resetAt`

## Locking Guidance

- Use short TTL locks for idempotent jobs and cache rebuild gates.
- Always design critical work to finish within lock TTL or renew externally.
- `Unlock` removes lock key without owner token validation; avoid long-running lock ownership assumptions.

## Observability Patterns

Attach an observer to capture hit/miss/error/latency by operation and driver:

```go
type Observer interface {
	OnCacheOp(ctx context.Context, op string, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver)
}
```

Practical metrics pattern (Prometheus/OpenTelemetry):

```go
c = c.WithObserver(cache.ObserverFunc(func(ctx context.Context, op, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
	_ = ctx
	_ = key // do not use raw keys as metric labels (high cardinality)

	status := "ok"
	if err != nil {
		status = "error"
	}

	// cache_operations_total{op,driver,status}
	// cache_duration_seconds{op,driver}
	// cache_reads_total{op,driver,result="hit|miss"} for read-ish ops

	if op == "get" || op == "get_json" || op == "get_string" || op == "remember" || op == "remember_stale" || op == "refresh_ahead" {
		result := "miss"
		if hit {
			result = "hit"
		}
		_ = result
	}

	_ = status
	_ = dur
	_ = driver
}))
```

Recommended metrics:

- hit ratio by operation (`get`, `remember`, `remember_stale`, `refresh_ahead`)
- latency percentiles by driver and op
- error counts by op/driver
- lock contention and timeout counts
- rate-limit allowed vs denied counts

Recommended logging:

- sample slow operations
- include op, key namespace (not full sensitive key), driver, duration, error

## Rollout Checklist

- Validate with integration tests for selected production drivers (from the `integration` module):
  - `cd integration && INTEGRATION_DRIVER=all go test -tags=integration ./all`
- Run with race detector in CI for contention-sensitive paths.
- Load test hot-key behavior (`Remember*`, `RefreshAhead*`, locks).
- Monitor hit ratio and upstream dependency load before/after rollout.
