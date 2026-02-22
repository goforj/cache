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
	// RateLimit increments a fixed-window counter and returns allowance metadata.

	// Example: fixed-window rate limit metadata
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	res, err := c.RateLimit("rl:api:ip:1.2.3.4", 100, time.Minute)
	fmt.Println(err == nil, res.Allowed, res.Count, res.Remaining, !res.ResetAt.IsZero())
	// Output: true true 1 99 true
}
