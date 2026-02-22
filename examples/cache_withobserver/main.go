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
	// WithObserver attaches an observer to receive operation events.

	// Example: attach observer
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	c = c.WithObserver(cache.ObserverFunc(func(ctx context.Context, op, key string, hit bool, err error, dur time.Duration, driver cache.Driver) {
		// See docs/production-guide.md for a real metrics recipe.
		fmt.Println(op, driver, hit, err == nil)
		_ = ctx
		_ = key
		_ = dur
	}))
	_, _, _ = c.GetBytes("profile:42")
}
