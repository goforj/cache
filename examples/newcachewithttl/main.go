package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.

	// Example: cache with custom default TTL
	ctx := context.Background()
	s := cache.NewMemoryStore(ctx)
	c := cache.NewCacheWithTTL(s, 2*time.Minute)
	fmt.Println(c.Driver(), c != nil) // memory true
}
