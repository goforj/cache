package cache

import (
	"context"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// benchmarkPayload keeps typed helper measurements representative of small application values.
type benchmarkPayload struct {
	Name string `json:"name"`
}

// BenchmarkCacheGetTyped compares the compatibility function and receiver method read paths.
func BenchmarkCacheGetTyped(b *testing.B) {
	c := NewCache(NewMemoryStore(context.Background()))
	if err := c.Set("key", benchmarkPayload{Name: "Ada"}, time.Hour); err != nil {
		b.Fatalf("seed cache: %v", err)
	}

	b.Run("Function", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, ok, err := Get[benchmarkPayload](c, "key"); err != nil || !ok {
				b.Fatalf("Get: ok=%v err=%v", ok, err)
			}
		}
	})
	b.Run("Method", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, ok, err := c.Get[benchmarkPayload]("key"); err != nil || !ok {
				b.Fatalf("Get: ok=%v err=%v", ok, err)
			}
		}
	})
}

// BenchmarkCacheSetTyped compares the compatibility function and receiver method write paths.
func BenchmarkCacheSetTyped(b *testing.B) {
	c := NewCache(NewMemoryStore(context.Background()))
	value := benchmarkPayload{Name: "Ada"}

	b.Run("Function", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if err := Set(c, "key", value, time.Hour); err != nil {
				b.Fatalf("Set: %v", err)
			}
		}
	})
	b.Run("Method", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if err := c.Set("key", value, time.Hour); err != nil {
				b.Fatalf("Set: %v", err)
			}
		}
	})
}

// BenchmarkCacheGetBytes measures the primary facade read path without observers.
func BenchmarkCacheGetBytes(b *testing.B) {
	cache := NewCache(NewMemoryStore(context.Background()))
	if err := cache.SetBytes("key", []byte("value"), time.Hour); err != nil {
		b.Fatalf("seed cache: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, ok, err := cache.GetBytes("key"); err != nil || !ok {
			b.Fatalf("GetBytes: ok=%v err=%v", ok, err)
		}
	}
}

// BenchmarkCacheGetBytesParallel measures concurrent access to the in-process backend.
func BenchmarkCacheGetBytesParallel(b *testing.B) {
	cache := NewCache(NewMemoryStore(context.Background()))
	if err := cache.SetBytes("key", []byte("value"), time.Hour); err != nil {
		b.Fatalf("seed cache: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, ok, err := cache.GetBytes("key"); err != nil || !ok {
				b.Fatalf("GetBytes: ok=%v err=%v", ok, err)
			}
		}
	})
}

// BenchmarkMemoStoreHit measures the memoized read path after its first backend lookup.
func BenchmarkMemoStoreHit(b *testing.B) {
	ctx := context.Background()
	base := NewMemoryStore(ctx)
	if err := base.Set(ctx, "key", []byte("value"), time.Hour); err != nil {
		b.Fatalf("seed store: %v", err)
	}
	store := NewMemoStore(base)
	if _, ok, err := store.Get(ctx, "key"); err != nil || !ok {
		b.Fatalf("prime memo: ok=%v err=%v", ok, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, ok, err := store.Get(ctx, "key"); err != nil || !ok {
			b.Fatalf("memo Get: ok=%v err=%v", ok, err)
		}
	}
}

// BenchmarkShapedStoreGet measures the shared compression and encryption read path.
func BenchmarkShapedStoreGet(b *testing.B) {
	ctx := context.Background()
	store, err := cachecore.WrapStore(NewMemoryStore(ctx), cachecore.BaseConfig{
		Compression:   cachecore.CompressionGzip,
		EncryptionKey: []byte("0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		b.Fatalf("wrap store: %v", err)
	}
	if err := store.Set(ctx, "key", []byte("value"), time.Hour); err != nil {
		b.Fatalf("seed store: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, ok, err := store.Get(ctx, "key"); err != nil || !ok {
			b.Fatalf("shaped Get: ok=%v err=%v", ok, err)
		}
	}
}
