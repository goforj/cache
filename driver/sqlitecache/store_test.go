package sqlitecache

import (
	"context"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/cachetest"
)

// TestSQLiteStoreContract applies the reusable store contract to an in-memory SQLite database.
func TestSQLiteStoreContract(t *testing.T) {
	store, err := New(Config{
		BaseConfig: cachecore.BaseConfig{DefaultTTL: time.Second, Prefix: "contract"},
		DSN:        "file::memory:?cache=shared",
		Table:      "cache_entries",
	})
	if err != nil {
		t.Fatalf("sqlite store create failed: %v", err)
	}

	cachetest.RunStoreContract(t, store, cachetest.Options{
		CaseName: t.Name(),
		TTL:      50 * time.Millisecond,
		TTLWait:  120 * time.Millisecond,
	})
}

// TestSQLiteStoreAppliesShapingConfiguration verifies wrapper configs forward every BaseConfig field.
func TestSQLiteStoreAppliesShapingConfiguration(t *testing.T) {
	store, err := New(Config{
		BaseConfig: cachecore.BaseConfig{
			Compression:   cachecore.CompressionGzip,
			EncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
		},
		DSN:   "file::memory:?cache=shared",
		Table: "cache_shaping",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := store.Set(ctx, "secret", []byte("payload"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := store.Get(ctx, "secret")
	if err != nil || !ok || string(got) != "payload" {
		t.Fatalf("shaped round trip: got=%q ok=%v err=%v", got, ok, err)
	}
}
