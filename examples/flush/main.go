//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// Flush clears all keys for this store scope.

	// Example: flush all keys
	ctx := context.Background()
	repo := cache.NewRepository(cache.NewMemoryStore(ctx))
	_ = repo.Flush(ctx)
}
