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
	// Delete removes a single key.

	// Example: delete key
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.Set("a", []byte("1"), time.Minute)
	fmt.Println(c.Delete("a") == nil) // true
}
