//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// NewCache creates a cache facade bound to a concrete store.

	// Example: cache from store
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	c := cache.NewCache(store)
	_ = c
}
