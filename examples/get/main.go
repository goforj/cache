//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// Get returns raw bytes for key when present.

	// Example: get bytes
	ctx := context.Background()
	s := cache.NewMemoryStore(ctx)
	c := cache.NewCache(s)
	_ = c.Set(ctx, "user:42", []byte("Ada"), 0)
	value, ok, _ := c.Get(ctx, "user:42")
	fmt.Println(ok, string(value)) // true Ada
}
