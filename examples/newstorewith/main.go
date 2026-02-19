//go:build ignore
// +build ignore

package main

import (
	"context"
	"time"

	"github.com/goforj/cache"
	"github.com/redis/go-redis/v9"
)

func main() {
	// NewStoreWith builds a store using a driver and a set of functional options.
	// Required data (e.g., Redis client) must be provided via options when needed.

	// Example: memory store (options)
	ctx := context.Background()
	memoryStore := cache.NewStoreWith(ctx, cache.DriverMemory)
	_ = memoryStore

	// Example: redis store (options)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	redisStore := cache.NewStoreWith(ctx, cache.DriverRedis,
		cache.WithRedisClient(client),
		cache.WithPrefix("app"),
		cache.WithDefaultTTL(5*time.Minute),
	)
	_ = redisStore
}
