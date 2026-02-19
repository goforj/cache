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
	if cfg.FileDir == "" {
		t.Fatalf("expected default file dir set")
	}
}

func TestStoreOptionsMutateConfig(t *testing.T) {
	var cfg StoreConfig
	cfg = WithDefaultTTL(time.Second)(cfg)
	cfg = WithMemoryCleanupInterval(2 * time.Second)(cfg)
	cfg = WithPrefix("svc")(cfg)
	cfg = WithFileDir("/tmp/opts")(cfg)
	cfg = WithMemcachedAddresses("127.0.0.1:11211")(cfg)
	client := newStubRedisClient()
	cfg = WithRedisClient(client)(cfg)

	if cfg.DefaultTTL != time.Second ||
		cfg.MemoryCleanupInterval != 2*time.Second ||
		cfg.Prefix != "svc" ||
		cfg.RedisClient != client ||
		cfg.FileDir != "/tmp/opts" ||
		len(cfg.MemcachedAddresses) != 1 ||
		cfg.MemcachedAddresses[0] != "127.0.0.1:11211" {
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

	file := NewFileStore(ctx, "/tmp/cache-file-test")
	if file.Driver() != DriverFile {
		t.Fatalf("expected file driver")
	}

	memStore := NewMemcachedStore(ctx, []string{"127.0.0.1:11211"})
	if memStore.Driver() != DriverMemcached {
		t.Fatalf("expected memcached driver")
	}
}
