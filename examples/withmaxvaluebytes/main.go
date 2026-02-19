//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithMaxValueBytes sets a per-entry size limit (0 disables the check).

	// Example: limit value size
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithMaxValueBytes(1024))
	fmt.Println(store.Driver()) // memory
}
