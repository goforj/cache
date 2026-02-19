package cache

import "time"

const (
	defaultCachePrefix           = "app"
	defaultCacheTTL              = 5 * time.Minute
	defaultMemoryCleanupInterval = 10 * time.Minute
)

// StoreConfig controls how a Store is constructed.
type StoreConfig struct {
	Driver Driver

	// DefaultTTL is used when a call provides ttl <= 0.
	DefaultTTL time.Duration

	// MemoryCleanupInterval controls in-process cache eviction.
	MemoryCleanupInterval time.Duration

	// Prefix is used by shared backends (e.g. redis keys).
	Prefix string

	// RedisClient is required when DriverRedis is used.
	RedisClient RedisClient
}

func (c StoreConfig) withDefaults() StoreConfig {
	if c.Driver == "" {
		c.Driver = DriverMemory
	}
	if c.DefaultTTL <= 0 {
		c.DefaultTTL = defaultCacheTTL
	}
	if c.MemoryCleanupInterval <= 0 {
		c.MemoryCleanupInterval = defaultMemoryCleanupInterval
	}
	if c.Prefix == "" {
		c.Prefix = defaultCachePrefix
	}
	return c
}
