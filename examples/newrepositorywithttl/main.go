//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// NewRepositoryWithTTL lets callers override the default TTL applied when ttl <= 0.

	// Example: repository with custom default TTL
	ctx := context.Background()
	store := cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory})
	repo := cache.NewRepositoryWithTTL(store, 2*time.Minute)
	_ = ctx
	_ = repo
}
