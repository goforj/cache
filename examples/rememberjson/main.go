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
	// RememberJSON returns key value or computes/stores JSON when missing.

	// Example: remember JSON
	type Settings struct { Enabled bool `json:"enabled"` }
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	settings, err := cache.RememberJSON[Settings](c, "settings:alerts", time.Minute, func() (Settings, error) {
		return Settings{Enabled: true}, nil
	})
	fmt.Println(err == nil, settings.Enabled) // true true
}
