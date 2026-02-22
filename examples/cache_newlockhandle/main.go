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
	// NewLockHandle creates a reusable lock handle for a key/ttl pair.

	// Example: lock handle acquire/release
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	lock := c.NewLockHandle("job:sync", 10*time.Second)
	locked, err := lock.Acquire()
	fmt.Println(err == nil, locked) // true true
	if locked {
		_ = lock.Release()
	}
}
