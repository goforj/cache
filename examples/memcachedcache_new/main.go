package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/memcachedcache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// New builds a lazy Memcached store that owns its connections. The caller can
	// release them through io.Closer. Addresses are treated as a fixed peer set.
	// A failed peer returns an error for its keys so locks never fail over to another peer.
	//
	// Defaults:
	// - Addresses: []string{"127.0.0.1:11211"} when empty
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - AllowServerFlush: false, unless every server is dedicated to this cache

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
