package bench

import (
	"github.com/goforj/cache/cachecore"
)

type benchConfig struct {
	BaseConfig    cachecore.BaseConfig
	FileDir       string
	NATSBucketTTL bool

	DynamoEndpoint string
	DynamoRegion   string
	DynamoTable    string
	DynamoClient   any
}

type benchStoreOption func(benchConfig) benchConfig

// benchWithPrefix overrides the cache namespace for local benchmark runs.
func benchWithPrefix(prefix string) benchStoreOption {
	return func(cfg benchConfig) benchConfig {
		cfg.BaseConfig.Prefix = prefix
		return cfg
	}
}

// benchWithNATSBucketTTL overrides the JetStream bucket TTL for local benchmark runs.
func benchWithNATSBucketTTL(enabled bool) benchStoreOption {
	return func(cfg benchConfig) benchConfig {
		cfg.NATSBucketTTL = enabled
		return cfg
	}
}
