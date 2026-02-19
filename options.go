package cache

import "time"

// StoreOption mutates StoreConfig when constructing a store.
type StoreOption func(StoreConfig) StoreConfig

// WithDefaultTTL overrides the fallback TTL used when ttl <= 0.
func WithDefaultTTL(ttl time.Duration) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DefaultTTL = ttl
		return cfg
	}
}

// WithMemoryCleanupInterval overrides the sweep interval for the memory driver.
func WithMemoryCleanupInterval(interval time.Duration) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.MemoryCleanupInterval = interval
		return cfg
	}
}

// WithPrefix sets the key prefix for shared backends (e.g., redis).
func WithPrefix(prefix string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.Prefix = prefix
		return cfg
	}
}

// WithRedisClient sets the redis client; required when using DriverRedis.
func WithRedisClient(client RedisClient) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.RedisClient = client
		return cfg
	}
}
