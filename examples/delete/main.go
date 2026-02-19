//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// Delete removes a single key.

	// Example: delete key
	ctx := context.Background()
	repo := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = repo.Delete(ctx, "a")
}
