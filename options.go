package cache

import "time"

// StoreOption mutates StoreConfig when constructing a store.
type StoreOption func(StoreConfig) StoreConfig

// WithDefaultTTL overrides the fallback TTL used when ttl <= 0.
// @group Options
//
// Example: override default TTL
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithDefaultTTL(30*time.Second))
//	fmt.Println(store.Driver()) // memory
func WithDefaultTTL(ttl time.Duration) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DefaultTTL = ttl
		return cfg
	}
}

// WithMemoryCleanupInterval overrides the sweep interval for the memory driver.
// @group Options
//
// Example: custom memory sweep
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithMemoryCleanupInterval(5*time.Minute))
//	fmt.Println(store.Driver()) // memory
func WithMemoryCleanupInterval(interval time.Duration) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.MemoryCleanupInterval = interval
		return cfg
	}
}

// WithPrefix sets the key prefix for shared backends (e.g., redis).
// @group Options
//
// Example: prefix keys
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverRedis, cache.WithPrefix("svc"))
//	fmt.Println(store.Driver()) // redis
func WithPrefix(prefix string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.Prefix = prefix
		return cfg
	}
}

// WithRedisClient sets the redis client; required when using DriverRedis.
// @group Options
//
// Example: inject redis client
//
//	ctx := context.Background()
//	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
//	store := cache.NewStoreWith(ctx, cache.DriverRedis, cache.WithRedisClient(rdb))
//	fmt.Println(store.Driver()) // redis
func WithRedisClient(client RedisClient) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.RedisClient = client
		return cfg
	}
}

// WithNATSKeyValue sets the NATS JetStream KeyValue bucket; required when using DriverNATS.
// @group Options
//
// Example: inject NATS key-value bucket
//
//	ctx := context.Background()
//	var kv cache.NATSKeyValue // provided by your NATS setup
//	store := cache.NewStoreWith(ctx, cache.DriverNATS, cache.WithNATSKeyValue(kv))
//	fmt.Println(store.Driver()) // nats
func WithNATSKeyValue(kv NATSKeyValue) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.NATSKeyValue = kv
		return cfg
	}
}

// WithFileDir sets the directory used by the file driver.
// @group Options
//
// Example: set file dir
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverFile, cache.WithFileDir("/tmp/cache"))
//	fmt.Println(store.Driver()) // file
func WithFileDir(dir string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.FileDir = dir
		return cfg
	}
}

// WithMemcachedAddresses sets memcached server addresses (host:port).
// @group Options
//
// Example: memcached cluster
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverMemcached, cache.WithMemcachedAddresses("127.0.0.1:11211"))
//	fmt.Println(store.Driver()) // memcached
func WithMemcachedAddresses(addrs ...string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.MemcachedAddresses = append([]string{}, addrs...)
		return cfg
	}
}

// WithDynamoEndpoint sets the DynamoDB endpoint (useful for local testing).
// @group Options
//
// Example: dynamo local endpoint
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoEndpoint("http://localhost:8000"))
//	fmt.Println(store.Driver()) // dynamodb
func WithDynamoEndpoint(endpoint string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoEndpoint = endpoint
		return cfg
	}
}

// WithDynamoRegion sets the DynamoDB region for requests.
// @group Options
//
// Example: set dynamo region
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoRegion("us-west-2"))
//	fmt.Println(store.Driver()) // dynamodb
func WithDynamoRegion(region string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoRegion = region
		return cfg
	}
}

// WithDynamoTable sets the table used by the DynamoDB driver.
// @group Options
//
// Example: custom dynamo table
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoTable("cache_entries"))
//	fmt.Println(store.Driver()) // dynamodb
func WithDynamoTable(table string) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoTable = table
		return cfg
	}
}

// WithDynamoClient injects a pre-built DynamoDB client.
// @group Options
//
// Example: inject dynamo client
//
//	ctx := context.Background()
//	var client cache.DynamoAPI // assume already configured
//	store := cache.NewStoreWith(ctx, cache.DriverDynamo, cache.WithDynamoClient(client))
//	fmt.Println(store.Driver()) // dynamodb
func WithDynamoClient(client DynamoAPI) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.DynamoClient = client
		return cfg
	}
}

// WithSQL configures the SQL driver (driver name + DSN + optional table).
// @group Options
//
// Example: sqlite via options
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverSQL,
//		cache.WithSQL("sqlite", "file::memory:?cache=shared", "cache_entries"),
//		cache.WithPrefix("svc"),
//	)
//	fmt.Println(store.Driver()) // sql
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

// WithCompression enables value compression using the chosen codec.
// @group Options
//
// Example: gzip compression
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithCompression(cache.CompressionGzip))
//	fmt.Println(store.Driver()) // memory
func WithCompression(codec CompressionCodec) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.Compression = codec
		return cfg
	}
}

// WithMaxValueBytes sets a per-entry size limit (0 disables the check).
// @group Options
//
// Example: limit value size
//
//	ctx := context.Background()
//	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithMaxValueBytes(1024))
//	fmt.Println(store.Driver()) // memory
func WithMaxValueBytes(limit int) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.MaxValueBytes = limit
		return cfg
	}
}

// WithEncryptionKey enables at-rest encryption using the provided AES key (16/24/32 bytes).
// @group Options
//
// Example: encrypt values
//
//	ctx := context.Background()
//	key := []byte("01234567890123456789012345678901")
//	store := cache.NewStoreWith(ctx, cache.DriverFile, cache.WithEncryptionKey(key))
//	fmt.Println(store.Driver()) // file
func WithEncryptionKey(key []byte) StoreOption {
	return func(cfg StoreConfig) StoreConfig {
		cfg.EncryptionKey = key
		return cfg
	}
}
