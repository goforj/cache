//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Set writes raw bytes to key.

	// Example: set bytes with ttl
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)
	_ = repo.Set(ctx, "token", []byte("abc"), time.Minute)
}
