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
	// Example: override default TTL via explicit StoreConfig.
	ctx := context.Background()
	store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{
		BaseConfig: cachecore.BaseConfig{DefaultTTL: 30 * time.Second},
	})
	fmt.Println(store.Driver()) // memory
}
