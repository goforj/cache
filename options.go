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

// WithFileDir sets the directory used by the file driver.
func WithFileDir(dir string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.FileDir = dir
		return cfg
	}
}

// WithMemcachedAddresses sets memcached server addresses (host:port).
func WithMemcachedAddresses(addrs ...string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.MemcachedAddresses = append([]string{}, addrs...)
		return cfg
	}
}

// WithDynamoEndpoint sets the DynamoDB endpoint (useful for local testing).
func WithDynamoEndpoint(endpoint string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoEndpoint = endpoint
		return cfg
	}
}

// WithDynamoRegion sets the DynamoDB region for requests.
func WithDynamoRegion(region string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoRegion = region
		return cfg
	}
}

// WithDynamoTable sets the table used by the DynamoDB driver.
func WithDynamoTable(table string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoTable = table
		return cfg
	}
}

// WithDynamoClient injects a pre-built DynamoDB client.
func WithDynamoClient(client DynamoAPI) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoClient = client
		return cfg
	}
}

// WithSQL configures the SQL driver (driver name + DSN + optional table).
func WithSQL(driverName, dsn, table string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.SQLDriverName = driverName
		cfg.SQLDSN = dsn
		if table != "" {
			cfg.SQLTable = table
		}
		return cfg
	}
}
