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
	// SetString writes a string value to key.

	// Example: set string
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	fmt.Println(c.SetString("user:42:name", "Ada", time.Minute) == nil) // true
}
