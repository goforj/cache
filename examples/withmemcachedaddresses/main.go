//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// WithMemcachedAddresses sets memcached server addresses (host:port).

	// Example: memcached cluster
	ctx := context.Background()
	store := cache.NewStoreWith(ctx, cache.DriverMemcached, cache.WithMemcachedAddresses("127.0.0.1:11211"))
	fmt.Println(store.Driver()) // memcached
}
