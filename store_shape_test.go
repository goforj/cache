package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

func TestShapingStoreGzipRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newShapingStore(newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval), CompressionGzip, 0)

	val := []byte("hello world, compress me")
	if err := store.Set(ctx, "k", val, time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	got, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(got) != string(val) {
		t.Fatalf("get failed: ok=%v err=%v val=%s", ok, err, string(got))
	}
}

func TestShapingStoreSizeLimit(t *testing.T) {
	ctx := context.Background()
	store := newShapingStore(newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval), CompressionNone, 5)
	err := store.Set(ctx, "k", []byte("toolong"), time.Minute)
	if !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("expected size error, got %v", err)
	}
	if _, err := store.Add(ctx, "k", []byte("toolong"), time.Minute); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("expected size error on add, got %v", err)
	}
}

func TestShapingStoreDecompressesOnlyWhenPrefixed(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval).(*memoryStore)
	mem.cache.Set("k", []byte("raw"), time.Minute)
	store := newShapingStore(mem, CompressionGzip, 0)

	got, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(got) != "raw" {
		t.Fatalf("unexpected get: ok=%v err=%v val=%s", ok, err, string(got))
	}
}

func TestShapingStoreUnsupportedCodec(t *testing.T) {
	if _, err := encodeValue("weird", 0, []byte("x")); !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("expected unsupported codec error")
	}
	if _, err := encodeValue(CompressionSnappy, 0, []byte("x")); !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("expected unsupported snappy codec error")
	}
}

func TestShapingStoreCompressedSizeLimit(t *testing.T) {
	// limit small enough to fail after gzip header is added.
	if _, err := encodeValue(CompressionGzip, 2, []byte("x")); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestDecodeValueCorrupt(t *testing.T) {
	_, err := decodeValue([]byte("CMP1gnotgzip"))
	if !errors.Is(err, ErrCorruptCompression) {
		t.Fatalf("expected corrupt error, got %v", err)
	}
	if _, err := decodeValue([]byte("CMP1z???")); !errors.Is(err, ErrUnsupportedCodec) {
		t.Fatalf("expected unsupported codec, got %v", err)
	}
}

func TestFactoryAppliesShaping(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStoreWithConfig(ctx, StoreConfig{
		BaseConfig: cachecore.BaseConfig{
			Compression:   CompressionGzip,
			MaxValueBytes: 1024,
		},
	})
	if _, ok := store.(*shapingStore); !ok {
		t.Fatalf("expected shaping store wrapper")
	}
}

func TestShapingStoreDelegatesMutations(t *testing.T) {
	ctx := context.Background()
	base := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store := newShapingStore(base, CompressionNone, 1)

	if _, err := store.Increment(ctx, "num", 1, time.Minute); err != nil {
		t.Fatalf("increment failed: %v", err)
	}
	if _, err := store.Decrement(ctx, "num", 1, time.Minute); err != nil {
		t.Fatalf("decrement failed: %v", err)
	}
	if err := store.Delete(ctx, "num"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := store.DeleteMany(ctx, "a", "b"); err != nil {
		t.Fatalf("delete many failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}

func TestShapingStorePassThroughWhenDisabled(t *testing.T) {
	base := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store := newShapingStore(base, CompressionNone, 0)
	if store != base {
		t.Fatalf("expected pass-through store when shaping disabled")
	}
}

func TestShapingStoreGetMissAndError(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store := newShapingStore(mem, CompressionGzip, 1)
	if _, ok, err := store.Get(ctx, "missing"); err != nil || ok {
		t.Fatalf("expected miss ok=%v err=%v", ok, err)
	}

	bad := &errorStore{err: errors.New("boom")}
	wrapped := newShapingStore(bad, CompressionGzip, 1)
	if _, _, err := wrapped.Get(ctx, "any"); err == nil {
		t.Fatalf("expected inner error")
	}
}

func TestShapingStoreAddSuccess(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryStore(defaultCacheTTL, defaultMemoryCleanupInterval)
	store := newShapingStore(mem, CompressionGzip, 1024)
	created, err := store.Add(ctx, "fresh", []byte("value"), time.Minute)
	if err != nil || !created {
		t.Fatalf("expected add success, created=%v err=%v", created, err)
	}
}
