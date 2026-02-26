package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// RememberStaleBytes returns a fresh value when available, otherwise computes and caches it.
	// If computing fails and a stale value exists, it returns the stale value.
	// The returned bool is true when a stale fallback was used.

	// Example: stale fallback on upstream failure
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	body, usedStale, err := c.RememberStaleBytes("profile:42", time.Minute, 10*time.Minute, func() ([]byte, error) {
		return []byte(`{"name":"Ada"}`), nil
	})
	fmt.Println(err == nil, usedStale, len(body) > 0)
}
