//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/rediscache"
	"github.com/redis/go-redis/v9"
	"time"
)

func main() {
	// New builds a Redis-backed cachecore.Store.
	// 
	// Defaults:
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - Client: nil allowed (operations return errors until a client is provided)

	// Example: explicit Redis driver config
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store := rediscache.New(rediscache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		Client: rdb,
	})
	fmt.Println(store.Driver()) // redis
}
