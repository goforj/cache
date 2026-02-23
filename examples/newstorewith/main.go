//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/rediscache"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Explicit constructors are preferred over the removed generic factory.

	// Example: memory store
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	fmt.Println(store.Driver()) // memory

	// Example: redis driver constructor
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store = rediscache.New(rediscache.Config{
		BaseConfig: cachecore.BaseConfig{Prefix: "app"},
		Client:     redisClient,
	})
	fmt.Println(store.Driver()) // redis
}
