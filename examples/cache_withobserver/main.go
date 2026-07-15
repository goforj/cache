package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// WithObserver attaches an observer to receive operation events.
	// WithObserver mutates c for backward compatibility and must be called during construction,
	// before c is used concurrently. The observer must be safe for concurrent callbacks.

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
