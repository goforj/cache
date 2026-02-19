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
	// Add writes value only when key is not already present.

	// Example: add once
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	created, _ := c.Add(ctx, "boot:seeded", []byte("1"), time.Hour)
	fmt.Println(created) // true
}
