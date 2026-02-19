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
	// Decrement decrements a numeric value and returns the result.

	// Example: decrement counter
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	val, _ := c.Decrement(ctx, "rate:login:42", 1, time.Minute)
	fmt.Println(val) // -1
}
