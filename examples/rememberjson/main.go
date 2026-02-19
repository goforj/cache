//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// RememberJSON returns key value or computes/stores JSON when missing.

	// Example: remember JSON
	type Settings struct {
		Enabled bool `json:"enabled"`
	}
	ctx := context.Background()
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewCache(store)
	settings, err := cache.RememberJSON[Settings](ctx, repo, "settings:alerts", time.Minute, func(context.Context) (Settings, error) {
		return Settings{Enabled: true}, nil
	})
	_ = settings
	_ = err
}
