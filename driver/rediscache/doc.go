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
//		Client: rdb,
//		Prefix: "app",
//	})
//	c := cache.NewCache(store)
package rediscache
