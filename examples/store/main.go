//go:build ignore
// +build ignore

package main

import (
	"context"

	"github.com/goforj/cache"
)

func main() {
	// Store returns the underlying store implementation.

	// Example: access store
	ctx := context.Background()
	cacheRepo := cache.NewCache(cache.NewMemoryStore(ctx))
	store := cacheRepo.Store()
	_ = store
}
