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
func NewStore(ctx context.Context, cfg StoreConfig) Store {
	cfg = cfg.withDefaults()
	var store Store
	switch cfg.Driver {
	case DriverNull:
		store = newNullStore()
	case DriverFile:
		store = newFileStore(cfg.FileDir, cfg.DefaultTTL)
	case DriverMemory:
		store = newMemoryStore(cfg.DefaultTTL, cfg.MemoryCleanupInterval)
	case DriverMemcached:
		store = newMemcachedStore(cfg.MemcachedAddresses, cfg.DefaultTTL, cfg.Prefix)
	case DriverDynamo:
		var err error
		store, err = newDynamoStore(ctx, cfg)
		if err != nil {
			return &errorStore{driver: DriverDynamo, err: err}
		}
	case DriverSQL:
		var err error
		store, err = newSQLStore(cfg)
		if err != nil {
			return &errorStore{driver: DriverSQL, err: err}
		}
	case DriverRedis:
		store = newRedisStore(cfg.RedisClient, cfg.DefaultTTL, cfg.Prefix)
	default:
		store = newMemoryStore(cfg.DefaultTTL, cfg.MemoryCleanupInterval)
	}
	return newShapingStore(store, cfg.Compression, cfg.MaxValueBytes)
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

// NewNullStore is a no-op store useful for tests where caching should be disabled.
// @group Constructors
//
// Example: null helper
//
//	ctx := context.Background()
//	store := cache.NewNullStore(ctx)
//	fmt.Println(store.Driver()) // null
func NewNullStore(ctx context.Context, opts ...StoreOption) Store {
	return NewStoreWith(ctx, DriverNull, opts...)
}

// NewDynamoStore is a convenience for a DynamoDB-backed store.
// @group Constructors
//
// Example: dynamo helper (stub)
//
//	ctx := context.Background()
//	store := cache.NewDynamoStore(ctx, cache.StoreConfig{DynamoEndpoint: "http://localhost:8000"})
//	fmt.Println(store.Driver()) // dynamodb
func NewDynamoStore(ctx context.Context, cfg StoreConfig, opts ...StoreOption) Store {
	cfg.Driver = DriverDynamo
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return NewStore(ctx, cfg)
}

// NewSQLStore builds a SQL-backed store (postgres, mysql, sqlite).
// @group Constructors
//
// Example: sqlite helper
//
//	ctx := context.Background()
//	store := cache.NewSQLStore(ctx, "sqlite", "file:cache.db?cache=shared&mode=rwc", "cache_entries")
//	fmt.Println(store.Driver()) // sql
//
// Example: postgres helper
//
//	dsnPg := "postgres://user:pass@localhost:5432/app?sslmode=disable"
//	storePg := cache.NewSQLStore(ctx, "pgx", dsnPg, "cache_entries")
//	fmt.Println(storePg.Driver()) // sql
//
// Example: mysql helper
//
//	dsnMy := "user:pass@tcp(localhost:3306)/app?parseTime=true"
//	storeMy := cache.NewSQLStore(ctx, "mysql", dsnMy, "cache_entries")
//	fmt.Println(storeMy.Driver()) // sql
func NewSQLStore(ctx context.Context, driverName, dsn, table string, opts ...StoreOption) Store {
	cfg := StoreConfig{
		Driver:        DriverSQL,
		SQLDriverName: driverName,
		SQLDSN:        dsn,
		SQLTable:      table,
	}
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return NewStore(ctx, cfg)
}
