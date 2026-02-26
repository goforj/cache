package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
)

func main() {
	// NewNullStoreWithConfig builds a null store with shared wrappers (compression/encryption/limits).

	// Example: null helper with shared wrappers enabled
	ctx := context.Background()
	store := cache.NewNullStoreWithConfig(ctx, cache.StoreConfig{
		BaseConfig: cachecore.BaseConfig{
			Compression:   cache.CompressionGzip,
			MaxValueBytes: 1024,
		},
	})
	fmt.Println(store.Driver()) // null
}
