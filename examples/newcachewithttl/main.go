//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.

	// Example: cache with custom default TTL
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	c := cache.NewCacheWithTTL(store, 2*time.Minute)
	_ = ctx
	_ = c
}
