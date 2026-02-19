//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// Get returns raw bytes for key when present.

	// Example: get bytes
	ctx := context.Background()
	repo := cache.NewRepository(cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory}))
	_ = repo.Set(ctx, "user:42", []byte("Ada"), 0)
	value, ok, _ := repo.Get(ctx, "user:42")
	_ = value
	_ = ok
}
