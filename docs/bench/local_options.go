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

func benchWithPrefix(prefix string) benchStoreOption {
	return func(cfg benchConfig) benchConfig {
		cfg.Prefix = prefix
		return cfg
	}
}

func benchWithNATSBucketTTL(enabled bool) benchStoreOption {
	return func(cfg benchConfig) benchConfig {
		cfg.NATSBucketTTL = enabled
		return cfg
	}
}
