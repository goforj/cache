//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"time"
)

func main() {
	// NewMemoryStoreWithConfig builds an in-process store using explicit root config.

	// Example: memory helper with root config
	ctx := context.Background()
	store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL:  30 * time.Second,
			Compression: cache.CompressionGzip,
		},
		MemoryCleanupInterval: 5 * time.Minute,
	})
	fmt.Println(store.Driver()) // memory
}
