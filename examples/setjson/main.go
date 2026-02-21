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
	// SetJSON encodes value as JSON and writes it to key using background context.

	// Example: set typed JSON
	type Settings struct { Enabled bool `json:"enabled"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	err := cache.SetJSON(c, "settings:alerts", Settings{Enabled: true}, time.Minute)
	fmt.Println(err == nil) // true
}
