//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// NewMemoryStore is a convenience for an in-process store with optional overrides.

	// Example: memory helper
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	_ = store
}
