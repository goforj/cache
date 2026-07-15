package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// NewCacheWithTTL lets callers override the default TTL applied when ttl <= 0.
	// NewCacheWithTTL panics when store is nil because accepting invalid wiring would defer failure to the first operation.

	// Example: cache with custom default TTL
	ctx := context.Background()
	s := cache.NewMemoryStore(ctx)
	c := cache.NewCacheWithTTL(s, 2*time.Minute)
	fmt.Println(c.Driver(), c != nil) // memory true
}
