//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// NewMemoStore decorates store with per-process read memoization.

	// Example: memoize a backing store
	ctx := context.Background()
	base := cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory})
	memoStore := cache.NewMemoStore(base)
	repo := cache.NewRepository(memoStore)
	_ = repo
}
