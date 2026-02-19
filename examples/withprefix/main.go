//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithPrefix sets the key prefix for shared backends (e.g., redis).

	// Example: prefix keys
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverRedis, cache.WithPrefix("svc"))
	fmt.Println(store.Driver()) // redis
}
