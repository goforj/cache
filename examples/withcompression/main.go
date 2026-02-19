//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithCompression enables value compression using the chosen codec.

	// Example: gzip compression
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverMemory, cache.WithCompression(cache.CompressionGzip))
	fmt.Println(store.Driver()) // memory
}
