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
	// RateLimitWithRemaining increments a fixed-window counter and returns allowance metadata.

	// Example: rate limit headers metadata
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	allowed, count, remaining, resetAt, err := c.RateLimitWithRemaining("rl:api:ip:1.2.3.4", 100, time.Minute)
	fmt.Println(err == nil, allowed, count >= 1, remaining >= 0, resetAt.After(time.Now()))
}
