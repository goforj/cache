//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Store returns the underlying store implementation.

	// Example: access store
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	fmt.Println(c.Store().Driver()) // memory
}
