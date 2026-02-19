//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// SetString writes a string value to key.

	// Example: set string
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)
	_ = repo.SetString(ctx, "user:42:name", "Ada", time.Minute)
}
