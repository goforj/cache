package cache

import (
	"context"
	"errors"

	"github.com/goforj/cache/cachecore"
)

func newStoreForDriver(ctx context.Context, driver cachecore.Driver, cfg StoreConfig) cachecore.Store {
	cfg = cfg.withDefaults()
	var store cachecore.Store
	switch driver {
	case cachecore.DriverNull:
		store = newNullStore()
	case cachecore.DriverFile:
		store = newFileStore(cfg.FileDir, cfg.DefaultTTL)
	case cachecore.DriverMemory:
		store = newMemoryStore(cfg.DefaultTTL, cfg.MemoryCleanupInterval)
	case cachecore.DriverMemcached:
		return &errorStore{driver: cachecore.DriverMemcached, err: errors.New("root memcached driver removed; use github.com/goforj/cache/driver/memcachedcache")}
	case cachecore.DriverDynamo:
		return &errorStore{driver: cachecore.DriverDynamo, err: errors.New("root dynamo driver removed; use github.com/goforj/cache/driver/dynamocache")}
	case cachecore.DriverSQL:
		return &errorStore{driver: cachecore.DriverSQL, err: errors.New("root sql driver removed; use github.com/goforj/cache/driver/sqlitecache, /driver/postgrescache, or /driver/mysqlcache")}
	case cachecore.DriverRedis:
		return &errorStore{driver: cachecore.DriverRedis, err: errors.New("root redis driver removed; use github.com/goforj/cache/driver/rediscache")}
	case cachecore.DriverNATS:
		return &errorStore{driver: cachecore.DriverNATS, err: errors.New("root nats driver removed; use github.com/goforj/cache/driver/natscache")}
	default:
		return &errorStore{driver: driver, err: errors.New("unknown driver; use explicit root constructor or driver module")}
	}
	store = newShapingStore(store, cfg.Compression, cfg.MaxValueBytes)
	encStore, err := newEncryptingStore(store, cfg.EncryptionKey)
	if err != nil {
		return &errorStore{driver: driver, err: err}
	}
	return encStore
}

// NewMemoryStore is a convenience for an in-process store using defaults.
// @group Constructors
//
// Example: memory helper
//
//	ctx := context.Background()
//	store := cache.NewMemoryStore(ctx)
//	fmt.Println(store.Driver()) // memory
func NewMemoryStore(ctx context.Context) cachecore.Store {
	return newStoreForDriver(ctx, cachecore.DriverMemory, StoreConfig{})
}

// NewMemoryStoreWithConfig builds an in-process store using explicit root config.
// @group Constructors
//
// Example: memory helper with root config
//
//	ctx := context.Background()
//	store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL:  30 * time.Second,
//			Compression: cache.CompressionGzip,
//		},
//		MemoryCleanupInterval: 5 * time.Minute,
//	})
//	fmt.Println(store.Driver()) // memory
func NewMemoryStoreWithConfig(ctx context.Context, cfg StoreConfig) cachecore.Store {
	return newStoreForDriver(ctx, cachecore.DriverMemory, cfg)
}

// NewFileStore is a convenience for a filesystem-backed store.
// @group Constructors
//
// Example: file helper
//
//	ctx := context.Background()
//	store := cache.NewFileStore(ctx, "/tmp/my-cache")
//	fmt.Println(store.Driver()) // file
func NewFileStore(ctx context.Context, dir string) cachecore.Store {
	return newStoreForDriver(ctx, cachecore.DriverFile, StoreConfig{FileDir: dir})
}

// NewFileStoreWithConfig builds a filesystem-backed store using explicit root config.
// @group Constructors
//
// Example: file helper with root config
//
//	ctx := context.Background()
//	store := cache.NewFileStoreWithConfig(ctx, cache.StoreConfig{
//		BaseConfig: cachecore.BaseConfig{
//			EncryptionKey: []byte("01234567890123456789012345678901"),
//			MaxValueBytes: 4096,
//			Compression:   cache.CompressionGzip,
//		},
//		FileDir: "/tmp/my-cache",
//	})
//	fmt.Println(store.Driver()) // file
func NewFileStoreWithConfig(ctx context.Context, cfg StoreConfig) cachecore.Store {
	return newStoreForDriver(ctx, cachecore.DriverFile, cfg)
}

// NewNullStore is a no-op store useful for tests where caching should be disabled.
// @group Constructors
//
// Example: null helper
//
//	ctx := context.Background()
//	store := cache.NewNullStore(ctx)
//	fmt.Println(store.Driver()) // null
func NewNullStore(ctx context.Context) cachecore.Store {
	return newStoreForDriver(ctx, cachecore.DriverNull, StoreConfig{})
}

// NewNullStoreWithConfig builds a null store with shared wrappers (compression/encryption/limits).
// @group Constructors
//
// Example: null helper with shared wrappers enabled
//
//	ctx := context.Background()
//	store := cache.NewNullStoreWithConfig(ctx, cache.StoreConfig{
//		BaseConfig: cachecore.BaseConfig{
//			Compression:   cache.CompressionGzip,
//			MaxValueBytes: 1024,
//		},
//	})
//	fmt.Println(store.Driver()) // null
func NewNullStoreWithConfig(ctx context.Context, cfg StoreConfig) cachecore.Store {
	return newStoreForDriver(ctx, cachecore.DriverNull, cfg)
}
