//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// NewRepository creates a cache repository bound to a concrete store.

	// Example: repository from store
	ctx := context.Background()
	store := cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory})
	repo := cache.NewRepository(store)
	_ = repo
}
