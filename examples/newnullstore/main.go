package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// NewNullStore is a no-op store useful for tests where caching should be disabled.

	// Example: null helper
	ctx := context.Background()
	store := cache.NewNullStore(ctx)
	fmt.Println(store.Driver()) // null
}
