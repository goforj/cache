package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Lock waits until the lock is acquired or timeout elapses.

	// Example: lock with timeout
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	locked, err := c.Lock("job:sync", 10*time.Second, time.Second)
	fmt.Println(err == nil, locked) // true true
}
