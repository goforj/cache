package cache_test

import (
	"context"
	"testing"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachetest"
)

// TestCachetestRunStoreContract_MemoryStore applies the reusable store contract to the memory driver.
func TestCachetestRunStoreContract_MemoryStore(t *testing.T) {
	store := cache.NewMemoryStore(context.Background())
	cachetest.RunStoreContract(t, store, cachetest.Options{})
}

// TestCachetestRunInspectorContract_MemoryStore applies the reusable inspector contract to the memory driver.
func TestCachetestRunInspectorContract_MemoryStore(t *testing.T) {
	store := cache.NewMemoryStore(context.Background())
	cachetest.RunInspectorContract(t, store)
}

// TestCachetestRunStoreContract_NullStore applies the reusable store contract to the null driver.
func TestCachetestRunStoreContract_NullStore(t *testing.T) {
	store := cache.NewNullStore(context.Background())
	cachetest.RunStoreContract(t, store, cachetest.Options{NullSemantics: true})
}
