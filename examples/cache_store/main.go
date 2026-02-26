package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// Store returns the underlying store implementation.

	// Example: access store
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	fmt.Println(c.Store().Driver()) // memory
}
