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
