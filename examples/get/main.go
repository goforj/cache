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
	store := cache.NewMemoryStore(ctx)
	cache := cache.NewCache(store)
	_ = cache.Set(ctx, "user:42", []byte("Ada"), 0)
	value, ok, _ := cache.Get(ctx, "user:42")
	_ = value
	_ = ok
}
