package cache

import (
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

func TestStoreConfigWithDefaults(t *testing.T) {
	cfg := (StoreConfig{}).withDefaults()

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

func TestStoreConfigWithDefaultsPreservesExplicitValues(t *testing.T) {
	cfg := (StoreConfig{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL:    time.Second,
			Prefix:        "svc",
			Compression:   CompressionGzip,
			MaxValueBytes: 1024,
			EncryptionKey: []byte("01234567890123456789012345678901"),
		},
		MemoryCleanupInterval: 2 * time.Second,
		FileDir:               "/tmp/cache-test",
	}).withDefaults()

	if cfg.DefaultTTL != time.Second {
		t.Fatalf("default ttl overwritten: %v", cfg.DefaultTTL)
	}
	if cfg.MemoryCleanupInterval != 2*time.Second {
		t.Fatalf("cleanup interval overwritten: %v", cfg.MemoryCleanupInterval)
	}
	if cfg.Prefix != "svc" {
		t.Fatalf("prefix overwritten: %q", cfg.Prefix)
	}
	if cfg.Compression != CompressionGzip {
		t.Fatalf("compression overwritten: %q", cfg.Compression)
	}
	if cfg.MaxValueBytes != 1024 {
		t.Fatalf("max value bytes overwritten: %d", cfg.MaxValueBytes)
	}
	if cfg.FileDir != "/tmp/cache-test" {
		t.Fatalf("file dir overwritten: %q", cfg.FileDir)
	}
	if len(cfg.EncryptionKey) == 0 {
		t.Fatalf("encryption key overwritten")
	}
}
