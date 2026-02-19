//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/goforj/cache"
)

func main() {
	// NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.

	// Example: repository with custom default TTL
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	c := cache.NewCacheWithTTL(store, 2*time.Minute)
	fmt.Println(c.Driver(), store != nil, ctx != nil)
}
