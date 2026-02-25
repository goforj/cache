//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/memcachedcache"
	"time"
)

func main() {
	// New builds a Memcached-backed cachecore.Store.
	// 
	// Defaults:
	// - Addresses: []string{"127.0.0.1:11211"} when empty
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty

	// Example: memcached cluster via explicit driver config
	store := memcachedcache.New(memcachedcache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		Addresses: []string{"127.0.0.1:11211"},
	})
	fmt.Println(store.Driver()) // memcached
}
