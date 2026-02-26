package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewNullStore is a no-op store useful for tests where caching should be disabled.

	// Example: null helper
	ctx := context.Background()
	store := cache.NewNullStore(ctx)
	fmt.Println(store.Driver()) // null
}
