package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// SetJSON encodes value as JSON and writes it to key.

	// Example: set typed JSON
	type Settings struct {
		Enabled bool `json:"enabled"`
	}
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	err := c.SetJSON("settings:alerts", Settings{Enabled: true}, time.Minute)
	fmt.Println(err == nil) // true
}
