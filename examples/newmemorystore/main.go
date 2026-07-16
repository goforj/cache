package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// NewMemoryStore is a convenience for an in-process store using defaults.

	// Example: memory helper
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	fmt.Println(store.Driver()) // memory
}
