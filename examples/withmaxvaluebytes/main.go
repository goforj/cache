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
	// Example: limit value size via explicit StoreConfig.
	ctx := context.Background()
	store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
		BaseConfig: cachecore.BaseConfig{MaxValueBytes: 1024},
	})
	fmt.Println(store.Driver()) // memory
}
