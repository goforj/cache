package cache_test

import (
	"context"
	"testing"

	"github.com/goforj/cache"
	"github.com/goforj/cache/cachetest"
)

func TestCachetestRunStoreContract_FileStore(t *testing.T) {
	store := cache.NewFileStore(context.Background(), t.TempDir())
	cachetest.RunStoreContract(t, store, cachetest.Options{})
}
