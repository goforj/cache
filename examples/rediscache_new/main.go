//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/goforj/cache/cachecore"
	"github.com/goforj/cache/driver/rediscache"
	"time"
)

func main() {
	// New builds a Redis-backed cachecore.Store.
	// 
	// Defaults:
	// - DefaultTTL: 5*time.Minute when zero
	// - Prefix: "app" when empty
	// - Addr: empty by default (no client auto-created unless Addr is set)
	// - Client: optional advanced override (takes precedence when set)
	// - If neither Client nor Addr is set, operations return errors until a client is provided

	// Example: explicit Redis driver config
	store := rediscache.New(rediscache.Config{
		BaseConfig: cachecore.BaseConfig{
			DefaultTTL: 5 * time.Minute,
			Prefix:     "app",
		},
		Addr: "127.0.0.1:6379",
	})
	fmt.Println(store.Driver()) // redis
}
