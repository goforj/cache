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

	// Example: set typed value
	type Settings struct { Enabled bool `json:"enabled"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	err := cache.Set(c, "settings:alerts", Settings{Enabled: true}, time.Minute)
	fmt.Println(err == nil) // true
}
