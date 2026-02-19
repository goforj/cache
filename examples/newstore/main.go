//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// NewStore returns a concrete store for the requested driver.
	// Caller is responsible for providing any driver-specific dependencies.

	// Example: select driver explicitly
	ctx := context.Background()
	store := cache.NewStore(ctx, cache.StoreConfig{
		Driver: cache.DriverMemory,
	})
	_ = store
}
