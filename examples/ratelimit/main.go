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
	// RateLimit increments key in a fixed window and reports whether requests are allowed.

	// Example: fixed-window rate limit
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	allowed, count, _ := c.RateLimit("rl:login:user:42", 5, time.Minute)
	fmt.Println(allowed, count >= 1) // true true
}
