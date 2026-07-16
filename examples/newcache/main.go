package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// NewCache creates a cache facade bound to a concrete store.
	// NewCache panics when store is nil because a cache cannot operate without its required backend.

	// Example: cache from store
	ctx := context.Background()
	s := cache.NewMemoryStore(ctx)
	c := cache.NewCache(s)
	fmt.Println(c.Driver()) // memory
}
