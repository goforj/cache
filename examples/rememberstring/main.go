//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// RememberString returns key value or computes/stores it when missing.

	// Example: remember string
	ctx := context.Background()
	repo := cache.NewRepository(cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory}))
	value, err := repo.RememberString(ctx, "settings:mode", time.Minute, func(context.Context) (string, error) {
		return "on", nil
	})
	_ = value
	_ = err
}
