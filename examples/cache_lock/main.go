package main

import (
	"context"
	"fmt"
	"github.com/goforj/cache"
	"time"
)

// main keeps this generated example executable so API drift fails during compilation.
func main() {
	// Lock waits until this Cache instance and the backend can begin a lock lifecycle or timeout elapses.

	// Example: lock with timeout
	ctx := context.Background()
	c := cache.NewCache(cache.NewMemoryStore(ctx))
	locked, err := c.Lock("job:sync", 10*time.Second, time.Second)
	fmt.Println(err == nil, locked) // true true
	if locked {
		_ = c.Unlock("job:sync")
	}
}
