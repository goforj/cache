//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Set encodes value with the default codec (JSON) and writes it to key.

	// Example: set typed values (struct + string)
	type Settings struct { Enabled bool `json:"enabled"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	err := cache.Set(c, "settings:alerts", Settings{Enabled: true}, time.Minute)
	err2 := cache.Set(c, "settings:mode", "dark", time.Minute)
	fmt.Println(err == nil, err2 == nil) // true true
}
