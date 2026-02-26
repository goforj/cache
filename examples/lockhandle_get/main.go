package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Get acquires the lock once, runs fn if acquired, then releases automatically.

	// Example: acquire once and auto-release
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	lock := c.NewLockHandle("job:sync", 10*time.Second)
	locked, err := lock.Get(func() error {
		// do protected work
		return nil
	})
	fmt.Println(err == nil, locked) // true true
}
