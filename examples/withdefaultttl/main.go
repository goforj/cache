//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// WithDefaultTTL overrides the fallback TTL used when ttl <= 0.

	// Example: override default TTL
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithDefaultTTL(30*time.Second))
	fmt.Println(store.Driver()) // memory
}
