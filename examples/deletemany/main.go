//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// DeleteMany removes multiple keys.

	// Example: delete many keys
	ctx := context.Background()
	repo := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = repo.DeleteMany(ctx, "a", "b")
}
