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
	// Increment increments a numeric value and returns the result.

	// Example: increment counter
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	val, _ := c.Increment("rate:login:42", 1, time.Minute)
	fmt.Println(val) // 1
}
