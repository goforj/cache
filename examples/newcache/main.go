package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewCache creates a cache facade bound to a concrete store.

	// Example: cache from store
	ctx := context.Background()
	s := cache.NewMemoryStore(ctx)
	c := cache.NewCache(s)
	fmt.Println(c.Driver()) // memory
}
