package cache

import (
	"context"
	"testing"
	"time"
)

func TestStoreConfigWithDefaults(t *testing.T) {
	cfg := (StoreConfig{}).withDefaults()
	if cfg.Driver != DriverMemory {
		t.Fatalf("expected default driver memory, got %s", cfg.Driver)
	}
	if cfg.DefaultTTL != defaultCacheTTL {
		t.Fatalf("unexpected default ttl: %v", cfg.DefaultTTL)
	}
	if cfg.MemoryCleanupInterval != defaultMemoryCleanupInterval {
		t.Fatalf("unexpected cleanup interval: %v", cfg.MemoryCleanupInterval)
	}
	if cfg.Prefix != defaultCachePrefix {
		t.Fatalf("unexpected prefix: %s", cfg.Prefix)
	}
}

func TestStoreOptionsMutateConfig(t *testing.T) {
	var cfg StoreConfig
	cfg = WithDefaultTTL(time.Second)(cfg)
	cfg = WithMemoryCleanupInterval(2 * time.Second)(cfg)
	cfg = WithPrefix("svc")(cfg)
	client := newStubRedisClient()
	cfg = WithRedisClient(client)(cfg)

	if cfg.DefaultTTL != time.Second ||
		cfg.MemoryCleanupInterval != 2*time.Second ||
		cfg.Prefix != "svc" ||
		cfg.RedisClient != client {
		t.Fatalf("options did not apply correctly: %+v", cfg)
	}
}

func TestFactoryHelpers(t *testing.T) {
	ctx := context.Background()
	mem := NewStoreWith(ctx, DriverMemory)
	if mem.Driver() != DriverMemory {
		t.Fatalf("expected memory driver")
	}
	if NewMemoryStore(ctx).Driver() != DriverMemory {
		t.Fatalf("expected memory helper driver")
	}

	redisClient := newStubRedisClient()
	rds := NewRedisStore(ctx, redisClient)
	if rds.Driver() != DriverRedis {
		t.Fatalf("expected redis driver")
	}
}
