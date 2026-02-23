package cache_test

import (
	"context"
	"testing"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachetest"
)

func TestCachetestRunStoreContract_MemoryStore(t *testing.T) {
	store := cache.NewMemoryStore(context.Background())
	cachetest.RunStoreContract(t, store, cachetest.Options{})
}

func TestCachetestRunStoreContract_NullStore(t *testing.T) {
	store := cache.NewNullStore(context.Background())
	cachetest.RunStoreContract(t, store, cachetest.Options{NullSemantics: true})
}
