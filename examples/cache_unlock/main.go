package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

func main() {
	// Unlock releases a previously acquired lock key.

	// Example: unlock key
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	locked, _ := c.TryLock("job:sync", 10*time.Second)
	if locked {
		_ = c.Unlock("job:sync")
	}
}
