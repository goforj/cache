//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Decrement decrements a numeric value and returns the result.

	// Example: decrement counter
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)
	value, _ := repo.Decrement(ctx, "rate:login:42", 1, time.Minute)
	_ = value
}
