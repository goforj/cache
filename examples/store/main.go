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
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)

	underlying := repo.Store()
	_ = underlying
}
