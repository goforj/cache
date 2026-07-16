package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// NewFileStore is a convenience for a filesystem-backed store.

	// Example: file helper
	ctx := context.Background()
	store := cache.NewFileStore(ctx, "/tmp/my-cache")
	fmt.Println(store.Driver()) // file
}
