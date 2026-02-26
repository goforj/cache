package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewFileStore is a convenience for a filesystem-backed store.

	// Example: file helper
	ctx := context.Background()
	store := cache.NewFileStore(ctx, "/tmp/my-cache")
	fmt.Println(store.Driver()) // file
}
