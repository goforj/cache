//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Example: gzip compression via explicit StoreConfig.
	ctx := context.Background()
	store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
		BaseConfig: cachecore.BaseConfig{Compression: cache.CompressionGzip},
	})
	fmt.Println(store.Driver()) // memory
}
