package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"time"
)

func main() {
	// WithObserver attaches an observer to receive operation events.

	// Example: attach observer
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	c = c.WithObserver(cache.ObserverFunc(func(ctx context.Context, event cache.CacheOpEvent) {
		// See docs/production-guide.md for a real metrics recipe.
		fmt.Println(event.Operation, event.Driver, event.Hit, event.Err == nil)
		_ = ctx
		_ = event.Key
		_ = event.Duration
	}))
	_, _, _ = c.GetBytes("profile:42")
}
