//go:build ignore
// +build ignore

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
	c = c.WithObserver(cache.ObserverFunc(func(ctx context.Context, op, key string, hit bool, err error, dur time.Duration, driver cachecore.Driver) {
		// See docs/production-guide.md for a real metrics recipe.
		fmt.Println(op, driver, hit, err == nil)
		_ = ctx
		_ = key
		_ = dur
	}))
	_, _, _ = c.GetBytes("profile:42")
}
