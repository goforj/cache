//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Example: custom memory sweep via explicit StoreConfig.
	ctx := context.Background()
	store := cache.NewMemoryStoreWithConfig(ctx, cache.StoreConfig{MemoryCleanupInterval: 5 * time.Minute})
	fmt.Println(store.Driver()) // memory
}
