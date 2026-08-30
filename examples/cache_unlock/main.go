package main

import (
	"context"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Unlock closes this Cache instance's lifecycle and releases the key only while it still owns the backend lock.

	// Example: unlock key
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	locked, _ := c.TryLock("job:sync", 10*time.Second)
	if locked {
		_ = c.Unlock("job:sync")
	}
}
