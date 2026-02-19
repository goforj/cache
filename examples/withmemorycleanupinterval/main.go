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
	// WithMemoryCleanupInterval overrides the sweep interval for the memory driver.

	// Example: custom memory sweep
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithMemoryCleanupInterval(5*time.Minute))
	fmt.Println(store.Driver()) // memory
}
