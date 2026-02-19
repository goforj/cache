//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Remember returns key value or computes/stores it when missing.

	// Example: remember bytes
	ctx := context.Background()
	repo := cache.NewRepository(cache.NewStore(ctx, cache.StoreConfig{Driver: cache.DriverMemory}))
	data, err := repo.Remember(ctx, "dashboard:summary", time.Minute, func(context.Context) ([]byte, error) {
		return []byte("payload"), nil
	})
	_ = data
	_ = err
}
