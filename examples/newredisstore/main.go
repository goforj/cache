//go:build ignore
// +build ignore

package main

import (
	"context"

	"github.com/goforj/cache"
	"github.com/redis/go-redis/v9"
)

func main() {
	// NewRedisStore is a convenience for a redis-backed store. Redis client is required.

	// Example: redis helper
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store := cache.NewRedisStore(ctx, client, cache.WithPrefix("app"))
	_ = store
}
