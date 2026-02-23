//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/driver/rediscache"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Redis driver constructor.
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	store := rediscache.New(rediscache.Config{Client: redisClient})
	fmt.Println(store.Driver()) // redis
}
