package cache

import (
	"context"
	"os"
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
	if cfg.Compression != CompressionNone {
		t.Fatalf("expected default compression none")
	}
}

func TestStoreOptionsMutateConfig(t *testing.T) {
	var cfg StoreConfig
	cfg = WithDefaultTTL(time.Second)(cfg)
	cfg = WithMemoryCleanupInterval(2 * time.Second)(cfg)
	cfg = WithPrefix("svc")(cfg)
	cfg = WithFileDir("/tmp/opts")(cfg)
	cfg = WithMemcachedAddresses("127.0.0.1:11211")(cfg)
	cfg = WithDynamoEndpoint("http://localhost:8000")(cfg)
	cfg = WithDynamoRegion("eu-west-1")(cfg)
	cfg = WithDynamoTable("tbl")(cfg)
	cfg = WithDynamoClient(&dynStub{})(cfg)
	cfg = WithSQL("sqlite", "file::memory:?cache=shared", "cache_entries")(cfg)
	client := newStubRedisClient()
	cfg = WithRedisClient(client)(cfg)
	natsKV := newStubNATSKeyValue("bucket")
	cfg = WithNATSKeyValue(natsKV)(cfg)

	if cfg.DefaultTTL != time.Second ||
		cfg.MemoryCleanupInterval != 2*time.Second ||
		cfg.Prefix != "svc" ||
		cfg.RedisClient != client ||
		cfg.NATSKeyValue != natsKV ||
		cfg.FileDir != "/tmp/opts" ||
		len(cfg.MemcachedAddresses) != 1 ||
		cfg.MemcachedAddresses[0] != "127.0.0.1:11211" ||
		cfg.DynamoEndpoint != "http://localhost:8000" ||
		cfg.DynamoRegion != "eu-west-1" ||
		cfg.DynamoTable != "tbl" ||
		cfg.DynamoClient == nil ||
		cfg.SQLDriverName != "sqlite" ||
		cfg.SQLDSN != "file::memory:?cache=shared" ||
		cfg.SQLTable != "cache_entries" {
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
	natsStore := NewNATSStore(ctx, newStubNATSKeyValue("bucket"))
	if natsStore.Driver() != DriverNATS {
		t.Fatalf("expected nats driver")
	}

	file := NewFileStore(ctx, "/tmp/cache-file-test")
	if file.Driver() != DriverFile {
		t.Fatalf("expected file driver")
	}

	memStore := NewMemcachedStore(ctx, []string{"127.0.0.1:11211"})
	if memStore.Driver() != DriverMemcached {
		t.Fatalf("expected memcached driver")
	}

	null := NewNullStore(ctx)
	if null.Driver() != DriverNull {
		t.Fatalf("expected null driver")
	}

	dyn := NewDynamoStore(ctx, StoreConfig{})
	if dyn.Driver() != DriverDynamo {
		t.Fatalf("expected dynamo driver")
	}

	sqlStore := NewSQLStore(ctx, "sqlite", "file::memory:?cache=shared", "cache_entries")
	if sqlStore.Driver() != DriverSQL {
		t.Fatalf("expected sql driver")
	}
}

func TestNewStoreFallsBackToMemory(t *testing.T) {
	ctx := context.Background()
	store := NewStore(ctx, StoreConfig{Driver: Driver("unknown")})
	if store.Driver() != DriverMemory {
		t.Fatalf("expected fallback to memory driver, got %s", store.Driver())
	}
}

func TestNewDynamoStoreErrorReturnsErrorStore(t *testing.T) {
	ctx := context.Background()
	empty := createEmptyAWSConfig(t)
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)

	store := NewDynamoStore(ctx, StoreConfig{})
	if es, ok := store.(*errorStore); !ok || es.driver != DriverDynamo {
		t.Fatalf("expected errorStore for dynamo failure")
	}
}

func TestNewSQLStoreErrorReturnsErrorStore(t *testing.T) {
	ctx := context.Background()
	store := NewSQLStore(ctx, "", "", "")
	if es, ok := store.(*errorStore); !ok || es.driver != DriverSQL {
		t.Fatalf("expected errorStore for sql failure")
	}
}

func createEmptyAWSConfig(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "empty-aws-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestNewDynamoStoreAppliesOptions(t *testing.T) {
	ctx := context.Background()
	store := NewDynamoStore(ctx, StoreConfig{
		DynamoClient: newDynStub(),
		DynamoTable:  "tbl",
	}, WithPrefix("opt"))
	if store.Driver() != DriverDynamo {
		t.Fatalf("expected dynamo driver")
	}
}

func TestNewSQLStoreAppliesOptions(t *testing.T) {
	ctx := context.Background()
	store := NewSQLStore(ctx, "sqlite", "file::memory:?cache=shared", "tbl", WithPrefix("opt"))
	if store.Driver() != DriverSQL {
		t.Fatalf("expected sql driver")
	}
}

func TestStoreConfigWithDefaultsMemcached(t *testing.T) {
	cfg := (StoreConfig{Driver: DriverMemcached}).withDefaults()
	if len(cfg.MemcachedAddresses) == 0 {
		t.Fatalf("expected default memcached address")
	}
	if cfg.DynamoRegion == "" || cfg.DynamoTable == "" {
		t.Fatalf("expected dynamo defaults set")
	}
}
