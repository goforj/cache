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
//	fmt.Println(store.Driver()) // memory
func NewStore(_ context.Context, cfg StoreConfig) Store {
	cfg = cfg.withDefaults()
	switch cfg.Driver {
	case DriverRedis:
		return newRedisStore(cfg.RedisClient, cfg.DefaultTTL, cfg.Prefix)
	case DriverMemcached:
		return newMemcachedStore(cfg.MemcachedAddresses, cfg.DefaultTTL, cfg.Prefix)
	case DriverFile:
		return newFileStore(cfg.FileDir, cfg.DefaultTTL)
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
//	fmt.Println(store.Driver()) // memory
//
// Example: redis store (options)
//
//	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
//	store = cache.NewStoreWith(ctx, cache.DriverRedis,
//		cache.WithRedisClient(redisClient),
//		cache.WithPrefix("app"),
//		cache.WithDefaultTTL(5*time.Minute),
//	)
//	fmt.Println(store.Driver()) // redis
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
//	fmt.Println(store.Driver()) // memory
func NewMemoryStore(ctx context.Context, opts ...StoreOption) Store {
	return NewStoreWith(ctx, DriverMemory, opts...)
}

// NewRedisStore is a convenience for a redis-backed store. Redis client is required.
// @group Constructors
//
// Example: redis helper
//
//	ctx := context.Background()
//	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
//	store := cache.NewRedisStore(ctx, redisClient, cache.WithPrefix("app"))
//	fmt.Println(store.Driver()) // redis
func NewRedisStore(ctx context.Context, client RedisClient, opts ...StoreOption) Store {
	return NewStoreWith(ctx, DriverRedis, append([]StoreOption{WithRedisClient(client)}, opts...)...)
}

// NewFileStore is a convenience for a filesystem-backed store.
// @group Constructors
//
// Example: file helper
//
//	ctx := context.Background()
//	store := cache.NewFileStore(ctx, "/tmp/my-cache")
//	fmt.Println(store.Driver()) // file
func NewFileStore(ctx context.Context, dir string, opts ...StoreOption) Store {
	return NewStoreWith(ctx, DriverFile, append([]StoreOption{WithFileDir(dir)}, opts...)...)
}

// NewMemcachedStore is a convenience for a memcached-backed store.
// @group Constructors
//
// Example: memcached helper
//
//	ctx := context.Background()
//	store := cache.NewMemcachedStore(ctx, []string{"127.0.0.1:11211"})
//	fmt.Println(store.Driver()) // memcached
func NewMemcachedStore(ctx context.Context, addrs []string, opts ...StoreOption) Store {
	return NewStoreWith(ctx, DriverMemcached, append([]StoreOption{WithMemcachedAddresses(addrs...)}, opts...)...)
}
