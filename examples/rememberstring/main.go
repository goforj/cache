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
	store := cache.NewMemoryStore(ctx)
	repo := cache.NewRepository(store)
	value, err := repo.RememberString(ctx, "settings:mode", time.Minute, func(context.Context) (string, error) {
		return "on", nil
	})
	_ = value
	_ = err
}
