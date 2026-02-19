//go:build ignore
// +build ignore

package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Add writes value only when key is not already present.

	// Example: add once
	ctx := context.Background()
	s := cache.NewMemoryStore(ctx)
	c := cache.NewCache(s)
	created, _ := c.Add(ctx, "boot:seeded", []byte("1"), time.Hour)
	_ = created
}
