//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
)

func main() {
	// GetString returns a UTF-8 string value for key when present.

	// Example: get string
	ctx := context.Background()
	repo := cache.NewRepository(cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory}))
	_ = repo.SetString(ctx, "user:42:name", "Ada", 0)
	name, ok, _ := repo.GetString(ctx, "user:42:name")
	_ = name
	_ = ok
}
