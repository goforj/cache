package cache

import "context"

// NewStore returns a concrete store for the requested driver.
// Caller is responsible for providing any driver-specific dependencies.
// @group Constructors
//
// Example: select driver explicitly
//
//	ctx := context.Background()
//	store := cache.NewStore(ctx, cache.StoreConfig{
//		Driver: cache.DriverMemory,
//	})
//	_ = store
func NewStore(_ context.Context, cfg StoreConfig) Store {
	cfg = cfg.withDefaults()
	switch cfg.Driver {
	case DriverRedis:
		return newRedisStore(cfg.RedisClient, cfg.DefaultTTL, cfg.Prefix)
	default:
		return newMemoryStore(cfg.DefaultTTL, cfg.MemoryCleanupInterval)
	}
}

// NewStoreWith builds a store using a driver and a set of functional options.
// Required data (e.g., Redis client) must be provided via options when needed.
// @group Constructors
//
// Example: memory store (options)
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverMemory)
//	_ = store
//
// Example: redis store (options)
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverRedis,
//		cache.WithRedisClient(redisClient),
//		cache.WithPrefix("app"),
//		cache.WithDefaultTTL(5*time.Minute),
//	)
//	_ = store
func NewStoreWith(ctx context.Context, driver Driver, opts ...StoreOption) Store {
	cfg := StoreConfig{Driver: driver}
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return NewStore(ctx, cfg)
}

// NewMemoryStore is a convenience for an in-process store with optional overrides.
// @group Constructors
//
// Example: memory helper
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	_ = store
func NewMemoryStore(ctx context.Context, opts ...StoreOption) Store {
	return NewStoreWith(ctx, DriverMemory, opts...)
}

// NewRedisStore is a convenience for a redis-backed store. Redis client is required.
// @group Constructors
//
// Example: redis helper
//
//	ctx := context.Background()
//	store := cache.NewRedisStore(ctx, redisClient, cache.WithPrefix("app"))
//	_ = store
func NewRedisStore(ctx context.Context, client RedisClient, opts ...StoreOption) Store {
	return NewStoreWith(ctx, DriverRedis, append([]StoreOption{WithRedisClient(client)}, opts...)...)
}
