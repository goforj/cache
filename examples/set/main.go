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
	repo := cache.NewRepository(cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory}))
	_ = repo.Set(ctx, "token", []byte("abc"), time.Minute)
}
