//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"github.com/redis/go-redis/v9"
)

func main() {
	// WithRedisClient sets the redis client; required when using DriverRedis.

	// Example: inject redis client
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store := cache.NewStoreWith(ctx, cache.DriverRedis, cache.WithRedisClient(rdb))
	fmt.Println(store.Driver()) // redis
}
