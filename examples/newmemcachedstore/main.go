//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// NewMemcachedStore is a convenience for a memcached-backed store.

	// Example: memcached helper
	ctx := context.Background()
	store := cache.NewMemcachedStore(ctx, []string{"127.0.0.1:11211"})
	fmt.Println(store.Driver()) // memcached
}
