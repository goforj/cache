//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// Example: explicit memory constructor.
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	fmt.Println(store.Driver()) // memory
}
