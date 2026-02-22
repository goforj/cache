//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
)

func main() {
	// DeleteMany removes multiple keys.

	// Example: delete many keys
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	fmt.Println(c.DeleteMany("a", "b") == nil) // true
}
