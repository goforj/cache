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
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewCache(store)
	value, _ := repo.Increment(ctx, "rate:login:42", 1, time.Minute)
	_ = value
}
