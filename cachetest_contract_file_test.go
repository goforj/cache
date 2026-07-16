package cache_test

import (
	"context"
	"testing"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachetest"
)

// TestCachetestRunStoreContract_FileStore applies the reusable store contract to the file driver.
func TestCachetestRunStoreContract_FileStore(t *testing.T) {
	store := cache.NewFileStore(context.Background(), t.TempDir())
	cachetest.RunStoreContract(t, store, cachetest.Options{})
}

// TestCachetestRunInspectorContract_FileStore applies the reusable inspector contract to the file driver.
func TestCachetestRunInspectorContract_FileStore(t *testing.T) {
	store := cache.NewFileStore(context.Background(), t.TempDir())
	cachetest.RunInspectorContract(t, store)
}
