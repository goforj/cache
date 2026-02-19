package cache

import (
	"os"
	"path/filepath"
	"time"
)

const (
	defaultCachePrefix           = "app"
	defaultCacheTTL              = 5 * time.Minute
	defaultMemoryCleanupInterval = 10 * time.Minute
)

func defaultFileDir() string {
	return filepath.Join(os.TempDir(), "cache-file")
}

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

	// FileDir controls where file driver stores cache entries.
	FileDir string

	// MemcachedAddresses are required when DriverMemcached is used (host:port).
	MemcachedAddresses []string

	// DynamoEndpoint is the HTTP endpoint for DynamoDB (e.g., localstack/dynamodb-local).
	DynamoEndpoint string
	// DynamoRegion sets the AWS region used for signing.
	DynamoRegion string
	// DynamoTable is the table name used for cache entries.
	DynamoTable string
	// DynamoClient allows injecting a preconfigured DynamoDB client.
	DynamoClient DynamoAPI

	// SQLDriverName is the database/sql driver name: "postgres", "mysql", or "sqlite".
	SQLDriverName string
	// SQLDSN is the driver-specific DSN.
	SQLDSN string
	// SQLTable is the table name for the SQL driver.
	SQLTable string

	// Compression controls value compression before storage.
	Compression CompressionCodec
	// MaxValueBytes enforces a maximum value size (post-compression) when > 0.
	MaxValueBytes int
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
	if c.FileDir == "" {
		c.FileDir = defaultFileDir()
	}
	if len(c.MemcachedAddresses) == 0 && c.Driver == DriverMemcached {
		c.MemcachedAddresses = []string{"127.0.0.1:11211"}
	}
	if c.DynamoRegion == "" {
		c.DynamoRegion = "us-east-1"
	}
	if c.DynamoTable == "" {
		c.DynamoTable = "cache_entries"
	}
	if c.SQLTable == "" {
		c.SQLTable = "cache_entries"
	}
	return c
}
