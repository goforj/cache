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
	store := cache.NewMemoryStore(ctx)
	c := cache.NewCache(store)
	data, err := c.Remember(ctx, "dashboard:summary", time.Minute, func(context.Context) ([]byte, error) {
		return []byte("payload"), nil
	})
	_ = data
	_ = err
}
