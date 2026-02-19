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
	// Flush clears all keys for this store scope.

	// Example: flush all keys
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	_ = c.Set("a", []byte("1"), time.Minute)
	fmt.Println(c.Flush() == nil) // true
}
