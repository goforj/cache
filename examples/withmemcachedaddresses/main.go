//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/driver/memcachedcache"
)

func main() {
	// Example: memcached cluster via explicit driver config.
	store := memcachedcache.New(memcachedcache.Config{
		Addresses: []string{"127.0.0.1:11211"},
	})
	fmt.Println(store.Driver()) // memcached
}
