//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Increment increments a numeric value and returns the result.

	// Example: increment counter
	ctx := context.Background()
	repo := cache.NewRepository(cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory}))
	value, _ := repo.Increment(ctx, "rate:login:42", 1, time.Minute)
	_ = value
}
