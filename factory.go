package cache

import "context"

// NewStore returns a concrete store for the requested driver.
// Caller is responsible for providing any driver-specific dependencies.
func NewStore(_ context.Context, cfg StoreConfig) Store {
	cfg = cfg.withDefaults()
	switch cfg.Driver {
	case DriverRedis:
		return newRedisStore(cfg.RedisClient, cfg.DefaultTTL, cfg.Prefix)
	default:
		return newMemoryStore(cfg.DefaultTTL, cfg.MemoryCleanupInterval)
	}
}
