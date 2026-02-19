//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Add writes value only when key is not already present.

	// Example: add once
	ctx := context.Background()
	repo := cache.NewRepository(cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory}))
	created, _ := repo.Add(ctx, "boot:seeded", []byte("1"), time.Hour)
	_ = created
}
