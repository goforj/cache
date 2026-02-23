//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewMemoryStore is a convenience for an in-process store using defaults.

	// Example: memory helper
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	fmt.Println(store.Driver()) // memory
}
