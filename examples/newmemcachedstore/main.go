//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/driver/memcachedcache"
)

func main() {
	// Memcached driver constructor.
	store := memcachedcache.New(memcachedcache.Config{
		Addresses: []string{"127.0.0.1:11211"},
	})
	fmt.Println(store.Driver()) // memcached
}
