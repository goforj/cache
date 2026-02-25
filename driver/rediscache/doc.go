// Package rediscache provides a Redis-backed cache.Store implementation.
//
// Example:
//
//	import (
//		"github.com/goforj/cache"
//		"github.com/goforj/cache/driver/rediscache"
//	)
//
//	store := rediscache.New(rediscache.Config{
//		Addr:   "127.0.0.1:6379",
//		Prefix: "app",
//	})
//	c := cache.NewCache(store)
package rediscache
